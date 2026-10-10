// Package session 统一管理登录态与鉴权相关缓存。
//
// 这里同时承载三类数据，它们生命周期高度相关，放一起便于统一失效：
//   - 会话：access/refresh 共用一个 sid，注销即删 key，实现"可强制下线"；
//   - 用户权限点：按租户+用户缓存，角色变更时按租户批量失效；
//   - 租户快照：状态/到期时间，避免每个请求回查平台库。
package session

import (
	"context"
	"encoding/json"
	"time"

	"pet-manager/server/internal/infra/cache"
)

const (
	permCacheTTL   = 10 * time.Minute
	tenantCacheTTL = 5 * time.Minute
)

// TenantInfo 是租户的只读快照，用于请求链路做状态门禁。
type TenantInfo struct {
	ID       uint64     `json:"id"`
	Code     string     `json:"code"`
	Schema   string     `json:"schema"`
	Name     string     `json:"name"`
	Status   string     `json:"status"`
	ExpireAt *time.Time `json:"expire_at"`
}

// IsExpired 判断租户是否已过期（ExpireAt 为空视为永久有效）。
func (t TenantInfo) IsExpired(now time.Time) bool {
	return t.ExpireAt != nil && t.ExpireAt.Before(now)
}

// Store 是登录态与鉴权缓存的统一门面。
type Store struct {
	c *cache.Client
}

// New 构造 Store。
func New(c *cache.Client) *Store { return &Store{c: c} }

// —— 会话 ——

func (s *Store) sessionKey(userType string, userID uint64, sid string) string {
	return s.c.Key("session", userType, itoa(userID), sid)
}

// userSessionPrefix 是某账号全部会话 key 的前缀，用于按前缀踢下线。
func (s *Store) userSessionPrefix(userType string, userID uint64) string {
	return s.c.Key("session", userType, itoa(userID)) + ":"
}

// SaveSession 建立会话。TTL 取 refresh 有效期，保证 refresh 到期前会话一直有效。
func (s *Store) SaveSession(ctx context.Context, userType string, userID uint64, sid string, ttl time.Duration) error {
	return s.c.Set(ctx, s.sessionKey(userType, userID, sid), "1", ttl)
}

// SessionAlive 校验会话是否仍然有效（未被注销）。
func (s *Store) SessionAlive(ctx context.Context, userType string, userID uint64, sid string) (bool, error) {
	n, err := s.c.Raw().Exists(ctx, s.sessionKey(userType, userID, sid)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// RevokeSession 注销单个会话（登出）。
func (s *Store) RevokeSession(ctx context.Context, userType string, userID uint64, sid string) error {
	return s.c.Del(ctx, s.sessionKey(userType, userID, sid))
}

// RevokeUserSessions 踢掉某账号的全部会话（改密、停用、删除账号时调用）。
func (s *Store) RevokeUserSessions(ctx context.Context, userType string, userID uint64) error {
	return s.c.DelByPrefix(ctx, s.userSessionPrefix(userType, userID))
}

// —— 权限点缓存 ——

func (s *Store) permKey(tenantID, userID uint64) string {
	return s.c.Key("perm", "t", itoa(tenantID), itoa(userID))
}

// tenantPermPrefix 是某租户全部账号权限点 key 的前缀，角色变更时按前缀失效。
func (s *Store) tenantPermPrefix(tenantID uint64) string {
	return s.c.Key("perm", "t", itoa(tenantID)) + ":"
}

// CachedPermissions 读取权限点缓存。
func (s *Store) CachedPermissions(ctx context.Context, tenantID, userID uint64) ([]string, bool) {
	raw, err := s.c.Get(ctx, s.permKey(tenantID, userID))
	if err != nil || raw == "" {
		return nil, false
	}
	var perms []string
	if err := json.Unmarshal([]byte(raw), &perms); err != nil {
		return nil, false
	}
	return perms, true
}

// CachePermissions 写入权限点缓存，TTL 由 permCacheTTL 决定。
func (s *Store) CachePermissions(ctx context.Context, tenantID, userID uint64, perms []string) error {
	raw, err := json.Marshal(perms)
	if err != nil {
		return err
	}
	return s.c.Set(ctx, s.permKey(tenantID, userID), raw, permCacheTTL)
}

// InvalidateTenantPermissions 租户内角色/授权变更后整体失效，下一次请求自动重建。
func (s *Store) InvalidateTenantPermissions(ctx context.Context, tenantID uint64) error {
	return s.c.DelByPrefix(ctx, s.tenantPermPrefix(tenantID))
}

// InvalidateAllPermissions 权限点字典本身增删改后，全量失效（跨所有租户）。
// 权限点变更影响每个租户的解析结果，无法按租户定位，只能整前缀清理。
func (s *Store) InvalidateAllPermissions(ctx context.Context) error {
	return s.c.DelByPrefix(ctx, s.c.Key("perm"))
}

// —— 租户快照缓存 ——

// tenantKey 是单个租户快照的缓存键。
func (s *Store) tenantKey(id uint64) string { return s.c.Key("tenant", itoa(id)) }

// CachedTenant 读取租户快照；未命中或反序列化失败都返回 false。
func (s *Store) CachedTenant(ctx context.Context, id uint64) (*TenantInfo, bool) {
	raw, err := s.c.Get(ctx, s.tenantKey(id))
	if err != nil || raw == "" {
		return nil, false
	}
	var info TenantInfo
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		return nil, false
	}
	return &info, true
}

// CacheTenant 写入租户快照，TTL 由 tenantCacheTTL 决定。
func (s *Store) CacheTenant(ctx context.Context, info TenantInfo) error {
	raw, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return s.c.Set(ctx, s.tenantKey(info.ID), raw, tenantCacheTTL)
}

// InvalidateTenant 租户信息变更（停用/续费/资料修改）后失效缓存。
func (s *Store) InvalidateTenant(ctx context.Context, id uint64) error {
	return s.c.Del(ctx, s.tenantKey(id))
}

// itoa 是 uint64 转十进制字符串的最小实现，避免为此引入额外依赖。
func itoa(v uint64) string {
	// 避免引入 strconv 之外的依赖，这里用最朴素的实现即可
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
