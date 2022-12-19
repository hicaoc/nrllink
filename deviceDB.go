package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/lib/pq"
)

type deviceInfo struct {
	ID              int    `json:"id" db:"id"` //设备唯一编号
	Name            string `json:"name" db:"name"`
	CPUID           string `json:"cpuid" db:"cpuid"`         //设备CPUID
	Password        string `json:"password" db:"password"`   //设备接入密码
	Gird            string `json:"gird" db:"gird"`           //设备位置
	DevType         int    `json:"dev_type" db:"dev_type"`   //设备型号
	DevModel        byte   `json:"dev_model" db:"dev_model"` //设备型号
	VoiceServerIP   string `json:"voice_server_ip"`
	VoiceServerPort string `json:"voice_server_port"`
	CallSign        string `json:"callsign"`                           //所有者呼号
	SSID            byte   `json:"ssid" db:"ssid"`                     //所有者呼号
	GroupID         int    `json:"group_id" db:"group_id"`             //内置群租编号
	GroupPassword   string `json:"group_password" db:"group_password"` //加入组的密码
	Status          int    `json:"status" db:"status"`                 //状态  0 正常   1 禁用接收  2 禁用发射  3 禁止发射和接收
	RFType          int    `json:"rf_type" db:"rf_type"`               //连接的射频设备类型：  0:无连接， 1,内置1W模块，2，内置2W模块，3，外接moto3188，4,moto3688, 5，外接yaesu，6，外接，icom，7，外接其它
	ISCerted        bool   `json:"is_certed" db:"is_certed"`           //是否认证过
	Traffic         int    `json:"traffic"`                            //流量消耗

	udpAddr    *net.UDPAddr
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

	CtlTime          int       `json:"ctl_time"`            //通话时长
	LastCtlBeginTime time.Time `json:"last_ctl_begin_time"` //上次控制开始时间
	LastCtlEndTime   time.Time `json:"last_ctl_end_time"`   //最后控制时间
	LastCtlDuration  int       `json:"last_ctl_duration"`   //上次控制持续时长  秒

	Note       string   `json:"note" db:"note"` //设备上线时间
	DeviceParm *control `json:"device_parm"`
}

func initAllDevList() {

	rows, err := db.Query(`SELECT id,name,cpuid,password,gird,dev_type,dev_model,
		group_id,status,is_certed,chan_name,
		create_time,update_time,online_time,note,rf_type  
	 from  devices`)

	if err != nil {
		log.Println("query all device list  err:", err)
	}

	for rows.Next() {

		dev := &deviceInfo{}
		err := rows.Scan(&dev.ID, &dev.Name, &dev.CPUID, &dev.Password, &dev.Gird, &dev.DevType, &dev.DevModel,
			&dev.GroupID, &dev.Status, &dev.ISCerted, &dev.ChanName,
			&dev.CreateTime, &dev.UpdateTime, &dev.OnlineTime, &dev.Note, &dev.RFType)
		if err != nil {
			log.Println("query  all device rows err:", err)
		}

		devCPUIDMap[dev.CPUID] = dev

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

			}

		}

	}

}

func (d *deviceInfo) String() string {
	return fmt.Sprintf("ID:%v callsign:%v cpuid:%v", d.ID, d.CallSign, d.CPUID)

}

func getDevice(cpuid string) (dev *deviceInfo) {
	dev = &deviceInfo{}

	row := db.QueryRow(`select id,name,cpuid,password,gird,dev_type,dev_model,
	group_id,status,is_certed,chan_name,
	create_time,update_time,online_time,note,rf_type  
 from  devices   where cpuid=?`, cpuid)

	err := row.Scan(&dev.ID, &dev.Name, &dev.CPUID, &dev.Password, &dev.Gird, &dev.DevType, &dev.DevModel,
		&dev.GroupID, &dev.Status, &dev.ISCerted, &dev.ChanName,
		&dev.CreateTime, &dev.UpdateTime, &dev.OnlineTime, &dev.Note, &dev.RFType)

	if err != nil {
		log.Println("query one device rows err:", err)
	}

	return dev

}

func queryDeviceParm(cpuid string) (dev deviceInfo, err error) {

	if dev, ok := devCPUIDMap[cpuid]; ok {

		t := time.Now()
		//fmt.Println(t.Sub(d.LastPacketTime))
		if t.Sub(dev.LastPacketTime) > 15*time.Second {
			dev.ISOnline = false
			return *dev, fmt.Errorf("dev offline: %v-%v %v ", dev.CPUID, dev.SSID, cpuid)

		} else {

			globelconn.WriteToUDP(encodeDeviceParm(dev, 0x01), dev.udpAddr)

			time.Sleep(300 * time.Millisecond)

			return *dev, nil
		}

	}

	return dev, fmt.Errorf("dev not found with cpuid %v ", cpuid)

}

func changeDeviceByteParm(cpuid string, offset int, str string) (res []byte, err error) {

	val, _ := strconv.Atoi(str)

	if d, ok := devCPUIDMap[cpuid]; ok {

		t := time.Now()
		// fmt.Println(t.Sub(d.LastPacketTime))
		if t.Sub(d.LastPacketTime) > 5*time.Second {
			d.ISOnline = false
			return nil, errors.New("device be offline")

		} else {
			d.DeviceParm.data[offset] = byte(val)
			newpacket := append(encodeDeviceParm(d, 0x03), d.DeviceParm.data...)
			globelconn.WriteToUDP(newpacket, d.udpAddr)
			time.Sleep(200 * time.Millisecond)

			rescode, _ := jsonextra.Marshal(d)
			return rescode, nil

		}

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

func changeDeviceIPParm(cpuid string, ip ipparm) (res []byte, err error) {

	if len(ip.destIPValue) != 15 {
		return nil, fmt.Errorf("DIP format err")

	}

	lip, lipok := checkIP(ip.localIPValue)
	netmask, netmaskok := checkIP(ip.localNetmaskValue)
	gateway, gatewayok := checkIP(ip.gatewayValue)
	dns, dnsok := checkIP(ip.dnsValue)

	if !(lipok && netmaskok && gatewayok && dnsok) {
		return nil, errors.New("ip format error")
	}

	if d, ok := devCPUIDMap[cpuid]; ok {

		t := time.Now()
		// fmt.Println(t.Sub(d.LastPacketTime))
		if t.Sub(d.LastPacketTime) > 5*time.Second {
			d.ISOnline = false
			return nil, errors.New("device be offline")

		} else {
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

			newpacket := append(encodeDeviceParm(d, 0x03), d.DeviceParm.data...)
			globelconn.WriteToUDP(newpacket, d.udpAddr)
			time.Sleep(200 * time.Millisecond)

			rescode, _ := jsonextra.Marshal(d)
			return rescode, nil

		}

	}

	return nil, errors.New("device is not found")

}

func changeDeviceUint16Parm(cpuid string, offset int, str string) (res []byte, err error) {

	val, _ := strconv.Atoi(str)

	if d, ok := devCPUIDMap[cpuid]; ok {

		t := time.Now()
		// fmt.Println(t.Sub(d.LastPacketTime))
		if t.Sub(d.LastPacketTime) > 5*time.Second {
			d.ISOnline = false
			return nil, errors.New("device be offline")

		} else {
			d.DeviceParm.data[offset+1] = byte(val & 0xFF)
			d.DeviceParm.data[offset] = byte(val >> 8)

			newpacket := append(encodeDeviceParm(d, 0x03), d.DeviceParm.data...)
			globelconn.WriteToUDP(newpacket, d.udpAddr)
			time.Sleep(200 * time.Millisecond)

			rescode, _ := jsonextra.Marshal(d)
			return rescode, nil

		}

	}

	return nil, errors.New("device is not found")

}

func changeDevice1W(ctr *control) (res []byte, err error) {

	if d, ok := devCPUIDMap[ctr.LocalCPUID]; ok {

		oneParm := bytes.Split(bytes.Split(d.DeviceParm.data[128:160], []byte{0x00})[0], []byte{','})
		oneParm[1] = []byte(ctr.OneTransmitFreq)
		oneParm[2] = []byte(ctr.OneReciveFreq)
		oneParm[3] = []byte(ctr.OneReciveCXCSS)
		oneParm[4] = []byte(strconv.Itoa(ctr.OneSQLLevel))
		oneParm[5] = []byte(ctr.OneTransmitCXCSS)

		p := bytes.Join(oneParm, []byte{','})

		fmt.Println("One w:", string(p))

		t := time.Now()
		// fmt.Println(t.Sub(d.LastPacketTime))
		if t.Sub(d.LastPacketTime) > 5*time.Second {
			d.ISOnline = false
			return nil, errors.New("device be offline")

		} else {
			copy(d.DeviceParm.data[128:], p)
			d.DeviceParm.data[160] = []byte(strconv.Itoa(ctr.OneVolume))[0]
			d.DeviceParm.data[161] = []byte(strconv.Itoa(ctr.OneMICSensitivity))[0]

			newpacket := append(encodeDeviceParm(d, 0x03), d.DeviceParm.data...)
			globelconn.WriteToUDP(newpacket, d.udpAddr)
			time.Sleep(200 * time.Millisecond)

			rescode, _ := jsonextra.Marshal(d)
			return rescode, nil

		}

	}

	return nil, errors.New("device is not found")

}

func changeDevice2W(ctr *control) (res []byte, err error) {

	if d, ok := devCPUIDMap[ctr.LocalCPUID]; ok {

		//oneParm := bytes.Split(bytes.Split(d.DeviceParm.data[128:160], []byte{0x00})[0], []byte{','})

		twoParm := bytes.Split(bytes.Split(d.DeviceParm.data[192:227], []byte{0x00})[0], []byte{','})

		twoParm[0] = []byte(ctr.TwoReciveFreq)
		twoParm[1] = []byte(ctr.TwoTransmitFreq)
		twoParm[2] = []byte(ctr.TwoReciveCXCSS)
		twoParm[3] = []byte(ctr.TwoTransmitCXCSS)

		p := bytes.Join(twoParm, []byte{','})

		fmt.Println("two w : ", string(p))

		p = append(p, byte(0x00))

		t := time.Now()
		// fmt.Println(t.Sub(d.LastPacketTime))
		if t.Sub(d.LastPacketTime) > 5*time.Second {
			d.ISOnline = false
			return nil, errors.New("device be offline")

		} else {
			copy(d.DeviceParm.data[192:], p)

			d.DeviceParm.data[238] = []byte(strconv.Itoa(ctr.TwoVolume))[0]
			d.DeviceParm.data[239] = []byte(strconv.Itoa(ctr.TwoSavePower))[0]
			d.DeviceParm.data[240] = []byte(strconv.Itoa(ctr.TwoSQLLevel))[0]
			d.DeviceParm.data[242] = []byte(strconv.Itoa(ctr.TwoMICLevel))[0]
			d.DeviceParm.data[244] = []byte(strconv.Itoa(ctr.TwoTOTLevel))[0]

			newpacket := append(encodeDeviceParm(d, 0x03), d.DeviceParm.data...)
			globelconn.WriteToUDP(newpacket, d.udpAddr)
			time.Sleep(200 * time.Millisecond)

			rescode, _ := jsonextra.Marshal(d)
			return rescode, nil

		}

	}

	return nil, errors.New("device is not found")

}

func addDevice(dev *deviceInfo) error {

	//	fmt.Println("user:", e)
	query := `INSERT INTO devices (	name,gird,dev_type,dev_model,status,group_id,cpuid,chan_name,note,password,rf_type,is_certed,online_time,create_time,update_time)
		 VALUES ('','',0,0,0,0,?,?,'','',0,false,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`

	_, err := db.Exec(query, dev.CPUID, dev.ChanName)

	if err != nil {
		log.Println("add dev failed, ", err, '\n', query)
		return err
	}

	return nil

}

func updateDevice(e *deviceInfo) error {

	if d, ok := devCPUIDMap[e.CPUID]; ok {
		d.Name = e.Name
		d.Gird = e.Gird
		d.DevType = e.DevType
		d.DevModel = e.DevModel
		d.Status = e.Status
		d.Note = e.Note
		d.Password = e.Password
		d.RFType = e.RFType
		d.ChanName = e.ChanName

		if d.GroupID != e.GroupID {

			if g, okok := publicGroupMap[e.GroupID]; okok {

				if e.GroupID == 0 || g.Password == "" || e.Password == g.Password {

					err := changeDevGroup(d, e.GroupID)
					if err != nil {
						return err
					}

				} else {
					fmt.Println("group password:", e.Password, []byte(g.Password), len(e.Password), len(g.Password), g)
					return fmt.Errorf("group pasword err")
				}

			} else if e.GroupID < 1000 && e.GroupID != 0 {
				err := changeDevGroup(d, e.GroupID)
				if err != nil {
					return err
				}

			} else {
				return fmt.Errorf("group not found")

			}

		}

	}

	_, err := db.Exec(`update devices set name=?, gird=?, dev_type=?, dev_model=?, 	group_id=?,status=?,
	chan_name=?,rf_type=?,note=?,password=?,update_time=CURRENT_TIMESTAMP  where id=?`,
		e.Name, e.Gird, e.DevType, e.DevModel, e.GroupID, e.Status, e.ChanName, e.RFType, e.Note, e.Password, e.ID)
	if err != nil {
		log.Println("update device failed, ", err)
		return err
	}

	return nil

}

func changeDeviceGroup(e *deviceInfo) error {

	if d, ok := devCPUIDMap[e.CPUID]; ok {

		if d.GroupID != e.GroupID {

			if g, okok := publicGroupMap[e.GroupID]; okok {

				if e.GroupID == 0 || g.Password == "" || e.Password == g.Password {

					err := changeDevGroup(d, e.GroupID)
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
				err := changeDevGroup(d, e.GroupID)
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
