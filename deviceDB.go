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

	"sync"
	"sync/atomic"

	"github.com/lib/pq"
)

type udpEndpoint struct {
	addr    *net.UDPAddr
	addrStr string
}

type deviceInfo struct {
	ID              int    `json:"id" db:"id"` //设备唯一编号
	Name            string `json:"name" db:"name"`
	DMRID           uint32 `json:"dmrid" db:"dmrid"`         //设备DMRID
	Password        string `json:"-" db:"password"`          //设备接入密码，不参与 JSON 序列化，防止接口泄露
	Gird            string `json:"gird" db:"gird"`           //设备位置
	DevType         byte   `json:"dev_type" db:"dev_type"`   //设备型号
	DevModel        byte   `json:"dev_model" db:"dev_model"` //设备型号
	VoiceServerIP   string `json:"voice_server_ip"`
	VoiceServerPort string `json:"voice_server_port"`
	CallSign        string `json:"callsign" db:"callsign"` //所有者呼号
	SSID            byte   `json:"ssid" db:"ssid"`         //所有者SSID
	Priority        int    `json:"priority" db:"priority"` //优先级 默认100， 数字大优先
	CallSignSSID    string `json:"callsignssid"`           //callsign+ssid
	GroupID         int    `json:"group_id" db:"group_id"` //内置群租编号
	GroupPassword   string `json:"-" db:"group_password"`  //加入组的密码，不参与 JSON 序列化
	QTH             string `json:"qth"`
	Status          int    `json:"status" db:"status"`       //状态  0 正常   1 禁用接收  2 禁用发射  3 禁止发射和接收 4 透明转发
	RFType          int    `json:"rf_type" db:"rf_type"`     //连接的射频设备类型：  0:无连接， 1,内置1W模块，2，内置2W模块，3，外接moto3188，4,moto3688, 5，外接yaesu，6，外接，icom，7，外接其它
	ISCerted        bool   `json:"is_certed" db:"is_certed"` //是否认证过
	Traffic         int    `json:"traffic"`                  //流量消耗

	udpAddr    atomic.Pointer[udpEndpoint]
	udpSocket  *net.UDPConn
	CreateTime string         `json:"create_time" db:"create_time"` //加入时间
	UpdateTime string         `json:"update_time" db:"update_time"` //信息更新时间
	OnlineTime string         `json:"online_time" db:"online_time"` //设备上线时间
	ISOnline   bool           `json:"is_online" `                   //当前是否在线
	ChanName   pq.StringArray `json:"chan_name" db:"chan_name"`     //射频信道名称

	LastPacketTime  time.Time   `json:"last_packet_time" ` //最后一次报文时间
	AccountExpired  bool        `json:"account_expired"`   //所属账号是否到期
	LastExpireCheck time.Time   `json:"last_expire_check"` //最后一次账号到期检查时间
	expireMu        sync.Mutex
	expireChecking  atomic.Bool

	VoiceTime          int       `json:"voice_time"`            //通话时长
	LastVoiceBeginTime time.Time `json:"last_voice_begin_time"` //上次语音开始时间
	LastVoiceEndTime   time.Time `json:"last_voice_end_time"`   //最后语音时间
	LastVoiceDuration  int       `json:"last_voice_duration"`   //上次语音持续时长  秒
	Loged              bool

	CtlTime          int       `json:"ctl_time"`            //通话时长
	LastCtlBeginTime time.Time `json:"last_ctl_begin_time"` //上次控制开始时间
	LastCtlEndTime   time.Time `json:"last_ctl_end_time"`   //最后控制时间
	LastCtlDuration  int       `json:"last_ctl_duration"`   //上次控制持续时长  秒

	Note          string     `json:"note" db:"note"` //设备上线时间
	DeviceParm    *control   `json:"device_parm"`
	LastATcommand *ATcommand `json:"last_atcommand"`
	pcmBuf        []int16    // 8 kHz mono PCM; the conference mixer consumes 160 samples per tick
	pcmMu         sync.Mutex
	pcmBuffer     []int
	speaking      bool
	opusMu        sync.Mutex
	opusDecoders  map[string]*nrlOpusDecoder
	voiceEcho     voiceEchoRecorder
	Ducked        bool `json:"ducted"`
	stateMu       sync.RWMutex
	//ticker        *time.Ticker
}

type deviceInfoJSON struct {
	ID                 int            `json:"id"`
	Name               string         `json:"name"`
	DMRID              uint32         `json:"dmrid"`
	Gird               string         `json:"gird"`
	DevType            byte           `json:"dev_type"`
	DevModel           byte           `json:"dev_model"`
	VoiceServerIP      string         `json:"voice_server_ip"`
	VoiceServerPort    string         `json:"voice_server_port"`
	CallSign           string         `json:"callsign"`
	SSID               byte           `json:"ssid"`
	Priority           int            `json:"priority"`
	CallSignSSID       string         `json:"callsignssid"`
	GroupID            int            `json:"group_id"`
	QTH                string         `json:"qth"`
	Status             int            `json:"status"`
	RFType             int            `json:"rf_type"`
	ISCerted           bool           `json:"is_certed"`
	Traffic            int            `json:"traffic"`
	CreateTime         string         `json:"create_time"`
	UpdateTime         string         `json:"update_time"`
	OnlineTime         string         `json:"online_time"`
	ISOnline           bool           `json:"is_online"`
	ChanName           pq.StringArray `json:"chan_name"`
	LastPacketTime     time.Time      `json:"last_packet_time"`
	AccountExpired     bool           `json:"account_expired"`
	LastExpireCheck    time.Time      `json:"last_expire_check"`
	VoiceTime          int            `json:"voice_time"`
	LastVoiceBeginTime time.Time      `json:"last_voice_begin_time"`
	LastVoiceEndTime   time.Time      `json:"last_voice_end_time"`
	LastVoiceDuration  int            `json:"last_voice_duration"`
	Loged              bool
	CtlTime            int            `json:"ctl_time"`
	LastCtlBeginTime   time.Time      `json:"last_ctl_begin_time"`
	LastCtlEndTime     time.Time      `json:"last_ctl_end_time"`
	LastCtlDuration    int            `json:"last_ctl_duration"`
	Note               string         `json:"note"`
	DeviceParm         *control       `json:"device_parm"`
	LastATcommand      *ATcommand     `json:"last_atcommand"`
	Ducked             bool           `json:"ducted"`
}

func (d *deviceInfo) snapshotJSON(hideDeviceParm bool) deviceInfoJSON {
	d.stateMu.RLock()
	d.expireMu.Lock()
	accountExpired := d.AccountExpired
	lastExpireCheck := d.LastExpireCheck
	d.expireMu.Unlock()

	snap := deviceInfoJSON{
		ID:                 d.ID,
		Name:               d.Name,
		DMRID:              d.DMRID,
		Gird:               d.Gird,
		DevType:            d.DevType,
		DevModel:           d.DevModel,
		VoiceServerIP:      d.VoiceServerIP,
		VoiceServerPort:    d.VoiceServerPort,
		CallSign:           d.CallSign,
		SSID:               d.SSID,
		Priority:           d.Priority,
		CallSignSSID:       d.CallSignSSID,
		GroupID:            d.GroupID,
		QTH:                d.QTH,
		Status:             d.Status,
		RFType:             d.RFType,
		ISCerted:           d.ISCerted,
		Traffic:            d.Traffic,
		CreateTime:         d.CreateTime,
		UpdateTime:         d.UpdateTime,
		OnlineTime:         d.OnlineTime,
		ISOnline:           d.ISOnline,
		ChanName:           d.ChanName,
		LastPacketTime:     d.LastPacketTime,
		AccountExpired:     accountExpired,
		LastExpireCheck:    lastExpireCheck,
		VoiceTime:          d.VoiceTime,
		LastVoiceBeginTime: d.LastVoiceBeginTime,
		LastVoiceEndTime:   d.LastVoiceEndTime,
		LastVoiceDuration:  d.LastVoiceDuration,
		Loged:              d.Loged,
		CtlTime:            d.CtlTime,
		LastCtlBeginTime:   d.LastCtlBeginTime,
		LastCtlEndTime:     d.LastCtlEndTime,
		LastCtlDuration:    d.LastCtlDuration,
		Note:               d.Note,
		DeviceParm:         d.DeviceParm,
		LastATcommand:      d.LastATcommand,
		Ducked:             d.Ducked,
	}
	d.stateMu.RUnlock()

	if hideDeviceParm {
		snap.DeviceParm = nil
	}
	return snap
}

func (d *deviceInfo) MarshalJSON() ([]byte, error) {
	if d == nil {
		return []byte("null"), nil
	}
	return jsonextra.Marshal(d.snapshotJSON(false))
}

func (d *deviceInfo) recordPacket(ts time.Time, pktBytes int) (dmrid uint32, groupID int) {
	d.stateMu.Lock()
	d.LastPacketTime = ts
	d.Traffic += pktBytes
	dmrid = d.DMRID
	groupID = d.GroupID
	d.stateMu.Unlock()
	return dmrid, groupID
}

func (d *deviceInfo) getGroupID() int {
	d.stateMu.RLock()
	id := d.GroupID
	d.stateMu.RUnlock()
	return id
}

func (d *deviceInfo) setGroupID(id int) {
	d.stateMu.Lock()
	d.GroupID = id
	d.stateMu.Unlock()
}

func (d *deviceInfo) isOnline() bool {
	d.stateMu.RLock()
	v := d.ISOnline
	d.stateMu.RUnlock()
	return v
}

func (d *deviceInfo) setOnline(v bool) {
	d.stateMu.Lock()
	d.ISOnline = v
	d.stateMu.Unlock()
}

func (d *deviceInfo) getStatus() int {
	d.stateMu.RLock()
	v := d.Status
	d.stateMu.RUnlock()
	return v
}

func (d *deviceInfo) getDevModel() byte {
	d.stateMu.RLock()
	v := d.DevModel
	d.stateMu.RUnlock()
	return v
}

func (d *deviceInfo) setDevModel(m byte) {
	d.stateMu.Lock()
	d.DevModel = m
	d.stateMu.Unlock()
}

func (d *deviceInfo) getPriority() int {
	d.stateMu.RLock()
	v := d.Priority
	d.stateMu.RUnlock()
	return v
}

func (d *deviceInfo) getDMRID() uint32 {
	d.stateMu.RLock()
	v := d.DMRID
	d.stateMu.RUnlock()
	return v
}

func (d *deviceInfo) setDeviceParm(p *control) {
	d.stateMu.Lock()
	d.DeviceParm = p
	d.stateMu.Unlock()
}

func (d *deviceInfo) getDeviceParm() *control {
	d.stateMu.RLock()
	p := d.DeviceParm
	d.stateMu.RUnlock()
	return p
}

func (d *deviceInfo) setQTHValue(q string) {
	d.stateMu.Lock()
	d.QTH = q
	d.stateMu.Unlock()
}

func (d *deviceInfo) setLastATCommand(at *ATcommand) {
	d.stateMu.Lock()
	d.LastATcommand = at
	d.stateMu.Unlock()
}

func (d *deviceInfo) markOfflineIfExpired(now time.Time, timeout time.Duration) (bool, int) {
	d.stateMu.Lock()
	defer d.stateMu.Unlock()
	if d.ISOnline && now.Sub(d.LastPacketTime) > timeout {
		d.ISOnline = false
		return true, d.GroupID
	}
	return false, d.GroupID
}

func (d *deviceInfo) setUDPAddr(addr *net.UDPAddr) {
	if addr == nil {
		d.udpAddr.Store(nil)
		return
	}
	d.udpAddr.Store(&udpEndpoint{addr: addr, addrStr: addr.String()})
}

func (d *deviceInfo) setUDPAddrWithStr(addr *net.UDPAddr, addrStr string) {
	if addr == nil {
		d.udpAddr.Store(nil)
		return
	}
	if ep := d.udpAddr.Load(); ep != nil && ep.addrStr == addrStr {
		return
	}
	if addrStr == "" {
		addrStr = addr.String()
	}
	d.udpAddr.Store(&udpEndpoint{addr: addr, addrStr: addrStr})
}

func (d *deviceInfo) clearUDPAddr() {
	d.udpAddr.Store(nil)
}

func (d *deviceInfo) getUDPAddr() (*net.UDPAddr, string) {
	if d == nil {
		return nil, ""
	}
	ep := d.udpAddr.Load()
	if ep == nil {
		return nil, ""
	}
	return ep.addr, ep.addrStr
}

func (d *deviceInfo) sendHeartbear() {

	packet := encodeNRL21(d.CallSign, 200, 2, 200, 0, []byte{})

	for {

		if d.udpSocket != nil {
			//发送心跳包
			if addr, _ := d.getUDPAddr(); addr != nil {
				_, err := d.udpSocket.WriteToUDP(packet, addr)
				if err != nil {
					log.Println("send hb err:", err, addr)
					d.udpSocket = globelconn
				}
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
		onlineDeviceTotal := 0

		for _, vv := range snapshotPublicGroups() {

			change := false
			vv.OnlineDevNumber = 0

			vv.connPool.mu.Lock()
			for kkk, vvv := range vv.connPool.devConnMap {
				//255设备不参与在线统计，永远在线
				if vvv.getDevModel() == 255 || vvv.SSID == 255 {
					continue
				}

				// 清理过期条目：设备地址已变化，旧key需要删除，不影响设备在线状态
				addr, addrStr := vvv.getUDPAddr()
				if addr != nil && kkk != addrStr {
					delete(vv.connPool.devConnMap, kkk)
					change = true
					continue
				}

				// 设备超时离线
				if wentOffline, gid := vvv.markOfflineIfExpired(t, 6*time.Second); wentOffline {
					log.Printf("public room device offline : %v-%v  %v, %v", vvv.CallSign, vvv.SSID, gid, addr)
					delete(vv.connPool.devConnMap, kkk)
					vvv.clearUDPAddr()
					change = true
					continue
				}

				vv.OnlineDevNumber = vv.OnlineDevNumber + 1
				onlineMap[vvv.ID] = vvv
			}

			//如果群组设备变化过，更新列表
			if change {
				vv.connPool.rebuildListLocked()
			}
			vv.connPool.mu.Unlock()

			vv.TotalDevNumber = vv.deviceCount()

			onlineDeviceTotal = onlineDeviceTotal + vv.OnlineDevNumber

		}

		//u.(*userinfo).Groups[dev.GroupID]
		userlist.Range(func(k, v any) bool {
			for _, vv := range v.(*userinfo).Groups {

				change := false
				vv.OnlineDevNumber = 0

				vv.connPool.mu.Lock()
				for kkk, vvv := range vv.connPool.devConnMap {

					// 清理过期条目：设备地址已变化，旧key需要删除，不影响设备在线状态
					addr, addrStr := vvv.getUDPAddr()
					if addr != nil && kkk != addrStr {
						delete(vv.connPool.devConnMap, kkk)
						change = true
						continue
					}

					// 设备超时离线
					if wentOffline, gid := vvv.markOfflineIfExpired(t, 6*time.Second); wentOffline {
						log.Printf("privite room device offline : %v-%v  %v, %v", vvv.CallSign, vvv.SSID, gid, addr)
						delete(vv.connPool.devConnMap, kkk)
						vvv.clearUDPAddr()
						change = true
						continue
					}

					vv.OnlineDevNumber = vv.OnlineDevNumber + 1
					onlineMap[vvv.ID] = vvv

				}
				if change {
					vv.connPool.rebuildListLocked()
				}
				vv.connPool.mu.Unlock()

				vv.TotalDevNumber = vv.deviceCount()
				onlineDeviceTotal = onlineDeviceTotal + vv.OnlineDevNumber

			}
			return true

		})

		prevOnlineDeviceTotal := getOnlineDevNumberStat()
		setOnlineDevMap(onlineMap)
		setOnlineDevNumberStat(onlineDeviceTotal)
		if prevOnlineDeviceTotal == 0 && onlineDeviceTotal > 0 {
			go func() {
				if err := reportCurrentServerStatus(); err != nil {
					log.Println("platform-servers: refresh report after online count recovered failed:", err)
				}
			}()
		}
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
		return
	}
	defer rows.Close()

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

		dev.pcmBuffer = make([]int, 1500)

		if dev.SSID == 255 || dev.DevModel == 255 {
			dev.GroupID = 999
		}

		setDevByCallsignSSID(callsignSSID, dev)

		if kk, ok := getPublicGroup(dev.GroupID); ok {

			kk.setDevice(dev.ID, dev)
			kk.DevList = append(kk.DevList, dev.ID)

		} else {

			if isPublicGroupID(dev.GroupID) {

				dev.GroupID = 0

				if kkk, ok := getPublicGroup(dev.GroupID); ok {
					kkk.setDevice(dev.ID, dev)
					kkk.DevList = append(kkk.DevList, dev.ID)

				}

			} else if isPrivateGroupID(dev.GroupID) {
				if user, okok := userlist.Load(dev.CallSign); okok {
					if gp, exists := user.(*userinfo).Groups[dev.GroupID]; exists {
						gp.setDevice(dev.ID, dev)
						gp.DevList = append(gp.DevList, dev.ID)
					} else {
						dev.GroupID = 0
						if pg0, ok := getPublicGroup(0); ok {
							pg0.setDevice(dev.ID, dev)
							pg0.DevList = append(pg0.DevList, dev.ID)
						}
					}

				} else {

					dev.GroupID = 0

					if kkk, ok := getPublicGroup(dev.GroupID); ok {
						kkk.setDevice(dev.ID, dev)
						kkk.DevList = append(kkk.DevList, dev.ID)

					}

				}

			} else {
				dev.GroupID = 0
				if pg0, ok := getPublicGroup(0); ok {
					pg0.setDevice(dev.ID, dev)
					pg0.DevList = append(pg0.DevList, dev.ID)
				}
			}

		}

	}
	if err := rows.Err(); err != nil {
		log.Println("iterate all device rows err:", err)
	}

}

func (d *deviceInfo) String() string {
	return fmt.Sprintf("%v,%v-%v,%v,%v,%v", d.LastVoiceEndTime.Format("2006-01-02 15:04:05"), d.CallSign, d.SSID, d.ID, d.GroupID, d.LastVoiceDuration/1000+1)

}

func getDevicelist(w string, args []any, p string, sort string) ([]*deviceInfo, int) {

	devlist := []*deviceInfo{}

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

	rows, err := db.Query(query, args...)

	if err != nil {
		log.Println("查询设备列表错误: ", err, "\n", query)
		return nil, 0

	}
	defer rows.Close()

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

		devlist = append(devlist, dev)
	}
	if err := rows.Err(); err != nil {
		log.Println("遍历设备列表错误: ", err, "\n", query)
		return nil, 0
	}

	var t int
	q := fmt.Sprintf(`SELECT count(*) as total FROM devices  %v  `, w)
	//fmt.Println(q)
	row := db.QueryRow(q, args...)
	err = row.Scan(&t)
	if err != nil {
		log.Println(" 查询用户列表total错误 err:", err, t)
		return nil, 0
	}
	//fmt.Println(emp)
	return devlist, t
	//fmt.Println(emp)

}

func getOnlineDevicelist(limit, page int, callsign string, groupid string, sortstr string) ([]*deviceInfo, int) {

	devlist := make([]*deviceInfo, 0)

	if limit <= 0 || page <= 0 {
		return devlist, 0
	}

	callsign = strings.ToUpper(callsign)

	onlineMap := getOnlineDevMap()
	for _, v := range onlineMap {

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
			devlist = append(devlist, v)
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
	if d, ok := getDevByCallsignSSID(callsignssid); ok {
		return d
	}
	return nil
}

func getDevice(callsign string, ssid byte) (dev *deviceInfo, err error) {
	dev = &deviceInfo{}

	row := db.QueryRow(`select id,name,callsign,
	CAST(ssid AS INTEGER) AS ssid,		priority,
	dmrid,password,gird,dev_type,dev_model,
	group_id,status,is_certed,chan_name,
	create_time,update_time,online_time,note,rf_type  
 from  devices   where callsign=? and ssid=?`, callsign, ssid)

	err = row.Scan(&dev.ID, &dev.Name, &dev.CallSign, &dev.SSID, &dev.Priority, &dev.DMRID, &dev.Password, &dev.Gird, &dev.DevType, &dev.DevModel,
		&dev.GroupID, &dev.Status, &dev.ISCerted, &dev.ChanName,
		&dev.CreateTime, &dev.UpdateTime, &dev.OnlineTime, &dev.Note, &dev.RFType)

	if err != nil {
		log.Println("query one device rows err:", err)
		return nil, err
	}

	callsignSSID := getCallsignSSID(dev.CallSign, dev.SSID)
	dev.CallSignSSID = callsignSSID

	return dev, nil

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

func queryDeviceParm(callsignwithssid string) (dev *deviceInfo, err error) {

	if dev, ok := getDevByCallsignSSID(callsignwithssid); ok {
		if addr, _ := dev.getUDPAddr(); addr != nil {
			globelconn.WriteToUDP(encodeNRL21(dev.CallSign, dev.SSID, 3, 0, 0, []byte{0x01}), addr)
		}

		time.Sleep(300 * time.Millisecond)

		return dev, nil

	}

	return dev, fmt.Errorf("dev not found with callsign and ssid %v ", callsignwithssid)

}

func changeDeviceByteParm(callsignssid string, offset int, str string) (res []byte, err error) {

	val, _ := strconv.Atoi(str)

	if d, ok := getDevByCallsignSSID(callsignssid); ok {

		d.DeviceParm.data[offset] = byte(val)
		newpacket := encodeNRL21(d.CallSign, d.SSID, 3, 0, 0, []byte{0x03})
		newpacket = append(newpacket, d.DeviceParm.data...)
		if addr, _ := d.getUDPAddr(); addr != nil {
			globelconn.WriteToUDP(newpacket, addr)
		}
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

	if d, ok := getDevByCallsignSSID(callsignssid); ok {

		d.DeviceParm.data[64] = byte(ssid)
		copy(d.DeviceParm.data[65:71], callsign)
		if len(callsign) == 5 {
			d.DeviceParm.data[70] = 0
		}
		if len(callsign) == 4 {
			d.DeviceParm.data[69] = 0
		}
		newpacket := encodeNRL21(d.CallSign, d.SSID, 3, 0, 0, []byte{0x03})
		newpacket = append(newpacket, d.DeviceParm.data...)
		if addr, _ := d.getUDPAddr(); addr != nil {
			globelconn.WriteToUDP(newpacket, addr)
		}
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

	if d, ok := getDevByCallsignSSID(callsignssid); ok {

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

		newpacket := encodeNRL21(d.CallSign, d.SSID, 3, 0, 0, []byte{0x03})
		newpacket = append(newpacket, d.DeviceParm.data...)
		if addr, _ := d.getUDPAddr(); addr != nil {
			globelconn.WriteToUDP(newpacket, addr)
		}
		time.Sleep(200 * time.Millisecond)

		rescode, _ := jsonextra.Marshal(d)
		return rescode, nil

	}

	return nil, errors.New("device is not found")

}

func DeviceAT(at *ATcommand) (dev *deviceInfo, err error) {

	if d, ok := getDevByCallsignSSID(getCallsignSSID(at.CallSign, at.SSID)); ok {

		atcommand := append([]byte{0x01}, []byte(at.String())...)

		packet := encodeNRL21(at.CallSign, at.SSID, 11, 11, d.DMRID, []byte(atcommand))

		if addr, _ := d.getUDPAddr(); addr != nil && globelconn != nil {
			globelconn.WriteToUDP(packet, addr)
		}

		return d, nil

	}

	return nil, errors.New("device is not found")

}

func changeDeviceUint16Parm(callsignssid string, offset int, str string) (res []byte, err error) {

	val, _ := strconv.Atoi(str)

	if d, ok := getDevByCallsignSSID(callsignssid); ok {

		d.DeviceParm.data[offset+1] = byte(val & 0xFF)
		d.DeviceParm.data[offset] = byte(val >> 8)

		newpacket := encodeNRL21(d.CallSign, d.SSID, 3, 0, 0, []byte{0x03})
		newpacket = append(newpacket, d.DeviceParm.data...)
		if addr, _ := d.getUDPAddr(); addr != nil {
			globelconn.WriteToUDP(newpacket, addr)
		}
		time.Sleep(200 * time.Millisecond)

		rescode, _ := jsonextra.Marshal(d)
		return rescode, nil

	}

	return nil, errors.New("device is not found")

}

func changeDevice1W(ctr *control) (res []byte, err error) {

	if d, ok := getDevByCallsignSSID(getCallsignSSID(ctr.CallSign, ctr.SSID)); ok {

		oneParm := bytes.Split(bytes.Split(d.DeviceParm.data[128:160], []byte{0x00})[0], []byte{','})
		if len(oneParm) < 6 {
			oneParm = make([][]byte, 6)
		}
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

		newpacket := append(encodeNRL21(d.CallSign, d.SSID, 3, 0, 0, []byte{0x03}), d.DeviceParm.data...)
		if addr, _ := d.getUDPAddr(); addr != nil {
			globelconn.WriteToUDP(newpacket, addr)
		}
		time.Sleep(200 * time.Millisecond)

		rescode, _ := jsonextra.Marshal(d)
		return rescode, nil

	}

	return nil, errors.New("device is not found")

}

func changeDevice2W(ctr *control) (res []byte, err error) {

	if d, ok := getDevByCallsignSSID(getCallsignSSID(ctr.CallSign, ctr.SSID)); ok {

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

		newpacket := encodeNRL21(d.CallSign, d.SSID, 3, 0, 0, []byte{0x03})
		newpacket = append(newpacket, d.DeviceParm.data...)
		if addr, _ := d.getUDPAddr(); addr != nil {
			globelconn.WriteToUDP(newpacket, addr)
		}
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
		 VALUES ('','',0,?,0,?,?,?,?,?,'','',0,false,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`

	_, err := db.Exec(query, dev.DevModel, dev.GroupID, dev.CallSign, strconv.Itoa(int(dev.SSID)), dev.DMRID, dev.ChanName)

	if err != nil {
		log.Println("add dev failed, ", err, '\n', query)
		return err
	}

	return nil

}

func offlineDevice(dev string) {

	log.Println("offlineDevice:", dev)

	if d, ok := getDevByCallsignSSID(dev); ok {

		//delete(publicGroupMap[d.GroupID].connPool.devConnMap, d.udpAddr.String())
		d.clearUDPAddr()
		d.setOnline(false)

	} else {
		log.Println("device is not found", dev)
	}
}

// getOnlineDeviceByID 按设备ID在内存设备表中查找设备，
// 用于请求参数缺少 callsign/ssid 时定位内存中的设备记录。
func getOnlineDeviceByID(id int) *deviceInfo {
	var found *deviceInfo
	forEachDev(func(_ string, d *deviceInfo) bool {
		if d.ID == id {
			found = d
			return false
		}
		return true
	})
	return found
}

func delDevice(dev *deviceInfo) error {

	//	fmt.Println("user:", e)
	query := `delete from devices where id=? `

	_, err := db.Exec(query, dev.ID)

	if err != nil {
		log.Println("delete dev failed, ", err, '\n', query)
		return err
	}

	// 内存中的设备以实际记录为准，请求参数可能缺少 callsignssid
	d := getOnlineDeviceByID(dev.ID)
	if d == nil {
		if dd, ok := getDevByCallsignSSID(dev.CallSignSSID); ok {
			d = dd
		}
	}

	if d != nil {
		deleteDevByCallsignSSID(d.CallSignSSID)
		addr, addrStr := d.getUDPAddr()

		if isPublicGroupID(d.GroupID) {
			if g, ok := getPublicGroup(d.GroupID); ok {
				g.deleteDevice(d.ID)
				if addr != nil {
					g.connPool.removeDevice(addrStr)
				}
			}
		} else if isPrivateGroupID(d.GroupID) {
			if user, ok := userlist.Load(d.CallSign); ok {
				if oldGroup, exists := user.(*userinfo).Groups[d.GroupID]; exists {
					oldGroup.deleteDevice(d.ID)
					if addr != nil {
						oldGroup.connPool.removeDevice(addrStr)
					}
				}
			}
		}

	}

	return nil

}

func updateDevice(e *deviceInfo) error {

	d, ok := getDevByCallsignSSID(getCallsignSSID(e.CallSign, e.SSID))

	//先切内存中的组，校验失败时不落库，避免库和内存不一致
	if ok && d.GroupID != e.GroupID {

		if d.DevModel == 255 {
			return errors.New("255设备不能移出999组")
		}

		if d.DevModel == 200 && e.GroupID == 999 {
			return errors.New("200设备不能加入255组")
		}

		if _, err := changeDevGroup(d, e.GroupID); err != nil {
			return err
		}
	}

	// Password 不再随接口返回，前端提交空密码表示不修改，从内存或数据库取回旧值
	if e.Password == "" {
		if ok {
			e.Password = d.Password
		} else if old, err := getDevice(e.CallSign, e.SSID); err == nil {
			e.Password = old.Password
		} else {
			return errors.New("设备不存在，无法保留原密码")
		}
	}

	_, err := db.Exec(`update devices set name=?, gird=?, dmrid=?, dev_type=?, dev_model=?, 	group_id=?,status=?,priority=?,
	chan_name=?,rf_type=?,note=?,password=?,update_time=CURRENT_TIMESTAMP  where id=?`,
		e.Name, e.Gird, e.DMRID, e.DevType, e.DevModel, e.GroupID, e.Status, e.Priority, e.ChanName, e.RFType, e.Note, e.Password, e.ID)
	if err != nil {
		log.Println("update device failed, ", err)
		return err
	}

	if ok {
		d.stateMu.Lock()
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
		d.stateMu.Unlock()
	}

	return nil

}

// 将设备当前所属群组持久化到数据库，供不经过 updateDevice 的切组路径使用
func updateDeviceGroupID(dev *deviceInfo) error {
	return updateDeviceGroupIDByID(dev.ID, dev.GroupID)
}

func updateDeviceGroupIDByID(devID, groupID int) error {
	_, err := db.Exec(`update devices set group_id=?, update_time=CURRENT_TIMESTAMP where id=?`, groupID, devID)
	if err != nil {
		log.Println("update device group failed, ", err)
		return err
	}

	return nil
}

type groupPersistTask struct {
	devID   int
	groupID int
}

var (
	groupPersistOnce sync.Once
	groupPersistCh   chan groupPersistTask
)

// enqueueUpdateDeviceGroupID persists device group changes sequentially on a
// background worker so UDP type 7 group switching does not block packet processing.
func enqueueUpdateDeviceGroupID(devID, groupID int) {
	groupPersistOnce.Do(func() {
		groupPersistCh = make(chan groupPersistTask, 256)
		go func() {
			for task := range groupPersistCh {
				if err := updateDeviceGroupIDByID(task.devID, task.groupID); err != nil {
					log.Println("save device group err:", err)
				}
			}
		}()
	})

	select {
	case groupPersistCh <- groupPersistTask{devID: devID, groupID: groupID}:
	default:
		go func() {
			if err := updateDeviceGroupIDByID(devID, groupID); err != nil {
				log.Println("save device group err:", err)
			}
		}()
	}
}

func changeDeviceGroup(e *deviceInfo) error {

	if d, ok := getDevByCallsignSSID(getCallsignSSID(e.CallSign, e.SSID)); ok {

		if d.GroupID != e.GroupID {
			_, err := changeDevGroup(d, e.GroupID)
			if err != nil {
				return err
			}

			if err := updateDeviceGroupID(d); err != nil {
				return err
			}

		}

	}

	return nil

}

// getDeviceByID 按数据库主键查询设备，用于写操作前的归属校验（防止 IDOR）
func getDeviceByID(id int) (dev *deviceInfo, err error) {
	dev = &deviceInfo{}

	row := db.QueryRow(`select id,name,callsign,
	CAST(ssid AS INTEGER) AS ssid,		priority,
	dmrid,password,gird,dev_type,dev_model,
	group_id,status,is_certed,chan_name,
	create_time,update_time,online_time,note,rf_type
 from  devices   where id=?`, id)

	err = row.Scan(&dev.ID, &dev.Name, &dev.CallSign, &dev.SSID, &dev.Priority, &dev.DMRID, &dev.Password, &dev.Gird, &dev.DevType, &dev.DevModel,
		&dev.GroupID, &dev.Status, &dev.ISCerted, &dev.ChanName,
		&dev.CreateTime, &dev.UpdateTime, &dev.OnlineTime, &dev.Note, &dev.RFType)

	if err != nil {
		log.Println("query device by id err:", err)
		return nil, err
	}

	dev.CallSignSSID = getCallsignSSID(dev.CallSign, dev.SSID)

	return dev, nil
}
