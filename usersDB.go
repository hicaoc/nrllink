package main

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"golang.org/x/crypto/bcrypt"
	//"github.com/lib/pq"
)

// func sqlmain() {
// 	db := getDB()
// 	if db != nil {
// 		UpdateUser(db)
// 	}
// }

// func getsha1(s string) string {

// 	// 生成一个hash的模式是`sha1.New()`，`sha1.Write(bytes)`
// 	// 然后是`sha1.Sum([]byte{})`，下面我们开始一个新的hash
// 	// 示例
// 	h := sha1.New()

// 	// 写入要hash的字节，如果你的参数是字符串，使用`[]byte(s)`
// 	// 把它强制转换为字节数组
// 	h.Write([]byte(s))

// 	// 这里计算最终的hash值，Sum的参数是用来追加而外的字节到要
// 	// 计算的hash字节里面，一般来讲，如果上面已经把需要hash的
// 	// 字节都写入了，这里就设为nil就可以了
// 	bs := h.Sum(nil)

// 	// SHA1散列值经常以16进制的方式输出，例如git commit就是
// 	// 这样，所以可以使用`%x`来将散列结果格式化为16进制的字符串

// 	return string(bs)
// 	// fmt.Println(s)
// 	// fmt.Printf("%x\n", bs)
// }

type userinfo struct {
	//uUID     string `db:"uuid"`
	PID      string `db:"pid" json:"pid"`
	ID       int    `db:"id" json:"id"`
	Name     string `db:"name" json:"name"`
	CallSign string `db:"callsign" json:"callsign"`
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

	Routes        string `json:"routes" db:"routes"`
	Status        int    `json:"status" db:"status"`
	LastLoginTime string `json:"last_login_time" db:"last_login_time"`
	LastLoginIP   string `json:"last_login_ip" db:"last_login_ip"`
	LoginErrTimes int    `json:"login_err_times" db:"login_err_times"`
	AlarmMsg      bool   `json:"alarm_msg" db:"alarm_msg"`
	NickName      string `json:"nickname" db:"nickname"`
	OpenID        string `json:"openid" db:"openid"`
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
	 callsign,gird,birthday,
	 sex,nickname,openid,avatar,address, status,
	 last_login_time, login_err_times, last_login_ip,
	 alarm_msg,roles,create_time,update_time FROM users  %v   ORDER by id asc %v  `, w, p)

	//fmt.Println(query)

	rows, err := db.Query(query)

	for rows.Next() {

		r := userinfo{}
		var roles string
		err := rows.Scan(&r.ID, &r.PID, &r.Name, &r.Phone,
			&r.CallSign, &r.Gird, &r.Birthday,
			&r.Sex, &r.NickName, &r.OpenID, &r.Avatar, &r.Address, &r.Status,
			&r.LastLoginTime, &r.LoginErrTimes, &r.LastLoginIP,
			&r.AlarmMsg, &roles, &r.CreateTime, &r.UpdateTime,
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

func getuser(username string) *userinfo {

	r := &userinfo{}

	var roles string

	query := `SELECT id,pid,name,phone,
	callsign,gird,birthday,
	sex,nickname,openid,avatar,address, status,
	last_login_time, login_err_times, last_login_ip,
	alarm_msg,roles,create_time,update_time FROM users where phone=?  `

	row := db.QueryRow(query, username)
	err := row.Scan(&r.ID, &r.PID, &r.Name, &r.Phone,
		&r.CallSign, &r.Gird, &r.Birthday,
		&r.Sex, &r.NickName, &r.OpenID, &r.Avatar, &r.Address, &r.Status,
		&r.LastLoginTime, &r.LoginErrTimes, &r.LastLoginIP,
		&r.AlarmMsg, &roles, &r.CreateTime, &r.UpdateTime)
	if err != nil {
		log.Println("getuser by username err :", err, "\n", query)
		return r
	}

	r.Roles = strings.Split(roles, ",")

	// r.userinit()
	// userlist.Store(r.CallSign, *r)

	return r
}

func getEmpListByRole(role string) ([]userinfo, int) {

	emp := []userinfo{}

	query := fmt.Sprintf(`SELECT * FROM users
	 where  roles like '%%%v%%'  ORDER BY id ASC`, role)

	rows, err := db.Query(query)

	if err != nil {
		log.Println("按角色查询用户列表错误: ", err, '\n', query)
		return nil, 0

	}

	for rows.Next() {

		r := userinfo{}
		var roles string
		err := rows.Scan(&r.ID, &r.Name, &r.CallSign, &r.Gird, &r.Phone, &r.Password, &r.Birthday, &r.Sex, &r.Avatar, &r.Address,
			&roles, &r.Introduction, &r.AlarmMsg, &r.Status, &r.UpdateTime, &r.LastLoginTime, &r.LoginErrTimes,
			&r.CreateTime, &r.OpenID, &r.NickName, &r.PID, &r.LastLoginIP)
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

	type resault struct {
		Password      string   `db:"password"`
		Roles         []string `db:"roles"`
		Status        int      `db:"status"`
		LoginErrTimes int      `db:"login_err_times"`
	}
	r := &resault{}

	var roles string

	row := db.QueryRow("SELECT password ,login_err_times,status,roles FROM users where phone=?", username)
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
		_, err = db.Exec(`update users set last_login_time=CURRENT_TIMESTAMP,last_login_ip=?,login_err_times=1 where phone=?`, ip, username)
		if err != nil {
			log.Println("update users last_login_time and last_login_ip  failed, ", err)
			return nil, false
		}

	}

	if !passwordOK {
		_, err = db.Exec(`update users set login_err_times = login_err_times + 1 where phone=?`, username)
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
	query := `INSERT INTO users (pid,name,phone,sex,callsign,gird,address,birthday,introduction,nickname,openid,last_login_ip,last_login_time,
		avatar,status,password,roles, alarm_msg,		
		create_time,login_err_times,update_time) 
	VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,
		CURRENT_TIMESTAMP,0,CURRENT_TIMESTAMP)`

	stmt, err := db.Prepare(query)

	if err != nil {
		log.Println("add user failed 1 , ", err, '\n', query)
		return err
	}

	//	e.Avatar = "https://wpimg.wallstcn.com/f778738c-e4f8-4870-b634-56703b4acafe.gif"
	e.Avatar = conf.WeiXin.AvatarURL

	res, err := stmt.Exec(e.PID, e.Name, e.Phone, e.Sex, e.CallSign, e.Gird, e.Address, e.Birthday, e.Introduction, e.NickName, e.OpenID, e.LastLoginIP, e.LastLoginTime,
		e.Avatar, e.Status, password, roles, e.AlarmMsg)
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
	userlist.Store(e.CallSign, *e)

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

	_, err := db.Exec(`update users set name=?,phone=?,sex=?,callsign=?,gird=?,address=?,birthday=?,introduction=?,
	avatar=?,status=?,alarm_msg=?,   update_time=CURRENT_TIMESTAMP,roles=?,pid=?  where id=?`,
		e.Name, e.Phone, e.Sex, e.CallSign, e.Gird, e.Address, e.Birthday, e.Introduction, e.Avatar, e.Status, e.AlarmMsg, roles, e.PID, e.ID)
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
	userlist.Store(e.CallSign, *e)

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
