package notifiers

import (
	"testing"
	"time"

	"ta-watcher/internal/config"

	"github.com/stretchr/testify/assert"
)

func TestEmailNotifierCreation(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.EmailConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
			errMsg:  "email config cannot be nil",
		},
		{
			name: "disabled config",
			config: &config.EmailConfig{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name: "valid enabled config",
			config: &config.EmailConfig{
				Enabled: true,
				SMTP: config.SMTPConfig{
					Host:     "smtp.gmail.com",
					Port:     587,
					Username: "test@gmail.com",
					Password: "password",
					TLS:      true,
				},
				From:     "test@gmail.com",
				To:       []string{"recipient@gmail.com"},
				Subject:  "Test Subject",
				Template: "Test Template",
			},
			wantErr: false,
		},
		{
			name: "invalid enabled config - missing host",
			config: &config.EmailConfig{
				Enabled: true,
				SMTP: config.SMTPConfig{
					Port:     587,
					Username: "test@gmail.com",
					Password: "password",
					TLS:      true,
				},
				From: "test@gmail.com",
				To:   []string{"recipient@gmail.com"},
			},
			wantErr: true,
			errMsg:  "invalid email config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier, err := NewEmailNotifier(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, notifier)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, notifier)
				assert.Equal(t, "email", notifier.Name())

				if tt.config != nil {
					assert.Equal(t, tt.config.Enabled, notifier.IsEnabled())
				}
			}
		})
	}
}

func TestEmailNotifierSendDisabled(t *testing.T) {
	config := &config.EmailConfig{
		Enabled: false,
	}

	notifier, err := NewEmailNotifier(config)
	assert.NoError(t, err)
	assert.False(t, notifier.IsEnabled())

	notification := &Notification{
		ID:        "test-1",
		Type:      TypeSystemAlert,
		Title:     "Test Notification",
		Message:   "This is a test",
		Timestamp: time.Now(),
	}

	// 发送通知应该成功（被跳过）
	err = notifier.Send(notification)
	assert.NoError(t, err)
}

func TestEmailNotifierTemplateRendering(t *testing.T) {
	config := &config.EmailConfig{
		Enabled: true,
		SMTP: config.SMTPConfig{
			Host:     "smtp.gmail.com",
			Port:     587,
			Username: "test@gmail.com",
			Password: "password",
			TLS:      true,
		},
		From:     "test@gmail.com",
		To:       []string{"recipient@gmail.com"},
		Subject:  "Alert: {{.Asset}} - {{.Title}}",
		Template: "Asset: {{.Asset}}, Message: {{.Message}}",
	}

	notifier, err := NewEmailNotifier(config)
	assert.NoError(t, err)

	notification := &Notification{
		ID:        "test-1",
		Type:      TypePriceAlert,
		Asset:     "BTCUSDT",
		Title:     "Price Alert",
		Message:   "Price exceeded threshold",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"price":     50000.0,
			"threshold": 49000.0,
		},
	}

	// 准备邮件内容
	subject, body, err := notifier.prepareEmail(notification)
	assert.NoError(t, err)
	assert.Contains(t, subject, "BTCUSDT")
	assert.Contains(t, subject, "Price Alert")
	assert.Contains(t, body, "BTCUSDT")
	assert.Contains(t, body, "Price exceeded threshold")
}

func TestEmailNotifierSetEnabled(t *testing.T) {
	config := &config.EmailConfig{
		Enabled: false,
	}

	notifier, err := NewEmailNotifier(config)
	assert.NoError(t, err)
	assert.False(t, notifier.IsEnabled())

	// 启用通知器
	notifier.SetEnabled(true)
	assert.True(t, notifier.IsEnabled())

	// 禁用通知器
	notifier.SetEnabled(false)
	assert.False(t, notifier.IsEnabled())
}

func TestEmailNotifierClose(t *testing.T) {
	config := &config.EmailConfig{
		Enabled: false,
	}

	notifier, err := NewEmailNotifier(config)
	assert.NoError(t, err)

	// 关闭通知器应该成功
	err = notifier.Close()
	assert.NoError(t, err)
}

func TestNotificationTypeString(t *testing.T) {
	tests := []struct {
		nType    NotificationType
		expected string
	}{
		{TypePriceAlert, "PRICE_ALERT"},
		{TypeStrategySignal, "STRATEGY_SIGNAL"},
		{TypeSystemAlert, "SYSTEM_ALERT"},
		{TypeHeartbeat, "HEARTBEAT"},
		{NotificationType(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.nType.String())
		})
	}
}

// MockEmailConfig 创建测试用的邮件配置
func mockEmailConfig() *config.EmailConfig {
	return &config.EmailConfig{
		Enabled: true,
		SMTP: config.SMTPConfig{
			Host:     "smtp.gmail.com",
			Port:     587,
			Username: "test@gmail.com",
			Password: "password",
			TLS:      true,
		},
		From:     "test@gmail.com",
		To:       []string{"recipient@gmail.com"},
		Subject:  "TA Watcher Alert - {{.Asset}}",
		Template: "Default template",
	}
}

// MockNotification 创建测试用的通知
func mockNotification() *Notification {
	return &Notification{
		ID:        "test-notification-1",
		Type:      TypePriceAlert,
		Asset:     "BTCUSDT",
		Strategy:  "test_strategy",
		Title:     "Price Alert",
		Message:   "BTC price has exceeded the threshold",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"current_price": 50000.0,
			"threshold":     49000.0,
			"change_pct":    2.04,
		},
	}
}
