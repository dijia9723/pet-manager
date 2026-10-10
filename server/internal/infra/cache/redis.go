// Package cache 封装 Redis 客户端，统一 key 前缀，避免各模块各写一套。
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"pet-manager/server/internal/config"
)

// Client 是对 go-redis 的薄封装，额外统一维护 key 前缀。
type Client struct {
	rdb    *redis.Client // 底层客户端
	prefix string        // 全局 key 前缀，用于多环境隔离与批量清理
}

// New 建立连接并做一次 Ping 探活，连接不通直接返回错误，避免带病启动。
func New(cfg config.RedisConfig) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("连接 Redis 失败: %w", err)
	}
	return &Client{rdb: rdb, prefix: cfg.KeyPrefix}, nil
}

// Raw 暴露底层客户端，供封装未覆盖的命令（如 Incr/Exists/Scan）使用。
func (c *Client) Raw() *redis.Client { return c.rdb }

// Key 统一加前缀，便于按前缀批量清理与隔离多环境。
func (c *Client) Key(parts ...string) string {
	out := c.prefix
	for i, p := range parts {
		if i > 0 {
			out += ":"
		}
		out += p
	}
	return out
}

// Set 写入键值并设置 TTL（ttl<=0 表示永不过期）。
func (c *Client) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

// Get 读取字符串值；键不存在时返回 redis.Nil 错误，调用方需自行判空。
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.rdb.Get(ctx, key).Result()
}

// Del 删除若干键；空列表直接返回，避免无意义的往返。
func (c *Client) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.rdb.Del(ctx, keys...).Err()
}

// DelByPrefix 用 SCAN 批量删除，用于权限变更后失效缓存。
func (c *Client) DelByPrefix(ctx context.Context, prefix string) error {
	var cursor uint64
	for {
		keys, next, err := c.rdb.Scan(ctx, cursor, prefix+"*", 200).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		if next == 0 {
			return nil
		}
		cursor = next
	}
}

// AcquireLock 简易分布式锁，用于租户开通等需要串行化的操作。
func (c *Client) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return c.rdb.SetNX(ctx, key, "1", ttl).Result()
}

// ReleaseLock 释放简单锁（直接删除 key）。
func (c *Client) ReleaseLock(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}

// Close 关闭连接池。
func (c *Client) Close() error { return c.rdb.Close() }
