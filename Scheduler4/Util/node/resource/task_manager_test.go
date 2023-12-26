/*
 * @Autor: XLF
 * @Date: 2023-03-03 10:34:53
 * @LastEditors: XLF
 * @LastEditTime: 2023-03-06 20:49:51
 * @Description:
 */
package resource_test

import (
	"fmt"
	"log"
	"scheduler4/Util/node/resource"
	ts "scheduler4/Util/node/task"
	"testing"
	"time"
)

var task1 = ts.NewTask("202200001", 0.2, 0.3, "127.0.0.1", "sleep 11", 0, "", 0)
var task2 = ts.NewTask("202200002", 0.2, 0.3, "127.0.0.1", "sleep 11", 0, "", 0)
var task3 = ts.NewTask("202200003", 0.2, 0.3, "127.0.0.1", "sleep 22", 0, "", 0)

func TestTaskManger(t *testing.T) {

	// init
	log.Println("test start")
	manager := resource.NewTaskManagerFrom([2]float32{10.0, 10.0})

	var tasks []*ts.Task
	for i := 0; i < 100; i++ {
		tasks = append(tasks, ts.NewTaskPointer(fmt.Sprintf("202200%3d", i), 0.1, 0.1, "127.0.0.1", "echo 'helloworld' ", 0, "", 0))
	}

	for i := 0; i < 100; i++ {
		err := manager.PreAlloc(tasks[i])
		checkErr(err)
		manager.RunTask(tasks[i])
		log.Println("------------------------------------")
		log.Println("max", manager.GetMaxRes())
		log.Println("used", manager.GetUsedRes())
		log.Println("avail", manager.GetAvailRes())

		time.Sleep(100 * time.Millisecond)
	}

	for i := 0; i < 10000; i++ {
		log.Println(i, " watching ---------------------------")
		log.Println("max", manager.GetMaxRes())
		log.Println("used", manager.GetUsedRes())
		log.Println("avail", manager.GetAvailRes())
		time.Sleep(500 * time.Millisecond)
	}

}
func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
