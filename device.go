package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
)

func (j *jsonapi) httpDevicesList(w http.ResponseWriter, req *http.Request) {
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

	var emplist []deviceInfo

	total := 0
	if stb.IsOnline {
		emplist, total = getOnlineDevicelist(stb.Limit, stb.Page, stb.Callsign, stb.GroupID, stb.Sort)
	} else {
		emplist, total = getDevicelist(queryToWhere("", *stb))

	}

	//emplist = selectuser()

	rescode, _ := jsonextra.Marshal(emplist)

	respone := fmt.Sprintf(`{"code":20000,"data":{"total":%v,"items":%s}}`,
		total, rescode)

	w.Write([]byte(respone))

}

func (j *jsonapi) httpDeviceList(w http.ResponseWriter, req *http.Request) {
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
		w.Write([]byte(`{"code":20000,"data":{"message":"查询设备表参数错误"}}`))
		return
	}

	devicelist := make(map[int]deviceInfo, 10)

	isadmin := checkrole(u, []string{"admin"})

	var id int

	totalstats.OnlineDevNumber = 0

	for _, vv := range devCallsignSSIDMap {

		if vv.ISOnline {
			totalstats.OnlineDevNumber++
		}

		dev := *vv

		if !isadmin && dev.CallSign != u.CallSign {

			dev.DeviceParm = nil
		}

		devicelist[id] = dev
		id++

	}

	rescode, _ := jsonextra.Marshal(devicelist)

	respone := fmt.Sprintf(`{"code":20000,"data":{"total":%v,"items":%s}}`,
		len(devCallsignSSIDMap), rescode)

	w.Write([]byte(respone))

}

func (j *jsonapi) httpGroupDeviceList(w http.ResponseWriter, req *http.Request) {
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
		w.Write([]byte(`{"code":20000,"data":{"message":"查询群组设备列表参数错误"}}`))
		return
	}

	grolupid, err := strconv.Atoi(stb.GroupID)
	if err != nil {
		log.Println("device list err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"查询群组设备列表参数错误，群组ID错误"}}`))
		return

	}

	onlinedevlist := make([]*deviceInfo, 0)
	offlinedevlist := make([]*deviceInfo, 0)

	if grolupid < 1000 && grolupid > 0 {
		if user, okok := userlist.Load(u.CallSign); okok {
			for _, v := range user.(*userinfo).Groups[grolupid].DevMap {
				if v.ISOnline {
					onlinedevlist = append(onlinedevlist, v)
				} else {
					offlinedevlist = append(offlinedevlist, v)
				}

			}

		}

	} else {

		if g, ok := publicGroupMap[grolupid]; ok {

			for _, v := range g.DevMap {
				if v.ISOnline {
					onlinedevlist = append(onlinedevlist, v)
				} else {
					offlinedevlist = append(offlinedevlist, v)
				}

			}

		}
	}

	devlist := append(onlinedevlist, offlinedevlist...)

	rescode, _ := jsonextra.Marshal(devlist)

	respone := fmt.Sprintf(`{"code":20000,"data":{"total":%v,"items":%s}}`,
		len(devlist), rescode)

	w.Write([]byte(respone))

}

func (j *jsonapi) httpMyDeviceList(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	mydevicelist := make(map[string]deviceInfo, 10)

	for kk, vv := range devCallsignSSIDMap {
		if vv.CallSign == u.CallSign {
			mydevicelist[kk] = *vv
		}

	}

	writeJSONResponseItems(w, mydevicelist, len(mydevicelist))

}

func (j *jsonapi) httpDevice(w http.ResponseWriter, req *http.Request) {
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
		log.Println("device err :", err)
		w.Write(ResParmErr)
		return
	}

	if dev, ok := devCallsignSSIDMap[getCallsignSSID(stb.Callsign, stb.SSID)]; ok {

		writeJSONResponseItem(w, dev)
	} else {
		w.Write(ResOpErr)
	}

}

func (j *jsonapi) httpDeviceQTH(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	// _, err := checktoken(w, req)
	// if err != nil {
	// 	return
	// }

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &query{}
	err := jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("get device qth parm err :", err)
		w.Write(ResParmErr)
		return
	}

	if qth, ok := QTHmapNew[getCallsignSSID(stb.Callsign, stb.SSID)]; ok {

		writeJSONResponseItem(w, qth)
	} else {
		writeJSONResponseItem(w, "火星")
	}

}

func (j *jsonapi) httpDeviceQTHs(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	_, err := checktoken(w, req)
	if err != nil {
		return
	}

	writeJSONResponseItem(w, QTHmap)

}

func (j *jsonapi) httpDeviceQTHs2(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	_, err := checktoken(w, req)
	if err != nil {
		return
	}

	writeJSONResponseItem(w, QTHmapNew)

}

// func (j *jsonapi) httpBindDevice(w http.ResponseWriter, req *http.Request) {
// 	sethttphead(w)

// 	u, ok := checktoken(w, req)
// 	if !ok {
// 		return
// 	}

// 	result, _ := io.ReadAll(req.Body)

// 	req.Body.Close()

// 	stb := &deviceInfo{}
// 	err := jsonextra.Unmarshal(result, &stb)

// 	if err != nil {
// 		log.Println("device bind err :", err)
// 		w.Write([]byte(`{"code":20000,"data":{"message":"绑定设备表参数错误"}}`))
// 		return
// 	}

// 	if stb.CallSign != u.CallSign {
// 		w.Write([]byte(`{"code":20000,"data":{"message":"设备绑定或解除绑定操作错误，必须本人操作"}}`))
// 		return
// 	}

// 	err = bindDevice(stb, u.ID)

// 	if err != nil {
// 		log.Println("device bind err :", err)
// 		w.Write([]byte(`{"code":20000,"data":{"message":"绑定设备表参数错误"}}`))
// 		return
// 	}
// 	w.Write([]byte(`{"code":20000,"data":{"message":"设备绑定成功"}}`))

// }

func (j *jsonapi) httpUpdateDevice(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &deviceInfo{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("device update err :", err)
		w.Write([]byte(`{"code":20001,"data":{"message":"设备信息更新错误,数据格式错误"}}`))
		return
	}

	if !checkrole(u, []string{"admin"}) && u.CallSign != stb.CallSign {
		log.Println("device parm query  err")
		w.Write([]byte(`{"code":20001,"data":{"message":"修改设备信息错误，不是本人，或者权限不够！"}}`))
		return

	}

	// if stb.CallSign != u.CallSign {
	// 	w.Write([]byte(`{"code":20000,"data":{"message":"更新设备信息错误，必须本人操作"}}`))
	// 	return
	// }

	dev := getDevice(stb.CallSign, stb.SSID)

	switch {
	case dev.Status != stb.Status:
		content := fmt.Sprintf("%s状态从%d修改为%d", dev.CallSignSSID, dev.Status, stb.Status)
		addOperatorLog(content, "修改设备状态", u)
	case dev.Priority != stb.Priority:
		content := fmt.Sprintf("%s优先级从%d修改为%d", dev.CallSignSSID, dev.Priority, stb.Priority)
		addOperatorLog(content, "修改设备优先级", u)
	case dev.GroupID != stb.GroupID:
		content := fmt.Sprintf("%s房间从%d修改为%d", dev.CallSignSSID, dev.GroupID, stb.GroupID)
		addOperatorLog(content, "修改设备房间", u)
	default:
		addOperatorLog(dev.CallSignSSID, "修改设备参数", u)

	}

	if !checkrole(u, []string{"admin"}) {
		stb.Priority = dev.Priority
	}

	err = updateDevice(stb)

	if err != nil {
		log.Println("device update err :", err)
		w.Write([]byte(`{"code":20001,"data":{"message":"设备信息更新错误,可能组不允许加入"}}`))
		return
	}
	w.Write([]byte(`{"code":20000,"data":{"message":"设备更新成功成功"}}`))

}

func (j *jsonapi) httpDeleteDevice(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &deviceInfo{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("device update err :", err)
		w.Write([]byte(`{"code":20001,"data":{"message":"设备删除错误,数据格式错误"}}`))
		return
	}

	if !checkrole(u, []string{"admin"}) && u.CallSign != stb.CallSign {
		log.Println("device parm query  err")
		w.Write([]byte(`{"code":20000,"data":{"message":"设备删从错误，不是本人，或者权限不够！"}}`))
		return

	}

	// if stb.CallSign != u.CallSign {
	// 	w.Write([]byte(`{"code":20000,"data":{"message":"更新设备信息错误，必须本人操作"}}`))
	// 	return
	// }

	addOperatorLog(stb.CallSignSSID, "删除设备", u)

	err = delDevice(stb)

	if err != nil {
		log.Println("device update err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"设备删除错误"}}`))
		return
	}
	w.Write([]byte(`{"code":20000,"data":{"message":"设备删除成功"}}`))

}

func (j *jsonapi) httpChangeDeviceGroupNRL(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &deviceInfo{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("device update err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"设备信息更新错误,数据格式错误"}}`))
		return
	}

	if !checkrole(u, []string{"admin"}) && u.CallSign != stb.CallSign {
		log.Println("device parm query  err")
		w.Write([]byte(`{"code":20000,"data":{"message":"修改设备信息错误，不是本人，或者权限不够！"}}`))
		return

	}

	dev := getDevice(stb.CallSign, stb.SSID)
	content := fmt.Sprintf("%s房间从%d修改为%d", dev.CallSignSSID, dev.GroupID, stb.GroupID)
	addOperatorLog(content, "修改设备房间", u)

	// if stb.CallSign != u.CallSign {
	// 	w.Write([]byte(`{"code":20000,"data":{"message":"更新设备信息错误，必须本人操作"}}`))
	// 	return
	// }

	err = changeDeviceGroup(stb)

	if err != nil {
		log.Println("device update err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"设备信息更新错误"}}`))
		return
	}
	w.Write([]byte(`{"code":20000,"data":{"message":"设备更新成功成功"}}`))

}

func (j *jsonapi) httpRoomList(w http.ResponseWriter, req *http.Request) {
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
		log.Println("my room list err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"查询我的房间错误"}}`))
		return
	}

	rescode, _ := jsonextra.Marshal(u.DevList)

	respone := fmt.Sprintf(`{"code":20000,"data":{"total":%v,"items":%s}}`,
		len(u.DevList), rescode)

	w.Write([]byte(respone))

}

func (j *jsonapi) httpDeviceAT(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &ATcommand{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("device parm update err :", err)
		w.Write([]byte(`{"code":20001,"data":{"message":"AT数据格式错误"}}`))
		return
	}

	if !checkrole(u, []string{"admin"}) && u.CallSign != stb.CallSign {
		log.Println("device parm query  err")
		w.Write([]byte(`{"code":20001,"data":{"message":"修改设备信息错误，不是本人，或者权限不够！"}}`))
		return

	}

	// if stb.CallSign != u.CallSign {
	// 	w.Write([]byte(`{"code":20000,"data":{"message":"更新设备信息错误，必须本人操作"}}`))
	// 	return
	// }

	addOperatorLog(getCallsignSSID(stb.CallSign, stb.SSID), "AT修改设备参数", u)

	var dev *deviceInfo

	dev, err = DeviceAT(stb)

	if err != nil {
		w.Write([]byte(`{"code":20001,"data":{"message":"AT命令执行失败"}}`))
		return
	}

	time.Sleep(200 * time.Millisecond)

	rescode, _ := jsonextra.Marshal(dev)

	respone := fmt.Sprintf(`{"code":20000,"data":{"message":"AT指令已经发送给设备","items":%s}}`, rescode)

	w.Write([]byte(respone))

	//w.Write([]byte(`{"code":20000,"data":{"message":"AT命令完成"}}`))
	//w.Write(res)

}

func (j *jsonapi) httpQueryDeviceParm(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &deviceInfo{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("device parm query  err :", err)
		w.Write([]byte(`{"code":20001,"data":{"message":"查询设备信息错误"}}`))
		return
	}

	if !checkrole(u, []string{"admin"}) && u.CallSign != stb.CallSign {
		log.Println("device parm query  err")
		w.Write([]byte(`{"code":20001,"data":{"message":"修改设备信息错误"}}`))
		return

	}

	callsignssid := getCallsignSSID(stb.CallSign, stb.SSID)

	fmt.Println("callsignssid:", callsignssid, stb.CallSign, stb.SSID)

	dev, err := queryDeviceParm(callsignssid)

	if err != nil {
		log.Println("device parm query  err :", err)
		w.Write([]byte(`{"code":20001,"data":{"message":"查询设备信息错误，可能设备不在线，或者固件版本过低"}}`))
		return

	}

	rescode, _ := jsonextra.Marshal(dev)
	respone := fmt.Sprintf(`{"code":20000,"data":{"items":%s}}`,
		rescode)

	w.Write([]byte(respone))

}

func (j *jsonapi) httpChangeDeviceParm(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	req.ParseForm()

	fmt.Println("REQ:", req.Form)

	if len(req.Form) < 4 {
		w.Write(ResParmErr)
		return
	}

	callsign := req.Form["callsign"][0]
	ssid := req.Form["ssid"][0]
	callsignssid := callsign + "-" + ssid

	if !checkrole(u, []string{"admin"}) && u.CallSign != callsign {
		log.Println("device parm query  err")
		w.Write(ResRightErr)
		return

	}

	if callsign == "" || ssid == "" {
		log.Println("device parm query  err")
		w.Write(ResParmErr)
		return
	}

outerLoop:
	for k, v := range req.Form {

		//fmt.Println(k, v)

		switch k {
		case "DMRID", "ssid", "callsign":
			continue

		case "dcd_select":
			_, err := changeDeviceByteParm(callsignssid, 0, v[0])
			if err != nil {
				w.Write([]byte(`{"code":20001,"data":{"message":"修改设备DCD选择失败"}}`))
				return
			}
			break outerLoop

		case "ptt_enable":
			_, err := changeDeviceByteParm(callsignssid, 1, v[0])
			if err != nil {
				w.Write([]byte(`{"code":20001,"data":{"message":"修改使能PTT失败"}}`))
				return
			}
			break outerLoop

		case "ptt_level_reversed":
			_, err := changeDeviceByteParm(callsignssid, 2, v[0])

			if err != nil {
				w.Write([]byte(`{"code":20001,"data":{"message":"修改设备信息错误"}}`))
				return
			}
			break outerLoop

		case "add_tail_voice":
			_, err := changeDeviceUint16Parm(callsignssid, 3, v[0])
			if err != nil {
				w.Write([]byte(`{"code":20001,"data":{"message":"加尾音失败"}}`))
				return
			}
			break outerLoop

		case "remove_tail_voice":
			_, err := changeDeviceUint16Parm(callsignssid, 5, v[0])
			if err != nil {
				w.Write([]byte(`{"code":20001,"data":{"message":"消尾音失败"}}`))
				return
			}
			break outerLoop

		case "ptt_resistive":
			_, err := changeDeviceByteParm(callsignssid, 7, v[0])
			if err != nil {
				w.Write([]byte(`{"code":20001,"data":{"message":"修改设备信息错误"}}`))
				return
			}
			break outerLoop

		case "monitor":
			_, err := changeDeviceByteParm(callsignssid, 8, v[0])
			if err != nil {
				w.Write([]byte(`{"code":20001,"data":{"message":"修改设备信息错误"}}`))
				return
			}
			break outerLoop

		case "key_func":
			_, err := changeDeviceByteParm(callsignssid, 9, v[0])
			if err != nil {
				w.Write([]byte(`{"code":20001,"data":{"message":"修改设备信息错误"}}`))
				return
			}
			break outerLoop

		case "realy_status":
			_, err := changeDeviceByteParm(callsignssid, 10, v[0])

			if err != nil {
				w.Write([]byte(`{"code":20001,"data":{"message":"修改设备信息错误"}}`))
				return
			}
			break outerLoop

		case "allow_relay_control":
			_, err := changeDeviceByteParm(callsignssid, 11, v[0])
			if err != nil {
				w.Write([]byte(`{"code":20001,"data":{"message":"修改设备信息错误"}}`))
				return
			}
			break outerLoop

		case "voice_bitrate":
			_, err := changeDeviceByteParm(callsignssid, 12, v[0])
			if err != nil {
				w.Write([]byte(`{"code":20001,"data":{"message":"修改语音码率失败"}}`))
				return
			}
			break outerLoop

		case "local_ipaddr", "gateway", "netmask", "dns_ipaddr", "dest_domainname":

			ipparm := ipparm{32, req.Form["local_ipaddr"][0], 36, req.Form["gateway"][0], 40, req.Form["netmask"][0], 44, req.Form["dns_ipaddr"][0], 80, req.Form["dest_domainname"][0]}
			_, err := changeDeviceIPParm(callsignssid, ipparm)
			if err != nil {
				w.Write([]byte(`{"code":20001,"data":{"message":"改变IP失败,IP不正确"}}`))
				return
			}

			addOperatorLog(callsignssid+":"+ipparm.String(), "修改地址成功", u)
			break outerLoop

		case "newcallsignssid":

			_, err := changeDeviceCallsignSSIDParm(callsignssid, v[0])
			if err != nil {
				w.Write([]byte(`{"code":20000,"data":{"message":"修改设备呼号SSID失败"}}`))
				return
			}
			addOperatorLog(callsignssid+":"+v[0], "修改呼号SSID成功", u)
			break outerLoop

		case "one_uv_power":
			_, err := changeDeviceByteParm(callsignssid, 163, v[0])
			if err != nil {
				w.Write([]byte(`{"code":20001,"data":{"message":"UV电源开关失败"}}`))
				return
			}
			break outerLoop

		case "moto_channel":
			_, err := changeDeviceByteParm(callsignssid, 164, v[0])
			if err != nil {
				w.Write([]byte(`{"code":20001,"data":{"message":"改变摩托3188/3688信道失败"}}`))
				return
			}
			break outerLoop

		default:
			fmt.Println("unknow parm: ", k, v)

		}

	}

	addOperatorLog(callsignssid, "修改设备参数", u)

	w.Write([]byte(`{"code":20000,"data":{"message":"修改成功"}}`))

	//w.Write(r)

	// 第一种方式
	// username := request.Form["username"][0]
	// password := request.Form["password"][0]

	// result, _ := io.ReadAll(req.Body)

	// req.Body.Close()

	// stb := &deviceInfo{}
	// err := jsonextra.Unmarshal(result, &stb)

	// 	if checkrole(u, "admin") == false {
	// 		w.Write([]byte("{\"code\":20000,\"data\":{\"message\":\"当前用户没有权限设置此参数\"}}"))
	// 		return

	// 	}

	//|| checkrole(u, []string{"admin"})

	// if stb.CallSign != u.CallSign {
	// 	w.Write([]byte(`{"code":20000,"data":{"message":"查询设备信息错误，必须本人操作"}}`))
	// 	return
	// }

	// dev := queryDeviceParm(stb.CPUID)

	// if dev == nil {
	// 	log.Println("device parm query  err :", err)
	// 	w.Write([]byte(`{"code":20000,"data":{"message":"查询设备信息错误"}}`))
	// 	return
	// }

	// rescode, _ := jsonextra.Marshal(dev)
	// respone := fmt.Sprintf(`{"code":20000,"data":{"items":%s}}`,
	// 	rescode)

	// w.Write([]byte(respone))

}

func (j *jsonapi) httpChange1W(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &control{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("device parm update err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"1W设备参数信息更新错误,数据格式错误"}}`))
		return
	}

	if !checkrole(u, []string{"admin"}) && u.CallSign != stb.CallSign {
		log.Println("device parm query  err")
		w.Write([]byte(`{"code":20000,"data":{"message":"修改设备信息错误，不是本人，或者权限不够！"}}`))
		return

	}

	// if stb.CallSign != u.CallSign {
	// 	w.Write([]byte(`{"code":20000,"data":{"message":"更新设备信息错误，必须本人操作"}}`))
	// 	return
	// }

	addOperatorLog(getCallsignSSID(stb.CallSign, stb.SSID), "修改设备参数", u)
	_, err = changeDevice1W(stb)

	if err != nil {
		w.Write([]byte(`{"code":20000,"data":{"message":"1W模块设置失败"}}`))
		return
	}
	w.Write([]byte(`{"code":20000,"data":{"message":"1W模块设置完成"}}`))
	//w.Write(res)

}

func (j *jsonapi) httpChange2W(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &control{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("device parm update err :", err)

		w.Write([]byte(`{"code":20000,"data":{"message":"1W设备参数信息更新错误,数据格式错误"}}`))
		return
	}

	if !checkrole(u, []string{"admin"}) && u.CallSign != stb.CallSign {
		log.Println("device parm query  err")
		w.Write([]byte(`{"code":20000,"data":{"message":"修改设备信息错误，不是本人，或者权限不够！"}}`))
		return

	}

	// if stb.CallSign != u.CallSign {
	// 	w.Write([]byte(`{"code":20000,"data":{"message":"更新设备信息错误，必须本人操作"}}`))
	// 	return
	// }

	addOperatorLog(getCallsignSSID(stb.CallSign, stb.SSID), "修改设备参数", u)

	_, err = changeDevice2W(stb)

	if err != nil {
		w.Write([]byte(`{"code":20000,"data":{"message":"2W模块设置失败"}}`))
		return
	}
	w.Write([]byte(`{"code":20000,"data":{"message":"2W模块设置成功"}}`))
	//w.Write(res)

}
