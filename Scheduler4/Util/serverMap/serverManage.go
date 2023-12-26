/*
- @Autor: XLF
- @Date: 2022-11-25 13:27:54

  - @LastEditors: XLF
  - @LastEditTime: 2023-06-25 16:18:06
  - @Description:
    邻域节点管理模块，通过对节点列表信息的处理，判断邻域是否要更新，以及供node模块的转发功能调用
*/
package servermap

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	config "scheduler4/Config"
	"scheduler4/Util/node/task"
	"time"

	gcsproto "gitee.com/linfeng-xu/protofile/gcsGrpc"
	nodegrpc "gitee.com/linfeng-xu/protofile/nodeGrpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var serverm ServerMap
var peerm PeerMap

func init() {
	serverm = NewServerMap()
	peerm = NewPeerMap()
}

func Run(addr string) {
	// 1. 从GCS中读取所有节点信息
	conn, err := grpc.Dial(config.Gcsip, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	client := gcsproto.NewGcsGrpcClient(conn)
begin:
	reply, err := client.GetNbh(context.Background(), &gcsproto.InitReq{Addr: addr, Num: int32(config.Nums)})
	if err != nil {
		log.Println(err)
		time.Sleep(3 * time.Second)
		goto begin
	}
	m := make(map[string]*NbhInfo)
	for i, v := range reply.Nbh {
		fmt.Println(i, v)
		if i == 0 || i == 1 {
			fmt.Println("兄弟节点：", v.Addr)
			m[v.Addr] = NewOnlyNbh(v.Addr, v.Cpu, v.Ram, int(v.Queuenum), v.Timestamp, -1, 1)
			continue
		}
		m[v.Addr] = NewOnlyNbh(v.Addr, v.Cpu, v.Ram, int(v.Queuenum), v.Timestamp, -1, 0) //addr string, cpu float32, ram float32, queuenum int, visitnum int, timestamp int64, score float32
	}
	serverm.LocalAddr = addr
	// 2. 将节点信息存入serverm中
	serverm.UpdateServerByMap(m)
	//go updateonly()
	// 3. 启动定时任务，定时更新serverm
	//go update30s()
}

func randint(len int) (int, int) {
	i, j := rand.Intn(len), rand.Intn(len)
	for i == j {
		j = rand.Intn(len)
	}
	return i, j
}

func Choose(t *task.Task) []string {
	lists := serverm.GetServerList()
	// 从数组中随机选取两个节点
	i, j := randint(len(lists))
	return []string{lists[i].Addr, lists[j].Addr}
}

func Choose111(v ...string) string {
	lists := serverm.GetServerList()
	if len(v) == 0 {
		// 从数组中随机选取1个节点
		i := rand.Intn(len(lists))
		return lists[i].Addr
	} else {
		i := rand.Intn(len(lists))
		for lists[i].Addr == v[0] {
			i = rand.Intn(len(lists))
		}
		return lists[i].Addr
	}
}

// 找到资源剩余最多的节点
func ChooseBest(t *task.Task) string {
	lists := serverm.GetServerList()
	index := 0
	has := -1
	len := len(lists)
	// 找到第一个资源足够的节点
	for i, v := range lists {
		if v.Addr == t.Prenode {
			has = i
			continue
		}
		if v.Cpu >= t.Cpu && v.Ram >= t.Ram {
			index = i
			break
		}
	}
	if index >= len { // 始终没有找到资源足够的节点
		i := rand.Intn(len)
		for ; i == has; i = rand.Intn(len) {
		}
		index = i
	}
	return lists[index].Addr
}

// 更新邻域节点信息，根据转发后的回调信息更新信息
func UpdateNbh(nodes *nodegrpc.NodeInfo) {
	serverm.UpdateServer(nodes)
}

// 获取邻域节点信息
func GetNbh() []*nodegrpc.NodeInfo {
	lists := serverm.GetServerList()
	return lists
}

// 邻域更新策略，根据邻域状态，对邻域做出更新决策，返回一个bool值，true表示更新，false表示不更新
func Nbh_policy() bool {
	neighbors := GetNbh()
	// 计算cpus和rams的平均值
	var cpu_avg float32 = 0
	var ram_avg float32 = 0
	for _, v := range neighbors {
		cpu_avg += v.Cpu
		ram_avg += v.Ram
	}
	cpu_avg /= float32(len(neighbors))
	ram_avg /= float32(len(neighbors))
	if cpu_avg < 2.0 || ram_avg < 2.0 {
		return true
	}
	return false
}

func update30s() {
	timer := time.NewTimer(5 * time.Second)
	for {
		<-timer.C
		if Nbh_policy() {
			serverm.closeAllConn()
			serverm.UpdateNum++
			if serverm.UpdateNum == 20 {
				serverm.refreshMap()
			} else {
				serverm.UpdateNbh()
			}
		}
		timer.Reset(5 * time.Second)
	}
}

func GetConnByAddr(addr string) (*nodegrpc.NodeServerClient, error) {
	return serverm.getConnByAddr(addr)
}

func Updateonly() {
	timer := time.NewTimer(5 * time.Second)
	for {
		<-timer.C
		//serverm.Updateonly()
		serverm.ChangeUpdateOnly()
		timer.Reset(10 * time.Second)
	}
}

func AddConns() {
	serverm.AddConns()
}

/*-----peerconn-----*/
func TryAddPeer(addr string) bool {
	return peerm.TryAddPeer(addr)
}

func AddPeerConn(addr string) {
	peerm.AddConn(addr)
}

func GetPeerConn(addr string) (*nodegrpc.NodeServerClient, error) {
	c, err := serverm.getConnByAddr(addr)
	if err == nil {
		return c, nil
	}
	return peerm.GetConn(addr)
}

func HasPeer(addr string) bool {
	return serverm.ConnList.Has(addr)
}
