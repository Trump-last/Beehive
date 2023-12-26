/*
* @Autor: XLF
* @Date: 2023-01-27 11:37:24
  - @LastEditors: XLF
  - @LastEditTime: 2023-05-22 10:30:24

* @Description:
策略模块
*/
package agent

import (
	"math"
	"math/rand"
	config "scheduler4/Config"
	publicstu "scheduler4/PublicStu"
	"scheduler4/Util/logging"
	"scheduler4/Util/node"
	"scheduler4/Util/node/task"
	"time"

	nodeGrpc "gitee.com/linfeng-xu/protofile/nodeGrpc"
)

/*
Forward _Policy:
0: 2Random
1: 1Chosen1Random
*/
//const Forward_policy int = 1

/*
Policy:
0: 多跳  targetNode: [1]string
1: 转发  targetNode: [2]string
2: 等待  targetNode: nil
3: 直接运行  targetNode: nil
*/
type TaskPolicy publicstu.TaskPolicy

/*
// 任务调度策略，根据邻域状态，节点状态，以及任务需求，对一组任务做出调度决策
func task_policy(resources [2]float32, neighbors []*nodeGrpc.NodeInfo, tasks []*task.Task, pri []int, len int) []TaskPolicy {
	res := make([]TaskPolicy, len)
	for index, task := range tasks {
		if node.RunTask(task) == nil {
			res[index] = TaskPolicy{Policy: 3, TargetNode: nil}
		} else {
			switch pri[index] {
			case 0:
				// 高优先级的任务
				if task.Maxhop <= 0 {
					// 达到多跳上限，在本节点等待运行
					res[index] = TaskPolicy{Policy: 2, TargetNode: nil}
				} else {
					//没有达到多跳上限，可再进行多跳
					addr := servermap.ChooseBest(task)
					res[index] = TaskPolicy{Policy: 0, TargetNode: []string{addr}}
				}
			case 1:
				// 中优先级的任务
				res[index] = TaskPolicy{Policy: 2, TargetNode: nil}
			case 2:
				// 低优先级的任务
				addrs := servermap.Choose(task)
				res[index] = TaskPolicy{Policy: 1, TargetNode: addrs}
			}
		}
	}
	return res
}s
*/

type xlfpolicy struct{}

func (p *xlfpolicy) TaskPolicy(resources [2]float32, neighbors []*nodeGrpc.NodeInfo, tasks []*task.Task, len int) []publicstu.TaskPolicy {
	res := make([]publicstu.TaskPolicy, len)
	nowtime := time.Now().UnixMilli()
	var waittime int64
	for index, task := range tasks {
		waittime = nowtime - task.Submittime
		tasks[index].Waittime = waittime
		if node.RunTask(task) == nil {
			res[index] = publicstu.TaskPolicy{Policy: 3, TargetNode: nil}
		} else {
			if task.Maxhop <= 0 {
				// 达到多跳上限，在本节点等待运行
				res[index] = publicstu.TaskPolicy{Policy: 2, TargetNode: nil}
			} else {
				if waittime < 40 {
					addrs := ChooseFWDscore(task, neighbors)
					res[index] = publicstu.TaskPolicy{Policy: 1, TargetNode: addrs}
				} else if waittime < 80 {
					res[index] = publicstu.TaskPolicy{Policy: 2, TargetNode: nil}
				} else {
					addr := ChooseBest(task, neighbors)
					res[index] = publicstu.TaskPolicy{Policy: 0, TargetNode: []string{addr}}
				}
			}
		}
	}
	logging.NoteEventQueue(nowtime, resources, neighbors, tasks, len, res)
	return res
}

// 转发策略
func ChooseFWD(t *task.Task, lists []*nodeGrpc.NodeInfo) []string {
	var addrs []string
	// 根据策略选择需要转发的两个节点
	if config.Policy_FWD == 0 {
		addrs = ChooseRandom(t, lists)
	} else if config.Policy_FWD == 1 {
		addrs = ChooseDRF(t, lists)
	} else {
		addrs = ChooseDP(t, lists)
	}
	return addrs
}

/*
Policy: choose best
遍历邻域所有节点，找到第一个资源满足的节点返回
若没有满足的节点，返回随机值
*/
func ChooseBest(t *task.Task, lists []*nodeGrpc.NodeInfo) string {
	//lists := serverm.GetServerList()
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
	UpdateNeighbor(t, lists, index)
	return lists[index].Addr
}

/*
选择策略时候更新list
*/
func UpdateNeighbor(t *task.Task, neighbors []*nodeGrpc.NodeInfo, index int) {
	neighbors[index].Cpu = neighbors[index].Cpu - t.Cpu
	neighbors[index].Ram = neighbors[index].Ram - t.Ram
}

/*
Policy: choose random
返回随机两个节点地址
*/
func ChooseRandom(t *task.Task, neighbors []*nodeGrpc.NodeInfo) []string {
	i, j := randint(len(neighbors))
	UpdateNeighbor(t, neighbors, i)
	UpdateNeighbor(t, neighbors, j)
	return []string{neighbors[i].Addr, neighbors[j].Addr}
}

/*
Policy: DP
点积法，通过多维向量点积的方法选择最优两个节点
*/
func ChooseDP(t *task.Task, neighbors []*nodeGrpc.NodeInfo) []string {
	var id1 int = 0
	var id2 int = 0
	var score1 float32 = 0
	var score2 float32 = 0

	for index, value := range neighbors {
		temp := CountDP(value.Cpu, value.Ram, t.Cpu, t.Ram)
		if temp > score1 {
			score2 = score1
			id2 = id1
			score1 = temp
			id1 = index
		} else if temp > score2 {
			score2 = temp
			id2 = index
		}
	}
	UpdateNeighbor(t, neighbors, id1)
	UpdateNeighbor(t, neighbors, id2)
	return []string{neighbors[id1].Addr, neighbors[id2].Addr}
}

func CountDP(cpu_cap float32, ram_cap float32, cpu_req float32, ram_req float32) float32 {
	var score float32 = 0
	score = cpu_cap*cpu_req + ram_cap*ram_req
	//fmt.Println("LR score = ", score)
	return score
}

/*
Policy: DRF
选择关键资源占比最多的两个节点
*/
func ChooseDRF(t *task.Task, neighbors []*nodeGrpc.NodeInfo) []string {
	var id1 int = 0
	var id2 int = 0
	var score1 float32 = 0
	var score2 float32 = 0

	for index, value := range neighbors {
		temp := CountDRF(value.Cpu, value.Ram, t.Cpu, t.Ram)
		if temp > score1 {
			score2 = score1
			id2 = id1
			score1 = temp
			id1 = index
		} else if temp > score2 {
			score2 = temp
			id2 = index
		}
	}
	//fmt.Println(id1,id2)
	UpdateNeighbor(t, neighbors, id1)
	UpdateNeighbor(t, neighbors, id2)
	return []string{neighbors[id1].Addr, neighbors[id2].Addr}
}

func CountDRF(cpu_cap float32, ram_cap float32, cpu_req float32, ram_req float32) float32 {
	score := math.Max(float64(cpu_cap/cpu_req), float64(ram_cap/ram_req))
	//fmt.Println("LR score = ", score)
	return float32(score)
}

func LeastRequest(cpu_cap float32, ram_cap float32, cpu_req float32, ram_req float32) float32 {
	var score float32 = 0
	score_cpu := (cpu_cap - cpu_req) / cpu_cap * float32(10)
	score_ram := (ram_cap - ram_req) / ram_cap * float32(10)
	score = (score_cpu + score_ram) / 2
	//fmt.Println("LR score = ", score)
	return score
}

func NodeAffinity() int {
	return 0
}

func Choose(t *task.Task, neighbors []*nodeGrpc.NodeInfo) []string {
	var id1 int = 0
	var id2 int = 0
	var score1 float32 = 0
	var score2 float32 = 0

	for index, value := range neighbors {
		temp := LeastRequest(value.Cpu, value.Ram, t.Cpu, t.Ram)
		if temp > score1 {
			score2 = score1
			id2 = id1
			score1 = temp
			id1 = index
		} else if temp > score2 {
			score2 = temp
			id2 = index
		}
	}
	//fmt.Println(id1,id2)
	UpdateNeighbor(t, neighbors, id1)
	UpdateNeighbor(t, neighbors, id2)
	return []string{neighbors[id1].Addr, neighbors[id2].Addr}
}

func randint(len int) (int, int) {
	i, j := rand.Intn(len), rand.Intn(len)
	for i == j {
		j = rand.Intn(len)
	}
	return i, j
}

type yatpolicyprob struct{}

const LearningProb = 0.1

/*
	func (p *yatpolicyprob) TaskPolicy(resources [2]float32, neighbors []*nodeGrpc.NodeInfo, tasks []*task.Task, len int) []TaskPolicy {
		res := make([]TaskPolicy, len)
		nowtime := time.Now().UnixMilli()
		//policy := config.Policy_TEST;
		var waittime int64
		for index, task := range tasks {
			if node.RunTask(task) == nil {
				res[index] = TaskPolicy{Policy: 3, TargetNode: nil}
				//policy = 3;
			} else {
				random := rand.Float32()
				if random <= LearningProb {
					res[index] = RamChoose(task, neighbors)
				} else{
					if task.Maxhop <= 0 {
						// 达到多跳上限，在本节点等待运行
						res[index] = TaskPolicy{Policy: 2, TargetNode: nil}
						//policy = 4;
					} else {
						waittime = nowtime - task.Submittime
						if waittime < 40 {
							addrs := ChooseFWD(task, neighbors)
							res[index] = TaskPolicy{Policy: 1, TargetNode: addrs}
						} else if waittime < 80 {
							res[index] = TaskPolicy{Policy: 2, TargetNode: nil}
							//policy = 4;
						} else {
							addr := ChooseBest(task, neighbors)
							res[index] = TaskPolicy{Policy: 0, TargetNode: []string{addr}}
							//policy = 5;
						}
					}
				}
			}
		}
		//fmt.Println(policy) //把policy写入日志，需要改一下
		return res
	}
*/
type yatpolicyrandom struct{}

func RamChoose(t *task.Task, lists []*nodeGrpc.NodeInfo) publicstu.TaskPolicy {
	random := rand.Intn(3)
	//var addrs []string
	res := publicstu.TaskPolicy{Policy: 2, TargetNode: nil}
	switch random {
	case 0:
		if t.Maxhop > 0 {
			addr := ChooseBest(t, lists)
			res = publicstu.TaskPolicy{Policy: 0, TargetNode: []string{addr}}
		}
	case 1:
		addrs := ChooseFWDscore(t, lists)
		res = publicstu.TaskPolicy{Policy: 1, TargetNode: addrs}
	case 2:
		//res = publicstu.TaskPolicy{Policy: 2, TargetNode: nil}
	}
	return res
}

func (p *yatpolicyrandom) TaskPolicy(resources [2]float32, neighbors []*nodeGrpc.NodeInfo, tasks []*task.Task, len int) []publicstu.TaskPolicy {
	res := make([]publicstu.TaskPolicy, len)
	nowtime := time.Now().UnixMilli()
	var waittime int64
	for index, task := range tasks {
		waittime = nowtime - task.Submittime
		tasks[index].Waittime = waittime
		if node.RunTask(task) == nil {
			res[index] = publicstu.TaskPolicy{Policy: 3, TargetNode: nil}
		} else {
			res[index] = RamChoose(task, neighbors)
		}
	}
	logging.NoteEventQueue(nowtime, resources, neighbors, tasks, len, res)
	return res
}

type yatpolicy1 struct{}

func (p *yatpolicy1) TaskPolicy(resources [2]float32, neighbors []*nodeGrpc.NodeInfo, tasks []*task.Task, len int) []TaskPolicy {
	res := make([]TaskPolicy, len)
	nowtime := time.Now().UnixMilli()
	//policy := config.Policy_TEST;
	//节点打分算法，不考虑policy
	var waittime int64
	for index, task := range tasks {
		if node.RunTask(task) == nil {
			res[index] = TaskPolicy{Policy: 3, TargetNode: nil}
			//policy = 3;
		} else {
			if task.Maxhop <= 0 {
				// 达到多跳上限，在本节点等待运行
				res[index] = TaskPolicy{Policy: 2, TargetNode: nil}
				//policy = 4;
			} else {
				waittime = nowtime - task.Submittime
				if waittime < 40 {
					addrs := ChooseFWDscore(task, neighbors)
					res[index] = TaskPolicy{Policy: 1, TargetNode: addrs}
				} else if waittime < 80 {
					res[index] = TaskPolicy{Policy: 2, TargetNode: nil}
					//policy = 4;
				} else {
					addr := ChooseBest(task, neighbors)
					res[index] = TaskPolicy{Policy: 0, TargetNode: []string{addr}}
					//policy = 5;
				}
			}
		}
	}
	//fmt.Println(policy) //把policy写入日志，需要改一下
	return res
}

// 转发策略
func ChooseFWDscore(t *task.Task, neighbors []*nodeGrpc.NodeInfo) []string {
	//var addrs []string
	var id int = -1
	var score float32 = 0
	len := len(neighbors)
	//var thres float32 = t.Cpu * t.Ram
	if config.Policy_FWD == 1 {
		for index, value := range neighbors {
			if value.Cpu > t.Cpu && value.Ram > t.Ram {
				temp := value.Cpu * value.Ram
				if temp > score {
					score = temp
				}
				id = index
			}
		}
	}
	//always choose forwards
	if id == -1 {
		id = rand.Intn(len)
	}
	random := rand.Intn(len)
	for random == id {
		random = rand.Intn(len)
	}
	//fmt.Println(id1,id2)
	UpdateNeighbor(t, neighbors, id)

	return []string{neighbors[id].Addr, neighbors[random].Addr}
}

const K_local float32 = 1.8
const K_time1 float32 = 1.2
const K_time2 float32 = 1
const K_time3 float32 = 0.8

// 邻域打分函数
func scoreNeighbours(neighbors []*nodeGrpc.NodeInfo, localcpu float32, localram float32) float32 {
	var temp1 float32 = localcpu * localram * K_local
	var score1 float32 = K_local * min(localcpu, localram)
	var temp2 float32 = 0
	var socre2 float32 = 0
	for _, v := range neighbors {
		k_time := countKtime(v.Timestamp)
		temp := k_time * v.Cpu * v.Ram
		if temp > temp1 {
			temp2 = temp1
			socre2 = score1
			temp1 = temp
			score1 = k_time * min(v.Cpu, v.Ram)
		} else if temp > temp2 {
			temp2 = temp
			socre2 = k_time * min(v.Cpu, v.Ram)
		}
	}
	if score1 > socre2 {
		return score1
	}
	return socre2
}

func countKtime(timestamp int64) float32 {
	nowtime := time.Now().UnixMilli()
	delta := nowtime - timestamp
	if delta < 10 {
		return K_time1
	} else if delta < 20 {
		return K_time2
	} else {
		return K_time3
	}
}

func min(a float32, b float32) float32 {
	if a > b {
		return b
	}
	return a
}
