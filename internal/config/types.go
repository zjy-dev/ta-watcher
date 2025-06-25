package config

import (
	"time"
)

// Config 应用程序配置
type Config struct {
	// 数据源配置
	DataSource DataSourceConfig `yaml:"datasource"`

	// Binance 配置
	Binance BinanceConfig `yaml:"binance"`

	// 监控配置
	Watcher WatcherConfig `yaml:"watcher"`

	// 通知配置
	Notifiers NotifiersConfig `yaml:"notifiers"`

	// 资产配置
	Assets AssetsConfig `yaml:"assets"`
}

// AssetsConfig 资产配置
type AssetsConfig struct {
	// 要监控的加密货币列表
	Symbols []string `yaml:"symbols"`

	// 支持的时间框架
	Timeframes []string `yaml:"timeframes"`

	// 基准货币（用于汇率计算）
	BaseCurrency string `yaml:"base_currency"`

	// 市值数据更新间隔
	MarketCapUpdateInterval time.Duration `yaml:"market_cap_update_interval"`
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

// DataSourceConfig 数据源配置
type DataSourceConfig struct {
	Primary    string        `yaml:"primary"`     // 主数据源: binance, coinbase
	Fallback   string        `yaml:"fallback"`    // 备用数据源
	Timeout    time.Duration `yaml:"timeout"`     // 请求超时时间
	MaxRetries int           `yaml:"max_retries"` // 最大重试次数

	Binance  BinanceConfig  `yaml:"binance"`  // Binance 配置
	Coinbase CoinbaseConfig `yaml:"coinbase"` // Coinbase 配置
}

// CoinbaseConfig Coinbase 配置
type CoinbaseConfig struct {
	RateLimit RateLimitConfig `yaml:"rate_limit"`
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
