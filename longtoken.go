package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// HAM ID 长期 API Token 接入层：
// 设备/App/小程序仍然只访问 NRL；NRL 拿到长期 token 后调用 HAM ID introspection 验证。
// token 本体不落库、不写日志，只缓存 introspection 结果。
const (
	longTokenPrefix     = "hamid_pat_"
	longTokenCacheTTL   = 30 * time.Second
	longTokenMaxCache   = 4096
	longTokenMaxBody    = 1 << 20
	longTokenHTTPExpiry = 5 * time.Second
)

type longTokenIntrospection struct {
	Active   bool   `json:"active"`
	Sub      string `json:"sub"`
	Username string `json:"username"`
	Kind     string `json:"kind"`
	Scope    string `json:"scope"`
	MAC      string `json:"mac"`
	Exp      int64  `json:"exp"`
}

type longTokenCacheItem struct {
	Claims    *Claims
	ExpiresAt time.Time
}

var (
	longTokenHTTPClient = &http.Client{Timeout: longTokenHTTPExpiry}
	longTokenCache      sync.Map
)

// longTokenEnabled 是否启用长期 token 登录。复用 OIDC 配置里的 Issuer/ClientID/ClientSecret。
func longTokenEnabled() bool {
	return conf.OIDC.Enabled && conf.OIDC.TokenLogin &&
		oidcConfValue(conf.OIDC.Issuer) != "" &&
		oidcConfValue(conf.OIDC.ClientID) != "" &&
		oidcConfValue(conf.OIDC.ClientSecret) != ""
}

func isLongToken(token string) bool {
	return strings.HasPrefix(strings.TrimSpace(token), longTokenPrefix)
}

// tokenFromRequest 兼容三种携带方式：
// 1) Authorization: Bearer hamid_pat_xxx
// 2) X-Token: hamid_pat_xxx（老 App/小程序可直接替换原 token）
// 3) X-Token: Bearer hamid_pat_xxx
func tokenFromRequest(req *http.Request) string {
	if req == nil {
		return ""
	}
	auth := strings.TrimSpace(req.Header.Get("Authorization"))
	if len(auth) > 7 && strings.EqualFold(auth[:7], "Bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	if strings.HasPrefix(auth, longTokenPrefix) {
		return auth
	}
	token := strings.TrimSpace(req.Header.Get("x-token"))
	if len(token) > 7 && strings.EqualFold(token[:7], "Bearer ") {
		return strings.TrimSpace(token[7:])
	}
	return token
}

// userFromRawToken 统一验证 NRL 本地 JWT 和 HAM ID 长期 token。
func userFromRawToken(ctx context.Context, rawToken string) (*Claims, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil, errors.New("token 为空")
	}

	if isLongToken(rawToken) {
		if !longTokenEnabled() {
			return nil, errors.New("长期 token 登录未启用")
		}
		claims, err := verifyLongToken(ctx, rawToken)
		if err != nil {
			return nil, err
		}
		return claims, nil
	}

	return ValidateToken(rawToken)
}

// verifyLongToken 调 HAM ID RFC 7662 introspection；结果短时缓存。
func verifyLongToken(ctx context.Context, rawToken string) (*Claims, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, longTokenHTTPExpiry)
	defer cancel()

	sum := sha256.Sum256([]byte(rawToken))
	cacheKey := hex.EncodeToString(sum[:])

	if cached, ok := longTokenCache.Load(cacheKey); ok {
		if item, ok := cached.(longTokenCacheItem); ok && time.Now().Before(item.ExpiresAt) {
			return item.Claims, nil
		}
		longTokenCache.Delete(cacheKey)
	}

	form := url.Values{
		"token":         {rawToken},
		"client_id":     {oidcConfValue(conf.OIDC.ClientID)},
		"client_secret": {oidcConfValue(conf.OIDC.ClientSecret)},
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimSuffix(oidcConfValue(conf.OIDC.Issuer), "/")+"/oauth/introspect",
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := longTokenHTTPClient.Do(httpReq)
	if err != nil {
		return nil, errors.New("长期 token 验证服务不可用")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("长期 token 验证失败")
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, longTokenMaxBody))
	if err != nil {
		return nil, errors.New("长期 token 验证失败")
	}
	var info longTokenIntrospection
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, errors.New("长期 token 验证响应无效")
	}
	if !info.Active || strings.TrimSpace(info.Sub) == "" {
		return nil, errors.New("长期 token 无效")
	}

	claims := &Claims{
		Username: strings.ToUpper(strings.TrimSpace(info.Sub)),
		Name:     strings.TrimSpace(info.Username),
		Roles:    []string{"ham"},
		// 后续 getuser 查不到本地账号时，只有在 virtual_login 开启时才允许临时会话。
		OIDCVirtual: true,
	}
	if claims.Name == "" {
		claims.Name = claims.Username
	}

	cacheUntil := time.Now().Add(longTokenCacheTTL)
	if info.Exp > 0 {
		exp := time.Unix(info.Exp, 0)
		if !time.Now().Before(exp) {
			return nil, errors.New("长期 token 已过期")
		}
		// 避免吊销后长时间仍可用；token 本身快过期时缓存到过期前即可。
		if exp.Before(cacheUntil) {
			cacheUntil = exp
		}
	}
	longTokenCache.Store(cacheKey, longTokenCacheItem{Claims: claims, ExpiresAt: cacheUntil})

	// 简单防止缓存无限增长：超限时清空；正常情况下最多几秒/几十秒后自然过期。
	if longTokenCacheSize() > longTokenMaxCache {
		longTokenCache.Range(func(key, value any) bool {
			longTokenCache.Delete(key)
			return true
		})
	}

	return claims, nil
}

func longTokenCacheSize() int {
	count := 0
	longTokenCache.Range(func(key, value any) bool {
		count++
		return true
	})
	return count
}
