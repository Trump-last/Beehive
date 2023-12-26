package resource

import (
	"errors"
	"fmt"
	"os/exec"
	config "scheduler4/Config"
	"scheduler4/Util/logging"
	ts "scheduler4/Util/node/task"
	"sync"
	"time"
)

type TaskID string

type TaskStatus string

const (
	TaskReady   TaskStatus = "TaskReady"
	TaskRunnnig TaskStatus = "TaskRunnnig"
	TaskExited  TaskStatus = "TaskExited"
)

type TaskManager struct {
	sync.RWMutex
	allTask   map[TaskID]*exec.Cmd
	taskStaus map[TaskID]TaskStatus
	resources *resourceState
}

// new taskmanager , and assume the machines the resource is [1.0,1.0]
func NewTaskManager() *TaskManager {
	var manager TaskManager
	manager.allTask = make(map[TaskID]*exec.Cmd)
	manager.taskStaus = make(map[TaskID]TaskStatus)
	manager.resources = newResourceState([ResourceNum]float32{1.0, 1.0})
	return &manager
}

// new taskmanager , and set the resource by user
func NewTaskManagerFrom(machineAttr [ResourceNum]float32) *TaskManager {
	var manager TaskManager
	manager.allTask = make(map[TaskID]*exec.Cmd)
	manager.taskStaus = make(map[TaskID]TaskStatus)
	manager.resources = newResourceState(machineAttr)
	return &manager
}

// get the max value of the resource, it equal to this node's capacity
func (manager *TaskManager) GetMaxRes() [ResourceNum]float32 {

	manager.RLock()
	defer manager.RUnlock()
	return decimals2floats(manager.resources.NodeCapacity)
}

// get how much resources used, it always less or equal than this node's capacity
func (manager *TaskManager) GetUsedRes() [ResourceNum]float32 {

	manager.RLock()
	defer manager.RUnlock()
	return decimals2floats(manager.resources.getUsedResource())
}

// get how much resources used, it's equal to (MaxRes - UsedRes)
func (manager *TaskManager) GetAvailRes() [ResourceNum]float32 {

	manager.RLock()
	defer manager.RUnlock()
	return decimals2floats(manager.resources.getAvailResource())
}

// preAlloc the task,first the task's resource will be reserved and then task will be waitting to run.
func (manager *TaskManager) PreAlloc(t *ts.Task) error {
	return manager.preAlloc(TaskID(t.Id), [2]float32{t.Cpu, t.Ram}, t.Command)
}

// cancel the task which is waitting to run ,and cancel the its reserved resource
func (manager *TaskManager) CancelAlloc(t *ts.Task) error {
	return manager.cancelAlloc(TaskID(t.Id))
}

// run the task , after it finished the resource ,it will give back the  resource the task have
func (manager *TaskManager) RunTask(t *ts.Task) error {
	go logging.NoteLogQueue(time.Now().Format(time.RFC3339Nano), config.Run, t.Id, config.GetOutBoundIP(), t.Cpu, t.Ram)
	return manager.runTask(TaskID(t.Id))
}

// for Print infomation
func (manager *TaskManager) String() string {
	manager.RLock()
	defer manager.RUnlock()

	var res string
	res += manager.resources.String()
	res += fmt.Sprintln(manager.allTask)
	res += fmt.Sprint(manager.taskStaus)
	return res
}

// ------------- below is privite ---------------------

func (manager *TaskManager) preAlloc(id TaskID, request [ResourceNum]float32, cmdstr string) error {
	manager.Lock()
	defer manager.Unlock()
	// create exec.Cmd
	if manager.resources.allocTaskResource(id, request) {
		excutable := "/bin/sh"
		args := []string{"-c",cmdstr+" >> task_output.log 2>&1"}
		cmd := exec.Command(excutable, args...)
		manager.allTask[id] = cmd
		manager.taskStaus[id] = TaskReady
		return nil
	} else {
		return errors.New("the resources is not enough")
	}
}

func (manager *TaskManager) cancelAlloc(id TaskID) error {
	manager.Lock()
	defer manager.Unlock()

	if _, ok := manager.allTask[id]; !ok {
		return errors.New("no task in the alloc table")
	}

	manager.resources.removeTaskResource(id)
	delete(manager.allTask, id)
	delete(manager.taskStaus, id)
	return nil
}

func (manager *TaskManager) runTask(id TaskID) error {
	manager.Lock()
	defer manager.Unlock()

	cmd, ok := manager.allTask[id]
	if !ok {
		return errors.New("this task is not existed")
	}

	err := cmd.Start()
	if err != nil {
		return err
	}

	if manager.taskStaus[id] == TaskRunnnig {
		return errors.New("the task is already running")
	}

	if manager.taskStaus[id] == TaskExited {
		return errors.New("the task is already existed")
	}
	manager.taskStaus[id] = TaskRunnnig

	go func() {
		cmd.Wait()

		manager.Lock()
		defer manager.Unlock()

		manager.resources.removeTaskResource(id)
		manager.taskStaus[id] = TaskExited

	}()
	return nil
}
