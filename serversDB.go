package main

import (
	"fmt"
	"log"
	"time"

	"github.com/lib/pq"
)

var ServersMap = make(map[int]*Server, 1000) //key 房间号

type Server struct {
	ID           int                 `json:"id" db:"id"`
	Name         string              `json:"name" db:"name"`
	JoinKey      string              `json:"join_key" db:"join_key"`
	CpuType      int                 `json:"cpu_type" db:"cpu_type"`
	MemSize      int                 `json:"mem_size" db:"mem_size"`
	InputRate    int                 `json:"input_rate" db:"input_rate"`
	OuputRate    int                 `json:"output_rate" db:"output_rate"`
	NetCard      string              `json:"netcard" db:"netcard"`         //网卡
	IPType       int                 `json:"ip_type" db:"ip_type"`         //动态，静态，是否经过nat
	IPAddr       string              `json:"ip_addr" db:"ip_addr"`         //动态IP可以写0.0.0.0
	DNSName      string              `json:"dns_name" db:"dns_name"`       //域名
	ServerType   int                 `json:"server_type" db:"server_type"` //服务器类型： 物理机，虚拟机，树莓派等
	GroupList    pq.Int64Array       `json:"group_list" db:"group_list"`   //服务器负责的群组列表
	DevMap       map[int]*deviceInfo `json:"devmap" `                      //key: 设备列表
	ISOnline     bool                `json:"is_online"`                    //服务器是否在线
	Status       int                 `json:"status" db:"status"`
	OwerID       int                 `json:"ower_id" db:"ower_id"`             //谁的服务器
	OwerCallsign string              `json:"ower_callsign" db:"ower_callsign"` //服务器所有者呼号
	CreateTime   time.Time           `json:"create_time" db:"create_time"`
	UpdateTime   time.Time           `json:"update_time" db:"update_time"`
	Note         string              `json:"note" db:"note"`
}

func (p *Server) String() string {

	return fmt.Sprintf("id:%v,name:%v,cputype:%v,status:%v", p.ID, p.Name, p.CpuType, p.Status)

}

func initServers() {

	rows, err := db.Queryx("SELECT * from  servers")

	if err != nil {
		log.Println("query all server list  err:", err)
	}

	for rows.Next() {
		pg := &Server{DevMap: make(map[int]*deviceInfo, 10)}
		err := rows.StructScan(pg)
		if err != nil {
			log.Println("query  server rows err:", err)
		}

		ServersMap[pg.ID] = pg

	}

}

// func addDevToServer(dev *deviceInfo, Serversid int) (err error) {

// 	//从之前的服务器

// 	if g, ok := ServersMap[dev.ID]; ok {
// 		delete(g.DevMap, dev.ID)
// 	}

// 	//加入新的服务器

// 	if g, ok := ServersMap[dev.ID]; ok {
// 		dev.ID = Serversid
// 		g.DevMap[dev.ID] = dev

// 	}

// 	return

// }

// func delDevFromGroup(dev *deviceInfo) (err error) {

// 	if g, ok := ServersMap[dev.ServersID]; ok {
// 		delete(g.DevMap, dev.ID)
// 		dev.GroupID = 0
// 	}

// 	return

// }

// func (u *userinfo) addGroupToServer(dev *deviceInfo, roomid int) (err error) {

// 	//加入新的组

// 	if g, ok := u.DevList[dev.ID]; ok {

// 		g.GroupID = roomid

// 	} else {

// 		dev.GroupID = roomid

// 		u.DevList[dev.ID] = dev

// 	}

// 	return

// }

func addServers(s *Server) error {

	//	fmt.Println("user:", e)
	query := `INSERT INTO servers (name,join_key,cpu_type,mem_size,input_rate,output_rate,netcard,
		ip_type,ip_addr,dns_name,server_type,group_list,ower_id,ower_callsign,status,note,create_time,update_time) 
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,now(),now()) `

	resault, err := db.Exec(query,
		s.Name, s.JoinKey, s.CpuType, s.MemSize, s.InputRate, s.OuputRate, s.NetCard,
		s.IPType, s.IPAddr, s.DNSName, s.ServerType, s.GroupList, s.OwerID, s.OwerCallsign, s.Status, s.Note)

	if err != nil {
		log.Println("add server failed, ", err, '\n', query)
		return err
	} else {
		fmt.Println("resault:", resault)
	}

	return nil

}

func updateServer(s *Server) error {

	_, err := db.Exec(`update servers set name=$1,cpu_type=$2,mem_size=$3,input_rate=$4,output_rate=$5,netcard=$6,
	ip_type=$7,ip_addr=$8,dns_name=$9,server_type=$10,group_list=$11,ower_id=$12,ower_callsign=$13,status=$14,note=$15,join_key=$16,update_time=now() where id=$17`,
		s.Name, s.CpuType, s.MemSize, s.InputRate, s.OuputRate, s.NetCard,
		s.IPType, s.IPAddr, s.DNSName, s.ServerType, s.GroupList, s.OwerID, s.OwerCallsign, s.Status, s.Note, s.JoinKey, s.ID)
	if err != nil {
		log.Println("update server failed, ", err)
		return err
	}

	initServers()

	return nil

}

func deleteServer(s *Server) error {

	_, err := db.Exec(`delete from servers  where id=$1`, s.ID)
	if err != nil {
		log.Println("delete server failed, ", err)
		return err
	}

	return nil

}
