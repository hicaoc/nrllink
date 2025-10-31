package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lib/pq"
)

func (j *jsonapi) httpWXMsg(w http.ResponseWriter, req *http.Request) {

	// vars := mux.Vars(req)
	// masterschname := vars["masterschname"]

	// sch := getAreaSchoolFormMap(masterschname)

	// if sch == nil {

	// 	log.Println("wx api schname not found ", masterschname)

	// 	return

	// }

	req.ParseForm()

	echostr := strings.TrimSpace(req.Form.Get("echostr"))
	timestamp := strings.Join(req.Form["timestamp"], "")
	nonce := strings.Join(req.Form["nonce"], "")
	signature := strings.Join(req.Form["signature"], "")
	encryptType := strings.Join(req.Form["encrypt_type"], "")
	msgSignature := strings.Join(req.Form["msg_signature"], "")

	if !ValidateUrl(timestamp, nonce, signature) {
		//	wmm.log.AppendObj(nil, "Wechat Message Service: this http request is not from Wechat platform!")
		// return
		log.Println("Wechat Message Service: this http request is not from Wechat platform!")
		fmt.Println("signature:", signature, "timestamp:", timestamp, "nonce:", nonce, "echostr:", echostr, "encryptType:", encryptType, "msgSignature:", msgSignature)
	}

	//微信安全模式更改/首次添加url，需要验证，将参数原样写会即可

	if echostr != "" {
		//todo 将参数值es原先写回即可
		_, err := w.Write([]byte(echostr))

		if err != nil {
			log.Println("make wx  echostr respons err 1 :", err)
			return
		}

		return
	}

	wxrequest := ParseTextRequestBody(req)

	// log.Println(wxrequest)

	switch wxrequest.Event {
	case "TEMPLATESENDJOBFINISH":
		return
	case "subscribe":

		msg, err := MakeTextResponseBody(wxrequest.ToUserName, wxrequest.FromUserName, conf.WeiXin.WeixinWelcome)
		if err != nil {
			log.Println("send respons err :", err)
		}
		_, err = w.Write(msg)
		if err != nil {
			log.Println("make wx  subscribe respons err 1 :", err)
			return
		}

		//获取关注用户其他信息，已经无法获取头像和昵称
		user, err := getWeixinUserInfo(wxrequest.FromUserName)

		if err != nil {
			log.Println("获取微信用户详情错误：", err)
			return
		}

		// log.Println(user)

		addwxuser(user)

		return

	case "unsubscribe":

		unsubscribe(wxrequest.FromUserName)

		return

	case "CLICK":

		//	fmt.Println("-----", wxrequest)
		switch wxrequest.EventKey {

		case "V1001_SIGN_TIMES":

			user, err := getuser(wxrequest.FromUserName)

			if err != nil {
				//log.Println("V1001_SIGN_TIMES get stu info err:", err)
				msg, err := MakeTextResponseBody(wxrequest.ToUserName, wxrequest.FromUserName, "未查询到相关信息,可能需要先绑定")
				if err != nil {
					log.Println("send respons err,gen resp msg err :", err)
				}

				_, err = w.Write(msg)
				if err != nil {
					log.Println("make wx  V1001_SIGN_TIMES respons err 1 :", err)
					return
				}
				return

			}

			var info string

			info = info + fmt.Sprintf("呼叫时长%v,呼叫次数:%v", user.TalkDuration, user.TalkTimes)

			info = info + "\n\n"

			msg, err := MakeTextResponseBody(wxrequest.ToUserName, wxrequest.FromUserName, info)
			if err != nil {
				log.Println("send respons err :", err)
			}
			_, err = w.Write(msg)
			if err != nil {
				log.Println("make wx  clickEventMap respons err 1 :", err)
				return
			}

			return

		case "V1001_PHONE":

			// status := ""
			// if checkPhoneCode(wxrequest.FromUserName) == true {

			// 	status = "你的微信已经绑定过了,可以重新多次绑定"

			// }

			if !checkUser(wxrequest.FromUserName) {

				user, err := getWeixinUserInfo(wxrequest.FromUserName)

				if err != nil {
					log.Println("获取微信用户详情错误：", err)
					return
				}

				// log.Println(user)

				addwxuser(user)

			}
			//rand.Seed(time.Now().UnixNano())
			//phonecode := rand.New(rand.NewSource(time.Now().UnixNano())).Intn(999999)
			phonecode := addBindCode(wxrequest.FromUserName)
			msg, err := MakeTextResponseBody(wxrequest.ToUserName, wxrequest.FromUserName, fmt.Sprintf("您的绑定码为：%v，请尽快联系管理员绑定，15分钟内有效。", phonecode))
			if err != nil {
				log.Println("MakeTextResponseBody respons err :", err)
			}
			_, err = w.Write(msg)
			if err != nil {
				log.Println("make wx  phonecode respons err 1 :", err)
				return
			}

			//setwxuserPhoneCode(phonecode, wxrequest.FromUserName, sch)
			return

		default:
			//fmt.Println("+++++", wxrequest.Event)
			//V1001_YINHANG_HELP
			// if wxrequest.Event != "CLICK" {
			// 	log.Println("Not click event :", wxrequest.MsgType)
			// 	return
			// }

			if m, ok := conf.WeiXin.ClickEventMap[wxrequest.EventKey]; ok {

				msg, err := MakeTextResponseBody(wxrequest.ToUserName, wxrequest.FromUserName, m)
				if err != nil {
					log.Println("send respons err :", err)
				}
				_, err = w.Write(msg)
				if err != nil {
					log.Println("make wx  clickEventMap respons err :", err)
					return
				}

			} else {
				log.Println("unknow click event:", wxrequest.EventKey, conf.WeiXin.ClickEventMap)
			}
			return

		}

	}

	//如果是事件消息，返回，暂时不处理
	if wxrequest.MsgType == "event" {
		return
	}

	//fmt.Println("weixin request:", wxrequest)

	addwxmsg(wxrequest)

	// stu, err := getHAMInfo(wxrequest.FromUserName, masterschname)

	// if err != nil || stu == nil || len(stu) == 0 {
	// 	//log.Println("V1001_SIGN_TIMES get stu info err:", err)
	// 	msg, err := MakeTextResponseBody(wxrequest.ToUserName, wxrequest.FromUserName, "未查询到相关信息,可能需要先绑定校区")
	// 	if err != nil {
	// 		log.Println("send respons err,gen resp msg err :", err)
	// 	}
	// 	w.Write(msg)
	// 	return

	// }

	if wxrequest.MsgType == "text" && conf.OpenAI.APIKEY != "" {

		_, err := w.Write([]byte("success"))
		if err != nil {
			log.Println("make wx respons err 2 :", err)

		}

		go func(fromUser, content, AccessToken string) {

			anser := sendMessage(fromUser, content)

			err = pushCustomMsg(AccessToken, wxrequest.FromUserName, anser)
			// msg, err := MakeTextResponseBody(wxrequest.ToUserName, wxrequest.FromUserName, anser)
			if err != nil {
				log.Println("send customMSg  err 1 :", err)
				return
			}

		}(wxrequest.FromUserName, wxrequest.Content, conf.WeiXin.WeiXinAccessToken.AccessToken)

		return

	}

	if wxrequest.MsgType == "text" {

		if m, ok := conf.WeiXin.KeywordsMap[wxrequest.Content]; ok {

			msg, err := MakeTextResponseBody(wxrequest.ToUserName, wxrequest.FromUserName, m)
			if err != nil {
				log.Println("send respons err :", err)
			}
			_, err = w.Write(msg)
			if err != nil {
				log.Println("make wx  KeywordsMap respons err :", err)
				return
			}

		} else if conf.WeiXin.DefaultKeywords != "" {

			//log.Println("Other text event:", wxrequest.EventKey, conf.WeiXin.KeyWords)
			//info := "公众号消息会有延时，请稍等，或请直联系管理员。"

			//if conf.WeiXin.DefaultKeywords != "" {
			info := conf.WeiXin.DefaultKeywords
			//}

			msg, err := MakeTextResponseBody(wxrequest.ToUserName, wxrequest.FromUserName, info)
			if err != nil {
				log.Println("send text respons err :", err)
			}

			_, err = w.Write(msg)
			if err != nil {
				log.Println("make wx respons err 2 :", err)
			}
			return

		}
		return

	}

	_, err := w.Write([]byte("success"))
	if err != nil {
		log.Println("make wx respons err 2 :", err)

	}

}

var BindCodeMap sync.Map

var randNum = rand.New(rand.NewSource(time.Now().UnixNano()))

func addBindCode(openid string) string {

	// Generate a random 6-digit number
	code := strconv.Itoa(randNum.Intn(90000000) + 10000000)

	BindCodeMap.Store(code, openid)

	time.AfterFunc(15*time.Minute, func() {
		// Delete the BindCode object from the maps
		BindCodeMap.Delete(code)

	})

	// Return the generated bind code
	return code
}

func getOpenID(bindCode string) (string, error) {
	// 从BindCodeMap中查找对应的BindCode对象
	if value, ok := BindCodeMap.Load(bindCode); ok {
		// 类型断言将值转换为BindCode对象
		if openid, ok := value.(string); ok {
			return openid, nil
		}
	}
	// 如果找不到对应的openid，则返回错误
	return "", fmt.Errorf("OpenID not found for bind code: %s", bindCode)
}

type MPUserInfo struct {
	ID string `json:"id" db:"id"`

	PhoneCode string `json:"phone_code"`
	AccessKey string `json:"access_key"`             //中心小程序后端程序和平台程序通讯密钥
	MPopenID  string `json:"mpopenid" db:"mpopenid"` //小程序openid
	Unionid   string `json:"unionid" db:"unionid"`

	NickName       string         `json:"nickName" db:"nickname"`
	Gender         int            `json:"gender" db:"gender"`
	Language       string         `json:"language" `
	City           string         `json:"city" `
	Province       string         `json:"province"`
	Country        string         `json:"country"`
	AvatarURL      string         `json:"avatarUrl"`
	Schname        string         `json:"schname" db:"schname"`
	MasterSchname  string         `json:"master_schname" db:"master_schname"`
	SchoolID       int            `json:"school_id" db:"school_id"`     //当前校区
	SchoolName     string         `json:"school_name" db:"school_name"` //当前校区名称
	SchList        pq.StringArray `json:"sch_list" db:"sch_list"`
	ServerURL      string         `json:"server_url" db:"server_url"`
	Phone          string         `json:"phone" db:"phone"`     // crm手机号
	MPPhone        string         `json:"mpphone" db:"mpphone"` //微信小程序验证手机号
	Subscribe      int            `json:"subscribe" db:"subscribe"`
	OpenID         string         `json:"openid" db:"openid"` //公众号openid
	Name           string         `json:"name" db:"name"`
	Sex            int            `json:"sex" db:"sex"`
	Headimgurl     string         `json:"headimgurl" db:"headimgurl"`
	SubscribeTime  string         `json:"subscribe_time" db:"subscribe_time"`
	Remark         string         `json:"remark" db:"remark"`
	Groupid        int            `json:"groupid" db:"groupid"`
	TagidList      pq.StringArray `json:"tagid_list" db:"tagid_list"`
	SubscribeScene string         `json:"subscribe_scene" db:"subscribe_scene"`
	QRscene        int            `json:"qr_scene" db:"qr_scene"`
	QRsceneStr     string         `json:"qr_scene_str" db:"qr_scene_str"`
	LoginErrTimes  int            `json:"login_err_times" db:"login_err_times"`
	LastLoginTime  string         `json:"last_login_time" db:"last_login_time"`
	LastLoginIP    string         `json:"last_login_ip" db:"last_login_ip"`
	SessionKey     string         `json:"session_key" db:"session_key"`
	Status         int            `json:"status" db:"status"`
	ErrCode        int            `json:"errcode"`
	ErrMsg         string         `json:"errmsg"`
}

func (j *jsonapi) httpWXMsgReturn(w http.ResponseWriter, req *http.Request) {

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	//fmt.Println("weixin return msg:", result)

	r := &WXRespone{}

	err := jsonextra.Unmarshal(result, r)

	if err != nil {
		log.Println("wx msg resp decode err:", err)
	}

	if r.ErrCode != 0 {
		log.Println("weixin msg resp err:", r.ErrMsg)

	}

}

// WXBody aa
type WXBody struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Openid       string `json:"openid"`
	Scope        string `json:"scope"`
}

// func GetAccessToken() string {

// 	return conf.WeiXin.WeiXinAccessToken.AccessToken

// }

func getToken() error {

	//log.Println("get weixin token start:", conf.WeiXin.SchName, conf.WeiXin.Name)

	if conf.WeiXin.AppID == "" || conf.WeiXin.AppSecret == "" {

		return errors.New("get weixin token err  appid or appsecret is null")
	}

	wxbody, err := GetWeixinAccessToken(conf.WeiXin.AppID, conf.WeiXin.AppSecret)

	if err != nil {
		log.Println("get weixin token err 1:", err)

	}

	conf.WeiXin.WeiXinAccessToken = wxbody
	conf.WeiXin.AesKey = EncodingAESKey2AESKey(conf.WeiXin.EncodingAESKey)

	//log.Println("get weixin token end:", conf.WeiXin.SchName, conf.WeiXin.Name)

	return nil

}

func GetAllWeixinAccessToken() {

	err := getToken()
	if err != nil {
		log.Println("weixin accessToken get err:", err)
	}
}

func setMenu() error {

	AccessToken := conf.WeiXin.WeiXinAccessToken.AccessToken

	if AccessToken == "" {
		return fmt.Errorf("set weixin menu err:  weiXinAccessToken %v is null ", conf.WeiXin.WeiXinAccessToken)

	}

	if conf.WeiXin.WeiXinMenu == "" {
		return fmt.Errorf("set weixin menu err:  weixinmenu %v is null", conf.WeiXin.WeiXinMenu)
	}

	err := setWeixinMenu(AccessToken, conf.WeiXin.WeiXinMenu)
	if err != nil {
		log.Println("set weixin menu err 1:", err)

	}

	return nil

}

func SetAllWeixinMenu() {

	err := setMenu()
	if err != nil {
		log.Println("weixin menu set err:", err)

	}

}

func GetWXTemplateAllList() error {

	AccessToken := conf.WeiXin.WeiXinAccessToken.AccessToken

	if AccessToken == "" {
		return fmt.Errorf("get weixin Template list err:   accessToken  is null")
	}

	list, err := GetWXTemplateList(AccessToken)
	if err != nil {
		return fmt.Errorf("get weixin Template list err:   accessToken  is null")
	}

	//fmt.Println(list)

	for _, v := range list.TemplateList {
		log.Println(v.Title, v.Deputy_industry, "ID:", v.TemplateID)
		switch {

		case v.Title == "账号登录成功提醒" && v.Deputy_industry != "培训":
			conf.WeiXin.TypeLoginSuccessID = v.TemplateID

		case v.Title == "账号登录失败通知" && v.Deputy_industry != "培训":
			conf.WeiXin.TypeLoginFailID = v.TemplateID

		}

	}

	getTypeMsgID()

	return nil

}

func GetWeiXinTypeTemplateAllList() {

	err := GetWXTemplateAllList()
	if err != nil {
		log.Println("GetWeiXinTypeTemplateAllList err:", err)

	}
}

func getTypeMsgID() error {

	AccessToken := conf.WeiXin.WeiXinAccessToken.AccessToken

	if AccessToken == "" {
		return fmt.Errorf("get weixin getTypeMsgID list err:  accessToken  is null")
	}

	var err error

	if conf.WeiXin.TypeLoginSuccessID == "" {
		conf.WeiXin.TypeLoginSuccessID, err = GetWXTemplateID(AccessToken, wxMsgLoginSucessData)
		if err != nil {
			log.Println("get weixin TypeLoginSuccessID err:", err)
		} else {
			log.Println("get weixin TypeLoginSuccessID ok:", conf.WeiXin.TypeLoginSuccessID)
		}

	}

	if conf.WeiXin.TypeLoginFailID == "" {
		conf.WeiXin.TypeLoginFailID, err = GetWXTemplateID(AccessToken, wxMsgLoginFailData)
		if err != nil {
			log.Println("get weixin TypeLoginFailID err:", err)
		} else {
			log.Println("get weixin TypeLoginFailID ok:", conf.WeiXin.TypeLoginFailID)
		}

	}

	return nil

}

func GetWeiXinTypeMsgID() {
	err := getTypeMsgID()
	if err != nil {
		log.Println("weixin get type msg id get err:", err)

	}

}

func cronGetWxToken() {

	for {

		time.Sleep(time.Minute * 60)

		GetAllWeixinAccessToken()

	}

}

func (j *jsonapi) httpMPuserLogin(w http.ResponseWriter, req *http.Request) {

	req.ParseForm()

	code := strings.TrimSpace(req.Form.Get("code"))

	log.Println("微信登录：", req.Header.Get("X-Forwarded-For"), code)

	req.Body.Close()

	wxresp, err := getMpsession(code, conf.WeiXin.MpAppid, conf.WeiXin.MpAppSecret)

	if err != nil {

		writeJSONResponse(w, &Response{20001, "微信腾讯服务器登录失败", err})
		return
	}

	user := &wxUserInfo{
		MPopenID:   wxresp.OpenID,
		Unionid:    wxresp.Unionid,
		SessionKey: wxresp.SessionKey,
		ErrCode:    wxresp.ErrCode,
		ErrMsg:     wxresp.ErrMsg,
	}

	//fmt.Println(wxresp.OpenID, wxresp.SessionKey, wxresp.Unionid, wxresp.ErrCode, wxresp.ErrMsg)

	user2, err := TeacherMPuserLogin(wxresp)

	if err != nil {

		log.Println("微信老师登录错误，本地没查询到信息", err)

	} else {

		user.OpenID = user2.OpenID
		user.Name = user2.Name
		user.Phone = user2.Phone
		user.MasterSchname = user2.MasterSchname
		user.Schname = user2.Schname
		user.SchoolName = user2.SchoolName
		user.NickName = user2.NickName
		user.Headimgurl = user2.Headimgurl
		user.CRMToken, err = GenerateToken(user.Phone, []string{""})

		if err != nil {
			log.Println("微信登录失败，生成访问令牌失败", err)
			writeJSONResponse(w, &Response{20001, "微信登录失败", err})
			return
		}
	}

	writeJSONResponse(w, &Response{20000, "ok", user})

}

type wxjscode struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	Unionid    string `json:"unionid"`
	ErrMsg     string `json:"errmsg"`
	ErrCode    int    `json:"errcode"`
}

func getMpsession(code, appid, secret string) (*wxjscode, error) {

	//client := &http.Client{}
	// content, err := jsonextra.Marshal(data)
	// fmt.Println("token:"+weiXinAccessToken+"\n", string(content))

	stb := &wxjscode{}

	url := strings.Join([]string{`https://api.weixin.qq.com/sns/jscode2session?appid=`, appid, "&secret=", secret, "&js_code=", code, "&grant_type=authorization_code"}, "")

	resp, err := http.Get(url)

	if err != nil {
		fmt.Println("get jscode2session error", err)

		return stb, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	//fmt.Println(string(body))

	err = jsonextra.Unmarshal(body, &stb)

	if err != nil {
		log.Println("decode jscode2session jscode err", err)
		return stb, err
	}
	//fmt.Println(stb)

	// usermap[stb.OpenID] = stb.SessionKey

	return stb, nil

}

func setWeixinMenu(AccessToken, WeiXinMenu string) error {

	client := &http.Client{}

	url := strings.Join([]string{`https://api.weixin.qq.com/cgi-bin/menu/create`, "?access_token=", AccessToken}, "")
	// content, err := jsonextra.Marshal(data)
	// fmt.Println("token:"+weiXinAccessToken+"\n", string(content))
	postReq, err := http.NewRequest("POST", url, bytes.NewReader([]byte(WeiXinMenu)))
	if err != nil {

		return err
	}
	defer postReq.Body.Close()

	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(postReq)

	if err != nil {
		log.Println(err)
		return err

	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {

		return err
	}

	r := &WXRespone{}

	err = jsonextra.Unmarshal(body, r)

	if err != nil {

		return err
	}

	if r.ErrCode != 0 {

		return errors.New(r.ErrMsg)

	}

	return nil

	//log.Println("weixin menu set ok")

}

func getWeixinUserInfo(openid string) (*wxUserInfo, error) {

	client := &http.Client{}
	// content, err := jsonextra.Marshal(data)
	// fmt.Println("token:"+weiXinAccessToken+"\n", string(content))
	postReq, err := http.NewRequest("POST",
		strings.Join([]string{`https://api.weixin.qq.com/cgi-bin/user/info`, "?access_token=", conf.WeiXin.WeiXinAccessToken.AccessToken, "&openid=", openid}, ""),
		bytes.NewReader([]byte("")))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if err != nil {
		return nil, err
	}

	defer postReq.Body.Close()

	resp, err := client.Do(postReq)
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		//log.Println("weixin user info get error", err)
		return nil, err
	}

	// fmt.Println(string(body))

	user := &wxUserInfo{}
	err = jsonextra.Unmarshal(body, &user)

	if err != nil {
		log.Println("decode wxuserinfo  err :", err, "\n", body)
		return nil, err

		//	return user
	}

	if user.ErrCode != 0 {
		return nil, errors.New(user.ErrMsg)
	}
	return user, nil

}

// GetWeixinAccessToken get
func GetWeixinAccessToken(appid string, appsecret string) (*WXBody, error) {

	URL := strings.Join([]string{"https://api.weixin.qq.com/cgi-bin/token",
		"?grant_type=", "client_credential",
		"&appid=", appid,
		"&secret=", appsecret}, "")

	infoBody, err := http.Get(URL)
	if err != nil {
		return nil, err
	}
	defer infoBody.Body.Close()

	info, err := io.ReadAll(infoBody.Body)
	if err != nil {
		return nil, err
	}

	atr := &WXBody{}

	err = jsonextra.Unmarshal(info, &atr)
	if err != nil {
		return nil, err
	}

	if atr.AccessToken == "" {
		log.Println(string(info))
		return nil, errors.New("获取微信访问令牌失败")
	}

	return atr, nil

}

type CustomServiceMsg struct {
	ToUser  string         `json:"touser"`
	MsgType string         `json:"msgtype"`
	Text    TextMsgContent `json:"text"`
}

type TextMsgContent struct {
	Content string `json:"content"`
}

func pushCustomMsg(accessToken, toUser, msg string) error {

	csMsg := &CustomServiceMsg{
		ToUser:  toUser,
		MsgType: "text",
		Text:    TextMsgContent{Content: msg},
	}

	body, err := json.MarshalIndent(csMsg, " ", "  ")
	if err != nil {
		return err
	}
	//fmt.Println(string(body))

	postReq, err := http.NewRequest("POST",
		strings.Join([]string{"https://api.weixin.qq.com/cgi-bin/message/custom/send?access_token=", accessToken}, ""),
		bytes.NewReader(body))
	if err != nil {
		return err
	}

	postReq.Header.Set("Content-Type", "application/json; encoding=utf-8")

	client := &http.Client{}
	resp, err := client.Do(postReq)
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}

func getwxmsglist(where, page string) (items []TextRequestBody, total int) {

	items = []TextRequestBody{}

	query := fmt.Sprintf(`SELECT * 	FROM wxmsg %v  ORDER BY timestamp DESC %v`, where, page)

	_, err := db.Exec(query)

	if err != nil {
		log.Println("查询微信消息列表错误: ", err)
		return nil, 0

	}

	q := fmt.Sprintf("SELECT count(*) as total from wxmsg %v ", where)
	_, err = db.Exec(q)
	if err != nil {
		log.Println(" 查询微信消息total错误 err:", err)
		return nil, 0
	}
	//fmt.Println(emp)
	return items, total

}

func addwxmsg(e *TextRequestBody) error {

	user, err := getUserInfoByOpenid(e.FromUserName)

	// fmt.Println("user:", user, err)

	if err != nil || user.Schname == "" {
		// 无法找到绑定信息的微信用户，
		user = &wxUserInfo{}
	}

	query := fmt.Sprintf(`INSERT INTO %v.wxmsg 
	(to_user_name,from_user_name,create_time,
	msg_type,event,event_key,url,pic_url,media_id,thumb_media_id,content,
	msg_id,location_x,location_y,label,timestamp) 
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,now())`, user.Schname)

	_, err = db.Exec(query, e.ToUserName, e.FromUserName, e.CreateTime,
		e.MsgType, e.Event, e.EventKey, e.URL, e.PicURL, e.MediaID, e.ThumbMediaID, e.Content,
		e.MsgID, e.LocationX, e.LocationY, e.Label)

	if err != nil {
		log.Println("录入微信消息失败: ", err, query)
		return err
	}
	return nil

}

func SendWeixinMs(accessToken string, data interface{}, userid int) {
	resp, err := SendWeixin(accessToken, data)

	if err != nil {
		log.Println("weixin msg err:", err)
		return
	}

	if resp != nil && resp.ErrCode != 0 {

		content, _ := jsonextra.Marshal(data)

		switch resp.ErrCode {
		case 43003, 43004, 43101:
			if userid != 0 {
				err = unsetphonecode(userid)
				if err != nil {
					log.Println("取消绑定错误：", userid, err)
				}
			}

		case 40241:

		default:
			log.Println("weixin msg err:", resp.ErrCode, resp.ErrMsg, userid, string(content))

		}

	}

}

// SendWeixinMs  send
func SendWeixin(accessToken string, data interface{}) (*WXRespone, error) {
	client := &http.Client{}
	content, err := jsonextra.Marshal(data)

	if err != nil {
		log.Println("code wxmsg err:", err)
		return nil, err
	}

	// fmt.Println(string(content))
	//	fmt.Println("token:"+accessToken+"\n", string(content))
	postReq, err := http.NewRequest("POST",
		strings.Join([]string{`https://api.weixin.qq.com/cgi-bin/message/template/send`, "?access_token=", accessToken}, ""),
		bytes.NewReader(content))

	if err != nil {
		log.Println("wxmsg http req err:", err)
		return nil, err
	}

	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(postReq)
	if err != nil {
		log.Println("get wxmsg http resp  err:", err)
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("get wxmsg http resp body err:", err)
		return nil, err
	}
	defer postReq.Body.Close()

	r := &WXRespone{}

	err = jsonextra.Unmarshal(body, &r)

	if err != nil {
		log.Println("weixin resp decode err: ", err)
		return nil, err

	}

	return r, nil
}

// 小程序中心后端访问平台绑定
func (j *jsonapi) httpMPPhoneCode(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	e := &MPUserInfo{}
	err := jsonextra.Unmarshal(result, &e)

	if err != nil {
		log.Println("MPphonecode decode err :", err)

		//writeJSONResponse(w, &Response{20001, "绑定微信失败", nil})
		w.Write(ResParmErr)
		return
	}
	//检查密钥
	if e.AccessKey != "wxphonecodekey" {
		w.Write(ResParmErr)
		return
	}

	//需要有 master_schname
	openid, err := getOpenID(e.PhoneCode)

	if err != nil {
		//log.Println("phonecode get wxuserinfo  err :", err, e.ID, e.Name, u.CurrentSchoolName)
		writeJSONResponse(w, &Response{20001, "绑定失败,可能是绑定码超时或者微信系统故障，请重新获取", nil})
		return
	}

	if len(openid) < 10 {

		log.Println("phonecode get wxuserinfo  err openid len  10 :", err, openid)
		writeJSONResponse(w, &Response{20001, "小程序绑定失败，请重新关注后绑定再试", nil})
		return

	}

	user, err := getUserInfoByOpenidForBind(openid)
	if err != nil {
		log.Println("phonecode get wxuserinfo  err openid len  10 :", err, openid)
		writeJSONResponse(w, &Response{20001, "小程序绑定失败，用户未提前绑定公众号", nil})
		return

	}

	if conf.WeiXin.WeiXinAccessToken.AccessToken == "" {
		log.Println("没有微信访问令牌：", err, user.Schname)
		writeJSONResponse(w, &Response{20001, "微信绑定失败，公众号未配置", nil})
		return
	}

	sendPhoneCodeOkTypeMsg(openid, e.Name, e.Phone)

	//	go sendHolidayMsg(s, c, u.Name+" 手工", u)

	writeJSONResponse(w, &Response{20000, "小程序绑定成功", user})

}

// func (j *jsonapi) htt
func (j *jsonapi) httpPhoneCode(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	_, err := checktoken(w, req)
	if err != nil {
		return
	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	e := &phonecode{}
	err = jsonextra.Unmarshal(result, &e)

	if err != nil {
		log.Println("phonecode decode err :", err)

		//writeJSONResponse(w, &Response{20001, "绑定微信失败", nil})
		w.Write(ResParmErr)
		return
	}

	//需要有 master_schname
	user, err := BindWeiXin(e)

	if err != nil {
		//log.Println("phonecode get wxuserinfo  err :", err, e.ID, e.Name, u.CurrentSchoolName)
		writeJSONResponse(w, &Response{20001, "绑定微信失败,可能是绑定码超时或者微信系统故障，请重新获取", nil})
		return
	}

	if len(user.OpenID) < 10 {

		log.Println("phonecode get wxuserinfo  err openid len  10 :", err, user.OpenID)
		writeJSONResponse(w, &Response{20001, "绑定微信失败，请重新关注后绑定再试", nil})
		return

	}

	err = updateweixininfo(*user, e.ID)
	if err != nil {
		log.Println("phonecode get wxuserinfo  err :", err, e.ID, e.Name)
		writeJSONResponse(w, &Response{20001, "绑定微信失败", nil})
		return
	}
	if conf.WeiXin.WeiXinAccessToken.AccessToken == "" {
		log.Println("发送绑定成功消息错误, 没有微信访问令牌：", err)
		writeJSONResponse(w, &Response{20001, "微信绑定失败，公众号未配置", nil})
		return
	}

	sendPhoneCodeOkTypeMsg(user.OpenID, e.Name, e.Phone)

	//	go sendHolidayMsg(s, c, u.Name+" 手工", u)

	writeJSONResponse(w, &Response{20000, "微信绑定成功", nil})

}
func (j *jsonapi) httpgetWeiXinMsg(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	_, err := checktoken(w, req)
	if err != nil {
		return
	}

	result, _ := io.ReadAll(req.Body)

	req.Body.Close()

	//list := make([]*wxmsg, 0)

	stb := &query{}
	err = jsonextra.Unmarshal(result, &stb)

	if err != nil {
		log.Println("wxmsg query by openid list err :", err)

		w.Write(ResParmErr)
		return
	}

	req.Body.Close()

	ww, pp, _ := queryToWhere("", *stb)
	list, total := getwxmsglist(ww, pp)

	writeJSONResponseItems(w, list, total)

}

func (j *jsonapi) httpGetWeiXinMsgContent(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	_, err := checktoken(w, req)
	if err != nil {
		return
	}

	req.Body.Close()

	req.ParseForm()

	//	key := strings.TrimSpace(req.Form.Get("access_token"))
	// schname := strings.TrimSpace(req.Form.Get("schname"))
	mediaID := strings.TrimSpace(req.Form.Get("media_id"))
	mediaType := strings.TrimSpace(req.Form.Get("media_type"))

	var URL string

	//fmt.Println(u.ID, u.Name, u.Phone, u.SchName, sch)

	if mediaType == "video" {
		URL = strings.Join([]string{`http://api.weixin.qq.com/cgi-bin/media/get?`, "access_token=", conf.WeiXin.WeiXinAccessToken.AccessToken, "&media_id=", mediaID}, "")
	} else {

		// https://api.weixin.qq.com/cgi-bin/media/get?access_token=ACCESS_TOKEN&media_id=MEDIA_ID
		URL = strings.Join([]string{`https://api.weixin.qq.com/cgi-bin/media/get?`, "access_token=", conf.WeiXin.WeiXinAccessToken.AccessToken, "&media_id=", mediaID}, "")
	}

	//fmt.Println(URL)

	res, err := http.Get(URL)
	if err != nil {
		log.Println("get weixin msg content err:", err)

		writeJSONResponse(w, &Response{20001, "获取微信消息内容失败,微信服务器错误", nil})
		return

	}
	defer res.Body.Close()

	for key, value := range res.Header {
		for _, v := range value {
			w.Header().Add(key, v)
		}
	}

	w.WriteHeader(res.StatusCode)
	io.Copy(w, res.Body)

}

func StudentMPuserLogin(resp *wxjscode) (*wxUserInfo, error) {

	user, err := getUserInfoByUnionid(resp.Unionid)

	if err != nil {
		return nil, fmt.Errorf("请先关注公众号，并联系管理员绑定账号后，再登陆小程序")

	}

	_, err = db.Exec(`UPDATE  wxuser set last_login_time=now(),session_key=? where mpopenid = ?`, resp.SessionKey, resp.OpenID)

	if err != nil {
		log.Println("update wxuser err", err)
		return nil, fmt.Errorf("%v", "出错啦，请联系学校!")

	}

	return user, nil

}
