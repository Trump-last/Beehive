/*
  - @Autor: XLF
  - @Date: 2022-10-28 15:58:01

* @LastEditors: XLF
* @LastEditTime: 2023-05-18 11:53:01
  - @Description:
    从外来入口接收的创建Task信息，主要用于队列的任务存储
*/
package task

type Task struct {
	Id         string        // 任务id，在分发任务的机器上的唯一标识
	Cpu        float32       //任务所需的cpu资源
	Ram        float32       //任务所需的内存资源
	Addr       string        //任务所在的机器的gRPC服务器调用地址（IP+[PORT]）
	Command    string        //任务的执行命令
	Submittime int64         //任务提交时间
	Waittime   int64         //任务等待时间
	Prenode    string        //前一跳的节点IP地址
	Maxhop     int32         //最大跳数
	WaitChan   chan struct{} //任务等待通道
	FinishChan chan struct{} //任务完成通道
}

func NewTask(id string, cpu float32, ram float32, addr string, command string, submittime int64, prenode string, maxhop int32) Task {
	return Task{
		Id:         id,
		Cpu:        cpu,
		Ram:        ram,
		Addr:       addr,
		Command:    command,
		Submittime: submittime,
		Prenode:    prenode,
		Maxhop:     maxhop,
		WaitChan:   make(chan struct{}, 1),
		FinishChan: make(chan struct{}, 1),
	}
}

func NewTaskPointer(id string, cpu float32, ram float32, addr string, command string, submittime int64, prenode string, maxhop int32) *Task {
	return &Task{
		Id:         id,
		Cpu:        cpu,
		Ram:        ram,
		Addr:       addr,
		Command:    command,
		Submittime: submittime,
		Prenode:    prenode,
		Maxhop:     maxhop,
		WaitChan:   make(chan struct{}, 1),
		FinishChan: make(chan struct{}, 1),
	}
}
