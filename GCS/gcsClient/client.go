/*
 * @Autor: XLF
 * @Date: 2022-12-06 15:01:07
 * @LastEditors: XLF
 * @LastEditTime: 2023-09-05 11:28:02
 * @Description:
 */
package main

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	gcsproto "gitee.com/linfeng-xu/protofile/gcsGrpc"
)

func main() {
	//grpc客户端模拟测试
	conn, err := grpc.Dial("127.0.0.1:23511", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	client := gcsproto.NewGcsGrpcClient(conn)
	reply, err := client.GetNbh(context.Background(), &gcsproto.InitReq{Addr: "", Num: 3})
	if err != nil {
		panic(err)
	}
	for i, v := range reply.Nbh {
		fmt.Println(i, v)
	}
}
