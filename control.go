package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strconv"
)

//  类型=01 音频包  48+500字节
//	类型=02 只发空闲包   48字节
//	类型=03 子类01 查询EEPROM设置  48+1字节
//	类型=03 子类02 返回EEPROM设置  48+1+512字节
//	类型=03 子类03 修改EEPROM设置  48+1+512字节
//	类型=03 子类04 主机重启  48+1字节

type control struct {
	DCDSelect         byte   `json:"dcd_select"`          //0x00  DCD 0=PTT DISABLE   1=MANUAL  2=SQL_LO  3=SQL_HI    4=VOX
	PTTEnable         byte   `json:"ptt_enable"`          //0x01  0=PTT DISABLE   1=PTT ENABLE
	PTTLevelReversed  byte   `json:"ptt_level_reversed"`  //0x02  PTT电平反转     NRL2100 待机=0  发射=1   NRL2300 PTT 待机=1 发射=0
	AddTailVoice      uint16 `json:"add_tail_voice"`      //0x03-0x04  默认加尾音20   步进5ms,最小大于20*5=100ms
	RemoveTailVoice   uint16 `json:"remove_tail_voice"`   //0x05-0x06  默认消尾音,步进5MS  50*5=250ms
	PTTresistive      byte   `json:"ptt_resistive"`       //0x07  PTT 电阻  0=0FF 1=EN
	MonitorOut        byte   `json:"monitor_out"`         //0x08  MONITOR 监听输出  0=0FF 1=EN
	KeyFunc           byte   `json:"key_func"`            //0x09  自定义KEY  0=Relay 1=MANUAL PTT
	RealyStatus       byte   `json:"realy_status"`        //0x0A  Relay继电器掉电状态 0=断开  1=吸合
	AllowRealyControl byte   `json:"allow_relay_control"` //0x0B  是否允许继电器控制
	VoiceBitrate      byte   `json:"voice_bitrate"`       //0x0C  H=原码率  M=码率/2
	LocalCPUID        string `json:"local_cpuid"`         //0x10-0x16  本机设备序列号，不可修改
	LocalPassword     string `json:"local_password"`
	PeerCPUID         string `json:"peer_cpuid"`      //0x17-0x1D 远程目标设备序列号,初始同本机序列号,可修改
	PeerPassword      string `json:"peer_password"`   //远程目标接入密码，0-9 A-F 可修改
	InitSign          byte   `json:"init_sign"`       //0x1F  //初始化标记
	LocalIPaddr       string `json:"local_ipaddr"`    //0x20-0x23  192.168.1.190
	Gateway           string `json:"gateway"`         //0x24-0x27  192.168.1.1
	NetMask           string `json:"netmask"`         //0x28-0x31  255.255.255.0
	DNSIP             string `json:"dns_ipaddr"`      //0x2C-0x2F  114.114.114.114
	DestPort          uint16 `json:"dest_port"`       //0x30-0x31  UDP AUDIO OUT目标端口号
	LoaclPort         uint16 `json:"local_port"`      //0x32-0x33  UDP AUDIO IN本机端口号
	SSID              byte   `json:"ssid"`            //0x40
	CallSign          string `json:"callsign"`        //0x41-0x47  呼号 最长6位 0X00结束符号
	DestDomainName    string `json:"dest_domainname"` //0x50-0x7F  目标IP或域名，IP=XXX.XXX.XXX.XXX    域名=XXX.XXX.XXX   50-7F 最长48字节 0X00结束符号

	//1w parm
	///OneWParm       string `json:"onew_parm"`       //0x80-0x9F 对讲机模块频率    按格式填写  27/29字节   0X00结束符号
	OneGBWBand        byte   `json:"one_band"`
	OneGBWDTMF        byte   `json:"one_dtmf"`
	OneReciveFreq     string `json:"one_recive_freq"`    //
	OneTransmitFreq   string `json:"one_transmit_freq"`  //
	OneReciveCXCSS    string `json:"one_recive_cxcss"`   //
	OneTransmitCXCSS  string `json:"one_transmit_cxcss"` //
	OneSQLLevel       int    `json:"one_sql_level"`
	OneVolume         int    `json:"one_volume"`          //0xA0  UV1模块音量1-9级
	OneMICSensitivity int    `json:"one_mic_sensitivity"` //0xA1  MIC灵敏度1-8
	OneMICEncryption  int    `json:"one_mic_encryption"`  //0xA2  MIC语音加密 0 1-8
	OneUVPower        byte   `json:"one_uv_power"`        //0xA3 PD 内置UV模块电源开关

	//moto 3188 3688信道
	MotoChannel byte `json:"moto_channel"`

	//2w parm
	TwoReciveFreq    string `json:"two_recive_freq"`    //0xC0-0xC8  UV2     对讲机模块频率     	 35字节
	TwoTransmitFreq  string `json:"two_transmit_freq"`  //0xCA-0xD3
	TwoReciveCXCSS   string `json:"two_recive_cxcss"`   //0xD4-0xD8 接收哑音
	TwoTransmitCXCSS string `json:"two_transmit_cxcss"` //0xDA-0xDE
	FLAG1            string `json:"flag1"`              //0xE0
	FLAG2            string `json:"flag2"`              //0xE2
	TwoVolume        int    `json:"two_volume"`         //0xEE 2W音量
	TwoSavePower     int    `json:"two_save_power"`     //0xEF 2W SAVE  0=开启省电  1=关闭省电
	TwoSQLLevel      int    `json:"two_sql_level"`      //0xF0  SQL，MICLVL, TOT，SCRAMLVL ,COMP  0X00结束符号
	TwoMICLevel      int    `json:"two_mic_level"`      //0xF2
	TwoTOTLevel      int    `json:"two_tot_level"`      //0xF4

	data []byte //原始
}

func decodeControlPacket(data []byte) *control {

	c := &control{}

	//子类型为2是响应
	if data[0] == 2 && len(data) > 512 {

		c.data = make([]byte, 512)

		copy(c.data, data[1:])

		c.DCDSelect = c.data[0]
		c.PTTEnable = c.data[1]
		c.PTTLevelReversed = c.data[2]
		c.AddTailVoice = uint16(c.data[3])<<8 | uint16(c.data[4])
		c.RemoveTailVoice = uint16(c.data[5])<<8 | uint16(c.data[6]) //0x05-0x06  默认消尾音,步进5MS  50*5=250ms
		//c.AddTailVoice = uint16(c.data[3])<<8 | uint16(c.data[4])
		//c.RemoveTailVoice = uint16(c.data[5])<<8 | uint16(c.data[6]) //0x05-0x06  默认消尾音,步进5MS  50*5=250ms
		c.PTTresistive = c.data[7]                        //0x07  PTT 电阻  0=0FF 1=EN
		c.MonitorOut = c.data[8]                          //0x08  MONITOR 监听输出  0=0FF 1=EN
		c.KeyFunc = c.data[9]                             //0x09  自定义KEY  0=Relay 1=MANUAL PTT
		c.RealyStatus = c.data[10]                        //0x0A  Relay继电器掉电状态 0=断开  1=吸合
		c.AllowRealyControl = c.data[11]                  //0x0B  是否允许继电器控制
		c.VoiceBitrate = c.data[12]                       //0x0C  H=原码率  M=码率/2
		c.LocalCPUID = fmt.Sprintf("%02X", c.data[16:20]) //0x10-0x16  本机设备序列号，不可修改
		c.LocalPassword = fmt.Sprintf("%02X", c.data[20:23])
		c.PeerCPUID = fmt.Sprintf("%02X", c.data[23:27])                                           //0x17-0x1D 远程目标设备序列号,初始同本机序列号,可修改
		c.PeerPassword = fmt.Sprintf("%02X", c.data[27:30])                                        //远程目标接入密码，0-9 A-F 可修改
		c.InitSign = c.data[31]                                                                    //0x1F  //初始化标记
		c.LocalIPaddr = fmt.Sprintf("%v.%v.%v.%v", c.data[32], c.data[33], c.data[34], c.data[35]) //0x20-0x23  192.168.1.190
		c.Gateway = fmt.Sprintf("%v.%v.%v.%v", c.data[36], c.data[37], c.data[38], c.data[39])     //0x24-0x27  192.168.1.1
		c.NetMask = fmt.Sprintf("%v.%v.%v.%v", c.data[40], c.data[41], c.data[42], c.data[43])     //0x28-0x31  255.255.255.0
		c.DNSIP = fmt.Sprintf("%v.%v.%v.%v", c.data[44], c.data[45], c.data[46], c.data[47])       //0x2C-0x2F  114.114.114.114
		c.DestPort = uint16(c.data[48])<<8 | uint16(c.data[49])                                    //0x30-0x31  UDP AUDIO OUT目标端口号
		c.LoaclPort = uint16(c.data[50])<<8 | uint16(c.data[51])                                   //0x32-0x33  UDP AUDIO IN本机端口号
		c.SSID = c.data[64]                                                                        //0x40
		c.CallSign = string(bytes.Split(c.data[65:72], []byte{0x00})[0])                           //0x41-0x47  呼号 最长6位 0X00结束符号
		c.DestDomainName = string(bytes.Split(c.data[80:128], []byte{0x00})[0])                    //0x50-0x7F  目标IP或域名，IP=XXX.XXX.XXX.XXX    域名=XXX.XXX.XXX   50-7F 最长48字节 0X00结束符号

		//1w parm
		//c.OneWParm = string(c.data[128:144]) //0x80-0x8F 对讲机模块频率    按格式填写  27/29字节   0X00结束符号

		oneParm := bytes.Split(bytes.Split(c.data[128:160], []byte{0x00})[0], []byte{','})

		if len(oneParm) >= 6 {

			if s, err := strconv.Atoi(string(oneParm[0])); err == nil {
				c.OneGBWBand = byte(s) & 0x01
				c.OneGBWDTMF = byte(s) & 0x02
			}

			c.OneTransmitFreq = string(oneParm[1])
			c.OneReciveFreq = string(oneParm[2])
			c.OneReciveCXCSS = string(oneParm[3])
			c.OneSQLLevel, _ = strconv.Atoi(string(oneParm[4]))

			c.OneTransmitCXCSS = string(oneParm[5])

		}

		c.OneVolume, _ = strconv.Atoi(string(c.data[160]))         //0xA0  UV1模块音量1-9级
		c.OneMICSensitivity, _ = strconv.Atoi(string(c.data[161])) //0xA1  MIC灵敏度1-8
		c.OneMICEncryption, _ = strconv.Atoi(string(c.data[162]))  //0xA2  MIC语音加密 0 1-8
		c.OneUVPower = c.data[163]                                 //0xA3 PD 内置UV模块电源开关

		//moto 3188
		c.MotoChannel = c.data[164]

		//取出\0 前的字符串，并用逗号分割
		twoParm := bytes.Split(bytes.Split(c.data[192:227], []byte{0x00})[0], []byte{','})

		if len(twoParm) >= 6 {

			c.TwoReciveFreq = string(twoParm[0])    //0xC0-0xC8  UV2     对讲机模块频率     	 35字节
			c.TwoTransmitFreq = string(twoParm[1])  //0xCA-0xD3
			c.TwoReciveCXCSS = string(twoParm[2])   //0xD4-0xD8
			c.TwoTransmitCXCSS = string(twoParm[3]) //0xDA-0xDE
			c.FLAG1 = string(twoParm[4])            //0xE0
			c.FLAG2 = string(twoParm[5])            //0xE2

		}

		//2w parm
		// c.TwoReciveFreq = string(c.data[192:201])    //0xC0-0xC8  UV2     对讲机模块频率     	 35字节
		// c.TwoTransmitFreq = string(c.data[202:212])  //0xCA-0xD3
		// c.TwoReciveCXCSS = fmt.Sprintf("%02X ", c.data[27:30])    string(c.data[212:217])   //0xD4-0xD8
		// c.TwoTransmitCXCSS =fmt.Sprintf("%02X ", c.data[27:30])    string(c.data[218:222]) //0xDA-0xDE
		// c.FLAG1 = string(c.data[224])                //0xE0
		// c.FLAG2 = string(c.data[226])                //0xE2
		c.TwoVolume, _ = strconv.Atoi(string(c.data[238]))    //0xEE 2W音量
		c.TwoSavePower, _ = strconv.Atoi(string(c.data[239])) //0xEF 2W SAVE  0=开启省电  1=关闭省电
		c.TwoSQLLevel, _ = strconv.Atoi(string(c.data[240]))  //0xF0  SQL，MICLVL, TOT，SCRAMLVL ,COMP  0X00结束符号
		c.TwoMICLevel, _ = strconv.Atoi(string(c.data[242]))  //0xF2
		c.TwoTOTLevel, _ = strconv.Atoi(string(c.data[244]))  //0xF4

	} else {

		fmt.Println("device parm:", data)

	}

	return c

}

func encodeDeviceParm(dev *deviceInfo, subtype byte) (packet []byte) {

	packet = append(packet, []byte{'N', 'R', 'L', '2'}...) //0-3
	packet = append(packet, []byte{0, 49}...)              //长度   4-5

	id, _ := hex.DecodeString(dev.CPUID)

	packet = append(packet, id...) //本机CPUID  6-10

	pass_hex_data, _ := hex.DecodeString(dev.Password)
	packet = append(packet, []byte(pass_hex_data)[:3]...) //本机设备密码  10-12
	packet = append(packet, id...)                        //目标CPUID  13-19
	packet = append(packet, []byte(pass_hex_data)[:3]...) //目标设备密码  10-12
	packet = append(packet, 3)                            //类型3  20
	packet = append(packet, 0)                            //busy 21
	packet = append(packet, []byte{0x00, 0x00}...)        //计数器  22-23

	packet = append(packet,
		append([]byte(dev.CallSign),
			make([]byte, 6-len(dev.CallSign))...)...) //callsign     24-29  //可能存在5位呼号的问题

	packet = append(packet, dev.SSID)                    // 30
	packet = append(packet, []byte{0x21, 0x03, 0x14}...) //version  31-33
	packet = append(packet, make([]byte, 12)...)         //Reserved  34-45
	packet = append(packet, []byte{0x00, 0x00}...)       //crc   46-47
	packet = append(packet, subtype)                     // 查询

	//log.Println(len(packet), fmt.Sprintf("CPUID:%v Callsign:%v-%v %02X", dev.CPUID, dev.CallSign, dev.SSID, packet))

	//fmt.Println(string(packet))

	return packet

}

// func sendParmQuery(CPUID string) {

// }

// func sendParmChange(CPUID string) {}
