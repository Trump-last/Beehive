/*
  - @Autor: XLF
  - @Date: 2022-10-24 13:58:25

* @LastEditors: XLF
* @LastEditTime: 2023-06-25 15:08:27
  - @Description:
    配置文件
*/
package config

import (
	"log"
	"net"
	"strings"

	"github.com/spf13/pflag"
)

const (
	WORKNUM     int32  = 2             // 工作节点数量
	TIMEOUT     int32  = 500           // 超时时间
	LOGFILEPATH string = "log.txt"     // 日志文件路径
	TaskLog     string = "TaskLog.csv" // 分发任务日志
	ScheLog     string = "SchLog.csv"  // 调度决策日志
)

var (
	Cpu         float32 // 节点CPU数量
	Ram         float32 // 节点Ram数量
	Model       string  // 模式，暂时无用
	Nums        int     // 邻域节点数量，默认3，需要>=2
	Policy_TEST int     //测试用policy 0，1
	Policy_FWD  int     //转发policy选择
	Gcsip       string  //gcs的ip+port地址
	Etcd        string  //etcd的ip+port地址
)

const (
	Dispense           = 0
	DivideEnd          = 1
	Run                = 2
	StartDivide        = 3
	StartWander        = 4
	WanderGet          = 5
	DivideGet          = 6
	StartDivideConfirm = 7
	StartDivideError   = 8
	DivideConfirm      = 9
	DivideError        = 10
)

func init() {
	pflag.Float32VarP(&Cpu, "cpu", "c", 0.0, "输入该机器的cpu资源量")
	pflag.Float32VarP(&Ram, "ram", "r", 0.0, "输入该机器的ram资源量")
	pflag.StringVarP(&Model, "model", "m", "local", "输入工作模式，默认是local")
	pflag.StringVarP(&Gcsip, "Gcsip", "g", "127.0.0.1:50051", "输入gcs的ip:port地址")
	pflag.StringVarP(&Etcd, "etcd", "e", "121.40.99.204:31225", "输入etcd地址，格式为ip:port，使用方法-e ip:port")
	pflag.IntVarP(&Nums, "Num", "n", 2, "输入邻域节点数量")
	pflag.IntVarP(&Policy_TEST, "policy", "p", 0, "测试策略")
	pflag.IntVarP(&Policy_FWD, "FWDpolicy", "f", 0, "转发策略")
	pflag.Parse()
}
func GetOutBoundIP() string {
	conn, err := net.Dial("udp", "192.0.0.0:53")
	if err != nil {
		log.Fatal(err)
		return ""
	}
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	ip := strings.Split(localAddr.String(), ":")[0]
	//ip = "127.0.0.1" //因为是在本机测试，所以这里只能用回环地址
	return ip
}
