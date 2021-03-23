package main

import (
	"log"
)

func initAllUserList() {

	rows, err := db.Queryx("SELECT * from  users where status=1")

	if err != nil {
		log.Println("query all user list  err:", err)
	}

	for rows.Next() {
		user := &userinfo{}

		err := rows.StructScan(user)
		if err != nil {
			log.Println("query  all device rows err:", err)
		}

		user.userinit()

		userlist[user.ID] = *user

	}

}

func (u *userinfo) userinit() {

	u.ConnPoll = make(map[int]*currentConnPool, 5)

	u.DevList = make(map[int]*deviceInfo, 10)

	u.ConnPoll[0] = &currentConnPool{devConnList: make(map[string]*connPool)}
	u.ConnPoll[1] = &currentConnPool{devConnList: make(map[string]*connPool)}
	u.ConnPoll[2] = &currentConnPool{devConnList: make(map[string]*connPool)}
	u.ConnPoll[3] = &currentConnPool{devConnList: make(map[string]*connPool)}
	u.ConnPoll[4] = &currentConnPool{devConnList: make(map[string]*connPool)}

}
