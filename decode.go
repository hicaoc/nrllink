package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"
)

type NRL21packet struct {
	timeStamp  time.Time
	UDPAddrStr string
	UDPAddr    *net.UDPAddr //报文来源UDP地址和端口
	Version    string       //协议标识 “NRL2” 每个报文都以 NRL2 4个字节开头
	Length     uint16       //上层数据长度
	CPUID      string       //设备唯一标识 长度7字节
	Password   string       //密码
	Type       byte         //上层数据类型 一个字节 0:心跳，1：控制指令 2：G.711语音 3：上线认证，4：设备状态，入电压，温度等，CPU使用率等 5:msg
	Status     byte         //设备状态位
	Count      uint16       //报文计数器2节
	CallSign   string       //所有者呼号 6字节
	SSID       byte         //所有者呼号 1字节
	DATA       []byte       //上层数据内容
}

func (n *NRL21packet) decodeNRL21(d []byte) (err error) {

	if len(d) < 48 {
		return errors.New("packet too short ")
	}
	n.Version = string(d[0:4])

	if n.Version != "NRL2" {
		return errors.New("not NRL packet ")
	}

	n.Length = binary.BigEndian.Uint16(d[4:6])

	n.CPUID = fmt.Sprintf("%02X", d[6:10])
	n.Password = fmt.Sprintf("%02X", d[10:13])
	n.Type = d[20]
	n.Status = d[21]
	n.Count = binary.BigEndian.Uint16(d[21:23])
	n.CallSign = string(bytes.TrimRight(d[24:30], string([]byte{13, 0})))
	n.SSID = d[30]
	n.DATA = d[48:]

	return nil

}

func (n *NRL21packet) String() string {
	return fmt.Sprintf("ver:%v len:%v CPUID:%v CallSign:%v-%v type:%v len:%v  Count:%v  %02X ", n.Version, n.Length, n.CPUID, n.CallSign, n.SSID, n.Type, len(n.DATA), n.Count, n.DATA)
}

type G711Voice struct {
	Number uint32
	DATA   []byte
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
