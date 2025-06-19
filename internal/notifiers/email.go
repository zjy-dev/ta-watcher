package notifiers

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"ta-watcher/internal/config"
)

// EmailNotifier 邮件通知器
type EmailNotifier struct {
	config   *config.EmailConfig
	auth     smtp.Auth
	template *template.Template
	enabled  bool
}

// NewEmailNotifier 创建新的邮件通知器
func NewEmailNotifier(cfg *config.EmailConfig) (*EmailNotifier, error) {
	if cfg == nil {
		return nil, fmt.Errorf("email config cannot be nil")
	}

	notifier := &EmailNotifier{
		config:  cfg,
		enabled: cfg.Enabled,
	}

	// 如果未启用，直接返回
	if !cfg.Enabled {
		return notifier, nil
	}

	// 验证配置
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid email config: %w", err)
	}

	// 设置 SMTP 认证
	notifier.auth = smtp.PlainAuth(
		"",
		cfg.SMTP.Username,
		cfg.SMTP.Password,
		cfg.SMTP.Host,
	)

	// 解析邮件模板
	if err := notifier.parseTemplate(); err != nil {
		return nil, fmt.Errorf("failed to parse email template: %w", err)
	}

	return notifier, nil
}

// parseTemplate 解析邮件模板
func (e *EmailNotifier) parseTemplate() error {
	// 默认邮件模板
	defaultTemplate := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>{{.Title}}</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .header { background-color: {{.HeaderColor}}; color: white; padding: 15px; border-radius: 5px 5px 0 0; }
        .content { background-color: #f9f9f9; padding: 20px; border: 1px solid #ddd; border-radius: 0 0 5px 5px; }
        .info { margin: 10px 0; }
        .info strong { color: #333; }
        .timestamp { color: #666; font-size: 0.9em; }
        .data { background-color: #fff; padding: 10px; border-left: 4px solid {{.HeaderColor}}; margin: 10px 0; }
    </style>
</head>
<body>
    <div class="header">
        <h2>{{.Title}}</h2>
        <div class="timestamp">{{.FormattedTime}}</div>
    </div>
    <div class="content">
        <div class="info"><strong>级别:</strong> {{.Level}}</div>
        <div class="info"><strong>类型:</strong> {{.Type}}</div>
        {{if .Asset}}<div class="info"><strong>资产:</strong> {{.Asset}}</div>{{end}}
        {{if .Strategy}}<div class="info"><strong>策略:</strong> {{.Strategy}}</div>{{end}}
        <div class="info"><strong>消息:</strong></div>
        <div class="data">{{.Message}}</div>
        {{if .Data}}
        <div class="info"><strong>详细数据:</strong></div>
        <div class="data">
            {{range $key, $value := .Data}}
            <strong>{{$key}}:</strong> {{$value}}<br>
            {{end}}
        </div>
        {{end}}
    </div>
</body>
</html>
`

	// 使用配置中的模板或默认模板
	templateText := e.config.Template
	if templateText == "" || templateText == "Default template" {
		templateText = defaultTemplate
	}

	// 解析模板
	tmpl, err := template.New("email").Parse(templateText)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	e.template = tmpl
	return nil
}

// Send 发送邮件通知
func (e *EmailNotifier) Send(notification *Notification) error {
	if !e.enabled {
		return nil // 未启用时直接返回
	}

	// 准备邮件内容
	subject, body, err := e.prepareEmail(notification)
	if err != nil {
		return fmt.Errorf("failed to prepare email: %w", err)
	}

	// 构建邮件
	msg := e.buildMessage(subject, body)

	// 发送邮件
	addr := fmt.Sprintf("%s:%d", e.config.SMTP.Host, e.config.SMTP.Port)

	if e.config.SMTP.TLS {
		return e.sendWithTLS(addr, msg)
	} else {
		return e.sendWithoutTLS(addr, msg)
	}
}

// prepareEmail 准备邮件内容
func (e *EmailNotifier) prepareEmail(notification *Notification) (string, string, error) {
	// 准备模板数据
	data := struct {
		*Notification
		FormattedTime string
		HeaderColor   string
	}{
		Notification:  notification,
		FormattedTime: notification.Timestamp.Format("2006-01-02 15:04:05"),
		HeaderColor:   e.getLevelColor(notification.Level),
	}

	// 渲染邮件内容
	var body bytes.Buffer
	if err := e.template.Execute(&body, data); err != nil {
		return "", "", fmt.Errorf("failed to execute template: %w", err)
	}

	// 准备邮件主题
	subject := e.config.Subject
	if subject == "" || strings.Contains(subject, "{{") {
		// 如果主题包含模板变量，进行替换
		subjTmpl := template.Must(template.New("subject").Parse(e.config.Subject))
		var subjBuf bytes.Buffer
		if err := subjTmpl.Execute(&subjBuf, data); err != nil {
			subject = fmt.Sprintf("TA Watcher Alert - %s", notification.Asset)
		} else {
			subject = subjBuf.String()
		}
	}

	return subject, body.String(), nil
}

// getLevelColor 根据级别获取颜色
func (e *EmailNotifier) getLevelColor(level NotificationLevel) string {
	switch level {
	case LevelInfo:
		return "#2196F3" // 蓝色
	case LevelWarning:
		return "#FF9800" // 橙色
	case LevelError:
		return "#F44336" // 红色
	case LevelCritical:
		return "#9C27B0" // 紫色
	default:
		return "#757575" // 灰色
	}
}

// buildMessage 构建邮件消息
func (e *EmailNotifier) buildMessage(subject, body string) []byte {
	var msg bytes.Buffer

	// 邮件头部
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	msg.WriteString(fmt.Sprintf("From: %s\r\n", e.config.From))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(e.config.To, ", ")))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("\r\n")

	// 邮件正文
	msg.WriteString(body)

	return msg.Bytes()
}

// sendWithTLS 使用 STARTTLS 发送邮件
func (e *EmailNotifier) sendWithTLS(addr string, msg []byte) error {
	// Gmail 等服务商在端口 587 使用 STARTTLS，而不是直接 TLS
	// 先建立普通连接
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	// 创建 SMTP 客户端
	client, err := smtp.NewClient(conn, e.config.SMTP.Host)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Quit()

	// 启动 TLS
	tlsConfig := &tls.Config{
		ServerName: e.config.SMTP.Host,
	}

	if err := client.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("failed to start TLS: %w", err)
	}

	// 认证
	if err := client.Auth(e.auth); err != nil {
		return fmt.Errorf("SMTP authentication failed: %w", err)
	}

	// 设置发送者
	if err := client.Mail(e.config.From); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// 设置接收者
	for _, to := range e.config.To {
		if err := client.Rcpt(to); err != nil {
			return fmt.Errorf("failed to set recipient %s: %w", to, err)
		}
	}

	// 发送数据
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to start data transmission: %w", err)
	}

	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return w.Close()
}

// sendWithoutTLS 不使用 TLS 发送邮件
func (e *EmailNotifier) sendWithoutTLS(addr string, msg []byte) error {
	return smtp.SendMail(addr, e.auth, e.config.From, e.config.To, msg)
}

// Close 关闭邮件通知器
func (e *EmailNotifier) Close() error {
	// 邮件通知器无需特殊关闭操作
	return nil
}

// IsEnabled 是否启用
func (e *EmailNotifier) IsEnabled() bool {
	return e.enabled
}

// Name 通知器名称
func (e *EmailNotifier) Name() string {
	return "email"
}

// SetEnabled 设置启用状态
func (e *EmailNotifier) SetEnabled(enabled bool) {
	e.enabled = enabled
}

// TestConnection 测试邮件连接
func (e *EmailNotifier) TestConnection() error {
	if !e.enabled {
		return fmt.Errorf("email notifier is disabled")
	}

	// 创建测试通知
	testNotification := &Notification{
		ID:        "test-" + strconv.FormatInt(time.Now().Unix(), 10),
		Type:      TypeSystemAlert,
		Level:     LevelInfo,
		Title:     "TA Watcher - 邮件连接测试",
		Message:   "这是一封测试邮件，用于验证邮件通知功能是否正常工作。",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"test_time": time.Now().Format("2006-01-02 15:04:05"),
			"version":   "1.0.0",
		},
	}

	return e.Send(testNotification)
}
