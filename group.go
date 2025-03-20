package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
)

// var CanSpeekerDev = &connPoll{}
// var globelConnPoll = make(map[string]connPoll, 100)

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

	if err != nil {
		log.Println("device list err :", err)
		w.Write(ResParmErr)
		return
	}

	if stb.GroupID <= 3 && stb.GroupID > 0 {
		if user, okok := userlist.Load(u.CallSign); okok {
			gp := user.(*userinfo).Groups[stb.GroupID]
			writeJSONResponseItem(w, gp)
			return
		}

	} else if g, ok := publicGroupMap[stb.GroupID]; ok {
		writeJSONResponseItem(w, g)
		return

	}
	w.Write(ResOpErr)

}

type minigroup struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Type            int    `json:"type"`
	OnlineDevNumber int    `json:"online_dev_number"`
	TotalDevNumber  int    `json:"total_dev_number"`
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

	grouplist := make([]minigroup, 0)

	for _, v := range publicGroupMap {
		g := minigroup{
			ID:              v.ID,
			Name:            v.Name,
			Type:            v.Type,
			OnlineDevNumber: v.OnlineDevNumber,
			TotalDevNumber:  v.TotalDevNumber,
		}
		grouplist = append(grouplist, g)

	}

	if user, okok := userlist.Load(u.CallSign); okok {
		gp1 := user.(*userinfo).Groups[1]
		gp2 := user.(*userinfo).Groups[2]
		gp3 := user.(*userinfo).Groups[3]

		g1 := minigroup{
			ID:              1,
			Name:            gp1.Name,
			Type:            gp1.Type,
			OnlineDevNumber: gp1.OnlineDevNumber,
			TotalDevNumber:  gp1.TotalDevNumber,
		}

		g2 := minigroup{
			ID:              2,
			Name:            gp2.Name,
			Type:            gp2.Type,
			OnlineDevNumber: gp2.OnlineDevNumber,
			TotalDevNumber:  gp2.TotalDevNumber,
		}

		g3 := minigroup{
			ID:              3,
			Name:            gp3.Name,
			Type:            gp3.Type,
			OnlineDevNumber: gp3.OnlineDevNumber,
			TotalDevNumber:  gp3.TotalDevNumber,
		}

		grouplist = append(grouplist, g1)
		grouplist = append(grouplist, g2)
		grouplist = append(grouplist, g3)

	} else {
		log.Println("user not found")
	}

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
