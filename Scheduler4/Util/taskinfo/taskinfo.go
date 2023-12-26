/*
 * @Autor: XLF
 * @Date: 2022-10-24 11:10:32
 * @LastEditors: XLF
 * @LastEditTime: 2023-03-06 20:50:22
 * @Description:
 */

package taskinfo

import (
	config "scheduler4/Config"
	"scheduler4/Util/worknode"
	"scheduler4/Util/workslice"
	"sync"
	"time"
)

type TaskInfo struct {
	mu         sync.Mutex //互斥锁
	id         string     //任务id
	cpu        float32    //cpu要求
	ram        float32    //内存要求
	command    string     //任务命令
	prenode    string     //前置节点
	submittime int64      //提交时间
	maxhop     int32      //最大跳数

	works   *workslice.WorkSlice //工作节点
	flag    int                  //任务状态
	errnum  int                  //错误次数
	timeout time.Duration        //任务超时时间
}

// 新建任务信息对象
func NewTaskInfo(id string, cpu float32, ram float32, com string, submittime int64, prenode string, maxhop int32, ws *workslice.WorkSlice) *TaskInfo {
	return &TaskInfo{
		mu:         sync.Mutex{},
		id:         id,
		works:      ws,
		cpu:        cpu,
		ram:        ram,
		submittime: submittime,
		prenode:    prenode,
		maxhop:     maxhop,
		command:    com,
		flag:       0,
		errnum:     0,
		timeout:    time.Duration(config.TIMEOUT) * time.Millisecond,
	}
}

// 手动设置任务资源信息，几乎不使用，除非手动修改任务资源信息
func (t *TaskInfo) SetResource(cpu float32, ram float32) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cpu = cpu
	t.ram = ram
}

// 手动设置任务状态，几乎不适用，除非手动设置任务状态
func (t *TaskInfo) SetFlag(flag int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.flag = flag
}

// 重置flag, 只用于控制协程的调用
func (t *TaskInfo) ResetFlag() {
	t.SetFlag(0)
}

// 任务远程确认调用
func (t *TaskInfo) Confirm(ip string, Delete chan string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	switch t.flag {
	case 0: //任务进行中
		index := t.works.QueryIsWork(ip) //判断是否为工作节点，只有工作节点可以操作任务对象
		if index != -1 {                 //判断是否为工作节点，只有工作节点可以操作任务对象
			t.flag = 1                        //标识任务完成
			t.works.SetWorkNodeFlag(index, 1) //设置工作节点状态为完成
			Delete <- t.id                    //任务已经完成，发送任务id到删除通道，等待删除
			return true
		} else { //不是工作节点
			return false //返回false，不允许操作
		}
	default: // 任务已完成，或者任务已失败等待重试
		return false //返回false，不允许操作
	}
}

// 任务远程错误调用
func (t *TaskInfo) AddErrNum(ip string, Error chan string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	switch t.flag {
	case 0: //任务进行中
		index := t.works.QueryIsWork(ip) //判断是否为工作节点，只有工作节点可以操作任务对象
		if index != -1 {
			t.works.SetWorkNodeFlag(index, 2)               //设置工作节点状态为失败
			t.errnum++                                      //任务错误次数加一
			if t.errnum >= int(t.works.GetWorkSliceLen()) { //任务错误次数大于等于工作节点数量
				t.flag = 2               //任务状态为失败
				t.works.ClearWorkSlice() //清空工作节点,为重启做准备
				Error <- t.id            //发送任务id到错误通道
			}
		}
		return
	default:
		return
	}
}

// 重启任务，重新选择工作节点
func (t *TaskInfo) ReNew(ws []worknode.WorkNode) (string, float32, float32, string, time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.flag = 0
	t.works.ResetWorkSlice(ws) //重置工作节点
	t.errnum = 0               //重置错误次数
	return t.id, t.cpu, t.ram, t.command, t.timeout
}

// 获取任务信息
func (t *TaskInfo) GetInfo() (string, float32, float32, string, int64, string, int32) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.id, t.cpu, t.ram, t.command, t.submittime, t.prenode, t.maxhop
}
