package main

import (
	"io"
	"log"
	"net/http"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type respData struct {
	Total int         `json:"total"`
	Items interface{} `json:"items"`
}

var ResOK, _ = jsonextra.Marshal(Response{20000, "操作成功", nil})
var ResOpErr, _ = jsonextra.Marshal(Response{20001, "操作失败", nil})
var ResRightErr, _ = jsonextra.Marshal(Response{20001, "权限不足", nil})
var ResParmErr, _ = jsonextra.Marshal(Response{20001, "参数错误", nil})

var ResTokenErr, _ = jsonextra.Marshal(Response{50008, "令牌错误，可能是登录超时，请重新登录", nil})
var ResAccountErr, _ = jsonextra.Marshal(Response{50008, "账号错误", nil})

var ResTokenFormatErr, _ = jsonextra.Marshal(Response{50008, "格式错误", nil})
var ResTokenSignErr, _ = jsonextra.Marshal(Response{50008, "签名错误", nil})
var ResTokenDecodeErr, _ = jsonextra.Marshal(Response{50008, "解码错误", nil})
var ResTokenTimeoutErr, _ = jsonextra.Marshal(Response{50008, "登录超时", nil})
var ResTokenExpireErr, _ = jsonextra.Marshal(Response{50008, "登录过期", nil})

func writeJSONResponse(w http.ResponseWriter, res *Response) error {
	w.Header().Set("Content-Type", "application/json")
	//w.WriteHeader(status)
	return jsonextra.NewEncoder(w).Encode(res)

}

func writeJSONResponseData(w http.ResponseWriter, list interface{}, total int) error {
	return writeJSONResponse(w, &Response{20000, "ok", respData{total, list}})
}

func checkHttpRequestTokenAndRight(w http.ResponseWriter, req *http.Request, rolelist []string) (*userinfo, []byte) {
	sethttphead(w)
	token, err := ValidateToken(req.Header.Get("x-token"))

	if err != nil {
		//writeJSONResponse(w, 50018, "Token错误", nil)

		return nil, ResTokenErr
	}

	u, err := getuser(token.Username)

	if err != nil {
		//writeJSONResponse(w, 50018, "账号错误", nil)

		return nil, ResAccountErr
	}

	if !checkrole(u, rolelist) {

		//log.Println("right err :", rolelist, u.Roles)
		//swriteJSONResponse(w, 20001, "权限不足", nil)

		return nil, ResRightErr

	}

	return u, nil

}

func checkHttpRequest(w http.ResponseWriter, req *http.Request, rolelist []string) (*userinfo, *query, []byte) {
	u, respData := checkHttpRequestTokenAndRight(w, req, rolelist)

	if respData != nil {
		return nil, nil, respData
	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	stb := &query{}
	err := jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("query err :", err)

		//writeJSONResponse(w, 20001, "参数错误", nil)

		return nil, nil, ResParmErr
	}

	return u, stb, nil

}
