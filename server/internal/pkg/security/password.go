// Package security 提供口令散列与随机串生成。
package security

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword 使用 bcrypt（默认 cost）散列口令，输出已含 salt 的编码串。
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("生成口令散列失败: %w", err)
	}
	return string(b), nil
}

// VerifyPassword 校验口令。
func VerifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// RandomToken 生成 n 字节密码学随机数，再做 URL 安全 Base64 编码。
// 结果长度约为 4n/3，返回串可直接放进 URL/Redis key。
// 随机数源失败属不可恢复的环境问题，故直接 panic 而非返回 error。
func RandomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("随机数生成失败: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
