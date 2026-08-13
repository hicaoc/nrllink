package main

import (
	"io"
	"net/http"
)

// httpGetConfig 管理员读取当前完整配置（含密钥字段，供编辑页面回显原值）
func (j *jsonapi) httpGetConfig(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	if !checkrole(u, []string{"admin"}) {
		w.Write(ResRightErr)
		return
	}

	confLock.RLock()
	defer confLock.RUnlock()
	writeJSONResponse(w, &Response{20000, "ok", conf})
}

// httpUpdateConfig 管理员更新配置并持久化到 yaml 配置文件
func (j *jsonapi) httpUpdateConfig(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	if !checkrole(u, []string{"admin"}) {
		w.Write(ResRightErr)
		return
	}

	body, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		writeJSONResponse(w, &Response{20001, "读取请求失败", nil})
		return
	}

	// 先把当前 conf marshal 成 JSON 再 unmarshal 进 newConf，
	// 再把请求 body 覆盖上去：既保留 AccessToken/WeiXinAccessToken 等
	// 运行时字段（无 yaml tag），也让前端未提交的字段保持原值
	confLock.RLock()
	cur, err := jsonextra.Marshal(conf)
	confLock.RUnlock()
	if err != nil {
		writeJSONResponse(w, &Response{20001, "读取当前配置失败", nil})
		return
	}

	newConf := &config{}
	if err := jsonextra.Unmarshal(cur, newConf); err != nil {
		writeJSONResponse(w, &Response{20001, "当前配置解析失败", nil})
		return
	}
	if err := jsonextra.Unmarshal(body, newConf); err != nil {
		writeJSONResponse(w, &Response{20001, "配置参数错误", nil})
		return
	}

	// Platformitem 含非导出字段 udpAddr 及运行时统计 Online/Total，
	// 全量替换会丢失，按 Host+Port 匹配旧条目并拷回运行时状态
	confLock.RLock()
	oldList := conf.PlatformList
	confLock.RUnlock()
	for i := range newConf.PlatformList {
		for _, old := range oldList {
			if old.Host == newConf.PlatformList[i].Host && old.Port == newConf.PlatformList[i].Port {
				newConf.PlatformList[i].udpAddr = old.udpAddr
				newConf.PlatformList[i].Online = old.Online
				newConf.PlatformList[i].Total = old.Total
				break
			}
		}
	}

	confLock.Lock()
	*conf = *newConf
	PlatformList = conf.PlatformList // 同步全局镜像（aprsget.go/serverList.go 会反向写 conf.PlatformList）
	confLock.Unlock()

	if err := conf.save(); err != nil {
		writeJSONResponse(w, &Response{20001, "配置保存失败: " + err.Error(), nil})
		return
	}

	writeJSONResponse(w, &Response{20000, "保存成功，端口/数据库等配置需重启服务后生效", nil})
}
