/*
 * @Autor: XLF
 * @Date: 2022-10-26 14:30:02
 * @LastEditors: XLF
 * @LastEditTime: 2023-05-19 14:16:17
 * @Description:
 */
package main

import (
	config "scheduler4/Config"
	"scheduler4/Util/agent"
	"scheduler4/Util/logging"
	"scheduler4/Util/node"
	"sync"
)

func main() {
	go logging.Run()
	wg := sync.WaitGroup{}
	wg.Add(1)
	go node.InitNode(config.Cpu, config.Ram, &wg)
	wg.Wait()
	agent.Run()
}
