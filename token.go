package main

import (
	"fmt"
	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenKey JWT 签名密钥，由 initTokenKey 在配置加载后初始化
var TokenKey []byte

// 定义你的签名密钥
//var jwtKey = []byte("BKWEUaKinE1te2ujmPGA")

// 定义 payload 的结构
type Claims struct {
	Username    string   `json:"username"`
	Roles       []string `json:"roles"`
	Name        string   `json:"name,omitempty"`
	OIDCVirtual bool     `json:"oidc_virtual,omitempty"`
	DMRID       string   `json:"dmrid,omitempty"`
	MDCID       string   `json:"mdcid,omitempty"`
	jwt.RegisteredClaims
}

// GenerateOIDCToken 为无本地账号的 OIDC 临时会话签发 token。
// OIDCVirtual 标记用于 checktoken 识别：本地查无账号时允许构造内存用户，不写 users 表。
// dmrid/mdcid 随 token 携带，进程重启后还原临时用户时不丢失。
func GenerateOIDCToken(username, name string, roles []string, dmrid, mdcid string) (string, error) {
	expirationTime := time.Now().Add(24 * 30 * time.Hour)
	claims := &Claims{
		Username:    username,
		Roles:       roles,
		Name:        name,
		OIDCVirtual: true,
		DMRID:       dmrid,
		MDCID:       mdcid,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "nrllink",
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(TokenKey)
}

// 生成 JWT token
func GenerateToken(username string, roles []string) (string, error) {
	// 设置过期时间
	expirationTime := time.Now().Add(24 * 30 * time.Hour)
	claims := &Claims{
		Username: username,
		Roles:    roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "nrllink",
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	// 创建 token 并签名
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(TokenKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// 验证 JWT token
func ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

	// 解析和验证 token
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(TokenKey), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("无效的令牌")
	}

	return claims, nil
}

// initTokenKey 初始化 JWT 签名密钥：优先使用配置文件 Web.Token_Key（重启后会话不失效），
// 未配置时用 crypto/rand 生成随机密钥（重启后所有会话失效）。
// 旧实现用 math/rand + 时间种子生成，可被离线爆破伪造 token，已废弃。
func initTokenKey() {
	if conf.Web.TokenKey != "" {
		if len(conf.Web.TokenKey) < 32 {
			log.Println("警告：配置文件 Web.Token_Key 长度小于 32 字节，建议更换为高熵随机密钥")
		}
		TokenKey = []byte(conf.Web.TokenKey)
		return
	}
	TokenKey = []byte(oidcRandString(32))
	log.Println("未配置 Web.Token_Key，已生成随机 JWT 密钥，重启后所有登录会话将失效")
}
