package main

import (
	"log"
	"time"

	// 安全：不引入 net/http/pprof，避免 /debug/pprof 未授权暴露在业务端口
	//（内存 profile 可直接读出 JWT 密钥等敏感数据）

	"github.com/ipipdotnet/ipdb-go"
	"github.com/json-iterator/go/extra"
)

var dbip *ipdb.City

func main() {

	// fmt.Println("heck database support ipv4:", db.IsIPv4())     // check database support ip type
	// fmt.Println("check database support ip type:", db.IsIPv6()) // check database support ip type
	// fmt.Println("database build time:", db.BuildTime())         // database build time
	// fmt.Println("database support language:", db.Languages())   // database support language
	// fmt.Println("database support fields:", db.Fields())        // database support fields

	extra.RegisterFuzzyDecoders()

	conf.init()

	initTokenKey()

	var err error

	dbip, err = ipdb.NewCity(conf.System.IPfile)
	if err != nil {
		log.Fatal(err)
	}

	setQTH("MEETLY-201", "会议模式", qth{"会议模式", "MEETLY-201", time.Now(), "多人同时讲话"})

	db = getDB()

	updatedb()

	chatgptInit()

	initAllUserList()

	initPublicGroup()

	initAllDevList()

	go jsonhttp.init()
	go callWSHub.run()

	logbuffer = make(chan *deviceInfo, 1000)
	go saveLog()

	go cronGetWxToken()

	go checkdeviceOnline()

	go NewAPRS().OnLoad()

	//go findNRL()

	go startPlatformServerSync()

	udpServer()

}
