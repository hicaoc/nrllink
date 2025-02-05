package main

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// 定义你的签名密钥
var jwtKey = []byte("BKWEUaKinE1te2ujmPGA")

// 定义 payload 的结构
type Claims struct {
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

// 生成 JWT token
func GenerateToken(username string, roles []string) (string, error) {
	// 设置过期时间
	expirationTime := time.Now().Add(24 * time.Hour)
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
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// 验证 JWT token
func ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

	// 解析和验证 token
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtKey, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("无效的令牌")
	}

	return claims, nil
}
