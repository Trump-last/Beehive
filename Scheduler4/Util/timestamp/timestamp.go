/*
  - @Autor: XLF
  - @Date: 2022-11-22 13:10:18
  - @LastEditors: XLF
  - @LastEditTime: 2022-11-22 13:10:18
  - @Description:
    时间戳工具文件，时间输入模板是2006-01-02 15:04:05；时间戳输出的是Unix时间戳
*/
package timestamp

import (
	"time"
)

// 字符串转换成时间戳，输入的时间格式必须是：2006-01-02 15:04:05，返回的是Unix时间戳
func StrToTime(str string) int64 {
	loc, _ := time.LoadLocation("Local")
	theTime, _ := time.ParseInLocation("2006-01-02 15:04:05", str, loc)
	sr := theTime.Unix()
	return sr
}

// 时间戳转换成字符串，输入的时间戳是Unix时间戳，返回的时间格式是：2006-01-02 15:04:05
func TimeToStr(timestamp int64) string {
	timeLayout := "2006-01-02 15:04:05" //转化所需模板
	dataTimeStr := time.Unix(timestamp, 0).Format(timeLayout)
	return dataTimeStr
}
