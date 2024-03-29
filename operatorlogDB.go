package main

import (
	"fmt"
	"log"
)

// OperatorLog 操作日记
type OperatorLog struct {
	ID         int    `db:"id" json:"id"`
	Timestamp  string `db:"timestamp" json:"timestamp"`
	Content    string `db:"content" json:"content"` //动作来源  自动刷卡，教务协助
	EventType  string `db:"event_type" json:"event_type"`
	Operator   string `db:"operator" json:"operator"` //教务操作时的操作人员
	OperatorID int    `db:"operator_id" json:"operator_id"`
	Note       string `db:"note" json:"note"`
}

func getOperatorLog(s string, p string, emp *userinfo) ([]*OperatorLog, int) {

	// if checkrole(emp, []string{"admin"}) == true {
	// 	schname = "public"
	// }

	loglist := []*OperatorLog{}

	query := fmt.Sprintf(`SELECT id,timestamp,content,event_type,operator,operator_id 
	FROM operator_log %v ORDER BY timestamp DESC %v`, s, p)

	rows, err := db.Query(query)

	if err != nil {
		log.Println("查询操作日记记录错误: ", err)
		return nil, 0

	}

	for rows.Next() {

		l := &OperatorLog{}

		err = rows.Scan(&l.ID, &l.Timestamp, &l.Content, &l.EventType, &l.Operator, &l.OperatorID)
		if err != nil {
			log.Println("select operator_log err:", err, query)
			continue
		}
		loglist = append(loglist, l)

	}

	var t int
	q := fmt.Sprintf("SELECT count(*) as total from operator_log %v ", s)
	row := db.QueryRow(q)
	err = row.Scan(&t)
	if err != nil {
		log.Println(" 查询操作日记记录total错误 err:", err, t)
		return nil, 0
	}

	//fmt.Println(emp)
	return loglist, t

}

func addOperatorLog(Content string, EventType string, emp *userinfo) {

	query := "INSERT INTO operator_log (timestamp,content,event_type,operator,operator_id) VALUES (CURRENT_TIMESTAMP,?,?,?,?)"

	//fmt.Println(query)

	_, err := db.Exec(query, Content, EventType, emp.Name, emp.ID)

	if err != nil {
		log.Println("记录日记错误: ", err, "\n", query)

	}
	//fmt.Println(emp)

}
