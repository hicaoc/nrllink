package main

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	//"github.com/lib/pq"
)

type userinfo struct {
	//uUID     string `db:"uuid"`
	PID      string `db:"pid" json:"pid"`
	ID       int    `db:"id" json:"id"`
	Name     string `db:"name" json:"name"`
	CallSign string `db:"callsign" json:"callsign"`
	MDCID    string `db:"mdcid" json:"mdcid"`
	DMRID    string `db:"dmrid" json:"dmrid"`
	Gird     string `db:"gird" json:"gird"`
	Phone    string `db:"phone" json:"phone"`
	Password string `db:"password" json:"password"`
	//	JobTime  string `db:"job_time" json:"job_time"`
	Birthday string `db:"birthday" json:"birthday"`
	Sex      int    `db:"sex" json:"sex"`
	Address  string
	Mail     string
	//CanSpeekerDev *connPoll
	//GroupsList []map[uint64]bool
	DevList map[int]*deviceInfo `json:"devlist"` //key 房间号
	//ConnPoll map[int]*currentConnPool //群组连接池表，每个组有一个连接池列表 /key为组号
	Groups map[int]*group //呼号map
	//	userID        int            `db:"user_id" json:"user_id"`
	//Position          int                 `db:"position" json:"position"`
	Introduction string   `db:"introduction" json:"introduction"`
	Avatar       string   `db:"avatar" json:"avatar"`
	Roles        []string `db:"roles" json:"roles"`
	UpdateTime   string   `db:"update_time" json:"update_time"`
	CreateTime   string   `db:"create_time" json:"create_time"`

	Routes        string        `json:"routes" db:"routes"`
	Status        int           `json:"status" db:"status"`
	LastLoginTime string        `json:"last_login_time" db:"last_login_time"`
	LastLoginIP   string        `json:"last_login_ip" db:"last_login_ip"`
	ExpireTime    string        `json:"expire_time" db:"expire_time"`
	LoginErrTimes int           `json:"login_err_times" db:"login_err_times"`
	AlarmMsg      bool          `json:"alarm_msg" db:"alarm_msg"`
	NickName      string        `json:"nickname" db:"nickname"`
	OpenID        string        `json:"openid" db:"openid"`
	TalkDuration  time.Duration `json:"talk_duration"`
	TalkTimes     int           `json:"talk_times"`
}

type role struct {
	ID          int    `json:"id" db:"id"`
	NameKey     string `json:"key" db:"name_key"`
	Name        string `json:"name" db:"name"`
	Description string `json:"description" db:"description"`
	Routes      string `json:"routes" db:"routes"`
}

// func getRoutes() []role {
// 	r := []role{}

// 	err := db.Select(&r, "SELECT * FROM routes ")

// 	if err != nil {
// 		log.Println("查询菜单路由表错误: ", err)
// 		return nil

// 	}
// 	//fmt.Println(emp)
// 	return r

// }

func getRoles(query string) []*role {
	rl := []*role{}

	q := fmt.Sprintf("SELECT * FROM roles %v ", query)

	rows, err := db.Query(q)

	if err != nil {
		log.Println("查询角色列表错误: ", err)
		return nil

	}

	for rows.Next() {
		r := &role{}

		err = rows.Scan(&r.ID, &r.NameKey, &r.Name, &r.Description, &r.Routes)

		if err != nil {
			log.Println("select roles err :", err, query)
			continue
		}

		rl = append(rl, r)

	}
	//fmt.Println(emp)
	return rl

}

func (s *role) String() string {

	return fmt.Sprintf("NameKey:%v Name:%v", s.NameKey, s.Name)

}

func (s *userinfo) String() string {

	return fmt.Sprintf("Name:%v Phone:%v ", s.Name, s.Phone)

}

func getRoleByKey(key string) *role {

	r := &role{}

	row := db.QueryRow("SELECT * from roles where name_key=?", key)
	err := row.Scan(&r.ID, &r.Name, &r.Name, &r.Description, &r.Routes)
	if err != nil {
		log.Println("query role by key err:", err, r, key)
	}
	return r
}

func addRole(r *role) {

	//	fmt.Println("routes:", r)

	_, err := db.Exec("insert into  roles (name_key,name,description,routes) VALUES (?,?,?,?)", r.NameKey, r.Name, r.Description, r.Routes)
	if err != nil {
		log.Println("add role failed, ", err)
		return
	}

}

func updateRole(r *role) {

	_, err := db.Exec("update roles set name_key=?,name=?,description=?,routes=? where id=?", r.NameKey, r.Name, r.Description, r.Routes, r.ID)
	if err != nil {
		log.Println("exec update role failed, ", err)
		return
	}

}

func delRole(k string) {

	if k == "admin" {
		return
	}
	_, err := db.Exec("delete from  roles  where name_key=?", k)
	if err != nil {
		log.Println("exec del role failed, ", err)
		return
	}

}

func selectuser(w string, p string, sort string) ([]userinfo, int) {

	emp := []userinfo{}

	query := fmt.Sprintf(`SELECT id,pid,name,phone,
	 callsign,gird,birthday,mdcid,dmrid,
	 sex,nickname,openid,avatar,address, status,
	 last_login_time, login_err_times, last_login_ip,
	 alarm_msg,roles,create_time,update_time,expire_time FROM users  %v  %v  %v  `, w, sort, p)

	//fmt.Println(query)

	rows, err := db.Query(query)

	for rows.Next() {

		r := userinfo{}
		var roles string
		err := rows.Scan(&r.ID, &r.PID, &r.Name, &r.Phone,
			&r.CallSign, &r.Gird, &r.Birthday, &r.MDCID, &r.DMRID,
			&r.Sex, &r.NickName, &r.OpenID, &r.Avatar, &r.Address, &r.Status,
			&r.LastLoginTime, &r.LoginErrTimes, &r.LastLoginIP,
			&r.AlarmMsg, &roles, &r.CreateTime, &r.UpdateTime, &r.ExpireTime,
		)
		if err != nil {
			log.Println("getuser by username err :", err, "\n", query)
			continue
		}
		r.Roles = strings.Split(roles, ",")
		emp = append(emp, r)
	}

	if err != nil {
		log.Println("查询用户列表错误: ", err, "\n", query)
		return nil, 0

	}

	var t int
	q := fmt.Sprintf(`SELECT count(*) as total FROM users  %v  `, w)
	//fmt.Println(q)
	row := db.QueryRow(q)
	err = row.Scan(&t)
	if err != nil {
		log.Println(" 查询用户列表total错误 err:", err, t)
		return nil, 0
	}
	//fmt.Println(emp)
	return emp, t
	//fmt.Println(emp)

}

func getuser(username string) (*userinfo, error) {

	r := &userinfo{}

	var roles string

	username = strings.ToUpper(username)

	query := `SELECT id,pid,name,phone,
	callsign,gird,birthday,mdcid,dmrid,
	sex,nickname,openid,avatar,address, status,
	last_login_time, login_err_times, last_login_ip,
	alarm_msg,roles,create_time,update_time,expire_time FROM users where phone=? or callsign=? `

	row := db.QueryRow(query, username, username)
	err := row.Scan(&r.ID, &r.PID, &r.Name, &r.Phone,
		&r.CallSign, &r.Gird, &r.Birthday, &r.MDCID, &r.DMRID,
		&r.Sex, &r.NickName, &r.OpenID, &r.Avatar, &r.Address, &r.Status,
		&r.LastLoginTime, &r.LoginErrTimes, &r.LastLoginIP,
		&r.AlarmMsg, &roles, &r.CreateTime, &r.UpdateTime, &r.ExpireTime)
	if err != nil {
		log.Println("getuser by username err :", err, "\n", query)
		return nil, err
	}

	r.Roles = strings.Split(roles, ",")

	// r.userinit()
	// userlist.Store(r.CallSign, *r)

	return r, nil
}

func getuserByID(id int) (*userinfo, error) {
	r := &userinfo{}
	var roles string

	query := `SELECT id,pid,name,phone,
	callsign,gird,birthday,mdcid,dmrid,
	sex,nickname,openid,avatar,address, status,
	last_login_time, login_err_times, last_login_ip,
	alarm_msg,roles,create_time,update_time,expire_time FROM users where id=? `

	row := db.QueryRow(query, id)
	err := row.Scan(&r.ID, &r.PID, &r.Name, &r.Phone,
		&r.CallSign, &r.Gird, &r.Birthday, &r.MDCID, &r.DMRID,
		&r.Sex, &r.NickName, &r.OpenID, &r.Avatar, &r.Address, &r.Status,
		&r.LastLoginTime, &r.LoginErrTimes, &r.LastLoginIP,
		&r.AlarmMsg, &roles, &r.CreateTime, &r.UpdateTime, &r.ExpireTime)
	if err != nil {
		log.Println("getuser by id err :", err, "\n", query)
		return nil, err
	}

	r.Roles = strings.Split(roles, ",")
	return r, nil
}

func getEmpListByRole(role string) ([]userinfo, int) {

	emp := []userinfo{}

	query := fmt.Sprintf(`SELECT id,name,callsign,gird,phone,password,birthday,mdcid,dmrid,
	sex,avatar,address,roles,introduction,alarm_msg,status,update_time,last_login_time,
	login_err_times,create_time,openid,nickname,pid,last_login_ip,expire_time FROM users
	 where  roles like '%%%v%%'  ORDER BY id ASC`, role)

	rows, err := db.Query(query)

	if err != nil {
		log.Println("按角色查询用户列表错误: ", err, '\n', query)
		return nil, 0

	}

	for rows.Next() {

		r := userinfo{}
		var roles string
		err := rows.Scan(&r.ID, &r.Name, &r.CallSign, &r.Gird, &r.Phone, &r.Password, &r.Birthday, &r.MDCID, &r.DMRID,
			&r.Sex, &r.Avatar, &r.Address,
			&roles, &r.Introduction, &r.AlarmMsg, &r.Status, &r.UpdateTime, &r.LastLoginTime, &r.LoginErrTimes,
			&r.CreateTime, &r.OpenID, &r.NickName, &r.PID, &r.LastLoginIP, &r.ExpireTime)
		if err != nil {
			log.Println("getuser by username err :", err, "\n", query)
			continue
		}
		r.Roles = strings.Split(roles, ",")
		emp = append(emp, r)
	}

	var t int
	q := fmt.Sprintf(`SELECT count(*) as total FROM users where  roles like '%%%v%%' ' `, role)
	//fmt.Println(q)
	row := db.QueryRow(q)
	err = row.Scan(&t)

	if err != nil {
		log.Println(" 查询教师用户列表total错误 err:", err, '\n', q, t)
		return nil, 0
	}
	//fmt.Println(emp)
	return emp, t
	//fmt.Println(emp)

}

func checkrole(emp *userinfo, roles []string) bool {

	//db.Exec(`select  '{"admin"}' <@ roles from user where ` )
	for _, rv := range roles {
		for _, v := range emp.Roles {
			if v == rv {
				return true
			}
		}
	}

	return false
}

func loginCheck(password string, username string, ip string) ([]string, bool) {

	//pass, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	//fmt.Println("user login:", username, password, string(pass), err)

	username = strings.ToUpper(username)

	type resault struct {
		Password      string   `db:"password"`
		Roles         []string `db:"roles"`
		Status        int      `db:"status"`
		LoginErrTimes int      `db:"login_err_times"`
	}
	r := &resault{}

	var roles string

	row := db.QueryRow("SELECT password ,login_err_times,status,roles FROM users where phone=? or callsign=?", username, username)
	err := row.Scan(&r.Password, &r.LoginErrTimes, &r.Status, &roles)
	if err != nil {
		log.Println("login err:", err, r, password, username)
		return nil, false
	}

	r.Roles = strings.Split(roles, ",")

	var passwordOK bool

	err = bcrypt.CompareHashAndPassword([]byte(r.Password), []byte(password))
	if err == nil {
		passwordOK = true
	}

	if r.LoginErrTimes < 10 && passwordOK {
		_, err = db.Exec(`update users set last_login_time=CURRENT_TIMESTAMP,last_login_ip=?,login_err_times=1 where phone=? or callsign=?`, ip, username, username)
		if err != nil {
			log.Println("update users last_login_time and last_login_ip  failed, ", err)
			return nil, false
		}

	}

	if !passwordOK {
		_, err = db.Exec(`update users set login_err_times = login_err_times + 1 where phone=? or callsign=?`, username, username)
		if err != nil {
			log.Println("update user login_err_times failed ,and password err ", err)
			return nil, false
		}

	}

	//fmt.Println(r.PasswordOK, r.Status, r.LoginErrTimes)

	return r.Roles, passwordOK && r.Status == 1 && r.LoginErrTimes < 10

}

// func roleCheck() {
// 	db.Exec(`select  '{"admin"}' <@ role from user`)
// }

// func updateop() {
// 	res, err := db.Exec("update user set username=? where user_id=?", "stu0003", 1)
// 	if err != nil {
// 		fmt.Println("exec failed, ", err)
// 		return
// 	}
// 	row, err := res.RowsAffected()
// 	if err != nil {
// 		fmt.Println("rows failed, ", err)
// 	}
// 	fmt.Println("update succ:", row)
// }

// func updatePassword() {
// 	db.Exec("UPDATE user  SET password = crypt('123', gen_salt('md5'))")
// 	db.Exec("SELECT password = crypt('123', password) FROM user where mobile_phone=")
// }

func addUser(e *userinfo) error {

	password, err := bcrypt.GenerateFromPassword([]byte(e.Password), bcrypt.DefaultCost)

	if err != nil {
		return err
	}

	// 插入数据

	//	fmt.Println("user:", e)

	roles := strings.Join(e.Roles, ",")
	query := `INSERT INTO users (pid,name,phone,sex,callsign,mdcid,dmrid,gird,address,birthday,introduction,nickname,openid,last_login_ip,last_login_time,
		avatar,status,password,roles, alarm_msg,		
		expire_time,create_time,login_err_times,update_time) 
	VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,
		CURRENT_TIMESTAMP,0,CURRENT_TIMESTAMP)`

	stmt, err := db.Prepare(query)

	if err != nil {
		log.Println("add user failed 1 , ", err, '\n', query)
		return err
	}

	//	e.Avatar = "https://wpimg.wallstcn.com/f778738c-e4f8-4870-b634-56703b4acafe.gif"
	e.Avatar = conf.WeiXin.AvatarURL

	res, err := stmt.Exec(e.PID, e.Name, e.Phone, e.Sex, e.CallSign, e.MDCID, e.DMRID, e.Gird, e.Address, e.Birthday, e.Introduction, e.NickName, e.OpenID, e.LastLoginIP, e.LastLoginTime,
		e.Avatar, e.Status, password, roles, e.AlarmMsg, e.ExpireTime)
	// Named queries can use structs, so if you have an existing struct (i.e. person := &Person{}) that you have populated, you can pass it in as &person
	//	tx.NamedExec("INSERT INTO person (first_name, last_name, email) VALUES (:first_name, :last_name, :email)", &Person{"Jane", "Citizen", "jane.citzen@example.com"})
	if err != nil {
		log.Println("add user failed 2, ", err, '\n', query)
		return err
	}

	id, err := res.LastInsertId()

	if err != nil {
		log.Println("add user failed 3, ", err, '\n', query)
		return err
	}

	fmt.Println(id)

	e.userinit()
	userlist.Store(e.CallSign, e)

	return nil

}

func deleteUser(e *userinfo) {

	_, err := db.Exec("delete from users where id=?", e.ID)
	if err != nil {
		log.Println("delete user failed, ", err)
		return
	}

	userlist.Delete(e.CallSign)

}

func updateUser(e *userinfo) error {

	roles := strings.Join(e.Roles, ",")

	_, err := db.Exec(`update users set name=?,phone=?,sex=?,callsign=?,	mdcid=?,dmrid=?,gird=?,address=?,birthday=?,introduction=?,
		avatar=?,status=?,alarm_msg=?,expire_time=?,update_time=CURRENT_TIMESTAMP,roles=?,pid=?  where id=?`,
		e.Name, e.Phone, e.Sex, e.CallSign, e.MDCID, e.DMRID, e.Gird, e.Address, e.Birthday, e.Introduction, e.Avatar, e.Status, e.AlarmMsg, e.ExpireTime, roles, e.PID, e.ID)
	if err != nil {
		log.Println("update user failed, ", err)
		return err
	}

	if e.Status == 1 {
		_, err := db.Exec("update users set login_err_times=0  where id=?", e.ID)
		if err != nil {
			log.Println("reset user login_err_time failed, ", err)
			return err
		}

	}

	if e.Password != "" {

		password, err := bcrypt.GenerateFromPassword([]byte(e.Password), bcrypt.DefaultCost)

		if err != nil {
			return err
		}
		//	fmt.Println("password:", e.Password, len(e.Password))
		_, err = db.Exec("update users set password=?  where id=?", password, e.ID)
		if err != nil {
			log.Println("update user password failed, ", err)
			return err
		}

	}

	e.userinit()
	userlist.Store(e.CallSign, e)
	mdcidmap.Store(e.MDCID, e.CallSign)
	dmridmap.Store(e.DMRID, e.CallSign)

	return nil

}

func updateUseravatar(e *userinfo) error {

	_, err := db.Exec(`update users set avatar=?,   update_time=CURRENT_TIMESTAMP  where id=?`,
		e.Avatar, e.ID)
	if err != nil {
		log.Println("update user failed, ", err)
		return err
	}

	if u, okok := userlist.Load(e.CallSign); okok {
		u.(*userinfo).Avatar = e.Avatar

	}

	return nil

}

func updateweixininfo(e wxUserInfo, studentid int) error {

	//'25001'||to_char(now(),'YYMMDDHHMMSS')||to_char(id,'fm00000')||to_char(ceil(random()*(100-1)+1),'fm00')+

	//卡号自动生成， 校区编号+年月日时分秒+学员ID+随机数

	_, err := db.Exec(`UPDATE students set openid=?,avatar=?,nickname=?,update_time=now() where id=?`, e.OpenID, e.Headimgurl, e.NickName, studentid)
	if err != nil {
		log.Println("update Student weixin phonecode, ", err)
		return err
	}

	return nil

}

func updateUserPassword(id int, password string) error {

	if password != "" {

		password, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

		if err != nil {
			return err
		}
		//	fmt.Println("password:", e.Password, len(e.Password))
		_, err = db.Exec("update users set password=?  where id=?", password, id)
		if err != nil {
			log.Println("update user password failed, ", err)
			return errors.New("passord update err")
		}

	}
	return nil

}

func updateUserProfile(id int, dmrid string, mdcid string, avatar string, password string) error {
	_, err := db.Exec(`update users set dmrid=?, mdcid=?, avatar=?, update_time=CURRENT_TIMESTAMP where id=?`, dmrid, mdcid, avatar, id)
	if err != nil {
		log.Println("update user profile failed, ", err)
		return err
	}

	if password != "" {
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}

		_, err = db.Exec("update users set password=? where id=?", passwordHash, id)
		if err != nil {
			log.Println("update user profile password failed, ", err)
			return errors.New("passord update err")
		}
	}

	u, err := getuserByID(id)
	if err != nil {
		log.Println("reload user profile failed, ", err)
		return err
	}

	if oldUser, ok := userlist.Load(u.CallSign); ok {
		old := oldUser.(*userinfo)
		if old.MDCID != "" && old.MDCID != u.MDCID {
			mdcidmap.Delete(old.MDCID)
		}
		if old.DMRID != "" && old.DMRID != u.DMRID {
			dmridmap.Delete(old.DMRID)
		}
	}

	u.userinit()
	userlist.Store(u.CallSign, u)
	if u.MDCID != "" {
		mdcidmap.Store(u.MDCID, u.CallSign)
	}
	if u.DMRID != "" {
		dmridmap.Store(u.DMRID, u.CallSign)
	}

	return nil
}
