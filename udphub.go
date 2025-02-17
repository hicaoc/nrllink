package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

//var userlist = make(map[string]userinfo, 1000) //key 用户id

var userlist sync.Map // callid ,userinfo

var devCallsignSSIDMap = make(map[string]*deviceInfo, 1000) //在线设备CPUID列表

var limitChan = make(chan bool, 1)

var globelconn *net.UDPConn

type currentConnPool struct {
	UDPAddr       *net.UDPAddr
	lastVoiceTime time.Time
	lastCtlTime   time.Time
	allowCALLSSID string
	//lastVoiceTime time.Time
	devConnList map[string]*deviceInfo //key cpuid
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

			log.Printf("from %v, decode err % X:", remoteaddr, data[:n])
			continue
			//break
			// <-limitChan
			// return
		}

		callsignSSID := getCallsignSSID(nrl.CallSign, nrl.SSID)

		if dev, ok := devCallsignSSIDMap[callsignSSID]; ok {

			dev.udpAddr = nrl.UDPAddr
			dev.ISOnline = true
			//设备呼号有变更，更新下
			//dev.CallSign = nrl.CallSign
			//dev.SSID = nrl.SSID
			dev.LastPacketTime = nrl.timeStamp
			dev.Traffic = dev.Traffic + 42 + 48 + len(nrl.DATA)
			totalstats.Traffic = totalstats.Traffic + 42 + 48 + len(nrl.DATA)

			//  没有加入公共组的设备，使用用户内置连接池
			if dev.GroupID > 0 && dev.GroupID < 1000 {

				if u, okok := userlist.Load(dev.CallSign); okok {

					NRL21parser(nrl, data[:n], dev, conn, u.(*userinfo).Groups[dev.GroupID])
				} else {

					fmt.Println("dev:", dev, nrl)

				}

			} else {

				//否则使用公共群组连接池
				if p, ok := publicGroupMap[dev.GroupID]; ok {

					NRL21parser(nrl, data[:n], dev, conn, p)
				}
			}

		} else {

			//升级用，先用cpuid加载下设备尝试
			dev := getDeviceByCpuID(nrl.CPUID)

			if dev.ID > 0 {

				updateDeviceCallsignSSIDByCPuid(nrl.CallSign, nrl.CPUID, nrl.SSID)

				fmt.Println("dev updated:", dev, nrl)

			} else {

				//设备不存在，加入设备,并加入加入缺省0公共群组,需要保存呼号callsign

				err = addDevice(&deviceInfo{
					CallSignSSID: callsignSSID,
					CallSign:     nrl.CallSign,
					SSID:         nrl.SSID,
					CPUID:        nrl.CPUID,
					DevModel:     nrl.DevMode,
					udpAddr:      nrl.UDPAddr,
					ChanName:     make([]string, 8)})

				if err != nil {
					fmt.Println("add dev failed, ", err, '\n', nrl)
					break
				}
			}

			d := getDevice(nrl.CallSign, nrl.SSID)

			devCallsignSSIDMap[callsignSSID] = d

			if p, ok := publicGroupMap[d.GroupID]; ok {

				p.DevMap[d.ID] = d

				NRL21parser(nrl, data[:n], d, conn, p)

			}

		}

	}

	<-limitChan
}

func udpServer() {
	udpAddr, err := net.ResolveUDPAddr("udp", "0.0.0.0:"+conf.System.Port)
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

	log.Println("data parse server started on udp :", udpAddr, conf.System.Port)

	for {
		limitChan <- true

		udpProcess(conn)
	}
}

func NRL21parser(nrl *NRL21packet, packet []byte, dev *deviceInfo, conn *net.UDPConn, gp *group) {

	switch nrl.Type {

	case 0:
		//控制指令，用户远程控制设备
		fmt.Println("recived control commond ", nrl)
	case 1, 8:
		//1 语音消息，需要转发给群组内其它设备,
		//fmt.Println("recived G.711 voice ")
		// fmt.Println(connpool.allowDEV, n.CPUID, n.CallSign)

		if (dev.Status & 1) == 1 {

			return
		}

		td := nrl.timeStamp.Sub(dev.LastVoiceEndTime).Milliseconds()

		if td > 200 {
			dev.LastVoiceBeginTime = nrl.timeStamp
			logbuffer <- dev
			dev.Loged = true
		}

		dev.Loged = false

		dev.LastVoiceDuration = int(nrl.timeStamp.Sub(dev.LastVoiceBeginTime).Milliseconds())
		dev.LastVoiceEndTime = nrl.timeStamp

		dev.VoiceTime = dev.VoiceTime + 63
		totalstats.VoiceTime = totalstats.VoiceTime + 63

		if gp.connPool.allowCALLSSID != "" && gp.connPool.allowCALLSSID != dev.CallSignSSID {
			return
		}

		//dev.LastPacketTime = nrl.timeStamp
		dev.LastVoiceEndTime = nrl.timeStamp
		dev.LastCtlEndTime = nrl.timeStamp

		forwardVoice(nrl, packet, conn, gp)
	case 2:
		//心跳包，用于保存设备在线存活状态， 目前设备1s一次发送
		if !dev.Loged && nrl.timeStamp.Sub(dev.LastVoiceEndTime).Milliseconds() > 200 {
			logbuffer <- dev
			dev.Loged = true
		}

		if _, ok := gp.connPool.devConnList[nrl.UDPAddrStr]; ok {
			//kk.LastPacketTime = nrl.timeStamp

		} else {

			gp.connPool.devConnList[nrl.UDPAddrStr] = dev
			log.Printf("device %v-%v online group %v, %v", nrl.CallSign, nrl.SSID, gp.ID, dev.udpAddr)
		}

		for kkk, vv := range gp.connPool.devConnList {
			if nrl.timeStamp.Sub(vv.LastPacketTime) > 10*time.Second {
				log.Printf("device %v-%v timeout offline %v, %v", nrl.CallSign, nrl.SSID, gp.ID, vv.udpAddr)
				delete(gp.connPool.devConnList, kkk)
			}

			if kkk != vv.udpAddr.String() {
				delete(gp.connPool.devConnList, kkk)
			}
		}

		//如果设备没有携带型号，则使用用户指定的型号，不更新
		if nrl.DevMode != 0 {
			dev.DevModel = nrl.DevMode
		}

		//如何是服务器自己发出的和其他服务器连接的心跳包，则更新在线状态，不能继续转发
		// dev.udpsocket 这个值只有发出心跳包的设备用到
		if dev.udpSocket != nil {
			return
		}

		//原样回复心跳，ssid小于100的设备，尝试获取设备的配置参数
		if dev.DeviceParm == nil && dev.DevModel < 100 {
			conn.WriteToUDP(encodeDeviceParm(dev, 0x01), dev.udpAddr)
		} else {
			conn.WriteToUDP(packet, nrl.UDPAddr)
		}

	case 3:
		//读取设备的配置参数

		dev.DeviceParm = decodeControlPacket(nrl.DATA)

	case 4:

	case 5: //文本消息

		forwardMsg(nrl, packet, dev, conn, gp.connPool)

	case 6: //设备到设备控制通道

		if (dev.Status & 1) == 1 {

			return
		}

		if nrl.timeStamp.Sub(dev.LastCtlEndTime).Milliseconds() > 200 {
			dev.LastCtlBeginTime = nrl.timeStamp
		}
		dev.LastCtlDuration = int(nrl.timeStamp.Sub(dev.LastCtlBeginTime).Milliseconds())
		dev.LastCtlEndTime = nrl.timeStamp

		dev.CtlTime = dev.CtlTime + 63
		//totalstats.CtlTime = totalstats.CtlTime + 63

		if gp.connPool.allowCALLSSID != "" && nrl.CPUID != gp.connPool.allowCALLSSID {
			return
		}

		if _, ok := gp.connPool.devConnList[nrl.UDPAddrStr]; !ok {
			dev.udpAddr = nrl.UDPAddr
			gp.connPool.devConnList[nrl.UDPAddrStr] = dev
		}

		forwardCtl(nrl, packet, conn, gp)

	case 7: //设备端操作指令

		t := packet[48]

		switch t {

		case 1: //切换组指令

			groupid := int(binary.BigEndian.Uint32(packet[49:53]))

			fmt.Printf("dev:%v-%v change group to %v to %v, data:  % X \n", dev.CallSign, dev.SSID, dev.GroupID, groupid, packet)
			str, err := changeDevGroup(dev, groupid)

			if err != nil {
				log.Println("change group err:", err)
				conn.WriteToUDP(append(packet, (strconv.Itoa(groupid)+",error")...), nrl.UDPAddr)
			} else {
				conn.WriteToUDP(append(packet, str...), nrl.UDPAddr)
			}

		case 2: //获取组列表

			resp := getGroupListForDevice(packet)

			conn.WriteToUDP(resp, nrl.UDPAddr)
			fmt.Printf("dev:%v-%v download grouplist. size: %v\n", dev.CallSign, dev.SSID, len(resp))

		}

	default:
		fmt.Println("unknow data:", nrl.Type, nrl)
		//conn.WriteToUDP(packet, n.Addr)

	}

}

func forwardVoice(nrl *NRL21packet, packet []byte, conn *net.UDPConn, gp *group) {

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

			//报文转发给其它设备，不包含自己
			if nrl.UDPAddrStr != kk && (vv.Status&2) != 2 {

				if vv.DevModel == 200 {
					newpacket := NRL21replace200dev(vv.CallSign, vv.SSID, 2, 200, calculateCpuId(vv.CallSign+"-200"), packet)
					conn.WriteToUDP(newpacket, vv.udpAddr)

				} else {
					//fmt.Println("case 2 :", clientAddrStr)
					conn.WriteToUDP(packet, vv.udpAddr)
				}
			} else {
				//更新自己的时间
				//vv.LastPacketTime = nrl.timeStamp
				vv.LastVoiceEndTime = nrl.timeStamp
				//必须要更新下地址，防止用户端口变化
				// vv.UDPAddr = n.UDPAddr

			}

		}

	default: //3个或3个以上设备，只允许一个设备发送语音，其它接收

		// 如果当前有会话，并且会话结束时间没超过1秒， 那么不转发其它设备报文, 或者语音包的DCD/PTT标志是0的时候，代表设备可能打开的是监听模式，丢弃无效语音
		if (nrl.UDPAddrStr != gp.connPool.UDPAddr.String() && nrl.timeStamp.Sub(gp.connPool.lastVoiceTime) < 200*time.Millisecond) || nrl.Status&0x01 == 0 {

			if k, ok := gp.connPool.devConnList[nrl.UDPAddrStr]; ok {
				k.LastCtlEndTime = nrl.timeStamp
			}

			return
			//否则重新让新设备抢占语音权，并更新上次报文时间
		} else {
			gp.connPool.UDPAddr = nrl.UDPAddr
			gp.connPool.lastVoiceTime = nrl.timeStamp

		}

		for kk, vv := range gp.connPool.devConnList {
			// if nrl.timeStamp.Sub(vv.lastTime) > 10*time.Second {
			// 	log.Println("device timeout offline:", nrl.CallSign, "-", nrl.SSID, " ", kk)
			// 	delete(gp.connPool.devConnList, kk)
			// 	continue
			// }

			if nrl.UDPAddrStr != kk && (vv.Status&2) != 2 {

				if vv.DevModel == 200 {
					newpacket := NRL21replace200dev(vv.CallSign, vv.SSID, 2, 200, calculateCpuId(vv.CallSign+"-200"), packet)
					conn.WriteToUDP(newpacket, vv.udpAddr)

				} else {
					conn.WriteToUDP(packet, vv.udpAddr)
				}

			} else {
				//更新自己连接池的上次报文接收时间
				//vv.LastPacketTime = nrl.timeStamp
				vv.LastVoiceEndTime = nrl.timeStamp

			}
		}

	}

}

// 文本消息转发
func forwardMsg(n *NRL21packet, packet []byte, dev *deviceInfo, conn *net.UDPConn, connpool *currentConnPool) {

	clientAddrStr := n.UDPAddr.String()

	if _, ok := connpool.devConnList[clientAddrStr]; ok {

		// if clientAddrStr != currentClientAddr {
		// 	continue
		// }

	} else {

		dev.udpAddr = n.UDPAddr

		connpool.devConnList[clientAddrStr] = dev

	}

	for kk, vv := range connpool.devConnList {

		if clientAddrStr != kk {
			if vv.DevModel == 200 {
				newpacket := NRL21replace200dev(vv.CallSign, vv.SSID, 2, 200, calculateCpuId(vv.CallSign+"-200"), packet)
				conn.WriteToUDP(newpacket, vv.udpAddr)

			} else {
				conn.WriteToUDP(packet, vv.udpAddr)
			}

		}
	}

}

// forwardCtl forwardCtl
func forwardCtl(nrl *NRL21packet, packet []byte, conn *net.UDPConn, gp *group) {

	switch len(gp.connPool.devConnList) {

	case 0:
		log.Println("err connpoll is null")
	case 1: //只有一个设备，缺省为环路测试，报文原样返回
		//fmt.Println("case 1 :", clientAddrStr)
		conn.WriteToUDP(packet, nrl.UDPAddr)
		gp.connPool.UDPAddr = nrl.UDPAddr
		gp.connPool.lastCtlTime = nrl.timeStamp

	case 2: //如果有2个设备，缺省为全双工通信，报文转发给对方

		for kk, vv := range gp.connPool.devConnList {
			//删除超时的会话

			//报文转发给其它设备，不包含自己
			if nrl.UDPAddrStr != kk && (vv.Status&2) != 2 {
				//fmt.Println("case 2 :", clientAddrStr)
				if vv.DevModel == 200 {
					newpacket := NRL21replace200dev(vv.CallSign, vv.SSID, 2, 200, calculateCpuId(vv.CallSign+"-200"), packet)
					conn.WriteToUDP(newpacket, vv.udpAddr)

				} else {
					conn.WriteToUDP(packet, vv.udpAddr)
				}

			} else {
				//更新自己的时间
				//vv.LastPacketTime = nrl.timeStamp
				vv.LastCtlEndTime = nrl.timeStamp
				//必须要更新下地址，防止用户端口变化
				// vv.UDPAddr = n.UDPAddr

			}

		}

	default: //3个或3个以上设备，只允许一个设备发送语音，其它接收

		// 如果当前有会话，并且会话结束时间没超过1秒， 那么不转发其它设备报文, 或者语音包的DCD/PTT标志是0的时候，代表设备可能打开的是监听模式，丢弃无效语音
		if (nrl.UDPAddrStr != gp.connPool.UDPAddr.String() && nrl.timeStamp.Sub(gp.connPool.lastCtlTime) < 200*time.Millisecond) || nrl.Status&0x01 == 0 {

			if k, ok := gp.connPool.devConnList[nrl.UDPAddrStr]; ok {
				k.LastCtlEndTime = nrl.timeStamp
			}

			// if nrl.CallSign == "BH4TDV" {
			// 	fmt.Println("*****return", gp.connPool.devConnList)
			// }

			return
			//否则重新让新设备抢占语音权，并更新上次报文时间
		} else {
			gp.connPool.UDPAddr = nrl.UDPAddr
			gp.connPool.lastCtlTime = nrl.timeStamp

		}

		for kk, vv := range gp.connPool.devConnList {
			// if nrl.timeStamp.Sub(vv.lastTime) > 5*time.Second {
			// 	log.Println("device timeout offline:", nrl.CallSign, "-", nrl.SSID, " ", kk)
			// 	delete(gp.connPool.devConnList, kk)
			// 	continue
			// }

			if nrl.UDPAddrStr != kk && (vv.Status&2) != 2 {

				if vv.DevModel == 200 {
					newpacket := NRL21replace200dev(vv.CallSign, vv.SSID, 2, 200, calculateCpuId(vv.CallSign+"-200"), packet)
					conn.WriteToUDP(newpacket, vv.udpAddr)

				} else {
					conn.WriteToUDP(packet, vv.udpAddr)
				}

			} else {
				//更新自己连接池的上次报文接收时间
				//vv.LastPacketTime = nrl.timeStamp
				vv.LastCtlEndTime = nrl.timeStamp

			}
		}

	}

}
