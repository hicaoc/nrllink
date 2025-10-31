package main

import (
	"encoding/base64"
	"encoding/json"
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
		log.Println("reg user list err :", err)
		w.Write(ResParmErr)
		return
	}

	emplist, total := selectReguser(queryToWhere("", *stb))

	rescode, _ := jsonextra.Marshal(emplist)

	respone := fmt.Sprintf(`{"code":20000,"data":{"total":%v,"items":%s}}`,
		total, rescode)

	w.Write([]byte(respone))

}

func (j *jsonapi) httpRegisterImage(w http.ResponseWriter, req *http.Request) {

	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	if !checkrole(u, []string{"admin"}) {

		w.Write(ResRightErr)
		return

	}

	result, _ := io.ReadAll(req.Body)

	stb := &query{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("reg user list err :", err)
		w.Write(ResParmErr)
		return
	}

	// 验证图片路径合法性
	absPath, err := filepath.Abs(stb.Path) // 转换为绝对路径
	if err != nil {
		http.Error(w, "Invalid image path", http.StatusBadRequest)
		return
	}

	// 检查文件是否存在并读取内容
	file, err := os.Open(absPath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	// 读取文件内容
	imageData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	// 将二进制数据转换为 Base64 编码
	base64Data := base64.StdEncoding.EncodeToString(imageData)

	// 构造 JSON 响应
	response := Response{
		Code:    20000,
		Message: "Image data retrieved successfully",
		Data:    base64Data,
	}

	// 设置响应头信息
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// 返回 JSON 响应
	json.NewEncoder(w).Encode(response)

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

	user, _ := getuser(phone)

	// if err != nil {
	// 	w.Write(ResOpErr)
	// }

	if user != nil {
		w.Write(ResUserAleadyExits)
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
	var licensePath string

	// // 处理上传的操作证文件
	// opCertFile, opCertHeader, err := r.FormFile("certificate")
	// if err == nil {
	// 	defer opCertFile.Close()
	// 	opCertPath = filepath.Join(uploadDir, callsign+"_certificate"+filepath.Ext(opCertHeader.Filename))
	// 	if err := saveFile(opCertFile, opCertPath); err != nil {
	// 		w.Write(ResOpErr)
	// 		return
	// 	}
	// }

	// 处理上传的电台执照文件
	licenseFile, licenseHeader, err := r.FormFile("license")
	if err == nil {
		log.Println("注册信息licenseFile：", err)
		defer licenseFile.Close()
		licensePath = filepath.Join(uploadDir, callsign+"_license"+filepath.Ext(licenseHeader.Filename))
		if err := saveFile(licenseFile, licensePath); err != nil {
			log.Println("注册信息licensePath：", err)
			w.Write(ResOpErr)
			return
		}
	}

	//fmt.Println("opCertPath:", opCertPath)
	fmt.Println("licensePath:", licensePath)

	reguser := &reguser{
		CallSign:    callsign,
		Name:        name,
		Phone:       phone,
		Address:     address,
		Mail:        mail,
		Password:    password,
		OpCertPath:  licensePath,
		LicensePath: licensePath,
		CreateTime:  time.Now().Format("2006-01-02 15:04:05"),
		UpdateTime:  time.Now().Format("2006-01-02 15:04:05"),
		Status:      1,
	}

	fmt.Println("注册信息：", reguser)

	err = createRegUser(reguser)

	if err != nil {
		log.Println("add reg user failed  , ", err)
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

	stb.Note = u.Name + u.CallSign

	err = addUserReg(req.Context(), stb)
	if err != nil {
		log.Println("area school add err :", err)
		//writeJSONResponse(w, &Response{20001,  "校区添加失败", nil))
		w.Write(ResOpErr)
		return
	}
	addOperatorLog(stb.Name+stb.CallSign+stb.Phone, "增加用户", u)

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
