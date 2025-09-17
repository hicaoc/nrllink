package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"
)

type TCPClient struct {
	conn      net.Conn
	onMessage func(message string)
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

func NewTCPClient(onMessage func(message string)) *TCPClient {
	return &TCPClient{

		onMessage: onMessage,
	}
}

func (c *TCPClient) Connect() error {

	conn, err := net.Dial("tcp", net.JoinHostPort(conf.APRS.APRSServerHost, conf.APRS.APRSServerPort))
	if err != nil {
		return err
	}
	c.conn = conn

	go c.readMessages()
	return nil
}

func (c *TCPClient) Send(message string) error {
	_, err := c.conn.Write([]byte(message))
	return err
}

func (c *TCPClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *TCPClient) readMessages() {
	reader := bufio.NewReader(c.conn)
	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("APRS:读取TCP消息错误: %v\n", err)
			return
		}
		if c.onMessage != nil {
			c.onMessage(strings.TrimSpace(message))
		}
	}
}

type Aprs struct {
	Status    string
	Timer     *time.Ticker
	tcpClient *TCPClient
}

func NewAPRS() *Aprs {

	return &Aprs{}
}

func (a *Aprs) OnLoad() {
	if conf.APRS.APRSServerHost == "" || conf.APRS.APRSServerPort == "" || conf.APRS.Longitude == 0 || conf.APRS.CallSign == "" {
		fmt.Println("APRS:arps启动失败，请检查APRS配置")
		return
	}

	// 模拟获取用户信息
	// 在实际应用中，这里应该从全局配置或其他地方获取

	// 初始化TCP客户端
	a.tcpClient = NewTCPClient(a.handleTcpMessage)
	err := a.tcpClient.Connect()
	if err != nil {
		fmt.Printf("APRS:TCP连接失败: %v\n", err)
		a.Status = "TCP连接失败"
		return
	}

	// 启动位置监听
	a.startLocationWatch()
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

	passcode := conf.APRS.Passcode

	if conf.APRS.Passcode == 0 {
		passcode = GenerateAPRSPasscode(conf.APRS.CallSign)
	}

	a.tcpClient.Send(fmt.Sprintf("user %s pass %d vers NRL 1.0\n", conf.APRS.CallSign, passcode))

	// 启动定时发送（每分钟一次）
	a.Timer = time.NewTicker(60 * time.Second)
	go func() {
		for range a.Timer.C {
			a.sendAprsPosition()
		}
	}()
}

func (a *Aprs) sendAprsPosition() {

	// 构造APRS数据包
	aprsPacket := a.formatAprsPacket(conf.SystemInfo.PlatformName, conf.APRS.CallSign, conf.APRS.SelfAddress, conf.APRS.SelfPort,
		conf.APRS.Longitude, conf.APRS.Latitude, conf.APRS.Altitude, totalstats.OnlineDevNumber, len(devCallsignSSIDMap))

	// 发送数据
	err := a.tcpClient.Send(aprsPacket)

	fmt.Printf("APRS:发送APRS位置: %s", aprsPacket)

	if err != nil {
		fmt.Printf("APRS:发送APRS位置失败: %v\n", err)
		a.Status = "发送失败"
	} else {
		a.Status = "位置已发送"
	}

}

func (a *Aprs) formatAprsPacket(name, callSign, address, port string, lat, lon float64, altitude string, onlineNumber, total int) string {
	latStr := a.decToAprs(lat, true)
	lonStr := a.decToAprs(lon, false)

	//    return `${callSign}-5>NRL,TCPIP*:!${latStr}/${lonStr}[A${altitude.toFixed(0)} ${deviceModel}@NRL微信小程序\n`;

	return fmt.Sprintf("%s-10>NRLSRV,TCPIP*:!%s/%sI000/000/A=%s @udp://%s:%s,%d,%d,%s,NRL模拟互联服务器\n",
		callSign, latStr, lonStr, altitude, address, port, onlineNumber, total, name)
}

func (a *Aprs) handleTcpMessage(message string) {
	fmt.Printf("APRS:收到TCP消息: %s\n", message)
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
