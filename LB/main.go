/*
 * @Autor: XLF
 * @Date: 2022-12-07 10:06:08
 * @LastEditors: XLF
 * @LastEditTime: 2023-09-06 11:40:48
 * @Description:
 */
package main

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"time"

	logging "loadbal/logging"

	gcsproto "gitee.com/linfeng-xu/protofile/gcsGrpc"
	lbproto "gitee.com/linfeng-xu/protofile/lbGrpc"
	nodeimf "gitee.com/linfeng-xu/protofile/nodeGrpc"
	"github.com/spf13/pflag"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Nodelist interface {
	addNode(addr string)
	SendTask(t *nodeimf.TaskBasicImf)
	refreshlist()
	closeAllConn()
}

var nodel Nodelist
var mode int
var gcsAddr string

func init() {
	pflag.IntVarP(&mode, "mode", "m", 0, "0:单任务模式，1:流式任务模式，默认是0单任务模式")
	pflag.StringVarP(&gcsAddr, "gcsAddr", "g", "127.0.0.1:23512", "the gcsAddr")
	pflag.Parse()
	switch mode {
	case 0:
		nodel = NewUnaryNodelist()
	case 1:
		nodel = NewStreamNodelist()
	}
}

/*************************************一元调用*************************************/
type UnaryNode struct {
	addr   string
	conn   *grpc.ClientConn
	client *nodeimf.NodeServerClient
}

func (n *UnaryNode) SendTask(t *nodeimf.TaskBasicImf) {
Dispense:
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	_, err := (*n.client).TaskDispense(ctx, t)
	if err != nil {
		cancel()
		goto Dispense
	}
	cancel()
}

type UnaryNodelist struct {
	node []*UnaryNode
	// index int   //当前节点索引
	lenth int //当前节点列表长度
}

func NewUnaryNodelist() *UnaryNodelist {
	return &UnaryNodelist{node: make([]*UnaryNode, 0), lenth: 0}
}

func (n *UnaryNodelist) addNode(addr string) {
	conn, _ := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	c := nodeimf.NewNodeServerClient(conn)
	n.node = append(n.node, &UnaryNode{addr: addr, conn: conn, client: &c})
}

func (n *UnaryNodelist) SendTask(t *nodeimf.TaskBasicImf) {
	nm := n.node[rand.Intn(n.lenth)]
	nm.SendTask(t)
}

func (n *UnaryNodelist) refreshlist() {
	conn, err := grpc.Dial(gcsAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	client := gcsproto.NewGcsGrpcClient(conn)
getaddr:
	reply, err := client.GetOnlyAddr(context.Background(), &gcsproto.EmptyReq{})
	if err != nil {
		fmt.Println(err)
		time.Sleep(time.Second * 5)
		goto getaddr
	}

	for _, v := range reply.Addr {
		n.addNode(v)
	}
	n.lenth = len(n.node)
}

func (n *UnaryNodelist) closeAllConn() {
	for _, v := range n.node {
		v.conn.Close()
	}
}

// 调用refreshlist定时更新节点列表，每隔一小时更新一次
// func timedrefresh() {
// 	for {
// 		time.Sleep(time.Hour * 1)
// 		nodel.closeAllConn()
// 		nodel.refreshlist()
// 	}
// }

/*************************************流式调用*************************************/
type StreamNode struct {
	addr     string
	conn     *grpc.ClientConn
	client   *nodeimf.NodeServerClient
	stream   nodeimf.NodeServer_TaskDispenseByStreamClient
	waitchan chan *nodeimf.TaskBasicImf
}

func (n *StreamNode) SendTask(t *nodeimf.TaskBasicImf) {
	n.waitchan <- t
}
func (n *StreamNode) Run() {
	for t := range n.waitchan {
		n.stream.Send(t)
	}
}

type StreamNodelist struct {
	node  []*StreamNode
	lenth int //当前节点列表长度
}

func NewStreamNodelist() *StreamNodelist {
	return &StreamNodelist{node: make([]*StreamNode, 0), lenth: 0}
}

func (n *StreamNodelist) addNode(addr string) {
	conn, _ := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	c := nodeimf.NewNodeServerClient(conn)
	stream, _ := c.TaskDispenseByStream(context.Background())
	strnode := &StreamNode{addr: addr, conn: conn, client: &c, stream: stream, waitchan: make(chan *nodeimf.TaskBasicImf, 100)}
	n.node = append(n.node, strnode)
	go strnode.Run()
}

func (n *StreamNodelist) SendTask(t *nodeimf.TaskBasicImf) {
	nm := n.node[rand.Intn(n.lenth)]
	nm.SendTask(t)
}

func (n *StreamNodelist) refreshlist() {
	conn, err := grpc.Dial(gcsAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	client := gcsproto.NewGcsGrpcClient(conn)
getaddr:
	reply, err := client.GetOnlyAddr(context.Background(), &gcsproto.EmptyReq{})
	if err != nil {
		fmt.Println(err)
		time.Sleep(time.Second * 5)
		goto getaddr
	}

	for _, v := range reply.Addr {
		n.addNode(v)
	}
	n.lenth = len(n.node)
}

func (n *StreamNodelist) closeAllConn() {
	for _, v := range n.node {
		v.conn.Close()
	}
}

/*************************************grpc server*************************************/
type taskin struct {
}

func (t *taskin) HelloLb(ctx context.Context, in *lbproto.EmptyReq) (*lbproto.EmptyReq, error) {
	return &lbproto.EmptyReq{}, nil
}

// 对外接口，供外部调用，接收任务
func (t *taskin) GetTask(ctx context.Context, in *lbproto.TaskBasicImf) (*lbproto.EmptyReq, error) {
	go grpcDispense(in.Taskid, in.Taskcpu, in.Taskram, in.Command)
	go logging.NoteLogQueue(time.Now().Format(time.RFC3339Nano), in.Taskid, "get", in.Taskcpu, in.Taskram)
	return &lbproto.EmptyReq{}, nil
}

func (t *taskin) GetTaskByStream(stream lbproto.LbServer_GetTaskByStreamServer) error {
	for {
		t, err := stream.Recv()
		switch err {
		case nil:
			go grpcDispense(t.Taskid, t.Taskcpu, t.Taskram, t.Command)
			go logging.NoteLogQueue(time.Now().Format(time.RFC3339Nano), t.Taskid, "get", t.Taskcpu, t.Taskram)
		case io.EOF:
			return stream.SendAndClose(&lbproto.EmptyReq{})
		default:
			return err
		}
	}
}

// 向节点发送任务
func grpcDispense(taskid string, taskcpu float32, taskram float32, command string) {
	//client := nodeimf.NewNodeServerClient(nodec)
	task := nodeimf.TaskBasicImf{Taskid: taskid, Taskcpu: taskcpu, Taskram: taskram, Command: command, Submittime: time.Now().UnixMilli(), PrenodeIp: "Home", Maxhop: 10} //构造任务
	nodel.SendTask(&task)                                                                                                                                                 //随机获取一个节点的连接
}

func run() {
	nodel.refreshlist()
	// go timedrefresh()
	gServer := grpc.NewServer()
	lbproto.RegisterLbServerServer(gServer, &taskin{})
	lis, _ := net.Listen("tcp", "0.0.0.0:23512")
	gServer.Serve(lis)
}

func main() {
	go logging.WriteLogGo()
	run()
}
