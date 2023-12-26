/*
 * @Autor: XLF
 * @Date: 2023-03-31 01:19:30
 * @LastEditors: XLF
 * @LastEditTime: 2023-05-22 15:53:22
 * @Description:
 */
package myqueue

import "time"

type FcfsQ[V interface{}] struct {
	data    chan V
	timer   *time.Timer
	tasknum int
}

func NewfcfsQ[V interface{}]() FcfsQ[V] {
	dataq := make(chan V, 200)
	fq := FcfsQ[V]{
		data:    dataq,
		timer:   time.NewTimer(3 * time.Millisecond),
		tasknum: 0,
	}
	return fq
}

// Push 向优先级队列中添加元素，priority指定优先级
func (fq *FcfsQ[V]) Push(v V) {
	fq.data <- v
}

func (fq *FcfsQ[V]) Pop(num int) ([]V, int) {
	fq.timer.Reset(3 * time.Millisecond)
	fq.tasknum = 0
	var ret []V
gettask:
	for {
		select {
		case val := <-fq.data:
			ret = append(ret, val)
			fq.tasknum++
			if fq.tasknum >= num {
				break gettask
			}
		case <-fq.timer.C:
			break gettask
		}
	}
	return ret, fq.tasknum
}

func (fq *FcfsQ[V]) Len() int {
	return len(fq.data)
}
