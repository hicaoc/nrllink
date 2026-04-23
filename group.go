package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
)

// var CanSpeekerDev = &connPoll{}
// var globelConnPoll = make(map[string]connPoll, 100)

type groupDetail struct {
	*group
	DevMap []*deviceInfo `json:"devmap"`
}

func newGroupDetail(g *group) groupDetail {
	devlist := make([]*deviceInfo, 0, len(g.devMap))
	for _, dev := range g.devMap {
		devlist = append(devlist, dev)
	}

	sort.Slice(devlist, func(i, j int) bool {
		return devlist[i].ID < devlist[j].ID
	})

	return groupDetail{
		group:  g,
		DevMap: devlist,
	}
}

func (j *jsonapi) httpPublicGroupList(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &query{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("device list err :", err)
		w.Write(ResParmErr)
		return
	}

	groupmap := make(map[int]*group)

	for k, v := range publicGroupMap {

		groupmap[k] = v
	}

	if user, okok := userlist.Load(u.CallSign); okok {
		groupmap[1] = user.(*userinfo).Groups[1]
		groupmap[2] = user.(*userinfo).Groups[2]
		groupmap[3] = user.(*userinfo).Groups[3]

	} else {
		log.Println("user not found")
	}

	rescode, _ := jsonextra.Marshal(groupmap)

	respone := fmt.Sprintf(`{"code":20000,"data":{"total":%v,"items":%s}}`,
		len(groupmap), rescode)

	w.Write([]byte(respone))

}

func (j *jsonapi) httpAllGroupListNRL(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	_, err := checktoken(w, req)
	if err != nil {
		return
	}

	str := ""

	for _, v := range publicGroupMap {
		str = str + fmt.Sprintf("%v,%v\n", v.ID, v.Name)

	}

	w.Write([]byte(str))

}

func (j *jsonapi) httpGetGroup(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &query{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil || stb.GroupID == "" {
		log.Println("device list err :", err)
		w.Write(ResParmErr)
		return
	}

	groupid, err := strconv.Atoi(stb.GroupID)
	if err != nil {
		log.Println("device list err :", err)
		w.Write(ResParmErr)
		return
	}

	if groupid <= 3 && groupid > 0 {
		if user, okok := userlist.Load(u.CallSign); okok {
			gp := user.(*userinfo).Groups[groupid]
			writeJSONResponseItem(w, newGroupDetail(gp))
			return
		}

	} else if g, ok := publicGroupMap[groupid]; ok {
		writeJSONResponseItem(w, newGroupDetail(g))
		return

	}
	w.Write(ResOpErr)

}

func (j *jsonapi) httpGetGroupList(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &query{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("device list err :", err)
		w.Write(ResParmErr)
		return
	}

	grouplist := getMiniGroupList(u)

	writeJSONResponseItem(w, grouplist)

}

func (j *jsonapi) httpUpdateGroup(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	if !checkrole(u, []string{"admin"}) {
		w.Write(ResRightErr)
		return

	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &group{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("update user  err :", err)
		w.Write(ResParmErr)
		return
	}

	// if checkrole(stb, []string{"admin"}) {
	// 	w.Write([]byte("{"code":20000,"data":{"message":"内置账号，无法修改"}}"))
	// 	return
	// }

	//stb.Area = u.Area
	updatePublicGroup(stb)

	addOperatorLog(stb.String(), "修改公共群组信息成功", u)

	w.Write(ResOK)

}

func (j *jsonapi) httpAddGroup(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	if !checkrole(u, []string{"admin"}) {
		w.Write(ResRightErr)
		return

	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &group{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("user add err :", err)
		w.Write(ResParmErr)
		return
	}

	stb.OwerID = u.ID
	stb.OwerCallsign = u.CallSign

	if addPublicGroup(stb) != nil {

		w.Write(ResOpErr)
		return

	}

	addOperatorLog(stb.String(), "新增公共群组成功", u)

	w.Write(ResOK)

}

func (j *jsonapi) httpDeleteGroup(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	if !checkrole(u, []string{"admin"}) {
		w.Write(ResRightErr)
		return

	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &group{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("user delete err :", err)
		w.Write(ResParmErr)
		return
	}

	deletePublicGroup(stb)
	addOperatorLog(stb.String(), "公共群组删除成功", u)

	w.Write(ResOK)

}
