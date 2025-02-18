package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func (j *jsonapi) httpRegisterList(w http.ResponseWriter, req *http.Request) {
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

	emplist, total := selectReguser(queryToWhere("", *stb))

	rescode, _ := jsonextra.Marshal(emplist)

	respone := fmt.Sprintf(`{"code":20000,"data":{"total":%v,"items":%s}}`,
		total, rescode)

	w.Write([]byte(respone))

}

func (j *jsonapi) httpRegister(w http.ResponseWriter, r *http.Request) {

	sethttphead(w)

	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// 限制上传文件的大小（这里限制为 10 MB）
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

	// 解析表单数据（解析时会检查大小限制）
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 这里的 10 << 20 同样是 10 MB 限制
		w.Write(ResParmErr)
		return
	}

	// 获取用户注册信息
	callsign := r.FormValue("callsign")
	name := r.FormValue("name")
	phone := r.FormValue("phone")
	address := r.FormValue("address")
	mail := r.FormValue("mail")
	password := r.FormValue("password")

	// 检查必填字段
	if callsign == "" || name == "" || phone == "" || password == "" {
		w.Write(ResParmErr)
		return
	}

	// 创建上传目录（按年月分割）
	currentMonth := time.Now().Format("2006-01")
	uploadDir := filepath.Join(conf.System.LicensePath, currentMonth)
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		w.Write(ResOpErr)
		return
	}

	// 保存文件路径
	var opCertPath, licensePath string

	// 处理上传的操作证文件
	opCertFile, opCertHeader, err := r.FormFile("certificate")
	if err == nil {
		defer opCertFile.Close()
		opCertPath = filepath.Join(uploadDir, callsign+"_certificate"+filepath.Ext(opCertHeader.Filename))
		if err := saveFile(opCertFile, opCertPath); err != nil {
			w.Write(ResOpErr)
			return
		}
	}

	// 处理上传的电台执照文件
	licenseFile, licenseHeader, err := r.FormFile("license")
	if err == nil {
		defer licenseFile.Close()
		licensePath = filepath.Join(uploadDir, callsign+"_license"+filepath.Ext(licenseHeader.Filename))
		if err := saveFile(licenseFile, licensePath); err != nil {
			w.Write(ResOpErr)
			return
		}
	}

	reguser := &reguser{
		CallSign:    callsign,
		Name:        name,
		Phone:       phone,
		Address:     address,
		Mail:        mail,
		Password:    password,
		OpCertPath:  opCertPath,
		LicensePath: licensePath,
		CreateTime:  time.Now().Format("2006-01-02 15:04:05"),
		UpdateTime:  time.Now().Format("2006-01-02 15:04:05"),
		Status:      1,
	}

	err = createRegUser(reguser)

	if err != nil {
		w.Write(ResParmErr)
		return

	}

	// 返回注册成功信息
	w.Write(ResOK)
}

func (j *jsonapi) httpAddReg(w http.ResponseWriter, req *http.Request) {

	sethttphead(w)

	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &reguser{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("query err :", err)

		w.Write(ResParmErr)
		return

	}
	err = addUserReg(req.Context(), stb)
	if err != nil {
		log.Println("area school add err :", err)
		//writeJSONResponse(w, &Response{20001,  "校区添加失败", nil))
		w.Write(ResOpErr)
		return
	}
	addOperatorLog(stb.Name, "增加用户", u)

	//writeJSONResponse(w, &Response{20000, "校区添加成功", nil))
	w.Write(ResOK)

}

func (j *jsonapi) httpUpdateReg(w http.ResponseWriter, req *http.Request) {
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

	stb := &reguser{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("query err :", err)

		w.Write(ResParmErr)
		return

	}

	err = updateRegUser(stb)

	if err != nil {
		w.Write(ResOpErr)
		return

	}

	w.Write(ResOK)

}

func (j *jsonapi) httpDeleteRegUser(w http.ResponseWriter, req *http.Request) {
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

	stb := &reguser{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("user delete err :", err)
		w.Write(ResOpErr)
		return
	}

	deleteRegUser(stb)
	addOperatorLog(stb.CallSign, "用户删除成功", u)

	w.Write([]byte(`{"code":20000,"data":{"isok":0,"message":"用户删除成功"}}`))

}
