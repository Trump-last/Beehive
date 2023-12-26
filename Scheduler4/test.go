/*
 * @Autor: XLF
 * @Date: 2022-10-24 16:43:03
 * @LastEditors: XLF
 * @LastEditTime: 2023-03-20 14:57:15
 * @Description:
 */
package main

import (
	"fmt"
	"math/rand"
	"time"
	"unsafe"

	nodegrpc "gitee.com/linfeng-xu/protofile/nodeGrpc"
)

type RewWindow struct {
	rewards [10]float32
	//最老的插入位置索引
	index int
	// 计数
	count int
	// 通知通道
	Notify chan struct{}
}

func (r *RewWindow) AddReward(reward float32) {
	r.rewards[r.index] = reward
	r.index = (r.index + 1) % 10
	r.count++
	if r.count == 10 {
		r.count = 0
		r.Notify <- struct{}{}
	}
}

// 从index开始，比较窗口前五个与后五个的reward，如果前五个的reward大于后五个的reward，则返回false，否则返回true
func (r *RewWindow) Compare() bool {
	var sum1, sum2 float32
	for i := 0; i < 5; i++ {
		sum1 += r.rewards[(r.index+i)%10]
		sum2 += r.rewards[(r.index+9-i)%10]
	}
	fmt.Println("sum1:", sum1/5, "sum2:", sum2/5)
	return sum1 <= sum2
}

func add10minus10(rw *RewWindow) {
	for {
		time.Sleep(time.Second * 1)
		//生成1-100的随机浮点数
		rw.AddReward(rand.Float32() * 100)
	}
}

func notify(rw *RewWindow) {
	for {
		<-rw.Notify
		fmt.Println(rw.rewards, "位置：", rw.index)
		if rw.Compare() {
			fmt.Println("老五个reward小于新五个reward")
		} else {
			fmt.Println("老五个reward大于新五个reward")
		}
	}
}

type nodes struct {
	Addr      string
	Cpu       float32
	Ram       float32
	Queuenum  int32
	Timestamp int64
}

func main() {
	m := nodegrpc.NodeInfo{
		Addr:      "127.0.0.1",
		Cpu:       1,
		Ram:       1,
		Queuenum:  1,
		Timestamp: 1,
	}
	z := nodes{
		Addr:      "127.0.0.1",
		Cpu:       1,
		Ram:       1,
		Queuenum:  1,
		Timestamp: 1,
	}
	fmt.Println(unsafe.Sizeof(m))
	fmt.Println(unsafe.Sizeof(z))
	for {
	}
}
