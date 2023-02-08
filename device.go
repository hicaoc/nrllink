package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

func (j *jsonapi) httpDeviceList(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, ok := checktoken(w, req)
	if !ok {
		return
	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &query{}
	err := jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("device list err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"查询设备表参数错误"}}`))
		return
	}

	devicelist := make(map[int]deviceInfo, 10)

	isadmin := checkrole(u, []string{"admin"})

	var id int

	totalstats.OnlineDevNumber = 0

	for _, vv := range devCPUIDMap {

		if vv.CallSign == "" {
			continue
		}

		t := time.Now()
		if t.Sub(vv.LastPacketTime) > 15*time.Second {
			vv.ISOnline = false
		} else {
			totalstats.OnlineDevNumber++
			vv.ISOnline = true
		}

		dev := *vv

		if !isadmin && dev.CallSign != u.CallSign {
			dev.CPUID = ""
			dev.DeviceParm = nil
		}

		devicelist[id] = dev
		id++

	}

	rescode, _ := jsonextra.Marshal(devicelist)

	respone := fmt.Sprintf(`{"code":20000,"data":{"total":%v,"items":%s}}`,
		len(devCPUIDMap), rescode)

	w.Write([]byte(respone))

}

func (j *jsonapi) httpMyDeviceList(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, ok := checktoken(w, req)
	if !ok {
		return
	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &query{}
	err := jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("device list err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"查询设备表参数错误"}}`))
		return
	}

	mydevicelist := make(map[string]deviceInfo, 10)

	for kk, vv := range devCPUIDMap {
		if vv.CallSign == u.CallSign {
			mydevicelist[kk] = *vv
		}

	}

	rescode, _ := jsonextra.Marshal(mydevicelist)

	respone := fmt.Sprintf(`{"code":20000,"data":{"total":%v,"items":%s}}`,
		len(devCPUIDMap), rescode)

	w.Write([]byte(respone))

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

	u, ok := checktoken(w, req)
	if !ok {
		return
	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &deviceInfo{}
	err := jsonextra.Unmarshal(result, &stb)

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

	// if stb.CallSign != u.CallSign {
	// 	w.Write([]byte(`{"code":20000,"data":{"message":"更新设备信息错误，必须本人操作"}}`))
	// 	return
	// }

	err = updateDevice(stb)

	if err != nil {
		log.Println("device update err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"设备信息更新错误"}}`))
		return
	}
	w.Write([]byte(`{"code":20000,"data":{"message":"设备更新成功成功"}}`))

}

func (j *jsonapi) httpChangeDeviceGroupNRL(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, ok := checktoken(w, req)
	if !ok {
		return
	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &deviceInfo{}
	err := jsonextra.Unmarshal(result, &stb)

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

	u, ok := checktoken(w, req)
	if !ok {
		return
	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &query{}
	err := jsonextra.Unmarshal(result, &stb)

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

func (j *jsonapi) httpQueryDeviceParm(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, ok := checktoken(w, req)
	if !ok {
		return
	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &deviceInfo{}
	err := jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("device parm query  err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"查询设备信息错误"}}`))
		return
	}

	if !checkrole(u, []string{"admin"}) && u.CallSign != stb.CallSign {
		log.Println("device parm query  err")
		w.Write([]byte(`{"code":20000,"data":{"message":"修改设备信息错误"}}`))
		return

	}

	dev, err := queryDeviceParm(stb.CPUID)

	if err != nil {
		log.Println("device parm query  err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"查询设备信息错误，可能设备不在线，或者固件版本过低"}}`))
		return

	}

	rescode, _ := jsonextra.Marshal(dev)
	respone := fmt.Sprintf(`{"code":20000,"data":{"items":%s}}`,
		rescode)

	w.Write([]byte(respone))

}

func (j *jsonapi) httpChangeDeviceParm(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, ok := checktoken(w, req)
	if !ok {
		return
	}

	req.ParseForm()

	fmt.Println("REQ:", len(req.Form))

	cpuid := req.Form["CPUID"][0]
	callsign := req.Form["callsign"][0]

	if !checkrole(u, []string{"admin"}) && u.CallSign != callsign {
		log.Println("device parm query  err")
		w.Write([]byte(`{"code":20000,"data":{"message":"修改设备信息错误"}}`))
		return

	}

	if cpuid == "" {

		log.Println("device parm query  err")
		w.Write([]byte(`{"code":20000,"data":{"message":"修改设备信息错误"}}`))
		return

	}

	for k, v := range req.Form {

		//fmt.Println(k, v)

		switch k {
		case "dcd_select":
			_, err := changeDeviceByteParm(cpuid, 0, v[0])
			if err != nil {
				w.Write([]byte(`{"code":20000,"data":{"message":"修改设备DCD选择失败"}}`))
				return
			}

		case "ptt_enable":
			_, err := changeDeviceByteParm(cpuid, 1, v[0])
			if err != nil {
				w.Write([]byte(`{"code":20000,"data":{"message":"修改使能PTT失败"}}`))
				return
			}

		case "ptt_level_reversed":
			_, err := changeDeviceByteParm(cpuid, 2, v[0])

			if err != nil {
				w.Write([]byte(`{"code":20000,"data":{"message":"修改设备信息错误"}}`))
				return
			}

		case "add_tail_voice":
			_, err := changeDeviceUint16Parm(cpuid, 3, v[0])
			if err != nil {
				w.Write([]byte(`{"code":20000,"data":{"message":"加尾音失败"}}`))
				return
			}

		case "remove_tail_voice":
			_, err := changeDeviceUint16Parm(cpuid, 5, v[0])
			if err != nil {
				w.Write([]byte(`{"code":20000,"data":{"message":"消尾音失败"}}`))
				return
			}

		case "ptt_resistive":
			_, err := changeDeviceByteParm(cpuid, 7, v[0])
			if err != nil {
				w.Write([]byte(`{"code":20000,"data":{"message":"修改设备信息错误"}}`))
				return
			}

		case "monitor_out":
			_, err := changeDeviceByteParm(cpuid, 8, v[0])
			if err != nil {
				w.Write([]byte(`{"code":20000,"data":{"message":"修改设备信息错误"}}`))
				return
			}

		case "key_func":
			_, err := changeDeviceByteParm(cpuid, 9, v[0])
			if err != nil {
				w.Write([]byte(`{"code":20000,"data":{"message":"修改设备信息错误"}}`))
				return
			}

		case "realy_status":
			_, err := changeDeviceByteParm(cpuid, 10, v[0])

			if err != nil {
				w.Write([]byte(`{"code":20000,"data":{"message":"修改设备信息错误"}}`))
				return
			}

		case "allow_relay_control":
			_, err := changeDeviceByteParm(cpuid, 11, v[0])
			if err != nil {
				w.Write([]byte(`{"code":20000,"data":{"message":"修改设备信息错误"}}`))
				return
			}

		case "voice_bitrate":
			_, err := changeDeviceByteParm(cpuid, 12, v[0])
			if err != nil {
				w.Write([]byte(`{"code":20000,"data":{"message":"修改语音码率失败"}}`))
				return
			}

		case "local_ipaddr":

			ipparm := ipparm{32, req.Form["local_ipaddr"][0], 36, req.Form["gateway"][0], 40, req.Form["netmask"][0], 44, req.Form["dns_ipaddr"][0], 80, req.Form["dest_domainname"][0]}
			_, err := changeDeviceIPParm(cpuid, ipparm)
			if err != nil {
				w.Write([]byte(`{"code":20000,"data":{"message":"改变IP失败,IP不正确"}}`))
				return
			}

		// case "gateway":
		// 	res, err := changeDeviceIPParm(cpuid, 36, v[0])
		// 	if err != nil {
		// 		w.Write([]byte(`{"code":20000,"data":{"message":"改变目网关IP失败,IP不正确"}}`))
		// 		return
		// 	}
		// 	r = append(r, res...)

		// case "netmask":
		// 	res, err := changeDeviceIPParm(cpuid, 40, v[0])
		// 	if err != nil {
		// 		w.Write([]byte(`{"code":20000,"data":{"message":"改变本地IP掩码失败,IP不正确"}}`))
		// 		return
		// 	}

		// case "dns_ipaddr":
		// 	res, err := changeDeviceIPParm(cpuid, 44, v[0])
		// 	if err != nil {
		// 		w.Write([]byte(`{"code":20000,"data":{"message":"改变DNS服务器地址失败,IP不正确"}}`))
		// 		return
		// 	}

		case "ssid":
			_, err := changeDeviceByteParm(cpuid, 64, v[0])
			if err != nil {
				w.Write([]byte(`{"code":20000,"data":{"message":"修改设备SSID失败"}}`))
				return
			}

		// case "dest_domainname":
		// 	res, err := changeDeviceMutiByteParm(cpuid, 80, v[0])
		// 	if err != nil {
		// 		w.Write([]byte(`{"code":20000,"data":{"message":"改变目标IP失败,IP格式不正确"}}`))
		// 		return
		// 	}
		// 	r = append(r, res...)

		case "one_uv_power":
			_, err := changeDeviceByteParm(cpuid, 163, v[0])
			if err != nil {
				w.Write([]byte(`{"code":20000,"data":{"message":"UV电源开关失败"}}`))
				return
			}

		case "moto_channel":
			_, err := changeDeviceByteParm(cpuid, 164, v[0])
			if err != nil {
				w.Write([]byte(`{"code":20000,"data":{"message":"改变摩托3188/3688信道失败"}}`))
				return
			}

		default:
			fmt.Println("unknow parm: ", k, v)

		}

	}

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

	u, ok := checktoken(w, req)
	if !ok {
		return
	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &control{}
	err := jsonextra.Unmarshal(result, &stb)

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

	u, ok := checktoken(w, req)
	if !ok {
		return
	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &control{}
	err := jsonextra.Unmarshal(result, &stb)

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

	_, err = changeDevice2W(stb)

	if err != nil {
		w.Write([]byte(`{"code":20000,"data":{"message":"2W模块设置失败"}}`))
		return
	}
	w.Write([]byte(`{"code":20000,"data":{"message":"2W模块设置成功"}}`))
	//w.Write(res)

}
