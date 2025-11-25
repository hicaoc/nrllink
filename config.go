package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	yaml "gopkg.in/yaml.v3"
)

var PlatformList []Platformitem

type Platformitem struct {
	Name   string `yaml:"Name" json:"name"` // 对应 YAML 和 JSON 的 name 字段
	Host   string `yaml:"Host" json:"host"` // 对应 YAML 和 JSON 的 host 字段
	Port   string `yaml:"Port" json:"port"` // 对应 YAML 和 JSON 的 port 字段
	Online int    `yaml:"Online" json:"online"`
	Total  int    `yaml:"Total" json:"total"`
}

type config struct {
	System struct {
		Port        string `yaml:"Port" json:"port"`
		Logpath     string `yaml:"LogPath" json:"log_path"`
		LicensePath string `yaml:"LicensePath" json:"license_path"`
		DBfile      string `yaml:"DBfile" json:"dbfile"`
		IPfile      string `yaml:"IPfile" json:"ipfile"`
		CallLogPath string `yaml:"CallLogPath" json:"calllog_path"`
	} `yaml:"System" json:"system"`

	Web struct {
		Path string `yaml:"Path" json:"path"`
		Port string `yaml:"Port" json:"port"`
		//TokenKey string `yaml:"Token_Key" json:"token_key"`
		ICP    string `yaml:"ICP" json:"icp"`
		SSLCrt string `yaml:"SSLCrt" json:"ssl_crt"`
		SSLKey string `yaml:"SSLKey" json:"ssl_key"`
	} `yaml:"Web" json:"web" `

	PlatformList []Platformitem `yaml:"PlatformList" json:"platforms"`

	SystemInfo struct {
		PlatformName  string `yaml:"Name" json:"name"`
		NameShorthand string `yaml:"NameShorthand" json:"nameshorthand"`
		LogoURL       string `yaml:"LogoURL" json:"logo_url"`
		Language      string `yaml:"Language" json:"language"`
	} `yaml:"SystemInfo" json:"systeminfo"`

	OpenAI struct {
		BaseURL string `yaml:"BaseURL" json:"base_url"`
		APIKEY  string `yaml:"APIKEY" json:"api_key"`
		Engine  string `yaml:"Engine" json:"engine"`
	} `yaml:"OpenAI" json:"openai"`

	APRS struct {
		APRSServerHost string `yaml:"APRSServerHost" json:"aprs_server_host"`
		APRSServerPort string `yaml:"APRSServerPort" json:"aprs_server_port"`
		SelfAddress    string `yaml:"SelfAddress" json:"self_address"`
		SelfPort       string `yaml:"SelfPort" json:"self_port"`

		CallSign string `yaml:"CallSign" json:"callsign"`
		SSID     string `yaml:"SSID" json:"ssid"`
		Passcode int    `yaml:"Passcode" json:"passcode"`

		Latitude  float64 `yaml:"Latitude" json:"latitude"`
		Longitude float64 `yaml:"Longitude" json:"longitude"`
		Altitude  string  `yaml:"Altitude" json:"altitude"`
	} `yaml:"APRS" json:"aprs"`

	WeiXin struct {
		// signmsgurl     string
		// rechargemsgurl string
		MpAppid           string `yaml:"MpAppid" json:"mp_appid"`
		MpAppSecret       string `yaml:"MpAppSecret" json:"mp_appsecret"`
		PhoneCodeURL      string `yaml:"PhoneCodeURL" json:"phone_code_url"`
		AvatarURL         string `yaml:"AvatarURL" json:"avatar_url"`
		AccessToken       string
		AppID             string `yaml:"AppID" json:"appid"`
		AppSecret         string `yaml:"AppSecret" json:"appsecret"`
		EncodingAESKey    string `yaml:"EncodingAESKey" json:"encodingaeskey"`
		AesKey            []byte
		ClickEventMap     map[string]string `yaml:"ClickEventMap" json:"click_event_map"`
		KeywordsMap       map[string]string `yaml:"KeywordsMap" json:"keywords_map"`
		WeixinWelcome     string            `yaml:"WeixinWelcome" json:"weixin_welcome"`
		DefaultKeywords   string            `yaml:"DefaultKeywords" json:"default_keywords"`
		WeiXinAccessToken *WXBody
		WeiXinMenu        string `yaml:"WeixinMenu" json:"weixin_menu"`
		ServerURL         string `yaml:"ServerURL" json:"server_url"` //本机api url地址

		WeixinAPIURL       string `yaml:"WeixinAPIURL" json:"weixin_api_url"` //微信URL接口地址
		Wxmsgurl           string `yaml:"WxMsgURL" json:"wx_msg_url"`         //微信模板消息url
		TypePhoneCodeID    string
		TypeLoginSuccessID string

		TypeLoginFailID string

		AlarmModeID string `yaml:"AlarmModeID" json:"alarm_mode_id"` //告警通知模板ID

	} `yaml:"WeiXin" json:"weixin"`

	//points  int
}

var conf = &config{}

func (c *config) init() {

	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Printf("get config filepath err #%v ", err)
		os.Exit(1)
	}

	confpath := dir + "/udphub.yaml"

	cc := flag.String("c", confpath, "config file path and name")
	oo := flag.String("o", "", "print config content to stdout and exit , yaml or json format")

	flag.Parse()

	if *cc != "" {
		confpath = *cc
	}

	yamlFile, err := os.ReadFile(confpath)

	if err != nil {
		log.Printf("udphub.yaml open err #%v ", err)
		os.Exit(1)

	}
	err = yaml.Unmarshal(yamlFile, conf)

	if err != nil {
		log.Fatalf("Unmarshal: %v \n %s", err, yamlFile)
	}

	// c.Parm.iDCfilterIPMap = make(map[uint32]bool, 0)
	// for _, v := range c.Parm.IDCfilterIP {
	// 	c.Parm.iDCfilterIPMap[ipstrToUInt32(v)] = true
	// }

	if *oo != "" {
		switch *oo {
		case "json":
			j, _ := jsonextra.MarshalIndent(conf, "", "    ")
			fmt.Println(string(j))
		case "yaml":
			j, _ := yaml.Marshal(conf)
			fmt.Println(string(j))

		}
		os.Exit(0)
	}

	PlatformList = conf.PlatformList

}

// Exist 判断文件存在
func Exist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
	// || os.IsExist(err)
}

var db *sql.DB

func getDB() *sql.DB {

	var err error

	db, err = sql.Open("sqlite3", conf.System.DBfile)

	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	return db
}

func updatedb() {
	sqlStatements := []string{
		//"ALTER TABLE devices ADD COLUMN callsign TEXT DEFAULT '';",
		"ALTER TABLE devices ADD COLUMN priority INTEGER DEFAULT 100;",
		//"ALTER TABLE public_groups ADD COLUMN allow_callsign_ssid TEXT DEFAULT '';",
		//"CREATE UNIQUE INDEX idx_ssid_callsign ON devices (ssid, callsign);",
		//"CREATE UNIQUE INDEX idx_name_unique ON public_groups(name);",

		"DELETE FROM users WHERE id NOT IN ( SELECT MIN(id) FROM users GROUP BY phone );",
		"DROP INDEX idx_phone_unique ;",
		"DROP INDEX idx_callsign_unique",
		"CREATE UNIQUE INDEX idx_users_phone_unique ON users(phone);",
		"CREATE UNIQUE INDEX idx_users_callsign_unique ON users(callsign);",
	}

	// 逐条执行 SQL 语句并输出日志
	for _, stmt := range sqlStatements {
		log.Printf("Executing SQL: %s\n", stmt) // 输出当前执行的 SQL
		_, err := db.Exec(stmt)
		if err != nil {
			// 如果执行出错，打印错误日志
			log.Printf("Error executing statement: %s\nError: %v\n", stmt, err)
		} else {
			// 如果执行成功，打印成功日志
			log.Printf("Successfully executed: %s\n", stmt)
		}
	}
}
