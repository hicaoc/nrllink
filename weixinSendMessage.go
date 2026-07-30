package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type WxTemplateMsg struct {
	ToUser      string `json:"touser"`      //OPENID
	TemplateID  string `json:"template_id"` //模板ID
	URL         string `json:"url"`
	MiniProgram struct {
		AppID    string `json:"appid"`
		PagePath string `json:"pagepath"`
	} `json:"miniprogram"`
	//ClientMsgID string      `json:"client_msg_id"`
	Data interface{} `json:"data"`
}

type typeMsgValue struct {
	Value string `json:"value"`
}

type getMsgID struct {
	TemplateIDShort string   `json:"template_id_short"`
	KeywordNameList []string `json:"keyword_name_list"`
}

// WxMsgBind  绑定成功通知
type WxMsgBind struct {
	Name       typeMsgValue `json:"thing1"`        //姓名
	Phone      typeMsgValue `json:"phone_number2"` //手机号码
	Time       typeMsgValue `json:"time3"`         //绑定时间
	SchoolName typeMsgValue `json:"thing4"`        //机构名称
}

var WxMsgBindData = getMsgID{TemplateIDShort: "51372",
	KeywordNameList: []string{"姓名", "手机号码", "绑定时间", "机构名称"}}

// WxMsgRefund 43944 退款到账通知
type WxMsgRefund struct {
	Name     typeMsgValue `json:"thing8"`  //学员姓名
	DataTime typeMsgValue `json:"time2"`   //时间
	Amount   typeMsgValue `json:"amount3"` //退款金额

}

// 账号登录成功提醒  66620
type wxMsgLoginSucess struct {
	Time    typeMsgValue `json:"time1"`
	Address typeMsgValue `json:"thing2"`
}

var wxMsgLoginSucessData = getMsgID{TemplateIDShort: "66620",
	KeywordNameList: []string{"登录时间", "登录地址"}}

// 账号登录失败通知  66622
type wxMsgLoginFail struct {
	Time    typeMsgValue `json:"time1"`
	Address typeMsgValue `json:"thing2"`
	Reason  typeMsgValue `json:"const3"`
}

var wxMsgLoginFailData = getMsgID{TemplateIDShort: "66622",
	KeywordNameList: []string{"登录时间", "登录地址", "失败原因"}}

func trimRuneString(s string, size int) string {

	str := []rune(s)
	if len(str) > size {
		return string(str[:size])
	}
	return s

}

func sendPhoneCodeOkTypeMsg(openid, name, phone string) {

	sm := &WxMsgBind{
		typeMsgValue{name},
		typeMsgValue{phone},
		typeMsgValue{time.Now().Format("2006-01-02 15:04:05")},
		typeMsgValue{trimRuneString(conf.SystemInfo.PlatformName, 20)}}

	tm1 := &WxTemplateMsg{ToUser: openid,
		TemplateID: conf.WeiXin.TypePhoneCodeID,
		//ClientMsgID: time.Now().String(),
		Data: sm}

	SendWeixinMs(conf.WeiXin.WeiXinAccessToken.AccessToken, tm1, 0)

}

func SendLoginSucessTypeMsg(ipaddr string, employee *userinfo) {

	sm := &wxMsgLoginSucess{
		typeMsgValue{time.Now().Format("2006-01-02 15:04:05")},
		typeMsgValue{ipaddr},
	}
	//url := fmt.Sprintf("%v/#/evaluation?openid=%v&schname=%v&signlog_id=%v&student_id=%v", conf.System.ServerURL, stu.OpenID, sch.SchName, eva.SignlogID, eva.StudentID)
	tm1 := &WxTemplateMsg{
		ToUser:     employee.OpenID,
		TemplateID: conf.WeiXin.TypeLoginSuccessID,
		//ClientMsgID: time.Now().String(),
		Data: sm}

	SendWeixinMs(conf.WeiXin.WeiXinAccessToken.AccessToken, tm1, 0)

}

func SendLoginFailTypeMsg(ipaddr, Reason string, emp *userinfo) {

	sm := &wxMsgLoginFail{
		typeMsgValue{time.Now().Format("2006-01-02 15:04:05")},
		typeMsgValue{ipaddr},
		typeMsgValue{},
		//typeMsgValue{Reason},
	}
	//url := fmt.Sprintf("%v/#/evaluation?openid=%v&schname=%v&signlog_id=%v&student_id=%v", conf.System.ServerURL, stu.OpenID, sch.SchName, eva.SignlogID, eva.StudentID)
	tm1 := &WxTemplateMsg{
		ToUser:     emp.OpenID,
		TemplateID: conf.WeiXin.TypeLoginFailID,
		//ClientMsgID: time.Now().String(),
		Data: sm}

	SendWeixinMs(conf.WeiXin.WeiXinAccessToken.AccessToken, tm1, 0)

}

// SendWeixinMs  send
func GetWXTemplateID(accessToken string, data getMsgID) (string, error) {
	client := &http.Client{}
	content, err := jsonextra.Marshal(data)

	if err != nil {
		log.Println("code wxmsg err:", err)
		return "", err
	}
	//	fmt.Println("token:"+accessToken+"\n", string(content))
	postReq, err := http.NewRequest("POST",
		strings.Join([]string{`https://api.weixin.qq.com/cgi-bin/template/api_add_template`, "?access_token=", accessToken}, ""),
		bytes.NewReader(content))

	if err != nil {
		log.Println("wxmsg get template id  req err:", err)
		return "", err
	}

	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(postReq)
	if err != nil {
		log.Println("get wxmsg template id   resp  err:", err)
		return "", err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("get wxmsg   template id  body err:", err)
		return "", err
	}
	defer postReq.Body.Close()

	r := &WXRespone{}

	err = jsonextra.Unmarshal(body, &r)

	if err != nil {
		log.Println("weixin template id decode err: ", err)
		return "", err

	}

	if r.ErrCode != 0 {
		log.Println("weixin resp template id err:", r.ErrCode, r.ErrMsg)
		return "", errors.New(r.ErrMsg)
	}

	return r.TemplateID, nil
}

type WxTemplate struct {
	TemplateID      string `json:"template_id"`
	Title           string `json:"title"`
	PrimaryIndustry string `json:"primary_industry"`
	Deputy_industry string `json:"deputy_industry"`
	Content         string `json:"content"`
	Example         string `json:"example"`
}

type WXTemplateList struct {
	TemplateList []WxTemplate `json:"template_list"`
}

func GetWXTemplateList(accessToken string) (*WXTemplateList, error) {

	//	fmt.Println("token:"+accessToken+"\n", string(content))
	resp, err := http.Get(strings.Join([]string{`https://api.weixin.qq.com/cgi-bin/template/get_all_private_template`, "?access_token=", accessToken}, ""))

	if err != nil {
		log.Println("wxmsg get template list err:", err)
		return nil, err
	}

	//resp.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("get wxmsg   template list  body err:", err)
		return nil, err
	}

	r := &WXTemplateList{}

	err = jsonextra.Unmarshal(body, &r)

	if err != nil {
		log.Println("weixin template list decode err: ", err)
		return nil, err

	}

	return r, nil
}

func DelWXTemplate(AccessToken, templateID string) error {

	client := &http.Client{}

	url := strings.Join([]string{`https://api.weixin.qq.com/cgi-bin/template/del_private_template`, "?access_token=", AccessToken}, "")
	// content, err := jsonextra.Marshal(data)
	// fmt.Println("token:"+weiXinAccessToken+"\n", string(content))
	postReq, err := http.NewRequest("POST", url, strings.NewReader(fmt.Sprintf(` { "template_id" : "%s" } `, templateID)))
	if err != nil {

		return err
	}

	//fmt.Println(fmt.Sprintf(` { "template_id" : "%s" } `, templateID))
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

}
