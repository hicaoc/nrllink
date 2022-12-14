package main

import (
	"fmt"
	"log"
	"sync"
)

//var ServersMap = make(map[int]*Server, 1000) //key 房间号

var ServersMap sync.Map

type Server struct {
	ID           int                 `json:"id" db:"id"`
	Name         string              `json:"name" db:"name"`
	JoinKey      string              `json:"join_key" db:"join_key"`
	CpuType      int                 `json:"cpu_type" db:"cpu_type"`
	MemSize      int                 `json:"mem_size" db:"mem_size"`
	InputRate    int                 `json:"input_rate" db:"input_rate"`
	OuputRate    int                 `json:"output_rate" db:"output_rate"`
	Providers    string              `json:"Providers"`                    //电信，联通，移动，其他
	NetCard      string              `json:"netcard" db:"netcard"`         //网卡
	IPType       int                 `json:"ip_type" db:"ip_type"`         //动态，静态，是否经过nat
	IPAddr       string              `json:"ip_addr" db:"ip_addr"`         //动态IP可以写0.0.0.0
	UDPPort      string              `json:"udp_port" db:"udp_port"`       //服务端口
	DNSName      string              `json:"dns_name" db:"dns_name"`       //域名
	ServerType   int                 `json:"server_type" db:"server_type"` //服务器类型： 物理机，虚拟机，树莓派等
	GroupList    []int               `json:"group_list" `                  //服务器负责的群组列表
	DevMap       map[int]*deviceInfo `json:"devmap" `                      //key: 设备列表
	ISOnline     bool                `json:"is_online"`                    //服务器是否在线
	Status       int                 `json:"status" db:"status"`
	OwerID       int                 `json:"ower_id" db:"ower_id"`             //谁的服务器
	OwerCallsign string              `json:"ower_callsign" db:"ower_callsign"` //服务器所有者呼号
	CreateTime   string              `json:"create_time" db:"create_time"`
	UpdateTime   string              `json:"update_time" db:"update_time"`
	Note         string              `json:"note" db:"note"`
}

func (p *Server) String() string {

	return fmt.Sprintf("id:%v,name:%v,cputype:%v,status:%v", p.ID, p.Name, p.CpuType, p.Status)

}

func initServers() {

	query := `SELECT id,name,join_key,cpu_type,mem_size,
	input_rate,output_rate,netcard,ip_type,ip_addr,
	udp_port,dns_name,server_type,group_list,
	status,ower_id,ower_callsign,create_time,update_time 
	from servers`

	rows, err := db.Query(query)

	if err != nil {
		log.Println("query all server list  err:", err)
	}

	for rows.Next() {
		var grouplist string
		pg := &Server{DevMap: make(map[int]*deviceInfo, 10)}
		err := rows.Scan(&pg.ID, &pg.Name, &pg.JoinKey, &pg.CpuType, &pg.MemSize,
			&pg.InputRate, &pg.OuputRate, &pg.NetCard, &pg.IPType, &pg.IPAddr,
			&pg.UDPPort, &pg.DNSName, &pg.ServerType, &grouplist,
			&pg.Status, &pg.OwerID, &pg.OwerCallsign, &pg.CreateTime, &pg.UpdateTime)
		if err != nil {
			log.Println("query  server rows err:", err)
		}

		pg.GroupList = convertStr2IntArray(grouplist)

		ServersMap.Store(pg.ID, pg)

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
		ip_type,ip_addr,udp_port,dns_name,server_type,group_list,ower_id,ower_callsign,status,note,create_time,update_time) 
	VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP) `

	grouplist := convertIntArray2Str(s.GroupList)

	resault, err := db.Exec(query,
		s.Name, s.JoinKey, s.CpuType, s.MemSize, s.InputRate, s.OuputRate, s.NetCard,
		s.IPType, s.IPAddr, s.UDPPort, s.DNSName, s.ServerType, grouplist, s.OwerID, s.OwerCallsign, s.Status, s.Note)

	if err != nil {
		log.Println("add server failed, ", err, '\n', query)
		return err
	} else {
		fmt.Println("resault:", resault)
	}

	initServers()

	return nil

}

func updateServer(s *Server) error {

	grouplist := convertIntArray2Str(s.GroupList)

	_, err := db.Exec(`update servers set name=?,cpu_type=?,mem_size=?,input_rate=?,output_rate=?,netcard=?,
	ip_type=?,ip_addr=?,udp_port=?,dns_name=?,server_type=?,group_list=?,ower_id=?,ower_callsign=?,status=?,note=?,join_key=?,update_time=CURRENT_TIMESTAMP where id=?`,
		s.Name, s.CpuType, s.MemSize, s.InputRate, s.OuputRate, s.NetCard,
		s.IPType, s.IPAddr, s.UDPPort, s.DNSName, s.ServerType, grouplist, s.OwerID, s.OwerCallsign, s.Status, s.Note, s.JoinKey, s.ID)
	if err != nil {
		log.Println("update server failed, ", err)
		return err
	}

	initServers()

	return nil

}

func deleteServer(s *Server) error {

	_, err := db.Exec(`delete from servers  where id=?`, s.ID)
	if err != nil {
		log.Println("delete server failed, ", err)
		return err
	}

	return nil

}
