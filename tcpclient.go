package main

import (
	"bufio"
	"log"
	"net"
	"time"
)

func NewTCPClient(host, port string, onMessage func(message []byte)) *TCPClient {
	return &TCPClient{
		host:      host,
		port:      port,
		onMessage: onMessage,
	}
}

type TCPClient struct {
	host      string
	port      string
	conn      net.Conn
	onMessage func(message []byte)
}

func (c *TCPClient) Connect() {

	for {

		conn, err := net.Dial("tcp", net.JoinHostPort(c.host, c.port))
		if err != nil {
			log.Println("无法连接到TCP服务器:", err)
			time.Sleep(5 * time.Second)
			continue
		} else {
			c.conn = conn
			log.Println("已连接到TCP服务器")
			break
		}

	}

	go c.readMessages()

}

func (c *TCPClient) Send(message string) error {

	_, err := c.conn.Write([]byte(message))
	if err != nil {

		return err
	}
	return nil
}

func (c *TCPClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *TCPClient) readMessages() error {
	reader := bufio.NewReader(c.conn)
	for {
		message, err := reader.ReadBytes('\n')
		if err != nil {
			log.Printf("读取TCP消息错误: %v\n", err)
			return err
		}
		if c.onMessage != nil {
			c.onMessage(message)
		}
	}
}
