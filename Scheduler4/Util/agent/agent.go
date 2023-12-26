/*
  - @Autor: XLF
  - @Date: 2023-01-26 15:50:57

* @LastEditors: XLF
* @LastEditTime: 2023-05-19 10:48:21
  - @Description:
    决策模块，获取nodeserver的状态，包括节点资源信息，与邻域信息，以及待调度的任务信息，最终做出以下决策：
    1. 任务调度决策，某个任务是否需要转发，转发给哪个节点；如果不需要，是否需要等待，还是直接运行。
    2. 邻域更新决策，每隔一段时间，根据算法判断邻域是否要更新，如果要更新，更新邻域。
*/
package agent

import (
	config "scheduler4/Config"
	publicstu "scheduler4/PublicStu"
	"scheduler4/Util/node"
	"scheduler4/Util/node/task"
	servermap "scheduler4/Util/serverMap"
	"sort"

	nodeGrpc "gitee.com/linfeng-xu/protofile/nodeGrpc"
)

type PolicyObj interface {
	// 任务调度决策
	TaskPolicy([2]float32, []*nodeGrpc.NodeInfo, []*task.Task, int) []publicstu.TaskPolicy
}

/*
task切片sort辅助接口
未来可挪至task相关代码处
*/
type SortTask []*task.Task

func (a SortTask) Len() int           { return len(a) }
func (a SortTask) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a SortTask) Less(i, j int) bool { return (a[i].Cpu * a[i].Ram) > (a[j].Cpu * a[j].Ram) }

/*
func RunEpisode() {
	// 3. 获取任务状态
	tasks, pri := node.GetDispenseChan()
	len := len(tasks)
	if len == 0 {
		return
	}
	// 1. 获取节点状态
	resources := node.GetState()
	// 2. 获取邻域状态
	neighbors := servermap.GetNbh()
	// 4. 任务调度决策
	task_results := task_policy(resources, neighbors, tasks, pri, len)
	for index, task_res := range task_results {
		switch task_res.Policy {
		case 0:
			// 多跳
			go node.Wander(tasks[index], task_res.TargetNode)
		case 1:
			// 转发
			go node.DivideTask(tasks[index], task_res.TargetNode)
		case 2:
			// 等待
			go node.InsertTask(tasks[index])
		case 3:
			// 直接运行,不需要操作
		}
	}
}

func Run() {
	timer := time.NewTicker(5 * time.Millisecond)
	for {
		RunEpisode()
		<-timer.C
	}
}
*/

func RunEpisode(p PolicyObj) {
	// 3. 获取任务状态
	tasks, tasknum := node.GetDispenseChan()
	if tasknum == 0 {
		return
	}
	//sort the tasks
	var s_tasks SortTask
	s_tasks = tasks
	sort.Sort(s_tasks)
	tasks = s_tasks
	// 1. 获取节点状态
	resources := node.GetState()
	// 2. 获取邻域状态
	neighbors := servermap.GetNbh()
	// 4. 任务调度决策
	task_results := p.TaskPolicy(resources, neighbors, tasks, tasknum)
	for index, task_res := range task_results {
		switch task_res.Policy {
		case 0:
			// 多跳
			go node.Wander(tasks[index], task_res.TargetNode)
		case 1:
			// 转发
			go node.DivideTask(tasks[index], task_res.TargetNode)
		case 2:
			// 等待
			go node.InsertTask(tasks[index])
		case 3:
			// 直接运行,不需要操作
		}
	}
}

func Run() {
	var p PolicyObj
	if config.Policy_TEST == 0 {
		p = &yatpolicyrandom{}
	} else {
		p = &xlfpolicy{}
	}
	for {
		RunEpisode(p)
	}
}
