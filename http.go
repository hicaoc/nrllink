package main

import (
	"fmt"
	"log"
	"net/http"

	// _ "net/http/pprof"
	// "github.com/jmoiron/sqlx"

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
}

var totalstats = totalStats{}

type totalStats struct {
	DevNumber       int `json:"dev_number"`
	OnlineDevNumber int `json:"online_dev_number"`
	UserNumber      int `json:"user_number"`
	VoiceTime       int `json:"voice_time"`
	Traffic         int `json:"traffic"`
	PacketNumber    int `json:"packet_number"`
	SessionNumber   int `json:"session_number"`
	MsgNumber       int `json:"msg_number"`
	LostPercent     int `json:"lost_percent"`
}

func (j *jsonapi) httpTotalStats(w http.ResponseWriter, req *http.Request) {

	totalstats.DevNumber = len(devCPUIDMap)
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
		Version:  "v1.0.0",
		ICP:      conf.Web.ICP,
		Mail:     "caoc@live.com",
		Callsign: "BH4RPN",
	}

	rescode, _ := jsonextra.Marshal(p)

	respone := fmt.Sprintf(`{"code":20000,"data":{"items":%s}}`, rescode)

	w.Write([]byte(respone))
}

func sethttphead(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")             //允许访问所有域
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type") //header的类型
	w.Header().Add("Access-Control-Allow-Headers", "x-token")
	w.Header().Set("content-type", "application/json") //返回数据格式是json

}

func (j *jsonapi) msghttp() {

	// fs := http.FileServer(http.Dir(conf.topnpath))

	// http.Handle("/topn/files/", http.StripPrefix("/topn/files", fs))

	// http.HandleFunc("/user/totalstats", j.httpUserTotalStats)
	// http.HandleFunc("/user/topnstats", j.httpTopNStats)
	// http.HandleFunc("/user/topnappstats", j.httpTopNAppStats)
	// http.HandleFunc("/user/topnaccountlist", j.httpTopNUserlist)

	// http.HandleFunc("/user/queryuser", j.httpqueryuser)
	// //http.HandleFunc("/user/usertimeline", j.httpUserTimeline)
	// http.HandleFunc("/user/datelist", j.httpUserDataList)

	http.HandleFunc("/platform/info", j.httpplatforminfo)
	http.HandleFunc("/platform/totalstats", j.httpTotalStats)

	http.HandleFunc("/device/list", j.httpDeviceList)

	http.HandleFunc("/device/mydevlist", j.httpMyDeviceList)
	// http.HandleFunc("/device/binddevice", j.httpBindDevice)
	http.HandleFunc("/device/update", j.httpUpdateDevice)

	http.HandleFunc("/device/changegroupnrl", j.httpChangeDeviceGroupNRL)

	http.HandleFunc("/device/query", j.httpQueryDeviceParm)
	http.HandleFunc("/device/change", j.httpChangeDeviceParm)
	http.HandleFunc("/device/change1w", j.httpChange1W)
	http.HandleFunc("/device/change2w", j.httpChange2W)

	http.HandleFunc("/group/list", j.httpPublicGroupList)
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

	http.HandleFunc("/user/login", j.httpUserLogin)
	http.HandleFunc("/user/info", j.httpUserInfo)
	http.HandleFunc("/user/logout", j.httpoplogout)

	http.HandleFunc("/user/alllist", j.httpUserAllList)
	http.HandleFunc("/user/list", j.httpUserList)
	http.HandleFunc("/user/userlistbyrole", j.httpUserListbyRole)
	http.HandleFunc("/user/detail", j.httpUserDetail)
	http.HandleFunc("/user/create", j.httpAddUser)
	http.HandleFunc("/user/update", j.httpUpdateUser)
	http.HandleFunc("/user/password", j.httpUpdateUserPassword)
	http.HandleFunc("/user/delete", j.httpDeleteUser)

	//http.HandleFunc("/routes", j.httpRoutes)
	http.HandleFunc("/roles/list", j.httpGetRoles)
	http.HandleFunc("/roles/create", j.httpRole)
	http.HandleFunc("/roles/routes", j.httpGetRoutes)
	http.HandleFunc("/roles/updateroutes", j.httpSetRoutes)

	//http.HandleFunc("/area/wxuserlist", j.httpGetWeiXinUserList)
	http.HandleFunc("/operatorlog/list", j.httpOperatorLogList)

	//http.HandleFunc("/login", j.httplogin)
	//http.HandleFunc("/reg", j.httpreg)

	http.Handle("/ws", websocket.Handler(upper))

	http.Handle("/", http.FileServer(http.Dir(conf.Web.Path)))

	err := http.ListenAndServe(":"+conf.Web.Port, nil)
	//err := http.ListenAndServeTLS(":"+conf.wwwport, "server.crt", "server.key", nil)

	if err != nil {
		log.Println("http server start err :", err)
	} else {
		log.Println("http server on port ", conf.Web.Port)
	}

}

// type queryparm struct {
// 	Area      string `json:"area"`
// 	QueryType string `json:"querytype"`
// 	AppID     string `json:"appid"`
// 	IPgroupID int    `json:"ipgroup_id"`
// 	BasID     string `json:"basid"`
// 	Date      string `json:"date"`
// }

type query struct {
	ID       string `json:"id"`
	User     string `json:"user"`
	Callsign string `json:"callsign"`

	CountryName string `json:"country_name"`
	RegionName  string `json:"region_name"`
	ISPDomain   string `json:"isp_domain"`
	AppID       string `json:"appid"`

	AreaID        string   `json:"areaid"`
	QueryType     string   `json:"querytype"`
	PhoneDistinct bool     `json:"phone_distinct"`
	QueryString   string   `json:"querys_tring"`
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
	IsDeleted     string   `json:"isdeleted"`
}

func queryToWhere(subquery string, q query) (string, string, string) {

	var s string
	var p string
	var sort string

	// t := reflect.TypeOf(q)
	// v := reflect.ValueOf(q)

	// for i := 0; i < t.NumField(); i++ {

	// 	if v.Field(i).String() != "" && t.Field(i).Name != "Page" && t.Field(i).Name != "Sort" && t.Field(i).Name != "Limit" {
	// 		fmt.Println("s:", s, len(s))
	// 		if s != "" {
	// 			s = s + " and " + t.Field(i).Tag.Get("json") + "='" + v.Field(i).String() + "'"
	// 		} else {
	// 			s = s + " " + t.Field(i).Tag.Get("json") + "='" + v.Field(i).String() + "'"
	// 		}
	// 	}

	// }

	if q.ID != "" {
		s = " id = " + q.ID
	}

	//自定义查询条件
	if q.QueryString != "" {
		if s != "" {
			s = s + " and " + q.QueryString + ""
		} else {
			s = " " + q.QueryString + " "
		}
	}

	if q.OperatorID != "" {
		if s != "" {
			s = s + " and  operator_id=" + q.OperatorID
		} else {
			s = " operator_id=" + q.OperatorID
		}
	}

	if q.CurrentArea != "" {
		if s != "" {
			s = s + " and " + subquery + "current_area=" + q.CurrentArea
		} else {
			s = " " + subquery + "current_area=" + q.CurrentArea
		}
	}

	if q.AreaID != "" {
		if s != "" {
			s = s + " and  areaid=" + q.AreaID
		} else {
			s = " areaid=" + q.AreaID
		}
	}

	if q.Area != "" {
		if s != "" {
			s = s + " and " + subquery + "area @> '{" + q.Area + "}'"
		} else {
			s = " " + subquery + "area @> '{" + q.Area + "}'"
		}
	}

	if q.IsDeleted != "" {
		if s != "" {
			s = s + " and isdeleted=" + q.IsDeleted + ""
		} else {
			s = " isdeleted=" + q.IsDeleted + ""
		}
	}

	if q.Phone != "" {
		if s != "" {
			s = s + " and phone='" + q.Phone + "'"
		} else {
			s = " phone='" + q.Phone + "'"
		}
	}

	if q.Callsign != "" {
		if s != "" {
			s = s + " and callsign='" + q.Callsign + "'"
		} else {
			s = " callsign='" + q.Callsign + "'"
		}
	}

	if q.Role != "" {
		if s != "" {
			s = s + " and " + subquery + "roles like '%" + q.Role + "%'"
		} else {
			s = " " + subquery + "roles like '%" + q.Role + "%'"
		}
	}

	if q.Date != "" {
		if s != "" {
			s = s + " and date='" + q.Date + "'"
		} else {
			s = " date='" + q.Date + "'"
		}
	}

	if q.Type != "" {
		if s != "" {
			s = s + "  and type=" + q.Type
		} else {
			s = " type=" + q.Type
		}
	}

	// if len(q.Status) > 0 {

	// 	if s != "" {
	// 		s = s + "  and status IN " + array2strings(q.Status)
	// 	} else {
	// 		s = " status IN " + array2strings(q.Status)
	// 	}

	// }
	if q.Status != "" {
		if s != "" {
			s = s + "  and status=" + q.Status
		} else {
			s = " status=" + q.Status
		}
	}

	if q.NotStatus != "" {
		if s != "" {
			s = s + "  and status !=" + q.NotStatus
		} else {
			s = " status !=" + q.NotStatus
		}
	}

	if q.Daterange != nil {

		if s != "" {
			s = s + "  and timestamp between '" + q.Daterange[0] + "' and '" + q.Daterange[1] + " 23:59:59'"

		} else {
			s = "  timestamp between '" + q.Daterange[0] + "' and '" + q.Daterange[1] + " 23:59:59'"
		}
	}

	if q.Month != "" {

		if s != "" {
			s = s + "  and  timestamp =  '" + q.Month + "' "

		} else {
			s = "  timestamp =  '" + q.Month + "' "
		}
	}

	if q.UpdateTime != nil {

		if s != "" {
			s = s + "  and update_time between '" + q.UpdateTime[0] + "' and '" + q.UpdateTime[1] + " 23:59:59'"

		} else {
			s = "  update_time between '" + q.UpdateTime[0] + "' and '" + q.UpdateTime[1] + " 23:59:59'"
		}
	}

	if q.Name != "" {
		if s != "" {
			s = s + " and (name like '%" + q.Name + "%' )"
		} else {
			s = " (name like '%" + q.Name + "%')"
		}
	}

	if q.CountryName != "" {
		if s != "" {
			s = s + " and (country_name like '%" + q.CountryName + "%' )"
		} else {
			s = " (country_name like '%" + q.CountryName + "%')"
		}
	}

	if q.RegionName != "" {
		if s != "" {
			s = s + " and (region_name like '%" + q.RegionName + "%' )"
		} else {
			s = " (region_name like '%" + q.RegionName + "%')"
		}
	}

	if q.ISPDomain != "" {
		if s != "" {
			s = s + " and (isp_domain like '%" + q.ISPDomain + "%' )"
		} else {
			s = " (isp_domain like '%" + q.ISPDomain + "%')"
		}
	}

	if q.IP != "" {
		if s != "" {
			s = s + " and (cidrip like '%" + q.IP + "%' )"
		} else {
			s = " (cidrip like '%" + q.IP + "%')"
		}
	}

	if q.NamePhone != "" {
		if s != "" {
			s = s + " and (" + subquery + "name like '%" + q.NamePhone + "%' or " + subquery + "phone like '%" + q.NamePhone + "%')"
		} else {
			s = " (" + subquery + "name like '%" + q.NamePhone + "%' or " + subquery + "phone like '%" + q.NamePhone + "%')"
		}
	}

	if q.EventType != "" {
		if s != "" {
			s = s + " and (" + subquery + "event_type like '%" + q.EventType + "%' )"
		} else {
			s = " (" + subquery + "event_type like '%" + q.EventType + "%' )"
		}
	}

	if q.Schname != "" {
		if s != "" {
			s = s + " and schname='" + q.Schname + "'"
		} else {
			s = " schname='" + q.Schname + "'"
		}
	}

	if s != "" {
		s = " where " + s + " "
	}

	if q.Limit > 0 && q.Page > 0 {

		p = fmt.Sprintf(" Limit %v offset %v", q.Limit, (q.Page-1)*q.Limit)
	}

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

	//	fmt.Println(s, p)
	return s, p, sort

}

// func array2strings(status []int) (s string) {
// 	s = "("

// 	for _, v := range status {
// 		s = s + strconv.Itoa(v) + ","
// 	}
// 	s = strings.TrimSuffix(s, ",") + ")"

// 	return s
// }
