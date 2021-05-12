package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

// var CanSpeekerDev = &connPoll{}
// var globelConnPoll = make(map[string]connPoll, 100)

func (j *jsonapi) httpGetRelayList(w http.ResponseWriter, req *http.Request) {
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
		log.Println("query freq list  err :", err)
		w.Write([]byte(`{"code":20000,"data":{"total":0,"message":"频点列表查询参数错误"}}`))
		return
	}

	where, page, sort := queryToWhere("", *stb)
	list, total := selectrelay(where, sort, page)

	rescode, _ := jsonextra.Marshal(list)

	respone := fmt.Sprintf(`{"code":20000,"data":{"total":%v,"items":%s}}`, total, rescode)

	w.Write([]byte(respone))

}

func (j *jsonapi) httpUpdaterelay(w http.ResponseWriter, req *http.Request) {
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

	stb := &relay{}
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
	updaterelay(stb)

	addOperatorLog(stb.String(), "修改频点成功", u)

	w.Write([]byte(`{"code":20000,"data":{"message":"修改频点成功"}}`))

}

func (j *jsonapi) httpAddrelay(w http.ResponseWriter, req *http.Request) {
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

	stb := &relay{}
	err := jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("user add err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"新增群组失败,json格式错误"}}`))
		return
	}

	stb.OwerCallsign = u.CallSign

	if addrelay(stb) != nil {

		w.Write([]byte(`{"code":20000,"data":{"isok":1,"message":"新增频点失败"}}`))
		return

	}

	addOperatorLog(stb.String(), "新增频点成功", u)

	w.Write([]byte(`{"code":20000,"data":{"isok":0,"message":"新增频点成功"}}`))

}

func (j *jsonapi) httpDeleterelay(w http.ResponseWriter, req *http.Request) {
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

	stb := &relay{}
	err := jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("freq delete err :", err)
		w.Write([]byte(`{"code":20000,"data":{"isok":1,"message":"频点删除失败"}}`))
		return
	}

	deleterelay(stb)
	addOperatorLog(stb.String(), "频点删除成功", u)

	w.Write([]byte(`{"code":20000,"data":{"isok":0,"message":"频点删除成功"}}`))

}
