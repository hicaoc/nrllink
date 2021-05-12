package main

import (
	"fmt"
	"log"
	"time"
)

type relay struct {
	ID           int       `json:"id" db:"id"`
	Name         string    `json:"name" db:"name"`
	UPFreq       string    `json:"up_freq" db:"up_freq"`
	DownFreq     string    `json:"down_freq" db:"down_freq"`
	SendCTSS     int       `json:"send_ctss" db:"send_ctss"`
	ReciveCTSS   int       `json:"recive_ctss" db:"send_ctss"`
	OwerCallsign string    `json:"ower_callsign" db:"ower_callsign"` //创建者呼号
	CreateTime   time.Time `json:"create_time" db:"create_time"`
	UpdateTime   time.Time `json:"update_time" db:"update_time"`
	Status       int       `json:"status" db:"status"`
	Note         string    `json:"note" db:"note"`
}

func (p *relay) String() string {

	return fmt.Sprintf("id:%v,name:%v,up_freq:%v,status:%v", p.ID, p.Name, p.UPFreq, p.Status)

}

func selectrelay(w string, p string, sort string) ([]relay, int) {

	emp := []relay{}

	query := fmt.Sprintf(`SELECT  id,name,up_freq,down_freq,send_ctss,recive_ctss,ower_callsign,status,note,
    to_char(create_time,'YYYY-MM-DD HH24:MI:SS') as create_time,to_char(update_time,'YYYY-MM-DD HH24:MI:SS') as uptime_time
    FROM relay  %v   ORDER by id asc %v  `, w, p)

	//fmt.Println(query)

	err := db.Select(&emp, query)

	if err != nil {
		log.Println("查询频点列表错误: ", err, "\n", query)
		return nil, 0

	}

	t := &total{}
	q := fmt.Sprintf(`SELECT count(*) as total FROM relay  %v  `, w)
	//fmt.Println(q)
	err2 := db.Get(t, q)
	if err2 != nil {
		log.Println(" 查询频点列表total错误 err:", err, t)
		return nil, 0
	}
	//fmt.Println(emp)
	return emp, t.Total
	//fmt.Println(emp)

}

func addrelay(s *relay) error {

	//	fmt.Println("user:", e)
	query := `INSERT INTO relay (name,up_freq,down_freq,send_ctss,recive_ctss,ower_callsign,status,note,create_time,update_time) 
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,now(),now()) `

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

	_, err := db.Exec(`update relay set name=$1,up_freq=$2,down_freq=$3,send_ctss=$4,recive_ctss=$5,status=$6,note=$7,update_time=now() where id=$8`,
		s.Name, s.UPFreq, s.DownFreq, s.SendCTSS, s.ReciveCTSS, s.Status, s.Note, s.ID)
	if err != nil {
		log.Println("update relay failed, ", err)
		return err
	}

	return nil

}

func deleterelay(s *relay) error {

	_, err := db.Exec(`delete from relay  where id=$1`, s.ID)
	if err != nil {
		log.Println("delete relay failed, ", err)
		return err
	}

	return nil

}
