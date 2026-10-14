package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	jsoniter "github.com/json-iterator/go"

	"golang.org/x/net/websocket"
)

var jsonextra = jsoniter.ConfigCompatibleWithStandardLibrary

type jsonapi struct {
}

var jsonhttp = &jsonapi{}

func (j *jsonapi) init() {

	//getyouzhantoken()

	j.msghttp()

}

type platforminfo struct {
	Name     string `json:"name"`
	LogoURL  string `json:"logourl"`
	Version  string `json:"version"`
	ICP      string `json:"icp"`
	Mail     string `json:"mail"`
	Callsign string `json:"callsign"`
	Language string `json:"language"`
}

// 与 Docker 镜像 tag 保持一致（发布时同步更新）
const version = "v2.13.15"

var totalstats = totalStats{}

type totalStats struct {
	DevNumber           int `json:"dev_number"`
	OnlineDevNumber     int `json:"online_dev_number"`
	UserNumber          int `json:"user_number"`
	VoiceTime           int `json:"voice_time"`
	Traffic             int `json:"traffic"`
	PacketNumber        int `json:"packet_number"`
	SessionNumber       int `json:"session_number"`
	MsgNumber           int `json:"msg_number"`
	LostPercent         int `json:"lost_percent"`
	PlatformDevOnline   int `json:"platform_dev_online"`
	PlatformDevTotal    int `json:"platform_dev_total"`
	PlatformServerTotal int `json:"platform_server_total"`
	PlatformBoxTotal    int `json:"platform_box_total"`
	PlatformAppTotal    int `json:"platform_app_total"`
	PlatformMPTotal     int `json:"platform_mptotal"`
}

func (j *jsonapi) httpTotalStats(w http.ResponseWriter, req *http.Request) {

	totalstats.DevNumber = len(devCallsignSSIDMap)
	totalstats.OnlineDevNumber = currentOnlineDeviceCount()
	totalstats.UserNumber = 1000
	//totalstats.UserNumber = len(userlist)

	rescode, _ := jsonextra.Marshal(totalstats)

	respone := fmt.Sprintf(`{"code":20000,"data":{"items":%s}}`, rescode)

	w.Write([]byte(respone))
}

func (j *jsonapi) httpplatforminfo(w http.ResponseWriter, req *http.Request) {

	p := platforminfo{
		Name:     conf.SystemInfo.PlatformName,
		LogoURL:  conf.SystemInfo.LogoURL,
		Language: conf.SystemInfo.Language,
		Version:  version,
		ICP:      conf.Web.ICP,
		Mail:     "caoc@live.com",
		Callsign: "BH4RPN",
	}

	rescode, _ := jsonextra.Marshal(p)

	respone := fmt.Sprintf(`{"code":20000,"data":{"items":%s}}`, rescode)

	w.Write([]byte(respone))
}

func (j *jsonapi) httpplatformList(w http.ResponseWriter, req *http.Request) {

	rescode, _ := jsonextra.Marshal(conf.PlatformList)

	respone := fmt.Sprintf(`{"code":20000,"data":{"items":%s}}`, rescode)

	w.Write([]byte(respone))
}

func sethttphead(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")             //允许访问所有域
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type") //header的类型
	w.Header().Add("Access-Control-Allow-Headers", "x-token")
	w.Header().Add("Access-Control-Allow-Headers", "Authorization")
	w.Header().Set("content-type", "application/json") //返回数据格式是json

}

func allowWebsocketHandshake(config *websocket.Config, req *http.Request) error {
	return nil
}

func (j *jsonapi) msghttp() {

	router := mux.NewRouter()

	router.HandleFunc("/api/msg/weixin", j.httpWXMsg)               //微信公众号接口
	router.HandleFunc("/weixinreturn/msgstatus", j.httpWXMsgReturn) //微信回调
	router.HandleFunc("/weixin/phonecode", j.httpPhoneCode)         //crm自己处理操作员的过来的绑定请求
	router.HandleFunc("/weixin/mpphonecode", j.httpMPPhoneCode)     //crm自己处理微信小程序后端的过来的绑定请求
	router.HandleFunc("/weixin/wxmsg", j.httpgetWeiXinMsg)          //查询微信用户发过来的消息
	router.HandleFunc("/api/getwxmsg", j.httpGetWeiXinMsgContent)

	//小程序登录
	router.HandleFunc("/api/weixin/wxlogin/teacher", j.httpMPuserLogin)

	http.HandleFunc("/platform/info", j.httpplatforminfo)
	http.HandleFunc("/platform/list", j.httpplatformList)
	http.HandleFunc("/platform/totalstats", j.httpTotalStats)

	http.HandleFunc("/device/list", j.httpDeviceList)
	http.HandleFunc("/device/db/list", j.httpDevicesList)

	http.HandleFunc("/device/get", j.httpDevice)
	http.HandleFunc("/device/qthmap", j.httpDeviceQTHs)
	http.HandleFunc("/device/qth", j.httpDeviceQTHs)
	http.HandleFunc("/device/qths", j.httpDeviceQTHs2)
	http.HandleFunc("/device/qth2", j.httpDeviceQTH)

	http.HandleFunc("/device/mydevlist", j.httpMyDeviceList)
	// http.HandleFunc("/device/binddevice", j.httpBindDevice)
	http.HandleFunc("/device/update", j.httpUpdateDevice)
	http.HandleFunc("/device/delete", j.httpDeleteDevice)

	http.HandleFunc("/device/changegroupnrl", j.httpChangeDeviceGroupNRL)

	http.HandleFunc("/device/query", j.httpQueryDeviceParm)
	http.HandleFunc("/device/change", j.httpChangeDeviceParm)
	http.HandleFunc("/device/change1w", j.httpChange1W)
	http.HandleFunc("/device/change2w", j.httpChange2W)
	http.HandleFunc("/device/at", j.httpDeviceAT)

	http.HandleFunc("/group/get", j.httpGetGroup)

	http.HandleFunc("/group/list/mini", j.httpGetGroupList)

	http.HandleFunc("/group/list", j.httpPublicGroupList)
	http.HandleFunc("/group/device/list", j.httpGroupDeviceList)
	http.HandleFunc("/group/create", j.httpAddGroup)
	http.HandleFunc("/group/update", j.httpUpdateGroup)
	http.HandleFunc("/group/delete", j.httpDeleteGroup)

	http.HandleFunc("/group/listnrl", j.httpAllGroupListNRL)

	http.HandleFunc("/room/list", j.httpRoomList)

	http.HandleFunc("/server/list", j.httpServersList)
	http.HandleFunc("/server/create", j.httpAddServer)
	http.HandleFunc("/server/update", j.httpUpdateServer)
	http.HandleFunc("/server/delete", j.httpDeleteServer)

	http.HandleFunc("/relay/list", j.httpGetRelayList)
	http.HandleFunc("/relay/create", j.httpAddrelay)
	http.HandleFunc("/relay/update", j.httpUpdaterelay)
	http.HandleFunc("/relay/delete", j.httpDeleterelay)

	// http.HandleFunc("/device/create", j.httpAddDevice)
	// http.HandleFunc("/device/update", j.httpUpdateDevice)
	// http.HandleFunc("/device/delete", j.httpDeleteDevice)
	http.HandleFunc("/user/reg/create", j.httpRegister)
	http.HandleFunc("/user/reg/add", j.httpAddReg)
	http.HandleFunc("/user/reg/update", j.httpUpdateReg)
	http.HandleFunc("/user/reg/list", j.httpRegisterList)
	http.HandleFunc("/user/reg/image/get", j.httpRegisterImage)
	http.HandleFunc("/user/reg/delete", j.httpDeleteRegUser)

	http.HandleFunc("/user/login", j.httpUserLogin)
	http.HandleFunc("/user/oidc/config", j.httpOIDCConfig)     //OIDC登录配置查询（公开）
	http.HandleFunc("/user/oidc/login", j.httpOIDCLogin)       //发起OIDC登录
	http.HandleFunc("/user/oidc/callback", j.httpOIDCCallback) //OIDC登录回调
	http.HandleFunc("/user/info", j.httpUserInfo)
	http.HandleFunc("/user/logout", j.httpoplogout)

	http.HandleFunc("/user/alllist", j.httpUserAllList)
	http.HandleFunc("/user/list", j.httpUserList)
	http.HandleFunc("/user/userlistbyrole", j.httpUserListbyRole)
	http.HandleFunc("/user/detail", j.httpUserDetail)
	http.HandleFunc("/user/create", j.httpAddUser)
	http.HandleFunc("/user/update", j.httpUpdateUser)
	http.HandleFunc("/user/profile/update", j.httpUpdateUserProfile)
	http.HandleFunc("/user/update/avatar", j.httpUpdateUserAvatar)

	http.HandleFunc("/user/mdcid", j.httpGetMDCID)
	http.HandleFunc("/user/dmrid", j.httpGetDMRID)

	http.HandleFunc("/user/password", j.httpUpdateUserPassword)
	http.HandleFunc("/user/delete", j.httpDeleteUser)
	http.HandleFunc("/user/unlock", j.httpUnlockUser)

	http.HandleFunc("/billing/info", j.httpBillingInfo)
	http.HandleFunc("/billing/packages/list", j.httpBillingPackages)
	http.HandleFunc("/billing/packages/create", j.httpBillingPackageCreate)
	http.HandleFunc("/billing/packages/update", j.httpBillingPackageUpdate)
	http.HandleFunc("/billing/packages/delete", j.httpBillingPackageDelete)
	http.HandleFunc("/billing/order/create", j.httpBillingOrderCreate)
	http.HandleFunc("/billing/order/query", j.httpBillingOrderQuery)
	http.HandleFunc("/billing/wechat/notify", j.httpBillingWechatNotify)

	//http.HandleFunc("/routes", j.httpRoutes)
	http.HandleFunc("/roles/list", j.httpGetRoles)
	http.HandleFunc("/roles/create", j.httpRole)
	http.HandleFunc("/roles/routes", j.httpGetRoutes)
	http.HandleFunc("/roles/updateroutes", j.httpSetRoutes)

	//http.HandleFunc("/area/wxuserlist", j.httpGetWeiXinUserList)
	http.HandleFunc("/operatorlog/list", j.httpOperatorLogList)

	http.HandleFunc("/config/get", j.httpGetConfig)
	http.HandleFunc("/config/update", j.httpUpdateConfig)

	//http.HandleFunc("/login", j.httplogin)
	//http.HandleFunc("/reg", j.httpreg)

	// /ws echo 调试接口已移除：无鉴权、断连后自旋耗尽 CPU，且连接 map 无锁并发写会崩进程
	http.Handle("/ws/calls", websocket.Server{
		Handler:   websocket.Handler(j.wsCallStream),
		Handshake: allowWebsocketHandshake,
	})

	http.Handle("/", http.FileServer(http.Dir(conf.Web.Path)))

	if conf.Web.SSLCrt != "" && conf.Web.SSLKey != "" {

		err := http.ListenAndServeTLS(":"+conf.Web.Port, conf.Web.SSLCrt, conf.Web.SSLKey, nil)
		if err != nil {
			log.Println("http server start err :", err)
		}

	} else {

		err := http.ListenAndServe(":"+conf.Web.Port, nil)

		if err != nil {
			log.Println("http server start err :", err)
		}

		//err := http.ListenAndServeTLS(":"+conf.wwwport, "server.crt", "server.key", nil)

	}

	log.Println("http server on port ", conf.Web.Port)

}

type query struct {
	ID       string `json:"id"`
	User     string `json:"user"`
	Callsign string `json:"callsign"`
	SSID     uint8  `json:"ssid"`

	CountryName string `json:"country_name"`
	RegionName  string `json:"region_name"`
	ISPDomain   string `json:"isp_domain"`
	AppID       string `json:"appid"`

	GroupID  string `json:"group_id"`
	DeviceID int    `json:"device_id"`

	AreaID        string   `json:"areaid"`
	QueryType     string   `json:"querytype"`
	PhoneDistinct bool     `json:"phone_distinct"`
	OperatorID    string   `json:"operator_id"`
	Schname       string   `json:"schname"`
	Name          string   `json:"name"`
	IP            string   `json:"ip"`
	NamePhone     string   `json:"namephone"`
	Phone         string   `json:"phone"`
	Date          string   `json:"date"`
	Role          string   `json:"role"`
	Month         string   `json:"month"`
	Daterange     []string `json:"daterange"`
	UpdateTime    []string `json:"update_time"`
	FollowTime    string   `json:"follow_time"`
	CurrentArea   string   `json:"current_area"`
	Area          string   `json:"area"`
	Type          string   `json:"type"`
	EventType     string   `json:"event_type"`
	Count         int      `json:"count"`
	Limit         int      `json:"limit"`
	Page          int      `json:"page"`
	Sort          string   `json:"sort"`
	Status        string   `json:"status"`
	NotStatus     string   `json:"note_status"`
	IsOnline      bool     `json:"isonline"`
	IsDeleted     string   `json:"isdeleted"`
	Path          string   `json:"path"`
}

// queryToWhere 把查询条件转换为参数化 WHERE 子句。
// 客户端输入一律通过 ? 占位符绑定，数字字段校验失败时忽略该条件，防止 SQL 注入。
// 返回 WHERE 子句、绑定参数、分页子句、排序子句。
func queryToWhere(subquery string, q query) (string, []any, string, string) {

	var conditions []string
	var args []any

	addCond := func(cond string, a ...any) {
		conditions = append(conditions, cond)
		args = append(args, a...)
	}

	// addNum 仅接受整数，非数字输入直接忽略该条件
	addNum := func(column, value string) {
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return
		}
		addCond(column+"=?", n)
	}

	if q.ID != "" {
		addNum("id", q.ID)
	}

	if q.OperatorID != "" {
		addNum("operator_id", q.OperatorID)
	}

	if q.CurrentArea != "" {
		addNum(subquery+"current_area", q.CurrentArea)
	}

	if q.AreaID != "" {
		addNum("areaid", q.AreaID)
	}

	if q.Area != "" {
		addCond(subquery+"area @> ('{'||?||'}')", q.Area)
	}

	if q.IsDeleted != "" {
		addNum("isdeleted", q.IsDeleted)
	}

	if q.Phone != "" {
		addCond("phone=?", q.Phone)
	}

	if q.Callsign != "" {
		addCond("callsign=?", strings.ToUpper(q.Callsign))
	}

	if q.GroupID != "" {
		addNum("group_id", q.GroupID)
	}

	if q.Role != "" {
		addCond(subquery+"roles like ?", "%"+q.Role+"%")
	}

	if q.Date != "" {
		addCond("date=?", q.Date)
	}

	if q.Type != "" {
		addNum("type", q.Type)
	}

	if q.Status != "" {
		addNum("status", q.Status)
	}

	if q.NotStatus != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(q.NotStatus)); err == nil {
			addCond("status!=?", n)
		}
	}

	if len(q.Daterange) >= 2 {
		addCond("timestamp between ? and ?", q.Daterange[0], q.Daterange[1]+" 23:59:59")
	}

	if q.Month != "" {
		addCond("timestamp=?", q.Month)
	}

	if len(q.UpdateTime) >= 2 {
		addCond("update_time between ? and ?", q.UpdateTime[0], q.UpdateTime[1]+" 23:59:59")
	}

	if q.Name != "" {
		addCond("(name like ?)", "%"+q.Name+"%")
	}

	if q.CountryName != "" {
		addCond("(country_name like ?)", "%"+q.CountryName+"%")
	}

	if q.RegionName != "" {
		addCond("(region_name like ?)", "%"+q.RegionName+"%")
	}

	if q.ISPDomain != "" {
		addCond("(isp_domain like ?)", "%"+q.ISPDomain+"%")
	}

	if q.IP != "" {
		addCond("(cidrip like ?)", "%"+q.IP+"%")
	}

	if q.NamePhone != "" {
		addCond("("+subquery+"name like ? or "+subquery+"phone like ?)",
			"%"+q.NamePhone+"%", "%"+q.NamePhone+"%")
	}

	if q.EventType != "" {
		addCond("("+subquery+"event_type like ?)", "%"+q.EventType+"%")
	}

	if q.Schname != "" {
		addCond("schname=?", q.Schname)
	}

	var s string
	if len(conditions) > 0 {
		s = " where " + strings.Join(conditions, " and ") + " "
	}

	var p string
	if q.Limit > 0 && q.Page > 0 {

		p = fmt.Sprintf(" Limit %v offset %v", q.Limit, (q.Page-1)*q.Limit)
	}

	var sort string
	switch q.Sort {
	case "+id":
		sort = "order by id asc"
	case "-id":
		sort = "order by id desc"
	case "+name":
		sort = "order by name asc"
	case "-name":
		sort = "order by name desc"
	case "+create_time":
		sort = "order by create_time asc"
	case "-create_time":
		sort = "order by create_time desc"
	case "+update_time":
		sort = "order by update_time asc"
	case "-update_time":
		sort = "order by  update_time desc"
	case "+follow_time":
		sort = "order by follow_time asc , id asc "
	case "-follow_time":
		sort = "order by follow_time desc , id desc "

	}

	return s, args, p, sort

}

// func array2strings(status []int) (s string) {
// 	s = "("

// 	for _, v := range status {
// 		s = s + strconv.Itoa(v) + ","
// 	}
// 	s = strings.TrimSuffix(s, ",") + ")"

// 	return s
// }
