package main

import (
	"log"
	"strings"
)

func initAllUserList() {

	rows, err := db.Query(`
	SELECT 
	id,name,callsign,
	mdcid,
	dmrid,
	gird,
	phone,
	password,
	birthday,
	sex,
	avatar,
	address,
	roles,
	introduction,
	alarm_msg,
	status,
	update_time,
	last_login_time,
	login_err_times,
	create_time, 
	openid,
    nickname,
    pid,
	last_login_ip,
	expire_time
	FROM users where status=1`)

	if err != nil {
		log.Println("query all user list  err:", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		user := &userinfo{}

		var roles string

		err := rows.Scan(&user.ID, &user.Name, &user.CallSign,
			&user.MDCID, &user.DMRID, &user.Gird,
			&user.Phone, &user.Password,
			&user.Birthday, &user.Sex, &user.Avatar, &user.Address,
			&roles, &user.Introduction, &user.AlarmMsg,
			&user.Status, &user.UpdateTime, &user.LastLoginTime, &user.LoginErrTimes,
			&user.CreateTime, &user.OpenID, &user.NickName, &user.PID, &user.LastLoginIP, &user.ExpireTime)
		if err != nil {
			log.Println("query  all user rows err:", err)
			continue
		}

		user.Roles = strings.Split(roles, ",")

		user.userinit()

		userlist.Store(user.CallSign, user)
		mdcidmap.Store(user.MDCID, user.CallSign)
		dmridmap.Store(user.DMRID, user.CallSign)

	}
	if err := rows.Err(); err != nil {
		log.Println("iterate all user rows err:", err)
	}

}

func (u *userinfo) userinit() {

	u.Groups = make(map[int]*group, 5)

	u.Groups[1] = &group{
		ID:           1,
		Name:         "个人房间1",
		Type:         8,
		devMap:       make(map[int]*deviceInfo),
		DevList:      []int{},
		OwerID:       u.ID,
		OwerCallsign: u.CallSign,
		connPool:     &currentConnPool{devConnMap: make(map[string]*deviceInfo)},
	}

	u.Groups[2] = &group{
		ID:           2,
		Name:         "个人房间2",
		Type:         8,
		OwerID:       u.ID,
		OwerCallsign: u.CallSign,
		devMap:       make(map[int]*deviceInfo),
		DevList:      []int{},
		connPool:     &currentConnPool{devConnMap: make(map[string]*deviceInfo)},
	}

	u.Groups[3] = &group{
		ID:           3,
		Name:         "个人房间3",
		Type:         8,
		OwerID:       u.ID,
		OwerCallsign: u.CallSign,
		DevList:      []int{},
		devMap:       make(map[int]*deviceInfo),
		connPool:     &currentConnPool{devConnMap: make(map[string]*deviceInfo)},
	}

	// u.Groups[4] = &group{
	// 	ID:       4,
	// 	Name:     "房间4",
	// 	type:    : 8,
	// 	connPool: &currentConnPool{devConnMap: make(map[string]*deviceInfo)},
	// }

	// u.ConnPoll = make(map[int]*currentConnPool, 5)

	u.DevList = make(map[int]*deviceInfo, 10)

	// u.ConnPoll[0] = &currentConnPool{devConnMap: make(map[string]*connPool)}
	// u.ConnPoll[1] = &currentConnPool{devConnMap: make(map[string]*connPool)}
	// u.ConnPoll[2] = &currentConnPool{devConnMap: make(map[string]*connPool)}
	// u.ConnPoll[3] = &currentConnPool{devConnMap: make(map[string]*connPool)}
	// u.ConnPoll[4] = &currentConnPool{devConnMap: make(map[string]*connPool)}

}
