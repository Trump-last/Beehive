/*
 * @Autor: XLF
 * @Date: 2022-11-08 14:12:09
 * @LastEditors: XLF
 * @LastEditTime: 2023-09-06 16:18:00
 * @Description:
 */
package logging

import (
	"bufio"
	"fmt"
	"log"
	"os"
	config "scheduler4/Config"
	publicstu "scheduler4/PublicStu"
	"scheduler4/Util/node/task"
	"time"

	"gitee.com/linfeng-xu/protofile/nodeGrpc"
)

type logs struct {
	//时间戳
	time string
	//节点iP
	nodeIP string
	//任务ID
	taskID string
	//任务类型
	taskType string
	//资源要求
	cpu float32
	ram float32
}

type schevents struct {
	timestamp int64
	resources [2]float32
	neighbors []*nodeGrpc.NodeInfo
	tasks     []*task.Task
	len       int
	res       []publicstu.TaskPolicy
}

func (s schevents) String() string {
	var info string
	info += fmt.Sprintf("%d,%d\n", s.timestamp, s.len)
	info += fmt.Sprintf("%f,%f\n", s.resources[0], s.resources[1])
	for _, node := range s.neighbors {
		info += fmt.Sprintf("%s,%.2f,%.2f,%d,%d;", node.Addr, node.Cpu, node.Ram, node.Queuenum, node.Timestamp)
	}
	info += "\n"
	for _, task := range s.tasks {
		info += fmt.Sprintf("%s,%.2f,%.2f,%d,%d;", task.Id, task.Cpu, task.Ram, task.Waittime, task.Maxhop)
	}
	info += "\n"
	for _, result := range s.res {
		info += fmt.Sprintf("%d,", result.Policy)
		l := len(result.TargetNode)
		for idx, node := range result.TargetNode {
			if idx == l-1 {
				info += node
				break
			}
			info += node + ","
		}
		info += ";"
	}
	info += "\n"
	return info
}

/*************************错误日志*************************/
func WriteLog(errinfo string) {
	//打开文件
	f, err := os.OpenFile(config.LOGFILEPATH, os.O_WRONLY|os.O_APPEND|os.O_SYNC|os.O_CREATE, 0666)
	if err != nil {
		fmt.Println(errinfo, "打开文件失败")
		return
	}
	defer f.Close()
	//写入文件
	log.SetOutput(f)
	log.Println(errinfo)
}

/*************************任务日志*************************/
var TaskLog = make(chan logs, 10000)

func NoteLogQueue(t string, model int, taskid string, nodeIP string, cpu float32, ram float32) {
	switch model {
	case config.Dispense:
		TaskLog <- logs{time: t, taskID: taskid, taskType: "Dispense", nodeIP: nodeIP, cpu: cpu, ram: ram}
	case config.DivideEnd:
		TaskLog <- logs{time: t, taskID: taskid, taskType: "DivideEnd", nodeIP: nodeIP, cpu: cpu, ram: ram}
	case config.Run:
		TaskLog <- logs{time: t, taskID: taskid, taskType: "Run", nodeIP: nodeIP, cpu: cpu, ram: ram}
	case config.StartWander:
		TaskLog <- logs{time: t, taskID: taskid, taskType: "StartWander", nodeIP: nodeIP, cpu: cpu, ram: ram}
	case config.StartDivide:
		TaskLog <- logs{time: t, taskID: taskid, taskType: "StartDivide", nodeIP: nodeIP, cpu: cpu, ram: ram}
	case config.WanderGet:
		TaskLog <- logs{time: t, taskID: taskid, taskType: "WanderGet", nodeIP: nodeIP, cpu: cpu, ram: ram}
	case config.DivideGet:
		TaskLog <- logs{time: t, taskID: taskid, taskType: "DivideGet", nodeIP: nodeIP, cpu: cpu, ram: ram}
	case config.StartDivideConfirm:
		TaskLog <- logs{time: t, taskID: taskid, taskType: "StartDivideConfirm", nodeIP: nodeIP, cpu: cpu, ram: ram}
	case config.StartDivideError:
		TaskLog <- logs{time: t, taskID: taskid, taskType: "StartDivideError", nodeIP: nodeIP, cpu: cpu, ram: ram}
	case config.DivideError:
		TaskLog <- logs{time: t, taskID: taskid, taskType: "DivideError", nodeIP: nodeIP, cpu: cpu, ram: ram}
	case config.DivideConfirm:
		TaskLog <- logs{time: t, taskID: taskid, taskType: "DivideConfirm", nodeIP: nodeIP, cpu: cpu, ram: ram}
	}
}

func WriteLogGo() {
	os.Remove(config.TaskLog)
	file, _ := os.OpenFile(config.TaskLog, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	defer file.Close()
	writer := bufio.NewWriterSize(file, 5*32*1024)
	var logfile logs
	index := 0
	timer := time.NewTimer(100 * time.Millisecond)
	for {
		<-timer.C
	getlog:
		for {
			select {
			case logfile = <-TaskLog:
				index++
				writer.WriteString(fmt.Sprintf("%s,%s,%s,%s,%f,%f\n", logfile.time, logfile.taskID, logfile.taskType, logfile.nodeIP, logfile.cpu, logfile.ram))
				if index == 2000 {
					break getlog
				}
			default:
				break getlog
			}
		}
		if index > 0 {
			writer.Flush()
			index = 0
		}
		timer.Reset(100 * time.Millisecond)
	}
}

/*************************决策日志*************************/
var ScheLog = make(chan schevents, 10000)

func NoteEventQueue(time int64, resources [2]float32, neighbors []*nodeGrpc.NodeInfo, tasks []*task.Task, len int, res []publicstu.TaskPolicy) {
	schevents := schevents{timestamp: time, resources: resources, neighbors: neighbors, tasks: tasks, len: len, res: res}
	ScheLog <- schevents
}

func WriteSchLog() {
	os.Remove(config.ScheLog)
	file, _ := os.OpenFile(config.ScheLog, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	defer file.Close()
	writer := bufio.NewWriterSize(file, 10*32*1024)
	var schevent schevents
	index := 0
	timer := time.NewTimer(100 * time.Millisecond)
	for {
		<-timer.C
	getlog:
		for {
			select {
			case schevent = <-ScheLog:
				index++
				writer.WriteString(schevent.String())
				if index == 3000 {
					break getlog
				}
			default:
				break getlog
			}
		}
		if index > 0 {
			writer.Flush()
			index = 0
		}
		timer.Reset(100 * time.Millisecond)
	}
}

func Run() {
	go WriteLogGo()
	go WriteSchLog()
}
