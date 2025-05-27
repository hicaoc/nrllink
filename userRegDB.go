package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"
)

type reguser struct {
	ID          int    `json:"id" db:"id"`
	Name        string `json:"name" db:"name"`
	Phone       string `json:"phone" db:"phone"`
	Password    string `db:"password" json:"password"`
	CallSign    string `db:"callsign" json:"callsign"`
	Birthday    string `db:"birthday" json:"birthday"`
	Sex         int    `db:"sex" json:"sex"`
	Address     string `db:"address" json:"address"`
	Mail        string `db:"mail" json:"mail"`
	OpCertPath  string `db:"op_cert_path" json:"op_cert_path"`
	LicensePath string `db:"license_path" json:"license_path"`
	Status      int    `json:"status" db:"status"` //1 未审核，2：审核通过
	CreateTime  string `db:"create_time" json:"create_time"`
	UpdateTime  string `db:"update_time" json:"update_time"`
	Note        string `db:"note" json:"note"`
}

func createRegUser(e *reguser) error {

	password, err := bcrypt.GenerateFromPassword([]byte(e.Password), bcrypt.DefaultCost)

	if err != nil {
		return err
	}

	// 插入用户数据到数据库
	query := `INSERT INTO registers (callsign, name, phone,sex,address,birthday,mail, password, op_cert_path, license_path, status,note) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	stmt, err := db.Prepare(query)

	if err != nil {
		log.Println("add reg user failed 1 , ", err, '\n', query)
		return err
	}

	res, err := stmt.Exec(e.CallSign, e.Name, e.Phone, e.Sex, e.Address, e.Birthday, e.Mail, password, e.OpCertPath, e.LicensePath, e.Status, e.Note)
	if err != nil {
		log.Println("add reg user failed 2, ", err, '\n', query)

		err2 := updateRegUserCertPath(e.OpCertPath, e.LicensePath, e.CallSign)

		if err2 != nil {
			log.Println("add reg user ok , ", err, '\n', query)
			return nil
		}

		return err
	}

	id, err := res.LastInsertId()

	if err != nil {
		log.Println("add reg user failed 3, ", err, '\n', query)
		return err
	}

	log.Println("插入注册用户数据到数据库", id)

	return nil

	// 插入数据

	//	fmt.Println("user:", e)

}

func selectReguser(w string, p string, sort string) ([]reguser, int) {

	emp := []reguser{}

	query := fmt.Sprintf(`SELECT 
	 id,name,phone,address,sex,
	 callsign,status,
	 op_cert_path,license_path, 
	 birthday,sex,address,mail,note, 
	 create_time,update_time FROM registers  %v %v %v  `, w, sort, p)

	//fmt.Println(query)

	rows, err := db.Query(query)

	if err != nil {
		log.Println("查询注册用户列表错误: ", err, "\n", query)
		return nil, 0

	}

	for rows.Next() {

		r := reguser{}

		err := rows.Scan(&r.ID, &r.Name, &r.Phone, &r.Address, &r.Sex,
			&r.CallSign, &r.Status,
			&r.OpCertPath, &r.LicensePath,
			&r.Birthday, &r.Sex, &r.Address, &r.Mail, &r.Note,
			&r.CreateTime, &r.UpdateTime,
		)
		if err != nil {
			log.Println("getreguser err :", err, "\n", query)
			continue
		}

		emp = append(emp, r)
	}

	if err != nil {
		log.Println("查询注册用户列表错误: ", err, "\n", query)
		return nil, 0

	}

	var t int
	q := fmt.Sprintf(`SELECT count(*) as total FROM registers  %v  `, w)
	//fmt.Println(q)
	row := db.QueryRow(q)
	err = row.Scan(&t)
	if err != nil {
		log.Println(" 查询注册用户列表total错误 err:", err, t)
		return nil, 0
	}
	//fmt.Println(emp)
	return emp, t
	//fmt.Println(emp)

}

func addUserReg(ctx context.Context, u *reguser) error {

	r := &reguser{}

	query1 := `SELECT  
	id,name,phone,password,
	callsign,status,
	birthday,sex,address,mail,note, 
	create_time,update_time
	FROM registers  where id=?  `

	row := db.QueryRow(query1, u.ID)
	err := row.Scan(&r.ID, &r.Name, &r.Phone, &r.Password,
		&r.CallSign, &r.Status,
		&r.Birthday, &r.Sex, &r.Address, &r.Mail, &r.Note,
		&r.CreateTime, &r.UpdateTime)
	if err != nil {
		log.Println("getuser by username err :", err, "\n", query1)
		return err
	}

	// 插入数据

	e := &userinfo{
		Name:     r.Name,
		Phone:    r.Phone,
		Password: r.Password,
		CallSign: r.CallSign,
		Birthday: r.Birthday,
		Sex:      r.Sex,
		Address:  r.Address,
		Mail:     r.Mail,
		Status:   1,
	}

	//	fmt.Println("user:", e)

	query := `INSERT INTO users 
	 (pid,
	 name,
	 phone,
	 sex,
	 callsign,
	 gird,
	 address,
	 birthday,
	 introduction,
	 nickname,
	 openid,
	 last_login_ip,
	 last_login_time,
		avatar,
		status,
		password,
		roles, 
		alarm_msg,		
		create_time,
		login_err_times,
		update_time) 
	VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,
		CURRENT_TIMESTAMP,0,CURRENT_TIMESTAMP)`

	stmt, err := db.Prepare(query)

	if err != nil {
		log.Println("add user failed 1 , ", err, '\n', query)
		return err
	}

	e.Avatar = conf.WeiXin.AvatarURL

	res, err := stmt.Exec(e.PID, e.Name, e.Phone, e.Sex, e.CallSign, e.Gird, e.Address, e.Birthday, e.Introduction, e.NickName, e.OpenID, e.LastLoginIP, e.LastLoginTime,
		e.Avatar, e.Status, e.Password, "ham", e.AlarmMsg)

	if err != nil {
		log.Println("add user failed 2, ", err, '\n', query)
		return err
	}

	id, err := res.LastInsertId()

	if err != nil {
		log.Println("add user failed 3, ", err, '\n', query)
		return err
	}

	_, err = db.Exec(`update registers set 
     status = 2 ,note = ?,
     update_time=CURRENT_TIMESTAMP 
	 where id=?`,
		u.Note, u.ID)
	if err != nil {
		log.Println("update reg user status failed, ", err)
		return err
	}

	fmt.Println(id)

	e.userinit()
	userlist.Store(e.CallSign, e)

	return nil

}

func updateRegUserCertPath(opCertPath, licensePath, callsign string) error {

	query := ""

	if opCertPath != "" {
		query = `update registers set 
			op_cert_path = ?,
			update_time=CURRENT_TIMESTAMP
			 where callsign=? `

		_, err := db.Exec(query, opCertPath, callsign)
		if err != nil {
			log.Println("update reg user OpCertPath failed, ", err)
			return err
		}

	}

	if licensePath != "" {
		query = `update registers set 
			license_path = ?,
			update_time=CURRENT_TIMESTAMP
	 		where callsign=?`

		_, err := db.Exec(query, licensePath, callsign)
		if err != nil {
			log.Println("update reg user LicensePath failed, ", err)
			return err
		}
	}

	return nil

}

func updateRegUser(e *reguser) error {

	_, err := db.Exec(`update registers set 
	name=?,phone=?,sex=?,callsign=?,	
	address=?,birthday=?,mail=?,
	status=?,note=?,
	create_time=CURRENT_TIMESTAMP,
	update_time=CURRENT_TIMESTAMP,
	 where id=?`,
		e.Name, e.Phone, e.Sex, e.CallSign, e.Address, e.Birthday, e.Mail, e.Status, e.Note, e.ID)
	if err != nil {
		log.Println("update reg user failed, ", err)
		return err
	}

	if e.Password != "" {

		password, err := bcrypt.GenerateFromPassword([]byte(e.Password), bcrypt.DefaultCost)

		if err != nil {
			return err
		}
		//	fmt.Println("password:", e.Password, len(e.Password))
		_, err = db.Exec("update registers set password=?  where id=?", password, e.ID)
		if err != nil {
			log.Println("update registers password failed, ", err)
			return err
		}

	}

	return nil

}

func deleteRegUser(e *reguser) {

	_, err := db.Exec("delete from registers where id=?", e.ID)
	if err != nil {
		log.Println("delete registers user failed, ", err)
		return
	}

}

// 保存文件到指定路径
func saveFile(file io.Reader, path string) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, file)
	return err
}
