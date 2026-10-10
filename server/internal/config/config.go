// Package config 负责加载应用配置：以 YAML 文件为基准，允许用 PM_ 前缀的环境变量逐项覆盖。
//
// 约定：环境变量名 = "PM_" + 配置路径的大写并把 "." 换成 "_"，
// 例如 database.host 对应 PM_DATABASE_HOST。
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config 是应用的完整配置树。
// 约定：所有配置项都可以用 PM_ 前缀的环境变量覆盖，例如 PM_DATABASE_PASSWORD。
type Config struct {
	App      AppConfig      `mapstructure:"app"`      // 应用元信息（名称、环境、监听地址）
	Log      LogConfig      `mapstructure:"log"`      // 日志（级别、格式、输出目标）
	Database DatabaseConfig `mapstructure:"database"` // PostgreSQL 连接池
	Redis    RedisConfig    `mapstructure:"redis"`    // Redis（会话、权限缓存、限流）
	JWT      JWTConfig      `mapstructure:"jwt"`      // JWT 签发参数
	CORS     CORSConfig     `mapstructure:"cors"`     // 跨域白名单
	Seed     SeedConfig     `mapstructure:"seed"`     // 首次启动的初始化数据
	Jobs     JobsConfig     `mapstructure:"jobs"`     // 定时任务/Shell 执行配置
	TOS      TOSConfig      `mapstructure:"tos"`      // 对象存储
	Hosting  HostingConfig  `mapstructure:"hosting"`  // 托管相关密钥
}

// HostingConfig encrypts SSH keys at rest. Use a stable base64-encoded 32-byte key.
type HostingConfig struct {
	EncryptionKey string `mapstructure:"encryption_key"` // 用于加密托管 SSH 私钥的对称密钥（base64 编码的 32 字节）
}

// TOSConfig 是平台统一对象存储；访问凭证不写入业务字典或 HTTP 响应。
type TOSConfig struct {
	Enabled        bool          `mapstructure:"enabled"`          // 是否启用对象存储，关闭时 storage 模块退化为占位实现
	Endpoint       string        `mapstructure:"endpoint"`         // 服务地址
	Region         string        `mapstructure:"region"`           // 区域
	AccessKey      string        `mapstructure:"access_key"`       // 访问密钥 ID（敏感）
	SecretKey      string        `mapstructure:"secret_key"`       // 访问密钥（敏感）
	Bucket         string        `mapstructure:"bucket"`           // 存储桶
	KeyPrefix      string        `mapstructure:"key_prefix"`       // 对象键前缀，默认 apps/
	PublicBaseURL  string        `mapstructure:"public_base_url"`  // 公共访问域名（CDN 等），为空则用预签名 URL
	PresignTTL     time.Duration `mapstructure:"presign_ttl"`      // 预签名 URL 有效期
	MaxUploadBytes int64         `mapstructure:"max_upload_bytes"` // 单文件上传大小上限（字节）
}

// JobsConfig 配置 Shell 任务在服务器上使用的 sh 可执行文件。
type JobsConfig struct {
	ShellPath string `mapstructure:"shell_path"` // Shell 解释器路径，Windows 下常改为 bash/sh
}

// SeedConfig 是首次启动时的初始化数据。
// 平台管理员口令必须能在部署时覆盖（PM_SEED_PLATFORM_ADMIN_PASSWORD），不要写死在代码里。
type SeedConfig struct {
	PlatformAdminUsername string `mapstructure:"platform_admin_username"` // 平台超级管理员账号，仅首次初始化时创建
	PlatformAdminPassword string `mapstructure:"platform_admin_password"` // 平台超级管理员初始口令，部署时务必覆盖
}

// AppConfig 描述应用自身的基础属性。
type AppConfig struct {
	Name string `mapstructure:"name"` // 应用名，用于日志与探活标识
	Env  string `mapstructure:"env"`  // 运行环境：dev / test / prod，影响日志与 Swagger 开关
	Addr string `mapstructure:"addr"` // HTTP 监听地址，形如 :8088
}

// IsProd 用于切换日志、Swagger 等开发态行为。
func (a AppConfig) IsProd() bool { return a.Env == "prod" }

// LogConfig 描述日志输出行为。
type LogConfig struct {
	Level  string `mapstructure:"level"`  // 日志级别：debug/info/warn/error
	Format string `mapstructure:"format"` // 输出格式：console（可读）或 json（机器采集）
	Output string `mapstructure:"output"` // 输出目标：stdout/stderr 或文件路径
}

// DatabaseConfig 描述 PostgreSQL 连接与连接池参数。
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`              // 主机
	Port            int           `mapstructure:"port"`              // 端口
	User            string        `mapstructure:"user"`              // 用户名
	Password        string        `mapstructure:"password"`          // 口令（敏感）
	DBName          string        `mapstructure:"dbname"`            // 数据库名
	SSLMode         string        `mapstructure:"sslmode"`           // SSL 模式：disable/require 等
	TimeZone        string        `mapstructure:"timezone"`          // 会话时区
	MaxOpenConns    int           `mapstructure:"max_open_conns"`    // 最大连接数
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`    // 最大空闲连接数
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"` // 单个连接最长存活时间，防连接老化
	PlatformSchema  string        `mapstructure:"platform_schema"`   // 平台公共库 schema，默认 public
	LogLevel        string        `mapstructure:"log_level"`         // SQL 日志级别
}

// DSN 返回平台连接串。默认 search_path 固定指向平台公共库，
// 租户 schema 的切换一律走 TenantTx / TenantScope，不依赖连接级配置。
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=%s search_path=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode, d.TimeZone, d.PlatformSchema,
	)
}

// RedisConfig 描述 Redis 连接参数。
type RedisConfig struct {
	Addr      string `mapstructure:"addr"`       // 地址 host:port
	Password  string `mapstructure:"password"`   // 口令（敏感）
	DB        int    `mapstructure:"db"`         // 逻辑库编号
	PoolSize  int    `mapstructure:"pool_size"`  // 连接池大小
	KeyPrefix string `mapstructure:"key_prefix"` // 键前缀，便于多环境共用实例时隔离
}

// JWTConfig 描述访问令牌的签发参数。
type JWTConfig struct {
	Secret     string        `mapstructure:"secret"`      // 签名密钥，生产环境必须覆盖
	Issuer     string        `mapstructure:"issuer"`      // 签发者标识
	AccessTTL  time.Duration `mapstructure:"access_ttl"`  // 访问令牌有效期
	RefreshTTL time.Duration `mapstructure:"refresh_ttl"` // 刷新令牌有效期
}

// CORSConfig 描述跨域白名单。
type CORSConfig struct {
	AllowOrigins []string `mapstructure:"allow_origins"` // 允许的前端来源，逐项精确匹配
}

// Load 从指定路径加载配置；path 为空时按 ./configs/config.yaml 查找。
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	// 显式路径优先；否则按 config.yaml 在 ./configs 与当前目录查找
	if path != "" {
		v.SetConfigFile(path)
	} else {
		v.SetConfigName("config")
		v.AddConfigPath("./configs")
		v.AddConfigPath(".")
	}

	// 先注册默认值：viper 的 AutomaticEnv 只能覆盖已注册的 key，缺省项才能被环境变量生效
	defaults(v)

	// 开启 PM_ 前缀环境变量覆盖：database.host -> PM_DATABASE_HOST
	v.SetEnvPrefix("PM")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		// 允许"只有环境变量"的无文件启动，方便容器化部署
		var notFound viper.ConfigFileNotFoundError
		if !isNotFound(err, &notFound) && path != "" {
			return nil, fmt.Errorf("读取配置文件失败: %w", err)
		}
	}

	// 把最终配置（默认值 + 文件 + 环境变量）反序列化到结构体
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	return &cfg, nil
}

// isNotFound 判断读取错误是否为"配置文件不存在"，用于放过无文件启动的场景。
func isNotFound(err error, target *viper.ConfigFileNotFoundError) bool {
	if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		return true
	}
	_ = target
	return false
}

// defaults 注册所有配置项的默认值，保证零配置文件也能安全启动（口令类除外，生产必须覆盖）。
func defaults(v *viper.Viper) {
	v.SetDefault("hosting.encryption_key", "")
	v.SetDefault("tos.enabled", false)
	v.SetDefault("tos.endpoint", "")
	v.SetDefault("tos.region", "")
	v.SetDefault("tos.access_key", "")
	v.SetDefault("tos.secret_key", "")
	v.SetDefault("tos.bucket", "")
	v.SetDefault("tos.key_prefix", "apps/")
	v.SetDefault("tos.public_base_url", "")
	v.SetDefault("tos.presign_ttl", "10m")
	v.SetDefault("tos.max_upload_bytes", 100*1024*1024)
	v.SetDefault("app.name", "pet-manager-api")
	v.SetDefault("app.env", "dev")
	v.SetDefault("app.addr", ":8080")
	v.SetDefault("jobs.shell_path", "sh")

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "console")
	v.SetDefault("log.output", "stdout")

	v.SetDefault("database.host", "127.0.0.1")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.password", "postgres")
	v.SetDefault("database.dbname", "pet_manager")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.timezone", "Asia/Shanghai")
	v.SetDefault("database.max_open_conns", 50)
	v.SetDefault("database.max_idle_conns", 10)
	v.SetDefault("database.conn_max_lifetime", "1h")
	v.SetDefault("database.platform_schema", "public")
	v.SetDefault("database.log_level", "warn")

	v.SetDefault("redis.addr", "127.0.0.1:6379")
	// 必须显式注册：viper 的 AutomaticEnv 只能覆盖已注册的 key，
	// 缺省时 Unmarshal 不会纳入 AllKeys，PM_REDIS_PASSWORD 会被静默忽略。
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.pool_size", 100)
	v.SetDefault("redis.key_prefix", "pm:")

	v.SetDefault("jwt.secret", "please-change-me-in-production")
	v.SetDefault("jwt.issuer", "pet-manager")
	v.SetDefault("jwt.access_ttl", "2h")
	v.SetDefault("jwt.refresh_ttl", "168h")

	v.SetDefault("cors.allow_origins", []string{"http://localhost:5173", "http://127.0.0.1:5173"})

	v.SetDefault("seed.platform_admin_username", "admin")
	v.SetDefault("seed.platform_admin_password", "Admin@123456")
}
