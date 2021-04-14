package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

// var CanSpeekerDev = &connPoll{}
// var globelConnPoll = make(map[string]connPoll, 100)

func (j *jsonapi) httpServersList(w http.ResponseWriter, req *http.Request) {
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
		log.Println("device list err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"查询设备表参数错误"}}`))
		return
	}

	rescode, _ := jsonextra.Marshal(ServersMap)

	respone := fmt.Sprintf(`{"code":20000,"data":{"total":%v,"items":%s}}`,
		len(ServersMap), rescode)

	w.Write([]byte(respone))

}

func (j *jsonapi) httpUpdateServer(w http.ResponseWriter, req *http.Request) {
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

	stb := &Server{}
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
	updateServer(stb)

	addOperatorLog(stb.String(), "修改公共群组信息成功", u)

	w.Write([]byte(`{"code":20000,"data":{"message":"公共群组更新成功"}}`))

}

func (j *jsonapi) httpAddServer(w http.ResponseWriter, req *http.Request) {
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

	stb := &Server{}
	err := jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("user add err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"新增群组失败,json格式错误"}}`))
		return
	}

	stb.OwerID = u.ID
	stb.OwerCallsign = u.CallSign

	if addServers(stb) != nil {

		w.Write([]byte(`{"code":20000,"data":{"isok":1,"message":"新增公共群组失败"}}`))
		return

	}

	addOperatorLog(stb.String(), "新增服务器成功", u)

	w.Write([]byte(`{"code":20000,"data":{"isok":0,"message":"新增公共群组成功"}}`))

}

func (j *jsonapi) httpDeleteServer(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, ok := checktoken(w, req)
	if !ok {
		return
	}

	if !checkrole(u, []string{"admin"}) {
		w.Write([]byte(`{"code":20000,"data":{"isok":1,"message":"当前用户没有权限设置此参数"}}`))
		return

	}

	result, _ := ioutil.ReadAll(req.Body)

	req.Body.Close()

	stb := &Server{}
	err := jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("user delete err :", err)
		w.Write([]byte(`{"code":20000,"data":{"isok":1,"message":"公共群组删除失败"}}`))
		return
	}

	deleteServer(stb)
	addOperatorLog(stb.String(), "服务器删除成功", u)

	w.Write([]byte(`{"code":20000,"data":{"isok":0,"message":"员工删除成功成功"}}`))

}
