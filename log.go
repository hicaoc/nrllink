package main

import (
	"log"
	"os"
	"path/filepath"
	"time"
)

var logbuffer chan *deviceInfo

func saveLog() {

	var err error
	timer := time.NewTimer(timeUntilNext10Minutes())
	file := createFile()

	for {
		select {
		case logEntry, ok := <-logbuffer:
			if !ok {
				log.Println("日志缓存被关闭:", err)
			}
			if conf.System.CallLogPath != "" && file != nil {
				if _, err := file.WriteString(logEntry.String() + "\n"); err != nil {
					log.Println("写入日志文件失败:", err)
				}
			}
		case <-timer.C:
			// 每10分钟轮换日志文件
			if file != nil {
				if err := file.Close(); err != nil {
					log.Println("关闭日志文件失败:", err)
				}
			}
			file = createFile()
			if file == nil {
				log.Println("创建或打开日志文件失败:", err)
			}

			// 重新设置定时器
			timer.Reset(timeUntilNext10Minutes())

		}
	}

}

func createFile() *os.File {

	parserDate := time.Now().Format("20060102")
	logDir := filepath.Join(conf.System.CallLogPath, parserDate)
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		log.Println("创建日志目录失败:", err)
		return nil
	}

	fileName := filepath.Join(logDir, "calllog."+time.Now().Format("20060102150405")+".txt")
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Println("创建或打开日志文件失败:", err)
		return nil
	}

	log.Println("开始新的日志文件:", fileName)
	return file

}

func timeUntilNext10Minutes() time.Duration {
	now := time.Now()
	next := now.Truncate(time.Minute * 10).Add(time.Minute * 10)
	return time.Until(next)
}
