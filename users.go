package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

//var operatermap = make(map[string]operater)

//var tokenidmap = make(map[string]operater, 0)

// type token struct {
// 	AccessToken string `json:"access_token"`
// 	ExpiresIn   int    `json:"expires_in"`
// 	Scope       string `json:"scope"`
// }

type loginreq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// type tokensignature struct{}

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

	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	if !checkrole(u, []string{"admin", "ham"}) {
		w.Write([]byte(`{"code":20000,"data":{"message":"当前用户没有权限设置此参数"}}`))
		return

	}
	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &query{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("user list err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"查询所有用户列表错误"}}`))
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

	_, err := checktoken(w, req)
	if err != nil {
		return
	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &query{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("user list err :", err)
		w.Write(ResParmErr)
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

	_, err := checktoken(w, req)
	if err != nil {
		return
	}

	req.ParseForm()
	role := strings.TrimSpace(req.Form.Get("role"))

	if role == "" {
		w.Write([]byte(`{"code":20000,"data":{"message":"根据角色查询员工错误"}}`))
		return
	}
	//result, _ := io.ReadAll(req.Body)

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

	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	rescode, _ := jsonextra.Marshal(u)

	respone := fmt.Sprintf(`{"code":20000,"data":%s}`, rescode)

	w.Write([]byte(respone))

}

func (j *jsonapi) httpGetMDCID(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	req.ParseForm()
	mdcid := strings.TrimSpace(req.Form.Get("mdcid"))

	if mdcid == "" {
		w.Write([]byte(`{"code":20000,"data":{"message":"MDC ID不能为空"}}`))
		return
	}

	if callsign, exists := mdcidmap[mdcid]; exists {
		w.Write([]byte(fmt.Sprintf(`{"code":20000,"data":{"callsign":"%s"}}`, callsign)))
	} else {
		w.Write([]byte(`{"code":20000,"data":{"message":"未找到对应的呼号"}}`))
	}
}

func (j *jsonapi) httpGetDMRID(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	req.ParseForm()
	dmrid := strings.TrimSpace(req.Form.Get("dmrid"))

	if dmrid == "" {
		w.Write([]byte(`{"code":20000,"data":{"message":"DMR ID不能为空"}}`))
		return
	}

	if callsign, exists := dmridmap[dmrid]; exists {
		w.Write([]byte(fmt.Sprintf(`{"code":20000,"data":{"callsign":"%s"}}`, callsign)))
	} else {
		w.Write([]byte(`{"code":20000,"data":{"message":"未找到对应的呼号"}}`))
	}

}

func (j *jsonapi) httpUpdateUser(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	if !checkrole(u, []string{"master", "admin"}) {
		w.Write([]byte(`{"code":20000,"data":{"message":"当前用户没有权限设置此参数"}}`))
		return

	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &userinfo{}
	err = jsonextra.Unmarshal(result, &stb)

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

func (j *jsonapi) httpUpdateUserAvatar(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &userinfo{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("update user  err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"账号操作失败"}}`))
		return
	}

	stb.CallSign = u.CallSign

	// if checkrole(stb, []string{"admin"}) {
	// 	w.Write([]byte("{"code":20000,"data":{"message":"内置账号，无法修改"}}"))
	// 	return
	// }

	//stb.Area = u.Area
	updateUseravatar(stb)

	addOperatorLog(stb.String(), "修改用户头像成功", u)

	w.Write([]byte(`{"code":20000,"data":{"message":"头像更新成功"}}`))

}

func (j *jsonapi) httpUpdateUserPassword(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &userinfo{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("update user  err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"账号操作失败"}}`))
		return
	}

	if u.ID != stb.ID {
		log.Println("update user  err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"无权限更新密码"}}`))
		return

	}

	// if checkrole(stb, []string{"admin"}) {
	// 	w.Write([]byte("{"code":20000,"data":{"message":"内置账号，无法修改"}}"))
	// 	return
	// }

	//stb.Area = u.Area
	err = updateUserPassword(stb.ID, stb.Password)

	if err != nil {
		log.Println("update user password err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"密码修改失败"}}`))
		return
	}

	addOperatorLog(stb.String(), "修改密码成功", u)

	w.Write([]byte(`{"code":20000,"data":{"message":"密码更新成功"}}`))

}

func (j *jsonapi) httpAddUser(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	if !checkrole(u, []string{"admin"}) {
		w.Write([]byte(`{"code":20000,"data":{"message":"当前用户没有权限设置此参数"}}`))
		return

	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &userinfo{}
	err = jsonextra.Unmarshal(result, &stb)

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

	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	if !checkrole(u, []string{"admin"}) {
		w.Write([]byte(`{"code":20000,"data":{"isok":1,"message":"当前用户没有权限设置此参数"}}`))
		return

	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &userinfo{}
	err = jsonextra.Unmarshal(result, &stb)

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
	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	if !checkrole(u, []string{"superadmin", "master", "admin", "view", "xiaozhang"}) {
		w.Write([]byte(`{"code":20000,"data":{"isok":1,"message":"当前用户没有权限设置此参数"}}`))
		return

	}

	query := " where name_key != 'admin' "

	if checkrole(u, []string{"admin"}) {
		query = ""
	}

	type responseinfo struct {
		Code int     `json:"code"`
		Data []*role `json:"data"`
	}

	r := responseinfo{Code: 20000, Data: getRoles(query)}
	rescode, _ := jsonextra.Marshal(r)
	w.Write(rescode)

}

func (j *jsonapi) httpRole(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	if !checkrole(u, []string{"admin"}) {
		w.Write([]byte(`{"code":20000,"data":{"isok":1,"message":"当前用户没有权限设置此参数"}}`))
		return

	}

	req.ParseForm()

	key := strings.TrimSpace(req.Form.Get("key"))

	result, _ := io.ReadAll(req.Body)

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

// func gentokenid(username string, roles []string) string {

// 	tokenHead := &tokenhead{Alg: "HS256"}
// 	tokenPayload := &tokenpayload{
// 		Iss:   "nrllink",
// 		Exp:   time.Now().Add(24 * 365 * time.Hour).Format("20060102"),
// 		Name:  username,
// 		Roles: roles,
// 	}

// 	head, _ := jsonextra.Marshal(tokenHead)
// 	base64head := base64.StdEncoding.EncodeToString(head)

// 	payload, _ := jsonextra.Marshal(tokenPayload)
// 	base64payload := base64.StdEncoding.EncodeToString(payload)

// 	key := []byte(conf.Web.TokenKey)
// 	h := hmac.New(sha256.New, key)
// 	h.Write([]byte(base64payload))
// 	sign := base64.StdEncoding.EncodeToString(h.Sum(nil))

// 	return base64head + "." + base64payload + "." + sign

// }

func (j *jsonapi) httpUserLogin(w http.ResponseWriter, req *http.Request) {

	sethttphead(w)

	result, _ := io.ReadAll(req.Body)
	//fmt.Println("adminlogin result:", string(result))
	req.Body.Close()
	stb := &loginreq{}
	err := jsonextra.Unmarshal(result, &stb)
	//fmt.Println("adminlogin username and password:", stb.Username, stb.Password)
	if err != nil {
		log.Println("login data decode:", req.Header.Get("X-Forwarded-For")+","+req.RemoteAddr, err)
		addOperatorLog("登录数据解码错误 "+req.Header.Get("X-Forwarded-For")+","+req.RemoteAddr, "登录错误", &userinfo{})
		w.Write([]byte("login data error "))
		return
	}

	if v, ok := loginCheck(stb.Password, stb.Username, req.RemoteAddr); ok {
		s, err := GenerateToken(stb.Username, v)
		if err != nil {
			log.Println("token generate err:", err)
			w.Write(ResTokenErr)
			return
		}

		res := &tokenrescode{Code: 20000,
			Data:    resdata{Token: s},
			Message: "login ok"}
		rescode, _ := jsonextra.Marshal(res)
		w.Write(rescode)

		addOperatorLog(stb.Username+" "+req.Header.Get("X-Forwarded-For")+","+req.RemoteAddr, "登录成功", &userinfo{})

		log.Println(req.Header.Get("X-Forwarded-For") + "," + req.RemoteAddr + " User login ok :username:" + stb.Username)
		return

	}

	res := &tokenrescode{Code: 60204, Message: "用户名或者密码错误"}
	rescode, _ := jsonextra.Marshal(res)
	w.Write(rescode)
	addOperatorLog("用户名或者密码错误 "+stb.Username+" "+req.Header.Get("X-Forwarded-For")+","+req.RemoteAddr, "登录失败", &userinfo{})

	log.Println(req.Header.Get("X-Forwarded-For") + "," + req.RemoteAddr + " User login err :username:" + stb.Username)

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

	u, err := checktoken(w, req)
	if err != nil {
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

	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	result, _ := io.ReadAll(req.Body)
	//fmt.Println("adminlogin result:", string(result))
	req.Body.Close()

	stb := &query{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("logout  err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"退出登录错误"}}`))
		return
	}

	if stb.SSID == 100 {
		offlineDevice(getCallsignSSID(u.CallSign, 100))
	}

	log.Println("logout:", result, string(result))

	//	if _, ok := checkcookie(req); ok {
	//req.ParseForm()
	//t := strings.TrimSpace(req.Form.Get("token"))
	res := &logout{Code: 20000, Data: "success"}
	rescode, _ := jsonextra.Marshal(res)

	w.Write([]byte(rescode))

	//	}

}

func checktoken(w http.ResponseWriter, req *http.Request) (*userinfo, error) {
	// 验证令牌，如果验证失败，向客户端写入错误响应并返回错误信息
	token, err := ValidateToken(req.Header.Get("x-token"))
	if err != nil {
		w.Write(ResTokenErr)
		return nil, fmt.Errorf("令牌错误，登录超时，请重新登录")
	}

	// 根据令牌中的用户名获取用户信息，如果获取失败，向客户端写入错误响应并返回错误信息
	emp, err := getuser(token.Username)
	if err != nil {
		w.Write(ResAccountErr)
		return nil, err
	}

	// 检查客户启用状态，如果员工被禁用，向客户端写入错误响应并返回错误信息
	if emp.Status != 1 {
		w.Write(ResAccountErr)
		return nil, fmt.Errorf("账号已禁用")
	}

	// 所有检查通过，返回员工信息指针和nil错误
	return emp, nil
}
