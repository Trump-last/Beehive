<!--
 * @Autor: XLF
 * @Date: 2023-12-26 19:24:42
 * @LastEditors: XLF
 * @LastEditTime: 2023-12-26 19:41:49
 * @Description: 
-->
# Beehive

Beehive is a distributed task scheduling framework built upon the small world network model. This repository contains its core code, designed to showcase the architecture and scheduling process of Beehive. It is not a specific real-world application version.

# Architecture


Beehive主要由三部分组成：

## 本地测试使用流程

1. 修改Scheduler4/Util/node/node.go 中的GetOutBoundIP()方法中的IP，本地测试就是用127.0.0.1（*本地回环地址*）
2. 首先启动GCS服务器，运行GCS/main.go
3. 运行节点服务器，Scheduler4/NodeServer/main.go
4. 启动LB服务器，LB/main.go
5. 使用gRPC发送任务给LB，目前临时测试文件在LB/client/main.go，可供使用者模仿使用

## Pod测试流程

1. 修改Scheduler4/Util/node/node.go 中的GetOutBoundIP()方法中的IP，注释掉`ip = "127.0.0.1"`这一行，其余行为如本地测试，不同的是启动的是GCS pod与LB pod。

# 接下来的工作

1. servermap肯定是要根据反馈调整的
2. 将Agent（决策模块）从Sche(节点服务器)中独立出来，以供算法设计使用
3. Proto文件一致性的保证
4. LB存在节点滞后性的问题，连接不上的问题未处理
5. node-node邻域关系招呼功能未添加，待后续考察后，看是否要添加
