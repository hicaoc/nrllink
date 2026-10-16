package main

import (
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	loginGuardMaxFails    = 3                // 连续登录错误达到此次数即封锁
	loginGuardMaxAccounts = 5                // 窗口期内尝试不同账号达到此数量即封锁
	loginGuardWindow      = 15 * time.Minute // 账号枚举统计窗口
	loginGuardBlockTime   = 15 * time.Minute // 封锁时长
)

type loginFailure struct {
	account string
	at      time.Time
}

type ipLoginState struct {
	failures     []loginFailure // 窗口期内的失败记录（成功即清零）
	accountFails map[string]int // 每个账号的连续失败次数
	blockedUntil time.Time
}

// ipLoginGuard 内存版登录保护：按客户端 IP 统计连续失败次数与账号枚举行为，
// 触发规则后临时封锁 IP，期满自动解除，不落库。
type ipLoginGuard struct {
	mu     sync.Mutex
	states map[string]*ipLoginState
}

var loginGuard = &ipLoginGuard{states: make(map[string]*ipLoginState)}

// clientIP 优先取反向代理透传的 X-Forwarded-For 首个地址，否则取 RemoteAddr。
// 安全：只有当直连对端是本机回环（同机部署的反向代理）时才信任 XFF，
// 否则攻击者可伪造该头绕过 IP 级登录保护或命中 127.0.0.1 白名单。
func clientIP(req *http.Request) string {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		host = req.RemoteAddr
	}
	if host == "127.0.0.1" || host == "::1" {
		if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
			if i := strings.Index(xff, ","); i >= 0 {
				return strings.TrimSpace(xff[:i])
			}
			return strings.TrimSpace(xff)
		}
	}
	return host
}

// blocked 判断 IP 当前是否处于封锁期；封锁期满时顺便清理状态
// loginGuardSkip 本机回环地址不执行登录保护策略（本机调试、健康检查等场景）
func loginGuardSkip(ip string) bool {
	return ip == "127.0.0.1"
}

func (g *ipLoginGuard) blocked(ip string) bool {
	if loginGuardSkip(ip) {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	st, ok := g.states[ip]
	if !ok {
		return false
	}
	if time.Now().Before(st.blockedUntil) {
		return true
	}
	if !st.blockedUntil.IsZero() {
		delete(g.states, ip)
	}
	return false
}

// recordSuccess 登录成功后清零该 IP 的失败记录
func (g *ipLoginGuard) recordSuccess(ip string) {
	if loginGuardSkip(ip) {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.states, ip)
}

// recordFailure 记录一次登录失败，命中封锁规则时返回 true
func (g *ipLoginGuard) recordFailure(ip, account string) bool {
	if loginGuardSkip(ip) {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	st := g.states[ip]
	if st == nil {
		st = &ipLoginState{accountFails: make(map[string]int)}
		g.states[ip] = st
	}

	// 只保留统计窗口内的失败记录
	cutoff := now.Add(-loginGuardWindow)
	kept := st.failures[:0]
	for _, f := range st.failures {
		if f.at.After(cutoff) {
			kept = append(kept, f)
		}
	}
	account = strings.ToUpper(strings.TrimSpace(account))
	st.failures = append(kept, loginFailure{account: account, at: now})

	// 规则1：同一账号连续登录错误达到上限，封锁IP（该账号登录成功会清零计数）
	st.accountFails[account]++
	if st.accountFails[account] >= loginGuardMaxFails {
		st.blockedUntil = now.Add(loginGuardBlockTime)
		st.failures = nil
		st.accountFails = make(map[string]int)
		log.Printf("登录保护：IP %v 连续 %v 次登录错误，封锁 %v", ip, loginGuardMaxFails, loginGuardBlockTime)
		return true
	}

	// 规则2：窗口期内用多个不同账号（呼号或手机号）尝试登录，封锁IP
	accounts := make(map[string]struct{}, loginGuardMaxAccounts)
	for _, f := range st.failures {
		accounts[f.account] = struct{}{}
	}
	if len(accounts) >= loginGuardMaxAccounts {
		st.blockedUntil = now.Add(loginGuardBlockTime)
		st.failures = nil
		st.accountFails = make(map[string]int)
		log.Printf("登录保护：IP %v 在 %v 内尝试 %v 个不同账号登录，封锁 %v",
			ip, loginGuardWindow, loginGuardMaxAccounts, loginGuardBlockTime)
		return true
	}

	return false
}

// rateLimiter 简单的单 key 滑动窗口限频器，用于未认证接口的防刷
type rateLimiter struct {
	mu      sync.Mutex
	max     int
	window  time.Duration
	records map[string][]time.Time
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	return &rateLimiter{max: max, window: window, records: make(map[string][]time.Time)}
}

// allow 记录一次请求并返回是否放行；超限时请求不计入窗口
func (l *rateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)
	kept := l.records[key][:0]
	for _, t := range l.records[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.max {
		l.records[key] = kept
		return false
	}
	l.records[key] = append(kept, now)
	return true
}
