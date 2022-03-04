package main

import (
	"errors"
	"fmt"
	"log"

	jsoniter "github.com/json-iterator/go"
	"github.com/lib/pq"
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
	Introduction string         `db:"introduction" json:"introduction"`
	Avatar       string         `db:"avatar" json:"avatar"`
	Roles        pq.StringArray `db:"roles" json:"roles"`
	UpdateTime   string         `db:"update_time" json:"update_time"`
	CreateTime   string         `db:"create_time" json:"create_time"`

	Routes        jsoniter.RawMessage `json:"routes" db:"routes"`
	Status        int                 `json:"status" db:"status"`
	LastLoginTime string              `json:"last_login_time" db:"last_login_time"`
	LastLoginIP   string              `json:"last_login_ip" db:"last_login_ip"`
	LoginErrTimes int                 `json:"login_err_times" db:"login_err_times"`
	AlarmMsg      bool                `json:"alarm_msg" db:"alarm_msg"`
	NickName      string              `json:"nickname" db:"nickname"`
	OpenID        string              `json:"openid" db:"openid"`
}

type role struct {
	ID          int                 `json:"id" db:"id"`
	NameKey     string              `json:"key" db:"name_key"`
	Name        string              `json:"name" db:"name"`
	Description string              `json:"description" db:"description"`
	Routes      jsoniter.RawMessage `json:"routes" db:"routes"`
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

func getRoles(query string) []role {
	r := []role{}

	q := fmt.Sprintf("SELECT * FROM roles %v ", query)

	err := db.Select(&r, q)

	if err != nil {
		log.Println("查询角色列表错误: ", err)
		return nil

	}
	//fmt.Println(emp)
	return r

}

func (s *role) String() string {

	return fmt.Sprintf("NameKey:%v Name:%v", s.NameKey, s.Name)

}

func (s *userinfo) String() string {

	return fmt.Sprintf("Name:%v Phone:%v ", s.Name, s.Phone)

}

// func getRoleByID(id int) *role {

// 	r := &role{}

// 	err := db.Get(r, "SELECT * from roles where id=$1", id)
// 	if err != nil {
// 		log.Println("query role err:", err, r, id)
// 	}
// 	return r
// }
func getRoleByKey(key string) *role {

	r := &role{}

	err := db.Get(r, "SELECT * from roles where name_key=$1", key)
	if err != nil {
		log.Println("query role by key err:", err, r, key)
	}
	return r
}

func addRole(r *role) {

	//	fmt.Println("routes:", r)

	_, err := db.Exec("insert into  roles (name_key,name,description,routes) VALUES ($1,$2,$3,$4)", r.NameKey, r.Name, r.Description, r.Routes)
	if err != nil {
		log.Println("add role failed, ", err)
		return
	}

}

func updateRole(r *role) {

	_, err := db.Exec("update roles set name_key=$1,name=$2,description=$3,routes=$4 where id=$5", r.NameKey, r.Name, r.Description, r.Routes, r.ID)
	if err != nil {
		log.Println("exec update role failed, ", err)
		return
	}

}

func delRole(k string) {

	if k == "admin" {
		return
	}
	_, err := db.Exec("delete from  roles  where name_key=$1", k)
	if err != nil {
		log.Println("exec del role failed, ", err)
		return
	}

}

func selectuser(w string, p string, sort string) ([]userinfo, int) {

	emp := []userinfo{}

	query := fmt.Sprintf(`SELECT  id,pid,name,phone,callsign,gird,to_char(birthday,'YYYY-MM-DD') as birthday,
	sex,nickname,openid,avatar,status,to_char(last_login_time,'YYYY-MM-DD HH24:MI:SS') as last_login_time,
	login_err_times,alarm_msg,roles,create_time,update_time FROM users  %v   ORDER by id asc %v  `, w, p)

	//fmt.Println(query)

	err := db.Select(&emp, query)

	if err != nil {
		log.Println("查询用户列表错误: ", err, "\n", query)
		return nil, 0

	}

	t := &total{}
	q := fmt.Sprintf(`SELECT count(*) as total FROM users  %v  `, w)
	//fmt.Println(q)
	err2 := db.Get(t, q)
	if err2 != nil {
		log.Println(" 查询用户列表total错误 err:", err, t)
		return nil, 0
	}
	//fmt.Println(emp)
	return emp, t.Total
	//fmt.Println(emp)

}

// func selectuserBySuperadmin(idlist pq.Int64Array, w string, p string, sort string) ([]userinfo, int) {

// 	emp := []userinfo{}

// 	query := fmt.Sprintf(`SELECT  id,pid,name,phone,callsign,gird,
// 	sex,nickname,openid,avatar,status,to_char(last_login_time,'YYYY-MM-DD HH24:MI:SS') as last_login_time,
// 	login_err_times,alarm_msg,roles,create_time,update_time FROM users  %v ORDER by id asc %v  `, w, p)

// 	//fmt.Println(query)

// 	err := db.Select(&emp, query, idlist)

// 	if err != nil {
// 		log.Println("查询用户列表错误: ", err, "\n", query)
// 		return nil, 0

// 	}

// 	t := &total{}
// 	q := fmt.Sprintf(`SELECT count(*) as total FROM users  %v  `, w)
// 	//fmt.Println(q)
// 	err2 := db.Get(t, q, idlist)
// 	if err2 != nil {
// 		log.Println(" 查询用户列表total错误 err:", err, t)
// 		return nil, 0
// 	}
// 	//fmt.Println(emp)
// 	return emp, t.Total
// 	//fmt.Println(emp)

// }

func getuser(username string) *userinfo {

	r := &userinfo{}

	query := `SELECT  id,pid,name,phone,callsign,gird,to_char(birthday,'YYYY-MM-DD') as birthday,
	sex,	nickname,openid,avatar,to_char(last_login_time,'YYYY-MM-DD HH24:MI:SS') as last_login_time,
	login_err_times,alarm_msg,roles,create_time,update_time FROM users where phone=$1  `

	err := db.Get(r, query, username)
	if err != nil {
		log.Println("getuser by username err :", err, "\n", query)
	}

	r.userinit()
	userlist.Store(r.CallSign, *r)
	// r.userinit()
	// userlist.Store(r.CallSign, *r)

	return r
}

// func getuserByID(id int) *userinfo {

// 	r := &userinfo{}

// 	if id == 0 {
// 		return r
// 	}

// 	//query := "SELECT  id,name,phone,to_char(birthday,'YYYY-MM-DD') as birthday,to_char(job_time,'YYYY-MM-DD') as job_time,sex,position,avatar,roles,update_time FROM user where id=$1"

// 	//fmt.Println(id, query)
// 	err := db.Get(r, `SELECT  id,pid,name,phone,callsign,gird,
// 	sex,	nickname,openid,avatar,to_char(last_login_time,'YYYY-MM-DD HH24:MI:SS') as last_login_time,
// 	login_err_times,alarm_msg,roles,create_time,update_time FROM users  where id=$1`, id)
// 	if err != nil {
// 		log.Println("get user by ID err:", err, r, id)
// 	}
// 	return r
// }

func getEmpListByRole(role string) ([]userinfo, int) {

	emp := []userinfo{}

	query := fmt.Sprintf(`SELECT  id,pid,name,phone,callsign,gird,to_char(birthday,'YYYY-MM-DD') as birthday,
	sex,nickname,openid,avatar,to_char(last_login_time,'YYYY-MM-DD HH24:MI:SS') as last_login_time,
	login_err_times,alarm_msg,roles,create_time,update_time FROM users
	 where  roles @> '{%v}'  ORDER BY id ASC`, role)

	err := db.Select(&emp, query)

	if err != nil {
		log.Println("按角色查询用户列表错误: ", err, '\n', query)
		return nil, 0

	}

	t := &total{}
	q := fmt.Sprintf(`SELECT count(*) as total FROM users where  roles @> '{%v}' ' `, role)
	//fmt.Println(q)
	err2 := db.Get(t, q)
	if err2 != nil {
		log.Println(" 查询教师用户列表total错误 err:", err, '\n', q, t)
		return nil, 0
	}
	//fmt.Println(emp)
	return emp, t.Total
	//fmt.Println(emp)

}

// func getEmpIDListByRole(role string, CurrentArea int) []int {

// 	emp := []int{}
// 	type idlist struct {
// 		ID int `json:"id"`
// 	}

// 	place := &idlist{}

// 	query := fmt.Sprintf(`SELECT  id FROM users  where  roles @> '{%v}' ORDER BY id ASC`, role, CurrentArea)

// 	rows, err := db.Queryx(query)

// 	if err != nil {
// 		log.Println("query user id  list by role  err:", err, "\n", query)
// 	}

// 	for rows.Next() {
// 		err := rows.StructScan(place)
// 		if err != nil {
// 			log.Println("query  user id   rows err:", err)
// 		}
// 		emp = append(emp, place.ID)

// 	}
// 	return emp

// 	//fmt.Println(emp)

// }

// func getEmpListByPosition(position string) []user {

// 	emp := []user{}

// 	err := db.Select(&emp, "SELECT id,name,phone,to_char(birthday,'YYYY-MM-DD') as birthday,to_char(job_time,'YYYY-MM-DD') as job_time,sex,avatar,roles,update_time FROM user where position=$1 ORDER BY id ASC", position)

// 	if err != nil {
// 		log.Println("按角色查询用户列表错误: ", err)
// 		return nil

// 	}
// 	//fmt.Println(emp)
// 	return emp

// }

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

	type resault struct {
		PasswordOK    bool           `db:"password_ok"`
		Roles         pq.StringArray `db:"roles"`
		Status        int            `db:"status"`
		LoginErrTimes int            `db:"login_err_times"`
	}
	r := &resault{}

	err := db.Get(r, "SELECT password = crypt($1, password) as password_ok ,login_err_times,status,roles FROM users where phone=$2", password, username)
	if err != nil {
		log.Println("login err:", err, r, password, username)
		return nil, false
	}

	// if r.LoginErrTimes >= 10 {
	// 	_, err = db.Exec(`update user login_err_times+1 where phone=$1`, username)
	// 	if err != nil {
	// 		log.Println("update user login_err_times failed, ", err)
	// 		return nil, false
	// 	}

	// 	return nil, false

	// }

	if r.LoginErrTimes < 10 && r.PasswordOK {
		_, err = db.Exec(`update users set last_login_time=now(),last_login_ip=$1,login_err_times=1 where phone=$2`, ip, username)
		if err != nil {
			log.Println("update users last_login_time and last_login_ip  failed, ", err)
			return nil, false
		}

	}

	if !r.PasswordOK {
		_, err = db.Exec(`update users set login_err_times = login_err_times + 1 where phone=$1`, username)
		if err != nil {
			log.Println("update user login_err_times failed ,and password err ", err)
			return nil, false
		}

	}

	//fmt.Println(r.PasswordOK, r.Status, r.LoginErrTimes)

	return r.Roles, r.PasswordOK && r.Status == 1 && r.LoginErrTimes < 10

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

	//	fmt.Println("user:", e)
	query := `INSERT INTO users (pid,name,phone,sex,callsign,gird,address,birthday,introduction,
		avatar,status,password,roles,alarm_msg,last_login_time,login_err_times,update_time) 
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,crypt($12, gen_salt('md5')),$13,false,now(),0,now())`
	//	e.Avatar = "https://wpimg.wallstcn.com/f778738c-e4f8-4870-b634-56703b4acafe.gif"
	e.Avatar = conf.avatarurl

	_, err := db.Exec(query,
		e.PID, e.Name, e.Phone, e.Sex, e.CallSign, e.Gird, e.Address, e.Birthday, e.Introduction,
		e.Avatar, e.Status, e.Password, e.Roles)
	// Named queries can use structs, so if you have an existing struct (i.e. person := &Person{}) that you have populated, you can pass it in as &person
	//	tx.NamedExec("INSERT INTO person (first_name, last_name, email) VALUES (:first_name, :last_name, :email)", &Person{"Jane", "Citizen", "jane.citzen@example.com"})
	if err != nil {
		log.Println("add user failed, ", err, '\n', query)
		return err
	}

	e.userinit()
	userlist.Store(e.CallSign, *e)

	return nil

}

func deleteUser(e *userinfo) {

	_, err := db.Exec("delete from users where id=$1", e.ID)
	if err != nil {
		log.Println("delete user failed, ", err)
		return
	}

	userlist.Delete(e.CallSign)

}

// func updateuserWeixinInfo(e wxUserInfo, userid int) error {

// 	//'25001'||to_char(now(),'YYMMDDHHMMSS')||to_char(id,'fm00000')||to_char(ceil(random()*(100-1)+1),'fm00')+

// 	//卡号自动生成， 地区编号+年月日时分秒+学员ID+随机数

// 	query := fmt.Sprintf(`update user set openid=$1,avatar=$2,nickname=$3,update_time=now() where id=$4`)

// 	_, err := db.Exec(query, e.OpenID, e.Headimgurl, e.NickName, userid)
// 	if err != nil {
// 		log.Println("update user weixin phonecode, ", err)
// 		return err
// 	}

// 	return nil

// }

func updateUser(e *userinfo) {

	_, err := db.Exec(`update users set name=$1,phone=$2,sex=$3,callsign=$4,gird=$5,address=$6,birthday=$7,introduction=$8,
	avatar=$9,status=$10,alarm_msg=$11,  update_time=now(),roles=$12,pid=$13  where id=$14`,
		e.Name, e.Phone, e.Sex, e.CallSign, e.Gird, e.Address, e.Birthday, e.Introduction, e.Avatar, e.Status, e.AlarmMsg, e.Roles, e.PID, e.ID)
	if err != nil {
		log.Println("update user failed, ", err)
		return
	}

	if e.Status == 1 {
		_, err := db.Exec("update users set login_err_times=0  where id=$1", e.ID)
		if err != nil {
			log.Println("reset user login_err_time failed, ", err)
			return
		}

	}

	if e.Password != "" {
		//	fmt.Println("password:", e.Password, len(e.Password))
		_, err := db.Exec("update users set password= crypt($1, gen_salt('md5'))  where id=$2", e.Password, e.ID)
		if err != nil {
			log.Println("update user password failed, ", err)
			return
		}

	}

	e.userinit()
	userlist.Store(e.CallSign, *e)

}

func updateUserPassword(id int, password string) error {

	if password != "" {
		//	fmt.Println("password:", e.Password, len(e.Password))
		_, err := db.Exec("update users set password= crypt($1, gen_salt('md5'))  where id=$2", password, id)
		if err != nil {
			log.Println("update user password failed, ", err)
			return errors.New("passord update err")
		}

	}
	return nil

}
