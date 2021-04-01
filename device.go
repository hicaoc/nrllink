package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

func (j *jsonapi) httpDeviceList(w http.ResponseWriter, req *http.Request) {
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

	devicelist := make(map[int]deviceInfo, 10)

	var id int

	for _, vv := range devCPUIDMap {

		t := time.Now()
		if t.Sub(vv.LastPacketTime) > 5*time.Second {
			vv.ISOnline = false
		}

		dev := *vv

		dev.CPUID = ""
		dev.DeviceParm = nil

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

	result, _ := ioutil.ReadAll(req.Body)

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

func (j *jsonapi) httpBindDevice(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, ok := checktoken(w, req)
	if !ok {
		return
	}

	result, _ := ioutil.ReadAll(req.Body)

	req.Body.Close()

	stb := &deviceInfo{}
	err := jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("device bind err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"绑定设备表参数错误"}}`))
		return
	}

	if stb.CallSign != u.CallSign {
		w.Write([]byte(`{"code":20000,"data":{"message":"设备绑定或解除绑定操作错误，必须本人操作"}}`))
		return
	}

	if stb.OwerID == 0 {

		err = bindDevice(stb, u.ID)

		if err != nil {
			log.Println("device bind err :", err)
			w.Write([]byte(`{"code":20000,"data":{"message":"绑定设备表参数错误"}}`))
			return
		}
		w.Write([]byte(`{"code":20000,"data":{"message":"设备绑定成功"}}`))
		return

	}
	w.Write([]byte(`{"code":20000,"data":{"message":"设备已经绑定，无需重复绑定"}}`))

}

func (j *jsonapi) httpUpdateDevice(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, ok := checktoken(w, req)
	if !ok {
		return
	}

	result, _ := ioutil.ReadAll(req.Body)

	req.Body.Close()

	stb := &deviceInfo{}
	err := jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("device update err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"设备信息更新错误,数据格式错误"}}`))
		return
	}

	if stb.CallSign != u.CallSign {
		w.Write([]byte(`{"code":20000,"data":{"message":"更新设备信息错误，必须本人操作"}}`))
		return
	}

	err = updateDevice(stb)

	if err != nil {
		log.Println("device update err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"设备信息更新错误,设备必须先绑定"}}`))
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

	result, _ := ioutil.ReadAll(req.Body)

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

	result, _ := ioutil.ReadAll(req.Body)

	req.Body.Close()

	stb := &deviceInfo{}
	err := jsonextra.Unmarshal(result, &stb)

	if stb.CallSign != u.CallSign {
		w.Write([]byte(`{"code":20000,"data":{"message":"查询设备信息错误，必须本人操作"}}`))
		return
	}

	dev := queryDeviceParm(stb.CPUID)

	if dev == nil {
		log.Println("device parm query  err :", err)
		w.Write([]byte(`{"code":20000,"data":{"message":"查询设备信息错误"}}`))
		return
	}

	rescode, _ := jsonextra.Marshal(dev)
	respone := fmt.Sprintf(`{"code":20000,"data":{"items":%s}}`,
		rescode)

	w.Write([]byte(respone))

}
