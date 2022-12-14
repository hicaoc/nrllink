package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

var publicGroupMap = make(map[int]*group, 1000) //key 房间号

type group struct {
	ID         int    `json:"id" db:"id"`
	Name       string `json:"name" db:"name"`
	Type       int    `json:"type" db:"type"`
	AllowCPUID string `json:"allow_cpuid" db:"allow_cpuid"`
	DevList    []int  `json:"devlist" db:"devlist"`
	//KeepTime     int           `json:"keep_time" db:"keep_time"`
	Password     string `json:"password" db:"password"`
	Status       int    `json:"status" db:"status"`
	OwerID       int    `json:"ower_id" db:"ower_id"`
	OwerCallsign string `json:"callsign" db:"callsign"`
	MasterServer int    `json:"master_server" db:"master_server"`
	SlaveServer  int    `json:"slave_server" db:"slave_server"`
	CreateTime   string `json:"create_time" db:"create_time"`
	UpdateTime   string `json:"update_time" db:"update_time"`
	Note         string `json:"note" db:"note"`
	connPool     *currentConnPool
	DevMap       map[int]*deviceInfo `json:"devmap" ` //key: 设备ID
}

func (p *group) String() string {

	return fmt.Sprintf("id:%v,name:%v,type:%v,status:%v", p.ID, p.Name, p.Type, p.Status)

}

func convertStr2IntArray(str string) []int {
	s := strings.Split(str, ",")

	res := make([]int, len(s))
	for i, v := range s {

		res[i], _ = strconv.Atoi(v)

	}
	return res

}

func convertIntArray2Str(gp []int) string {

	res := make([]string, len(gp))
	for i, v := range gp {

		res[i] = strconv.Itoa(v)

	}
	return strings.Join(res, ",")

}

func initPublicGroup() {

	pg0 := &group{
		ID:           0,
		Name:         "公共大厅",
		OwerCallsign: "default",
		connPool:     &currentConnPool{devConnList: make(map[string]*connPool)},
		DevMap:       make(map[int]*deviceInfo, 10),
		CreateTime:   time.Now().Format("2006-01-02 15:04:05"),
		UpdateTime:   time.Now().Format("2006-01-02 15:04:05"),
	}

	publicGroupMap[0] = pg0

	rows, err := db.Query("SELECT * from  public_groups")

	if err != nil {
		log.Println("query all public group list  err:", err)
	}

	var devlist string

	for rows.Next() {
		pg := &group{}
		err := rows.Scan(&pg.ID,
			&pg.Name,
			&pg.Type,
			&pg.OwerCallsign,
			&pg.Password,
			&pg.AllowCPUID,
			&pg.OwerID,
			&devlist,
			&pg.MasterServer,
			&pg.SlaveServer,
			&pg.Status,
			&pg.CreateTime,
			&pg.UpdateTime,
			&pg.Note)
		if err != nil {
			log.Println("query  all public group rows err:", err)
		}

		pg.DevList = convertStr2IntArray(devlist)

		pg.connPool = &currentConnPool{devConnList: make(map[string]*connPool)}
		pg.DevMap = make(map[int]*deviceInfo, 10)

		// 类型为3的公共组，只能一个设备转发，用于中继收听
		if pg.Type == 3 {
			pg.connPool.allowCPUID = pg.AllowCPUID
		}

		publicGroupMap[pg.ID] = pg

		fmt.Println("pg:", pg)

	}

	fmt.Println("publicGroupMap:", publicGroupMap)

}

func getGroup(name string) (pg *group) {
	pg = &group{}
	var devlist string

	row := db.QueryRow(`select * FROM public_groups  where name=?`, name)
	err := row.Scan(&pg.ID,
		&pg.Name,
		&pg.Type,
		&pg.OwerCallsign,
		&pg.Password,
		&pg.AllowCPUID,
		&pg.OwerID,
		&devlist,
		&pg.MasterServer,
		&pg.SlaveServer,
		&pg.Status,
		&pg.CreateTime,
		&pg.UpdateTime,
		&pg.Note)
	if err != nil {
		log.Println("get group by name err:", err, name)
		return nil
	}
	pg.DevList = convertStr2IntArray(devlist)
	return pg

}

func changeDevGroup(dev *deviceInfo, groupid int) (err error) {

	//从之前的组删除

	if dev.GroupID >= 1000 || dev.GroupID == 0 {

		if g, ok := publicGroupMap[dev.GroupID]; ok {
			delete(g.DevMap, dev.ID)
		} else {
			return fmt.Errorf("dev not in group ")
		}
	}

	//加入新的组

	if groupid >= 1000 || groupid == 0 {

		if g, ok := publicGroupMap[groupid]; ok {
			dev.GroupID = groupid
			g.DevMap[dev.ID] = dev

		} else {

			return fmt.Errorf("group not found")

		}
	} else {

		//私有房间

		dev.GroupID = groupid

	}

	return

}

func addPublicGroup(pg *group) error {

	//	fmt.Println("user:", e)
	var devllist = convertIntArray2Str(pg.DevList)
	query := `INSERT INTO public_groups (name,type,allow_cpuid,callsign,ower_id,password,devlist,
		master_server,slave_server,status,note,create_time,update_time	) 
	VALUES (?,?,?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`

	_, err := db.Exec(query, pg.Name, pg.Type, pg.AllowCPUID, pg.OwerCallsign, pg.OwerID, pg.Password, devllist,
		pg.MasterServer, pg.SlaveServer, pg.Status, pg.Note)

	if err != nil {
		log.Println("add public group failed, ", err, '\n', query)
		return err
	}

	newpg := getGroup(pg.Name)

	if newpg == nil {

		return fmt.Errorf("群组添加失败")

	}
	if _, ok := publicGroupMap[newpg.ID]; !ok {
		newpg.connPool = &currentConnPool{devConnList: make(map[string]*connPool)}
		newpg.DevMap = make(map[int]*deviceInfo, 10)
		publicGroupMap[newpg.ID] = newpg
	}

	//initPublicGroup()

	return nil

}

func updatePublicGroup(pg *group) error {

	_, err := db.Exec(`update public_groups set name=?, type=?, allow_cpuid=?, password=?, status=?,
	master_server=?, slave_server=?, note=?,  update_time=CURRENT_TIMESTAMP  where id=?`,
		pg.Name, pg.Type, pg.AllowCPUID, pg.Password, pg.Status, pg.MasterServer, pg.SlaveServer, pg.Note, pg.ID)

	if err != nil {
		log.Println("update public group failed, ", err)
		return err
	}

	if p, ok := publicGroupMap[pg.ID]; ok {

		p.Name = pg.Name
		p.Type = pg.Type
		p.MasterServer = pg.MasterServer
		p.SlaveServer = pg.SlaveServer
		p.Status = pg.Status
		p.Note = pg.Note
		p.UpdateTime = time.Now().Format("2006-01-02 15:04:05")
		p.AllowCPUID = pg.AllowCPUID
		p.connPool.allowCPUID = pg.AllowCPUID
		p.Password = pg.Password

		if pg.Type == 3 {
			p.connPool.allowCPUID = pg.AllowCPUID
		} else {
			p.connPool.allowCPUID = ""
		}

	}

	return nil

}

func deletePublicGroup(pg *group) error {

	_, err := db.Exec(`delete from public_groups  where id=?`, pg.ID)
	if err != nil {
		log.Println("delete public group failed, ", err)
		return err
	}
	delete(publicGroupMap, pg.ID)

	return nil

}
