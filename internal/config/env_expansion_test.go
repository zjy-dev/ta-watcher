package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEnvManager 测试环境变量管理器
func TestEnvManager(t *testing.T) {
	// 保存原有环境变量
	originalEnvVars := map[string]string{
		"SMTP_HOST":     os.Getenv("SMTP_HOST"),
		"SMTP_USERNAME": os.Getenv("SMTP_USERNAME"),
		"SMTP_PASSWORD": os.Getenv("SMTP_PASSWORD"),
		"FROM_EMAIL":    os.Getenv("FROM_EMAIL"),
		"TO_EMAIL":      os.Getenv("TO_EMAIL"),
	}

	// 清理环境变量
	defer func() {
		for key, value := range originalEnvVars {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()

	// 设置测试环境变量
	os.Setenv("SMTP_HOST", "test.smtp.com")
	os.Setenv("SMTP_USERNAME", "testuser")
	os.Setenv("SMTP_PASSWORD", "testpassword123")
	os.Setenv("FROM_EMAIL", "sender@example.com")
	os.Setenv("TO_EMAIL", "recipient@example.com")

	t.Run("基本环境变量展开", func(t *testing.T) {
		envMgr := NewEnvManager()
		envMgr.loadSystemEnvVars()
		result := expandStringEnvVar("${SMTP_HOST}", envMgr)
		assert.Equal(t, "test.smtp.com", result)
	})

	t.Run("带默认值的环境变量展开", func(t *testing.T) {
		envMgr := NewEnvManager()
		envMgr.loadSystemEnvVars()

		// 存在的环境变量
		result := expandStringEnvVar("${SMTP_HOST:default.smtp.com}", envMgr)
		assert.Equal(t, "test.smtp.com", result)

		// 不存在的环境变量使用默认值
		result = expandStringEnvVar("${NONEXISTENT_VAR:default_value}", envMgr)
		assert.Equal(t, "default_value", result)
	})

	t.Run("混合文本和环境变量", func(t *testing.T) {
		envMgr := NewEnvManager()
		envMgr.loadSystemEnvVars()
		result := expandStringEnvVar("Hello ${SMTP_USERNAME}!", envMgr)
		assert.Equal(t, "Hello testuser!", result)
	})

	t.Run("邮件配置展开", func(t *testing.T) {
		envMgr := NewEnvManager()
		envMgr.loadSystemEnvVars()

		emailConfig := &EmailConfig{
			SMTP: SMTPConfig{
				Host:     "${SMTP_HOST}",
				Username: "${SMTP_USERNAME}",
				Password: "${SMTP_PASSWORD}",
			},
			From: "${FROM_EMAIL}",
			To:   []string{"${TO_EMAIL}"},
		}

		err := expandEmailConfig(emailConfig, envMgr)
		assert.NoError(t, err)
		assert.Equal(t, "test.smtp.com", emailConfig.SMTP.Host)
		assert.Equal(t, "testuser", emailConfig.SMTP.Username)
		assert.Equal(t, "testpassword123", emailConfig.SMTP.Password)
		assert.Equal(t, "sender@example.com", emailConfig.From)
		assert.Equal(t, []string{"recipient@example.com"}, emailConfig.To)
	})

	t.Run("完整配置展开", func(t *testing.T) {
		envMgr := NewEnvManager()
		envMgr.loadSystemEnvVars()

		config := &Config{
			Notifiers: NotifiersConfig{
				Email: EmailConfig{
					SMTP: SMTPConfig{
						Host:     "${SMTP_HOST}",
						Username: "${SMTP_USERNAME}",
						Password: "${SMTP_PASSWORD}",
					},
					From: "${FROM_EMAIL}",
					To:   []string{"${TO_EMAIL}"},
				},
				Feishu: FeishuConfig{
					WebhookURL: "${FEISHU_WEBHOOK_URL:https://default.webhook.url}",
				},
			},
		}

		// 临时设置全局环境管理器
		originalGlobalEnvManager := globalEnvManager
		globalEnvManager = envMgr
		defer func() {
			globalEnvManager = originalGlobalEnvManager
		}()

		err := expandEnvVars(config)
		assert.NoError(t, err)
		assert.Equal(t, "test.smtp.com", config.Notifiers.Email.SMTP.Host)
		assert.Equal(t, "testuser", config.Notifiers.Email.SMTP.Username)
		assert.Equal(t, "testpassword123", config.Notifiers.Email.SMTP.Password)
		assert.Equal(t, "sender@example.com", config.Notifiers.Email.From)
		assert.Equal(t, []string{"recipient@example.com"}, config.Notifiers.Email.To)
		assert.Equal(t, "https://default.webhook.url", config.Notifiers.Feishu.WebhookURL)
	})
}

func TestEnvVarExpansionEdgeCases(t *testing.T) {
	envMgr := NewEnvManager()
	envMgr.loadSystemEnvVars()

	t.Run("空字符串", func(t *testing.T) {
		result := expandStringEnvVar("", envMgr)
		assert.Equal(t, "", result)
	})

	t.Run("无环境变量的普通字符串", func(t *testing.T) {
		result := expandStringEnvVar("plain text", envMgr)
		assert.Equal(t, "plain text", result)
	})

	t.Run("格式错误的环境变量", func(t *testing.T) {
		result := expandStringEnvVar("${MISSING_CLOSING", envMgr)
		assert.Equal(t, "${MISSING_CLOSING", result)
	})

	t.Run("嵌套环境变量", func(t *testing.T) {
		// 设置测试环境变量
		os.Setenv("PREFIX", "test")
		defer os.Unsetenv("PREFIX")

		envMgr.SetEnv("PREFIX", "test")
		result := expandStringEnvVar("${PREFIX}_${PREFIX}_suffix", envMgr)
		assert.Equal(t, "test_test_suffix", result)
	})

	t.Run("多个环境变量", func(t *testing.T) {
		envMgr.SetEnv("VAR1", "hello")
		envMgr.SetEnv("VAR2", "world")
		result := expandStringEnvVar("${VAR1} ${VAR2}!", envMgr)
		assert.Equal(t, "hello world!", result)
	})
}

func TestConfigLoadWithEnvVars(t *testing.T) {
	// 设置测试环境变量
	os.Setenv("SMTP_USERNAME", "test@example.com")
	os.Setenv("SMTP_PASSWORD", "testpassword")
	os.Setenv("FROM_EMAIL", "sender@example.com")
	defer func() {
		os.Unsetenv("SMTP_USERNAME")
		os.Unsetenv("SMTP_PASSWORD")
		os.Unsetenv("FROM_EMAIL")
	}()

	// 创建测试配置文件内容
	configContent := `
notifiers:
  email:
    enabled: true
    smtp:
      host: "smtp.gmail.com"
      port: 587
      username: "${SMTP_USERNAME}"
      password: "${SMTP_PASSWORD}"
      tls: true
    from: "${FROM_EMAIL}"
    to:
      - "recipient@example.com"

# 添加必需的其他配置段
binance:
  rate_limit:
    requests_per_minute: 1200
    retry_delay: 2s
    max_retries: 3

watcher:
  interval: 5m
  max_workers: 10
  buffer_size: 100
  log_level: "info"
  enable_metrics: true

assets:
  - "BTCUSDT"
  - "ETHUSDT"
`

	// 创建临时配置文件
	tmpFile := "test_config_with_env.yaml"
	err := os.WriteFile(tmpFile, []byte(configContent), 0644)
	assert.NoError(t, err)
	defer os.Remove(tmpFile)

	// 加载配置
	config, err := LoadConfig(tmpFile)
	assert.NoError(t, err)
	assert.NotNil(t, config)

	// 验证环境变量已正确展开
	assert.Equal(t, "test@example.com", config.Notifiers.Email.SMTP.Username)
	assert.Equal(t, "testpassword", config.Notifiers.Email.SMTP.Password)
	assert.Equal(t, "sender@example.com", config.Notifiers.Email.From)
}

func TestTestConfigIntegration(t *testing.T) {
	// 设置测试环境变量
	os.Setenv("EMAIL_INTEGRATION_TEST", "1")
	os.Setenv("SMTP_HOST", "test.smtp.com")
	os.Setenv("SMTP_USERNAME", "test@example.com")
	os.Setenv("SMTP_PASSWORD", "testpass")
	os.Setenv("FROM_EMAIL", "sender@test.com")
	os.Setenv("TO_EMAIL", "recipient@test.com")

	defer func() {
		os.Unsetenv("EMAIL_INTEGRATION_TEST")
		os.Unsetenv("SMTP_HOST")
		os.Unsetenv("SMTP_USERNAME")
		os.Unsetenv("SMTP_PASSWORD")
		os.Unsetenv("FROM_EMAIL")
		os.Unsetenv("TO_EMAIL")
	}()

	// 重置全局环境管理器以使用新的环境变量
	globalEnvManager = nil

	// 加载配置文件并测试环境变量展开
	projectRoot := FindProjectRoot()
	if projectRoot == "" {
		t.Skip("Could not find project root, skipping test")
	}

	configPath := filepath.Join(projectRoot, "config.example.yaml")
	cfg, err := LoadConfig(configPath)
	assert.NoError(t, err)

	// 验证环境变量展开是否正确工作
	assert.Equal(t, "test.smtp.com", cfg.Notifiers.Email.SMTP.Host)
	assert.Equal(t, "test@example.com", cfg.Notifiers.Email.SMTP.Username)
	assert.Equal(t, "testpass", cfg.Notifiers.Email.SMTP.Password)
	assert.Equal(t, "sender@test.com", cfg.Notifiers.Email.From)
	assert.Equal(t, []string{"recipient@test.com"}, cfg.Notifiers.Email.To)
}
