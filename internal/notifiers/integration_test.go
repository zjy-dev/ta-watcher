//go:build integration

package notifiers

import (
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"ta-watcher/internal/config"

	"github.com/stretchr/testify/assert"
)

// 集成测试只在设置了环境变量时运行
func TestEmailNotifierIntegration(t *testing.T) {
	// 初始化环境变量管理器，优先使用 .env.example（用于集成测试）
	envFile := config.DetermineEnvFile()
	if envFile == "" {
		// 如果 DetermineEnvFile 没有找到文件，尝试手动构建路径
		projectRoot := config.FindProjectRoot()
		if projectRoot != "" {
			envFile = filepath.Join(projectRoot, ".env.example")
		} else {
			envFile = ".env.example" // 最后的回退选项
		}
	}

	t.Logf("Attempting to load env file: %s", envFile)
	err := config.InitEnvManager(envFile)
	if err != nil {
		t.Logf("Warning: Failed to load env file %s: %v", envFile, err)
		t.Logf("Will proceed with system environment variables only")
	}

	if !config.IsIntegrationTestEnabled("EMAIL") {
		t.Skip("Skipping integration test. Set EMAIL_INTEGRATION_TEST=1 to run.")
		return
	}

	// 加载正常的配置文件
	projectRoot := config.FindProjectRoot()
	if projectRoot == "" {
		t.Fatal("Could not find project root directory")
	}

	configPath := filepath.Join(projectRoot, "config.example.yaml")
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config from %s: %v", configPath, err)
	}

	// 确保邮件通知已启用
	if !cfg.Notifiers.Email.Enabled {
		// 在集成测试中强制启用邮件通知
		cfg.Notifiers.Email.Enabled = true
	}

	emailConfig := &cfg.Notifiers.Email

	// 创建邮件通知器
	notifier, err := NewEmailNotifier(emailConfig)
	assert.NoError(t, err)
	assert.True(t, notifier.IsEnabled())

	// 测试连接
	err = notifier.TestConnection()
	if err != nil {
		t.Logf("Email connection test failed: %v", err)
		t.Skip("Email connection failed, skipping integration test")
		return
	}

	t.Log("✅ Email connection test passed")

	// 创建测试通知
	notification := &Notification{
		ID:        "integration-test-" + strconv.FormatInt(time.Now().Unix(), 10),
		Type:      TypeSystemAlert,
		Level:     LevelInfo,
		Asset:     "BTCUSDT",
		Strategy:  "integration_test",
		Title:     "TA Watcher 集成测试",
		Message:   "这是一封来自 TA Watcher 的集成测试邮件。如果您收到这封邮件，说明邮件通知功能工作正常。",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"test_type":  "integration",
			"test_time":  time.Now().Format("2006-01-02 15:04:05"),
			"price":      105000.50,
			"change_pct": 2.45,
			"volume":     "1,234,567 BTC",
			"market_cap": "$2.1T",
		},
	}

	// 发送测试邮件
	t.Log("📧 Sending test email...")
	err = notifier.Send(notification)
	assert.NoError(t, err)

	t.Log("✅ Test email sent successfully")
	t.Log("📬 Please check your email inbox to verify the email was received")

	// 关闭通知器
	err = notifier.Close()
	assert.NoError(t, err)
}

func TestEmailNotifierIntegrationWithManager(t *testing.T) {
	// 初始化环境变量管理器
	envFile := config.DetermineEnvFile()
	if envFile == "" {
		// 如果 DetermineEnvFile 没有找到文件，尝试手动构建路径
		projectRoot := config.FindProjectRoot()
		if projectRoot != "" {
			envFile = filepath.Join(projectRoot, ".env.example")
		} else {
			envFile = ".env.example" // 最后的回退选项
		}
	}

	t.Logf("Attempting to load env file: %s", envFile)
	err := config.InitEnvManager(envFile)
	if err != nil {
		t.Logf("Warning: Failed to load env file %s: %v", envFile, err)
		t.Logf("Will proceed with system environment variables only")
	}

	if !config.IsIntegrationTestEnabled("EMAIL") {
		t.Skip("Skipping integration test. Set EMAIL_INTEGRATION_TEST=1 to run.")
		return
	}

	// 加载正常的配置文件
	projectRoot := config.FindProjectRoot()
	if projectRoot == "" {
		t.Fatal("Could not find project root directory")
	}

	configPath := filepath.Join(projectRoot, "config.example.yaml")
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config from %s: %v", configPath, err)
	}

	// 确保邮件通知已启用
	if !cfg.Notifiers.Email.Enabled {
		// 在集成测试中强制启用邮件通知
		cfg.Notifiers.Email.Enabled = true
	}

	emailConfig := &cfg.Notifiers.Email

	// 创建通知管理器
	manager := NewManager()

	// 创建并添加邮件通知器
	emailNotifier, err := NewEmailNotifier(emailConfig)
	assert.NoError(t, err)

	err = manager.AddNotifier(emailNotifier)
	assert.NoError(t, err)

	assert.Equal(t, 1, manager.TotalCount())
	assert.Equal(t, 1, manager.EnabledCount())

	// 设置过滤器（只允许警告级别以上）
	filter := &NotificationFilter{
		MinLevel: LevelWarning,
		Types:    []NotificationType{TypePriceAlert, TypeStrategySignal},
	}
	manager.SetFilter(filter)

	// 发送一个 INFO 级别的通知（应该被过滤）
	infoNotification := &Notification{
		ID:        "integration-filtered-" + strconv.FormatInt(time.Now().Unix(), 10),
		Type:      TypeSystemAlert,
		Level:     LevelInfo,
		Title:     "这条消息应该被过滤",
		Message:   "您不应该收到这封邮件，因为它应该被过滤器过滤掉。",
		Timestamp: time.Now(),
	}

	t.Log("📧 Sending filtered notification (should not be sent)...")
	err = manager.Send(infoNotification)
	assert.NoError(t, err)

	// 发送一个 WARNING 级别的价格警报（应该通过过滤器）
	warningNotification := &Notification{
		ID:        "integration-warning-" + strconv.FormatInt(time.Now().Unix(), 10),
		Type:      TypePriceAlert,
		Level:     LevelWarning,
		Asset:     "BTCUSDT",
		Strategy:  "price_monitor",
		Title:     "比特币价格警报",
		Message:   "比特币价格已突破重要阻力位，建议关注后续走势。",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"current_price": 105500.00,
			"resistance":    105000.00,
			"support":       104000.00,
			"volume_24h":    "15,678 BTC",
			"change_24h":    "+3.25%",
		},
	}

	t.Log("📧 Sending warning notification (should be sent)...")
	err = manager.Send(warningNotification)
	assert.NoError(t, err)

	// 发送一个 CRITICAL 级别的策略信号
	criticalNotification := &Notification{
		ID:        "integration-critical-" + strconv.FormatInt(time.Now().Unix(), 10),
		Type:      TypeStrategySignal,
		Level:     LevelCritical,
		Asset:     "ETHUSDT",
		Strategy:  "golden_cross",
		Title:     "以太坊金叉信号",
		Message:   "以太坊出现黄金交叉信号，50日均线向上突破200日均线，这是一个强烈的看涨信号。",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"signal_type":   "GOLDEN_CROSS",
			"ma_50":         2520.45,
			"ma_200":        2518.30,
			"current_price": 2523.47,
			"confidence":    0.85,
			"action":        "BUY",
		},
	}

	t.Log("📧 Sending critical notification (should be sent)...")
	err = manager.Send(criticalNotification)
	assert.NoError(t, err)

	t.Log("✅ Integration test completed successfully")
	t.Log("📬 Please check your email inbox:")
	t.Log("   - You should NOT receive the INFO level message (filtered)")
	t.Log("   - You should receive the WARNING level price alert")
	t.Log("   - You should receive the CRITICAL level strategy signal")

	// 关闭管理器
	err = manager.Close()
	assert.NoError(t, err)
}

func TestEmailSendWithTemplateIntegration(t *testing.T) {
	// 初始化环境变量管理器
	envFile := config.DetermineEnvFile()
	if envFile == "" {
		// 如果 DetermineEnvFile 没有找到文件，尝试手动构建路径
		projectRoot := config.FindProjectRoot()
		if projectRoot != "" {
			envFile = filepath.Join(projectRoot, ".env.example")
		} else {
			envFile = ".env.example" // 最后的回退选项
		}
	}

	t.Logf("Attempting to load env file: %s", envFile)
	err := config.InitEnvManager(envFile)
	if err != nil {
		t.Logf("Warning: Failed to load env file %s: %v", envFile, err)
		t.Logf("Will proceed with system environment variables only")
	}

	if !config.IsIntegrationTestEnabled("EMAIL") {
		t.Skip("Skipping integration test. Set EMAIL_INTEGRATION_TEST=1 to run.")
		return
	}

	// 加载正常的配置文件
	projectRoot := config.FindProjectRoot()
	if projectRoot == "" {
		t.Fatal("Could not find project root directory")
	}

	configPath := filepath.Join(projectRoot, "config.example.yaml")
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config from %s: %v", configPath, err)
	}

	// 确保邮件通知已启用
	if !cfg.Notifiers.Email.Enabled {
		// 在集成测试中强制启用邮件通知
		cfg.Notifiers.Email.Enabled = true
	}

	emailConfig := &cfg.Notifiers.Email

	// 自定义邮件模板
	emailConfig.Template = `
亲爱的用户，

您好！这是来自 TA Watcher 的{{.Level}}级别通知。

📊 交易对: {{.Asset}}
🎯 策略: {{.Strategy}}
📈 当前价格: {{.Data.current_price}}
📅 时间: {{.Timestamp.Format "2006-01-02 15:04:05"}}

{{.Message}}

感谢您使用 TA Watcher！

---
此邮件由 TA Watcher 自动发送，请勿回复。
`

	// 创建邮件通知器
	notifier, err := NewEmailNotifier(emailConfig)
	assert.NoError(t, err)

	// 创建测试通知
	notification := &Notification{
		ID:        "template-test-" + strconv.FormatInt(time.Now().Unix(), 10),
		Type:      TypePriceAlert,
		Level:     LevelWarning,
		Asset:     "BTCUSDT",
		Strategy:  "template_test",
		Title:     "模板测试邮件",
		Message:   "这是一封测试自定义邮件模板的邮件。如果您看到格式化的内容，说明模板功能正常工作。",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"current_price": 105234.67,
			"change_24h":    "+2.34%",
			"volume":        "15,432 BTC",
		},
	}

	// 发送测试邮件
	t.Log("📧 Sending template test email...")
	err = notifier.Send(notification)
	assert.NoError(t, err)

	t.Log("✅ Template test email sent successfully")
	t.Log("📬 Please check your email inbox to verify the template formatting")

	// 关闭通知器
	err = notifier.Close()
	assert.NoError(t, err)
}
