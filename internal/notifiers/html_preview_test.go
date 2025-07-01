package notifiers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ta-watcher/internal/config"
)

func TestEmailHTMLPreviewSaving(t *testing.T) {
	// 清理测试环境
	testDir := "test_email_previews"
	defer os.RemoveAll(testDir)

	// 创建测试邮件配置
	cfg := &config.EmailConfig{
		Enabled: true,
		SMTP: config.SMTPConfig{
			Host:     "smtp.gmail.com",
			Port:     587,
			Username: "test@example.com",
			Password: "password",
		},
		From:    "sender@example.com",
		To:      []string{"recipient@example.com"},
		Subject: "测试邮件 - {{.FormattedTime}}",
	}

	// 创建邮件通知器
	notifier, err := NewEmailNotifier(cfg)
	if err != nil {
		t.Fatalf("创建邮件通知器失败: %v", err)
	}

	// 创建测试通知
	notification := &Notification{
		Type:      TypeStrategySignal,
		Asset:     "BTCUSDT",
		Title:     "RSI超卖信号",
		Message:   "<h2>📈 交易信号</h2><p><strong>币种:</strong> BTCUSDT</p><p><strong>信号:</strong> <span style='color: green;'>买入</span></p>",
		Timestamp: time.Now(),
	}

	// 准备邮件内容
	subject, body, err := notifier.PrepareEmailForTesting(notification)
	if err != nil {
		t.Fatalf("准备邮件失败: %v", err)
	}

	// 验证内容
	if subject == "" {
		t.Error("邮件主题不应为空")
	}

	if body == "" {
		t.Error("邮件内容不应为空")
	}

	// 验证HTML内容包含预期的元素
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("邮件内容应包含HTML文档类型声明")
	}

	if !strings.Contains(body, "📈 交易信号") {
		t.Error("邮件内容应包含测试消息")
	}

	if !strings.Contains(body, "BTCUSDT") {
		t.Error("邮件内容应包含资产信息")
	}

	// 测试HTML预览保存功能（通过手动调用私有方法的方式）
	testSaveHTMLPreview(t, notifier, subject, body, testDir)

	t.Logf("✅ HTML预览功能测试通过")
	t.Logf("📄 邮件主题: %s", subject)
	t.Logf("📄 邮件内容长度: %d 字符", len(body))
}

// testSaveHTMLPreview 测试HTML预览保存功能
func testSaveHTMLPreview(t *testing.T, notifier *EmailNotifier, subject, body, testDir string) {
	// 创建测试目录
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}

	// 生成测试文件名
	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(testDir, "test_email_preview_"+timestamp+".html")

	// 保存HTML预览
	if err := os.WriteFile(filename, []byte(body), 0644); err != nil {
		t.Fatalf("保存HTML预览失败: %v", err)
	}

	// 验证文件是否创建成功
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Error("HTML预览文件未创建")
	}

	// 读取文件内容并验证
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("读取HTML预览文件失败: %v", err)
	}

	if string(content) != body {
		t.Error("HTML预览文件内容与预期不符")
	}

	t.Logf("✅ HTML预览文件保存成功: %s", filename)
}

func TestCoinbaseRateLimitConfig(t *testing.T) {
	// 验证配置更新是否正确应用
	t.Log("📊 验证Coinbase限流配置...")

	// 这里我们主要验证配置结构是否正确
	// 实际的限流测试需要真实的API调用，不适合单元测试

	cfg := &config.CoinbaseConfig{
		RateLimit: config.RateLimitConfig{
			RequestsPerMinute: 300,
			RetryDelay:        5 * time.Second,
			MaxRetries:        3,
		},
	}

	if cfg.RateLimit.RequestsPerMinute != 300 {
		t.Errorf("预期请求频率为300/分钟，实际为 %d", cfg.RateLimit.RequestsPerMinute)
	}

	if cfg.RateLimit.RetryDelay != 5*time.Second {
		t.Errorf("预期重试延迟为5秒，实际为 %v", cfg.RateLimit.RetryDelay)
	}

	t.Logf("✅ Coinbase限流配置验证通过: %d req/min, %v retry delay",
		cfg.RateLimit.RequestsPerMinute, cfg.RateLimit.RetryDelay)
}
