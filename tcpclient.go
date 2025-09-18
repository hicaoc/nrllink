package main

import (
	"bufio"
	"log"
	"net"
)

func NewTCPClient(onMessage func(message []byte)) *TCPClient {
	return &TCPClient{
		onMessage: onMessage,
	}
}

type TCPClient struct {
	conn      net.Conn
	onMessage func(message []byte)
}

func (c *TCPClient) Connect(host, port string) error {

	conn, err := net.Dial("tcp", net.JoinHostPort(host, port))
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
		message, err := reader.ReadBytes('\n')
		if err != nil {
			log.Printf("读取TCP消息错误: %v\n", err)
			return
		}
		if c.onMessage != nil {
			c.onMessage(message)
		}
	}
}
