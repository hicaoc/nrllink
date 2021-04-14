package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

//var operatermap = make(map[string]operater)

//var tokenidmap = make(map[string]operater, 0)

type token struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

type loginreq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type tokenhead struct {
	Alg string `json:"alg"`
}
type tokenpayload struct {
	Iss string `json:"iss"`
	//Sub string  `json:"sub"`
	//Aud string `json:"aud"`
	//Nbf string `json:"nbf"`
	//Jat string `json:"jat"`
	//Jti string `json:"jti"`
	Exp   string   `json:"exp"`
	Name  string   `json:"name"`
	Roles []string `json:"roles"`
}
type tokensignature struct{}

type tokenrescode struct {
	Code    int     `json:"code"`
	Data    resdata `json:"data"`
	Message string  `json:"message"`
}

type resdata struct {
	Token string `json:"token"`
}

func (j *jsonapi) httpUserAllList(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, ok := checktoken(w, req)
	if !ok {
		return
	}

	if !checkrole(u, []string{"admin", "superadmin"}) {
		w.Write([]byte(`{"code":20000,"data":{"message":"当前用户没有权限设置此参数"}}`))
		return

	}
	result, _ := ioutil.ReadAll(req.Body)

	req.Body.Close()

	stb := &query{}
	err := jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("user list err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"查询所有员工列表参数错误"}}`))
		return
	}

	// oplist := []operater{}
	// for _, v := range operatermap {
	// 	oplist = append(oplist, v)
	// }
	var emplist []userinfo
	var total int

	emplist, total = selectuser(queryToWhere("", *stb))

	rescode, _ := jsonextra.Marshal(emplist)

	respone := fmt.Sprintf(`{"code":20000,"data":{"total":%v,"items":%s}}`,
		total, rescode)

	w.Write([]byte(respone))

}

func (j *jsonapi) httpUserList(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	_, ok := checktoken(w, req)
	if !ok {
		return
	}

	result, _ := ioutil.ReadAll(req.Body)

	req.Body.Close()

	stb := &query{}
	err := jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("user list err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"查询员工列表参数错误"}}`))
		return
	}

	// oplist := []operater{}
	// for _, v := range operatermap {
	// 	oplist = append(oplist, v)
	// }
	//fmt.Println(u.CurrentArea)
	//stb.CurrentArea = strconv.Itoa(u.CurrentArea)
	//员工漫游修改位常驻

	emplist, total := selectuser(queryToWhere("", *stb))

	//emplist = selectuser()

	rescode, _ := jsonextra.Marshal(emplist)

	respone := fmt.Sprintf(`{"code":20000,"data":{"total":%v,"items":%s}}`,
		total, rescode)

	w.Write([]byte(respone))

}

func (j *jsonapi) httpUserListbyRole(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	_, ok := checktoken(w, req)
	if !ok {
		return
	}

	req.ParseForm()
	role := strings.TrimSpace(req.Form.Get("role"))

	if role == "" {
		w.Write([]byte(`{"code":20000,"data":{"message":"根据角色查询员工错误"}}`))
		return
	}
	//result, _ := ioutil.ReadAll(req.Body)

	// req.Body.Close()

	// stb := &query{}
	// err := jsonextra.Unmarshal(result, &stb)

	// if err != nil {
	// 	log.Println("teacher list err :", err)
	// 	w.Write([]byte("{"code":20000,"data":{"message":"查询教师列表参数错误"}}"))
	// 	return
	// }

	// oplist := []operater{}
	// for _, v := range operatermap {
	// 	oplist = append(oplist, v)
	// }
	//fmt.Println(u.CurrentArea)
	//stb.CurrentArea = strconv.Itoa(u.CurrentArea)

	emplist, total := getEmpListByRole(role)

	//emplist = selectuser()

	rescode, _ := jsonextra.Marshal(emplist)

	respone := fmt.Sprintf(`{"code":20000,"data":{"total":%v,"items":%s}}`, total, rescode)

	w.Write([]byte(respone))

}

func (j *jsonapi) httpUserDetail(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, ok := checktoken(w, req)
	if !ok {
		return
	}

	rescode, _ := jsonextra.Marshal(u)

	respone := fmt.Sprintf(`{"code":20000,"data":%s}`, rescode)

	w.Write([]byte(respone))

}

func (j *jsonapi) httpUpdateUser(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, ok := checktoken(w, req)
	if !ok {
		return
	}

	if !checkrole(u, []string{"master", "admin"}) {
		w.Write([]byte(`{"code":20000,"data":{"message":"当前用户没有权限设置此参数"}}`))
		return

	}

	result, _ := ioutil.ReadAll(req.Body)

	req.Body.Close()

	stb := &userinfo{}
	err := jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("update user  err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"账号操作失败"}}`))
		return
	}

	// if checkrole(stb, []string{"admin"}) {
	// 	w.Write([]byte("{"code":20000,"data":{"message":"内置账号，无法修改"}}"))
	// 	return
	// }

	//stb.Area = u.Area
	updateUser(stb)

	addOperatorLog(stb.String(), "修改用户信息成功", u)

	w.Write([]byte(`{"code":20000,"data":{"message":"员工信息更新成功"}}`))

}

func (j *jsonapi) httpAddUser(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, ok := checktoken(w, req)
	if !ok {
		return
	}

	if !checkrole(u, []string{"admin"}) {
		w.Write([]byte(`{"code":20000,"data":{"message":"当前用户没有权限设置此参数"}}`))
		return

	}

	result, _ := ioutil.ReadAll(req.Body)

	req.Body.Close()

	stb := &userinfo{}
	err := jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("user add err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"新增用户失败"}}`))
		return
	}

	if addUser(stb) != nil {

		w.Write([]byte(`{"code":20000,"data":{"isok":1,"message":"新增用户失败，可能手机号码已经存在"}}`))
		return

	}

	addOperatorLog(stb.String(), "新增用户信息成功", u)

	w.Write([]byte(`{"code":20000,"data":{"isok":0,"message":"新增用户成功"}}`))

}

func (j *jsonapi) httpDeleteUser(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, ok := checktoken(w, req)
	if !ok {
		return
	}

	if !checkrole(u, []string{"admin", "master"}) {
		w.Write([]byte(`{"code":20000,"data":{"isok":1,"message":"当前用户没有权限设置此参数"}}`))
		return

	}

	result, _ := ioutil.ReadAll(req.Body)

	req.Body.Close()

	stb := &userinfo{}
	err := jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("user delete err :", err)
		w.Write([]byte(`{"code":20000,"data":{"isok":1,"message":"员工删除失败"}}`))
		return
	}

	if checkrole(stb, []string{"admin"}) {
		w.Write([]byte(`{"code":20000,"data":{"isok":1,"message":"内置账号无法删除"}}`))
		return

	}

	deleteUser(stb)
	addOperatorLog(stb.String(), "用户删除成功", u)

	w.Write([]byte(`{"code":20000,"data":{"isok":0,"message":"员工删除成功成功"}}`))

}

func (j *jsonapi) httpGetRoles(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, ok := checktoken(w, req)
	if !ok {
		return
	}

	if checkrole(u, []string{"superadmin", "master", "admin", "view", "xiaozhang"}) == false {
		w.Write([]byte(`{"code":20000,"data":{"isok":1,"message":"当前用户没有权限设置此参数"}}`))
		return

	}

	query := " where name_key != 'admin' "

	if checkrole(u, []string{"admin"}) {
		query = ""
	}

	type responseinfo struct {
		Code int    `json:"code"`
		Data []role `json:"data"`
	}

	r := responseinfo{Code: 20000, Data: getRoles(query)}
	rescode, _ := jsonextra.Marshal(r)
	w.Write(rescode)

}

func (j *jsonapi) httpRole(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, ok := checktoken(w, req)

	if !ok {
		return
	}

	if checkrole(u, []string{"admin"}) == false {
		w.Write([]byte(`{"code":20000,"data":{"isok":1,"message":"当前用户没有权限设置此参数"}}`))
		return

	}

	req.ParseForm()

	key := strings.TrimSpace(req.Form.Get("key"))

	result, _ := ioutil.ReadAll(req.Body)

	//	fmt.Println("role result:", req.Method, key, string(result))

	req.Body.Close()

	// fmt.Println("stb:", stb)

	switch req.Method {

	case "POST":
		stb := &role{}
		err := jsonextra.Unmarshal(result, &stb)
		if err != nil {
			log.Println("role add err :", err)
			w.Write([]byte(`{"code":20000,"data":{"isok":1,"message":"角色添加操作失败"}}`))
			return
		}
		//	stb.Key = stb.Name
		addRole(stb)

		addOperatorLog(stb.String(), "增加角色", u)

		type responseinfo struct {
			Code int  `json:"code"`
			Data role `json:"data"`
		}

		r := responseinfo{Code: 20000, Data: *stb}
		rescode, _ := jsonextra.Marshal(r)
		w.Write(rescode)

	case "PUT":
		stb := &role{}
		err := jsonextra.Unmarshal(result, &stb)
		if err != nil {
			log.Println("role update err :", err)
			w.Write([]byte(`{"code":20000,"data":{"isok":1,"message":"角色修改操作失败"}}`))
			return
		}
		updateRole(stb)
		addOperatorLog(stb.String(), "修改角色", u)
		w.Write([]byte(`{"code":20000,"data":{"isok":0,"message":"角色修改操作成功"}}`))

	case "DELETE":
		//fmt.Println("del role")
		delRole(key)
		addOperatorLog(key, "删除角色", u)
		w.Write([]byte(`{"code":20000,"data":{"isok":0,"message":"角色删除操作成功"}}`))

	default:
		log.Println("role not support req.method:", req.Method)

	}

	//selectuser()

	// type responseinfo struct {
	// 	Code int    `json:"code"`
	// 	Data []role `json:"data"`
	// }

	// r := responseinfo{Code: 20000, Data: getRoles()}
	// rescode, _ := jsonextra.Marshal(r)
	// w.Write(rescode)

}

func gentokenid(username string, roles []string) string {

	tokenHead := &tokenhead{Alg: "HS256"}
	tokenPayload := &tokenpayload{
		Iss:   "roland",
		Exp:   time.Now().Add(24 * time.Hour).Format("20060102"),
		Name:  username,
		Roles: roles,
	}

	head, _ := jsonextra.Marshal(tokenHead)
	base64head := base64.StdEncoding.EncodeToString(head)

	payload, _ := jsonextra.Marshal(tokenPayload)
	base64payload := base64.StdEncoding.EncodeToString(payload)

	key := []byte("rolandkey")
	h := hmac.New(sha256.New, key)
	h.Write([]byte(base64payload))
	sign := base64.StdEncoding.EncodeToString(h.Sum(nil))

	return base64head + "." + base64payload + "." + sign

}

func (j *jsonapi) httpUserLogin(w http.ResponseWriter, req *http.Request) {

	sethttphead(w)

	result, _ := ioutil.ReadAll(req.Body)
	//fmt.Println("adminlogin result:", string(result))
	req.Body.Close()
	stb := &loginreq{}
	err := jsonextra.Unmarshal(result, &stb)
	//fmt.Println("adminlogin username and password:", stb.Username, stb.Password)
	if err != nil {
		log.Println("login data decode:", req.RemoteAddr, err)
		addOperatorLog("登录数据解码错误 "+req.RemoteAddr, "登录错误", &userinfo{})
		w.Write([]byte("login data error "))
		return
	}

	if v, ok := loginCheck(stb.Password, stb.Username, req.RemoteAddr); ok {
		s := gentokenid(stb.Username, v)

		res := &tokenrescode{Code: 20000,
			Data:    resdata{Token: s},
			Message: "login ok"}
		rescode, _ := jsonextra.Marshal(res)
		w.Write(rescode)

		addOperatorLog(stb.Username+" "+req.RemoteAddr, "登录成功", &userinfo{})

		log.Println(req.RemoteAddr + " User login ok :username:" + stb.Username)
		return

	}

	res := &tokenrescode{Code: 60204, Message: "用户名或者密码错误"}
	rescode, _ := jsonextra.Marshal(res)
	w.Write(rescode)
	addOperatorLog("用户名或者密码错误 "+stb.Username+" "+req.RemoteAddr, "登录失败", &userinfo{})

	log.Println(req.RemoteAddr + " User login err :username:" + stb.Username + " password:" + stb.Password)

}

// func (j *jsonapi) httpRoutes(w http.ResponseWriter, req *http.Request) {
// 	sethttphead(w)

// 	u, ok := checktoken(w, req)
// 	if !ok {
// 		return
// 	}

// 	//selectuser()

// 	r := responseinfo{Code: 20000, Data: operatermap[u]}
// 	rescode, _ := jsonextra.Marshal(r)
// 	w.Write(rescode)

// }

func (j *jsonapi) httpUserInfo(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, ok := checktoken(w, req)
	if !ok {
		return
	}

	u.Routes = getRoleByKey(u.Roles[0]).Routes

	rescode, _ := jsonextra.Marshal(u)

	respone := fmt.Sprintf(`{"code":20000,"data":%s}`, rescode)

	w.Write([]byte(respone))

}

type logout struct {
	Code int    `json:"code"`
	Data string `json:"data"`
}

func (j *jsonapi) httpoplogout(w http.ResponseWriter, req *http.Request) {

	sethttphead(w)

	result, _ := ioutil.ReadAll(req.Body)
	//fmt.Println("adminlogin result:", string(result))
	req.Body.Close()

	log.Println("logout:", result)

	//	if _, ok := checkcookie(req); ok {
	//req.ParseForm()
	//t := strings.TrimSpace(req.Form.Get("token"))
	res := &logout{Code: 20000, Data: "success"}
	rescode, _ := jsonextra.Marshal(res)

	w.Write([]byte(rescode))

	//	}

}

func checktoken(w http.ResponseWriter, req *http.Request) (*userinfo, bool) {

	token := req.Header.Get("x-token")

	p := strings.Split(token, ".")

	if len(p) != 3 {

		//	log.Println("token err  len != 3", p)
		w.Write([]byte(`{"code":50008,"data":{"isok":1,"message":"token format err"}}`))
		return nil, false
	}

	key := []byte("rolandkey")
	h := hmac.New(sha256.New, key)
	h.Write([]byte(p[1]))
	sign := base64.StdEncoding.EncodeToString(h.Sum(nil))

	if strings.EqualFold(sign, p[2]) == false {
		log.Println("token err key not equal")
		w.Write([]byte(`{"code":50008,"data":{"isok":1,"message":"token sign err"}}`))
		return nil, false
	}

	jsonpayload, err := base64.StdEncoding.DecodeString(p[1])

	if err != nil {
		log.Println("token err decode base64 ")
		w.Write([]byte(`{"code":50008,"data":{"isok":1,"message":"token decode err"}}`))
		return nil, false
	}

	payload := &tokenpayload{}

	err = jsonextra.Unmarshal(jsonpayload, payload)

	if err != nil {
		log.Println("token err :decode payload")
		w.Write([]byte(`{"code":50008,"data":{"isok":1,"message":"token data decode err"}}`))
		return nil, false
	}

	if payload.Exp != time.Now().Add(24*time.Hour).Format("20060102") {
		log.Println("token err ,timeout exp ")
		w.Write([]byte(`{"code":50014,"data":{"isok":1,"message":"token exp err"}}`))
		return nil, false

	}

	return getuser(payload.Name), true

}
