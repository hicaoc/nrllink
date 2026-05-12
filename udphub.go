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

//dev model:
/*
 { id: 100, name: 'NRL-微信小程序' },
  { id: 101, name: 'NRL-安卓' },
  { id: 102, name: 'NRL-IOS' },
  { id: 103, name: 'NRL-Win' },
  { id: 105, name: 'NRL-浏览器' },
  { id: 106, name: 'NRL-救援' },

  { id: 200, name: 'NRL-Server' },
  { id: 201, name: 'NRL-BM' },
  { id: 250, name: 'NRL-保姆' }
  { id: 255, name: 'FULL NET' }

*/

//var userlist = make(map[string]userinfo, 1000) //key 用户id

var userlist sync.Map // callid ,userinfo

var mdcidmap sync.Map //key mdcid, value callsign

var dmridmap sync.Map //key dmrid, value callsign

var devCallsignSSIDMap = make(map[string]*deviceInfo, 1000) //key : callsign+ssid 在线设备列表

var onlinedevMap = make(map[int]*deviceInfo, 1000) //所有设备列表
//var offlineDevList devlist

var ServerMap = make(map[string]*deviceInfo) //呼号对应的服务器设备

var limitChan = make(chan bool, 1)

var globelconn *net.UDPConn

type qth struct {
	QTH          string    `json:"qth"`
	CallSignSSID string    `json:"callsign_ssid"`
	JoinTime     time.Time `json:"join_time"`
	Name         string    `json:"name"`
}

var (
	QTHmapNew = make(map[string]qth) // callsign+ssid
	QTHmap    = make(map[string]string)
	qthMu     sync.RWMutex
)

func setQTH(callsignSSID, oldQTH string, newQTH qth) {
	qthMu.Lock()
	defer qthMu.Unlock()

	QTHmap[callsignSSID] = oldQTH
	QTHmapNew[callsignSSID] = newQTH
}

func getQTHEntry(callsignSSID string) (qth, bool) {
	qthMu.RLock()
	defer qthMu.RUnlock()

	q, ok := QTHmapNew[callsignSSID]
	return q, ok
}

func snapshotQTHMap() map[string]string {
	qthMu.RLock()
	defer qthMu.RUnlock()

	res := make(map[string]string, len(QTHmap))
	for k, v := range QTHmap {
		res[k] = v
	}
	return res
}

func snapshotQTHMapNew() map[string]qth {
	qthMu.RLock()
	defer qthMu.RUnlock()

	res := make(map[string]qth, len(QTHmapNew))
	for k, v := range QTHmapNew {
		res[k] = v
	}
	return res
}

type currentConnPool struct {
	mu            sync.RWMutex
	UDPAddr       *net.UDPAddr
	lastVoiceTime time.Time
	lastCtlTime   time.Time
	lastPriority  int

	//allowCALLSSID []string
	devConnMap  map[string]*deviceInfo //key udpaddr
	devConnList []*deviceInfo
}

func (p *currentConnPool) count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.devConnMap)
}

func (p *currentConnPool) ensureDevice(addr string, dev *deviceInfo) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.devConnMap[addr]; ok {
		return false
	}

	p.devConnMap[addr] = dev
	p.devConnList = append(p.devConnList, dev)
	return true
}

func (p *currentConnPool) snapshotMap() map[string]*deviceInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	res := make(map[string]*deviceInfo, len(p.devConnMap))
	for k, v := range p.devConnMap {
		res[k] = v
	}
	return res
}

func (p *currentConnPool) snapshotList() []*deviceInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	res := make([]*deviceInfo, len(p.devConnList))
	copy(res, p.devConnList)
	return res
}

func (p *currentConnPool) getDevice(addr string) (*deviceInfo, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	dev, ok := p.devConnMap[addr]
	return dev, ok
}

func (p *currentConnPool) voiceState() (*net.UDPAddr, time.Time, int) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.UDPAddr, p.lastVoiceTime, p.lastPriority
}

func (p *currentConnPool) setVoiceState(addr *net.UDPAddr, ts time.Time, priority int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.UDPAddr = addr
	p.lastVoiceTime = ts
	p.lastPriority = priority
}

func (p *currentConnPool) ctlState() (*net.UDPAddr, time.Time) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.UDPAddr, p.lastCtlTime
}

func (p *currentConnPool) setCtlState(addr *net.UDPAddr, ts time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.UDPAddr = addr
	p.lastCtlTime = ts
}

func (p *currentConnPool) removeDevice(addr string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.devConnMap, addr)
	p.rebuildListLocked()
}

func (p *currentConnPool) rebuildListLocked() {
	list := make([]*deviceInfo, 0, len(p.devConnMap))
	for _, dev := range p.devConnMap {
		list = append(list, dev)
	}
	p.devConnList = list
}

func udpServer() {

	port, err := strconv.Atoi(conf.System.Port)
	if err != nil {
		fmt.Println(" udp port err:" + err.Error())
		os.Exit(1)

	}

	conn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.IPv4zero,
		Port: port,
	})

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

	log.Println("data parse server started on udp :", conf.System.Port)

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

		nrl, err := newNRL21packet(remoteaddr, data[:n])
		if err != nil {
			log.Printf("from %v, decode err %v  % X:", remoteaddr, err, data[:n])
			continue
		}

		totalstats.PacketNumber++

		callsignSSID := getCallsignSSID(nrl.CallSign, nrl.SSID)

		if dev, ok := devCallsignSSIDMap[callsignSSID]; ok {
			if nrl.Type == 2 && refreshDeviceAccountExpired(dev, nrl.timeStamp) {
				removeExpiredDeviceFromPool(dev, groupForDevice(dev))
				log.Printf("billing: expired account heartbeat ignored: %v-%v expire check at %v", dev.CallSign, dev.SSID, dev.LastExpireCheck.Format("2006-01-02 15:04:05"))
				continue
			}

			//dev.udpAddr = nrl.UDPAddr
			dev.LastPacketTime = nrl.timeStamp
			dev.Traffic = dev.Traffic + 42 + 48 + len(nrl.DATA)
			totalstats.Traffic = totalstats.Traffic + 42 + 48 + len(nrl.DATA)

			if nrl.DevModel != 200 {
				NRL21SetDevDMRID(dev.DMRID, data[:n])
			}

			//  没有加入公共组的设备，使用用户内置连接池
			if dev.GroupID > 0 && dev.GroupID <= 3 {

				if u, okok := userlist.Load(dev.CallSign); okok {

					NRL21parser(nrl, data[:n], dev, conn, u.(*userinfo).Groups[dev.GroupID])
				} else {

					fmt.Println("dev:", dev, nrl)

				}

			} else if dev.GroupID >= 999 || dev.GroupID == 0 {

				//否则使用公共群组连接池
				if p, ok := publicGroupMap[dev.GroupID]; ok {

					NRL21parser(nrl, data[:n], dev, conn, p)
				}
			} else {

				log.Printf("dev:%v, nrl:%v, dev.GroupID error:%v", dev, nrl, dev.GroupID)
			}

		} else {

			//设备不存在，加入设备,并加入加入缺省0公共群组,需要保存呼号callsign
			dd := &deviceInfo{
				CallSignSSID: callsignSSID,
				CallSign:     nrl.CallSign,
				SSID:         nrl.SSID,
				//DMRID:        nrl.DMRID, // 过渡期，暂时不从nrl报文中获取DMRID，盒子不支持，当前从服务器配置中获取
				DevModel: nrl.DevModel,
				Priority: 100,
				//udpAddr:      nrl.UDPAddr,
				ChanName:  make([]string, 16),
				pcmBuffer: make([]int, 160),
			}

			if nrl.DevModel == 255 {
				dd.GroupID = 999
			}

			err = addDevice(dd)

			if err != nil {
				log.Println("add dev failed, ", err, '\n', nrl)
			}

			d, err := getDevice(nrl.CallSign, nrl.SSID)

			if err != nil {
				log.Printf("Failed to get device for server %v-%d: %v", nrl.CallSign, nrl.SSID, err)
				continue
			}

			log.Println("add dev ok", dd.CallSign, dd.SSID, d.GroupID, "groupid:", dd.GroupID, dd.CallSign, dd.SSID, d)

			d.pcmBuffer = make([]int, 1000)

			devCallsignSSIDMap[callsignSSID] = d
			if nrl.Type == 2 && refreshDeviceAccountExpired(d, nrl.timeStamp) {
				log.Printf("billing: expired account new device heartbeat ignored: %v-%v", d.CallSign, d.SSID)
				continue
			}

			if p, ok := publicGroupMap[d.GroupID]; ok {

				p.devMap[d.ID] = d

				NRL21parser(nrl, data[:n], d, conn, p)

			} else {

				publicGroupMap[0].devMap[d.ID] = d

				NRL21parser(nrl, data[:n], d, conn, publicGroupMap[0])

			}

		}

	}

	<-limitChan
}

func groupForDevice(dev *deviceInfo) *group {
	if dev == nil {
		return nil
	}
	if dev.GroupID > 0 && dev.GroupID <= 3 {
		if u, ok := userlist.Load(dev.CallSign); ok {
			return u.(*userinfo).Groups[dev.GroupID]
		}
	}
	return publicGroupMap[dev.GroupID]
}

func NRL21parser(nrl *NRL21packet, packet []byte, dev *deviceInfo, conn *net.UDPConn, gp *group) {

	switch nrl.Type {

	case 0:
		//控制指令，用户远程控制设备
		fmt.Println("recived control commond ", nrl)
	case 1, 8:
		//1 语音消息，需要转发给群组内其它设备,
		//fmt.Println("recived G.711 voice ")

		//设备状态为禁发

		if (dev.Status & 1) == 1 {
			return
		}

		td := nrl.timeStamp.Sub(dev.LastVoiceEndTime).Milliseconds()

		if td > 200 {
			dev.LastVoiceBeginTime = nrl.timeStamp
			logbuffer <- dev
			dev.Loged = true
			if nrl.DevModel == 200 || (nrl.DevModel == 255 && nrl.SSID == 255) {
				callWSHub.trackCallStart(gp, nrl.OriginalCallsign, nrl.OriginalSSID, nrl.timeStamp)
			} else {
				callWSHub.trackCallStart(gp, dev.CallSign, dev.SSID, nrl.timeStamp)
			}
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

		//来自其他服务器255的包
		if nrl.DevModel == 255 && nrl.SSID == 255 {
			dev.ISOnline = true
			forwardServerVoice(nrl, dev, packet, conn, gp)
			return
		}

		if nrl.DevModel == 200 && nrl.SSID == 200 {
			forwardServerVoice(nrl, dev, packet, conn, gp)
			return
		}

		forwardVoice(nrl, dev, packet, gp)

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
		if gp.connPool.ensureDevice(nrl.UDPAddrStr, dev) && nrl.SSID == 200 {
			ServerMap[nrl.CallSign] = dev
		}

		//如何是服务器自己发出的和其他服务器连接的心跳包，则更新在线状态，不能继续转发
		// dev.udpsocket 这个值只有发出心跳包的设备用到
		if dev.udpSocket == nil {

			if dev.DeviceParm == nil && dev.DevModel < 100 {
				//发送查询设备参数
				conn.WriteToUDP(encodeNRL21(dev.CallSign, dev.SSID, 3, 0, 0, []byte{0x01}), dev.udpAddr)

			} else {
				conn.WriteToUDP(packet, nrl.UDPAddr)
			}

		}

		// 心跳包里带了设备型号时，持续同步，避免首次上线拿到 0 或旧值后一直不刷新。
		if nrl.DevModel != 0 {
			dev.DevModel = nrl.DevModel
		}

		if !dev.ISOnline {

			//查询设备qth信息

			dev.QTH = getQTH(dev.udpAddr.IP.String())
			setQTH(dev.CallSignSSID, dev.QTH, qth{dev.QTH, dev.CallSignSSID, time.Now(), dev.Name})

			log.Printf("dev online:%v %v-%v %v  group %v \n", dev.udpAddr.String(), dev.CallSign, dev.SSID, dev.QTH, gp.ID)

			if dev.DevModel != 200 {
				at := &ATcommand{CallSign: dev.CallSign, SSID: dev.SSID, Type: 0x01, ATcommand: "AT+READ", Data: "123"}
				DeviceAT(at)

			}

			dev.ISOnline = true

		}

		//原样回复心跳，ssid小于100的设备，尝试获取设备的配置参数

	case 3:
		//读取设备的配置参数

		dev.DeviceParm = decodeControlPacket(nrl.DATA)

	case 4:

	case 5: //文本消息,其他多媒体

		/**
			[text]adsfkafdaasdfasfd
		    [img]https://xxx.oss.aliyun.com/adafasdfasdfasdfasfd
		    [video]https://asfdfasdfasfdasdfasdfasdf
		    [file]https://adfasldfasdfasdfas//*
		*/

		forwardMsg(nrl, packet, dev, conn, gp.connPool)

	case 6, 10: //设备到设备控制通道

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

		// if _, ok := gp.connPool.devConnMap[nrl.UDPAddrStr]; !ok {
		// 	dev.udpAddr = nrl.UDPAddr
		// 	gp.connPool.devConnMap[nrl.UDPAddrStr] = dev
		// }

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
			if nrl.DevModel == 200 || (nrl.DevModel == 255 && nrl.SSID == 255) {
				callWSHub.trackCallStart(gp, nrl.OriginalCallsign, nrl.OriginalSSID, nrl.timeStamp)
			} else {
				callWSHub.trackCallStart(gp, dev.CallSign, dev.SSID, nrl.timeStamp)
			}
		}

		dev.Loged = false

		dev.LastVoiceDuration = int(nrl.timeStamp.Sub(dev.LastVoiceBeginTime).Milliseconds())
		dev.LastVoiceEndTime = nrl.timeStamp

		dev.VoiceTime = dev.VoiceTime + 20
		totalstats.VoiceTime = totalstats.VoiceTime + 20

		// if gp.connPool.allowCALLSSID != "" && gp.connPool.allowCALLSSID != dev.CallSignSSID {
		// 	return
		// }

		//dev.LastPacketTime = nrl.timeStamp
		dev.LastVoiceEndTime = nrl.timeStamp
		dev.LastCtlEndTime = nrl.timeStamp
		forwardServerVoice(nrl, dev, packet, conn, gp)

	case 11: //AT  01 读取，02写入多条 0x0d,0x0a分割，03读取所有，

		at := decodeATPacket(dev.CallSign, dev.SSID, nrl.DATA)
		//fmt.Println(at)
		dev.LastATcommand = at

	case 12: //COM 透传
		forwardCOM(nrl, packet, gp)

	default:
		fmt.Println("unknow data:", nrl.Type, nrl)
		//conn.WriteToUDP(packet, n.Addr)

	}

}

func update200QTH(Originalcallsignssid string, nrl *NRL21packet, dev *deviceInfo) {

	callsignssid := getCallsignSSID(nrl.CallSign, nrl.SSID)
	originalqth := getQTH(nrl.OriginalIP.String())

	setQTH(Originalcallsignssid, callsignssid+"-"+originalqth, qth{
		originalqth,
		Originalcallsignssid,
		time.Now(),
		callsignssid + "-" + dev.Name,
	})

}

func FullNetOutput(nrl *NRL21packet, dev *deviceInfo, packet []byte) {

	if conf.APRS.CallSign == "" {
		return
	}

	newpacket := NRL21replace200and255dev(conf.APRS.CallSign, 255, nrl.Type, 255, nrl.CallSign, nrl.SSID, nrl.UDPAddr.IP.To4(), dev.DMRID, packet)

	for _, v := range conf.PlatformList {

		if v.udpAddr != nil && v.Host != conf.APRS.SelfAddress {
			globelconn.WriteToUDP(newpacket, v.udpAddr)
		}

	}

}

func forwardCOM(nrl *NRL21packet, packet []byte, gp *group) {

	if gp.ID > 3 {
		return
	}

	for _, vv := range gp.connPool.snapshotMap() {

		//报文转发给其它设备，不包含自己
		if vv.udpAddr != nil && nrl.UDPAddrStr != vv.udpAddr.String() {

			if vv.DevModel == 200 {
				continue
			} else {
				globelconn.WriteToUDP(packet, vv.udpAddr)
			}
		} else {
			//更新自己的时间
			vv.LastVoiceEndTime = nrl.timeStamp
		}

	}

}

func forwardVoice(nrl *NRL21packet, dev *deviceInfo, packet []byte, gp *group) {

	numbs := gp.connPool.count()

	//房间类型为中继互联的时候，使用不允许出现双工
	if gp.Type == 1 || gp.ID == 999 {
		numbs = 3
	}

	//log.Println("PCMMIX: ", dev.CallSignSSID, "numbs", dev.GroupID, numbs, "type", gp.Type, "gp.ID", gp.ID, "gp.Name", gp.Name)

	switch numbs {

	case 0:
		//log.Println("err connpoll is null")
		return
	case 1: //只有一个设备，缺省为环路测试，报文原样返回

		//fmt.Println("case 1 :", clientAddrStr)
		globelconn.WriteToUDP(packet, nrl.UDPAddr)
		gp.connPool.setVoiceState(nrl.UDPAddr, nrl.timeStamp, dev.Priority)
		callWSHub.publishVoiceFrame(gp, dev.CallSign, dev.SSID, nrl.DATA, nrl.timeStamp)

	case 2: //如果有2个设备，缺省为全双工通信，报文转发给对方
		callWSHub.publishVoiceFrame(gp, dev.CallSign, dev.SSID, nrl.DATA, nrl.timeStamp)

		for _, vv := range gp.connPool.snapshotMap() {

			//报文转发给其它设备，不包含自己
			if vv.udpAddr != nil && nrl.UDPAddrStr != vv.udpAddr.String() && ((vv.Status & 2) != 2) {

				//普通设备转发给其他200和255设备

				if vv.DevModel == 200 && ((vv.Status & 4) != 4) {
					newpacket := NRL21replace200and255dev(vv.CallSign, vv.SSID, nrl.Type, 200, nrl.CallSign, nrl.SSID, nrl.UDPAddr.IP.To4(), dev.DMRID, packet)
					globelconn.WriteToUDP(newpacket, vv.udpAddr)

				} else {
					globelconn.WriteToUDP(packet, vv.udpAddr)
				}
			} else {
				//更新自己的时间
				vv.LastVoiceEndTime = nrl.timeStamp
			}

		}

	default: //3个或3个以上设备，只允许一个设备发送语音，其它接收

		//房间类型为会议室的时候，需要将语音进行混音，语音先放入缓存，等待其他设备的语音包
		if gp.Type == 7 && nrl.Type == 1 {

			// 直接追加到设备缓存，混音端按需取160字节
			dev.pcmMu.Lock()
			dev.pcmBuf = append(dev.pcmBuf, nrl.DATA...)
			dev.pcmMu.Unlock()
			return
		}

		//如果设备的优先级小于上次语音包设备优先级，如果是自己的包，优先级相等，条件不符合，继续其他判断
		//如果当前有会话，并且会话结束时间没超过200毫秒， 那么不转发其它设备报文
		//如果上次语言发送者不等于当前语音发送者 并且，当前时间和上次语音时间间隔小于200毫秒 不转发设备过来的语音包
		lastUDPAddr, lastVoiceTime, lastPriority := gp.connPool.voiceState()
		lastAddrStr := ""
		if lastUDPAddr != nil {
			lastAddrStr = lastUDPAddr.String()
		}

		if dev.Priority <= lastPriority &&
			nrl.UDPAddrStr != lastAddrStr &&
			nrl.timeStamp.Sub(lastVoiceTime) < 200*time.Millisecond {

			dev.LastVoiceEndTime = nrl.timeStamp
			// if k, ok := gp.connPool.devConnMap[nrl.UDPAddrStr]; ok {
			// 	k.LastVoiceEndTime = nrl.timeStamp
			// }

			return

		} else {

			//否则重新让新设备抢占语音权，并更新上次报文时间

			gp.connPool.setVoiceState(nrl.UDPAddr, nrl.timeStamp, dev.Priority)

		}
		callWSHub.publishVoiceFrame(gp, dev.CallSign, dev.SSID, nrl.DATA, nrl.timeStamp)

		for _, vv := range gp.connPool.snapshotList() {
			// if nrl.timeStamp.Sub(vv.lastTime) > 10*time.Second {
			// 	log.Println("device timeout offline:", nrl.CallSign, "-", nrl.SSID, " ", kk)
			// 	delete(gp.connPool.devConnMap, kk)
			// 	continue
			// }

			if vv.udpAddr != nil && nrl.UDPAddrStr != vv.udpAddr.String() && (vv.Status&2) != 2 {

				if vv.DevModel == 200 {
					//普通设备发给200设备，需要将原始呼号和SSID放到协议头
					newpacket := NRL21replace200and255dev(vv.CallSign, vv.SSID, nrl.Type, 200, nrl.CallSign, nrl.SSID, nrl.UDPAddr.IP.To4(), dev.DMRID, packet)
					globelconn.WriteToUDP(newpacket, vv.udpAddr)
					//这里不处理255设备
				} else if vv.DevModel == 255 || vv.SSID == 255 {
					continue

				} else {
					//普通设备发给普通设备，直接转发
					globelconn.WriteToUDP(packet, vv.udpAddr)
				}

			} else {

				//更新自己连接池的上次报文接收时间
				//vv.LastPacketTime = nrl.timeStamp
				vv.LastVoiceEndTime = nrl.timeStamp

			}
		}

		if gp.ID == 999 {
			FullNetOutput(nrl, dev, packet)

		}

	}

}
func forwardServerVoice(nrl *NRL21packet, dev *deviceInfo, packet []byte, conn *net.UDPConn, gp *group) {

	Originalcallsignssid := getCallsignSSID(nrl.OriginalCallsign, nrl.OriginalSSID)

	if q, ok := getQTHEntry(Originalcallsignssid); !ok {
		update200QTH(Originalcallsignssid, nrl, dev)

	} else if time.Since(q.JoinTime) > 10*time.Minute {
		update200QTH(Originalcallsignssid, nrl, dev)

	}

	lastUDPAddr, lastVoiceTime, _ := gp.connPool.voiceState()
	lastAddrStr := ""
	if lastUDPAddr != nil {
		lastAddrStr = lastUDPAddr.String()
	}

	if (nrl.UDPAddrStr != lastAddrStr) && nrl.timeStamp.Sub(lastVoiceTime) < 200*time.Millisecond {

		if k, ok := gp.connPool.getDevice(nrl.UDPAddrStr); ok {
			k.LastVoiceEndTime = nrl.timeStamp
		}

		return
		//否则重新让新设备抢占语音权，并更新上次报文时间
	} else {

		gp.connPool.setVoiceState(nrl.UDPAddr, nrl.timeStamp, dev.Priority)

	}

	var newpacket []byte

	if nrl.DevModel == 200 {
		newpacket = NRL21replace200and255dev(nrl.OriginalCallsign, nrl.OriginalSSID, nrl.Type, 200, nrl.CallSign, nrl.SSID, nrl.OriginalIP, nrl.DMRID, packet)
	} else if nrl.DevModel == 255 && nrl.SSID == 255 {
		newpacket = NRL21replace200and255dev(nrl.OriginalCallsign, nrl.OriginalSSID, nrl.Type, 200, nrl.CallSign, nrl.SSID, nrl.OriginalIP, nrl.DMRID, packet)
	}
	callWSHub.publishVoiceFrame(gp, nrl.OriginalCallsign, nrl.OriginalSSID, nrl.DATA, nrl.timeStamp)

	for _, vv := range gp.connPool.snapshotList() {

		if vv.udpAddr != nil && nrl.UDPAddrStr != vv.udpAddr.String() && (vv.Status&2) != 2 {

			//转发给200设备
			if vv.DevModel == 200 {
				new200packet := NRL21replace200and255dev(vv.CallSign, vv.SSID, nrl.Type, 200, nrl.OriginalCallsign, nrl.OriginalSSID, nrl.OriginalIP, nrl.DMRID, packet)
				conn.WriteToUDP(new200packet, vv.udpAddr)

			} else if nrl.DevModel == 255 && nrl.SSID == 255 && vv.DevModel != 255 && vv.SSID != 255 {
				conn.WriteToUDP(newpacket, vv.udpAddr)
			} else {
				//转发给普通设备，需要将原始信息替换协议头信息
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

	//200设备转发给其他设备
	if nrl.DevModel == 200 {

		// 999房间不接收200设备的消息
		if dev.GroupID == 999 {
			return
		}

		newpacket := NRL21replace200and255dev(nrl.OriginalCallsign, nrl.OriginalSSID, nrl.Type, 200, nrl.CallSign, nrl.SSID, nrl.OriginalIP, nrl.DMRID, packet)

		for kk, vv := range connpool.snapshotMap() {

			if clientAddrStr != kk {

				//马工3188盒子，部分盒子使用5类型控制3188信道，会干扰，临时关闭
				if vv.DevModel == 9 || vv.DevModel == 255 || vv.SSID == 255 {
					continue
				}
				//200设备转发给其他200设备，需要替换包头的呼号为新200设备的呼号
				if vv.DevModel == 200 {

					newpacket := NRL21replace200and255dev(vv.CallSign, vv.SSID, nrl.Type, 200, nrl.OriginalCallsign, nrl.OriginalSSID, nrl.OriginalIP, nrl.DMRID, packet)

					conn.WriteToUDP(newpacket, vv.udpAddr)

					// 200和255不允许相互转发

				} else {

					conn.WriteToUDP(newpacket, vv.udpAddr)
				}

			}
		}

		return
	}

	if nrl.DevModel == 255 || nrl.SSID == 255 {
		newpacket := NRL21replace200and255dev(nrl.OriginalCallsign, nrl.OriginalSSID, nrl.Type, 255, nrl.CallSign, nrl.SSID, nrl.OriginalIP, nrl.DMRID, packet)

		for kk, vv := range connpool.snapshotMap() {

			if clientAddrStr != kk {
				// 255不转发给255设备
				if vv.DevModel == 9 || vv.DevModel == 255 || vv.SSID == 255 {
					continue
					// 255和200不允许相互转发
				} else if vv.DevModel == 200 {
					continue
				} else {

					conn.WriteToUDP(newpacket, vv.udpAddr)
				}

			}
		}

		return
	}

	//普通设备转发给其他设备
	for kk, vv := range connpool.snapshotMap() {

		if clientAddrStr != kk {

			//普通设备转发给200设备
			if vv.DevModel == 200 {

				// 999房间不转发给200设备
				if dev.GroupID == 999 {
					continue
				}

				newpacket := NRL21replace200and255dev(vv.CallSign, vv.SSID, nrl.Type, 200, nrl.CallSign, nrl.SSID, nrl.UDPAddr.IP.To4(), vv.DMRID, packet)
				conn.WriteToUDP(newpacket, vv.udpAddr)

				//普通设备不转发给255设备
			} else if vv.DevModel == 9 || vv.DevModel == 255 || vv.SSID == 255 {
				continue

				//普通设备转发给普通设备
			} else {
				conn.WriteToUDP(packet, vv.udpAddr)
			}

		}
	}

	//普通设备转发给全网通设备，输出到各个平台
	if dev.GroupID == 999 {
		FullNetOutput(nrl, dev, packet)
	}

}

// forwardCtl forwardCtl
func forwardCtl(nrl *NRL21packet, packet []byte, conn *net.UDPConn, gp *group) {

	numbs := gp.connPool.count()

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

		gp.connPool.setCtlState(nrl.UDPAddr, nrl.timeStamp)

	case 2: //如果有2个设备，缺省为全双工通信，报文转发给对方

		for kk, vv := range gp.connPool.snapshotMap() {
			//删除超时的会话

			//报文转发给其它设备，不包含自己
			if nrl.UDPAddrStr != kk && (vv.Status&2) != 2 {
				//fmt.Println("case 2 :", clientAddrStr)
				if vv.DevModel == 200 {
					continue

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

		// 如果当前有会话，并且会话结束时间没超过1秒， 那么不转发其它设备报文,  丢弃无效语音
		lastUDPAddr, lastCtlTime := gp.connPool.ctlState()
		lastAddrStr := ""
		if lastUDPAddr != nil {
			lastAddrStr = lastUDPAddr.String()
		}

		if nrl.UDPAddrStr != lastAddrStr && nrl.timeStamp.Sub(lastCtlTime) < 200*time.Millisecond {

			if k, ok := gp.connPool.getDevice(nrl.UDPAddrStr); ok {
				k.LastCtlEndTime = nrl.timeStamp
			}

			// if nrl.CallSign == "BH4TDV" {
			// 	fmt.Println("*****return", gp.connPool.devConnMap)
			// }

			return
			//否则重新让新设备抢占语音权，并更新上次报文时间
		} else {
			gp.connPool.setCtlState(nrl.UDPAddr, nrl.timeStamp)
		}

		for kk, vv := range gp.connPool.snapshotMap() {
			// if nrl.timeStamp.Sub(vv.lastTime) > 5*time.Second {
			// 	log.Println("device timeout offline:", nrl.CallSign, "-", nrl.SSID, " ", kk)
			// 	delete(gp.connPool.devConnMap, kk)
			// 	continue
			// }

			if vv.udpAddr != nil && nrl.UDPAddrStr != kk && (vv.Status&2) != 2 {

				if vv.DevModel == 200 {
					continue

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
