/*
  - @Autor: XLF
  - @Date: 2022-10-24 09:14:29

* @LastEditors: XLF
* @LastEditTime: 2023-05-22 14:19:32
  - @Description:
    工作节点的存储对象
*/
package worknode

type WorkNode struct {
	worknodeIP   string // ip:port
	worknodeFlag int32  // 0:空闲 1:完成 2:失败
}

// 获取工作节点的IP地址
func (n *WorkNode) GetIP() string {
	return n.worknodeIP
}

// 获取工作节点的状态
func (n *WorkNode) GetFlag() int32 {
	return n.worknodeFlag
}

// 设置工作节点的IP地址
func (n *WorkNode) SetIP(ip string) {
	n.worknodeIP = ip
}

// 设置工作节点的状态
func (n *WorkNode) SetFlag(flag int32) {
	n.worknodeFlag = flag
}

// 初始化工作节点
func (n *WorkNode) InitWorkNode(ip string, flag int32) {
	n.worknodeIP = ip
	n.worknodeFlag = flag
}

// 重置工作节点
func (n *WorkNode) ResetWorkNode() {
	n.worknodeIP = ""
	n.worknodeFlag = 0
}

func SetWorksByString(s []string) []WorkNode {
	ws := make([]WorkNode, len(s))
	for index, i := range s {
		ws[index] = WorkNode{worknodeIP: i, worknodeFlag: 0}
	}
	return ws
}
