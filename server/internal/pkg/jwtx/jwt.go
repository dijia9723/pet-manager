// Package jwtx 负责签发与校验登录凭证。
// 目录/包名刻意与 github.com/golang-jwt/jwt 区分开。
package jwtx

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"pet-manager/server/internal/config"
	"pet-manager/server/internal/pkg/errcode"
)

const (
	TokenTypeAccess  = "access"  // 访问凭证，随业务请求携带
	TokenTypeRefresh = "refresh" // 刷新凭证，仅用于换取新的访问凭证

	// SessionIDClaimKey 会话 ID 用于在 Redis 中做注销与续签，
	// 是"JWT 无状态"与"可强制下线"之间的折中。
	SessionIDClaimKey = "sid"
)

// Claims 是贯穿全站的登录态载荷。
//
// TenantSchema 直接随凭证下发，避免每个请求回查数据库；
// 它的值由系统生成（tenant_<id>），并被 db.ValidateSchema 再次校验，
// 因此可以安全地用于 search_path。
type Claims struct {
	UserID       uint64 `json:"uid"`
	Username     string `json:"uname"`
	UserType     string `json:"utype"`
	TenantID     uint64 `json:"tid,omitempty"`
	TenantCode   string `json:"tcode,omitempty"`
	TenantSchema string `json:"tschema,omitempty"`
	TokenType    string `json:"typ"`
	SessionID    string `json:"sid"`
	jwt.RegisteredClaims
}

// Manager 持有签发/校验凭证所需的密钥与有效期配置，是并发安全的无状态对象。
type Manager struct {
	secret     []byte        // HS256 对称密钥
	issuer     string        // 签发方标识，校验时会比对
	accessTTL  time.Duration // access 凭证有效期
	refreshTTL time.Duration // refresh 凭证有效期
}

// New 从配置构造 Manager。
func New(cfg config.JWTConfig) *Manager {
	return &Manager{
		secret:     []byte(cfg.Secret),
		issuer:     cfg.Issuer,
		accessTTL:  cfg.AccessTTL,
		refreshTTL: cfg.RefreshTTL,
	}
}

// AccessTTL 返回访问凭证有效期。
func (m *Manager) AccessTTL() time.Duration { return m.accessTTL }

// RefreshTTL 返回刷新凭证有效期，也是会话在 Redis 中的存活时长。
func (m *Manager) RefreshTTL() time.Duration { return m.refreshTTL }

// Sign 签发指定类型的凭证，返回 token 与其有效期秒数。
func (m *Manager) Sign(claims Claims, tokenType string) (string, int64, error) {
	ttl := m.accessTTL
	if tokenType == TokenTypeRefresh {
		ttl = m.refreshTTL
	}
	now := time.Now()
	claims.TokenType = tokenType
	claims.RegisteredClaims = jwt.RegisteredClaims{
		Issuer:    m.issuer,
		Subject:   claims.Username,
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", 0, errcode.Wrap(errcode.ErrInternal, err)
	}
	return signed, int64(ttl.Seconds()), nil
}

// Parse 校验并解析凭证。
func (m *Manager) Parse(raw string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("非预期的签名算法")
		}
		return m.secret, nil
	}, jwt.WithIssuer(m.issuer), jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, errcode.New(errcode.ErrTokenExpired)
		}
		return nil, errcode.Wrap(errcode.ErrTokenInvalid, err)
	}
	return claims, nil
}
