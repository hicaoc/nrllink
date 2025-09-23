package main

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

type Aprs struct {
	Status    string
	Timer     *time.Ticker
	tcpClient *TCPClient
	errorChan chan error
}

func NewAPRS() *Aprs {
	return &Aprs{}
}

func (a *Aprs) OnLoad() {
	if conf.APRS.APRSServerHost == "" || conf.APRS.APRSServerPort == "" || conf.APRS.Longitude == 0 || conf.APRS.CallSign == "" {
		fmt.Println("APRS:arps启动失败，请检查APRS配置")
		return
	}

	a.errorChan = make(chan error, 1)

	for {

		a.tcpClient = NewTCPClient(conf.APRS.APRSServerHost, conf.APRS.APRSServerPort, a.handleTcpMessage)
		a.tcpClient.Connect()

		a.Login()

		time.Sleep(time.Second * 5)

		// 重新启动 ticker
		a.startLocationWatch()

		err := <-a.errorChan

		a.tcpClient.Close()

		time.Sleep(time.Second * 5)
		fmt.Printf("APRS 发送错误，重新初始化TCP连接: %v\n", err)

	}

}

func (a *Aprs) OnUnload() {
	// 清除定时器
	if a.Timer != nil {
		a.Timer.Stop()
	}
	// 关闭TCP连接
	if a.tcpClient != nil {
		a.tcpClient.Close()
	}
}

func (a *Aprs) startLocationWatch() {
	// 模拟获取位置信息
	// 在实际应用中，这里应该使用适当的位置服务API

	a.sendAprsPosition()

	// 启动定时发送（每分钟一次）
	a.Timer = time.NewTicker(60 * time.Second)
	go func() {
		for range a.Timer.C {
			err := a.sendAprsPosition()
			if err != nil {
				a.errorChan <- fmt.Errorf("发送APRS位置失败: %v", err)
				fmt.Printf("发送APRS位置失败: %v\n", err)
				return // ✅ 主动退出 goroutine
			}
		}
	}()

}

func (a *Aprs) Login() {

	//认证
	passcode := conf.APRS.Passcode

	if conf.APRS.Passcode == 0 {
		passcode = GenerateAPRSPasscode(conf.APRS.CallSign)
	}

	for {
		err := a.tcpClient.Send(fmt.Sprintf("user %s pass %d vers NRL 1.0\n", conf.APRS.CallSign, passcode))

		if err != nil {
			fmt.Printf("APRS:认证失败: %v\n", err)
			time.Sleep(time.Second * 10)
			continue
		} else {
			fmt.Println("APRS:认证成功")
			return
		}

	}

}

func (a *Aprs) sendAprsPosition() error {

	// 构造APRS数据包
	aprsPacket := a.formatAprsPacket(conf.APRS.CallSign, conf.APRS.SSID, conf.APRS.SelfAddress, conf.APRS.SelfPort,
		conf.APRS.Longitude, conf.APRS.Latitude, conf.APRS.Altitude)

	// 发送数据
	err := a.tcpClient.Send(aprsPacket)
	fmt.Printf("APRS:发送APRS位置: %s", aprsPacket)

	if err != nil {
		fmt.Printf("APRS:发送APRS位置失败: %v\n", err)
		a.Status = "发送失败"
		return err
	} else {
		a.Status = "位置已发送"
	}

	aprsPacket2 := a.formatAprsPackettwo(conf.SystemInfo.PlatformName, conf.APRS.CallSign, conf.APRS.SSID,
		totalstats.OnlineDevNumber, len(devCallsignSSIDMap))

	err = a.tcpClient.Send(aprsPacket2)
	fmt.Printf("APRS:发送附加信息: %s", aprsPacket2)

	if err != nil {
		fmt.Printf("APRS:发送附加信息失败: %v\n", err)
		a.Status = "发送失败"
		return err
	} else {
		a.Status = "位置已发送"
	}
	return nil

}

func (a *Aprs) formatAprsPacket(callSign, ssid, address, port string, lat, lon float64, altitude string) string {
	latStr := a.decToAprs(lat, true)
	lonStr := a.decToAprs(lon, false)

	// return fmt.Sprintf("%s-%s>NRLSRV,TCPIP*:!%s/%sI000/000/A=%s @udp://%s:%s,NRL互联服务器\n",
	// 	callSign, ssid, latStr, lonStr, altitude, address, port)

	return fmt.Sprintf("%s-%s>NRLSRV,TCPIP*:!%s/%sI @udp://%s:%s,NRL互联服务器\n",
		callSign, ssid, latStr, lonStr, address, port)
}

func (a *Aprs) formatAprsPackettwo(name, callSign, ssid string, onlineNumber, total int) string {

	return fmt.Sprintf("%s-%s>NRLSRV,TCPIP*:>在线:%d,高峰:%d,%s\n",
		callSign, ssid, onlineNumber, total, name)
}

func (a *Aprs) handleTcpMessage(message []byte) {
	fmt.Printf("APRS:收到TCP消息: %s\n", bytes.TrimSpace(message))
	a.Status = "收到服务器响应"

	// 2秒后清除状态
	time.AfterFunc(2*time.Second, func() {
		a.Status = ""
	})
}

func (a *Aprs) decToAprs(dec float64, isLat bool) string {
	dir := ""
	if dec >= 0 {
		if isLat {
			dir = "N"
		} else {
			dir = "E"
		}
	} else {
		if isLat {
			dir = "S"
		} else {
			dir = "W"
		}
	}

	dec = abs(dec)
	deg := int(dec)
	min := (dec - float64(deg)) * 60

	return fmt.Sprintf("%02d%05.2f%s", deg, min, dir)
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func GenerateAPRSPasscode(callsign string) int {
	parts := strings.Split(callsign, "-")
	callsign = strings.ToUpper(parts[0])

	passcode := 29666
	i := 0
	for i < len(callsign) {
		passcode ^= int(callsign[i]) * 256
		if i+1 < len(callsign) {
			passcode ^= int(callsign[i+1])
		}
		i += 2
	}
	passcode &= 32767
	return passcode
}
