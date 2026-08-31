package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	oidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
)

// oidcStateCookieName 存放 OIDC 登录过程中的 state|nonce|code_verifier，用于防 CSRF 和 PKCE 校验
const oidcStateCookieName = "oidc_state"

// oidcUserInfo OIDC userinfo 端点返回的用户信息（openid profile email scope）
type oidcUserInfo struct {
	Sub               string `json:"sub"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
	Callsign          string `json:"callsign"`
	Email             string `json:"email"`
}

// oidcCallsign 优先使用 Provider 明确返回的呼号/用户名，HAM ID 的 sub 本身就是呼号。
func oidcCallsign(info *oidcUserInfo) string {
	callsign := strings.ToUpper(strings.TrimSpace(info.Callsign))
	if callsign == "" {
		callsign = strings.ToUpper(strings.TrimSpace(info.PreferredUsername))
	}
	if callsign == "" {
		callsign = strings.ToUpper(strings.TrimSpace(info.Sub))
	}
	return callsign
}

// newVirtualOIDCUserFromInfo 构造无本地账号的 OIDC 临时用户。
// 不写 users 表，仅在当前进程内存中初始化公共能力所需的 Groups/DevList。
func newVirtualOIDCUserFromInfo(info *oidcUserInfo) *userinfo {
	callsign := oidcCallsign(info)
	name := strings.TrimSpace(info.Name)
	if name == "" {
		name = strings.TrimSpace(info.PreferredUsername)
	}
	if name == "" {
		name = callsign
	}

	u := &userinfo{
		ID:          0,
		Name:        name,
		CallSign:    callsign,
		Phone:       "oidc:" + info.Sub,
		Status:      1,
		Roles:       []string{"ham"},
		OIDCVirtual: true,
		OIDCSub:     info.Sub,
	}
	u.userinit()

	// 临时用户也放入内存列表，私有房间和语音 WS 才能正常工作。
	// 若之后同呼号本地用户登录/创建，会被本地用户覆盖。
	if cached, ok := userlist.LoadOrStore(callsign, u); ok {
		if old, ok := cached.(*userinfo); ok && !old.OIDCVirtual {
			return old
		}
	}
	return u
}

// virtualOIDCUserFromClaims 根据 OIDC virtual token 还原临时用户。
func virtualOIDCUserFromClaims(claims *Claims) *userinfo {
	info := &oidcUserInfo{
		Sub:  claims.Username,
		Name: claims.Name,
	}
	u := newVirtualOIDCUserFromInfo(info)
	if len(claims.Roles) > 0 {
		u.Roles = append([]string{}, claims.Roles...)
	}
	return u
}

// oidcVerifier 缓存，issuer/client_id 变化时重建（RemoteKeySet 内部会缓存 JWKS）
var (
	oidcVerifierLock sync.Mutex
	oidcVerifier     *oidc.IDTokenVerifier
	oidcVerifierKey  string
)

// oidcEnabled 判断 OIDC 登录是否配置并启用
// oidcConfValue 读取 OIDC 配置字符串字段，去除首尾空白（配置界面粘贴时容易带入空格，
// 例如 redirect_url 前多一个空格会导致 redirect_uri 与注册值不匹配）
func oidcConfValue(s string) string {
	return strings.TrimSpace(s)
}

func oidcEnabled() bool {
	return conf.OIDC.Enabled && oidcConfValue(conf.OIDC.Issuer) != "" &&
		oidcConfValue(conf.OIDC.ClientID) != "" && oidcConfValue(conf.OIDC.RedirectURL) != ""
}

// oidcOAuth2Config 手工构造 oauth2 配置（Provider 的 discovery 端点被前置代理拦截，不能用标准 discovery）
func oidcOAuth2Config() *oauth2.Config {
	issuer := strings.TrimSuffix(oidcConfValue(conf.OIDC.Issuer), "/")
	return &oauth2.Config{
		ClientID:     oidcConfValue(conf.OIDC.ClientID),
		ClientSecret: oidcConfValue(conf.OIDC.ClientSecret),
		RedirectURL:  oidcConfValue(conf.OIDC.RedirectURL),
		Scopes:       []string{"openid", "profile", "email"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  issuer + "/oauth/authorize",
			TokenURL: issuer + "/oauth/token",
			//Provider 不支持 client_secret_basic，client_id/client_secret 必须放 POST body
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}
}

// getOIDCVerifier id_token 验签器，id_token 签名算法为 EdDSA（默认只收 RS256，必须显式指定）
func getOIDCVerifier(ctx context.Context) *oidc.IDTokenVerifier {
	issuer := strings.TrimSuffix(oidcConfValue(conf.OIDC.Issuer), "/")

	oidcVerifierLock.Lock()
	defer oidcVerifierLock.Unlock()

	key := issuer + "|" + oidcConfValue(conf.OIDC.ClientID)
	if oidcVerifier != nil && oidcVerifierKey == key {
		return oidcVerifier
	}

	keySet := oidc.NewRemoteKeySet(ctx, issuer+"/oauth/jwks")
	oidcVerifier = oidc.NewVerifier(issuer, keySet, &oidc.Config{
		ClientID:             oidcConfValue(conf.OIDC.ClientID),
		SupportedSigningAlgs: []string{"EdDSA"},
	})
	oidcVerifierKey = key

	return oidcVerifier
}

// oidcRandString 生成 n 字节随机数的 base64url 编码字符串（state/nonce/PKCE verifier/初始密码用）
func oidcRandString(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		log.Println("oidc rand read err:", err)
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// oidcSetStateCookie 写入登录状态 cookie，https 时才加 Secure 标记
func oidcSetStateCookie(w http.ResponseWriter, req *http.Request, state, nonce, verifier string) {
	secure := req.TLS != nil || strings.EqualFold(req.Header.Get("X-Forwarded-Proto"), "https")
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookieName,
		Value:    state + "|" + nonce + "|" + verifier,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// oidcClearStateCookie 登录流程结束后清除状态 cookie
func oidcClearStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}

// isMiniProgramRequest 判断请求是否来自微信小程序的 web-view（UA 含 miniProgram）
func isMiniProgramRequest(req *http.Request) bool {
	return strings.Contains(strings.ToLower(req.UserAgent()), "miniprogram")
}

// oidcMiniProgramJump 小程序 web-view 环境：返回一个极简 HTML，通过 jweixin 的
// wx.miniProgram.reLaunch 把 OIDC 结果直接带回小程序页面，不再经过 Web 前端
// （避免 web-view 缓存旧版前端导致无法回跳）
func oidcMiniProgramJump(w http.ResponseWriter, mpURL, tip string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%s</title>
<style>
body{margin:0;display:flex;flex-direction:column;align-items:center;justify-content:center;height:100vh;font-family:sans-serif}
.spinner{width:36px;height:36px;border:4px solid #e8e8e8;border-top-color:#07c160;border-radius:50%%;animation:spin .8s linear infinite}
@keyframes spin{to{transform:rotate(360deg)}}
.tip{margin-top:16px;color:#666;font-size:15px}
</style></head>
<body><div class="spinner"></div><p class="tip">%s</p>
<script src="https://res.wx.qq.com/open/js/jweixin-1.3.2.js"></script>
<script>wx.miniProgram.reLaunch({url:%q});</script>
</body></html>`, tip, tip, mpURL)
}

// oidcLoginErr 登录失败时跳回前端登录页（hash 路由）并带上错误信息
func oidcLoginErr(w http.ResponseWriter, req *http.Request, msg string) {
	if isMiniProgramRequest(req) {
		oidcMiniProgramJump(w, "/pages/login/login?oidc_error="+url.QueryEscape(msg), msg)
		return
	}
	http.Redirect(w, req, "/#/login?oidc_error="+url.QueryEscape(msg), http.StatusFound)
}

// updateOIDCLoginSuccess 登录成功后更新 last_login_time/last_login_ip 并重置 login_err_times
func updateOIDCLoginSuccess(userID int, ip string) {
	_, err := db.Exec(`update users set last_login_time=CURRENT_TIMESTAMP,last_login_ip=?,login_err_times=1 where id=?`, ip, userID)
	if err != nil {
		log.Println("oidc login update users last_login_time and last_login_ip failed, ", err)
	}
}

// matchOrCreateOIDCUser 按 OIDC 身份信息匹配或创建本地用户，角色完全本地管理（忽略 userinfo 的 roles）
func matchOrCreateOIDCUser(info *oidcUserInfo) (*userinfo, error) {

	//先按 oidc_sub 查
	user, err := getuserByOIDCSub(info.Sub)
	if err == nil {
		return user, nil
	}

	callsign := oidcCallsign(info)

	//再按呼号查，查到则绑定 oidc_sub
	user, err = getuserByCallsign(callsign)
	if err == nil {
		_, uerr := db.Exec(`update users set oidc_sub=? where id=?`, info.Sub, user.ID)
		if uerr != nil {
			log.Println("oidc bind oidc_sub failed, ", uerr)
			return nil, uerr
		}
		log.Println("oidc login bind local user:", callsign)
		return user, nil
	}

	if !conf.OIDC.AutoProvision {
		return nil, err
	}

	//自动建号，默认角色 ham。这里必须补齐所有查询列的默认值，避免 SQLite 插入 NULL 导致后续 Scan 失败。
	pass, _ := bcrypt.GenerateFromPassword([]byte(oidcRandString(32)), bcrypt.DefaultCost)

	name := strings.TrimSpace(info.Name)
	if name == "" {
		name = strings.TrimSpace(info.PreferredUsername)
	}
	if name == "" {
		name = callsign
	}
	// phone 有唯一索引；用 oidc: 前缀避免和真实手机号/呼号语义混淆。
	phone := "oidc:" + info.Sub

	query := `INSERT INTO users
	 (pid,
	 name,
	 phone,
	 callsign,
	 oidc_sub,
	 gird,
	 birthday,
	 mdcid,
	 dmrid,
	 sex,
	 nickname,
	 openid,
	 avatar,
	 address,
	 status,
	 password,
	 roles,
	 introduction,
	 alarm_msg,
	 last_login_time,
	 login_err_times,
	 last_login_ip,
	 expire_time,
	 create_time,
	 update_time)
	VALUES ('',?,?,?,?,?,?,?,?,?,?,?,?,?,1,?,'ham','',0,
		CURRENT_TIMESTAMP,0,'','',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`

	res, err := db.Exec(query, name, phone, callsign, info.Sub,
		"", "", "", 0, 0, name, "", conf.WeiXin.AvatarURL, "",
		string(pass))
	if err != nil {
		log.Println("oidc auto provision add user failed, ", err, '\n', query)
		return nil, err
	}

	id, _ := res.LastInsertId()

	user, err = getuserByOIDCSub(info.Sub)
	if err != nil {
		log.Println("oidc get new user failed, ", err)
		return nil, err
	}

	user.ID = int(id)
	user.userinit()
	userlist.Store(user.CallSign, user)

	log.Println("oidc auto provision new user:", callsign)

	return user, nil
}

// httpOIDCConfig 公开接口，前端查询 OIDC 登录是否启用及按钮文案
func (j *jsonapi) httpOIDCConfig(w http.ResponseWriter, req *http.Request) {

	sethttphead(w)

	buttonName := oidcConfValue(conf.OIDC.ButtonName)
	if buttonName == "" {
		buttonName = "HAM统一认证平台登录"
	}

	writeJSONResponse(w, &Response{20000, "ok", &struct {
		Enabled    bool   `json:"enabled"`
		ButtonName string `json:"button_name"`
	}{oidcEnabled(), buttonName}})
}

// httpOIDCLogin 发起 OIDC 登录：生成 state/nonce/PKCE，302 到 Provider 授权端点
func (j *jsonapi) httpOIDCLogin(w http.ResponseWriter, req *http.Request) {

	if !oidcEnabled() {
		oidcLoginErr(w, req, "OIDC登录未启用")
		return
	}

	state := oidcRandString(32)
	nonce := oidcRandString(32)
	verifier := oidcRandString(32)

	oidcSetStateCookie(w, req, state, nonce, verifier)

	authURL := oidcOAuth2Config().AuthCodeURL(state,
		oidc.Nonce(nonce),
		oauth2.S256ChallengeOption(verifier),
	)

	http.Redirect(w, req, authURL, http.StatusFound)
}

// httpOIDCCallback OIDC 回调：换 token、验 id_token、拉 userinfo、匹配本地用户、签发本站 JWT
func (j *jsonapi) httpOIDCCallback(w http.ResponseWriter, req *http.Request) {

	if !oidcEnabled() {
		oidcLoginErr(w, req, "OIDC登录未启用")
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), 30*time.Second)
	defer cancel()

	oidcStart := time.Now() // 耗时统计起点：定位授权后回跳慢在哪一段

	//校验 state 防 CSRF，cookie 用后清除
	cookie, err := req.Cookie(oidcStateCookieName)
	if err != nil {
		oidcLoginErr(w, req, "登录状态已失效，请重新登录")
		return
	}
	oidcClearStateCookie(w)

	parts := strings.Split(cookie.Value, "|")
	if len(parts) != 3 || parts[0] == "" || parts[0] != req.URL.Query().Get("state") {
		oidcLoginErr(w, req, "登录状态校验失败，请重新登录")
		return
	}
	nonce := parts[1]
	verifier := parts[2]

	if errStr := req.URL.Query().Get("error"); errStr != "" {
		oidcLoginErr(w, req, "认证服务器返回错误:"+errStr)
		return
	}

	code := req.URL.Query().Get("code")
	if code == "" {
		oidcLoginErr(w, req, "认证服务器未返回授权码")
		return
	}

	//授权码换 token（带 PKCE code_verifier）
	token, err := oidcOAuth2Config().Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		log.Println("oidc code exchange err:", err)
		oidcLoginErr(w, req, "换取访问令牌失败")
		return
	}
	exchangeDur := time.Since(oidcStart)

	//验 id_token 签名并校验 nonce
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		oidcLoginErr(w, req, "认证服务器未返回id_token")
		return
	}

	idToken, err := getOIDCVerifier(ctx).Verify(ctx, rawIDToken)
	if err != nil {
		log.Println("oidc id_token verify err:", err)
		oidcLoginErr(w, req, "id_token校验失败")
		return
	}
	if idToken.Nonce != nonce {
		oidcLoginErr(w, req, "nonce校验失败，请重新登录")
		return
	}

	//拉 userinfo（id_token 只有 iss/sub/aud/exp/iat/nonce，无 profile）
	info := &oidcUserInfo{}
	resp, err := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token)).Get(strings.TrimSuffix(oidcConfValue(conf.OIDC.Issuer), "/") + "/oauth/userinfo")
	if err != nil {
		log.Println("oidc get userinfo err:", err)
		oidcLoginErr(w, req, "获取用户信息失败")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Println("oidc get userinfo status err:", resp.StatusCode)
		oidcLoginErr(w, req, "获取用户信息失败")
		return
	}
	if err := jsonextra.NewDecoder(resp.Body).Decode(info); err != nil || info.Sub == "" {
		log.Println("oidc userinfo decode err:", err)
		oidcLoginErr(w, req, "用户信息解析失败")
		return
	}
	userinfoDur := time.Since(oidcStart) - exchangeDur

	//匹配本地账号；未启用自动建号且开启临时登录时，允许无本地账号进入。
	user, err := matchOrCreateOIDCUser(info)
	if err != nil {
		if !conf.OIDC.AutoProvision && conf.OIDC.VirtualLogin {
			user = newVirtualOIDCUserFromInfo(info)
		} else if !conf.OIDC.AutoProvision {
			oidcLoginErr(w, req, "账号不存在，请联系管理员")
			return
		} else {
			oidcLoginErr(w, req, "本地账号创建失败，请联系管理员")
			return
		}
	}

	if user.Status != 1 {
		oidcLoginErr(w, req, "账号已禁用")
		return
	}

	//本地账号才更新登录信息；无库临时会话不写 users/operator_log。
	if !user.OIDCVirtual {
		updateOIDCLoginSuccess(user.ID, req.RemoteAddr)
	}

	log.Printf("oidc callback 耗时: 总计 %v（code换token %v，验签+拉userinfo %v）", time.Since(oidcStart), exchangeDur, userinfoDur)

	//与账密登录一致，用 callsign 作为 token identity（checktoken->getuser 按 phone 或 callsign 查）。
	//虚拟会话 token 带 oidc_virtual 标记，后续请求查不到本地账号时可还原临时用户。
	var s string
	if user.OIDCVirtual {
		s, err = GenerateOIDCToken(user.CallSign, user.Name, user.Roles)
	} else {
		s, err = GenerateToken(user.CallSign, user.Roles)
	}
	if err != nil {
		log.Println("oidc token generate err:", err)
		oidcLoginErr(w, req, "生成访问令牌失败")
		return
	}

	if !user.OIDCVirtual {
		addOperatorLog(user.CallSign+" "+req.Header.Get("X-Forwarded-For")+","+req.RemoteAddr, "OIDC登录成功", user)
	}
	log.Println(req.Header.Get("X-Forwarded-For") + "," + req.RemoteAddr + " OIDC User login ok :callsign:" + user.CallSign + ",virtual:" + strconv.FormatBool(user.OIDCVirtual))

	//小程序 web-view 环境直接把 token 带回小程序页面；浏览器则 302 到 Web 前端（hash 路由）
	if isMiniProgramRequest(req) {
		oidcMiniProgramJump(w, "/pages/login/login?oidc_token="+url.QueryEscape(s), "登录成功，正在返回小程序...")
		return
	}
	http.Redirect(w, req, "/#/oidc-callback?token="+url.QueryEscape(s), http.StatusFound)
}
