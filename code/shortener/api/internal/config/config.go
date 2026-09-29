// Package config 负责从环境变量读取后端的全部可调参数。
//
// 之所以只从环境变量读取而不读取配置文件，是因为后面两个阶段都要把配置注入到容器里：
// Docker Compose 通过 services 下面的 environment 字段注入，
// Kubernetes 通过 ConfigMap 与 Secret 注入，这两种方式最终都是设置进程环境变量。
// 也就是说，这一层的读取方式与部署方式是同一套约定，容器阶段不需要修改任何代码。
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config 保存后端的全部可调参数。
// 字段与接口契约、健康检查、后台写回这三块功能一一对应，没有多余字段。
type Config struct {
	// HTTPAddr 是 HTTP 服务的监听地址，例如 ":8080"。
	HTTPAddr string
	// DatabaseURL 是 PostgreSQL 的连接字符串，格式为
	// postgres://用户:口令@主机:端口/数据库?sslmode=disable。
	DatabaseURL string
	// RedisAddr 是 Redis 的地址，格式为「主机:端口」。
	RedisAddr string
	// RedisPassword 是 Redis 的口令。本地联调的 Redis 没有设置口令，因此默认值是空字符串。
	RedisPassword string
	// RedisDB 是 Redis 的逻辑数据库编号。
	RedisDB int
	// CacheTTL 是短链接缓存键的生存时间，对应接口契约中 link:{code} 键的 300 秒。
	CacheTTL time.Duration
	// ClickFlushInterval 是后台协程把点击增量写回 PostgreSQL 的周期。
	ClickFlushInterval time.Duration
	// ShutdownTimeout 是收到停止信号之后等待 HTTP 服务退出与写回操作完成的时限。
	ShutdownTimeout time.Duration
}

// 默认值的集中定义。把默认值写成常量而不是散落在 Load 函数里，
// 目的是让「哪些参数可以不设置」这件事一目了然。
const (
	defaultHTTPAddr           = ":8080"
	defaultDatabaseURL        = "postgres://shortener:shortener_dev_password@localhost:5432/shortener?sslmode=disable"
	defaultRedisAddr          = "localhost:6379"
	defaultRedisDB            = 0
	defaultCacheTTL           = 300 * time.Second
	defaultClickFlushInterval = 5 * time.Second
	defaultShutdownTimeout    = 10 * time.Second
)

// Load 读取环境变量并且填充 Config。
// 取值规则是：环境变量已设置并且可以解析时使用环境变量的取值，否则使用上面定义的默认值。
// 只有取值不能为空的两个字段（数据库连接串与 Redis 地址）在缺少取值时返回错误。
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:      getString("HTTP_ADDR", defaultHTTPAddr),
		DatabaseURL:   getString("DATABASE_URL", defaultDatabaseURL),
		RedisAddr:     getString("REDIS_ADDR", defaultRedisAddr),
		RedisPassword: getString("REDIS_PASSWORD", ""),
	}

	var err error
	if cfg.RedisDB, err = getInt("REDIS_DB", defaultRedisDB); err != nil {
		return Config{}, err
	}
	if cfg.CacheTTL, err = getDuration("CACHE_TTL", defaultCacheTTL); err != nil {
		return Config{}, err
	}
	if cfg.ClickFlushInterval, err = getDuration("CLICK_FLUSH_INTERVAL", defaultClickFlushInterval); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = getDuration("SHUTDOWN_TIMEOUT", defaultShutdownTimeout); err != nil {
		return Config{}, err
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("环境变量 DATABASE_URL 不能为空")
	}
	if cfg.RedisAddr == "" {
		return Config{}, fmt.Errorf("环境变量 REDIS_ADDR 不能为空")
	}
	if cfg.CacheTTL <= 0 {
		return Config{}, fmt.Errorf("环境变量 CACHE_TTL 必须大于 0，当前取值是 %s", cfg.CacheTTL)
	}
	if cfg.ClickFlushInterval <= 0 {
		return Config{}, fmt.Errorf("环境变量 CLICK_FLUSH_INTERVAL 必须大于 0，当前取值是 %s", cfg.ClickFlushInterval)
	}

	return cfg, nil
}

// Summary 返回用于启动日志的配置摘要。
// 本函数刻意不包含 DatabaseURL 与 RedisPassword：DatabaseURL 里面带有数据库口令，
// 把口令写进日志会让任何能够读取日志的人获得数据库凭据，这是配置输出必须避免的一件事。
func (c Config) Summary() string {
	return fmt.Sprintf(
		"监听地址=%s Redis地址=%s Redis库编号=%d 缓存生存时间=%s 点击写回周期=%s 优雅退出时限=%s",
		c.HTTPAddr, c.RedisAddr, c.RedisDB, c.CacheTTL, c.ClickFlushInterval, c.ShutdownTimeout,
	)
}

// getString 读取字符串型环境变量，变量不存在或者为空字符串时返回默认值。
func getString(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

// getInt 读取整数型环境变量。变量不存在时返回默认值，取值无法解析为整数时返回错误。
func getInt(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("环境变量 %s 的取值 %q 不是合法的整数", key, value)
	}
	return parsed, nil
}

// getDuration 读取时间段型环境变量，取值写法与 time.ParseDuration 一致，例如 "300s"、"5s"、"1m30s"。
func getDuration(key string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("环境变量 %s 的取值 %q 不是合法的时间段，正确写法例如 300s 或者 1m30s", key, value)
	}
	return parsed, nil
}
