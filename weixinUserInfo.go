package main

import (
	_ "database/sql"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/lib/pq"
	//_ "github.com/lib/pq"
)

type wxUserInfo struct {
	ID             string         `json:"id" db:"id"`
	Schname        string         `json:"schname" db:"schname"`
	MasterSchname  string         `json:"master_schname" db:"master_schname"`
	SchoolID       int            `json:"school_id" db:"school_id"`
	SchoolName     string         `json:"school_name" db:"school_name"`
	SchList        pq.StringArray `json:"sch_list" db:"sch_list"`
	ServerURL      string         `json:"server_url" db:"server_url"`
	ISHAM          bool           `json:"is_HAM" db:"is_HAM"`
	Phone          string         `json:"phone" db:"phone"`
	PhoneCode      int            `json:"phone_code" db:"phone_code"`
	PhoneCodeTime  string         `json:"phone_code_time" db:"phone_code_time"`
	Subscribe      int            `json:"subscribe" db:"subscribe"`
	OpenID         string         `json:"openid" db:"openid"`
	MPopenID       string         `json:"mpopenid" db:"mpopenid" `
	Name           string         `json:"name" db:"name"`
	NickName       string         `json:"nickname" db:"nickname"`
	Sex            int            `json:"sex" db:"sex"`
	Language       string         `json:"language" db:"language"`
	City           string         `json:"city" db:"city"`
	Province       string         `json:"province" db:"province"`
	Country        string         `json:"country" db:"country"`
	Headimgurl     string         `json:"headimgurl" db:"headimgurl"`
	SubscribeTime  string         `json:"subscribe_time" db:"subscribe_time"`
	Unionid        string         `json:"unionid" db:"unionid"`
	Remark         string         `json:"remark" db:"remark"`
	Groupid        int            `json:"groupid" db:"groupid"`
	TagidList      pq.StringArray `json:"tagid_list" db:"tagid_list"`
	SubscribeScene string         `json:"subscribe_scene" db:"subscribe_scene"`
	QRscene        int            `json:"qr_scene" db:"qr_scene"`
	QRsceneStr     string         `json:"qr_scene_str" db:"qr_scene_str"`
	Password       string         `json:"password" db:"password"`
	LoginErrTimes  int            `json:"login_err_times" db:"login_err_times"`
	LastLoginTime  string         `json:"last_login_time" db:"last_login_time"`
	LastLoginIP    string         `json:"last_login_ip" db:"last_login_ip"`
	SessionKey     string         `json:"session_key" db:"session_key"`
	CRMToken       string         `json:"crm_token" `
	Status         int            `json:"status" db:"status"`
	ErrCode        int            `json:"errcode"`
	ErrMsg         string         `json:"errmsg"`
}

type phonecode struct {
	PhoneCode     string `json:"phone_code" db:"phone_code"`
	Phone         string `json:"phone" db:"phone"`
	Name          string `json:"name" db:"name"`
	ID            int    `json:"id" db:"id"`
	Schname       string `json:"schname" db:"schname"`
	SchoolID      int    `json:"school_id" db:"school_id"`
	SchoolName    string `json:"school_name" db:"school_name"`
	ISHAM         bool   `json:"is_HAM" db:"is_HAM"`
	MasterSchname string `json:"master_schname" db:"master_schname"`
}

func (user *wxUserInfo) String() string {
	return fmt.Sprintf("%v %v %v %v %v %v %v %v %v %v %v %v %v %v %v %v %v %v %v %v %v %v ",
		user.Phone, user.Schname, user.SchoolName, user.SchList,
		user.Subscribe, user.OpenID, user.Name, user.NickName, user.Sex,
		user.Language, user.City, user.Province, user.Country, user.Headimgurl,
		user.SubscribeTime, user.Unionid, user.Remark, user.Groupid, user.TagidList,
		user.SubscribeScene, user.QRscene, user.QRsceneStr)
}

func checkUser(openid string) bool {

	var total int

	_, err := db.Exec(`SELECT count(*) as total from wxuser where openid=?  `, openid)
	if err != nil {
		log.Println("query user isexites err:", err, total, openid)
	}

	if total == 0 {
		return false
	}
	return true
}

func BindWeiXin(pc *phonecode) (*wxUserInfo, error) {

	openid, err := getOpenID(pc.PhoneCode)
	if err != nil {
		//log.Println("query openid err:", master.SchName, pc.Schname)
		return nil, err
	}

	r := &wxUserInfo{}

	_, err = db.Exec(`SELECT * from wxuser where openid=?  `, openid)
	if err != nil {
		//log.Println("query openid err:", master.SchName, pc.Schname)
		return nil, err
	}

	//sch := pc.Phone + "," + pc.Name + "," + pc.ID + "," + pc.Schname + "," + pc.SchoolName + "," + pc.ServerURL

	_, err = db.Exec(`UPDATE wxuser set phone=?,
	name=? 
	where openid=? `, pc.Phone, pc.Name, openid)

	if err != nil {
		log.Println("绑定失败：", err)
		return nil, err
	}

	sql2 := `UPDATE public.client set phone=?,name=? ,unionid=?,update_time=now() where openid=?`

	_, _ = db.Exec(sql2, pc.Phone, pc.Name, openid)

	return r, nil

}

func unsetphonecode(userid int) error {

	//fmt.Println("unbind HAMid:", HAMid, schname)

	_, err := db.Exec(`UPDATE users set openid='' where id=?  `, userid)

	if err != nil {
		log.Println("学员取消绑定失败:", err)
		return err
	}

	return nil

}

func getUserInfoByOpenid(openid string) (*wxUserInfo, error) {

	r := &wxUserInfo{}

	_, err := db.Exec(`SELECT * from wxuser where openid=?  `, openid)
	if err != nil {
		log.Println("query userinfo by  openid err:", err, openid)
		return r, err
	}

	return r, nil

}

type wxclient struct {
	ID            int    `db:"id" json:"id"`
	Phone         string `db:"phone" json:"phone"`
	Name          string `db:"name" json:"name"`
	Schname       string `db:"schname" json:"schname"`
	MasterSchname string `db:"master_schname" json:"master_schname"`
	MPopenID      string `db:"mpopenid" json:"mpopenid"`
	OpenID        string `db:"openid" json:"openid"`
	UnionID       string `db:"unionid" json:"unionid"`
	CreateTime    string `db:"create_time" json:"create_time"`
}

func getUserInfoByOpenidForBind(openid string) (*wxUserInfo, error) {

	c := &wxclient{}

	_, err := db.Exec(`SELECT * from public.client where openid=?  `, openid)
	if err != nil {
		log.Println("query client by  unionid err:", err)
		return nil, err
	}

	r := &wxUserInfo{}

	_, err = db.Exec(`SELECT * from  wxuser where unionid=?  `, c.UnionID)
	if err != nil {
		log.Println("query userinfo by  unionid err:", err)
		return r, err
	}

	return r, nil

}

func getUserInfoByUnionid(unionid string) (*wxUserInfo, error) {

	c := &wxclient{}

	query := `SELECT id,name,master_schname,unionid,mpopenid,openid,schname,phone from public.client where unionid=? `

	_, err := db.Exec(query, unionid)
	if err != nil {
		log.Println("query client by  unionid err:", err, "\n", query)
		return nil, err
	}

	r := &wxUserInfo{}

	_, err = db.Exec(`SELECT * from  wxuser where unionid=?  `, c.UnionID)
	if err != nil {
		log.Println("query userinfo by  unionid err:", err)
		return r, err
	}

	return r, nil

}

func getMPUserInfoByOpenID(openid string) (*wxUserInfo, error) {

	c := &wxclient{}

	query := `SELECT id,name,master_schname,unionid,mpopenid,openid,schname,phone from public.client where mpopenid=? `

	_, err := db.Exec(query, openid)
	if err != nil {
		log.Println("query client by  unionid err:", err)
		return nil, err
	}

	r := &wxUserInfo{}

	_, err = db.Exec(`SELECT * from wxuser where mpopenid=?  `, c.MPopenID)
	if err != nil {
		log.Println("query mp userinfo by  openid err:", err)
		return r, err
	}

	return r, nil

}

func unsubscribe(openid string) error {

	_, err := db.Exec(`DELETE from  wxuser  where openid=? `, openid)

	if err != nil {
		log.Println("取消关注失败: ", openid, err)
		return err
	}
	return nil

}

func addwxuser(user *wxUserInfo) error {

	if s, err := strconv.ParseInt(user.SubscribeTime, 10, 64); err == nil {
		user.SubscribeTime = time.Unix(s, 0).Format("2006-01-02 15:04:05")
	} else {
		log.Println("添加微信用户失败: ", err, user.SubscribeTime)
		user.SubscribeTime = time.Now().Format("2006-01-02 15:04:05")
	}

	//log.Println("wx subscribe user info:", user)

	_, err := db.Exec(`INSERT INTO wxuser
	(subscribe,openid,name,nickname,sex,language,city,province,country,headimgurl,
		subscribe_time,unionid,remark,groupid,tagid_list,subscribe_scene,qr_scene,qr_scene_str)
	VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?  )`,
		user.Subscribe, user.OpenID, user.Name, user.NickName, user.Sex, user.Language, user.City, user.Province, user.Country, user.Headimgurl,
		user.SubscribeTime, user.Unionid, user.Remark, user.Groupid, user.TagidList, user.SubscribeScene, user.QRscene, user.QRsceneStr)

	if err != nil {
		//log.Println("添加微信用户失败: ", err)
		return err
	}

	sql2 := `INSERT into public.client (master_schname,openid,unionid,create_time,update_time) 
	values(?,?,?,now(),now())`

	_, err = db.Exec(sql2, user.OpenID, user.Unionid)
	if err != nil {
		log.Println("绑定失败：", err, sql2)
		return err
	}

	return nil

}

// CheckSessionKey 检查小程序登录状态
func CheckSessionKey() bool {
	return true
}

func TeacherMPuserLogin(resp *wxjscode) (*wxUserInfo, error) {

	user, err := getMPUserInfoByOpenID(resp.OpenID)

	if err != nil {
		return nil, fmt.Errorf("请先关注 公众号，并绑定账号后，再登陆小程序")

	}

	return user, nil

}

func HAMMPuserLogin(resp *wxjscode) (*wxUserInfo, error) {

	user, err := getUserInfoByUnionid(resp.Unionid)

	if err != nil {
		return nil, fmt.Errorf("请先关注 公众号，并联系绑定账号后，再登陆小程序")

	}

	_, err = db.Exec(`UPDATE  wxuser set last_login_time=now(),session_key=? where mpopenid =?`, resp.SessionKey, resp.OpenID)

	if err != nil {
		log.Println("update wxuser err", err)
		return nil, fmt.Errorf("%v", "出错啦，请联系!")

	}

	return user, nil

}
