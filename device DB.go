package main

import (
	"log"
	"time"
)

type deviceInfo struct {
	ID            int       `json:"id" db:"id"` //设备唯一编号
	Name          string    `json:"name" db:"name"`
	CPUID         string    `json:"cpuid" db:"cpuid"`                     //设备CPUID
	Gird          string    `json:"gird" db:"gird"`                       //设备位置
	DevType       int       `json:"dev_type" db:"dev_type"`               //设备型号
	DevModel      int       `json:"dev_model" db:"dev_model"`             //设备型号
	CallSign      string    `json:"callsign" db:"callsign"`               //所有者呼号
	SSID          byte      `json:"ssid" db:"ssid"`                       //所有者呼号
	OwerID        int       `json:"ower_id" db:"ower_id"`                 //所有者ID
	PublicGroupID int       `json:"public_group_id" db:"public_group_id"` //公共群组编号
	GroupID       int       `json:"group_id" db:"group_id"`               //内置群租编号
	Status        int       `json:"status" db:"status"`                   //状态  0 未知   1 正常 2 拉黑
	ISCerted      bool      `json:"is_certed" db:"is_certed"`             //是否认证过
	ISOnline      bool      `json:"is_online" db:"is_online"`             //当前是否在线
	CreateTime    time.Time `json:"create_time" db:"create_time"`         //加入时间
	UpdateTime    time.Time `json:"update_time" db:"update_time"`         //信息更新时间
	OnlineTime    time.Time `json:"online_time" db:"online_time"`         //设备上线时间
	Note          string    `json:"note" db:"note"`                       //设备上线时间

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

		if kk, ok := publicGroupMap[dev.PublicGroupID]; ok {

			kk.DevMap[dev.ID] = dev
			kk.DevList = append(kk.DevList, int64(dev.ID))

		}

		if kk, okok := userlist[dev.OwerID]; okok {
			kk.DevList[dev.ID] = dev

		}

	}

}

func getDevice(cpuid string) (dev *deviceInfo) {
	dev = &deviceInfo{}

	//query := "SELECT  id,name,phone,to_char(birthday,'YYYY-MM-DD') as birthday,to_char(job_time,'YYYY-MM-DD') as job_time,sex,position,avatar,roles,update_time FROM user where id=$1"

	//fmt.Println(id, query)
	err := db.Get(dev, `select * FROM devices  where id=$1`, cpuid)
	if err != nil {
		log.Println("get dev by CPUID err:", err, dev, cpuid)
	}
	return dev

}

func bindDevice(dev *deviceInfo, userid int) error {

	if dev.ID == 0 {

		//	fmt.Println("user:", e)
		query := `INSERT INTO devices (	name,cpuid,gird,dev_type,dev_model,callsign,ssid,ower_id,online_time,note,create_time,update_time) 
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,now(),now())`

		_, err := db.Exec(query,
			dev.Name, dev.CPUID, dev.Gird, dev.DevType, dev.DevModel, dev.CallSign, dev.SSID, userid, dev.OnlineTime, dev.Note)

		if err != nil {
			log.Println("bing dev failed, ", err, '\n', query)
			return err
		}

		d := getDevice(dev.CPUID)

		devCPUIDMap[dev.CPUID].OwerID = userid

		if u, ok := userlist[userid]; ok {
			if _, okok := u.DevList[d.ID]; !okok {
				dev.ID = d.ID
				u.DevList[d.ID] = dev
			}

		}

		return nil
	} else {

		_, err := db.Exec(`update devices set ower_id=$1  where id=$2`, userid, dev.ID)

		if err != nil {
			log.Println("rebind device failed, ", err)
			return err
		}
		devCPUIDMap[dev.CPUID].OwerID = userid
	}
	return nil

}

func unbindDevice(dev *deviceInfo) error {

	if dev.ID == 0 {

		//	fmt.Println("user:", e)
		query := `INSERT INTO devices (	name,cpuid,gird,dev_type,dev_model,callsign,ssid,ower_id,create_time,update_time	) 
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8, now(),now())`

		_, err := db.Exec(query,
			dev.Name, dev.CPUID, dev.Gird, dev.DevType, dev.DevModel, dev.CallSign, dev.SSID, 0)

		if err != nil {
			log.Println("bing dev failed, ", err, '\n', query)
			return err
		}

		devCPUIDMap[dev.CPUID].OwerID = 0

		return nil
	} else {

		_, err := db.Exec(`update devices set ower_id=$1  where id=$2`, 0, dev.ID)

		if err != nil {
			log.Println("rebind device failed, ", err)
			return err
		}
		devCPUIDMap[dev.CPUID].OwerID = 0
	}
	return nil

}

func updateDevice(e *deviceInfo) error {

	_, err := db.Exec(`update devices set name=$1, gird=$2, dev_type=$3, dev_model=$4, 
	public_group_id=$5,group_id=$6, note=$7,update_time=now()  where id=$8`,
		e.Name, e.Gird, e.DevType, e.DevModel, e.PublicGroupID, e.GroupID, e.Note, e.ID)
	if err != nil {
		log.Println("update device failed, ", err)
		return err
	}

	if d, ok := devCPUIDMap[e.CPUID]; ok {
		d.Name = e.Name
		d.Gird = e.Gird
		d.DevType = e.DevType
		d.DevModel = e.DevModel

		if d.PublicGroupID != e.PublicGroupID {
			addDevToGroup(d, e.GroupID)
			d.PublicGroupID = e.PublicGroupID

		}

		if d.GroupID != e.GroupID {
			if u, okok := userlist[e.OwerID]; okok {
				u.addDevToRoom(d, e.GroupID)
				d.GroupID = e.GroupID
			}
		}
	}

	return nil

}
