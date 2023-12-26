/*
 * @Autor: XLF
 * @Date: 2023-02-14 15:30:10
 * @LastEditors: XLF
 * @LastEditTime: 2023-09-06 16:14:01
 * @Description:
 */
package logging

import (
	"bufio"
	"fmt"
	"os"
	"time"
)

var Tasklog = "Tasklog.csv"

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

var TasklogQueue = make(chan logs, 10000)

func NoteLogQueue(t string, taskid string, nodeIP string, cpu float32, ram float32) {
	TasklogQueue <- logs{time: t, taskID: taskid, taskType: "LB", nodeIP: nodeIP, cpu: cpu, ram: ram}
}

func WriteLogGo() {
	os.Remove(Tasklog)
	/*
		go func() {
			for task := range TasklogQueue {
				file, _ := os.OpenFile(Tasklog, os.O_WRONLY|os.O_APPEND|os.O_SYNC|os.O_CREATE, 0666)
				file.WriteString(fmt.Sprintf("%s,%s,%s,%s,%f,%f\n", task.time, task.taskID, task.taskType, task.nodeIP, task.cpu, task.ram))
				file.Close()
			}
		}()*/
	file, _ := os.OpenFile(Tasklog, os.O_WRONLY|os.O_APPEND|os.O_SYNC|os.O_CREATE, 0666)
	defer file.Close()
	writer := bufio.NewWriterSize(file, 10*32*1024)
	var logfile logs
	index := 0
	timer := time.NewTimer(100 * time.Millisecond)
	for {
		<-timer.C
	getlog:
		for {
			select {
			case logfile = <-TasklogQueue:
				index++
				writer.WriteString(fmt.Sprintf("%s,%s,%s,%s,%f,%f\n", logfile.time, logfile.taskID, logfile.taskType, logfile.nodeIP, logfile.cpu, logfile.ram))
				if index == 4500 {
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
