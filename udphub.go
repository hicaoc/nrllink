package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/json-iterator/go/extra"
)

var userlist = make(map[string]userinfo, 1000) //key 用户id

var devCPUIDMap = make(map[string]*deviceInfo, 1000) //在线设备CPUID列表

var limitChan = make(chan bool, 1)

var globelconn *net.UDPConn

type connPool struct {
	UDPAddr       *net.UDPAddr
	lastTime      time.Time
	lastVoiceTime time.Time
}

type currentConnPool struct {
	UDPAddr       *net.UDPAddr
	lastVoiceTime time.Time
	allowCPUID    string
	//lastVoiceTime time.Time
	devConnList map[string]*connPool //key cpuid
}

func main() {

	extra.RegisterFuzzyDecoders()

	conf.init()

	db = getDB()

	initServers()
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
		nrl.UDPAddrStr = remoteaddr.String()
		nrl.timeStamp = time.Now()

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

		if dev, ok := devCPUIDMap[nrl.CPUID]; ok {

			dev.udpAddr = nrl.UDPAddr
			dev.LastPacketTime = nrl.timeStamp

			dev.Traffic = dev.Traffic + 42 + 48 + len(nrl.DATA)

			totalstats.Traffic = totalstats.Traffic + 42 + 48 + len(nrl.DATA)

			//  没有加入公共组的设备，使用用户内置连接池
			if dev.GroupID > 0 && dev.GroupID < 1000 {
				if u, okok := userlist[dev.CallSign]; okok {
					NRL21parser(nrl, data[:n], dev, conn, u.Groups[dev.GroupID])
				}

			} else {
				//否则使用公共群组连接池
				if p, ok := publicGroupMap[dev.GroupID]; ok {

					NRL21parser(nrl, data[:n], dev, conn, p)
				}
			}

		} else {

			//设备不存在，加入设备,并加入加入缺省0公共群组

			addDevice(&deviceInfo{CPUID: nrl.CPUID, ChanName: make([]string, 8)})

			d := getDevice(nrl.CPUID)

			devCPUIDMap[nrl.CPUID] = d

			if p, ok := publicGroupMap[0]; ok {

				p.DevMap[d.ID] = d

				NRL21parser(nrl, data[:n], d, conn, p)

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

	globelconn = conn

	log.Println("data parse server started on udp :", udpAddr, conf.udpport)

	for {
		limitChan <- true

		udpProcess(conn)
	}
}

func NRL21parser(nrl *NRL21packet, packet []byte, dev *deviceInfo, conn *net.UDPConn, gp *group) {

	//fmt.Println(nrl)

	switch nrl.Type {

	case 0:
		//控制指令，用户远程控制设备
		fmt.Println("recived control commond ", nrl)
	case 1:
		//语音消息，需要转发给群组内其它设备,
		//fmt.Println("recived G.711 voice ")
		// fmt.Println(connpool.allowDEV, n.CPUID, n.CallSign)

		if dev.Status == 1 {

			return
		}

		if nrl.timeStamp.Sub(dev.LastVoiceEndTime).Milliseconds() > 200 {
			dev.LastVoiceBeginTime = nrl.timeStamp
		}
		dev.LastVoiceDuration = int(nrl.timeStamp.Sub(dev.LastVoiceBeginTime).Milliseconds())
		dev.LastVoiceEndTime = nrl.timeStamp

		dev.VoiceTime = dev.VoiceTime + 63
		totalstats.VoiceTime = totalstats.VoiceTime + 63

		if gp.connPool.allowCPUID != "" && nrl.CPUID != gp.connPool.allowCPUID {
			return
		}

		if _, ok := gp.connPool.devConnList[nrl.UDPAddrStr]; !ok {
			gp.connPool.devConnList[nrl.UDPAddrStr] = &connPool{nrl.UDPAddr, nrl.timeStamp, nrl.timeStamp}
		}

		forwardVoice(nrl, packet, dev, conn, gp)
	case 2:
		//心跳包，用于保存设备在线存活状态， 目前设备60ms一次发送，后期需要优化成60秒以上一次

		if kk, ok := gp.connPool.devConnList[nrl.UDPAddrStr]; ok {
			kk.lastTime = nrl.timeStamp

		} else {
			gp.connPool.devConnList[nrl.UDPAddrStr] = &connPool{nrl.UDPAddr, nrl.timeStamp, time.Time{}}
		}
		//原样回复心跳
		conn.WriteToUDP(packet, nrl.UDPAddr)

		//设备端有bug，某些报文没有填充callsign
		if dev.CallSign != nrl.CallSign || dev.SSID != nrl.SSID {
			dev.CallSign = nrl.CallSign
			dev.SSID = nrl.SSID
			updateDevice(dev)
		}

		if !dev.ISOnline {
			dev.ISOnline = true
			conn.WriteToUDP(encodeDeviceParm(dev, 0x01), dev.udpAddr)
		}

	case 3:
		//控制报文
		fmt.Println(dev)
		dev.DeviceParm = decodeControlPacket(nrl.DATA)

	case 4:

	case 5:

		forwardMsg(nrl, packet, dev, conn, gp.connPool)

	default:
		fmt.Println("unknow data:", nrl)
		//conn.WriteToUDP(packet, n.Addr)

	}

}

func forwardVoice(nrl *NRL21packet, packet []byte, dev *deviceInfo, conn *net.UDPConn, gp *group) {

	switch len(gp.connPool.devConnList) {

	case 0:
		log.Println("err connpoll is null")
	case 1: //只有一个设备，缺省为环路测试，报文原样返回
		//fmt.Println("case 1 :", clientAddrStr)
		conn.WriteToUDP(packet, nrl.UDPAddr)
		gp.connPool.UDPAddr = nrl.UDPAddr
		gp.connPool.lastVoiceTime = nrl.timeStamp

	case 2: //如果有2个设备，缺省为全双工通信，报文转发给对方

		for kk, vv := range gp.connPool.devConnList {
			//删除超时的会话

			if nrl.timeStamp.Sub(vv.lastTime) > 5*time.Second {
				log.Println("device timeout offline:", nrl.CallSign, "-", nrl.SSID, " ", kk)
				delete(gp.connPool.devConnList, kk)
				continue
			}
			//报文转发给其它设备，不包含自己
			if nrl.UDPAddrStr != kk {
				//fmt.Println("case 2 :", clientAddrStr)
				conn.WriteToUDP(packet, vv.UDPAddr)
			} else {
				//更新自己的时间
				vv.lastTime = nrl.timeStamp
				vv.lastVoiceTime = nrl.timeStamp
				//必须要更新下地址，防止用户端口变化
				// vv.UDPAddr = n.UDPAddr

			}

		}

	default: //3个或3个以上设备，只允许一个设备发送语音，其它接收

		// 如果当前有会话，并且会话结束时间没超过1秒， 那么不转发其它设备报文, 或者语音包的DCD/PTT标志是0的时候，代表设备可能打开的是监听模式，丢弃无效语音
		if (nrl.UDPAddrStr != gp.connPool.UDPAddr.String() && nrl.timeStamp.Sub(gp.connPool.lastVoiceTime) < 200*time.Millisecond) || nrl.Status&0x01 == 0 {

			if k, ok := gp.connPool.devConnList[nrl.UDPAddrStr]; ok {
				k.lastVoiceTime = nrl.timeStamp
			}

			return
			//否则重新让新设备抢占语音权，并更新上次报文时间
		} else {
			gp.connPool.UDPAddr = nrl.UDPAddr
			gp.connPool.lastVoiceTime = nrl.timeStamp

		}

		for kk, vv := range gp.connPool.devConnList {
			if nrl.timeStamp.Sub(vv.lastTime) > 5*time.Second {
				log.Println("device timeout offline:", nrl.CallSign, "-", nrl.SSID, " ", kk)
				delete(gp.connPool.devConnList, kk)
				continue
			}

			if nrl.UDPAddrStr != kk {
				conn.WriteToUDP(packet, vv.UDPAddr)
			} else {
				//更新自己连接池的上次报文接收时间
				vv.lastTime = nrl.timeStamp
				vv.lastVoiceTime = nrl.timeStamp

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
		connpool.devConnList[clientAddrStr] = &connPool{n.UDPAddr, n.timeStamp, time.Time{}}

	}

	for kk, vv := range connpool.devConnList {

		if clientAddrStr != kk {
			conn.WriteToUDP(packet, vv.UDPAddr)
		}
	}

}
