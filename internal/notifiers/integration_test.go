//go:build integration

package notifiers

import (
	"os"
	"strconv"
	"testing"
	"time"

	"ta-watcher/internal/config"

	"github.com/stretchr/testify/assert"
)

// 集成测试只在设置了环境变量时运行
func TestEmailNotifierIntegration(t *testing.T) {
	if !shouldRunIntegrationTest() {
		t.Skip("Skipping integration test. Set EMAIL_INTEGRATION_TEST=1 to run.")
		return
	}

	// 从环境变量获取真实的邮件配置
	emailConfig := getEmailConfigFromEnv(t)
	if emailConfig == nil {
		t.Skip("Email config not available from environment variables")
		return
	}

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
	if !shouldRunIntegrationTest() {
		t.Skip("Skipping integration test. Set EMAIL_INTEGRATION_TEST=1 to run.")
		return
	}

	// 从环境变量获取真实的邮件配置
	emailConfig := getEmailConfigFromEnv(t)
	if emailConfig == nil {
		t.Skip("Email config not available from environment variables")
		return
	}

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

// shouldRunIntegrationTest 检查是否应该运行集成测试
func shouldRunIntegrationTest() bool {
	return os.Getenv("EMAIL_INTEGRATION_TEST") == "1"
}

// getEmailConfigFromEnv 从环境变量获取邮件配置
func getEmailConfigFromEnv(t *testing.T) *config.EmailConfig {
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPortStr := os.Getenv("SMTP_PORT")
	smtpUsername := os.Getenv("SMTP_USERNAME")
	smtpPassword := os.Getenv("SMTP_PASSWORD")
	fromEmail := os.Getenv("FROM_EMAIL")
	toEmail := os.Getenv("TO_EMAIL")

	// 检查必需的环境变量
	if smtpHost == "" || smtpUsername == "" || smtpPassword == "" || fromEmail == "" || toEmail == "" {
		t.Log("Missing required environment variables:")
		t.Log("  SMTP_HOST, SMTP_USERNAME, SMTP_PASSWORD, FROM_EMAIL, TO_EMAIL")
		t.Log("Example:")
		t.Log("  export SMTP_HOST=smtp.gmail.com")
		t.Log("  export SMTP_PORT=587")
		t.Log("  export SMTP_USERNAME=your_email@gmail.com")
		t.Log("  export SMTP_PASSWORD=your_app_password")
		t.Log("  export FROM_EMAIL=your_email@gmail.com")
		t.Log("  export TO_EMAIL=zhangjingyao666@gmail.com")
		return nil
	}

	// 解析端口
	smtpPort := 587 // 默认端口
	if smtpPortStr != "" {
		if port, err := strconv.Atoi(smtpPortStr); err == nil {
			smtpPort = port
		}
	}

	// 解析 TLS 设置
	useTLS := true
	if tlsStr := os.Getenv("SMTP_TLS"); tlsStr != "" {
		if tls, err := strconv.ParseBool(tlsStr); err == nil {
			useTLS = tls
		}
	}

	return &config.EmailConfig{
		Enabled: true,
		SMTP: config.SMTPConfig{
			Host:     smtpHost,
			Port:     smtpPort,
			Username: smtpUsername,
			Password: smtpPassword,
			TLS:      useTLS,
		},
		From:     fromEmail,
		To:       []string{toEmail},
		Subject:  "TA Watcher Alert - {{.Asset}} {{.Level}}",
		Template: "", // 使用默认模板
	}
}

func TestEmailNotifierPerformance(t *testing.T) {
	if !shouldRunIntegrationTest() {
		t.Skip("Skipping performance test. Set EMAIL_INTEGRATION_TEST=1 to run.")
		return
	}

	emailConfig := getEmailConfigFromEnv(t)
	if emailConfig == nil {
		t.Skip("Email config not available from environment variables")
		return
	}

	notifier, err := NewEmailNotifier(emailConfig)
	assert.NoError(t, err)

	// 测试模板渲染性能
	notification := &Notification{
		ID:        "perf-test-" + strconv.FormatInt(time.Now().Unix(), 10),
		Type:      TypePriceAlert,
		Level:     LevelWarning,
		Asset:     "BTCUSDT",
		Strategy:  "performance_test",
		Title:     "性能测试通知",
		Message:   "这是一个用于测试邮件通知器性能的测试消息。",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"test_data_1": "value1",
			"test_data_2": 123.45,
			"test_data_3": true,
			"test_data_4": []string{"a", "b", "c"},
		},
	}

	// 测试模板渲染时间
	start := time.Now()
	for i := 0; i < 100; i++ {
		_, _, err := notifier.prepareEmail(notification)
		assert.NoError(t, err)
	}
	duration := time.Since(start)

	t.Logf("⏱️ Template rendering performance: 100 renders in %v (avg: %v per render)",
		duration, duration/100)

	// 性能应该在合理范围内（每次渲染不超过10ms）
	avgDuration := duration / 100
	assert.Less(t, avgDuration, 10*time.Millisecond,
		"Template rendering too slow: %v per render", avgDuration)
}
