package notifiers

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/smtp"
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
	// 统一的交易信号邮件模板
	defaultTemplate := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; 
            line-height: 1.6; 
            margin: 0; 
            padding: 20px; 
            background-color: #f5f5f5; 
        }
        .container {
            max-width: 900px;
            margin: 0 auto;
            background-color: white;
            border-radius: 10px;
            box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
            overflow: hidden;
        }
        .header { 
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); 
            color: white; 
            padding: 25px; 
            text-align: center;
        }
        .header h1 {
            margin: 0;
            font-size: 24px;
            font-weight: 600;
        }
        .header .timestamp { 
            color: rgba(255, 255, 255, 0.9); 
            font-size: 14px; 
            margin-top: 8px;
        }
        .content { 
            padding: 30px; 
        }
        .message-content {
            font-size: 16px;
            line-height: 1.8;
        }
        .footer {
            background-color: #f8f9fa;
            padding: 20px;
            text-align: center;
            font-size: 12px;
            color: #6c757d;
            border-top: 1px solid #dee2e6;
        }
        /* 响应式设计 */
        @media (max-width: 600px) {
            body { padding: 10px; }
            .container { margin: 0; }
            .header, .content { padding: 20px; }
            .header h1 { font-size: 20px; }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>{{.Title}}</h1>
            <div class="timestamp">{{.FormattedTime}}</div>
        </div>
        <div class="content">
            <div class="message-content">
                {{.Message}}
            </div>
        </div>
        <div class="footer">
            <p>此邮件由 TA Watcher 自动发送，请勿回复。</p>
            <p>如需帮助，请联系系统管理员。</p>
        </div>
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

	var sendErr error
	if e.config.SMTP.TLS {
		sendErr = e.sendWithTLS(addr, msg)
	} else {
		sendErr = e.sendWithoutTLS(addr, msg)
	}

	if sendErr != nil {
		return sendErr
	}

	// 发送成功，记录日志
	log.Printf("📧 邮件发送成功: %s -> %v (主题: %s)", e.config.From, e.config.To, subject)

	return nil
}

// prepareEmail 准备邮件内容
func (e *EmailNotifier) prepareEmail(notification *Notification) (string, string, error) {
	// 准备模板数据
	data := struct {
		*Notification
		FormattedTime string
	}{
		Notification:  notification,
		FormattedTime: notification.Timestamp.Format("2006-01-02 15:04:05"),
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
	// 先建立普通连接，设置30秒超时
	conn, err := net.DialTimeout("tcp", addr, 30*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	// 设置连接超时
	conn.SetDeadline(time.Now().Add(60 * time.Second))

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

	// 快速连接测试，不发送实际邮件
	addr := fmt.Sprintf("%s:%d", e.config.SMTP.Host, e.config.SMTP.Port)

	// 建立连接测试，设置10秒超时
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	// 设置连接超时
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	// 创建 SMTP 客户端进行基本握手
	client, err := smtp.NewClient(conn, e.config.SMTP.Host)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Quit()

	// 如果使用TLS，测试TLS启动
	if e.config.SMTP.TLS {
		tlsConfig := &tls.Config{
			ServerName: e.config.SMTP.Host,
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("failed to start TLS: %w", err)
		}
	}

	// 测试认证（不发送实际邮件）
	if err := client.Auth(e.auth); err != nil {
		return fmt.Errorf("SMTP authentication failed: %w", err)
	}

	return nil
}
