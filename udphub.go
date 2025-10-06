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

var devCallsignSSIDMap = make(map[string]*deviceInfo, 1000) //key : callsign+ssid 在线设备CPUID列表

var ServerMap = make(map[string]*deviceInfo) //呼号对应的服务器设备

var limitChan = make(chan bool, 1)

var globelconn *net.UDPConn

type qth struct {
	QTH          string
	CallSignSSID string
	JoinTime     time.Time
}

var QTHmapNew = make(map[string]qth) // callsign+ssid
var QTHmap = make(map[string]string)

type currentConnPool struct {
	UDPAddr       *net.UDPAddr
	lastVoiceTime time.Time
	lastCtlTime   time.Time
	lastPriority  int

	//allowCALLSSID []string
	devConnMap  map[string]*deviceInfo //key udpaddr
	devConnList []*deviceInfo
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

	//启动服务器互联
	for _, v := range queryServers() {
		if v.Status == 1 {
			v.Start()
		}
	}

	log.Println("data parse server started on udp :", udpAddr, conf.System.Port)

	for {
		limitChan <- true

		udpProcess(conn)
	}
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

			log.Printf("from %v, decode err %v  % X:", remoteaddr, err, data[:n])
			continue
			//break
			// <-limitChan
			// return
		}

		callsignSSID := getCallsignSSID(nrl.CallSign, nrl.SSID)

		if dev, ok := devCallsignSSIDMap[callsignSSID]; ok {

			//dev.udpAddr = nrl.UDPAddr
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
			// dev := getDeviceByCpuID(nrl.CPUID)

			// if dev.ID > 0 {

			// 	updateDeviceCallsignSSIDByCPuid(nrl.CallSign, nrl.CPUID, nrl.SSID)

			// 	fmt.Println("dev updated:", dev, nrl)

			// } else {

			//设备不存在，加入设备,并加入加入缺省0公共群组,需要保存呼号callsign

			err = addDevice(&deviceInfo{
				CallSignSSID: callsignSSID,
				CallSign:     nrl.CallSign,
				SSID:         nrl.SSID,
				CPUID:        nrl.CPUID,
				DevModel:     nrl.DevMode,
				Priority:     100,
				//udpAddr:      nrl.UDPAddr,
				ChanName: make([]string, 8)})

			if err != nil {
				fmt.Println("add dev failed, ", err, '\n', nrl)
				continue
			}
			//}

			d := getDevice(nrl.CallSign, nrl.SSID)

			devCallsignSSIDMap[callsignSSID] = d

			if p, ok := publicGroupMap[d.GroupID]; ok {

				p.DevMap[d.ID] = d

				NRL21parser(nrl, data[:n], d, conn, p)

			} else {

				publicGroupMap[0].DevMap[d.ID] = d

				NRL21parser(nrl, data[:n], d, conn, publicGroupMap[0])

			}

		}

	}

	<-limitChan
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

		//设备状态为禁发

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

		// if gp.connPool.allowCALLSSID != "" && gp.connPool.allowCALLSSID != dev.CallSignSSID {
		// 	return
		// }

		//dev.LastPacketTime = nrl.timeStamp
		dev.LastVoiceEndTime = nrl.timeStamp
		dev.LastCtlEndTime = nrl.timeStamp

		forwardVoice(nrl, dev, packet, conn, gp)
	case 2:

		//处理服务器转发的定制心跳，不能响应回去，会循环
		if len(packet) == 52 {

			dev.QTH = getQTH(net.IP(packet[48:]).String())

			log.Printf("forward dev online:%v %v %v-%v %v\n", nrl.UDPAddrStr, net.IP(packet[48:]).String(), dev.CallSign, dev.SSID, dev.QTH)
			return

		}

		dev.udpAddr = nrl.UDPAddr

		//心跳包，用于保存设备在线存活状态， 目前设备1s一次发送
		if !dev.Loged && nrl.timeStamp.Sub(dev.LastVoiceEndTime).Milliseconds() > 200 {
			logbuffer <- dev
			dev.Loged = true
		}

		//判断设备是否已经在组内，有可能设备网络重新连接过，udp端口号变化过，需要重新加入组内
		if _, ok := gp.connPool.devConnMap[nrl.UDPAddrStr]; !ok {

			//如果设备新地址不在组内，需要先删除之前的设备地址key

			gp.connPool.devConnMap[nrl.UDPAddrStr] = dev

			//非常重要，非常重要，如果没有，设备list就没有初始化
			gp.connPool.devConnList = append(gp.connPool.devConnList, dev)

			//如果是200设备，将设备保存在servermap
			if nrl.SSID == 200 {
				ServerMap[nrl.CallSign] = dev
			}

		}

		//如何是服务器自己发出的和其他服务器连接的心跳包，则更新在线状态，不能继续转发
		// dev.udpsocket 这个值只有发出心跳包的设备用到
		if dev.udpSocket == nil {

			if dev.DeviceParm == nil && dev.DevModel < 100 {
				conn.WriteToUDP(encodeDeviceParm(dev, 0x01), dev.udpAddr)
			} else {
				conn.WriteToUDP(packet, nrl.UDPAddr)
			}

		}

		if !dev.ISOnline {

			//如果设备没有携带型号，则使用用户指定的型号，不更新
			if nrl.DevMode != 0 {
				dev.DevModel = nrl.DevMode
			}

			//收到200设备第一次上线，并且是透明模式，将所有设备信息的IP信息通过心跳发给对方服务器
			// if dev.SSID == 200 && (dev.Status&4 == 4) {
			// 	for _, vv := range devCallsignSSIDMap {
			// 		if vv.ISOnline {

			// 			bytes, err := hex.DecodeString(vv.CPUID)
			// 			if err != nil {
			// 				bytes = []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
			// 			}

			// 			pack := encodeNRL21(vv.CallSign, vv.SSID, 2, vv.DevModel, bytes, vv.udpAddr.IP.To4())
			// 			conn.WriteToUDP(pack, dev.udpAddr)
			// 			//fmt.Println(pack)
			// 		}

			// 	}
			// } else {

			// 	//200设备手工上线

			// 	//将普通新上线的设备心跳附加IPv4地址转发给所有200的服务器
			// 	for _, vv := range ServerMap {
			// 		if vv.udpAddr != nil && vv.ISOnline && (vv.Status&4 == 4) {
			// 			p := append(packet, nrl.UDPAddr.IP.To4()...)
			// 			conn.WriteToUDP(p, vv.udpAddr)
			// 			log.Printf("forward hb packet: %v %v %v \n", len(p), vv.udpAddr.String(), nrl.UDPAddr.IP.To4())
			// 		}

			// 	}

			// }

			//查询设备qth信息

			dev.QTH = getQTH(dev.udpAddr.IP.String())
			QTHmap[dev.CallSignSSID] = dev.QTH
			QTHmapNew[dev.CallSignSSID] = qth{dev.CallSignSSID, dev.QTH, time.Now()}

			log.Printf("dev online:%v %v-%v %v  group %v \n", dev.udpAddr.String(), dev.CallSign, dev.SSID, dev.QTH, gp.ID)

			dev.ISOnline = true

		}

		//原样回复心跳，ssid小于100的设备，尝试获取设备的配置参数

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

		// if gp.connPool.allowCALLSSID != "" && nrl.CPUID != gp.connPool.allowCALLSSID {
		// 	return
		// }

		if _, ok := gp.connPool.devConnMap[nrl.UDPAddrStr]; !ok {
			dev.udpAddr = nrl.UDPAddr
			gp.connPool.devConnMap[nrl.UDPAddrStr] = dev
		}

		forwardCtl(nrl, packet, conn, gp)

	case 7: //设备端操作指令

		t := packet[48]

		switch t {

		case 1: //切换组指令

			groupid := int(binary.BigEndian.Uint32(packet[49:53]))

			log.Printf("dev:%v-%v change group to %v to %v, data:  % X \n", dev.CallSign, dev.SSID, dev.GroupID, groupid, packet)
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
			log.Printf("dev:%v-%v download grouplist. size: %v\n", dev.CallSign, dev.SSID, len(resp))

		}
		//服务器互联
	case 9:

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

		// if gp.connPool.allowCALLSSID != "" && gp.connPool.allowCALLSSID != dev.CallSignSSID {
		// 	return
		// }

		//保存外部设备信息，用于解析QTH

		callsignssid := getCallsignSSID(nrl.OriginalCallsign, nrl.OriginalSSID)

		if q, ok := QTHmapNew[callsignssid]; !ok {
			update200QTH(callsignssid, nrl)

		} else if time.Since(q.JoinTime) > 10*time.Minute {
			update200QTH(callsignssid, nrl)

		}

		//dev.LastPacketTime = nrl.timeStamp
		dev.LastVoiceEndTime = nrl.timeStamp
		dev.LastCtlEndTime = nrl.timeStamp
		forwardServerVoice(nrl, packet, conn, gp)

	default:
		fmt.Println("unknow data:", nrl.Type, nrl)
		//conn.WriteToUDP(packet, n.Addr)

	}

}

func update200QTH(callsignssid string, nrl *NRL21packet) {

	q := getCallsignSSID(nrl.CallSign, nrl.SSID) + "-" + getQTH(nrl.OriginalIP.String())
	QTHmap[callsignssid] = q
	QTHmapNew[callsignssid] = qth{callsignssid, q, time.Now()}

}

func forwardVoice(nrl *NRL21packet, dev *deviceInfo, packet []byte, conn *net.UDPConn, gp *group) {

	numbs := len(gp.connPool.devConnMap)

	//房间类型为中继互联的时候，使用不允许出现双工
	if gp.Type == 1 {
		numbs = 3
	}

	switch numbs {

	case 0:
		//log.Println("err connpoll is null")
		return
	case 1: //只有一个设备，缺省为环路测试，报文原样返回
		//fmt.Println("case 1 :", clientAddrStr)
		conn.WriteToUDP(packet, nrl.UDPAddr)
		gp.connPool.UDPAddr = nrl.UDPAddr
		gp.connPool.lastVoiceTime = nrl.timeStamp

	case 2: //如果有2个设备，缺省为全双工通信，报文转发给对方

		for _, vv := range gp.connPool.devConnMap {

			//报文转发给其它设备，不包含自己
			if vv.udpAddr != nil && nrl.UDPAddrStr != vv.udpAddr.String() && ((vv.Status & 2) != 2) {

				if vv.DevModel == 200 && ((vv.Status & 4) != 4) {
					newpacket := NRL21replace200dev(vv.CallSign, vv.SSID, 9, 200, nrl.CallSign, nrl.SSID, nrl.UDPAddr.IP.To4(), calculateCpuId(vv.CallSign+"-200"), packet)
					conn.WriteToUDP(newpacket, vv.udpAddr)

				} else {
					conn.WriteToUDP(packet, vv.udpAddr)
				}
			} else {
				//更新自己的时间
				vv.LastVoiceEndTime = nrl.timeStamp
			}

		}

	default: //3个或3个以上设备，只允许一个设备发送语音，其它接收

		//语音包的DCD/PTT标志是0的时候，代表设备可能打开的是监听模式，丢弃无效语音，
		if nrl.Status&0x01 == 0 {
			return
		}

		//如果设备的优先级小于上次语音包设备优先级，如果是自己的包，优先级相等，条件不符合，继续其他判断
		//如果当前有会话，并且会话结束时间没超过200毫秒， 那么不转发其它设备报文
		//如果上次语言发送者不等于当前语音发送者 并且，当前时间和上次语音时间间隔小于200毫秒 不转发设备过来的语音包
		if dev.Priority <= gp.connPool.lastPriority &&
			nrl.UDPAddrStr != gp.connPool.UDPAddr.String() &&
			nrl.timeStamp.Sub(gp.connPool.lastVoiceTime) < 200*time.Millisecond {

			dev.LastVoiceEndTime = nrl.timeStamp
			// if k, ok := gp.connPool.devConnMap[nrl.UDPAddrStr]; ok {
			// 	k.LastVoiceEndTime = nrl.timeStamp
			// }

			return

		} else {

			//否则重新让新设备抢占语音权，并更新上次报文时间

			gp.connPool.UDPAddr = nrl.UDPAddr
			gp.connPool.lastVoiceTime = nrl.timeStamp
			gp.connPool.lastPriority = dev.Priority

		}

		for _, vv := range gp.connPool.devConnList {
			// if nrl.timeStamp.Sub(vv.lastTime) > 10*time.Second {
			// 	log.Println("device timeout offline:", nrl.CallSign, "-", nrl.SSID, " ", kk)
			// 	delete(gp.connPool.devConnMap, kk)
			// 	continue
			// }

			if vv.udpAddr != nil && nrl.UDPAddrStr != vv.udpAddr.String() && (vv.Status&2) != 2 {

				if vv.DevModel == 200 {
					//普通设备发给200设备，需要将原始呼号和SSID放到协议头
					newpacket := NRL21replace200dev(vv.CallSign, vv.SSID, 9, 200, nrl.CallSign, nrl.SSID, nrl.UDPAddr.IP.To4(), calculateCpuId(vv.CallSign+"-200"), packet)
					conn.WriteToUDP(newpacket, vv.udpAddr)

				} else {
					//普通设备发给普通设备，直接转发
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
func forwardServerVoice(nrl *NRL21packet, packet []byte, conn *net.UDPConn, gp *group) {

	if ((nrl.UDPAddrStr != gp.connPool.UDPAddr.String()) && nrl.timeStamp.Sub(gp.connPool.lastVoiceTime) < 200*time.Millisecond) || nrl.Status&0x01 == 0 {

		if k, ok := gp.connPool.devConnMap[nrl.UDPAddrStr]; ok {
			k.LastVoiceEndTime = nrl.timeStamp
		}

		return
		//否则重新让新设备抢占语音权，并更新上次报文时间
	} else {

		gp.connPool.UDPAddr = nrl.UDPAddr
		gp.connPool.lastVoiceTime = nrl.timeStamp

	}

	for _, vv := range gp.connPool.devConnList {

		if vv.udpAddr != nil && nrl.UDPAddrStr != vv.udpAddr.String() && (vv.Status&2) != 2 {

			//转发给其他服务器-200设备，需要使用携带原始信息
			if vv.DevModel == 200 {
				newpacket := NRL21replace200dev(vv.CallSign, vv.SSID, 9, 200, nrl.OriginalCallsign, nrl.OriginalSSID, nrl.OriginalIP, []byte(nrl.CPUID), packet)
				conn.WriteToUDP(newpacket, vv.udpAddr)

			} else {
				//转发给普通设备，需要将原始信息替换协议头信息
				newpacket := NRL21replace200dev(nrl.OriginalCallsign, nrl.OriginalSSID, 1, 200, nrl.CallSign, nrl.SSID, nrl.OriginalIP, []byte(nrl.CPUID), packet)
				conn.WriteToUDP(newpacket, vv.udpAddr)
			}

		} else {

			vv.LastVoiceEndTime = nrl.timeStamp

		}
	}

}

// 文本消息转发
func forwardMsg(nrl *NRL21packet, packet []byte, dev *deviceInfo, conn *net.UDPConn, connpool *currentConnPool) {

	clientAddrStr := nrl.UDPAddr.String()

	if _, ok := connpool.devConnMap[clientAddrStr]; ok {

		// if clientAddrStr != currentClientAddr {
		// 	continue
		// }

	} else {

		dev.udpAddr = nrl.UDPAddr

		connpool.devConnMap[clientAddrStr] = dev

	}

	for kk, vv := range connpool.devConnMap {

		if clientAddrStr != kk {
			if vv.DevModel == 200 {
				newpacket := NRL21replace200dev(vv.CallSign, vv.SSID, 5, 200, nrl.CallSign, nrl.SSID, nrl.UDPAddr.IP.To4(), calculateCpuId(vv.CallSign+"-200"), packet)
				conn.WriteToUDP(newpacket, vv.udpAddr)

			} else {
				conn.WriteToUDP(packet, vv.udpAddr)
			}

		}
	}

}

// forwardCtl forwardCtl
func forwardCtl(nrl *NRL21packet, packet []byte, conn *net.UDPConn, gp *group) {

	numbs := len(gp.connPool.devConnMap)

	//房间类型为中继互联的时候，使用不允许出现双工
	if gp.Type == 1 {
		numbs = 3
	}

	switch numbs {

	case 0:
		//log.Println("err connpoll is null")
		return
	case 1: //只有一个设备，缺省为环路测试，报文原样返回
		//fmt.Println("case 1 :", clientAddrStr)
		conn.WriteToUDP(packet, nrl.UDPAddr)
		gp.connPool.UDPAddr = nrl.UDPAddr
		gp.connPool.lastCtlTime = nrl.timeStamp

	case 2: //如果有2个设备，缺省为全双工通信，报文转发给对方

		for kk, vv := range gp.connPool.devConnMap {
			//删除超时的会话

			//报文转发给其它设备，不包含自己
			if nrl.UDPAddrStr != kk && (vv.Status&2) != 2 {
				//fmt.Println("case 2 :", clientAddrStr)
				if vv.DevModel == 200 {
					return

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

			if k, ok := gp.connPool.devConnMap[nrl.UDPAddrStr]; ok {
				k.LastCtlEndTime = nrl.timeStamp
			}

			// if nrl.CallSign == "BH4TDV" {
			// 	fmt.Println("*****return", gp.connPool.devConnMap)
			// }

			return
			//否则重新让新设备抢占语音权，并更新上次报文时间
		} else {
			gp.connPool.UDPAddr = nrl.UDPAddr
			gp.connPool.lastCtlTime = nrl.timeStamp

		}

		for kk, vv := range gp.connPool.devConnMap {
			// if nrl.timeStamp.Sub(vv.lastTime) > 5*time.Second {
			// 	log.Println("device timeout offline:", nrl.CallSign, "-", nrl.SSID, " ", kk)
			// 	delete(gp.connPool.devConnMap, kk)
			// 	continue
			// }

			if vv.udpAddr != nil && nrl.UDPAddrStr != kk && (vv.Status&2) != 2 {

				if vv.DevModel == 200 {
					return

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
