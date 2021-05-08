package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"
)

type deviceInfo struct {
	ID              int    `json:"id" db:"id"` //设备唯一编号
	Name            string `json:"name" db:"name"`
	CPUID           string `json:"cpuid" db:"cpuid"`         //设备CPUID
	Password        string `json:"password" db:"password"`   //设备接入密码
	Gird            string `json:"gird" db:"gird"`           //设备位置
	DevType         int    `json:"dev_type" db:"dev_type"`   //设备型号
	DevModel        int    `json:"dev_model" db:"dev_model"` //设备型号
	VoiceServerIP   string `json:"voice_server_ip"`
	VoiceServerPort string `json:"voice_server_port"`
	CallSign        string `json:"callsign" db:"callsign"`   //所有者呼号
	SSID            byte   `json:"ssid" db:"ssid"`           //所有者呼号
	OwerID          int    `json:"ower_id" db:"ower_id"`     //所有者ID
	GroupID         int    `json:"group_id" db:"group_id"`   //内置群租编号
	Status          int    `json:"status" db:"status"`       //状态  0 未知   1 正常 2 拉黑
	ISCerted        bool   `json:"is_certed" db:"is_certed"` //是否认证过
	Traffic         int    `json:"traffic"`
	udpAddr         *net.UDPAddr
	CreateTime      time.Time `json:"create_time" db:"create_time"` //加入时间
	UpdateTime      time.Time `json:"update_time" db:"update_time"` //信息更新时间
	OnlineTime      time.Time `json:"online_time" db:"online_time"` //设备上线时间
	ISOnline        bool      `json:"is_online" db:"is_online"`     //当前是否在线
	LastPacketTime  time.Time `json:"last_packet_time" `            //最后一次报文时间
	LastVoiceTime   time.Time `json:"last_voice_time"`              //最后语音时间
	Note            string    `json:"note" db:"note"`               //设备上线时间
	DeviceParm      *control  `json:"device_parm"`
}

func initAllDevList() {

	rows, err := db.Queryx("SELECT * from  devices")

	if err != nil {
		log.Println("query all device list  err:", err)
	}

	for rows.Next() {

		dev := &deviceInfo{}
		err := rows.StructScan(dev)
		if err != nil {
			log.Println("query  all device rows err:", err)
		}

		devCPUIDMap[dev.CPUID] = dev

		if kk, ok := publicGroupMap[dev.GroupID]; ok {

			kk.DevMap[dev.ID] = dev
			kk.DevList = append(kk.DevList, int64(dev.ID))

		}

		if kk, okok := userlist[dev.OwerID]; okok {
			kk.DevList[dev.ID] = dev

		}

	}

}

func (d *deviceInfo) String() string {
	return fmt.Sprintf("ID:%v callsign:%v cpuid:%v", d.ID, d.CallSign, d.CPUID)

}

func getDevice(cpuid string) (dev *deviceInfo) {
	dev = &deviceInfo{}

	//query := "SELECT  id,name,phone,to_char(birthday,'YYYY-MM-DD') as birthday,to_char(job_time,'YYYY-MM-DD') as job_time,sex,position,avatar,roles,update_time FROM user where id=$1"

	//fmt.Println(id, query)
	err := db.Get(dev, `select * FROM devices  where cpuid=$1`, cpuid)
	if err != nil {
		log.Println("get dev by CPUID err:", err, dev, cpuid)
	}
	return dev

}

func queryDeviceParm(cpuid string) (dev deviceInfo, err error) {

	if dev, ok := devCPUIDMap[cpuid]; ok {

		fmt.Println(dev)
		fmt.Println(dev.CPUID, dev.CallSign, dev.CreateTime, dev.ID, dev.ISOnline)

		t := time.Now()
		//fmt.Println(t.Sub(d.LastPacketTime))
		if t.Sub(dev.LastPacketTime) > 5*time.Second {
			dev.ISOnline = false
			return *dev, nil

		} else {

			globelconn.WriteToUDP(encodeDeviceParm(dev, 0x01), dev.udpAddr)

			time.Sleep(200 * time.Millisecond)

			return *dev, nil
		}

	}

	return dev, fmt.Errorf("dev not found with cpuid %v ", cpuid)

	//query := "SELECT  id,name,phone,to_char(birthday,'YYYY-MM-DD') as birthday,to_char(job_time,'YYYY-MM-DD') as job_time,sex,position,avatar,roles,update_time FROM user where id=$1"

	//fmt.Println(id, query)

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
			return []byte(fmt.Sprintf(`{"code":20000,"data":{"items":%s}}`, rescode)), nil

		}

	}

	return nil, errors.New("device is not found")

	//query := "SELECT  id,name,phone,to_char(birthday,'YYYY-MM-DD') as birthday,to_char(job_time,'YYYY-MM-DD') as job_time,sex,position,avatar,roles,update_time FROM user where id=$1"

	//fmt.Println(id, query)

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
			return []byte(fmt.Sprintf(`{"code":20000,"data":{"items":%s}}`, rescode)), nil

		}

	}

	return nil, errors.New("device is not found")

	//query := "SELECT  id,name,phone,to_char(birthday,'YYYY-MM-DD') as birthday,to_char(job_time,'YYYY-MM-DD') as job_time,sex,position,avatar,roles,update_time FROM user where id=$1"

	//fmt.Println(id, query)

}

func changeDevice1W(ctr *control) (res []byte, err error) {

	if d, ok := devCPUIDMap[ctr.LocalCPUID]; ok {

		oneParm := bytes.Split(bytes.Split(d.DeviceParm.data[128:160], []byte{0x00})[0], []byte{','})
		oneParm[1] = []byte(ctr.OneReciveFreq)
		oneParm[2] = []byte(ctr.OneTransmitFreq)
		oneParm[3] = []byte(ctr.OneReciveCXCSS)
		oneParm[4] = []byte(strconv.Itoa(ctr.OneSQLLevel))
		oneParm[5] = []byte(ctr.OneTransmitCXCSS)

		p := bytes.Join(oneParm, []byte{','})

		fmt.Println(string(p))

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
			return []byte(fmt.Sprintf(`{"code":20000,"data":{"items":%s}}`, rescode)), nil

		}

	}

	return nil, errors.New("device is not found")

	//query := "SELECT  id,name,phone,to_char(birthday,'YYYY-MM-DD') as birthday,to_char(job_time,'YYYY-MM-DD') as job_time,sex,position,avatar,roles,update_time FROM user where id=$1"

	//fmt.Println(id, query)

}

func bindDevice(dev *deviceInfo, userid int) error {

	if dev.OwerID == 0 {

		//	fmt.Println("user:", e)
		query := `INSERT INTO devices (	name,cpuid,password,gird,dev_type,dev_model,callsign,ssid,ower_id,online_time,note,create_time,update_time) 
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,now(),now())`

		_, err := db.Exec(query,
			dev.Name, dev.CPUID, dev.Password, dev.Gird, dev.DevType, dev.DevModel, dev.CallSign, dev.SSID, userid, dev.OnlineTime, dev.Note)

		if err != nil {
			log.Println("bing dev failed, ", err, '\n', query)
			return err
		}

	} else {

		query := `update devices set ower_id=$1,callsign=$2,update_time=now() where id=$3 `

		_, err := db.Exec(query, userid, dev.CallSign, dev.ID)

		if err != nil {
			log.Println("change bing dev failed, ", err, '\n', query)
			return err
		}

	}

	d := getDevice(dev.CPUID)

	if ddd, ok := devCPUIDMap[dev.CPUID]; ok {
		ddd.OwerID = userid
		ddd.ID = d.ID

	}

	//在原来用户中，删除这个设备
	if u, ok := userlist[dev.OwerID]; ok {
		if _, okok := u.DevList[d.ID]; !okok {
			//dev.ID = d.ID
			delete(u.DevList, d.ID)
		}
	}

	//绑定到新用户
	if u, ok := userlist[userid]; ok {
		if _, okok := u.DevList[d.ID]; !okok {
			//dev.ID = d.ID
			u.DevList[d.ID] = dev
		}
	}

	if kkk, ok := publicGroupMap[0]; ok {
		if kkkk, okok := kkk.DevMap[dev.ID]; okok {
			kkk.DevMap[d.ID] = kkkk
			delete(kkk.DevMap, dev.ID)

		}

	}

	//dev.ID = d.ID

	return nil

}

// func unbindDevice(dev *deviceInfo) error {

// 	if dev.ID == 0 {

// 		//	fmt.Println("user:", e)
// 		query := `INSERT INTO devices (	name,cpuid,gird,dev_type,dev_model,callsign,ssid,ower_id,create_time,update_time	)
// 	VALUES ($1,$2,$3,$4,$5,$6,$7,$8, now(),now())`

// 		_, err := db.Exec(query,
// 			dev.Name, dev.CPUID, dev.Gird, dev.DevType, dev.DevModel, dev.CallSign, dev.SSID, 0)

// 		if err != nil {
// 			log.Println("bing dev failed, ", err, '\n', query)
// 			return err
// 		}

// 		devCPUIDMap[dev.CPUID].OwerID = 0

// 		return nil
// 	} else {

// 		_, err := db.Exec(`update devices set ower_id=$1  where id=$2`, 0, dev.ID)

// 		if err != nil {
// 			log.Println("rebind device failed, ", err)
// 			return err
// 		}
// 		devCPUIDMap[dev.CPUID].OwerID = 0
// 	}
// 	return nil

// }

func updateDevice(e *deviceInfo) error {

	//不更新设备所有者，所有者在绑定的时候一次性生成

	if e.ID > 1000000 {
		return fmt.Errorf("temp device no support change group %v %v ", e.CPUID, e.GroupID)
	}

	if d, ok := devCPUIDMap[e.CPUID]; ok {
		d.Name = e.Name
		d.Gird = e.Gird
		d.DevType = e.DevType
		d.DevModel = e.DevModel
		d.Status = e.Status
		d.Note = e.Note
		d.Password = e.Password

		if d.GroupID != e.GroupID {
			err := changeDevGroup(d, e.GroupID)
			if err != nil {
				return err
			}

		}

	}

	_, err := db.Exec(`update devices set name=$1, gird=$2, dev_type=$3, dev_model=$4, 
	group_id=$5,status=$6, note=$7,password=$8,update_time=now()  where id=$9`,
		e.Name, e.Gird, e.DevType, e.DevModel, e.GroupID, e.Status, e.Note, e.Password, e.ID)
	if err != nil {
		log.Println("update device failed, ", err)
		return err
	}

	return nil

}
