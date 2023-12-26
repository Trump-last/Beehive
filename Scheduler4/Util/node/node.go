/*
 * @Autor: XLF
 * @Date: 2022-10-28 15:57:46
 * @LastEditors: XLF
 * @LastEditTime: 2023-06-25 15:16:34
 * @Description:
 */
package node

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	config "scheduler4/Config"
	public "scheduler4/PublicStu"
	"scheduler4/Util/logging"
	"scheduler4/Util/myqueue"
	"scheduler4/Util/node/resource"
	task "scheduler4/Util/node/task"
	servermap "scheduler4/Util/serverMap"
	taskinfo "scheduler4/Util/taskinfo"
	"scheduler4/Util/workslice"
	"strings"
	"sync/atomic"
	"time"

	nodegrpc "gitee.com/linfeng-xu/protofile/nodeGrpc"

	cmap "github.com/orcaman/concurrent-map/v2"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type Resource interface {
	PreAlloc(*task.Task) error
	CancelAlloc(*task.Task) error
	RunTask(*task.Task) error
	GetAvailRes() [resource.ResourceNum]float32
}

type Node struct {
	IP        string                                         // 节点的IP地址
	GRPCport  string                                         // 节点的gRPC端口
	IdTaskMap cmap.ConcurrentMap[string, *taskinfo.TaskInfo] // 节点的任务信息
	Res       Resource                                       // 节点的资源信息
	Mytask    myqueue.FcfsQ[*task.Task]                      // 节点的由LB获得的任务分发队列
	Request   chan *task.Task                                // 节点的request队列，用于存放转发而来的任务
	Latebind  chan *task.Task                                // 节点的latebind队列，用于存放资源不足的任务
	ConfirMsg chan string                                    // 任务调度成功的确认消息队列，供管理协程删除map对应的记录
	ErrorMsg  chan string                                    // 任务出错的消息队列，供管理协程重试map对应的任务
	State     int32                                          // 节点状态	0：未完成初始化	1：已完成初始化
}

func NewNode(cpu float32, ram float32) Node {
	return Node{
		IP:        config.GetOutBoundIP(),
		GRPCport:  GetFreePort(),
		Res:       resource.NewTaskManagerFrom([2]float32{cpu, ram}),
		IdTaskMap: cmap.New[*taskinfo.TaskInfo](),
		Mytask:    myqueue.NewfcfsQ[*task.Task](),
		Request:   make(chan *task.Task, 100),
		Latebind:  make(chan *task.Task, 100),
		ConfirMsg: make(chan string, 100),
		ErrorMsg:  make(chan string, 100),
		State:     0,
	}
}

func NewNodePointer(cpu float32, ram float32) *Node {
	return &Node{
		IP:        config.GetOutBoundIP(),
		GRPCport:  GetFreePort(),
		Res:       resource.NewTaskManagerFrom([2]float32{cpu, ram}),
		IdTaskMap: cmap.New[*taskinfo.TaskInfo](),
		Mytask:    myqueue.NewfcfsQ[*task.Task](),
		Request:   make(chan *task.Task, 100),
		Latebind:  make(chan *task.Task, 100),
		ConfirMsg: make(chan string, 100),
		ErrorMsg:  make(chan string, 100),
		State:     0,
	}
}

func GetFreePort() string {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return ""
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return ""
	}
	defer l.Close()
	//将端口号转换为字符串
	return fmt.Sprintf("%d", l.Addr().(*net.TCPAddr).Port)
}

/************************Node grpc server************************/

func serverInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	if info.FullMethod == "/nodeGrpc.NodeServer/TaskDivide" {
		s, _ := metadata.FromIncomingContext(ctx)
		addr := s["addr"][0]
		// fmt.Println(addr, "测试拦截器")
		if !servermap.HasPeer(addr) {
			if servermap.TryAddPeer(addr) {
				fmt.Println("添加节点", addr)
				servermap.AddPeerConn(addr)
			}
		}
	}
	// Pre-processing logic
	resp, err := handler(ctx, req)
	// Post-processing logic
	return resp, err
}

// 节点打招呼获取节点信息
func (n *Node) HelloNode(ctx context.Context, arg *nodegrpc.EmptyReply) (*nodegrpc.NodeList, error) {
	//nbh := servermap.GetNbh()                          // 获取邻域节点列表
	//lists := make([]*nodegrpc.NodeInfo, 1, len(nbh)+1) // 构造gRPC消息
	lists := make([]*nodegrpc.NodeInfo, 1) // 构造gRPC消息
	lists[0] = n.GetInfo()
	//lists = append(lists, nbh...) // 将邻域节点的信息放入回传数组
	return &nodegrpc.NodeList{Nodes: lists}, nil
}

// 获取节点状态信息
func (n *Node) GetState(ctx context.Context, arg *nodegrpc.EmptyReply) (*nodegrpc.NodeState, error) {
	s := atomic.LoadInt32(&n.State)
	return &nodegrpc.NodeState{State: s}, nil
}

// 任务分发gRPC实现，是所有任务执行的入口
func (n *Node) TaskDispense(ctx context.Context, arg *nodegrpc.TaskBasicImf) (*nodegrpc.EmptyReply, error) {
	select {
	case <-ctx.Done():
		return &nodegrpc.EmptyReply{}, errors.New("dispense task timeout")
	default:
	}

	go logging.NoteLogQueue(time.Now().Format(time.RFC3339Nano), config.Dispense, arg.Taskid, n.IP+":"+n.GRPCport, arg.Taskcpu, arg.Taskram)
	t := task.NewTaskPointer(arg.Taskid, arg.Taskcpu, arg.Taskram, n.IP+":"+n.GRPCport, arg.Command, arg.Submittime, arg.PrenodeIp, arg.Maxhop) // 构造task.task对象
	n.PushTask(t)
	return &nodegrpc.EmptyReply{}, nil
}

// 任务分发gRPC实现，是所有任务执行的入口
func (n *Node) TaskDispenseByStream(stream nodegrpc.NodeServer_TaskDispenseByStreamServer) error {
	// select {
	// case <-ctx.Done():
	// 	return &nodegrpc.EmptyReply{}, errors.New("dispense task timeout")
	// default:
	// }

	// go logging.NoteLogQueue(time.Now().Format(time.RFC3339Nano), config.Dispense, arg.Taskid, n.IP, arg.Taskcpu, arg.Taskram)
	// t := task.NewTaskPointer(arg.Taskid, arg.Taskcpu, arg.Taskram, n.IP+":"+n.GRPCport, arg.Command, arg.Submittime, arg.PrenodeIp, arg.Maxhop) // 构造task.task对象
	// n.PushTask(t)
	// return &nodegrpc.EmptyReply{}, nil
	for {
		arg, err := stream.Recv()
		switch err {
		case io.EOF:
			return nil
		case nil:
			go logging.NoteLogQueue(time.Now().Format(time.RFC3339Nano), config.Dispense, arg.Taskid, n.IP+":"+n.GRPCport, arg.Taskcpu, arg.Taskram)
			go func() {
				t := task.NewTaskPointer(arg.Taskid, arg.Taskcpu, arg.Taskram, n.IP+":"+n.GRPCport, arg.Command, arg.Submittime, arg.PrenodeIp, arg.Maxhop) // 构造task.task对象
				n.PushTask(t)
			}()
		default:
			return err
		}
	}
}

// 流浪任务计划，在第一次两次探测失败后启动，进行任务的多跳
func (n *Node) WanderTask(ctx context.Context, arg *nodegrpc.TaskBasicImf) (*nodegrpc.NodeInfo, error) {
	select {
	case <-ctx.Done():
		return nil, errors.New("WanderTask task timeout")
	default:
	}

	t := task.NewTaskPointer(arg.Taskid, arg.Taskcpu, arg.Taskram, n.IP+":"+n.GRPCport, arg.Command, arg.Submittime, arg.PrenodeIp, arg.Maxhop-1) // 构造task.task对象
	go logging.NoteLogQueue(time.Now().Format(time.RFC3339Nano), config.WanderGet, arg.Taskid, n.IP+":"+n.GRPCport+" get task from "+arg.PrenodeIp, arg.Taskcpu, arg.Taskram)
	//n.PushTaskByPriority(t, 0)
	n.PushTask(t)
	return n.GetInfo(), nil
}

// 转发函数，本节点作为调度器节点转发任务
func (n *Node) dividing(t *task.Task, addr []string) {
	ws := workslice.SetWorksByString(addr)                                                                // 将邻域服务器列表转换为workslice.WorkSlice对象
	taskinf := taskinfo.NewTaskInfo(t.Id, t.Cpu, t.Ram, t.Command, t.Submittime, t.Prenode, t.Maxhop, ws) // 构造taskinfo.TaskInfo对象
	n.IdTaskMap.Set(t.Id, taskinf)                                                                        // 将taskinfo.TaskInfo对象存入IdTaskMap
	for _, v := range addr {
		client, err := servermap.GetConnByAddr(v)
		if err != nil { // 如果找不到连接，就临时建立一个
			go n.tempdivide(v, t)
			continue
		}
		go divideGrpc(v, t, client)
	}
}

// 临时转发函数，用于servermap由于更新丢失连接的情况
func (n *Node) tempdivide(ip string, t *task.Task) {
	conn, err := grpc.Dial(ip, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return
	}
	defer conn.Close()
	client := nodegrpc.NewNodeServerClient(conn)
	msg := &nodegrpc.TaskDivideImf{ // 构造gRPC消息
		BasicImf: &nodegrpc.TaskBasicImf{
			Taskid:  t.Id,
			Taskram: t.Ram,
			Taskcpu: t.Cpu,
			Command: t.Command,
		},
		Taskip:  t.Addr,
		Timeout: 0,
	}
	md := metadata.Pairs("addr", n.IP+":"+n.GRPCport)
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	reply, errc := client.TaskDivide(ctx, msg) // 发起gRPC调用
	if errc != nil {
		logging.WriteLog("tempdivide error is " + ip + " " + fmt.Sprintln(errc))
		t, ok := n.IdTaskMap.Get(t.Id) // 获取任务对象
		if ok {
			t.AddErrNum(ip, n.ErrorMsg) // 记录任务的出错次数
		}
		return
	}
	servermap.UpdateNbh(reply) // 更新某个邻域节点的信息，及其它的邻域的节点信息
}

// 重转发函数，管理协程的调用
func (n *Node) nextwander(t *task.Task, addr []string) {
	client, err := servermap.GetConnByAddr(addr[0])
	if err != nil { // 如果找不到连接，就临时建立一个
		n.tempwander(t, addr[0])
		return
	}
	msg := &nodegrpc.TaskBasicImf{ // 构造gRPC消息
		Taskid:     t.Id,
		Taskram:    t.Ram,
		Taskcpu:    t.Cpu,
		Command:    t.Command,
		PrenodeIp:  n.IP + ":" + n.GRPCport,
		Submittime: t.Submittime,
		Maxhop:     t.Maxhop,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	reply, err := (*client).WanderTask(ctx, msg) // 发起gRPC调用
	if err != nil {                              // 如果调用失败，就记录日志
		logging.WriteLog("wanderdivide: " + fmt.Sprintln(err))
		t.Prenode = addr[0]
		n.PushTask(t) // 将任务重新放入任务队列
		return
	}
	servermap.UpdateNbh(reply) // 更新某个邻域节点的信息，及其它的邻域的节点信息
}

// 建立临时流浪连接，因为map更新后，之前选择的节点信息会丢失，连接会访问不到，所以新建一个临时连接，一般这种情况比较少
func (n *Node) tempwander(t *task.Task, addr string) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logging.WriteLog("wanderdivide: " + fmt.Sprintln(err))
		return
	}
	defer conn.Close()
	client := nodegrpc.NewNodeServerClient(conn) // 构造gRPC客户端
	msg := &nodegrpc.TaskBasicImf{               // 构造gRPC消息
		Taskid:     t.Id,
		Taskram:    t.Ram,
		Taskcpu:    t.Cpu,
		Command:    t.Command,
		PrenodeIp:  n.IP + ":" + n.GRPCport,
		Submittime: t.Submittime,
		Maxhop:     t.Maxhop,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	reply, err := client.WanderTask(ctx, msg) // 发起gRPC调用
	if err != nil {                           // 如果调用失败，就记录日志
		logging.WriteLog("wanderdivide: " + fmt.Sprintln(err))
		t.Prenode = addr
		n.PushTask(t) // 将任务重新放入任务队列
		return
	}
	servermap.UpdateNbh(reply) // 更新某个邻域节点的信息，及其它的邻域的节点信息
}

// 转发gRPC发起函数
func divideGrpc(ip string, t *task.Task, client *nodegrpc.NodeServerClient) {
	msg := &nodegrpc.TaskDivideImf{ // 构造gRPC消息
		BasicImf: &nodegrpc.TaskBasicImf{
			Taskid:  t.Id,
			Taskram: t.Ram,
			Taskcpu: t.Cpu,
			Command: t.Command,
		},
		Taskip:  t.Addr,
		Timeout: 0,
	}
	md := metadata.Pairs("addr", n.IP+":"+n.GRPCport)
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	reply, errc := (*client).TaskDivide(ctx, msg) // 发起gRPC调用
	if errc != nil {
		logging.WriteLog("divide error is " + ip + " " + fmt.Sprintln(errc))
		t, ok := n.IdTaskMap.Get(t.Id) // 获取任务对象
		if ok {
			t.AddErrNum(ip, n.ErrorMsg) // 记录任务的出错次数
		}
		return
	}
	servermap.UpdateNbh(reply) // 更新某个邻域节点的信息，及其它的邻域的节点信息
}

// 转发gRPC实现, 本节点作为接收节点，接收来自于其他节点的任务
func (n *Node) TaskDivide(ctx context.Context, arg *nodegrpc.TaskDivideImf) (*nodegrpc.NodeInfo, error) {
	select {
	case <-ctx.Done():
		return nil, errors.New("TaskDivide task timeout")
	default:
	}
	t := task.NewTaskPointer(arg.BasicImf.Taskid, arg.BasicImf.Taskcpu, arg.BasicImf.Taskram, arg.Taskip, arg.BasicImf.Command, arg.BasicImf.Submittime, arg.BasicImf.PrenodeIp, arg.BasicImf.Maxhop) // 构造task.task对象
	go logging.NoteLogQueue(time.Now().Format(time.RFC3339Nano), config.DivideGet, t.Id, n.IP+":"+n.GRPCport+" get divide task from "+t.Addr, t.Cpu, t.Ram)
	n.Request <- t
	go n.timekeep(t) // 启动定时器，定时检查任务是否超时
	return n.GetInfo(), nil
}

// 本节点作为接收节点，在接收到任务后，每个任务会根据其等待时间启动这个计时协程(这里写死每个任务等待为20毫秒)
func (n *Node) timekeep(t *task.Task) {
	select {
	case <-time.After(20 * time.Millisecond): // 等待20毫秒
		t.WaitChan <- struct{}{} // 通知调度器，任务已经等待了20毫秒
		n.errorManage(t)
	case <-t.FinishChan: // 如果任务已经完成，就退出
		return
	}
}

// 本节点作为调度节点，返回给接收节点，确认任务的最终调度
func (n *Node) TaskConfirm(ctx context.Context, arg *nodegrpc.NodeIp) (*nodegrpc.Confirm, error) {
	select {
	case <-ctx.Done():
		t, ok := n.IdTaskMap.Get(arg.Taskid) // 获取任务对象
		go logging.NoteLogQueue(time.Now().Format(time.RFC3339Nano), config.DivideError, arg.Taskid, n.IP+":"+n.GRPCport+" receive divide Error from "+arg.Nodeip, 0.0, 0.0)
		if ok {
			t.AddErrNum(arg.Nodeip, n.ErrorMsg) // 记录任务的出错次数
		}
		return nil, errors.New("TaskConfirm task timeout")
	default:
	}
	t, ok := n.IdTaskMap.Get(arg.Taskid) // 获取任务对象
	go logging.NoteLogQueue(time.Now().Format(time.RFC3339Nano), config.DivideConfirm, arg.Taskid, n.IP+":"+n.GRPCport+" receive divide Confirm from "+arg.Nodeip, 0.0, 0.0)
	if ok {
		comfirm := t.Confirm(arg.Nodeip, n.ConfirMsg) // 确认任务的最终调度
		if comfirm {
			s := strings.Split(arg.Nodeip, ":")[0]
			go logging.NoteLogQueue(time.Now().Format(time.RFC3339Nano), config.DivideEnd, arg.Taskid, s, 0.0, 0.0)
		}
		return &nodegrpc.Confirm{Confirm: comfirm}, nil // 返回确认结果
	}
	return &nodegrpc.Confirm{Confirm: false}, nil // 返回确认结果
}

// 本节点作为调度节点，对接收节点的任务报错，记录相关任务的出错次数
func (n *Node) TaskError(ctx context.Context, arg *nodegrpc.NodeIp) (*nodegrpc.EmptyReply, error) {
	select {
	case <-ctx.Done():
		t, ok := n.IdTaskMap.Get(arg.Taskid) // 获取任务对象
		go logging.NoteLogQueue(time.Now().Format(time.RFC3339Nano), config.DivideError, arg.Taskid, n.IP+":"+n.GRPCport+" receive divide Error from "+arg.Nodeip, 0.0, 0.0)
		if ok {
			t.AddErrNum(arg.Nodeip, n.ErrorMsg) // 记录任务的出错次数
		}
		return nil, errors.New("TaskError task timeout")
	default:
	}
	t, ok := n.IdTaskMap.Get(arg.Taskid) // 获取任务对象
	go logging.NoteLogQueue(time.Now().Format(time.RFC3339Nano), config.DivideError, arg.Taskid, n.IP+":"+n.GRPCport+" receive divide Error from "+arg.Nodeip, 0.0, 0.0)
	if ok {
		t.AddErrNum(arg.Nodeip, n.ErrorMsg) // 记录任务的出错次数
	}
	return &nodegrpc.EmptyReply{}, nil
}

// etcd注册节点信息
func (n *Node) EtcdRegister() {
	// 注册节点信息
begin:
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{config.Etcd},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		logging.WriteLog("etcd register: " + fmt.Sprintln(err))
		goto begin
	}
	defer cli.Close()
	kv := clientv3.NewKV(cli)
	// 服务注册
	m := n.Res.GetAvailRes()
	key := fmt.Sprintf("/dcss/%s:%s", n.IP, n.GRPCport)                                               // key值为：/dcss/ip:grpcport
	value := fmt.Sprintf("%s:%s;%.2f;%.2f;%d;%d", n.IP, n.GRPCport, m[0], m[1], 0, time.Now().Unix()) // value值为：ip:grpcport;cpu;ram;queuenum;time
	//创建一个20S的租约
setlease:
	lease := clientv3.NewLease(cli)
	leaseGrantResp, err := lease.Grant(context.Background(), 20)
	if err != nil {
		logging.WriteLog("etcd setlease: " + fmt.Sprintln(err))
		goto setlease
	}
	//插入数据
setdata:
	leaseID := leaseGrantResp.ID
	_, err = kv.Put(context.Background(), key, value, clientv3.WithLease(leaseID))
	if err != nil {
		logging.WriteLog("etcd setdata: " + fmt.Sprintln(err))
		goto setdata
	}
	keepRespChan, _ := lease.KeepAlive(context.Background(), leaseID)
	for keepResp := range keepRespChan {
		if keepResp == nil {
			logging.WriteLog("租约失效")
			break
		}
	}
}

// -------------------------------------------------以下开始是本节点内部的任务管理流程-------------------------------------------------

// 任务重试协程，当任务出错次数超过阈值时，会调用重试
func (n *Node) errManage() {
	for id := range n.ErrorMsg {
		go func(zid string) {
			t, ok := n.IdTaskMap.Get(zid)
			if !ok {
				return
			}
			id, cpu, ram, command, submittime, prenode, maxhop := t.GetInfo()
			atask := task.NewTaskPointer(id, cpu, ram, n.IP+":"+n.GRPCport, command, submittime, prenode, maxhop)
			n.IdTaskMap.Remove(zid)
			n.PushTask(atask)
		}(id)
	}
}

// 任务确认协程，当任务调度成功时，会将任务从IdTask表内删除
func (n *Node) confirmManage() {
	for id := range n.ConfirMsg {
		n.IdTaskMap.Remove(id)
	}
}

// 调度任务获取检查
func (n *Node) schedManage() {
	for {
		var t *task.Task
		select {
		case t = <-n.Request: // 从请求队列中获取任务
		case t = <-n.Latebind: // 从延迟绑定队列中获取任务
		}
		select { // 选择是否超时
		case <-t.WaitChan: // 如果超时，则调用错误处理
		default: // 如果没有超时，则调度任务
			go n.sched(t)
		}
	}
}

// 错误处理
func (n *Node) errorManage(t *task.Task) {
errorM:
	client, err := servermap.GetPeerConn(t.Addr)
	if err != nil {
		logging.WriteLog("errorManage: " + fmt.Sprintln(err))
		goto errorM
	}
	go logging.NoteLogQueue(time.Now().Format(time.RFC3339Nano), config.StartDivideError, t.Id, n.IP+":"+n.GRPCport+" begin divde ErrorManage to "+t.Addr, t.Cpu, t.Ram)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, errc := (*client).TaskError(ctx, &nodegrpc.NodeIp{Taskid: t.Id, Nodeip: n.IP + ":" + n.GRPCport})
	if errc != nil {
		logging.WriteLog("ErrorAdd:" + t.Addr + " " + fmt.Sprintln(errc))
		return
	}
}

// 任务调度具体过程
func (n *Node) sched(t *task.Task) {
	err := n.Res.PreAlloc(t) // 预分配资源
	if err != nil {          // 如果预分配失败，则将任务放入延迟绑定队列
		n.Latebind <- t
		return
	}
	t.FinishChan <- struct{}{} //通知等待协程，该任务已经完成，不需要等待了

	ok := n.remote_confirm(t) // 向调度节点远程确认
	if ok {                   // 如果远程确认成功，则调用任务执行
		n.Res.RunTask(t) // 调用任务执行
	} else {
		n.Res.CancelAlloc(t) // 如果远程确认失败，则取消预分配，释放资源
	}
}

// 远程确认函数
func (n *Node) remote_confirm(t *task.Task) bool {
remote_con:
	client, err := servermap.GetPeerConn(t.Addr)
	if err != nil {
		logging.WriteLog("remote_confirm: " + fmt.Sprintln(err))
		goto remote_con
	}
	go logging.NoteLogQueue(time.Now().Format(time.RFC3339Nano), config.StartDivideConfirm, t.Id, n.IP+":"+n.GRPCport+" begin divde confirm to "+t.Addr, t.Cpu, t.Ram)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	confirm, errc := (*client).TaskConfirm(ctx, &nodegrpc.NodeIp{Taskid: t.Id, Nodeip: n.IP + ":" + n.GRPCport})
	if errc != nil {
		logging.WriteLog("remote_confirm:" + t.Addr + " " + fmt.Sprintln(errc))
		return false
	}
	return confirm.Confirm
}

/*
// 任务加入到PriorityQ
func (n *Node) PushTask(t *task.Task) {
	time := time.Now().UnixMilli()
	if time-t.Submittime < 10 {
		// 表示有充足的时间处理，可以放在低优先级
		n.Mytask.Push(t, 2)
	} else if time-t.Submittime <= 60 {
		// 表示时间耗费了一半，可以放在中优先级
		n.Mytask.Push(t, 1)
	} else {
		// 表示很紧急，要放在高优先级
		n.Mytask.Push(t, 0)
	}
}
// 任务根据优先级加入到Mytask
func (n *Node) PushTaskByPriority(t *task.Task, pri int) {
	n.Mytask.Push(t, pri)
}
*/

func (n *Node) PushTask(t *task.Task) {
	n.Mytask.Push(t)
}

// 获取节点信息
func (n *Node) GetInfo() *nodegrpc.NodeInfo {
	res := n.Res.GetAvailRes()                            // 获取本节点的剩余资源
	nbhs := servermap.GetNbh()                            // 获取本节点的邻域节点
	score := public.ScoreNeighbours(nbhs, res[0], res[1]) // 计算邻域节点的分数
	nodeinfor := &nodegrpc.NodeInfo{                      // 构造本节点的信息，放在回传数组的第一个位置，表示是邻域节点信息，后续的成员是两跳的邻域节点信息
		Addr:      n.IP + ":" + n.GRPCport,
		Cpu:       res[0],
		Ram:       res[1],
		Queuenum:  int32(len(n.Request) + len(n.Latebind) + n.Mytask.Len()),
		Timestamp: time.Now().Unix(),
		Nbhscore:  score,
	}
	return nodeinfor
}

func (n *Node) Run() {
	go n.schedManage()
	go n.errManage()
	go n.confirmManage()
}
