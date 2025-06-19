package config

import (
	"time"
)

// Config 应用程序配置
type Config struct {
	// Binance 配置
	Binance BinanceConfig `yaml:"binance"`

	// 监控配置
	Watcher WatcherConfig `yaml:"watcher"`

	// 通知配置
	Notifiers NotifiersConfig `yaml:"notifiers"`

	// 资产配置
	Assets []string `yaml:"assets"`

	// 策略配置
	Strategies []StrategyConfig `yaml:"strategies"`
}

// BinanceConfig Binance 配置
type BinanceConfig struct {
	// 限流配置
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	RequestsPerMinute int           `yaml:"requests_per_minute"` // 每分钟请求数
	RetryDelay        time.Duration `yaml:"retry_delay"`         // 重试延迟
	MaxRetries        int           `yaml:"max_retries"`         // 最大重试次数
}

// WatcherConfig 监控配置
type WatcherConfig struct {
	Interval      time.Duration `yaml:"interval"`       // 监控间隔
	MaxWorkers    int           `yaml:"max_workers"`    // 最大工作协程数
	BufferSize    int           `yaml:"buffer_size"`    // 缓冲区大小
	LogLevel      string        `yaml:"log_level"`      // 日志级别
	EnableMetrics bool          `yaml:"enable_metrics"` // 是否启用指标收集
}

// NotifiersConfig 通知配置
type NotifiersConfig struct {
	Email  EmailConfig  `yaml:"email"`  // 邮件通知
	Feishu FeishuConfig `yaml:"feishu"` // 飞书通知
	Wechat WechatConfig `yaml:"wechat"` // 微信通知
}

// EmailConfig 邮件配置
type EmailConfig struct {
	Enabled  bool       `yaml:"enabled"`  // 是否启用
	SMTP     SMTPConfig `yaml:"smtp"`     // SMTP 配置
	From     string     `yaml:"from"`     // 发送者邮箱
	To       []string   `yaml:"to"`       // 接收者邮箱列表
	Subject  string     `yaml:"subject"`  // 邮件主题模板
	Template string     `yaml:"template"` // 邮件内容模板
}

// SMTPConfig SMTP 配置
type SMTPConfig struct {
	Host     string `yaml:"host"`     // SMTP 服务器地址
	Port     int    `yaml:"port"`     // SMTP 端口
	Username string `yaml:"username"` // 用户名
	Password string `yaml:"password"` // 密码
	TLS      bool   `yaml:"tls"`      // 是否使用 TLS
}

// FeishuConfig 飞书配置
type FeishuConfig struct {
	Enabled    bool   `yaml:"enabled"`     // 是否启用
	WebhookURL string `yaml:"webhook_url"` // Webhook URL
	Secret     string `yaml:"secret"`      // 签名密钥
	Template   string `yaml:"template"`    // 消息模板
}

// WechatConfig 微信配置
type WechatConfig struct {
	Enabled    bool   `yaml:"enabled"`     // 是否启用
	WebhookURL string `yaml:"webhook_url"` // Webhook URL
	Template   string `yaml:"template"`    // 消息模板
}

// StrategyConfig 策略配置
type StrategyConfig struct {
	Name     string                 `yaml:"name"`     // 策略名称
	Enabled  bool                   `yaml:"enabled"`  // 是否启用
	Assets   []string               `yaml:"assets"`   // 监控资产
	Params   map[string]interface{} `yaml:"params"`   // 策略参数
	Interval string                 `yaml:"interval"` // K线间隔
}
