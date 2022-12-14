package main

import (
	"fmt"
	"log"
)

type relay struct {
	ID           int    `json:"id" db:"id"`
	Name         string `json:"name" db:"name"`
	UPFreq       string `json:"up_freq" db:"up_freq"`
	DownFreq     string `json:"down_freq" db:"down_freq"`
	SendCTSS     string `json:"send_ctss" db:"send_ctss"`
	ReciveCTSS   string `json:"recive_ctss" db:"recive_ctss"`
	OwerCallsign string `json:"ower_callsign" db:"ower_callsign"` //创建者呼号
	CreateTime   string `json:"create_time" db:"create_time"`
	UpdateTime   string `json:"update_time" db:"update_time"`
	Status       int    `json:"status" db:"status"`
	Note         string `json:"note" db:"note"`
}

func (p *relay) String() string {

	return fmt.Sprintf("id:%v,name:%v,up_freq:%v,status:%v", p.ID, p.Name, p.UPFreq, p.Status)

}

func selectrelay(w string, p string, sort string) ([]*relay, int) {

	emp := []*relay{}

	query := fmt.Sprintf(`SELECT  id,name,up_freq,down_freq,send_ctss,recive_ctss,ower_callsign,status,note,
    create_time, update_time
    FROM relay  %v   ORDER by id asc %v  `, w, p)

	//fmt.Println(query)

	rows, err := db.Query(query)
	if err != nil {
		log.Println("查询频点列表错误: ", err, "\n", query)
		return nil, 0

	}

	for rows.Next() {
		r := &relay{}
		err = rows.Scan(&r.ID, &r.Name, &r.UPFreq, &r.DownFreq, &r.SendCTSS, &r.ReciveCTSS, &r.OwerCallsign, &r.Status, &r.Note, &r.CreateTime, &r.UpdateTime)
		if err != nil {
			log.Println("select relay err:", err, query)
			continue
		}
		emp = append(emp, r)
	}

	var t int
	q := fmt.Sprintf(`SELECT count(*) as total FROM relay  %v  `, w)
	//fmt.Println(q)
	row := db.QueryRow(q)
	err = row.Scan(&t)
	if err != nil {
		log.Println(" 查询频点列表total错误 err:", err, t)
		return nil, 0
	}

	//fmt.Println(emp)
	return emp, t
	//fmt.Println(emp)

}

func addrelay(s *relay) error {

	//	fmt.Println("user:", e)
	query := `INSERT INTO relay (name,up_freq,down_freq,send_ctss,recive_ctss,ower_callsign,status,note,create_time,update_time) 
	VALUES (?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP) `

	resault, err := db.Exec(query,
		s.Name, s.UPFreq, s.DownFreq, s.SendCTSS, s.ReciveCTSS, s.OwerCallsign, s.Status, s.Note)

	if err != nil {
		log.Println("add relay failed, ", err, '\n', query)
		return err
	} else {
		fmt.Println("resault:", resault)
	}

	return nil

}

func updaterelay(s *relay) error {

	_, err := db.Exec(`update relay set name=?,up_freq=?,down_freq=?,send_ctss=?,recive_ctss=?,status=?,note=?,update_time=CURRENT_TIMESTAMP where id=?`,
		s.Name, s.UPFreq, s.DownFreq, s.SendCTSS, s.ReciveCTSS, s.Status, s.Note, s.ID)
	if err != nil {
		log.Println("update relay failed, ", err)
		return err
	}

	return nil

}

func deleterelay(s *relay) error {

	_, err := db.Exec(`delete from relay  where id=?`, s.ID)
	if err != nil {
		log.Println("delete relay failed, ", err)
		return err
	}

	return nil

}
