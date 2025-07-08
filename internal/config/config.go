package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		DataSource: DataSourceConfig{
			Primary:    "coinbase",
			Fallback:   "",
			Timeout:    30 * time.Second,
			MaxRetries: 3,
			Binance: BinanceConfig{
				RateLimit: RateLimitConfig{
					RequestsPerMinute: 1200,
					RetryDelay:        time.Second,
					MaxRetries:        3,
				},
			},
			Coinbase: CoinbaseConfig{
				RateLimit: RateLimitConfig{
					RequestsPerMinute: 20,
					RetryDelay:        20 * time.Second,
					MaxRetries:        10,
				},
			},
		},
		Binance: BinanceConfig{
			RateLimit: RateLimitConfig{
				RequestsPerMinute: 1200,
				RetryDelay:        time.Second,
				MaxRetries:        3,
			},
		},
		Watcher: WatcherConfig{
			Interval:      5 * time.Minute,
			MaxWorkers:    10,
			BufferSize:    100,
			LogLevel:      "info",
			EnableMetrics: true,
		},
		Notifiers: NotifiersConfig{
			Email: EmailConfig{
				Enabled: false,
				SMTP: SMTPConfig{
					Host: "smtp.gmail.com",
					Port: 587,
					TLS:  true,
				},
				Subject:  "TA Watcher Alert - {{.Asset}}",
				Template: "Default template",
			},
			Feishu: FeishuConfig{
				Enabled:  false,
				Template: "Default feishu template",
			},
			Wechat: WechatConfig{
				Enabled:  false,
				Template: "Default wechat template",
			},
		},
		Assets: AssetsConfig{
			Symbols: []string{
				"BTC",
				"ETH",
				"BNB",
			},
			Timeframes: []string{
				"1d", // 日线
				"1w", // 周线
				"1M", // 月线
			},
			BaseCurrency:            "USDT",
			MarketCapUpdateInterval: 1 * time.Hour,
		},
	}
}

// LoadConfig 从文件加载配置
func LoadConfig(filename string) (*Config, error) {
	// 检查文件是否存在
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", filename)
	}

	// 读取文件内容
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 解析 YAML
	config := DefaultConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 展开环境变量
	if err := expandEnvVars(config); err != nil {
		return nil, fmt.Errorf("failed to expand environment variables: %w", err)
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return config, nil
}

// LoadConfigWithEnvFile 从文件加载配置并指定环境变量文件
func LoadConfigWithEnvFile(filename string, envFile string) (*Config, error) {
	// 初始化环境变量管理器
	if err := InitEnvManager(envFile); err != nil {
		return nil, fmt.Errorf("failed to initialize env manager: %w", err)
	}

	// 检查文件是否存在
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", filename)
	}

	// 读取文件内容
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 解析 YAML
	config := DefaultConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 展开环境变量
	if err := expandEnvVars(config); err != nil {
		return nil, fmt.Errorf("failed to expand environment variables: %w", err)
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return config, nil
}

// SaveConfig 保存配置到文件
func SaveConfig(config *Config, filename string) error {
	// 验证配置
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// 序列化为 YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Validate 验证配置有效性
func (c *Config) Validate() error {
	// 验证 DataSource 配置
	if err := c.DataSource.Validate(); err != nil {
		return fmt.Errorf("datasource config: %w", err)
	}

	// 验证 Binance 配置
	if err := c.Binance.Validate(); err != nil {
		return fmt.Errorf("binance config: %w", err)
	}

	// 验证 Watcher 配置
	if err := c.Watcher.Validate(); err != nil {
		return fmt.Errorf("watcher config: %w", err)
	}

	// 验证 Notifiers 配置
	if err := c.Notifiers.Validate(); err != nil {
		return fmt.Errorf("notifiers config: %w", err)
	}

	// 验证资产配置
	if err := c.Assets.Validate(); err != nil {
		return fmt.Errorf("invalid assets config: %w", err)
	}

	return nil
}

// Validate 验证 DataSource 配置
func (c *DataSourceConfig) Validate() error {
	if c.Primary == "" {
		return fmt.Errorf("primary datasource cannot be empty")
	}

	supportedSources := []string{"binance", "coinbase"}
	primaryValid := false
	for _, source := range supportedSources {
		if c.Primary == source {
			primaryValid = true
			break
		}
	}
	if !primaryValid {
		return fmt.Errorf("unsupported primary datasource: %s", c.Primary)
	}

	// 验证 Binance 配置
	if err := c.Binance.Validate(); err != nil {
		return fmt.Errorf("binance config: %w", err)
	}

	// 验证 Coinbase 配置
	if err := c.Coinbase.Validate(); err != nil {
		return fmt.Errorf("coinbase config: %w", err)
	}

	return nil
}

// Validate 验证 Binance 配置
func (c *BinanceConfig) Validate() error {
	if c.RateLimit.RequestsPerMinute <= 0 {
		return fmt.Errorf("requests_per_minute must be positive")
	}
	if c.RateLimit.MaxRetries < 0 {
		return fmt.Errorf("max_retries cannot be negative")
	}
	return nil
}

// Validate 验证 Coinbase 配置
func (c *CoinbaseConfig) Validate() error {
	if c.RateLimit.RequestsPerMinute <= 0 {
		return fmt.Errorf("requests_per_minute must be positive")
	}
	if c.RateLimit.MaxRetries < 0 {
		return fmt.Errorf("max_retries cannot be negative")
	}
	return nil
}

// Validate 验证 Watcher 配置
func (c *WatcherConfig) Validate() error {
	if c.Interval <= 0 {
		return fmt.Errorf("interval must be positive")
	}
	if c.MaxWorkers <= 0 {
		return fmt.Errorf("max_workers must be positive")
	}
	if c.BufferSize <= 0 {
		return fmt.Errorf("buffer_size must be positive")
	}
	validLogLevels := []string{"debug", "info", "warn", "error"}
	valid := false
	for _, level := range validLogLevels {
		if c.LogLevel == level {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid log_level: %s, must be one of %v", c.LogLevel, validLogLevels)
	}
	return nil
}

// Validate 验证 Notifiers 配置
func (c *NotifiersConfig) Validate() error {
	if err := c.Email.Validate(); err != nil {
		return fmt.Errorf("email: %w", err)
	}
	if err := c.Feishu.Validate(); err != nil {
		return fmt.Errorf("feishu: %w", err)
	}
	if err := c.Wechat.Validate(); err != nil {
		return fmt.Errorf("wechat: %w", err)
	}
	return nil
}

// Validate 验证邮件配置
func (c *EmailConfig) Validate() error {
	if !c.Enabled {
		return nil // 未启用时不验证
	}

	if c.SMTP.Host == "" {
		return fmt.Errorf("smtp host cannot be empty")
	}
	if c.SMTP.Port <= 0 || c.SMTP.Port > 65535 {
		return fmt.Errorf("invalid smtp port: %d", c.SMTP.Port)
	}
	if c.SMTP.Username == "" {
		return fmt.Errorf("smtp username cannot be empty")
	}
	if c.SMTP.Password == "" {
		return fmt.Errorf("smtp password cannot be empty")
	}
	if c.From == "" {
		return fmt.Errorf("from email cannot be empty")
	}
	if len(c.To) == 0 {
		return fmt.Errorf("to email list cannot be empty")
	}

	return nil
}

// Validate 验证飞书配置
func (c *FeishuConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.WebhookURL == "" {
		return fmt.Errorf("webhook_url cannot be empty")
	}
	return nil
}

// Validate 验证微信配置
func (c *WechatConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.WebhookURL == "" {
		return fmt.Errorf("webhook_url cannot be empty")
	}
	return nil
}

// Validate 验证资产配置
func (a *AssetsConfig) Validate() error {
	// 验证加密货币列表
	if len(a.Symbols) == 0 {
		return fmt.Errorf("symbols list cannot be empty")
	}

	// 验证时间框架
	if len(a.Timeframes) == 0 {
		return fmt.Errorf("timeframes list cannot be empty")
	}

	// 验证支持的时间框架
	validTimeframes := map[string]bool{
		"1m": true, "3m": true, "5m": true, "15m": true, "30m": true,
		"1h": true, "2h": true, "4h": true, "6h": true, "8h": true, "12h": true,
		"1d": true, "3d": true, "1w": true, "1M": true,
	}

	for _, tf := range a.Timeframes {
		if !validTimeframes[tf] {
			return fmt.Errorf("invalid timeframe: %s", tf)
		}
	}

	// 验证基准货币
	if a.BaseCurrency == "" {
		return fmt.Errorf("base_currency cannot be empty")
	}

	// 验证市值更新间隔
	if a.MarketCapUpdateInterval <= 0 {
		return fmt.Errorf("market_cap_update_interval must be positive")
	}

	return nil
}
