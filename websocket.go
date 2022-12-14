package main

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/net/websocket"
)

var wsConnPoll = make(map[string]*websocket.Conn, 20)

func upper(ws *websocket.Conn) {

	wsConnPoll[ws.RemoteAddr().String()] = ws

	var err error
	for {
		var reply string

		if err = websocket.Message.Receive(ws, &reply); err != nil {
			fmt.Println(time.Now(), err)
			continue
		}

		if err = websocket.Message.Send(ws, strings.ToUpper(reply)); err != nil {
			fmt.Println(time.Now(), err)
			continue
		}
	}
}
