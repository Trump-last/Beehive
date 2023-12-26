/*
 * @Autor: XLF
 * @Date: 2022-12-07 16:38:53
 * @LastEditors: XLF
 * @LastEditTime: 2023-06-26 10:14:32
 * @Description:
 */
package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	lbproto "gitee.com/linfeng-xu/protofile/lbGrpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// 向loadbalance发送任务
	conn, _ := grpc.Dial("127.0.0.1:23512", grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer conn.Close()
	client := lbproto.NewLbServerClient(conn)
	stream, _ := client.GetTaskByStream(context.Background())
	for i := 0; i < 20; i++ {
		task := &lbproto.TaskBasicImf{
			Taskid:  strconv.Itoa(i),
			Taskram: float32(1),
			Taskcpu: float32(1),
			Command: "task:" + strconv.Itoa(i),
		}
		if err := stream.Send(task); err != nil {
			fmt.Println(err)
		}
		time.Sleep(time.Second * 1)
	}
	response, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatalf("Failed to receive response: %v", err)
	}
	// 处理响应
	fmt.Printf("Response: %v\n", response)
}
