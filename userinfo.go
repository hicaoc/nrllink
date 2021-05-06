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

	u.Groups = make(map[int]*group, 5)

	u.Groups[1] = &group{
		ID:       1,
		Name:     "房间1",
		connPool: &currentConnPool{devConnList: make(map[string]*connPool)},
	}

	u.Groups[2] = &group{
		ID:       2,
		Name:     "房间2",
		connPool: &currentConnPool{devConnList: make(map[string]*connPool)},
	}

	u.Groups[3] = &group{
		ID:       3,
		Name:     "房间3",
		connPool: &currentConnPool{devConnList: make(map[string]*connPool)},
	}

	u.Groups[4] = &group{
		ID:       4,
		Name:     "房间4",
		connPool: &currentConnPool{devConnList: make(map[string]*connPool)},
	}

	// u.ConnPoll = make(map[int]*currentConnPool, 5)

	u.DevList = make(map[int]*deviceInfo, 10)

	// u.ConnPoll[0] = &currentConnPool{devConnList: make(map[string]*connPool)}
	// u.ConnPoll[1] = &currentConnPool{devConnList: make(map[string]*connPool)}
	// u.ConnPoll[2] = &currentConnPool{devConnList: make(map[string]*connPool)}
	// u.ConnPoll[3] = &currentConnPool{devConnList: make(map[string]*connPool)}
	// u.ConnPoll[4] = &currentConnPool{devConnList: make(map[string]*connPool)}

}
