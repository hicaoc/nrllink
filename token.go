package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var TokenKey = RandString(32)

// 定义你的签名密钥
//var jwtKey = []byte("BKWEUaKinE1te2ujmPGA")

// 定义 payload 的结构
type Claims struct {
	Username    string   `json:"username"`
	Roles       []string `json:"roles"`
	Name        string   `json:"name,omitempty"`
	OIDCVirtual bool     `json:"oidc_virtual,omitempty"`
	jwt.RegisteredClaims
}

// GenerateOIDCToken 为无本地账号的 OIDC 临时会话签发 token。
// OIDCVirtual 标记用于 checktoken 识别：本地查无账号时允许构造内存用户，不写 users 表。
func GenerateOIDCToken(username, name string, roles []string) (string, error) {
	expirationTime := time.Now().Add(24 * 30 * time.Hour)
	claims := &Claims{
		Username:    username,
		Roles:       roles,
		Name:        name,
		OIDCVirtual: true,
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

func RandString(len int) []byte {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	bytes := make([]byte, len)
	for i := range len {
		b := r.Intn(26) + 65
		bytes[i] = byte(b)
	}
	return bytes
}
