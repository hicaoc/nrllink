package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type APRSTV struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data []struct {
		Scall  string   `json:"scall"`
		Call   string   `json:"call"`
		Ssid   string   `json:"ssid"`
		Tm     string   `json:"tm"`
		Lat    string   `json:"lat"`
		Lon    string   `json:"lon"`
		Alt    float64  `json:"alt"`
		Stable string   `json:"stable"`
		Symbol string   `json:"symbol"`
		Rotate int      `json:"rotate"`
		Fmt    string   `json:"fmt"`
		Speed  float64  `json:"speed"`
		Course float64  `json:"course"`
		Power  *float64 `json:"power"`
		//Antenna *string  `json:"antenna"`
		Gain  *float64 `json:"gain"`
		Msg   string   `json:"msg"`
		Path  string   `json:"path"`
		State string   `json:"state"`
		Dev   *string  `json:"dev"`
	} `json:"data"`
}

type ApiServer struct {
	Name string `json:"name"`
	Host string `json:"host"`
	Port string `json:"port"`
	Ower string `json:"ower"`
}

func NewAPRSTV() *APRSTV {
	return &APRSTV{}
}

func (a *APRSTV) decodeMsg(str string) (host, port string, err error) {

	s1 := strings.Split(strings.TrimSpace(str)[1:], ",")

	parsedURL, err := url.Parse(s1[0])
	if err != nil {
		//fmt.Println("Error parsing URL:", err, s1)
		return "", "", err
	}
	//fmt.Println("decodeMsg:", parsedURL.String())
	host = parsedURL.Hostname()
	port = parsedURL.Port()

	return

}

func (a *APRSTV) decodeState(str string) (online int, total int, name string, err error) {
	s1 := strings.Split(str, ",")

	if len(s1) != 3 {
		//fmt.Println("Error parsing state:", str)
		return 0, 0, "", errors.New("Error parsing state")
	}

	s2 := strings.Split(s1[0], ":")
	s3 := strings.Split(s1[1], ":")
	online, _ = strconv.Atoi(s2[1])
	total, _ = strconv.Atoi(s3[1])
	name = s1[2]
	return

}

type NRLStats struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data []struct {
		Type  string `json:"type"`
		Total int    `json:"total"`
	} `json:"data"`
}

func (a *APRSTV) GetNRL() {
	apiURL := "https://aprs.tv/api/findnrl"

	// 构造查询参数
	params := url.Values{}
	params.Add("dest", "NRLSRV")
	params.Add("duration", "60") // 可选，不加则默认全部

	// 拼接完整 URL
	uri, err := url.Parse(apiURL)
	if err != nil {
		fmt.Println("Error parsing URL:", err)
		conf.PlatformList = PlatformList
		return
	}
	uri.RawQuery = params.Encode() // 自动编码

	// 发起 POST 请求（无 body）
	resp, err := http.Post(uri.String(), "", nil)
	if err != nil {
		fmt.Println("Request failed:", err)
		conf.PlatformList = PlatformList
		return
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		conf.PlatformList = PlatformList
		return
	}

	//fmt.Println("Response Body:", string(body))

	// 解析JSON响应
	var apiResponse APRSTV
	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		fmt.Println("Error unmarshaling JSON response:", err)
		conf.PlatformList = PlatformList
		return
	}

	conf.PlatformList = []Platformitem{}

	totalstats.PlatformDevOnline = 0
	totalstats.PlatformDevTotal = 0

	if len(apiResponse.Data) == 0 {
		conf.PlatformList = PlatformList
		return
	}

	// 用于按 host:port 去重，防止同一服务器多条beacon导致重复转发
	seen := make(map[string]bool)

	// 打印解析后的数据
	for _, item := range apiResponse.Data {
		host, port, err := apiResponse.decodeMsg(item.Msg)
		if err != nil {
			//fmt.Println("Error decoding message:", err)
			continue
		}
		online, total, name, err := apiResponse.decodeState(item.State)
		if err != nil {
			//fmt.Println("Error decoding state:", err)
			continue
		}

		// 按 host:port 去重
		hostPort := host + ":" + port
		if seen[hostPort] {
			continue
		}
		seen[hostPort] = true

		targetAddr, err := net.ResolveUDPAddr("udp", hostPort)
		if err != nil {
			log.Println("APRS: host or port err：", err)
		}

		// 过滤自身，使用 SelfAddress 匹配
		if host == conf.APRS.SelfAddress {
			targetAddr = nil
		}

		p := Platformitem{
			Host:    host,
			Port:    port,
			Name:    name,
			Online:  online,
			Total:   total,
			udpAddr: targetAddr,
		}

		totalstats.PlatformDevOnline = totalstats.PlatformDevOnline + online
		totalstats.PlatformDevTotal = totalstats.PlatformDevTotal + total

		conf.PlatformList = append(conf.PlatformList, p)

		//fmt.Printf("name:%s, Ower:%s, host:%s, port:%s, online:%d, total:%d,\n", name, item.Scall, host, port, online, total)
	}

}

func (a *APRSTV) GetNRLStat() {
	apiURL := "https://aprs.tv/api/findnrltotal"

	// 构造查询参数
	params := url.Values{}
	//params.Add("dest", "NRLSRV")
	params.Add("duration", "60") // 可选，不加则默认全部

	// 拼接完整 URL
	uri, err := url.Parse(apiURL)
	if err != nil {
		fmt.Println("Error parsing URL:", err)
		return
	}
	uri.RawQuery = params.Encode() // 自动编码

	// 发起 POST 请求（无 body）
	resp, err := http.Post(uri.String(), "", nil)
	if err != nil {
		fmt.Println("Request failed:", err)
		return
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}

	//fmt.Println("Response Body:", string(body))

	// 解析JSON响应
	var apiResponse NRLStats
	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		fmt.Println("Error unmarshaling JSON response:", err)
		return
	}

	// 打印解析后的数据
	for _, item := range apiResponse.Data {

		switch item.Type {
		case "NRLSRV":
			totalstats.PlatformServerTotal = item.Total
		case "NRLBOX":
			totalstats.PlatformBoxTotal = item.Total
		case "NRLAPP":
			totalstats.PlatformAppTotal = item.Total
		case "NRLMP":
			totalstats.PlatformMPTotal = item.Total
		}
		//fmt.Printf("%s,%d\n", item.Type, item.Total)
	}
}

func findNRL() {

	aprstv := NewAPRSTV()

	// 仅保留统计信息同步，平台列表由 startPlatformServerSync() 负责维护。
	Timer := time.NewTicker(60 * time.Second)

	for range Timer.C {
		aprstv.GetNRLStat()
	}

}
