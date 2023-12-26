/*
- @Autor: XLF
- @Date: 2022-11-16 10:48:49

 * @LastEditors: XLF
 * @LastEditTime: 2023-06-25 15:41:59
  - @Description:
    存储节点列表，提供基础操作，如增删改查
*/

package servermap

import (
	"context"
	"errors"
	"fmt"
	config "scheduler4/Config"
	"scheduler4/Util/logging"
	"sync"

	gcsproto "gitee.com/linfeng-xu/protofile/gcsGrpc"
	nodegrpc "gitee.com/linfeng-xu/protofile/nodeGrpc"
	cmap "github.com/orcaman/concurrent-map/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

/*------------------邻域节点------------------*/

// 邻域节点的信息
type NbhInfo struct {
	// 成员锁
	mu sync.RWMutex
	// 单节点信息
	*nodegrpc.NodeInfo
	// 该成员的HTT2通道
	// conn *grpc.ClientConn
	// 该成员的grpc的client对象
	// client *nodegrpc.NodeServerClient
	// 特权位，表示这节点是不是基础环节点
	priority int
}

// 仅新建节点信息
func NewOnlyNbh(addr string, cpu float32, ram float32, queuenum int, timestamp int64, score float32, pri int) *NbhInfo {
	return &NbhInfo{
		mu: sync.RWMutex{},
		NodeInfo: &nodegrpc.NodeInfo{
			Addr:      addr,
			Cpu:       cpu,
			Ram:       ram,
			Queuenum:  int32(queuenum),
			Timestamp: timestamp,
			Nbhscore:  score,
		},
		priority: pri,
	}
}

// 新建节点信息，用于node-GCS初始化
func NewNbhInfo(addr string, cpu float32, ram float32, queuenum int, timestamp int64, score float32, pri int) *NbhInfo {
	// co, _ := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	// c := nodegrpc.NewNodeServerClient(co)
	return &NbhInfo{
		mu: sync.RWMutex{},
		NodeInfo: &nodegrpc.NodeInfo{
			Addr:      addr,
			Cpu:       cpu,
			Ram:       ram,
			Queuenum:  int32(queuenum),
			Timestamp: timestamp,
			Nbhscore:  score,
		},
		// coon:     co,
		// client:   &c,
		priority: pri,
	}
}

func (n *NbhInfo) GetLocal() *nodegrpc.NodeInfo {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.NodeInfo
}

// func (n *NbhInfo) CloseConn() {
// 	n.mu.Lock()
// 	n.coon.Close()
// 	defer n.mu.Unlock()
// }

// 用于节点转发反馈信息的更新
func (n *NbhInfo) Update(nodes *nodegrpc.NodeInfo) {
	n.mu.Lock()
	n.NodeInfo = nodes
	defer n.mu.Unlock()
}

/*---------------连接成员----------------*/
type ConnInfo struct {
	mu sync.RWMutex
	// 该成员的HTT2通道
	conn *grpc.ClientConn
	// 该成员的grpc的client对象
	client *nodegrpc.NodeServerClient
}

func NewConnInfo(addr string) *ConnInfo {
	co, _ := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	c := nodegrpc.NewNodeServerClient(co)
	return &ConnInfo{
		mu:     sync.RWMutex{},
		conn:   co,
		client: &c,
	}
}

func (c *ConnInfo) GetClient() *nodegrpc.NodeServerClient {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client
}

func (c *ConnInfo) SetClient(addr string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.conn.Close()
	co, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return err
	}
	client0 := nodegrpc.NewNodeServerClient(co)
	c.conn = co
	c.client = &client0
	return nil
}

func (c *ConnInfo) CloseConn() {
	c.mu.Lock()
	c.conn.Close()
	defer c.mu.Unlock()
}

/*------------------邻域节点列表------------------*/
// 邻域节点列表，
type ServerMap struct {
	// 节点列表
	ServerList []cmap.ConcurrentMap[string, *NbhInfo]
	// 连接列表
	ConnList cmap.ConcurrentMap[string, *ConnInfo]
	// 本地节点的地址
	LocalAddr string
	// 正在使用的序列号
	index int
	// 更新次数
	UpdateNum int
}

// 新建列表
func NewServerMap() ServerMap {
	z := make([]cmap.ConcurrentMap[string, *NbhInfo], 2)
	z[0] = cmap.New[*NbhInfo]()
	z[1] = cmap.New[*NbhInfo]()
	return ServerMap{
		ServerList: z,
		ConnList:   cmap.New[*ConnInfo](),
		index:      0,
		UpdateNum:  0,
	}
}

// 添加单个节点
func (s *ServerMap) AddServer(node *NbhInfo) {
	if node.Addr == s.LocalAddr {
		return
	}
	s.ServerList[s.index].Set(node.Addr, node)
}

// 批量添加节点
func (s *ServerMap) AddServerByList(m map[string]*NbhInfo) {
	s.ServerList[s.index].MSet(m)
	s.ServerList[s.index].Remove(s.LocalAddr)
}

// 删除单个节点
func (s *ServerMap) DelServer(addr string) {
	s.ServerList[s.index].Remove(addr)
}

// 更新单个节点
func (s *ServerMap) UpdateServer(nodes *nodegrpc.NodeInfo) {
	if nodes.Addr == s.LocalAddr {
		s.ServerList[s.index].Remove(s.LocalAddr)
		return
	} else {
		node, ok := s.ServerList[s.index].Get(nodes.Addr)
		if ok {
			node.Update(nodes)
		}
	}
}

// 返回节点列表
func (s *ServerMap) GetServerList() []*nodegrpc.NodeInfo {
	m := s.ServerList[s.index].Items()
	list := make([]*nodegrpc.NodeInfo, len(m))
	index := 0
	for _, v := range m {
		k := v.GetLocal()
		list[index] = &nodegrpc.NodeInfo{
			Addr:      k.Addr,
			Cpu:       k.Cpu,
			Ram:       k.Ram,
			Queuenum:  k.Queuenum,
			Timestamp: k.Timestamp,
			Nbhscore:  k.Nbhscore,
		}
		index++
	}
	return list
}

// 批量更新节点byMap
func (s *ServerMap) UpdateServerByMap(m map[string]*NbhInfo) {
	s.ServerList[1-s.index].Clear()
	s.ServerList[1-s.index].MSet(m)
	s.ServerList[1-s.index].Remove(s.LocalAddr)
	s.index = 1 - s.index
}

// 批量更新节点bySlice
func (s *ServerMap) UpdateServerBySlice(m []*NbhInfo) {
	s.ServerList[1-s.index].Clear()
	for _, v := range m {
		s.ServerList[1-s.index].Set(v.Addr, v)
	}
	s.ServerList[1-s.index].Remove(s.LocalAddr)
	s.index = 1 - s.index
}

func (s *ServerMap) UpdateNbh() {
	items := s.ServerList[s.index].Items()
	allnodes := make([]*nodegrpc.NodeInfo, 0)
	brothers := make([]*nodegrpc.NodeInfo, 0)
	for _, v := range items {
		conn, _ := s.ConnList.Get(v.Addr)
		client := *conn.GetClient()
		reply, err := client.HelloNode(context.Background(), &nodegrpc.EmptyReply{})
		if err != nil {
			if v.priority == 1 {
				// 如果出现错误的是基础节点，则需要向GCS要新的map，重置邻域节点表
				s.refreshMap()
				return
			} else {
				continue
			}
		}
		if v.priority == 1 {
			brothers = append(brothers, reply.Nodes[0])
		}
		allnodes = append(allnodes, reply.Nodes...)
	}

	m := unique(allnodes)
	maxlen := len(m)
	if maxlen > config.Nums {
		for i := 0; i < len(m); i++ {
			for j := i + 1; j < len(m); j++ {
				if m[i].Cpu+m[i].Ram < m[j].Cpu+m[j].Ram {
					m[i], m[j] = m[j], m[i]
				}
			}
		}
		m = m[:config.Nums]
	} else {
		m = m[:maxlen]
	}

	n := make(map[string]*NbhInfo)

	for _, v := range m {
		if _, ok := n[v.Addr]; ok {
			continue
		}
		n[v.Addr] = NewNbhInfo(v.Addr, v.Cpu, v.Ram, int(v.Queuenum), v.Timestamp, v.Nbhscore, 0)
	}

	for _, v := range brothers {
		if value, ok := n[v.Addr]; ok {
			value.priority = 1
		} else {
			n[v.Addr] = NewNbhInfo(v.Addr, v.Cpu, v.Ram, int(v.Queuenum), v.Timestamp, v.Nbhscore, 1)
		}
	}
	s.UpdateServerByMap(n)
}

func unique(m []*nodegrpc.NodeInfo) []*nodegrpc.NodeInfo {
	maps := make(map[string]*nodegrpc.NodeInfo)
	for _, v := range m {
		if v.Addr == serverm.LocalAddr {
			continue
		}
		if _, ok := maps[v.Addr]; !ok {
			maps[v.Addr] = v
		} else {
			if maps[v.Addr].Timestamp < v.Timestamp {
				maps[v.Addr] = v
			}
		}
	}
	res := make([]*nodegrpc.NodeInfo, 0)
	for _, v := range maps {
		res = append(res, v)
	}
	return res
}

// 将之前使用的Map所有成员通道关闭
func (s *ServerMap) closeAllConn() {
	items := s.ConnList.Items()
	for _, v := range items {
		v.CloseConn()
	}
}

// 根据Addr获取conn
func (s *ServerMap) getConnByAddr(addr string) (*nodegrpc.NodeServerClient, error) {
	node, ok := s.ConnList.Get(addr)
	if !ok {
		logging.WriteLog("取通道错误")
		return nil, errors.New("get conn err")
	}
	return node.client, nil
}

// 重置map，当出现基础节点失败，或者达到一定数量的小更新后
func (s *ServerMap) refreshMap() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("refreshMap", err)
		}
	}()
	conn, err := grpc.Dial(config.Gcsip, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	client := gcsproto.NewGcsGrpcClient(conn)
	reply, err := client.GetNbh(context.Background(), &gcsproto.InitReq{Addr: s.LocalAddr, Num: int32(config.Nums)})
	if err != nil {
		panic(err)
	}
	m := reply.Nbh
	n := make(map[string]*NbhInfo)
	for index, v := range m {
		if index == 0 || index == 1 {
			n[v.Addr] = NewNbhInfo(v.Addr, v.Cpu, v.Ram, int(v.Queuenum), v.Timestamp, -1, 1)
			continue
		}
		n[v.Addr] = NewNbhInfo(v.Addr, v.Cpu, v.Ram, int(v.Queuenum), v.Timestamp, -1, 0)
	}
	s.UpdateNum = 0
	s.UpdateServerByMap(n)
}

func (s *ServerMap) Updateonly() {
	items := s.ServerList[s.index].Items()
	for _, v := range items {
		conn, _ := s.ConnList.Get(v.Addr)
		client := *conn.GetClient()
		reply, err := client.HelloNode(context.Background(), &nodegrpc.EmptyReply{})
		if err != nil {
			// 如果出现错误的是基础节点，则需要向GCS要新的map，重置邻域节点表
			continue
		}
		v.Update(reply.Nodes[0])
	}
}

func (s *ServerMap) ChangeUpdateOnly() {
	items := s.ServerList[s.index].Items()
	m := make(map[string]*NbhInfo)
	for _, v := range items {
		conn, _ := s.ConnList.Get(v.Addr)
		client := *conn.GetClient()
		reply, err := client.HelloNode(context.Background(), &nodegrpc.EmptyReply{})
		if err != nil {
			m[v.Addr] = v
			continue
		}
		m[v.Addr] = NewNbhInfo(reply.Nodes[0].Addr, reply.Nodes[0].Cpu, reply.Nodes[0].Ram, int(reply.Nodes[0].Queuenum), reply.Nodes[0].Timestamp, reply.Nodes[0].Nbhscore, v.priority)
	}
	s.ServerList[1-s.index].Clear()
	s.ServerList[1-s.index].MSet(m)
	s.index = 1 - s.index
}

func (s *ServerMap) AddConns() {
	items := s.ServerList[s.index].Items()
	for _, v := range items {
		conn := NewConnInfo(v.Addr)
		s.ConnList.Set(v.Addr, conn)
	}
}

/*--------------------------peermap--------------------------*/
type PeerInfo struct {
	connect *grpc.ClientConn
	client  *nodegrpc.NodeServerClient
}

func NewPeerInfo(addr string) *PeerInfo {
	co, _ := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	c := nodegrpc.NewNodeServerClient(co)
	return &PeerInfo{
		connect: co,
		client:  &c,
	}
}

func (p *PeerInfo) CloseConn() {
	p.connect.Close()
}

type PeerMap struct {
	peerList cmap.ConcurrentMap[string, struct{}]
	connList cmap.ConcurrentMap[string, *PeerInfo]
}

func NewPeerMap() PeerMap {
	return PeerMap{
		peerList: cmap.New[struct{}](),
		connList: cmap.New[*PeerInfo](),
	}
}

func (p *PeerMap) TryAddPeer(addr string) bool {
	return p.peerList.SetIfAbsent(addr, struct{}{})
}

func (p *PeerMap) AddConn(addr string) {
	p.connList.Set(addr, NewPeerInfo(addr))
}

func (p *PeerMap) GetConn(addr string) (*nodegrpc.NodeServerClient, error) {
	node, ok := p.connList.Get(addr)
	if !ok {
		logging.WriteLog("取通道错误")
		return nil, errors.New("get conn err")
	}
	return node.client, nil
}

func (p *PeerMap) CloseConn(addr string) {
	node, ok := p.connList.Get(addr)
	if !ok {
		logging.WriteLog("取通道错误")
		return
	}
	node.CloseConn()
	p.connList.Remove(addr)
	p.peerList.Remove(addr)
}

func (p *PeerMap) CloseAllConn() {
	items := p.connList.Items()
	for _, v := range items {
		v.CloseConn()
	}
	p.connList.Clear()
	p.peerList.Clear()
}
