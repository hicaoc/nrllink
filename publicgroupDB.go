package main

import (
	"fmt"
	"log"
	"time"

	"github.com/lib/pq"
)

var publicGroupMap = make(map[int]*publicgroup, 1000) //key 房间号

type publicgroup struct {
	ID               int           `json:"id" db:"id"`
	Name             string        `json:"name" db:"name"`
	Type             int           `json:"type" db:"type"`
	AllowCPUID       string        `json:"allow_cpuid" db:"allow_cpuid"`
	DevList          pq.Int64Array `json:"devlist" db:"devlist"`
	Status           int           `json:"status" db:"status"`
	OwerID           int           `json:"ower_id" db:"ower_id"`
	OwerCallsign     string        `json:"callsign" db:"callsign"`
	MasterServer     int           `json:"master_server" db:"master_server"`
	MasterServerPort int           `json:"master_server_port" db:"master_server_port"`
	BackupServer     int           `json:"backup_server" db:"backup_server"`
	BackupServerPort int           `json:"backup_server_port" db:"backup_server_port"`
	CreateTime       time.Time     `json:"create_time" db:"create_time"`
	UpdateTime       time.Time     `json:"update_time" db:"update_time"`
	Note             string        `json:"note" db:"note"`
	connPool         *currentConnPool
	DevMap           map[int]*deviceInfo `json:"devmap" ` //key: 设备ID
}

func (p *publicgroup) String() string {

	return fmt.Sprintf("id:%v,name:%v,type:%v,status:%v", p.ID, p.Name, p.Type, p.Status)

}

func initPublicGroup() {

	pg0 := &publicgroup{
		ID:           0,
		Name:         "公共大厅",
		OwerCallsign: "default",
		connPool:     &currentConnPool{devConnList: make(map[string]*connPool)},
		DevMap:       make(map[int]*deviceInfo, 10),
		CreateTime:   time.Now(),
		UpdateTime:   time.Now(),
	}

	publicGroupMap[0] = pg0

	rows, err := db.Queryx("SELECT * from  public_groups")

	if err != nil {
		log.Println("query all public group list  err:", err)
	}

	for rows.Next() {
		pg := &publicgroup{}
		err := rows.StructScan(pg)
		if err != nil {
			log.Println("query  all public group rows err:", err)
		}

		pg.connPool = &currentConnPool{devConnList: make(map[string]*connPool)}
		pg.DevMap = make(map[int]*deviceInfo, 10)

		// 类型为3的公共组，只能一个设备转发，用于中继收听
		if pg.Type == 3 {
			pg.connPool.allowCPUID = pg.AllowCPUID
		}

		publicGroupMap[pg.ID] = pg

	}

}

func getGroup(name string) (gp *publicgroup) {
	gp = &publicgroup{}

	//query := "SELECT  id,name,phone,to_char(birthday,'YYYY-MM-DD') as birthday,to_char(job_time,'YYYY-MM-DD') as job_time,sex,position,avatar,roles,update_time FROM user where id=$1"

	//fmt.Println(id, query)
	err := db.Get(gp, `select * FROM public_groups  where name=$1`, name)
	if err != nil {
		log.Println("get group by name err:", err, name)
	}
	return gp

}

func addDevToGroup(dev *deviceInfo, publicgroupid int) (err error) {

	//从之前的组删除

	if g, ok := publicGroupMap[dev.PublicGroupID]; ok {
		delete(g.DevMap, dev.ID)
	}

	//加入新的组

	if g, ok := publicGroupMap[publicgroupid]; ok {
		dev.PublicGroupID = publicgroupid
		g.DevMap[dev.ID] = dev

	}

	return

}

// func delDevFromGroup(dev *deviceInfo) (err error) {

// 	if g, ok := publicGroupMap[dev.PublicGroupID]; ok {
// 		delete(g.DevMap, dev.ID)
// 		dev.GroupID = 0
// 	}

// 	return

// }

func (u *userinfo) addDevToRoom(dev *deviceInfo, roomid int) (err error) {

	//加入新的组

	if g, ok := u.DevList[dev.ID]; ok {

		g.GroupID = roomid

	} else {

		dev.GroupID = roomid

		u.DevList[dev.ID] = dev

	}

	return

}

func addPublicGroup(pg *publicgroup) error {

	//	fmt.Println("user:", e)
	query := `INSERT INTO public_groups (name,type,allow_cpuid,callsign,ower_id,devlist,master_server,backup_server,status,note,create_time,update_time	) 
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,now(),now())`

	_, err := db.Exec(query, pg.Name, pg.Type, pg.AllowCPUID, pg.OwerCallsign, pg.OwerID, pg.DevList, pg.MasterServer, pg.BackupServer, pg.Status, pg.Note)

	if err != nil {
		log.Println("add public group failed, ", err, '\n', query)
		return err
	}
	newpg := getGroup(pg.Name)
	if _, ok := publicGroupMap[newpg.ID]; !ok {
		newpg.connPool = &currentConnPool{devConnList: make(map[string]*connPool)}
		newpg.DevMap = make(map[int]*deviceInfo, 10)
		publicGroupMap[newpg.ID] = newpg
	}

	//initPublicGroup()

	return nil

}

func updatePublicGroup(pg *publicgroup) error {

	_, err := db.Exec(`update public_groups set name=$1, type=$2,   allow_cpuid=$3, status=$4,master_server=$5,backup_server=$6, note=$7 ,update_time=now()  where id=$8`,
		pg.Name, pg.Type, pg.AllowCPUID, pg.Status, pg.MasterServer, pg.BackupServer, pg.Note, pg.ID)

	if err != nil {
		log.Println("update public group failed, ", err)
		return err
	}

	if p, ok := publicGroupMap[pg.ID]; ok {

		p.Name = pg.Name
		p.Type = pg.Type
		p.Status = pg.Status
		p.Note = pg.Note
		p.UpdateTime = time.Now()
		p.AllowCPUID = pg.AllowCPUID
		p.connPool.allowCPUID = pg.AllowCPUID

		if pg.Type == 3 {
			p.connPool.allowCPUID = pg.AllowCPUID
		} else {

			p.connPool.allowCPUID = ""
		}

	}

	return nil

}

func deletePublicGroup(pg *publicgroup) error {

	_, err := db.Exec(`delete from public_groups  where id=$1`, pg.ID)
	if err != nil {
		log.Println("delete public group failed, ", err)
		return err
	}
	delete(publicGroupMap, pg.ID)

	return nil

}
