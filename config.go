package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"

	yaml "gopkg.in/yaml.v3"
	_ "modernc.org/sqlite"
)

var PlatformList []Platformitem

type Platformitem struct {
	Name    string `yaml:"Name" json:"name"` // 对应 YAML 和 JSON 的 name 字段
	Host    string `yaml:"Host" json:"host"` // 对应 YAML 和 JSON 的 host 字段
	Port    string `yaml:"Port" json:"port"` // 对应 YAML 和 JSON 的 port 字段
	Online  int    `yaml:"Online" json:"online"`
	Total   int    `yaml:"Total" json:"total"`
	udpAddr *net.UDPAddr
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
		MpAppid           string            `yaml:"MpAppid" json:"mp_appid"`
		MpAppSecret       string            `yaml:"MpAppSecret" json:"mp_appsecret"`
		PhoneCodeURL      string            `yaml:"PhoneCodeURL" json:"phone_code_url"`
		AvatarURL         string            `yaml:"AvatarURL" json:"avatar_url"`
		AccessToken       string            `yaml:"-"`
		AppID             string            `yaml:"AppID" json:"appid"`
		AppSecret         string            `yaml:"AppSecret" json:"appsecret"`
		EncodingAESKey    string            `yaml:"EncodingAESKey" json:"encodingaeskey"`
		AesKey            []byte            `yaml:"-"`
		ClickEventMap     map[string]string `yaml:"ClickEventMap" json:"click_event_map"`
		KeywordsMap       map[string]string `yaml:"KeywordsMap" json:"keywords_map"`
		WeixinWelcome     string            `yaml:"WeixinWelcome" json:"weixin_welcome"`
		DefaultKeywords   string            `yaml:"DefaultKeywords" json:"default_keywords"`
		WeiXinAccessToken *WXBody           `yaml:"-"`
		WeiXinMenu        string            `yaml:"WeixinMenu" json:"weixin_menu"`
		ServerURL         string            `yaml:"ServerURL" json:"server_url"` //本机api url地址

		WeixinAPIURL       string `yaml:"WeixinAPIURL" json:"weixin_api_url"` //微信URL接口地址
		Wxmsgurl           string `yaml:"WxMsgURL" json:"wx_msg_url"`         //微信模板消息url
		TypePhoneCodeID    string `yaml:"-"`
		TypeLoginSuccessID string `yaml:"-"`

		TypeLoginFailID string `yaml:"-"`

		AlarmModeID string `yaml:"AlarmModeID" json:"alarm_mode_id"` //告警通知模板ID

	} `yaml:"WeiXin" json:"weixin"`

	Billing struct {
		Enabled                  bool   `yaml:"Enabled" json:"enabled"`
		AccountExpireRecheckSecs int    `yaml:"AccountExpireRecheckSecs" json:"account_expire_recheck_secs"`
		PackageUnitPriceCents    int    `yaml:"PackageUnitPriceCents" json:"package_unit_price_cents"`
		NotifyURL                string `yaml:"NotifyURL" json:"notify_url"`
		WechatPay                struct {
			AppID          string `yaml:"AppID" json:"appid"`
			MchID          string `yaml:"MchID" json:"mch_id"`
			APIv3Key       string `yaml:"APIv3Key" json:"api_v3_key"`
			SerialNo       string `yaml:"SerialNo" json:"serial_no"`
			PrivateKeyPath string `yaml:"PrivateKeyPath" json:"private_key_path"`
			NotifyURL      string `yaml:"NotifyURL" json:"notify_url"`
			Description    string `yaml:"Description" json:"description"`
		} `yaml:"WechatPay" json:"wechat_pay"`
	} `yaml:"Billing" json:"billing"`

	// OIDC 集中认证登录配置（外部 OIDC Provider，如 https://www.hamptt.com）
	OIDC struct {
		Enabled       bool   `yaml:"enabled" json:"enabled"`               //是否启用 OIDC 登录
		Issuer        string `yaml:"issuer" json:"issuer"`                 //OIDC Provider issuer 地址
		ClientID      string `yaml:"client_id" json:"client_id"`           //OIDC 客户端ID
		ClientSecret  string `yaml:"client_secret" json:"client_secret"`   //OIDC 客户端密钥
		RedirectURL   string `yaml:"redirect_url" json:"redirect_url"`     //OIDC 回调地址，需在 Provider 端登记
		AutoProvision bool   `yaml:"auto_provision" json:"auto_provision"` //找不到本地账号时是否自动建号
		VirtualLogin  bool   `yaml:"virtual_login" json:"virtual_login"`   //找不到本地账号时是否生成无库临时会话
		TokenLogin    bool   `yaml:"token_login" json:"token_login"`       //接受 HAM ID 长期 API Token 登录
		ButtonName    string `yaml:"button_name" json:"button_name"`       //前端登录按钮文案
	} `yaml:"OIDC" json:"oidc"`

	//points  int
}

var conf = &config{}

// confPath 为配置文件路径，在 init() 中确定，save() 持久化时使用
var confPath string

// confLock 保护 conf 的并发读写（Web 配置接口与运行时 goroutine）
var confLock sync.RWMutex

func (c *config) init() {

	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Printf("get config filepath err #%v ", err)
		os.Exit(1)
	}

	confPath = dir + "/udphub.yaml"

	cc := flag.String("c", confPath, "config file path and name")
	oo := flag.String("o", "", "print config content to stdout and exit , yaml or json format")

	flag.Parse()

	if *cc != "" {
		confPath = *cc
	}

	yamlFile, err := os.ReadFile(confPath)

	if err != nil {
		log.Printf("udphub.yaml open err #%v ", err)
		os.Exit(1)

	}
	err = yaml.Unmarshal(yamlFile, conf)

	if err != nil {
		// 主文件损坏时回退到 save() 留下的 .bak 备份
		bak, berr := os.ReadFile(confPath + ".bak")
		if berr != nil {
			log.Fatalf("Unmarshal: %v \n %s", err, yamlFile)
		}
		if berr = yaml.Unmarshal(bak, conf); berr != nil {
			log.Fatalf("Unmarshal: %v \n %s", err, yamlFile)
		}
		log.Printf("config file corrupted, loaded backup %s.bak instead (err #%v)", confPath, err)
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

// save 将当前配置持久化回 yaml 配置文件。
// 先写临时文件再 rename，避免写一半把原文件写坏。
func (c *config) save() error {

	confLock.RLock()
	b, err := yaml.Marshal(c)
	confLock.RUnlock()

	if err != nil {
		return err
	}

	tmp := confPath + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}

	// 覆盖前把现有配置备份到 .bak，写坏或误改时可手动恢复
	if old, err := os.ReadFile(confPath); err == nil {
		if err := os.WriteFile(confPath+".bak", old, 0644); err != nil {
			os.Remove(tmp)
			return fmt.Errorf("backup config: %w", err)
		}
	}

	return os.Rename(tmp, confPath)
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

	db, err = sql.Open("sqlite", conf.System.DBfile)

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
		//"ALTER TABLE devices RENAME COLUMN cpuid TO dmrid;",

		"ALTER TABLE users ADD COLUMN mdcid string DEFAULT '';",

		"ALTER TABLE devices ADD COLUMN dmrid INTEGER DEFAULT 0;",
		"ALTER TABLE users ADD COLUMN dmrid INTEGER DEFAULT 0;",
		"ALTER TABLE users ADD COLUMN expire_time TEXT DEFAULT '';",
		//"update devices set dmrid='';",
		//"ALTER TABLE public_groups ADD COLUMN allow_callsign_ssid TEXT DEFAULT '';",
		//"CREATE UNIQUE INDEX idx_ssid_callsign ON devices (ssid, callsign);",
		//"CREATE UNIQUE INDEX idx_name_unique ON public_groups(name);",

		`CREATE TABLE IF NOT EXISTS billing_packages (
			id INTEGER UNIQUE,
			name TEXT,
			months INTEGER,
			unit_price_cents INTEGER,
			price_cents INTEGER,
			status INTEGER,
			note TEXT,
			create_time TEXT,
			update_time TEXT,
			PRIMARY KEY("id" AUTOINCREMENT)
		);`,
		`CREATE TABLE IF NOT EXISTS billing_orders (
			id INTEGER UNIQUE,
			out_trade_no TEXT UNIQUE,
			user_id INTEGER,
			callsign TEXT,
			package_id INTEGER,
			months INTEGER,
			amount_cents INTEGER,
			status TEXT,
			prepay_id TEXT,
			code_url TEXT,
			transaction_id TEXT,
			payer_openid TEXT,
			paid_at TEXT,
			expire_before TEXT,
			expire_after TEXT,
			raw_notify TEXT,
			create_time TEXT,
			update_time TEXT,
			PRIMARY KEY("id" AUTOINCREMENT)
		);`,

		"DELETE FROM users WHERE id NOT IN ( SELECT MIN(id) FROM users GROUP BY phone );",
		"DROP INDEX idx_phone_unique ;",
		"DROP INDEX idx_callsign_unique",
		"CREATE UNIQUE INDEX idx_users_phone_unique ON users(phone);",
		"CREATE UNIQUE INDEX idx_users_callsign_unique ON users(callsign);",

		//OIDC 集中认证：users 表记录 OIDC sub 用于账号绑定
		"ALTER TABLE users ADD COLUMN oidc_sub TEXT;",
		"CREATE INDEX IF NOT EXISTS idx_users_oidc_sub ON users(oidc_sub);",
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
