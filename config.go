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

type config struct {
	System struct {
		Port    string `yaml:"Port" json:"port"`
		Logpath string `yaml:"LogPath" json:"log_path"`
		DBfile  string `yaml:"DBfile" json:"dbfile"`
	} `yaml:"System" json:"system"`

	Web struct {
		Path     string `yaml:"Path" json:"path"`
		Port     string `yaml:"Port" json:"port"`
		TokenKey string `yaml:"Token_Key" json:"token_key"`
		ICP      string `yaml:"ICP" json:"icp"`
		SSLCrt   string `yaml:"SSLCrt" json:"ssl_crt"`
		SSLKey   string `yaml:"SSLKey" json:"ssl_key"`
	} `yaml:"Web" json:"web" `

	SystemInfo struct {
		PlatformName  string `yaml:"Name" json:"name"`
		NameShorthand string `yaml:"NameShorthand" json:"nameshorthand"`
		LogoURL       string `yaml:"LogoURL" json:"logo_url"`
	} `yaml:"SystemInfo" json:"systeminfo"`

	WeiXin struct {
		// signmsgurl     string
		// rechargemsgurl string
		PhoneCodeURL string `yaml:"PhoneCodeURL" json:"phone_code_url"`
		AvatarURL    string `yaml:"AvatarURL" json:"avatar_url"`
		// accessToken  string
		ServerURL string `yaml:"ServerURL" json:"server_url"` //本机api url地址

		WeixinAPIURL string `yaml:"WeixinAPIURL" json:"weixin_api_url"` //微信URL接口地址
		Wxmsgurl     string `yaml:"WxMsgURL" json:"wx_msg_url"`         //微信模板消息url

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
		if *oo == "json" {
			j, _ := jsonextra.MarshalIndent(conf, "", "    ")
			fmt.Println(string(j))
		} else if *oo == "yaml" {
			j, _ := yaml.Marshal(conf)
			fmt.Println(string(j))

		}
		os.Exit(0)
	}

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
