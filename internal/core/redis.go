package core

import (
	"context"
	"time"

	"apeadmin-gin/internal/config"

	goredis "github.com/redis/go-redis/v9"
)

// ────────────────────────────────────────────────────────────────────────────
// Redis 客户端（多实例部署的可选共享层）
//
// redis.enabled=false（默认）时为 nil，全系统回退单实例内存实现；
// redis.enabled=true 时用于：token 黑名单、权限缓存、（后续）任务队列。
// ────────────────────────────────────────────────────────────────────────────

var redisClient *goredis.Client

// InitRedis 按配置初始化 Redis（bootstrap 启动时调用；未启用则跳过）
func InitRedis(cfg *config.Config) error {
	if cfg == nil || !cfg.Redis.Enabled || cfg.Redis.URL == "" {
		return nil
	}
	opts, err := goredis.ParseURL(cfg.Redis.URL)
	if err != nil {
		return err
	}
	client := goredis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return err
	}
	redisClient = client
	return nil
}

// GetRedis 返回共享 Redis 客户端（未启用时返回 nil，调用方需判空回退）
func GetRedis() *goredis.Client { return redisClient }

// CloseRedis 关闭连接（优雅停机时调用）
func CloseRedis() {
	if redisClient != nil {
		_ = redisClient.Close()
		redisClient = nil
	}
}

// RedisEnabled 是否启用 Redis
func RedisEnabled() bool { return redisClient != nil }
