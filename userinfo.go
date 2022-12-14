package main

import (
	"log"
	"strings"
)

func initAllUserList() {

	rows, err := db.Query("SELECT * from  users where status=1")

	if err != nil {
		log.Println("query all user list  err:", err)
	}

	for rows.Next() {
		user := &userinfo{}

		var roles string

		err := rows.Scan(&user.ID, &user.Name, &user.CallSign, &user.Gird, &user.Phone, &user.Password,
			&user.Birthday, &user.Sex, &user.Avatar, &user.Address, &roles, &user.Introduction, &user.AlarmMsg,
			&user.Status, &user.UpdateTime, &user.LastLoginTime, &user.LoginErrTimes, &user.CreateTime, &user.OpenID, &user.NickName, &user.PID, &user.LastLoginIP)
		if err != nil {
			log.Println("query  all user rows err:", err)
			continue
		}

		user.Roles = strings.Split(roles, ",")

		user.userinit()

		userlist.Store(user.CallSign, *user)

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
