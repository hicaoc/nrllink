package main

import (
	"fmt"
	"log"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

var publicGroupMap = make(map[int]*group, 1000) //key 房间号

/*
export const groupTypeOptions = [
  { id: 0, name: '公共房间' },
  { id: 1, name: '中继互联' },
  { id: 2, name: '设备互联' },
  // { id: 3, name: '守听' },
  { id: 4, name: '数模互联' },
  { id: 5, name: '俱乐部' },
  { id: 6, name: '车友会' },
  { id: 7, name: '会议组' },
  { id: 8, name: '私人房间' },
  { id: 100, name: '其他' }
]

*/

type group struct {
	ID                int      `json:"id" db:"id"`
	Name              string   `json:"name" db:"name"`
	Type              int      `json:"type" db:"type"` //1: 私人房间 2：公共房间 3：中继房间 7:会议房间（多人讲话）
	AllowCALLSSIDList []string `json:"allow_callsign_ssid" `
	AllowCALLSSID     string   `db:"allow_callsign_ssid"`
	DevList           []int    `json:"devlist" db:"devlist"`
	//KeepTime     int           `json:"keep_time" db:"keep_time"`
	Password string `json:"password" db:"password"`
	//Status       int    `json:"status" db:"status"`
	OwerID       int    `json:"ower_id" db:"ower_id"`
	OwerCallsign string `json:"callsign" db:"callsign"`

	//MasterServer int    `json:"master_server" db:"master_server"`
	//SlaveServer  int    `json:"slave_server" db:"slave_server"`
	CreateTime string `json:"create_time" db:"create_time"`
	UpdateTime string `json:"update_time" db:"update_time"`
	Note       string `json:"note" db:"note"`
	connPool   *currentConnPool
	DevMap     map[int]*deviceInfo `json:"devmap" ` //key: 设备ID
	//OutDevMap       map[string]*deviceInfo `json:"out_dev_map"` //callsign+ssid
	OnlineDevNumber int `json:"online_dev_number"`
	TotalDevNumber  int `json:"total_dev_number"`
	Recorder        int `json:"recored"`
	Timer           *time.Timer
	ticker          *time.Ticker
}

func (p *group) String() string {

	return fmt.Sprintf("id:%v,name:%v,type:%v ", p.ID, p.Name, p.Type)

}

func (p *group) mixPCM() {
	pcm := make([]int, 160)
	globalG711 := make([]byte, 160)
	newG711 := make([]byte, 160)
	data := make([]byte, 160)

	globalPacket := encodeNRL21("MEETLY", 201, 1, 201, 0, data)
	speakerPacket := encodeNRL21("MEETLY", 201, 1, 201, 0, data)
	speakerB_Packet := encodeNRL21("MEETLY", 201, 1, 201, 0, data)

	log.Println("mixPCM:", "p:", p)

	type activeSpeaker struct {
		dev     *deviceInfo
		rawG711 []byte
	}
	speakers := make([]activeSpeaker, 0, 10)

	for range p.ticker.C {
		speakers = speakers[:0]
		for i := range pcm {
			pcm[i] = 0
		}

		if len(p.connPool.devConnList) <= 2 {
			continue
		}

		// 1. 收集发言者数据
		for _, vv := range p.connPool.devConnList {
			vv.speaking = false
			select {
			case g711 := <-vv.pcmG711Chan:
				raw := g711[0][:160]
				vv.speaking = true
				speakers = append(speakers, activeSpeaker{vv, raw})
			default:
			}
		}

		numbs := len(speakers)
		if numbs == 0 {
			continue
		}

		// 2. 策略优化
		if numbs == 1 {
			// --- 单人发言直通 (Bypass) ---
			// 直接透传原始字节，音质无损
			copy(globalPacket[48:], speakers[0].rawG711)

			for _, vv := range p.connPool.devConnList {
				if vv.udpAddr == nil || vv.speaking {
					continue
				}
				globelconn.WriteToUDP(globalPacket, vv.udpAddr)
			}
		} else {
			// --- 多人混音处理 ---
			for _, s := range speakers {
				for i, v := range s.rawG711 {
					ori := int(alaw2linear(v))
					pcm[i] += ori
					s.dev.pcmBuffer[i] = ori
				}
			}

			// 计算全局混合音（给普通听众）
			for i, v := range pcm {
				if v > 32767 {
					v = 32767
				} else if v < -32768 {
					v = -32768
				}
				globalG711[i] = Linear2Alaw(int16(v))
			}
			copy(globalPacket[48:], globalG711)

			if numbs == 2 {
				// --- 双人对讲互传优化 ---
				copy(speakerPacket[48:], speakers[1].rawG711)   // A 听 B
				copy(speakerB_Packet[48:], speakers[0].rawG711) // B 听 A

				for _, vv := range p.connPool.devConnList {
					if vv.udpAddr == nil {
						continue
					}
					switch vv.CallSignSSID {
					case speakers[0].dev.CallSignSSID:
						globelconn.WriteToUDP(speakerPacket, vv.udpAddr)
					case speakers[1].dev.CallSignSSID:
						globelconn.WriteToUDP(speakerB_Packet, vv.udpAddr)
					default:
						globelconn.WriteToUDP(globalPacket, vv.udpAddr)
					}
				}
			} else {
				// --- 3人及以上标准混音 ---
				for _, vv := range p.connPool.devConnList {
					if vv.udpAddr == nil {
						continue
					}
					if vv.speaking {
						for i, v := range pcm {
							v -= vv.pcmBuffer[i]
							if v > 32767 {
								v = 32767
							} else if v < -32768 {
								v = -32768
							}
							newG711[i] = Linear2Alaw(int16(v))
						}
						copy(speakerPacket[48:], newG711)
						globelconn.WriteToUDP(speakerPacket, vv.udpAddr)
					} else {
						globelconn.WriteToUDP(globalPacket, vv.udpAddr)
					}
				}
			}
		}
	}
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
		connPool:     &currentConnPool{devConnMap: make(map[string]*deviceInfo)},
		DevMap:       make(map[int]*deviceInfo, 10),
		CreateTime:   time.Now().Format("2006-01-02 15:04:05"),
		UpdateTime:   time.Now().Format("2006-01-02 15:04:05"),
	}

	publicGroupMap[0] = pg0

	pg999 := &group{
		ID:           999,
		Name:         "全网互联",
		OwerCallsign: "default",
		connPool:     &currentConnPool{devConnMap: make(map[string]*deviceInfo)},
		DevMap:       make(map[int]*deviceInfo, 10),
		CreateTime:   time.Now().Format("2006-01-02 15:04:05"),
		UpdateTime:   time.Now().Format("2006-01-02 15:04:05"),
	}

	publicGroupMap[999] = pg999

	rows, err := db.Query(
		`SELECT 
			id,
			name,
			type,
			callsign,
			password,
			ower_id,
			allow_callsign_ssid,
			devlist, 
			create_time,update_time,note
		from  public_groups `)

	if err != nil {
		log.Println("query all public group list  err:", err)
	}

	var devlist string

	for rows.Next() {
		pg := &group{}
		err := rows.Scan(
			&pg.ID,
			&pg.Name,
			&pg.Type,
			&pg.OwerCallsign,
			&pg.Password,
			&pg.OwerID,
			&pg.AllowCALLSSID,
			&devlist,
			&pg.CreateTime,
			&pg.UpdateTime,
			&pg.Note)
		if err != nil {
			log.Println("query  all public group rows err:", err)
		}

		pg.DevList = convertStr2IntArray(devlist)

		if pg.AllowCALLSSID == "" {
			pg.AllowCALLSSIDList = []string{}
		} else {
			pg.AllowCALLSSIDList = strings.Split(pg.AllowCALLSSID, ",")

		}

		pg.connPool = &currentConnPool{devConnMap: make(map[string]*deviceInfo)}

		pg.DevMap = make(map[int]*deviceInfo, 10)

		// 类型为3的公共组，只能一个设备转发，用于中继收听

		//pg.connPool.allowCALLSSID = pg.AllowCALLSSIDList

		publicGroupMap[pg.ID] = pg

		if pg.Type == 7 {
			if pg.ticker == nil {
				pg.ticker = time.NewTicker(20000 * time.Microsecond)
				go pg.mixPCM()
			}

		}

		fmt.Println("pg:", pg)

	}

	//fmt.Println("publicGroupMap:", publicGroupMap)

}

type minigroup struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Type            int    `json:"type"`
	OnlineDevNumber int    `json:"online_dev_number"`
	TotalDevNumber  int    `json:"total_dev_number"`
}

func getUserGroupList(u *userinfo) []minigroup {

	grouplist := make([]minigroup, 0)

	if user, okok := userlist.Load(u.CallSign); okok {
		gp1 := user.(*userinfo).Groups[1]
		gp2 := user.(*userinfo).Groups[2]
		gp3 := user.(*userinfo).Groups[3]

		g1 := minigroup{
			ID:              1,
			Name:            gp1.Name,
			Type:            gp1.Type,
			OnlineDevNumber: gp1.OnlineDevNumber,
			TotalDevNumber:  gp1.TotalDevNumber,
		}

		g2 := minigroup{
			ID:              2,
			Name:            gp2.Name,
			Type:            gp2.Type,
			OnlineDevNumber: gp2.OnlineDevNumber,
			TotalDevNumber:  gp2.TotalDevNumber,
		}

		g3 := minigroup{
			ID:              3,
			Name:            gp3.Name,
			Type:            gp3.Type,
			OnlineDevNumber: gp3.OnlineDevNumber,
			TotalDevNumber:  gp3.TotalDevNumber,
		}

		grouplist = append(grouplist, g1)
		grouplist = append(grouplist, g2)
		grouplist = append(grouplist, g3)

	} else {
		log.Println("user not found")
	}

	return grouplist

}

func getMiniGroupList(u *userinfo) []minigroup {

	grouplist := getUserGroupList(u)

	for _, v := range publicGroupMap {

		g := minigroup{
			ID:              v.ID,
			Name:            v.Name,
			Type:            v.Type,
			OnlineDevNumber: v.OnlineDevNumber,
			TotalDevNumber:  v.TotalDevNumber,
		}
		grouplist = append(grouplist, g)

	}

	sort.Slice(grouplist, func(i, j int) bool {
		return grouplist[i].ID < grouplist[j].ID
	})

	return grouplist
}

func getGroup(name string) (pg *group) {
	pg = &group{}
	var devlist string

	row := db.QueryRow(`select 
	id,
	name,
	type,
	callsign,
	ower_id,
	password,
	allow_callsign_ssid,
	devlist, 
	create_time,update_time,note 
	FROM public_groups  where name=?`, name)
	err := row.Scan(
		&pg.ID,
		&pg.Name,
		&pg.Type,
		&pg.OwerCallsign,
		&pg.OwerID,
		&pg.Password,
		&pg.AllowCALLSSID,
		&devlist,
		//&pg.MasterServer,
		//&pg.SlaveServer,
		&pg.CreateTime,
		&pg.UpdateTime,
		&pg.Note)
	if err != nil {
		log.Println("get group by name err:", err, name)
		return nil
	}

	if pg.AllowCALLSSID == "" {
		pg.AllowCALLSSIDList = []string{}
	} else {
		pg.AllowCALLSSIDList = strings.Split(pg.AllowCALLSSID, ",")

	}

	pg.DevList = convertStr2IntArray(devlist)
	return pg

}

func changeDevGroup(dev *deviceInfo, groupid int) (group string, err error) {

	//检查目标组是否允许此设备加入

	if g, ok := publicGroupMap[groupid]; ok {

		if len(g.AllowCALLSSIDList) > 0 {
			if !slices.Contains(g.AllowCALLSSIDList, dev.CallSignSSID) {
				return "", fmt.Errorf("group not allow this callsign")
			}

		}

	}

	//从之前的组删除

	if dev.GroupID >= 999 || dev.GroupID == 0 {

		if g, ok := publicGroupMap[dev.GroupID]; ok {
			delete(g.connPool.devConnMap, dev.udpAddr.String())

			list := []*deviceInfo{}

			for _, vv := range g.connPool.devConnMap {
				list = append(list, vv)
			}

			g.connPool.devConnList = list

			delete(g.DevMap, dev.ID)

		} else {

			return "", fmt.Errorf("dev not in group ")
		}
	} else {

		//私人房间

		if user, okok := userlist.Load(dev.CallSign); okok {
			delete(user.(*userinfo).Groups[dev.GroupID].DevMap, dev.ID)
			delete(user.(*userinfo).Groups[dev.GroupID].connPool.devConnMap, dev.udpAddr.String())
			//delete(user.(*userinfo).Groups[dev.GroupID].connPool.devConnList, dev.udpAddr.String())

			list := []*deviceInfo{}
			for _, vv := range user.(*userinfo).Groups[dev.GroupID].connPool.devConnMap {
				list = append(list, vv)
			}
			user.(*userinfo).Groups[dev.GroupID].connPool.devConnList = list

		}

	}

	//加入新的组

	if groupid >= 999 || groupid == 0 {

		if g, ok := publicGroupMap[groupid]; ok {
			dev.GroupID = groupid
			g.DevMap[dev.ID] = dev
			group = strconv.Itoa(g.ID) + g.Name

		} else {

			return "", fmt.Errorf("group not found")

		}
	} else {

		if user, okok := userlist.Load(dev.CallSign); okok {
			user.(*userinfo).Groups[groupid].DevMap[dev.ID] = dev
			group = strconv.Itoa(user.(*userinfo).Groups[groupid].ID) + user.(*userinfo).Groups[groupid].Name

		}

		//私有房间

		dev.GroupID = groupid

	}

	return

}

func addPublicGroup(pg *group) error {

	pg.AllowCALLSSID = strings.Join(pg.AllowCALLSSIDList, ",")

	//	fmt.Println("user:", e)
	var devllist = convertIntArray2Str(pg.DevList)
	query := `INSERT INTO public_groups (name,type,allow_callsign_ssid,callsign,ower_id,password,devlist,
		note,create_time,update_time	) 
	VALUES (?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`

	_, err := db.Exec(query, pg.Name, pg.Type, pg.AllowCALLSSID, pg.OwerCallsign, pg.OwerID, pg.Password, devllist,
		pg.Note)

	if err != nil {
		log.Println("add public group failed, ", err, '\n', query)
		return err
	}

	newpg := getGroup(pg.Name)

	if newpg == nil {

		return fmt.Errorf("群组添加失败")

	}
	if _, ok := publicGroupMap[newpg.ID]; !ok {
		newpg.connPool = &currentConnPool{devConnMap: make(map[string]*deviceInfo)}
		newpg.DevMap = make(map[int]*deviceInfo, 10)
		if newpg.Type == 7 {
			if newpg.ticker == nil {
				newpg.ticker = time.NewTicker(20000 * time.Microsecond)
				go newpg.mixPCM()
			}

		}
		publicGroupMap[newpg.ID] = newpg
	}

	//initPublicGroup()

	return nil

}

func updatePublicGroup(pg *group) error {

	pg.AllowCALLSSID = strings.Join(pg.AllowCALLSSIDList, ",")

	_, err := db.Exec(`update public_groups set name=?, type=?, allow_callsign_ssid=?, password=?, 
	 note=?,  update_time=CURRENT_TIMESTAMP  where id=?`,
		pg.Name, pg.Type, pg.AllowCALLSSID, pg.Password, pg.Note, pg.ID)

	if err != nil {
		log.Println("update public group failed, ", err)
		return err
	}

	if p, ok := publicGroupMap[pg.ID]; ok {

		p.Name = pg.Name

		//类型从其他改成7，需要启动mixPCM
		if pg.Type == 7 && p.Type != 7 {
			if p.ticker == nil {
				p.ticker = time.NewTicker(20000 * time.Microsecond)
				go p.mixPCM()
			}

		}
		//从7改为其他 需要停止mixPCM
		if pg.Type != 7 && p.Type == 7 {
			if p.ticker != nil {
				p.ticker.Stop()
				p.ticker = nil

			}

		}

		p.Type = pg.Type

		p.Note = pg.Note
		p.UpdateTime = time.Now().Format("2006-01-02 15:04:05")
		p.AllowCALLSSID = pg.AllowCALLSSID
		p.AllowCALLSSIDList = pg.AllowCALLSSIDList
		//p.connPool.allowCALLSSID = pg.AllowCALLSSIDList
		p.Password = pg.Password

		//p.connPool.allowCALLSSID = pg.AllowCALLSSIDList

	}

	return nil

}

func deletePublicGroup(pg *group) error {

	_, err := db.Exec(`delete from public_groups  where id=?`, pg.ID)
	if err != nil {
		log.Println("delete public group failed, ", err)
		return err
	}

	if _, ok := publicGroupMap[pg.ID]; ok {
		if pg.ticker != nil {
			pg.ticker.Stop()
			pg.ticker = nil
		}
	}

	delete(publicGroupMap, pg.ID)

	return nil

}

func getGroupListForDevice(packet []byte) []byte {

	grouplist := make([]minigroup, 0)

	for _, v := range publicGroupMap {

		g := minigroup{
			ID:              v.ID,
			Name:            v.Name,
			Type:            v.Type,
			OnlineDevNumber: v.OnlineDevNumber,
			TotalDevNumber:  v.TotalDevNumber,
		}
		grouplist = append(grouplist, g)

	}

	sort.Slice(grouplist, func(i, j int) bool {
		return grouplist[i].ID < grouplist[j].ID
	})

	csv := make([]string, len(grouplist))

	for i, group := range grouplist {
		csv[i] = fmt.Sprintf("%d,%s", group.ID, group.Name)
	}

	output := strings.Join(csv, "\n")

	packet = append(packet, output...)

	return packet

}
