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

// TestEmailNotifierIntegration 邮件通知器集成测试
// config 模块会自动根据环境选择合适的 .env 文件
func TestEmailNotifierIntegration(t *testing.T) {
	// 检查是否启用了邮件集成测试
	if !config.IsIntegrationTestEnabled("EMAIL") {
		t.Skip("跳过集成测试。设置 EMAIL_INTEGRATION_TEST=1 来运行邮件测试。")
		return
	}

	// 查找项目根目录
	projectRoot := config.FindProjectRoot()
	if projectRoot == "" {
		t.Fatal("找不到项目根目录")
	}

	// 加载示例配置文件，config 模块会自动处理环境变量展开
	configPath := filepath.Join(projectRoot, "config.example.yaml")
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("从 %s 加载配置失败: %v", configPath, err)
	}

	// 在集成测试中强制启用邮件通知
	cfg.Notifiers.Email.Enabled = true
	emailConfig := &cfg.Notifiers.Email

	// 创建邮件通知器
	notifier, err := NewEmailNotifier(emailConfig)
	assert.NoError(t, err, "创建邮件通知器应该成功")
	assert.True(t, notifier.IsEnabled(), "邮件通知器应该已启用")

	// 测试邮件服务器连接
	t.Log("🔗 正在测试邮件服务器连接...")
	err = notifier.TestConnection()
	if err != nil {
		t.Logf("邮件连接测试失败: %v", err)
		t.Skip("邮件连接失败，跳过集成测试")
		return
	}
	t.Log("✅ 邮件服务器连接测试通过")

	// 创建测试通知消息
	notification := &Notification{
		ID:        "integration-test-" + strconv.FormatInt(time.Now().Unix(), 10),
		Type:      TypeSystemAlert,
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
	t.Log("📧 正在发送测试邮件...")
	err = notifier.Send(notification)
	assert.NoError(t, err, "发送测试邮件应该成功")

	t.Log("✅ 测试邮件发送成功")
	t.Log("📬 请检查您的邮箱以确认邮件已收到")

	// 关闭通知器
	err = notifier.Close()
	assert.NoError(t, err, "关闭邮件通知器应该成功")
}

// TestEmailNotifierIntegrationWithManager 使用通知管理器的邮件集成测试
func TestEmailNotifierIntegrationWithManager(t *testing.T) {
	// 检查是否启用了邮件集成测试
	if !config.IsIntegrationTestEnabled("EMAIL") {
		t.Skip("跳过集成测试。设置 EMAIL_INTEGRATION_TEST=1 来运行邮件测试。")
		return
	}

	// 查找项目根目录
	projectRoot := config.FindProjectRoot()
	if projectRoot == "" {
		t.Fatal("找不到项目根目录")
	}

	// 加载示例配置文件，config 模块会自动处理环境变量展开
	configPath := filepath.Join(projectRoot, "config.example.yaml")
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("从 %s 加载配置失败: %v", configPath, err)
	}

	// 在集成测试中强制启用邮件通知
	cfg.Notifiers.Email.Enabled = true
	emailConfig := &cfg.Notifiers.Email

	// 创建通知管理器
	manager := NewManager()

	// 创建并添加邮件通知器
	emailNotifier, err := NewEmailNotifier(emailConfig)
	assert.NoError(t, err, "创建邮件通知器应该成功")

	err = manager.AddNotifier(emailNotifier)
	assert.NoError(t, err, "添加邮件通知器到管理器应该成功")

	assert.Equal(t, 1, manager.TotalCount(), "管理器应该包含1个通知器")
	assert.Equal(t, 1, manager.EnabledCount(), "管理器应该有1个启用的通知器")

	// 发送系统告警通知
	systemNotification := &Notification{
		ID:        "integration-system-" + strconv.FormatInt(time.Now().Unix(), 10),
		Type:      TypeSystemAlert,
		Title:     "系统告警测试",
		Message:   "这是一条系统告警消息，用于测试通知管理器的功能。",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"alert_type": "system",
			"component":  "watcher",
			"status":     "running",
		},
	}

	t.Log("📧 正在发送系统告警通知...")
	err = manager.Send(systemNotification)
	assert.NoError(t, err, "发送系统告警通知应该成功")

	// 发送价格警报通知
	priceNotification := &Notification{
		ID:        "integration-price-" + strconv.FormatInt(time.Now().Unix(), 10),
		Type:      TypePriceAlert,
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

	t.Log("📧 正在发送价格警报通知...")
	err = manager.Send(priceNotification)
	assert.NoError(t, err, "发送价格警报通知应该成功")

	// 发送策略信号通知
	strategyNotification := &Notification{
		ID:        "integration-strategy-" + strconv.FormatInt(time.Now().Unix(), 10),
		Type:      TypeStrategySignal,
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

	t.Log("📧 正在发送策略信号通知...")
	err = manager.Send(strategyNotification)
	assert.NoError(t, err, "发送策略信号通知应该成功")

	t.Log("✅ 通知管理器集成测试完成")
	t.Log("📬 请检查您的邮箱以确认收到了所有测试邮件")

	// 关闭管理器
	err = manager.Close()
	assert.NoError(t, err, "关闭通知管理器应该成功")
}

// TestEmailSendWithTemplateIntegration 邮件模板集成测试
func TestEmailSendWithTemplateIntegration(t *testing.T) {
	// 检查是否启用了邮件集成测试
	if !config.IsIntegrationTestEnabled("EMAIL") {
		t.Skip("跳过集成测试。设置 EMAIL_INTEGRATION_TEST=1 来运行邮件测试。")
		return
	}

	// 查找项目根目录
	projectRoot := config.FindProjectRoot()
	if projectRoot == "" {
		t.Fatal("找不到项目根目录")
	}

	// 加载示例配置文件，config 模块会自动处理环境变量展开
	configPath := filepath.Join(projectRoot, "config.example.yaml")
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("从 %s 加载配置失败: %v", configPath, err)
	}

	// 在集成测试中强制启用邮件通知
	cfg.Notifiers.Email.Enabled = true
	emailConfig := &cfg.Notifiers.Email

	// 自定义邮件模板
	emailConfig.Template = `
亲爱的用户，

您好！这是来自 TA Watcher 的交易信号通知。

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
	assert.NoError(t, err, "创建邮件通知器应该成功")

	// 创建测试通知
	notification := &Notification{
		ID:        "template-test-" + strconv.FormatInt(time.Now().Unix(), 10),
		Type:      TypePriceAlert,
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
	t.Log("📧 正在发送模板测试邮件...")
	err = notifier.Send(notification)
	assert.NoError(t, err, "发送模板测试邮件应该成功")

	t.Log("✅ 模板测试邮件发送成功")
	t.Log("📬 请检查您的邮箱以确认模板格式化效果")

	// 关闭通知器
	err = notifier.Close()
	assert.NoError(t, err, "关闭邮件通知器应该成功")
}
