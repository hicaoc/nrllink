package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
)

type deviceInfo struct {
	ID              int    `json:"id" db:"id"` //设备唯一编号
	Name            string `json:"name" db:"name"`
	DMRID           string `json:"dmrid" db:"dmrid"`         //设备DMRID
	Password        string `json:"password" db:"password"`   //设备接入密码
	Gird            string `json:"gird" db:"gird"`           //设备位置
	DevType         byte   `json:"dev_type" db:"dev_type"`   //设备型号
	DevModel        byte   `json:"dev_model" db:"dev_model"` //设备型号
	VoiceServerIP   string `json:"voice_server_ip"`
	VoiceServerPort string `json:"voice_server_port"`
	CallSign        string `json:"callsign" db:"callsign"`             //所有者呼号
	SSID            byte   `json:"ssid" db:"ssid"`                     //所有者SSID
	Priority        int    `json:"priority" db:"priority"`             //优先级 默认100， 数字大优先
	CallSignSSID    string `json:"callsignssid"`                       //callsign+ssid
	GroupID         int    `json:"group_id" db:"group_id"`             //内置群租编号
	GroupPassword   string `json:"group_password" db:"group_password"` //加入组的密码
	QTH             string `json:"qth"`
	Status          int    `json:"status" db:"status"`       //状态  0 正常   1 禁用接收  2 禁用发射  3 禁止发射和接收 4 透明转发
	RFType          int    `json:"rf_type" db:"rf_type"`     //连接的射频设备类型：  0:无连接， 1,内置1W模块，2，内置2W模块，3，外接moto3188，4,moto3688, 5，外接yaesu，6，外接，icom，7，外接其它
	ISCerted        bool   `json:"is_certed" db:"is_certed"` //是否认证过
	Traffic         int    `json:"traffic"`                  //流量消耗

	udpAddr    *net.UDPAddr
	udpSocket  *net.UDPConn
	CreateTime string         `json:"create_time" db:"create_time"` //加入时间
	UpdateTime string         `json:"update_time" db:"update_time"` //信息更新时间
	OnlineTime string         `json:"online_time" db:"online_time"` //设备上线时间
	ISOnline   bool           `json:"is_online" `                   //当前是否在线
	ChanName   pq.StringArray `json:"chan_name" db:"chan_name"`     //射频信道名称

	LastPacketTime time.Time `json:"last_packet_time" ` //最后一次报文时间

	VoiceTime          int       `json:"voice_time"`            //通话时长
	LastVoiceBeginTime time.Time `json:"last_voice_begin_time"` //上次语音开始时间
	LastVoiceEndTime   time.Time `json:"last_voice_end_time"`   //最后语音时间
	LastVoiceDuration  int       `json:"last_voice_duration"`   //上次语音持续时长  秒
	Loged              bool

	CtlTime          int       `json:"ctl_time"`            //通话时长
	LastCtlBeginTime time.Time `json:"last_ctl_begin_time"` //上次控制开始时间
	LastCtlEndTime   time.Time `json:"last_ctl_end_time"`   //最后控制时间
	LastCtlDuration  int       `json:"last_ctl_duration"`   //上次控制持续时长  秒

	Note          string        `json:"note" db:"note"` //设备上线时间
	DeviceParm    *control      `json:"device_parm"`
	LastATcommand *ATcommand    `json:"last_atcommand"`
	pcmG711Chan   chan [][]byte //g711数据缓存通道
	pcmBuffer     []int
	speaking      bool
	Ducked        bool `json:"ducted"`
	//ticker        *time.Ticker
}

func (d *deviceInfo) sendHeartbear() {

	packet := encodeNRL21(d.CallSign, 200, 2, 200, "", []byte{})

	for {

		if d.udpSocket != nil {
			//发送心跳包
			_, err := d.udpSocket.WriteToUDP(packet, d.udpAddr)
			if err != nil {
				log.Println("send hb err:", err)
				d.udpSocket = globelconn
			}
			time.Sleep(time.Second * 2)

		} else {
			fmt.Println("send hb stoped: device udp socket is nil")
			break
		}

	}

}

func checkdeviceOnline() {
	time.Sleep(10 * time.Second)

	for {

		time.Sleep(5 * time.Second)

		onlineMap := make(map[int]*deviceInfo, 100)
		//offlinelist := []*deviceInfo{}

		t := time.Now()
		totalstats.OnlineDevNumber = 0

		for _, vv := range publicGroupMap {

			change := false
			vv.OnlineDevNumber = 0

			for kkk, vvv := range vv.connPool.devConnMap {

				switch {
				case t.Sub(vvv.LastPacketTime) > 6*time.Second && vvv.ISOnline || kkk != vvv.udpAddr.String():
					log.Printf("public room device offline : %v-%v  %v, %v", vvv.CallSign, vvv.SSID, vvv.GroupID, vvv.udpAddr)
					delete(vv.connPool.devConnMap, kkk)
					delete(vv.connPool.devConnMap, vvv.udpAddr.String())
					vvv.ISOnline = false
					vvv.udpAddr = nil

					change = true
					//offlinelist = append(offlinelist, vvv)

				default:
					vv.OnlineDevNumber = vv.OnlineDevNumber + 1

					onlineMap[vvv.ID] = vvv

				}

				//fmt.Println("dev conn pool:", vv.ID, vv.Name, len(vv.connPool.devConnMap), kkk, vvv.udpAddr.String(), vvv.CallSign, vvv.SSID)
			}

			//如果群组设备变化过，更新列表
			if change {
				list := []*deviceInfo{}
				for _, vvv := range vv.connPool.devConnMap {
					list = append(list, vvv)
				}
				vv.connPool.devConnList = list
			}

			vv.TotalDevNumber = len(vv.DevMap)

			totalstats.OnlineDevNumber = totalstats.OnlineDevNumber + vv.OnlineDevNumber

		}

		//u.(*userinfo).Groups[dev.GroupID]
		userlist.Range(func(k, v any) bool {
			for _, vv := range v.(*userinfo).Groups {

				vv.OnlineDevNumber = 0

				for kkk, vvv := range vv.connPool.devConnMap {

					switch {
					case t.Sub(vvv.LastPacketTime) > 6*time.Second && vvv.ISOnline || kkk != vvv.udpAddr.String():
						log.Printf("privite room device offline : %v-%v  %v, %v", vvv.CallSign, vvv.SSID, vvv.GroupID, vvv.udpAddr)
						delete(vv.connPool.devConnMap, kkk)
						delete(vv.connPool.devConnMap, vvv.udpAddr.String())
						vvv.ISOnline = false
						vvv.udpAddr = nil

						//offlinelist = append(offlinelist, vvv)

					default:
						vv.OnlineDevNumber = vv.OnlineDevNumber + 1
						onlineMap[vvv.ID] = vvv

					}

				}

				vv.TotalDevNumber = len(vv.DevMap)
				totalstats.OnlineDevNumber = totalstats.OnlineDevNumber + vv.OnlineDevNumber

			}
			return true

		})

		onlinedevMap = onlineMap
		//offlineDevList = offlinelist

	}

}

func initAllDevList() {

	rows, err := db.Query(`SELECT 
	id,
	name,
	callsign,
	CAST(ssid AS INTEGER) AS ssid,
	priority,	
	dmrid,
	password,
	gird,
	dev_type,
	dev_model,
	group_id,
	status,
	is_certed,
	chan_name,
	create_time,
	update_time,
	online_time,
	note,
	rf_type  
	FROM  devices`)

	if err != nil {
		log.Println("query all device list  err:", err)
	}

	for rows.Next() {

		dev := &deviceInfo{}
		err := rows.Scan(&dev.ID, &dev.Name, &dev.CallSign, &dev.SSID, &dev.Priority, &dev.DMRID, &dev.Password, &dev.Gird, &dev.DevType, &dev.DevModel,
			&dev.GroupID, &dev.Status, &dev.ISCerted, &dev.ChanName,
			&dev.CreateTime, &dev.UpdateTime, &dev.OnlineTime, &dev.Note, &dev.RFType)
		if err != nil {
			log.Println("query  all device rows err:", err)
		}

		callsignSSID := getCallsignSSID(dev.CallSign, dev.SSID)
		dev.CallSignSSID = callsignSSID

		dev.pcmG711Chan = make(chan [][]byte, 3)
		dev.pcmBuffer = make([]int, 500)

		devCallsignSSIDMap[callsignSSID] = dev

		if kk, ok := publicGroupMap[dev.GroupID]; ok {

			kk.DevMap[dev.ID] = dev
			kk.DevList = append(kk.DevList, dev.ID)

		} else {

			if dev.GroupID > 1000 {

				dev.GroupID = 0

				if kkk, ok := publicGroupMap[dev.GroupID]; ok {
					kkk.DevMap[dev.ID] = dev
					kkk.DevList = append(kkk.DevList, dev.ID)

				}

			} else {
				if user, okok := userlist.Load(dev.CallSign); okok {
					gp := user.(*userinfo).Groups[dev.GroupID]
					gp.DevMap[dev.ID] = dev
					gp.DevList = append(gp.DevList, dev.ID)

				} else {

					dev.GroupID = 0

					if kkk, ok := publicGroupMap[dev.GroupID]; ok {
						kkk.DevMap[dev.ID] = dev
						kkk.DevList = append(kkk.DevList, dev.ID)

					}

				}

			}

		}

	}

}

func (d *deviceInfo) String() string {
	return fmt.Sprintf("%v,%v-%v,%v,%v,%v", d.LastVoiceEndTime.Format("2006-01-02 15:04:05"), d.CallSign, d.SSID, d.ID, d.GroupID, d.LastVoiceDuration/1000+1)

}

func getDevicelist(w string, p string, sort string) ([]deviceInfo, int) {

	devlist := []deviceInfo{}

	query := fmt.Sprintf(`	select 
	id,
	name,
	callsign,
	CAST(ssid AS INTEGER) AS ssid,
	priority,
	dmrid,
	password,
	gird,
	dev_type,
	dev_model,
	group_id,
	status,
	is_certed,
	chan_name,
	create_time,
	update_time,
	online_time,
	note,
	rf_type  
 from  devices  %v  %v  %v  `, w, sort, p)

	//fmt.Println(query)

	rows, err := db.Query(query)

	if err != nil {
		log.Println("查询设备列表错误: ", err, "\n", query)
		return nil, 0

	}

	for rows.Next() {

		dev := &deviceInfo{}

		err := rows.Scan(&dev.ID, &dev.Name, &dev.CallSign, &dev.SSID, &dev.Priority, &dev.DMRID, &dev.Password, &dev.Gird, &dev.DevType, &dev.DevModel,
			&dev.GroupID, &dev.Status, &dev.ISCerted, &dev.ChanName,
			&dev.CreateTime, &dev.UpdateTime, &dev.OnlineTime, &dev.Note, &dev.RFType)
		if err != nil {
			log.Println("getDevicelist err :", err, "\n", query)
			continue
		}

		d := getDeviceFromMap(getCallsignSSID(dev.CallSign, dev.SSID))

		if d != nil {

			dev = d

		}

		devlist = append(devlist, *dev)
	}

	var t int
	q := fmt.Sprintf(`SELECT count(*) as total FROM devices  %v  `, w)
	//fmt.Println(q)
	row := db.QueryRow(q)
	err = row.Scan(&t)
	if err != nil {
		log.Println(" 查询用户列表total错误 err:", err, t)
		return nil, 0
	}
	//fmt.Println(emp)
	return devlist, t
	//fmt.Println(emp)

}

func getOnlineDevicelist(limit, page int, callsign string, groupid string, sortstr string) ([]deviceInfo, int) {

	devlist := make([]deviceInfo, 0)

	if limit <= 0 || page <= 0 {
		return devlist, 0
	}

	callsign = strings.ToUpper(callsign)

	for _, v := range onlinedevMap {

		match := true
		yes, _ := regexp.MatchString(callsign, v.CallSign)
		if callsign != "" && !yes {
			match = false
		}

		if groupid != "" {
			gid, err := strconv.Atoi(groupid)
			if err != nil || v.GroupID != gid {
				match = false
			}
		}

		if match {
			devlist = append(devlist, *v)
		}

	}

	sort.Slice(devlist, func(i, j int) bool {
		if sortstr == "+id" {
			return devlist[i].ID < devlist[j].ID
		}
		return devlist[i].ID > devlist[j].ID
	})

	begin := min(limit*(page-1), len(devlist))
	end := min(limit*page, len(devlist))

	return devlist[begin:end], len(devlist)

}

func getDeviceFromMap(callsignssid string) (dev *deviceInfo) {
	if d, ok := devCallsignSSIDMap[callsignssid]; ok {
		return d
	}
	return nil
}

func getDevice(callsign string, ssid byte) (dev *deviceInfo) {
	dev = &deviceInfo{}

	row := db.QueryRow(`select id,name,callsign,
	CAST(ssid AS INTEGER) AS ssid,		priority,
	dmrid,password,gird,dev_type,dev_model,
	group_id,status,is_certed,chan_name,
	create_time,update_time,online_time,note,rf_type  
 from  devices   where callsign=? and ssid=?`, callsign, ssid)

	err := row.Scan(&dev.ID, &dev.Name, &dev.CallSign, &dev.SSID, &dev.Priority, &dev.DMRID, &dev.Password, &dev.Gird, &dev.DevType, &dev.DevModel,
		&dev.GroupID, &dev.Status, &dev.ISCerted, &dev.ChanName,
		&dev.CreateTime, &dev.UpdateTime, &dev.OnlineTime, &dev.Note, &dev.RFType)

	if err != nil {
		log.Println("query one device rows err:", err)
	}

	callsignSSID := getCallsignSSID(dev.CallSign, dev.SSID)
	dev.CallSignSSID = callsignSSID

	return dev

}

func getDeviceByDMRID(dmrid string) (dev *deviceInfo) {
	dev = &deviceInfo{}

	row := db.QueryRow(`select
	id,
	name,
	callsign,
	CAST(ssid AS INTEGER) AS ssid,
	dmrid,
	password,gird,dev_type,dev_model,
	group_id,status,is_certed,chan_name,
	create_time,update_time,online_time,note,rf_type
 from  devices   where dmrid=? `, dmrid)

	err := row.Scan(&dev.ID, &dev.Name, &dev.CallSign, &dev.SSID, &dev.DMRID, &dev.Password, &dev.Gird, &dev.DevType, &dev.DevModel,
		&dev.GroupID, &dev.Status, &dev.ISCerted, &dev.ChanName,
		&dev.CreateTime, &dev.UpdateTime, &dev.OnlineTime, &dev.Note, &dev.RFType)

	if err != nil {
		log.Println("query one device rows err:", err)
	}

	callsignSSID := getCallsignSSID(dev.CallSign, dev.SSID)
	dev.CallSignSSID = callsignSSID

	return dev

}

func queryDeviceParm(callsignwithssid string) (dev deviceInfo, err error) {

	if dev, ok := devCallsignSSIDMap[callsignwithssid]; ok {

		globelconn.WriteToUDP(encodeNRL21(dev.CallSign, dev.SSID, 3, 0, "", []byte{0x01}), dev.udpAddr)

		time.Sleep(300 * time.Millisecond)

		return *dev, nil

	}

	return dev, fmt.Errorf("dev not found with callsign and ssid %v ", callsignwithssid)

}

func changeDeviceByteParm(callsignssid string, offset int, str string) (res []byte, err error) {

	val, _ := strconv.Atoi(str)

	if d, ok := devCallsignSSIDMap[callsignssid]; ok {

		d.DeviceParm.data[offset] = byte(val)
		newpacket := encodeNRL21(d.CallSign, d.SSID, 3, 0, "", []byte{0x03})
		newpacket = append(newpacket, d.DeviceParm.data...)
		globelconn.WriteToUDP(newpacket, d.udpAddr)
		time.Sleep(200 * time.Millisecond)

		rescode, _ := jsonextra.Marshal(d)
		return rescode, nil

	}

	return nil, errors.New("device is not found")

}

func changeDeviceCallsignSSIDParm(callsignssid string, newcallsignssid string) (res []byte, err error) {

	s := strings.Split(newcallsignssid, "-")

	callsign := s[0]
	ssid, _ := strconv.Atoi(s[1])

	if !IsCallSign(callsign) || len(callsign) > 6 || len(callsign) < 3 || ssid > 255 {
		return nil, errors.New("callsign or ssid error")
	}

	if d, ok := devCallsignSSIDMap[callsignssid]; ok {

		d.DeviceParm.data[64] = byte(ssid)
		copy(d.DeviceParm.data[65:71], callsign)
		if len(callsign) == 5 {
			d.DeviceParm.data[70] = 0
		}
		if len(callsign) == 4 {
			d.DeviceParm.data[69] = 0
		}
		newpacket := encodeNRL21(d.CallSign, d.SSID, 3, 0, "", []byte{0x03})
		newpacket = append(newpacket, d.DeviceParm.data...)
		globelconn.WriteToUDP(newpacket, d.udpAddr)
		time.Sleep(200 * time.Millisecond)

		rescode, _ := jsonextra.Marshal(d)
		return rescode, nil

	}

	return nil, errors.New("device is not found")

}

type ipparm struct {
	localIPOffset      int
	localIPValue       string
	localNetmaskOffset int
	localNetmaskValue  string
	gatewayOffset      int
	gatewayValue       string
	dnsOffset          int
	dnsValue           string
	destIPOffset       int
	destIPValue        string
}

func (i *ipparm) String() string {
	return fmt.Sprintf("%v,%v,%v,%v,%v", i.localIPValue, i.localNetmaskValue, i.gatewayValue, i.dnsValue, i.destIPValue)
}

func checkIP(str string) ([]byte, bool) {

	ipaddr := net.ParseIP(str)

	if ipaddr == nil {
		return nil, false
	}

	ip := ipaddr.To4()

	if ip == nil {
		return nil, false
	}

	return ip, true

}

func changeDeviceIPParm(callsignssid string, ip ipparm) (res []byte, err error) {

	if len(ip.destIPValue) >= 30 {
		return nil, fmt.Errorf("DIP format err")

	}

	lip, lipok := checkIP(ip.localIPValue)
	netmask, netmaskok := checkIP(ip.localNetmaskValue)
	gateway, gatewayok := checkIP(ip.gatewayValue)
	dns, dnsok := checkIP(ip.dnsValue)

	if !(lipok && netmaskok && gatewayok && dnsok) {
		return nil, errors.New("ip format error")
	}

	if d, ok := devCallsignSSIDMap[callsignssid]; ok {

		for _, v := range lip {
			d.DeviceParm.data[ip.localIPOffset] = v
			ip.localIPOffset++
		}

		for _, v := range netmask {
			d.DeviceParm.data[ip.localNetmaskOffset] = v
			ip.localNetmaskOffset++
		}

		for _, v := range gateway {
			d.DeviceParm.data[ip.gatewayOffset] = v
			ip.gatewayOffset++
		}

		for _, v := range dns {
			d.DeviceParm.data[ip.dnsOffset] = v
			ip.dnsOffset++
		}

		for _, v := range ip.destIPValue {
			d.DeviceParm.data[ip.destIPOffset] = byte(v)
			ip.destIPOffset++
		}
		d.DeviceParm.data[ip.destIPOffset] = 0

		newpacket := encodeNRL21(d.CallSign, d.SSID, 3, 0, "", []byte{0x03})
		newpacket = append(newpacket, d.DeviceParm.data...)
		globelconn.WriteToUDP(newpacket, d.udpAddr)
		time.Sleep(200 * time.Millisecond)

		rescode, _ := jsonextra.Marshal(d)
		return rescode, nil

	}

	return nil, errors.New("device is not found")

}

func DeviceAT(at *ATcommand) (dev *deviceInfo, err error) {

	if d, ok := devCallsignSSIDMap[getCallsignSSID(at.CallSign, at.SSID)]; ok {

		atcommand := append([]byte{0x01}, []byte(at.String())...)

		packet := encodeNRL21(at.CallSign, at.SSID, 11, 11, d.DMRID, []byte(atcommand))

		globelconn.WriteToUDP(packet, d.udpAddr)

		return d, nil

	}

	return nil, errors.New("device is not found")

}

func changeDeviceUint16Parm(callsignssid string, offset int, str string) (res []byte, err error) {

	val, _ := strconv.Atoi(str)

	if d, ok := devCallsignSSIDMap[callsignssid]; ok {

		d.DeviceParm.data[offset+1] = byte(val & 0xFF)
		d.DeviceParm.data[offset] = byte(val >> 8)

		newpacket := encodeNRL21(d.CallSign, d.SSID, 3, 0, "", []byte{0x03})
		newpacket = append(newpacket, d.DeviceParm.data...)
		globelconn.WriteToUDP(newpacket, d.udpAddr)
		time.Sleep(200 * time.Millisecond)

		rescode, _ := jsonextra.Marshal(d)
		return rescode, nil

	}

	return nil, errors.New("device is not found")

}

func changeDevice1W(ctr *control) (res []byte, err error) {

	if d, ok := devCallsignSSIDMap[getCallsignSSID(ctr.CallSign, ctr.SSID)]; ok {

		oneParm := bytes.Split(bytes.Split(d.DeviceParm.data[128:160], []byte{0x00})[0], []byte{','})
		//oneParm[0] = []byte{'0'}  //无需修改，使用原始的GBW数据
		oneParm[1] = []byte(ctr.OneTransmitFreq)
		oneParm[2] = []byte(ctr.OneReciveFreq)
		oneParm[3] = []byte(ctr.OneReciveCXCSS)
		oneParm[4] = []byte(strconv.Itoa(ctr.OneSQLLevel))
		oneParm[5] = []byte(ctr.OneTransmitCXCSS)
		//oneParm[6] = []byte{'0'}  //无需修改，使用原始的fLAG值

		p := bytes.Join(oneParm, []byte{','})
		p = append(p, 0x00)

		fmt.Println("One w:", string(p))

		if len(p) > 32 {
			p = p[:32]
		}
		copy(d.DeviceParm.data[128:], p)
		d.DeviceParm.data[160] = []byte(strconv.Itoa(ctr.OneVolume))[0]
		d.DeviceParm.data[161] = []byte(strconv.Itoa(ctr.OneMICSensitivity))[0]

		newpacket := append(encodeNRL21(d.CallSign, d.SSID, 3, 0, "", []byte{0x03}), d.DeviceParm.data...)
		globelconn.WriteToUDP(newpacket, d.udpAddr)
		time.Sleep(200 * time.Millisecond)

		rescode, _ := jsonextra.Marshal(d)
		return rescode, nil

	}

	return nil, errors.New("device is not found")

}

func changeDevice2W(ctr *control) (res []byte, err error) {

	if d, ok := devCallsignSSIDMap[getCallsignSSID(ctr.CallSign, ctr.SSID)]; ok {

		//oneParm := bytes.Split(bytes.Split(d.DeviceParm.data[128:160], []byte{0x00})[0], []byte{','})

		twoParm := bytes.Split(bytes.Split(d.DeviceParm.data[192:227], []byte{0x00})[0], []byte{','})

		twoParm[0] = []byte(ctr.TwoReciveFreq)
		twoParm[1] = []byte(ctr.TwoTransmitFreq)
		twoParm[2] = []byte(ctr.TwoReciveCXCSS)
		twoParm[3] = []byte(ctr.TwoTransmitCXCSS)

		p := bytes.Join(twoParm, []byte{','})

		fmt.Println("two w : ", string(p))

		p = append(p, byte(0x00))

		copy(d.DeviceParm.data[192:], p)

		d.DeviceParm.data[238] = []byte(strconv.Itoa(ctr.TwoVolume))[0]
		d.DeviceParm.data[239] = []byte(strconv.Itoa(ctr.TwoSavePower))[0]
		d.DeviceParm.data[240] = []byte(strconv.Itoa(ctr.TwoSQLLevel))[0]
		d.DeviceParm.data[242] = []byte(strconv.Itoa(ctr.TwoMICLevel))[0]
		d.DeviceParm.data[244] = []byte(strconv.Itoa(ctr.TwoTOTLevel))[0]

		newpacket := encodeNRL21(d.CallSign, d.SSID, 3, 0, "", []byte{0x03})
		newpacket = append(newpacket, d.DeviceParm.data...)
		globelconn.WriteToUDP(newpacket, d.udpAddr)
		time.Sleep(200 * time.Millisecond)

		rescode, _ := jsonextra.Marshal(d)
		return rescode, nil

	}

	return nil, errors.New("device is not found")

}

func IsCallSign(s string) bool {

	for i := 0; i < len(s); i++ {
		char := s[i] // Get the byte value of the character

		// Check if the character is an uppercase letter (A-Z)
		isUpper := char >= 'A' && char <= 'Z'

		// Check if the character is a digit (0-9)
		isDigit := char >= '0' && char <= '9'

		// If the character is neither an uppercase letter nor a digit, return false
		if !isUpper && !isDigit {
			return false
		}
	}
	return true // All characters are uppercase or digits
}

func addDevice(dev *deviceInfo) error {

	//	fmt.Println("user:", e)
	query := `INSERT INTO devices (	name,gird,dev_type,dev_model,status,group_id,callsign,ssid,dmrid,chan_name,note,password,rf_type,is_certed,online_time,create_time,update_time)
		 VALUES ('','',0,0,0,0,?,?,?,?,'','',0,false,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`

	_, err := db.Exec(query, dev.CallSign, strconv.Itoa(int(dev.SSID)), dev.DMRID, dev.ChanName)

	if err != nil {
		log.Println("add dev failed, ", err, '\n', query)
		return err
	}

	return nil

}

func offlineDevice(dev string) {

	log.Println("offlineDevice:", dev)

	if d, ok := devCallsignSSIDMap[dev]; ok {

		//delete(publicGroupMap[d.GroupID].connPool.devConnMap, d.udpAddr.String())
		d.udpAddr = nil
		d.ISOnline = false

	} else {
		log.Println("device is not found", dev)
	}
}

func delDevice(dev *deviceInfo) error {

	//	fmt.Println("user:", e)
	query := `delete from devices where id=? `

	_, err := db.Exec(query, dev.ID)

	if err != nil {
		log.Println("delete dev failed, ", err, '\n', query)
		return err
	}

	if d, ok := devCallsignSSIDMap[dev.CallSignSSID]; ok {
		delete(devCallsignSSIDMap, dev.CallSignSSID)
		delete(publicGroupMap[dev.GroupID].DevMap, dev.ID)
		delete(publicGroupMap[dev.GroupID].connPool.devConnMap, d.udpAddr.String())
	}

	return nil

}

func updateDevice(e *deviceInfo) error {

	_, err := db.Exec(`update devices set name=?, gird=?, dmrid=?, dev_type=?, dev_model=?, 	group_id=?,status=?,priority=?,
	chan_name=?,rf_type=?,note=?,password=?,update_time=CURRENT_TIMESTAMP  where id=?`,
		e.Name, e.Gird, e.DMRID, e.DevType, e.DevModel, e.GroupID, e.Status, e.Priority, e.ChanName, e.RFType, e.Note, e.Password, e.ID)
	if err != nil {
		log.Println("update device failed, ", err)
		return err
	}

	if d, ok := devCallsignSSIDMap[getCallsignSSID(e.CallSign, e.SSID)]; ok {
		d.Name = e.Name
		d.Gird = e.Gird
		d.DMRID = e.DMRID
		d.DevType = e.DevType
		d.DevModel = e.DevModel
		d.Status = e.Status
		d.Priority = e.Priority
		d.Note = e.Note
		d.Password = e.Password
		d.RFType = e.RFType
		d.ChanName = e.ChanName

		if d.GroupID != e.GroupID {

			if g, okok := publicGroupMap[e.GroupID]; okok {

				if e.GroupID == 0 || g.Password == "" || e.Password == g.Password {

					_, err := changeDevGroup(d, e.GroupID)
					if err != nil {
						return err
					}

				} else {
					fmt.Println("group password:", e.Password, []byte(g.Password), len(e.Password), len(g.Password), g)
					return fmt.Errorf("group pasword err")
				}

			} else if e.GroupID < 1000 && e.GroupID != 0 {
				_, err := changeDevGroup(d, e.GroupID)
				if err != nil {
					return err
				}

			} else {
				return fmt.Errorf("group not found")

			}

		}

	}

	return nil

}

func changeDeviceGroup(e *deviceInfo) error {

	if d, ok := devCallsignSSIDMap[getCallsignSSID(e.CallSign, e.SSID)]; ok {

		if d.GroupID != e.GroupID {

			if g, okok := publicGroupMap[e.GroupID]; okok {

				if e.GroupID == 0 || g.Password == "" || e.Password == g.Password {

					_, err := changeDevGroup(d, e.GroupID)
					if err != nil {
						return err
					}
					_, err = db.Exec(`update devices set group_id=? where id=? `, e.GroupID, d.ID)
					if err != nil {
						log.Println("update device failed, ", err)
						return err
					}

				} else {
					return fmt.Errorf("group pasword err")
				}

			} else if e.GroupID < 1000 && e.GroupID != 0 {
				_, err := changeDevGroup(d, e.GroupID)
				if err != nil {
					return err
				}

			} else {
				return fmt.Errorf("group not found")

			}

		}

	}

	return nil

}
