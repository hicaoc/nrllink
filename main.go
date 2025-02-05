package main

import "github.com/json-iterator/go/extra"

func main() {

	extra.RegisterFuzzyDecoders()

	conf.init()

	db = getDB()

	updatedb()

	initPublicGroup()
	initAllUserList()
	initAllDevList()

	go jsonhttp.init()

	logbuffer = make(chan *deviceInfo, 1000)
	go saveLog()

	udpServer()

}
