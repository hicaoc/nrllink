package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// APNs Push Service 配置（先写死 pgarlic，后续改为配置文件）
const (
	apnsPushEndpoint  = "https://ptt.pgarlic.com/push/internal/notify"
	apnsPushKey       = "a46c8525bbf4af99c27ac1148cf1952649e19bc962247409feaafd25827605b3"
	apnsPushEnabled   = true
	apnsMinInterval   = 3 * time.Second // 同一组最短推送间隔，防止频繁推送
)

// 记录每个组上次推送时间，防止短时间内重复推送
var (
	apnsLastPushTime   = make(map[int]time.Time)
	apnsLastPushTimeMu sync.Mutex
)

var apnsHTTPClient = &http.Client{
	Timeout: 5 * time.Second,
}

// apnsPushNotify 通知 APNs push service 有人开始发言
// 异步调用，不阻塞 UDP 处理
func apnsPushNotify(dev *deviceInfo, gp *group) {
	if !apnsPushEnabled {
		return
	}

	// 防抖：同一组在短时间内不重复推送
	apnsLastPushTimeMu.Lock()
	if last, ok := apnsLastPushTime[gp.ID]; ok {
		if time.Since(last) < apnsMinInterval {
			apnsLastPushTimeMu.Unlock()
			return
		}
	}
	apnsLastPushTime[gp.ID] = time.Now()
	apnsLastPushTimeMu.Unlock()

	groupID := strconv.Itoa(gp.ID)

	payload := map[string]string{
		"group_id":     groupID,
		"speaker":      dev.CallSignSSID,
		"speaker_name": dev.CallSign,
	}

	if dev.DMRID > 0 {
		payload["dmr_id"] = strconv.FormatUint(uint64(dev.DMRID), 10)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[APNs] JSON marshal err: %v", err)
		return
	}

	req, err := http.NewRequest("POST", apnsPushEndpoint, bytes.NewReader(body))
	if err != nil {
		log.Printf("[APNs] create request err: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Key", apnsPushKey)

	resp, err := apnsHTTPClient.Do(req)
	if err != nil {
		log.Printf("[APNs] push notify err: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		log.Printf("[APNs] push OK: group=%d speaker=%s", gp.ID, dev.CallSignSSID)
	} else {
		log.Printf("[APNs] push failed: status=%d group=%d speaker=%s",
			resp.StatusCode, gp.ID, dev.CallSignSSID)
	}

}
