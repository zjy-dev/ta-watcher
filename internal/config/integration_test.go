//go:build integration

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigIntegration 测试配置文件的完整集成流程
func TestConfigIntegration(t *testing.T) {
	// 创建临时目录用于测试
	tempDir := t.TempDir()

	// 测试配置文件路径
	configPath := filepath.Join(tempDir, "test_config.yaml")

	// 创建测试配置
	testConfig := &Config{
		DataSource: DataSourceConfig{
			Primary:    "coinbase",
			Fallback:   "binance",
			Timeout:    30 * time.Second,
			MaxRetries: 3,
			Binance: BinanceConfig{
				RateLimit: RateLimitConfig{
					RequestsPerMinute: 1200,
					RetryDelay:        2 * time.Second,
					MaxRetries:        3,
				},
			},
			Coinbase: CoinbaseConfig{
				RateLimit: RateLimitConfig{
					RequestsPerMinute: 100,
					RetryDelay:        15 * time.Second,
					MaxRetries:        3,
				},
			},
		},
		Binance: BinanceConfig{
			RateLimit: RateLimitConfig{
				RequestsPerMinute: 100,
				RetryDelay:        1 * time.Second,
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
				Enabled: true,
				SMTP: SMTPConfig{
					Host:     "smtp.gmail.com",
					Port:     587,
					Username: "test@example.com",
					Password: "test_password",
					TLS:      true,
				},
				From:     "test@example.com",
				To:       []string{"recipient@example.com"},
				Subject:  "TA Watcher Alert",
				Template: "Default email template",
			},
			Feishu: FeishuConfig{
				Enabled:    false,
				WebhookURL: "",
				Secret:     "",
				Template:   "",
			},
			Wechat: WechatConfig{
				Enabled:    false,
				WebhookURL: "",
				Template:   "",
			},
		},
		Assets: AssetsConfig{
			Symbols:                 []string{"BTC", "ETH"},
			Timeframes:              []string{"1d", "1w"},
			BaseCurrency:            "USDT",
			MarketCapUpdateInterval: time.Hour,
		},
	}

	t.Run("SaveAndLoadConfig", func(t *testing.T) {
		// 保存配置到文件
		err := SaveConfig(testConfig, configPath)
		require.NoError(t, err, "保存配置文件应该成功")

		// 验证文件是否存在
		_, err = os.Stat(configPath)
		require.NoError(t, err, "配置文件应该存在")

		// 加载配置文件
		loadedConfig, err := LoadConfig(configPath)
		require.NoError(t, err, "加载配置文件应该成功")

		// 验证配置内容
		assert.Equal(t, testConfig.Notifiers.Email.SMTP.Host, loadedConfig.Notifiers.Email.SMTP.Host)
		assert.Equal(t, testConfig.Notifiers.Email.SMTP.Port, loadedConfig.Notifiers.Email.SMTP.Port)
		assert.Equal(t, testConfig.Watcher.LogLevel, loadedConfig.Watcher.LogLevel)
	})

	t.Run("ValidateConfig", func(t *testing.T) {
		// 先保存配置到临时文件
		tempConfigPath := filepath.Join(tempDir, "validate_test_config.yaml")
		err := SaveConfig(testConfig, tempConfigPath)
		require.NoError(t, err, "保存测试配置应该成功")

		// 加载配置
		loadedConfig, err := LoadConfig(tempConfigPath)
		require.NoError(t, err)

		// 验证配置
		err = loadedConfig.Validate()
		assert.NoError(t, err, "有效的配置应该通过验证")
	})

	t.Run("InvalidConfigValidation", func(t *testing.T) {
		// 创建无效配置 - 缺少必需的 DataSource 配置
		invalidConfig := &Config{
			DataSource: DataSourceConfig{
				Primary: "", // 无效值 - primary不能为空
			},
			Binance: BinanceConfig{
				RateLimit: RateLimitConfig{
					RequestsPerMinute: -1, // 无效值
				},
			},
			Notifiers: NotifiersConfig{
				Email: EmailConfig{
					SMTP: SMTPConfig{
						Port: -1, // 无效端口
					},
				},
			},
		}

		// 验证应该失败
		err := invalidConfig.Validate()
		assert.Error(t, err, "无效配置应该验证失败")
	})

	t.Run("LoadNonExistentConfig", func(t *testing.T) {
		nonExistentPath := filepath.Join(tempDir, "non_existent.yaml")

		_, err := LoadConfig(nonExistentPath)
		assert.Error(t, err, "加载不存在的配置文件应该失败")
	})

	t.Run("SaveToInvalidPath", func(t *testing.T) {
		invalidPath := "/invalid/path/config.yaml"

		err := SaveConfig(testConfig, invalidPath)
		assert.Error(t, err, "保存到无效路径应该失败")
	})
}

// TestExampleConfigFile 测试示例配置文件的加载和验证
func TestExampleConfigFile(t *testing.T) {
	// 示例配置文件路径（相对于项目根目录）
	exampleConfigPath := "../../config.example.yaml"

	// 检查示例配置文件是否存在
	if _, err := os.Stat(exampleConfigPath); os.IsNotExist(err) {
		t.Skip("示例配置文件不存在，跳过测试")
		return
	}

	t.Run("LoadExampleConfig", func(t *testing.T) {
		// 加载示例配置文件
		config, err := LoadConfig(exampleConfigPath)
		require.NoError(t, err, "加载示例配置文件应该成功")

		// 验证配置结构完整性
		assert.NotEmpty(t, config.Notifiers.Email.SMTP.Host, "Email SMTPHost 不应为空")
		assert.Greater(t, config.Notifiers.Email.SMTP.Port, 0, "Email SMTPPort 应该大于0")
		assert.NotEmpty(t, config.Watcher.LogLevel, "Watcher LogLevel 不应为空")
		assert.Greater(t, config.Watcher.Interval, time.Duration(0), "Watcher Interval 应该大于0")
	})

	t.Run("ValidateExampleConfig", func(t *testing.T) {
		// 加载示例配置
		config, err := LoadConfig(exampleConfigPath)
		require.NoError(t, err)

		// 注意：示例配置中的凭据可能是占位符，所以我们不验证凭据字段
		// 只验证结构和基本值
		assert.NotEmpty(t, config.Notifiers.Email.SMTP.Host)
		assert.Greater(t, config.Notifiers.Email.SMTP.Port, 0)
		assert.Greater(t, config.Binance.RateLimit.RequestsPerMinute, 0)
		assert.Greater(t, config.Binance.RateLimit.MaxRetries, 0)
		assert.Greater(t, config.Binance.RateLimit.RetryDelay, time.Duration(0))
	})
}

// TestConfigConcurrency 测试配置文件的并发访问
func TestConfigConcurrency(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "concurrent_config.yaml")

	// 创建基础配置
	baseConfig := &Config{
		DataSource: DataSourceConfig{
			Primary:    "coinbase",
			Fallback:   "binance",
			Timeout:    30 * time.Second,
			MaxRetries: 3,
			Binance: BinanceConfig{
				RateLimit: RateLimitConfig{
					RequestsPerMinute: 1200,
					RetryDelay:        2 * time.Second,
					MaxRetries:        3,
				},
			},
			Coinbase: CoinbaseConfig{
				RateLimit: RateLimitConfig{
					RequestsPerMinute: 100,
					RetryDelay:        15 * time.Second,
					MaxRetries:        3,
				},
			},
		},
		Binance: BinanceConfig{
			RateLimit: RateLimitConfig{
				RequestsPerMinute: 100,
				RetryDelay:        1 * time.Second,
				MaxRetries:        3,
			},
		},
		Watcher: WatcherConfig{
			Interval:      5 * time.Minute,
			MaxWorkers:    10,
			BufferSize:    100,
			LogLevel:      "info",
			EnableMetrics: false,
		},
		Notifiers: NotifiersConfig{
			Email: EmailConfig{
				Enabled: true,
				SMTP: SMTPConfig{
					Host:     "smtp.test.com",
					Port:     587,
					Username: "test@test.com",
					Password: "password",
					TLS:      true,
				},
				From: "test@test.com",
				To:   []string{"to@test.com"},
			},
		},
		Assets: AssetsConfig{
			Symbols:                 []string{"BTC"},
			Timeframes:              []string{"1d"},
			BaseCurrency:            "USDT",
			MarketCapUpdateInterval: time.Hour,
		},
	}

	// 先保存基础配置
	err := SaveConfig(baseConfig, configPath)
	require.NoError(t, err)

	// 并发读取配置
	t.Run("ConcurrentLoad", func(t *testing.T) {
		const numGoroutines = 10
		results := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				_, err := LoadConfig(configPath)
				results <- err
			}()
		}

		// 检查所有goroutine的结果
		for i := 0; i < numGoroutines; i++ {
			err := <-results
			assert.NoError(t, err, "并发加载配置应该成功")
		}
	})

	// 注意：并发写入测试可能导致文件竞争，在实际应用中应该避免
	// 这里我们测试多个独立文件的保存
	t.Run("ConcurrentSaveToSeparateFiles", func(t *testing.T) {
		const numGoroutines = 5
		results := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(index int) {
				separateConfigPath := filepath.Join(tempDir, "config_"+string(rune('0'+index))+".yaml")
				err := SaveConfig(baseConfig, separateConfigPath)
				results <- err
			}(i)
		}

		// 检查所有goroutine的结果
		for i := 0; i < numGoroutines; i++ {
			err := <-results
			assert.NoError(t, err, "并发保存到不同文件应该成功")
		}
	})
}

// TestConfigBackwardCompatibility 测试配置文件的向后兼容性
func TestConfigBackwardCompatibility(t *testing.T) {
	tempDir := t.TempDir()

	// 模拟旧版本配置文件（缺少某些新字段）
	oldConfigYAML := `
datasource:
  primary: "coinbase"
  timeout: "30s"
  max_retries: 3
  binance:
    rate_limit:
      requests_per_minute: 1200
      retry_delay: "2s"
      max_retries: 3
  coinbase:
    rate_limit:
      requests_per_minute: 100
      retry_delay: "15s" 
      max_retries: 3

binance:
  rate_limit:
    requests_per_minute: 1000
    max_retries: 2

watcher:
  log_level: "info"
  interval: "5m"

notifiers:
  email:
    enabled: true
    smtp:
      host: "smtp.old.com"
      port: 587
      username: "old@test.com"
      password: "old_password"
      tls: true
    from: "old@test.com"
    to: ["recipient@test.com"]

assets:
  symbols: ["BTC", "ETH"]
  timeframes: ["1d"]
  base_currency: "USDT"
  market_cap_update_interval: "1h"
`

	oldConfigPath := filepath.Join(tempDir, "old_config.yaml")
	err := os.WriteFile(oldConfigPath, []byte(oldConfigYAML), 0644)
	require.NoError(t, err)

	t.Run("LoadOldConfig", func(t *testing.T) {
		// 加载旧配置应该成功，缺失字段使用默认值
		config, err := LoadConfig(oldConfigPath)
		require.NoError(t, err, "加载旧版本配置应该成功")

		// 验证基本字段存在
		assert.Equal(t, "smtp.old.com", config.Notifiers.Email.SMTP.Host)
		assert.Equal(t, "info", config.Watcher.LogLevel)
		assert.Equal(t, "coinbase", config.DataSource.Primary)

		// 验证缺失字段有默认值
		assert.Greater(t, config.Watcher.Interval, time.Duration(0), "应该有默认检查间隔值")
		assert.Greater(t, config.Binance.RateLimit.RequestsPerMinute, 0, "应该有限流配置")
		assert.Greater(t, config.DataSource.Binance.RateLimit.RequestsPerMinute, 0, "DataSource应该有限流配置")
	})
}
