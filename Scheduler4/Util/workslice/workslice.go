/*
  - @Autor: XLF
  - @Date: 2022-10-24 09:25:31

* @LastEditors: XLF
* @LastEditTime: 2023-05-22 14:20:51
  - @Description:
    工作节点的存储数组
*/
package workslice

import (
	worknode "scheduler4/Util/worknode"
)

type WorkSlice struct {
	workSlice []worknode.WorkNode
}

// 新建工作列表
func NewWorkSlice(len int32) *WorkSlice {
	s := new(WorkSlice)
	s.InitWorkSlice(len)
	return s
}

// 初始化工作节点数组
func (s *WorkSlice) InitWorkSlice(len int32) {
	s.workSlice = make([]worknode.WorkNode, len)
}

// 设置工作节点数组
func (s *WorkSlice) SetWorkSlice(ws []worknode.WorkNode) {
	s.workSlice = make([]worknode.WorkNode, len(ws))
	copy(s.workSlice, ws)
}

// 重置工作节点数组
func (s *WorkSlice) ResetWorkSlice(ws []worknode.WorkNode) {
	s.workSlice = make([]worknode.WorkNode, len(ws))
	copy(s.workSlice, ws)
}

// 清空工作节点数组
func (s *WorkSlice) ClearWorkSlice() {
	s.InitWorkSlice(int32(len(s.workSlice)))
}

// 根据IP获取工作节点状态
func (s *WorkSlice) GetWorkNodeFlag(ip string) int32 {
	if s.IsEmpty() {
		return -1
	}
	for _, v := range s.workSlice {
		if v.GetIP() == ip {
			return v.GetFlag()
		}
	}
	return -1
}

// 根据index设置工作节点状态
func (s *WorkSlice) SetWorkNodeFlag(index int32, flag int32) bool {
	if index == -1 {
		return false
	}
	s.workSlice[index].SetFlag(flag)
	return true
}

// 查询当前节点是否是工作节点
func (s *WorkSlice) QueryIsWork(ip string) int32 {
	if s.IsEmpty() {
		return -1
	}
	for i, v := range s.workSlice {
		if v.GetIP() == ip {
			if v.GetFlag() != 0 {
				return -1
			}
			return int32(i)
		}
	}
	return -1
}

// 判断当前列表是否为空，（空指的是，数组里的对象都未被赋值，不是指数组为空）
func (s *WorkSlice) IsEmpty() bool {
	return s.workSlice[0].GetIP() == ""
}

// 返回工作节点数组的数量
func (s *WorkSlice) GetWorkSliceLen() int32 {
	return int32(len(s.workSlice))
}

func SetWorksByString(s []string) *WorkSlice {
	ws := NewWorkSlice(int32(len(s)))
	for i, v := range s {
		ws.workSlice[i].InitWorkNode(v, 0)
	}
	return ws
}
