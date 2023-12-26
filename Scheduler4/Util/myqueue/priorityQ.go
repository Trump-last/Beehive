/*
 * @Autor: XLF
 * @Date: 2023-03-02 14:03:02
 * @LastEditors: XLF
 * @LastEditTime: 2023-03-31 01:22:49
 * @Description:
 */
package myqueue

// 优先级队列
type PriorityQ[V interface{}] struct {
	data []chan V
}

// New 创建优先级队列，num指定有多少个优先级
func NewPriQ[V interface{}](num int) PriorityQ[V] {
	dataq := make([]chan V, num)
	for i := 0; i < num; i++ {
		dataq[i] = make(chan V, 100)
	}
	pq := PriorityQ[V]{
		data: dataq,
	}
	return pq
}

// Push 向优先级队列中添加元素，priority指定优先级
func (pq *PriorityQ[V]) Push(v V, priority int) {
	pq.data[priority] <- v
}

// Pop 从优先级队列中取出元素，max指定最多取出多少个元素，返回值为取出的元素和对应的优先级数组
func (pq *PriorityQ[V]) Pop(max int) ([]V, []int) {
	var ret []V
	var retpri []int
	index := 0
outloop:
	for i, v := range pq.data {
	inloop:
		for {
			select {
			case val := <-v:
				ret = append(ret, val)
				retpri = append(retpri, i)
				index++
				if index >= max {
					break outloop
				}
			default:
				break inloop
			}
		}
	}
	return ret, retpri
}
