/*
 * @Autor: XLF
 * @Date: 2023-01-26 18:39:03
 * @LastEditors: XLF
 * @LastEditTime: 2023-06-25 14:58:56
 * @Description:
 */
package node

import (
	"fmt"
	"net"
	config "scheduler4/Config"
	"scheduler4/Util/logging"
	task "scheduler4/Util/node/task"
	servermap "scheduler4/Util/serverMap"
	"sync"
	"sync/atomic"
	"time"

	nodegrpc "gitee.com/linfeng-xu/protofile/nodeGrpc"

	"google.golang.org/grpc"
)

var n *Node

func InitNode(cpu, ram float32, wg *sync.WaitGroup) {
newserver:
	n = NewNodePointer(cpu, ram)
	gServer := grpc.NewServer(
		grpc.UnaryInterceptor(serverInterceptor),
	)
	nodegrpc.RegisterNodeServerServer(gServer, n)
	fmt.Println(n.IP + ":" + n.GRPCport)
	lis, err := net.Listen("tcp", n.IP+":"+n.GRPCport)
	if err != nil {
		goto newserver
	}
	go gServer.Serve(lis)                  //开始服务
	go n.EtcdRegister()                    // 启动成功，向ETCD注册自身节点信息
	time.Sleep(15 * time.Second)           //等待其他节点注册
	servermap.Run(n.IP + ":" + n.GRPCport) //向GCS获取初始化邻域
	time.Sleep(3 * time.Second)
	servermap.AddConns() //建立邻域连接
	fmt.Println(time.Now().Format(time.RFC3339Nano), "I am ready")
	n.Run()
	go servermap.Updateonly() //开始更新邻域信息
	atomic.StoreInt32(&n.State, 1)
	wg.Done()
}

// 获取本地节点的状态
func GetState() [2]float32 {
	return n.Res.GetAvailRes()
}

// 由agent转发任务逻辑调用
func DivideTask(t *task.Task, addr []string) {
	for _, s := range addr {
		go logging.NoteLogQueue(time.Now().Format(time.RFC3339Nano), config.StartDivide, t.Id, n.IP+":"+n.GRPCport+" startdivide to "+s, t.Cpu, t.Ram)
	}
	n.dividing(t, addr)
}

// 多跳任务进行
func Wander(t *task.Task, addr []string) {
	go logging.NoteLogQueue(time.Now().Format(time.RFC3339Nano), config.StartWander, t.Id, n.IP+":"+n.GRPCport+" wander to "+addr[0], t.Cpu, t.Ram)
	n.nextwander(t, addr)
}

/*
// 获取Dispense任务队列中的任务

	func GetDispenseChan() ([]*task.Task, []int) {
		return n.Mytask.Pop(10)
	}
*/
func GetDispenseChan() ([]*task.Task, int) {
	return n.Mytask.Pop(10)
}

func InsertTask(t *task.Task) {
	// 将任务插入Dispense任务队列
	n.PushTask(t)
}

/*
func InsertTaskByPri(t *task.Task, pri int) {
	// 将任务插入Dispense任务队列
	n.PushTaskByPriority(t, pri)
}
*/
// 由agent算法认为可在本地运行的任务进行调用
func RunTask(t *task.Task) error {
	//预留资源申请
	err := n.Res.PreAlloc(t)
	if err != nil {
		//如果资源不足，返回错误，agent需要处理这个错误，默认将该任务重新加入到dispense队列，等待重新调度
		return err
	} else {
		//预留资源成功，那么就将任务加入到Reservation队列中，等待执行
		n.Res.RunTask(t) //这里不需要在加入到队列中
		return nil
	}
}
