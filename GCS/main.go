/*
 * @Autor: XLF
 * @Date: 2022-12-05 11:10:43
 * @LastEditors: XLF
 * @LastEditTime: 2023-09-05 11:26:42
 * @Description:
 */
package main

import (
	"context"
	"errors"
	"fmt"
	"gcs/slicemap"
	"log"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	gcsProto "gitee.com/linfeng-xu/protofile/gcsGrpc"
	nodeGrpc "gitee.com/linfeng-xu/protofile/nodeGrpc"
	"github.com/spf13/pflag"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// 全局节点存储map
var allnodes *slicemap.SliceMap

var (
	Etcd    string
	nodenum int
)

func init() {
	pflag.StringVarP(&Etcd, "etcd", "e", "121.40.99.204:31225", "输入etcd地址，格式为ip:port，使用方法-e ip:port")
	pflag.IntVarP(&nodenum, "nodenum", "n", 0, "输入节点总数量，使用方法-n num")
	pflag.Parse()
	allnodes = slicemap.NewSliceMap()
}

// 节点信息
func Newnodeinfo(addr string, cpu float32, ram float32, queuenum int32, timestamp int64) *gcsProto.NbhInfo {
	return &gcsProto.NbhInfo{
		Addr:      addr,
		Cpu:       cpu,
		Ram:       ram,
		Queuenum:  queuenum,
		Timestamp: timestamp,
	}
}

// 解码器，从etcd获得一条记录，将记录内容解码为对应的类型
func decodeNodeInfo(s string) (string, float32, float32, int32, int64) {
	var (
		addr string
		time int64
	)
	// 从s中根据;分割字符串
	ss := strings.Split(s, ";")
	// 将分割后的字符串转换为对应的类型
	addr = ss[0]
	cpu, _ := strconv.ParseFloat(ss[1], 32)
	ram, _ := strconv.ParseFloat(ss[2], 32)
	queuenum, _ := strconv.Atoi(ss[3])
	time, _ = strconv.ParseInt(ss[4], 10, 64)
	return addr, float32(cpu), float32(ram), int32(queuenum), time
}

// etcd监听函数，监听etcd中/dcss/下的所有节点信息
func etcdWatch() {
connect:
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{Etcd},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		// handle error!
		goto connect
	}
	defer cli.Close()
	getResp, _ := cli.Get(context.Background(), "/dcss", clientv3.WithPrefix())
	for _, ve := range getResp.Kvs {
		node := Newnodeinfo(decodeNodeInfo(string(ve.Value)))
		allnodes.Add(node)
	}
	watchStartRevision := getResp.Header.Revision + 1
	watcher := clientv3.NewWatcher(cli)
	watchChan := watcher.Watch(context.Background(), "/dcss", clientv3.WithPrefix(), clientv3.WithRev(watchStartRevision))
	fmt.Println("watch start at revision:", watchStartRevision, "now we have", allnodes.Len(), "nodes")
	for watchResp := range watchChan {
		for _, event := range watchResp.Events {
			switch event.Type {
			case mvccpb.PUT: // 插入或者更新
				node := Newnodeinfo(decodeNodeInfo(string(event.Kv.Value)))
				allnodes.Add(node)
			case mvccpb.DELETE: // 删除
				fmt.Println("delete", string(event.Kv.Key))
				allnodes.Del(string(event.Kv.Key))
			}
		}
	}
}

/******************************************** grpc函数实现 ********************************************/
func serverInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	if info.FullMethod == "/gcsServer.GcsGrpc/GetNbh" {
		if allnodes.Len() != nodenum {
			return nil, errors.New("node number not match")
		}
	}
	// Pre-processing logic
	resp, err := handler(ctx, req)
	// Post-processing logic
	return resp, err
}

type gcss struct {
	nstate int32
}

// 获取邻域节点函数，从通道内获取，所以具有一定延后性
func (g *gcss) GetNbh(ctx context.Context, args *gcsProto.InitReq) (*gcsProto.Nbhnodes, error) {
	if allnodes.Len() == 0 {
		return nil, errors.New("no nodes")
	}
	nodes := allnodes.GetSlice(args.Addr, int(args.Num))
	return &gcsProto.Nbhnodes{Nbh: nodes}, nil
}

// 获取所有节点函数，直接从map中读取，所以没有延后性
func (g *gcss) GetAllNodes(ctx context.Context, args *gcsProto.EmptyReq) (*gcsProto.AllNodes, error) {
	s := atomic.LoadInt32(&g.nstate)
	if s != 1 {
		return nil, errors.New("nodes not ready")
	}
	nodes := allnodes.GetAll()
	return &gcsProto.AllNodes{Nbh: nodes}, nil
}

// 获取所有节点的地址函数，直接从map中读取，所以没有延后性
func (g *gcss) GetOnlyAddr(ctx context.Context, args *gcsProto.EmptyReq) (*gcsProto.OnlyAddr, error) {
	s := atomic.LoadInt32(&g.nstate)
	if s != 1 {
		return nil, errors.New("nodes not ready")
	}
	nodes := allnodes.GetAll()
	addrs := make([]string, len(nodes))
	for i, v := range nodes {
		addrs[i] = v.Addr
	}
	return &gcsProto.OnlyAddr{Addr: addrs}, nil
}

func GetOutBoundIP() string {
	conn, err := net.Dial("udp", "192.0.0.0:53")
	if err != nil {
		log.Fatal(err)
		return ""
	}
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	ip := strings.Split(localAddr.String(), ":")[0]
	//ip = "127.0.0.1" //因为是在本机测试，所以这里只能用回环地址
	return ip
}

func (g *gcss) CheckState() {
	timer := time.NewTimer(30 * time.Second)
	for {
		<-timer.C
		l := allnodes.Len()
		if l == nodenum {
			break
		}
		timer.Reset(30 * time.Second)
	}
	timer.Reset(30 * time.Second)
	nodes := allnodes.GetAll()
	addrs := make(map[string]*grpc.ClientConn)
	for _, v := range nodes {
		conn, _ := grpc.Dial(v.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
		addrs[v.Addr] = conn
	}

	time.Sleep(15 * time.Second)
	for {
		for k, v := range addrs {
			client := nodeGrpc.NewNodeServerClient(v)
			reply, err := client.GetState(context.Background(), &nodeGrpc.EmptyReply{})
			if err != nil {
				continue
			}
			if reply.State == 1 {
				v.Close()
				delete(addrs, k)
			}
		}
		if len(addrs) == 0 {
			atomic.StoreInt32(&g.nstate, 1)
			break
		}
		<-timer.C
		timer.Reset(30 * time.Second)
	}
}

func main() {
	go etcdWatch()
	gServer := grpc.NewServer(
		grpc.UnaryInterceptor(serverInterceptor),
	)
	gcs := &gcss{nstate: 0}
	go gcs.CheckState()
	gcsProto.RegisterGcsGrpcServer(gServer, gcs)
	ip := GetOutBoundIP()
	lis, _ := net.Listen("tcp", ip+":23511")
	gServer.Serve(lis)
}
