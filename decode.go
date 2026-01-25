package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

type NRL21packet struct {
	timeStamp  time.Time
	UDPAddrStr string
	UDPAddr    *net.UDPAddr //报文来源UDP地址和端口
	Version    string       //协议标识 “NRL2” 每个报文都以 NRL2 4个字节开头
	Length     uint16       //上层数据长度
	DMRID      string       //设备唯一标识 长度9字节
	Password   string       //密码5个字节
	Type       byte         //上层数据类型 一个字节 0:保留， 1：G.711语音，2：心跳  3：设备配置 4：保留，5. 文本消息，6，设备控制设备， 7，设备要求加入组等指令 9:服务器互联语音,11 AT透传
	Status     byte         //设备状态位
	Count      uint16       //报文计数器2节
	CallSign   string       //所有者呼号 6字节
	SSID       byte         //所有者呼号 1字节
	DevModel   byte         //设备型号

	OriginalCallsign string //原始呼号
	OriginalSSID     uint8  //原始SSID
	OriginalIP       net.IP //原始IP

	DATA []byte //上层数据内容
}

func newNRL21packet(remoteaddr *net.UDPAddr, d []byte) (packet *NRL21packet, err error) {
	packet = &NRL21packet{}

	err = packet.decodeNRL21(d)

	if err != nil {
		return nil, err
	}

	packet.UDPAddr = remoteaddr
	packet.UDPAddrStr = remoteaddr.String()
	packet.timeStamp = time.Now()
	return
}

/*
NRL2 协议规范 (NRL2 Protocol Specification)


报文头部固定长度为 48 字节，大端序 (BigEndian)。

偏移 (Offset) | 长度 (Length) | 字段名 (Field)      | 说明 (Description)
--------------|---------------|---------------------|---------------------------------------------------
0-3           | 4             | Version             | 协议标识，固定为 ASCII "NRL2"
4-5           | 2             | Length              | 报文总长度 (头部+数据)，uint16
6-14          | 9             | DMRID               | 设备DMR标识。decode 取 7+2 字节 [6:15]
15-19         | 5             | Password            | 设备访问密码
20            | 1             | Type                | 数据类型：
              |               |                     | 0: 保留
              |               |                     | 1: G.711 语音 (PCM A-law/u-law)
              |               |                     | 2: 心跳 (Heartbeat)
              |               |                     | 3: 设备配置 (Config)
              |               |                     | 5: 文本消息 (Text Message)
              |               |                     | 6: 设备控制 (Control)
              |               |                     | 7: 组加入指令 (Join Group)
			  |			      |                     | 8: Opus 16K
              |               |                     | 9: 服务器互联语音 (Server Interconnect Voice)
              |               |                     | 11: AT 透传 (AT Passthrough)
21            | 1             | Status              | 设备状态位
22-23         | 2             | Count               | 报文计数器
24-29         | 6             | CallSign            | 所有者呼号，不足 6 位补 0
30            | 1             | SSID                | 所有者 SSID
31            | 1             | DevMode             | 设备型号
32-45         | 16            | Reserved/Extended   | 保留项或扩展内容
46-47         | 2             | CRC                 | 校验码
48-           | -             | DATA                | 上层协议数据负载

// Type 9 的扩展内容
32-37         | 6             | OrigCallsign        | 原始呼号 (仅 Type 9 使用；其余类型作为扩展内容)
38            | 1             | OrigSSID            | 原始 SSID (仅 Type 9 使用；其余类型作为扩展内容)
39-42         | 4             | OrigIP              | 原始 IP 地址 (仅 Type 9 使用；其余类型作为扩展内容)



// 32-48 位的默认扩展内容为暂时保留给会议模式的呼叫者列表
32-47         | 16

*/

//type 5 文本消息子类型规范，使用[]头表达内容的媒体类型,不带方括号的默认为纯文本类型，
/*
[text]text/plain  纯文本消息
[loc]application/location  位置消息，经纬度
[json]application/json  json消息
[xml]application/xml  xml消息
[html]text/html  html消息

//下面几种体积大的类型，须通过oss上传，NRL报文只传输媒体在oss上的链接
[bin]application/octet-stream url
[img]image/jpeg url
[video]video/mp4 url
[audio]audio/mp3 url

*/

//type 8 语音编码格式 opus
/*
Sample rate: 16kHz
Channels: 1
Frame size: 20ms (320 samples @16k)
Bitrate: 32–40 kbps (Default VBR)
Application: OPUS_APPLICATION_VOIP
Complexity: 10
*/

func (n *NRL21packet) decodeNRL21(d []byte) (err error) {

	if len(d) < 48 {
		return errors.New("packet too short ")
	}
	n.Version = string(d[0:4])

	if n.Version != "NRL2" {
		return errors.New("not NRL packet ")
	}

	n.Length = binary.BigEndian.Uint16(d[4:6])

	n.DMRID = string(d[6:15])
	n.Password = string(d[15:20])
	n.Type = d[20]
	n.Status = d[21]
	n.Count = binary.BigEndian.Uint16(d[22:24])
	n.CallSign = string(bytes.TrimRight(d[24:30], string([]byte{13, 0})))

	if !IsCallSign(n.CallSign) {
		return errors.New("callsign error")
	}

	n.SSID = d[30]
	n.DevModel = d[31]

	if n.Type == 9 || n.DevModel == 200 {
		n.OriginalCallsign = string(bytes.TrimRight(d[32:38], string([]byte{13, 0})))
		n.OriginalSSID = d[38]
		n.OriginalIP = d[39:43]
	}

	n.DATA = d[48:]

	return nil

}

func (n *NRL21packet) String() string {
	return fmt.Sprintf("ver:%v len:%v DMIID:%v CallSign:%v-%v type:%v len:%v  Count:%v  %02X ", n.Version, n.Length, n.DMRID, n.CallSign, n.SSID, n.Type, len(n.DATA), n.Count, n.DATA)
}

type G711Voice struct {
	Number uint32
	DATA   []byte
}

// func calculateCpuId(callSign string) []byte {
// 	// 将字符串生成 32 位哈希值
// 	var hash uint32 = 0
// 	for _, char := range callSign {
// 		hash = (hash*31 + uint32(char))
// 	}

// 	cpuIdBytes := make([]byte, 4)
// 	binary.BigEndian.PutUint32(cpuIdBytes[:4], hash)

// 	return cpuIdBytes

// 	// 将哈希值转换为 7 字节的十六进制字符串

// }
var zero6 = make([]byte, 9)

func NRL21SetCallsignSSID(callsign string, ssid byte, packet []byte) []byte {
	copy(packet[24:30], zero6)
	copy(packet[24:30], callsign)

	packet[30] = ssid

	return packet

}

var zero9 = make([]byte, 9)

func NRL21SetDevDMRID(dmrid string, packet []byte) {

	copy(packet[6:15], zero9)
	copy(packet[6:15], dmrid)

}

func NRL21replace200dev(callsign string, ssid, packetType, DevModel uint8, originalCallsign string, originaSSID uint8, originalIP net.IP, dmrid string, data []byte) (packet []byte) {

	packet = make([]byte, len(data))

	copy(packet, data)

	// 写入 DMRID

	copy(packet[6:15], dmrid)

	// 写入 Type  2
	packet[20] = packetType

	// 写入 CallSign
	copy(packet[24:30], callsign)

	if len(callsign) == 5 {
		packet[29] = 0
	}

	// 写入 SSID
	packet[30] = ssid

	// 写入 DevMode
	packet[31] = DevModel

	// 协议原始呼号
	copy(packet[32:38], originalCallsign)

	// 写入原始SSID
	packet[38] = originaSSID

	// 写入 IP 地址
	copy(packet[39:43], originalIP)

	return packet

}

func encodeNRL21(callsign string, ssid, packetType, DevModel uint8, dmrid string, data []byte) (packet []byte) {

	//编码报名

	const fixedBufferSize = 48

	// 计算总大小
	totalSize := fixedBufferSize + len(data)

	// 创建字节切片
	packet = make([]byte, totalSize)

	// 写入固定头部
	copy(packet[0:4], []byte("NRL2"))

	// 写入长度
	binary.BigEndian.PutUint16(packet[4:6], uint16(totalSize))

	// 写入 DMRID

	copy(packet[6:15], dmrid)

	// 写入 Type  2
	packet[20] = packetType

	// 写入 Status
	packet[21] = 1

	// 写入 Count
	// binary.BigEndian.PutUint16(data[18:20], n.Count)

	// 写入 CallSign
	copy(packet[24:30], callsign)
	// if len(callsign) == 5 {
	// 	packet[29] = 0
	// }

	// 写入 SSID
	packet[30] = ssid

	// 写入 DevMode
	packet[31] = DevModel

	// 写入 DATA
	if len(data) > 0 {
		copy(packet[48:], data)
	}

	return packet

}

func getCallsignSSID(callsign string, ssid byte) string {

	var builder strings.Builder

	// 拼接字符串
	builder.WriteString(callsign)
	builder.WriteString("-")
	builder.WriteString(strconv.Itoa(int(ssid)))
	return builder.String()
}

// type COMData struct {
// 	Number int32
// 	DATA   []byte
// }

// type controlData struct {
// 	DeviceID    string
// 	SubDeviceID string
// 	Value       int
// }
