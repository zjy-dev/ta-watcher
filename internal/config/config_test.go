package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 1200, config.Binance.RateLimit.RequestsPerMinute)
	assert.Equal(t, time.Second, config.Binance.RateLimit.RetryDelay)
	assert.Equal(t, 3, config.Binance.RateLimit.MaxRetries)

	assert.Equal(t, 5*time.Minute, config.Watcher.Interval)
	assert.Equal(t, 10, config.Watcher.MaxWorkers)
	assert.Equal(t, 100, config.Watcher.BufferSize)
	assert.Equal(t, "info", config.Watcher.LogLevel)
	assert.True(t, config.Watcher.EnableMetrics)

	assert.False(t, config.Notifiers.Email.Enabled)
	assert.Equal(t, "smtp.gmail.com", config.Notifiers.Email.SMTP.Host)
	assert.Equal(t, 587, config.Notifiers.Email.SMTP.Port)
	assert.True(t, config.Notifiers.Email.SMTP.TLS)

	assert.Len(t, config.Assets.Symbols, 3)
	assert.Contains(t, config.Assets.Symbols, "BTC")
	assert.Contains(t, config.Assets.Symbols, "ETH")
	assert.Contains(t, config.Assets.Symbols, "BNB")
	assert.Equal(t, "USDT", config.Assets.BaseCurrency)
	assert.Len(t, config.Assets.Timeframes, 3)
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid default config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "invalid binance rate limit",
			config: func() *Config {
				c := DefaultConfig()
				c.Binance.RateLimit.RequestsPerMinute = 0
				return c
			}(),
			wantErr: true,
			errMsg:  "requests_per_minute must be positive",
		},
		{
			name: "invalid watcher interval",
			config: func() *Config {
				c := DefaultConfig()
				c.Watcher.Interval = 0
				return c
			}(),
			wantErr: true,
			errMsg:  "interval must be positive",
		},
		{
			name: "invalid log level",
			config: func() *Config {
				c := DefaultConfig()
				c.Watcher.LogLevel = "invalid"
				return c
			}(),
			wantErr: true,
			errMsg:  "invalid log_level",
		},
		{
			name: "empty assets",
			config: func() *Config {
				c := DefaultConfig()
				c.Assets.Symbols = []string{}
				return c
			}(),
			wantErr: true,
			errMsg:  "symbols list cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEmailConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *EmailConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "disabled email config",
			config: &EmailConfig{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name: "valid email config",
			config: &EmailConfig{
				Enabled: true,
				SMTP: SMTPConfig{
					Host:     "smtp.gmail.com",
					Port:     587,
					Username: "test@gmail.com",
					Password: "password",
					TLS:      true,
				},
				From: "test@gmail.com",
				To:   []string{"recipient@gmail.com"},
			},
			wantErr: false,
		},
		{
			name: "missing smtp host",
			config: &EmailConfig{
				Enabled: true,
				SMTP: SMTPConfig{
					Port:     587,
					Username: "test@gmail.com",
					Password: "password",
				},
				From: "test@gmail.com",
				To:   []string{"recipient@gmail.com"},
			},
			wantErr: true,
			errMsg:  "smtp host cannot be empty",
		},
		{
			name: "invalid smtp port",
			config: &EmailConfig{
				Enabled: true,
				SMTP: SMTPConfig{
					Host:     "smtp.gmail.com",
					Port:     0,
					Username: "test@gmail.com",
					Password: "password",
				},
				From: "test@gmail.com",
				To:   []string{"recipient@gmail.com"},
			},
			wantErr: true,
			errMsg:  "invalid smtp port",
		},
		{
			name: "missing username",
			config: &EmailConfig{
				Enabled: true,
				SMTP: SMTPConfig{
					Host:     "smtp.gmail.com",
					Port:     587,
					Password: "password",
				},
				From: "test@gmail.com",
				To:   []string{"recipient@gmail.com"},
			},
			wantErr: true,
			errMsg:  "smtp username cannot be empty",
		},
		{
			name: "missing password",
			config: &EmailConfig{
				Enabled: true,
				SMTP: SMTPConfig{
					Host:     "smtp.gmail.com",
					Port:     587,
					Username: "test@gmail.com",
				},
				From: "test@gmail.com",
				To:   []string{"recipient@gmail.com"},
			},
			wantErr: true,
			errMsg:  "smtp password cannot be empty",
		},
		{
			name: "missing from email",
			config: &EmailConfig{
				Enabled: true,
				SMTP: SMTPConfig{
					Host:     "smtp.gmail.com",
					Port:     587,
					Username: "test@gmail.com",
					Password: "password",
				},
				To: []string{"recipient@gmail.com"},
			},
			wantErr: true,
			errMsg:  "from email cannot be empty",
		},
		{
			name: "empty to email list",
			config: &EmailConfig{
				Enabled: true,
				SMTP: SMTPConfig{
					Host:     "smtp.gmail.com",
					Port:     587,
					Username: "test@gmail.com",
					Password: "password",
				},
				From: "test@gmail.com",
				To:   []string{},
			},
			wantErr: true,
			errMsg:  "to email list cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLoadAndSaveConfig(t *testing.T) {
	// 创建临时配置文件
	tmpFile, err := os.CreateTemp("", "config_test_*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// 保存默认配置
	defaultConfig := DefaultConfig()
	err = SaveConfig(defaultConfig, tmpFile.Name())
	require.NoError(t, err)

	// 加载配置
	loadedConfig, err := LoadConfig(tmpFile.Name())
	require.NoError(t, err)

	// 验证配置内容
	assert.Equal(t, defaultConfig.Binance.RateLimit.RequestsPerMinute, loadedConfig.Binance.RateLimit.RequestsPerMinute)
	assert.Equal(t, defaultConfig.Watcher.Interval, loadedConfig.Watcher.Interval)
	assert.Equal(t, defaultConfig.Assets, loadedConfig.Assets)
}

func TestLoadConfigFileNotFound(t *testing.T) {
	_, err := LoadConfig("nonexistent.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config file not found")
}

func TestSaveConfigInvalidConfig(t *testing.T) {
	invalidConfig := DefaultConfig()
	invalidConfig.Assets.Symbols = []string{} // 使配置无效

	tmpFile, err := os.CreateTemp("", "config_test_*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	err = SaveConfig(invalidConfig, tmpFile.Name())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid config")
}

func TestBinanceConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *BinanceConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &BinanceConfig{
				RateLimit: RateLimitConfig{
					RequestsPerMinute: 1200,
					MaxRetries:        3,
					RetryDelay:        time.Second,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid requests per minute",
			config: &BinanceConfig{
				RateLimit: RateLimitConfig{
					RequestsPerMinute: 0,
					MaxRetries:        3,
					RetryDelay:        time.Second,
				},
			},
			wantErr: true,
			errMsg:  "requests_per_minute must be positive",
		},
		{
			name: "negative max retries",
			config: &BinanceConfig{
				RateLimit: RateLimitConfig{
					RequestsPerMinute: 1200,
					MaxRetries:        -1,
					RetryDelay:        time.Second,
				},
			},
			wantErr: true,
			errMsg:  "max_retries cannot be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
