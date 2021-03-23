package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/json-iterator/go/extra"
)

var userlist = make(map[int]userinfo, 1000) //key 用户id

var devCPUIDMap = make(map[string]*deviceInfo, 1000) //在线设备CPUID列表

var limitChan = make(chan bool, 1)

type connPool struct {
	UDPAddr  *net.UDPAddr
	lastTime time.Time
}

type currentConnPool struct {
	UDPAddr     *net.UDPAddr
	lastTime    time.Time
	devConnList map[string]*connPool //key cpuid
}

func main() {

	extra.RegisterFuzzyDecoders()

	conf.init()

	db = getDB()

	initPublicGroup()
	initAllUserList()
	initAllDevList()

	go jsonhttp.init()

	udpServer()

}

func udpProcess(conn *net.UDPConn) {

	data := make([]byte, 1460)

	for {
		n, remoteaddr, err := conn.ReadFromUDP(data)
		if err != nil {
			fmt.Println("failed read udp msg, error: ", err)
			break
		}

		nrl := &NRL21packet{}
		nrl.UDPAddr = remoteaddr

		err = nrl.decodeNRL21(data[:n])
		totalstats.PacketNumber++

		if err != nil {

			log.Println("decode err:", data[:n])
			continue
			//break
			// <-limitChan
			// return
		}

		//fmt.Println(remoteaddr.String(), nrl.CPUID)

		if dev, ok := devCPUIDMap[string(nrl.CPUID)]; ok {

			dev.CallSign = nrl.CallSign
			dev.SSID = nrl.SSID
			dev.ISOnline = true

			//  绑定后，没有加入公共组的设备，使用用户内置连接池
			if dev.OwerID != 0 && dev.PublicGroupID == 0 {
				if u, okok := userlist[dev.OwerID]; okok {
					NRL21parser(nrl, data[:n], dev, conn, u.ConnPoll[dev.GroupID])
				}

			} else {
				//否则使用公共群组连接池
				if p, ok := publicGroupMap[dev.PublicGroupID]; ok {
					NRL21parser(nrl, data[:n], dev, conn, p.connPool)
				}
			}

		} else {
			//设备不存在，加入设备,并加入加入缺省0好公共群组
			newdev := &deviceInfo{
				ID:         0,
				CPUID:      string(nrl.CPUID),
				CallSign:   string(nrl.CallSign),
				SSID:       nrl.SSID,
				ISOnline:   true,
				OnlineTime: time.Now(),
			}

			devCPUIDMap[string(nrl.CPUID)] = newdev

			if p, ok := publicGroupMap[0]; ok {

				NRL21parser(nrl, data[:n], newdev, conn, p.connPool)

			}

		}

	}

	<-limitChan
}

func udpServer() {
	udpAddr, err := net.ResolveUDPAddr("udp", "0.0.0.0:"+conf.udpport)
	if err != nil {
		fmt.Println(" udp addr or port err:" + err.Error())
		os.Exit(1)
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	//conn.SetReadBuffer(5000)

	if err != nil {
		fmt.Println("read from connect failed, err:" + err.Error())
		os.Exit(1)
	}

	defer conn.Close()

	log.Println("data parse server started on udp :", udpAddr, conf.udpport)

	for {
		limitChan <- true

		udpProcess(conn)
	}
}

func NRL21parser(n *NRL21packet, packet []byte, devinfo *deviceInfo, conn *net.UDPConn, connpool *currentConnPool) {

	clientAddrStr := n.UDPAddr.String()

	if _, ok := connpool.devConnList[clientAddrStr]; !ok {
		connpool.devConnList[clientAddrStr] = &connPool{n.UDPAddr, time.Now()}

	}

	//fmt.Println(nrl)

	switch n.Type {

	case 0:
		//控制指令，用户远程控制设备
		fmt.Println("recived control commond ", n)
	case 1:
		//语音消息，需要转发给群组内其它设备,
		//fmt.Println("recived G.711 voice ")

		forwardVoice(n, packet, devinfo, conn, connpool)
	case 2:
		//心跳包，用于保存设备在线存活状态， 目前设备60ms一次发送，后期需要优化成60秒以上一次
		// if bytes.Contains([]byte{0x55}, n.DATA) {
		// 	fmt.Println("recived hb")
		// }
		if k, ok := connpool.devConnList[n.UDPAddr.String()]; ok {
			k.lastTime = time.Now()
		}
		//原样回复心跳
		conn.WriteToUDP(packet, n.UDPAddr)
	case 3:
		//数据是用户可以的MD5值，设备开机第一次连接时发送，服务器通过这个值和CPUID验证合法用户
		fmt.Println("recive online package :", string(n.DATA))

	case 4:

	case 5:

		forwardMsg(n, packet, devinfo, conn, connpool)

	default:
		fmt.Println("unknow data:", n)
		//conn.WriteToUDP(packet, n.Addr)

	}

}

func forwardVoice(n *NRL21packet, packet []byte, devinfo *deviceInfo, conn *net.UDPConn, connpool *currentConnPool) {

	clientAddrStr := n.UDPAddr.String()
	currentTime := time.Now()

	switch len(connpool.devConnList) {

	case 0:
		log.Println("err connpoll is null")
	case 1: //只有一个设备，缺省为环路测试，报文原样返回
		//fmt.Println("case 1 :", clientAddrStr)
		conn.WriteToUDP(packet, n.UDPAddr)
		connpool.UDPAddr = n.UDPAddr
		connpool.lastTime = currentTime
	case 2: //如果有2个设备，缺省为全双工通信，报文转发给对方
		for kk, vv := range connpool.devConnList {
			//删除超时的会话
			//fmt.Println(currentTime.Sub(vv.lasttime))
			if currentTime.Sub(vv.lastTime) > 5*time.Second {
				log.Println("device timeout offline:", kk)
				delete(connpool.devConnList, kk)
				continue
			}
			//报文转发给其它设备，不包含自己
			if clientAddrStr != kk {
				//fmt.Println("case 2 :", clientAddrStr)
				conn.WriteToUDP(packet, vv.UDPAddr)
			} else {
				//更新连接池的上次报文接收时间
				vv.lastTime = currentTime

				vv.UDPAddr = n.UDPAddr
				vv.lastTime = currentTime

			}
		}

	default: //3个或3个以上设备，只允许一个设备发送语音，其它接收

		// 如果当前有会话，并且会话结束时间没超过1秒， 那么不转发其它设备报文
		if clientAddrStr != connpool.UDPAddr.String() && currentTime.Sub(connpool.lastTime) < 200*time.Millisecond {

			if k, ok := connpool.devConnList[clientAddrStr]; ok {
				k.lastTime = currentTime
				connpool.devConnList[clientAddrStr] = k
			}
			//fmt.Println("yes: ", clientAddrStr, currentClientAddr.UDPAddr, currentTime.Sub(currentClientAddr.lasttime))
			return
			//否则重新让新设备抢占语音权，并更新上次报文时间
		} else {
			connpool.UDPAddr = n.UDPAddr
			connpool.lastTime = currentTime
		}

		//fmt.Println("no: ", clientAddrStr, currentClientAddr.UDPAddr, currentTime.Sub(currentClientAddr.lasttime))

		for kk, vv := range connpool.devConnList {
			if currentTime.Sub(vv.lastTime) > 5*time.Second {
				//fmt.Println("timeout  ", vv.UDPAddr, "  ", currentTime.Sub(vv.lasttime))
				log.Println("device timeout offline:", kk)
				delete(connpool.devConnList, kk)
				continue
			}

			if clientAddrStr != kk {
				conn.WriteToUDP(packet, vv.UDPAddr)
			} else {
				//更新连接池的上次报文接收时间
				vv.lastTime = currentTime
				connpool.devConnList[clientAddrStr] = vv

			}
		}

	}

}

func forwardMsg(n *NRL21packet, packet []byte, devinfo *deviceInfo, conn *net.UDPConn, connpool *currentConnPool) {

	clientAddrStr := n.UDPAddr.String()

	if _, ok := connpool.devConnList[clientAddrStr]; ok {

		// if clientAddrStr != currentClientAddr {
		// 	continue
		// }

	} else {
		connpool.devConnList[clientAddrStr] = &connPool{n.UDPAddr, time.Now()}

	}

	for kk, vv := range connpool.devConnList {

		if clientAddrStr != kk {
			conn.WriteToUDP(packet, vv.UDPAddr)
		}
	}

}
