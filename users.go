package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
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

	if !checkrole(u, []string{"admin"}) {
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

	for i := range emplist {
		emplist[i].desensitize()
	}

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

	for i := range emplist {
		emplist[i].desensitize()
	}

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

	for i := range emplist {
		emplist[i].desensitize()
		emplist[i].Password = ""
	}

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

	// 呼号反查需登录，防止未授权枚举；响应用 JSON 编码防止特殊字符破坏报文
	if _, err := checktoken(w, req); err != nil {
		return
	}

	req.ParseForm()
	mdcid := strings.TrimSpace(req.Form.Get("mdcid"))

	if mdcid == "" {
		w.Write([]byte(`{"code":20000,"data":{"message":"MDC ID不能为空"}}`))
		return
	}

	value, exists := mdcidmap.Load(mdcid)
	if exists {
		rescode, _ := jsonextra.Marshal(map[string]any{"code": 20000, "data": map[string]string{"callsign": value.(string)}})
		w.Write(rescode)
	} else {
		w.Write([]byte(`{"code":20000,"data":{"message":"未找到对应的呼号"}}`))
	}
}

func (j *jsonapi) httpGetDMRID(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	// 呼号反查需登录，防止未授权枚举；响应用 JSON 编码防止特殊字符破坏报文
	if _, err := checktoken(w, req); err != nil {
		return
	}

	req.ParseForm()
	dmrid := strings.TrimSpace(req.Form.Get("dmrid"))

	if dmrid == "" {
		w.Write([]byte(`{"code":20000,"data":{"message":"DMR ID不能为空"}}`))
		return
	}

	value, exists := dmridmap.Load(dmrid)
	if exists {
		rescode, _ := jsonextra.Marshal(map[string]any{"code": 20000, "data": map[string]string{"callsign": value.(string)}})
		w.Write(rescode)
	} else {
		w.Write([]byte(`{"code":20000,"data":{"message":"未找到对应的呼号"}}`))
	}

}

func isValidMDCID(mdcid string) bool {
	if len(mdcid) != 4 {
		return false
	}

	for _, r := range mdcid {
		if !((r >= '0' && r <= '9') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}

	return true
}

func isValidDMRID(dmrid string) bool {
	if dmrid == "" {
		return true
	}

	if _, err := strconv.ParseUint(dmrid, 10, 32); err != nil {
		return false
	}

	return true
}

func (j *jsonapi) httpUpdateUser(w http.ResponseWriter, req *http.Request) {
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

	if !checkrole(u, []string{"master", "admin"}) {
		w.Write([]byte(`{"code":20000,"data":{"message":"当前用户没有权限设置此参数"}}`))
		return
	}

	// master 只能修改普通账号的基础信息：不能变更角色/状态/呼号，也不能操作管理账号，
	// 防止 master 通过修改 roles 字段提权为 admin
	if !checkrole(u, []string{"admin"}) {
		target, err := getuserByID(stb.ID)
		if err != nil {
			w.Write([]byte(`{"code":20000,"data":{"message":"账号操作失败"}}`))
			return
		}
		if checkrole(target, []string{"admin", "master"}) ||
			strings.Join(stb.Roles, ",") != strings.Join(target.Roles, ",") ||
			stb.Status != target.Status ||
			!strings.EqualFold(stb.CallSign, target.CallSign) {
			w.Write([]byte(`{"code":20000,"data":{"message":"无权修改管理账号或变更角色/状态/呼号"}}`))
			return
		}
	}

	stb.MDCID = strings.ToUpper(strings.TrimSpace(stb.MDCID))
	if stb.MDCID != "" && !isValidMDCID(stb.MDCID) {
		w.Write([]byte(`{"code":20000,"data":{"message":"MDC ID 必须是 4 位十六进制字符串"}}`))
		return
	}

	stb.DMRID = strings.TrimSpace(stb.DMRID)
	if !isValidDMRID(stb.DMRID) {
		w.Write([]byte(`{"code":20000,"data":{"message":"DMR ID 必须是 0 到 4294967295 之间的整数"}}`))
		return
	}

	// 普通管理员之间不允许互改密码，只有内置 admin 账号（admin 角色）可以修改其他管理员的密码
	if stb.Password != "" && stb.ID != u.ID {
		if target, err := getuserByID(stb.ID); err == nil &&
			checkrole(target, []string{"admin"}) && !isBuiltinAdmin(u) {
			w.Write([]byte(`{"code":20000,"data":{"message":"只有 admin 账号可以修改其他管理员的密码"}}`))
			return
		}
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

func (j *jsonapi) httpUpdateUserProfile(w http.ResponseWriter, req *http.Request) {
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
		log.Println("update user profile err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"账号操作失败"}}`))
		return
	}

	if u.ID != stb.ID {
		w.Write([]byte(`{"code":20000,"data":{"message":"当前用户没有权限设置此参数"}}`))
		return
	}

	if u.OIDCVirtual {
		writeVirtualAccountErr(w)
		return
	}

	stb.MDCID = strings.ToUpper(strings.TrimSpace(stb.MDCID))
	if stb.MDCID != "" && !isValidMDCID(stb.MDCID) {
		w.Write([]byte(`{"code":20000,"data":{"message":"MDC ID 必须是 4 位十六进制字符串"}}`))
		return
	}

	stb.DMRID = strings.TrimSpace(stb.DMRID)
	if !isValidDMRID(stb.DMRID) {
		w.Write([]byte(`{"code":20000,"data":{"message":"DMR ID 必须是 0 到 4294967295 之间的整数"}}`))
		return
	}

	stb.Avatar = strings.TrimSpace(stb.Avatar)
	// 资料修改包含密码变更时，必须验证旧密码
	if stb.Password != "" {
		var opw struct {
			OldPassword string `json:"old_password"`
		}
		if err := jsonextra.Unmarshal(result, &opw); err != nil {
			w.Write(ResParmErr)
			return
		}
		if err := verifyUserPassword(u.ID, opw.OldPassword); err != nil {
			w.Write([]byte(`{"code":20000,"data":{"message":"旧密码错误"}}`))
			return
		}
	}

	if err := updateUserProfile(u.ID, stb.DMRID, stb.MDCID, stb.Avatar, stb.Password); err != nil {
		w.Write([]byte(`{"code":20000,"data":{"message":"个人信息更新失败"}}`))
		return
	}

	addOperatorLog(u.String(), "修改个人资料成功", u)
	w.Write([]byte(`{"code":20000,"data":{"message":"个人信息更新成功"}}`))

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

	// 只能修改本人头像，防止篡改请求体 id 越权修改他人（含管理员）头像
	if u.ID != stb.ID {
		w.Write([]byte(`{"code":20000,"data":{"message":"无权修改他人头像"}}`))
		return
	}

	if u.OIDCVirtual {
		writeVirtualAccountErr(w)
		return
	}

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

	if u.OIDCVirtual {
		writeVirtualAccountErr(w)
		return
	}

	// if checkrole(stb, []string{"admin"}) {
	// 	w.Write([]byte("{"code":20000,"data":{"message":"内置账号，无法修改"}}"))
	// 	return
	// }

	//stb.Area = u.Area
	// 修改密码必须验证旧密码，防止 token 泄露后被直接改密接管
	var opw struct {
		OldPassword string `json:"old_password"`
	}
	if err := jsonextra.Unmarshal(result, &opw); err != nil {
		w.Write(ResParmErr)
		return
	}
	if err := verifyUserPassword(stb.ID, opw.OldPassword); err != nil {
		log.Println("old password verify failed:", stb.ID, err)
		w.Write([]byte(`{"code":20000,"data":{"message":"旧密码错误"}}`))
		return
	}

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

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &userinfo{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("user delete err :", err)
		w.Write([]byte(`{"code":20000,"data":{"isok":1,"message":"员工删除失败"}}`))
		return
	}

	// 管理员可删除任意账号，普通用户仅允许注销本人账号
	selfDelete := u.ID == stb.ID
	if !selfDelete && !checkrole(u, []string{"admin"}) {
		w.Write([]byte(`{"code":20000,"data":{"isok":1,"message":"当前用户没有权限设置此参数"}}`))
		return

	}

	if selfDelete && u.OIDCVirtual {
		writeVirtualAccountErr(w)
		return
	}

	// 内置账号保护以数据库记录为准，防止通过篡改请求体 roles 字段绕过
	if target, err := getuserByID(stb.ID); err == nil && checkrole(target, []string{"admin"}) {
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

	// IP 登录保护：封锁期内只返回通用登录错误，不泄露封锁详情
	ip := clientIP(req)
	if loginGuard.blocked(ip) {
		log.Println("登录保护：封锁期内的IP尝试登录 " + ip + " username:" + stb.Username)
		res := &tokenrescode{Code: 60204, Message: "登录错误"}
		rescode, _ := jsonextra.Marshal(res)
		w.Write(rescode)
		return
	}

	if v, ok := loginCheck(stb.Password, stb.Username, req.RemoteAddr); ok {
		loginGuard.recordSuccess(ip)

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

	loginGuard.recordFailure(ip, stb.Username)

	res := &tokenrescode{Code: 60204, Message: "用户名或者密码错误"}
	rescode, _ := jsonextra.Marshal(res)
	w.Write(rescode)
	addOperatorLog("用户名或者密码错误 "+stb.Username+" "+req.Header.Get("X-Forwarded-For")+","+req.RemoteAddr, "登录失败", &userinfo{})

	log.Println(req.Header.Get("X-Forwarded-For") + "," + req.RemoteAddr + " User login err :username:" + stb.Username)

}

// httpUnlockUser 管理员解锁被登录保护锁定的账号，同时清零数据库中的失败计数
func (j *jsonapi) httpUnlockUser(w http.ResponseWriter, req *http.Request) {
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
	account := strings.ToUpper(strings.TrimSpace(req.Form.Get("account")))
	if account == "" {
		w.Write([]byte(`{"code":20000,"data":{"isok":1,"message":"账号不能为空"}}`))
		return
	}

	accountGuard.unlock(account)
	if _, err := db.Exec(`update users set login_err_times=0 where phone=? or callsign=?`, account, account); err != nil {
		log.Println("unlock user reset login_err_times failed:", err)
	}

	addOperatorLog(account, "管理员解锁账号", u)
	w.Write([]byte(`{"code":20000,"data":{"isok":0,"message":"账号已解锁"}}`))

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

	rescode, _ := jsonextra.Marshal(struct {
		*userinfo
		BillingEnabled bool `json:"billing_enabled"`
	}{u, billingEnabled()})

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

// writeVirtualAccountErr OIDC 临时用户没有 users.id，不能执行绑定本地账号的写操作。
// 返回 true 表示已写入错误响应。
func writeVirtualAccountErr(w http.ResponseWriter) bool {
	w.Write([]byte(`{"code":20001,"data":{"message":"OIDC 临时会话没有本地账号，此操作需要先创建本地账号"}}`))
	return true
}

func checktoken(w http.ResponseWriter, req *http.Request) (*userinfo, error) {
	// 统一支持本地 JWT 和 HAM ID 长期 API Token；验证失败时写入错误响应。
	token, err := userFromRawToken(req.Context(), tokenFromRequest(req))
	if err != nil {
		w.Write(ResTokenErr)
		return nil, fmt.Errorf("令牌错误，登录超时，请重新登录")
	}

	// 根据令牌中的用户名获取用户信息；OIDC virtual token 在本地无账号时还原临时用户。
	emp, err := userFromTokenClaims(token)
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

// checktokenSilent 与 checktoken 逻辑相同，但鉴权失败时不向客户端写错误响应，仅返回 nil。
// 用于 token 可选的公开接口（如 /group/get），调用方需自行处理 u 为 nil 的情况。
func checktokenSilent(req *http.Request) *userinfo {
	token, err := userFromRawToken(req.Context(), tokenFromRequest(req))
	if err != nil {
		return nil
	}

	emp, err := userFromTokenClaims(token)
	if err != nil || emp.Status != 1 {
		return nil
	}

	return emp
}

// userFromTokenClaims 先查本地账号；查不到且是 OIDC 临时会话时返回内存用户。
// 本地账号存在时永远优先，后续创建/绑定同名本地账号不需要刷新前端逻辑。
func userFromTokenClaims(token *Claims) (*userinfo, error) {
	emp, err := getuser(token.Username)
	if err == nil {
		return emp, nil
	}
	if token.OIDCVirtual && ((oidcEnabled() || longTokenEnabled()) && conf.OIDC.VirtualLogin) {
		return virtualOIDCUserFromClaims(token), nil
	}
	return nil, err
}
