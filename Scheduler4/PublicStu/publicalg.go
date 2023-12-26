/*
 * @Autor: XLF
 * @Date: 2023-06-25 15:06:45
 * @LastEditors: XLF
 * @LastEditTime: 2023-06-25 15:11:22
 * @Description:
 */
package public

import (
	"time"

	"gitee.com/linfeng-xu/protofile/nodeGrpc"
)

const (
	K_local float32 = 1.8
	K_time1 float32 = 1.2
	K_time2 float32 = 1
	K_time3 float32 = 0.8
)

// 邻域打分函数
func ScoreNeighbours(neighbors []*nodeGrpc.NodeInfo, localcpu float32, localram float32) float32 {
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
