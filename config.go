package main

import (
	"bufio"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type config struct {
	udpport string

	wwwpath  string
	wwwport  string
	logpath  string
	topnpath string

	platformName  string
	NameShorthand string
	logourl       string
	icp           string
	serversslcrt  string
	serversslkey  string

	//weixinmsg  bool
	dbhost     string
	dbport     int
	dbuser     string
	dbpassword string
	dbname     string
	// signmsgurl     string
	// rechargemsgurl string
	phonecodeurl string
	avatarurl    string
	// accessToken  string
	serverurl string //本机api url地址

	weixinAPIURL string //微信URL接口地址
	wxmsgurl     string //微信模板消息url
	tokenkey     string
	AlarmModeID  string //告警通知模板ID

	//points  int
}

var conf = &config{}

func (c *config) init() {

	conf.readconffile()

	//	go c.cronread()

}

func (c *config) readconffile() {

	log.Println("read config file udphub.conf ......")

	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}

	f, err := os.Open(dir + "/udphub.conf")
	if err != nil {
		log.Println("open gameparser.conf file err:", err)
		// 		fmt.Println(`
		// 			wwwpath ./www/
		// 			wwwport 9001
		// 			logpath ./log/
		// `)
		os.Exit(1)
	}
	defer f.Close()

	rd := bufio.NewReader(f)

	for {

		line, err := rd.ReadString('\n')

		if err != nil || io.EOF == err {

			break
		}

		s := strings.SplitN(strings.TrimSuffix(line, "\n"), "=", 2)

		switch s[0] {
		case "udpport":
			c.udpport = s[1]

		case "wwwpath":
			c.wwwpath = s[1]
		case "wwwport":
			c.wwwport = s[1]
		case "logpath":
			c.logpath = s[1]
		case "topnpath":
			c.topnpath = s[1]

		case "platformname":
			c.platformName = s[1]
		case "nameshorthand":
			c.NameShorthand = s[1]
		case "logourl":
			c.logourl = s[1]
		case "icp":
			c.icp = s[1]

		case "serversslcrt":
			c.serversslcrt = s[1]
		case "serversslkey":
			c.serversslkey = s[1]
		case "dbhost":
			c.dbhost = s[1]
		case "dbport":
			port, err := strconv.Atoi(s[1])
			if err != nil {
				log.Println("read conf err ,dbport err")
				os.Exit(1)
			}
			c.dbport = port
		case "dbuser":
			c.dbuser = s[1]
		case "dbpassword":
			c.dbpassword = s[1]
		case "dbname":
			c.dbname = s[1]
			// case "signmsgurl":
			// 	c.signmsgurl = s[1]
			// case "rechargemsgurl":
			// 	c.rechargemsgurl = s[1]
		case "avatarurl":
			c.avatarurl = s[1]
		case "phonecodeurl":
			c.phonecodeurl = s[1]
		case "serverurl":
			c.serverurl = s[1]
		case "wxmsgurl":
			c.wxmsgurl = s[1]

		// case "accessToken":
		// 	c.accessToken = s[1]
		case "weixinAPIURL":
			c.weixinAPIURL = s[1]
		case "tokenkey":
			c.tokenkey = s[1]

		}

	}

	log.Println("read conf file ok ", c)

}

//Exist 判断文件存在
func Exist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
	// || os.IsExist(err)
}
