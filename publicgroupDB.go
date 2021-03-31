package main

import (
	"fmt"
	"log"
	"time"

	"github.com/lib/pq"
)

var publicGroupMap = make(map[int]*publicgroup, 1000) //key 房间号

type publicgroup struct {
	ID           int           `json:"id" db:"id"`
	Name         string        `json:"name" db:"name"`
	Type         int           `json:"type" db:"type"`
	DevList      pq.Int64Array `json:"devlist" db:"devlist"`
	Status       int           `json:"status" db:"status"`
	OwerID       int           `json:"ower_id" db:"ower_id"`
	OwerCallsign string        `json:"callsign" db:"callsign"`
	CreateTime   time.Time     `json:"create_time" db:"create_time"`
	UpdateTime   time.Time     `json:"update_time" db:"update_time"`
	Note         string        `json:"note" db:"note"`
	connPool     *currentConnPool
	DevMap       map[int]*deviceInfo `json:"devmap" ` //key: 设备ID
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
		pg := &publicgroup{connPool: &currentConnPool{devConnList: make(map[string]*connPool)},
			DevMap: make(map[int]*deviceInfo, 10)}
		err := rows.StructScan(pg)
		if err != nil {
			log.Println("query  all public group rows err:", err)
		}

		publicGroupMap[pg.ID] = pg

	}

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
	query := `INSERT INTO public_groups (name,type,callsign,ower_id,devlist,status,note,create_time,update_time	) 
	VALUES ($1,$2,$3,$4,$5,$6,$7,now(),now()) RETURNING id`

	resault, err := db.Exec(query,
		pg.Name, pg.Type, pg.OwerCallsign, pg.OwerID, pg.DevList, pg.Status, pg.Note)

	if err != nil {
		log.Println("bing dev failed, ", err, '\n', query)
		return err
	} else {
		fmt.Println("resault:", resault)
	}

	initPublicGroup()

	return nil

}

func updatePublicGroup(pg *publicgroup) error {

	_, err := db.Exec(`update public_groups set name=$1, type=$2,   status=$3, note=$4 ,update_time=now()  where id=$5`,
		pg.Name, pg.Type, pg.Status, pg.Note, pg.ID)
	if err != nil {
		log.Println("update device failed, ", err)
		return err
	}

	initPublicGroup()

	return nil

}

func deletePublicGroup(pg *publicgroup) error {

	_, err := db.Exec(`delete from public_groups  where id=$5`,
		pg.Name, pg.Type, pg.Status, pg.Note, pg.ID)
	if err != nil {
		log.Println("delete public group failed, ", err)
		return err
	}

	return nil

}
