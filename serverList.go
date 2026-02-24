package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	platformServersReportURL = "https://nrlptt.com/api/platform-servers/report"
	platformServersListURL   = "https://nrlptt.com/api/platform-servers"
)

var platformServerHTTPClient = &http.Client{Timeout: 10 * time.Second}

type platformServerReport struct {
	Name   string `json:"name"`
	Host   string `json:"host"`
	Port   string `json:"port"`
	Online int    `json:"online"`
	Total  int    `json:"total"`
}

type platformServersListResponse struct {
	Data []platformServerListItem `json:"data"`
}

type platformServersListResponseWithItems struct {
	Data struct {
		Items []Platformitem `json:"items"`
	} `json:"data"`
}

type platformServerListItem struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Host      string `json:"host"`
	Port      string `json:"port"`
	Online    int    `json:"online"`
	Total     int    `json:"total"`
	Hidden    bool   `json:"hidden"`
	SortOrder int    `json:"sort_order"`
	Source    string `json:"source"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updated_at"`
}

func startPlatformServerSync() {
	if conf.APRS.SelfAddress == "" || conf.APRS.SelfPort == "" {
		log.Println("platform-servers: skip sync/report, APRS self address or port is empty")
		return
	}

	if err := reportCurrentServerStatus(); err != nil {
		log.Println("platform-servers: initial report failed:", err)
	}
	if err := syncPlatformServerList(); err != nil {
		log.Println("platform-servers: initial sync failed:", err)
	}

	reportTicker := time.NewTicker(3 * time.Minute)
	listTicker := time.NewTicker(5 * time.Minute)
	defer reportTicker.Stop()
	defer listTicker.Stop()

	for {
		select {
		case <-reportTicker.C:
			if err := reportCurrentServerStatus(); err != nil {
				log.Println("platform-servers: report failed:", err)
			}
		case <-listTicker.C:
			if err := syncPlatformServerList(); err != nil {
				log.Println("platform-servers: sync list failed:", err)
			}
		}
	}
}

func reportCurrentServerStatus() error {
	payload := platformServerReport{
		Name:   strings.TrimSpace(conf.SystemInfo.PlatformName),
		Host:   strings.TrimSpace(conf.APRS.SelfAddress),
		Port:   strings.TrimSpace(conf.APRS.SelfPort),
		Online: totalstats.OnlineDevNumber,
		Total:  len(devCallsignSSIDMap),
	}

	if payload.Name == "" {
		payload.Name = strings.TrimSpace(conf.SystemInfo.NameShorthand)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, platformServersReportURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := platformServerHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return &requestError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	return nil
}

func syncPlatformServerList() error {
	resp, err := platformServerHTTPClient.Get(platformServersListURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return &requestError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	list, err := decodePlatformServerList(body)
	if err != nil {
		return err
	}

	for i := range list {
		hostPort := net.JoinHostPort(list[i].Host, list[i].Port)
		targetAddr, err := net.ResolveUDPAddr("udp", hostPort)
		if err != nil {
			log.Println("platform-servers: invalid udp addr:", hostPort, err)
			list[i].udpAddr = nil
			continue
		}
		if list[i].Host == conf.APRS.SelfAddress {
			list[i].udpAddr = nil
			continue
		}
		list[i].udpAddr = targetAddr
	}

	PlatformList = list
	conf.PlatformList = list
	return nil
}

func decodePlatformServerList(body []byte) ([]Platformitem, error) {
	var direct []Platformitem
	if err := json.Unmarshal(body, &direct); err == nil {
		return direct, nil
	}

	var withData platformServersListResponse
	if err := json.Unmarshal(body, &withData); err == nil && withData.Data != nil {
		list := make([]Platformitem, 0, len(withData.Data))
		for _, item := range withData.Data {
			list = append(list, Platformitem{
				Name:   item.Name,
				Host:   item.Host,
				Port:   item.Port,
				Online: item.Online,
				Total:  item.Total,
			})
		}
		return list, nil
	}

	var withItems platformServersListResponseWithItems
	if err := json.Unmarshal(body, &withItems); err == nil && withItems.Data.Items != nil {
		return withItems.Data.Items, nil
	}

	return nil, &decodeError{Body: string(body)}
}

type requestError struct {
	StatusCode int
	Body       string
}

func (e *requestError) Error() string {
	return "http status " + http.StatusText(e.StatusCode) + "(" + strconv.Itoa(e.StatusCode) + ") body: " + strings.TrimSpace(e.Body)
}

type decodeError struct {
	Body string
}

func (e *decodeError) Error() string {
	return "unexpected platform-servers response: " + strings.TrimSpace(e.Body)
}
