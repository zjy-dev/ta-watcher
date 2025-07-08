package notifiers

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"hash/fnv"
	"html/template"
	"log"
	"net"
	"net/smtp"
	"os"
	"path/filepath"
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
	// 传统蓝色浅色主题邮件模板
	defaultTemplate := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        @import url('https://fonts.googleapis.com/css2?family=Microsoft+YaHei:wght@400;500;600;700&family=Consolas:wght@400;500;600&display=swap');
        
        body { 
            font-family: 'Microsoft YaHei', 'PingFang SC', 'Hiragino Sans GB', 'Helvetica Neue', Arial, sans-serif; 
            line-height: 1.6; 
            margin: 0; 
            padding: 20px; 
            background-color: #f0f4f8;
            color: #2c3e50;
        }
        .container {
            max-width: 900px;
            margin: 0 auto;
            background-color: white;
            border-radius: 12px;
            box-shadow: 0 4px 20px rgba(52, 73, 94, 0.1);
            overflow: hidden;
            border: 1px solid #e3f2fd;
        }
        .header { 
            background: linear-gradient(135deg, #3498db 0%, #2980b9 100%); 
            color: white; 
            padding: 30px; 
            text-align: center;
        }
        .header h1 {
            margin: 0 0 12px 0;
            font-size: 26px;
            font-weight: 600;
        }
        .header .timestamp { 
            color: rgba(255, 255, 255, 0.9); 
            font-size: 14px; 
            margin-bottom: 20px;
            font-family: 'Consolas', monospace;
        }
        .summary-section {
            background: linear-gradient(135deg, #e3f2fd 0%, #f8faff 100%);
            padding: 25px;
            border-bottom: 1px solid #e0e7ed;
        }
        .summary-title {
            font-size: 18px;
            font-weight: 600;
            color: #2980b9;
            margin-bottom: 15px;
            text-align: center;
        }
        .summary-stats {
            display: flex;
            justify-content: space-around;
            margin-bottom: 20px;
            flex-wrap: wrap;
        }
        .stat-item {
            text-align: center;
            padding: 15px;
            background: white;
            border-radius: 8px;
            border: 1px solid #d6e9f5;
            flex: 1;
            margin: 0 5px;
            min-width: 120px;
        }
        .stat-number {
            font-size: 24px;
            font-weight: 600;
            margin-bottom: 5px;
        }
        .stat-label {
            font-size: 12px;
            color: #7f8c8d;
        }
        .stat-buy { color: #27ae60; }
        .stat-sell { color: #e74c3c; }
        .stat-total { color: #3498db; }
        .content { 
            padding: 30px; 
        }
        .signals-title {
            font-size: 20px;
            font-weight: 600;
            color: #2c3e50;
            margin-bottom: 25px;
            padding-bottom: 12px;
            border-bottom: 2px solid #ecf0f1;
        }
        .signal-item {
            border: 1px solid #e3f2fd;
            border-radius: 10px;
            margin-bottom: 20px;
            overflow: hidden;
            background: white;
            box-shadow: 0 2px 8px rgba(52, 73, 94, 0.08);
        }
        .signal-header {
            padding: 18px 22px;
            background: linear-gradient(135deg, #f8faff 0%, #e3f2fd 100%);
            border-bottom: 1px solid #e0e7ed;
        }
        .signal-header-top {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 12px;
        }
        .signal-number {
            background: #3498db;
            color: white;
            padding: 6px 12px;
            border-radius: 6px;
            font-size: 14px;
            font-weight: 500;
            min-width: 30px;
            text-align: center;
        }
        .signal-asset {
            font-size: 20px;
            font-weight: 600;
            color: #2c3e50;
            margin-left: 15px;
        }
        .signal-direction {
            padding: 8px 16px;
            border-radius: 6px;
            font-size: 14px;
            font-weight: 500;
        }
        .signal-buy { background: #d5f4e6; color: #27ae60; }
        .signal-sell { background: #fdeaea; color: #e74c3c; }
        .signal-meta {
            font-size: 13px;
            color: #7f8c8d;
            font-family: 'Consolas', monospace;
        }
        .signal-body {
            padding: 22px;
        }
        .signal-core {
            background: #f8faff;
            border: 1px solid #e3f2fd;
            border-radius: 8px;
            padding: 16px;
            margin-bottom: 18px;
        }
        .core-title {
            font-size: 14px;
            font-weight: 600;
            color: #2980b9;
            margin-bottom: 8px;
        }
        .core-value {
            font-family: 'Consolas', monospace;
            font-size: 16px;
            font-weight: 600;
            color: #27ae60;
        }
        .signal-analysis {
            margin-bottom: 18px;
        }
        .analysis-title {
            font-size: 14px;
            font-weight: 600;
            color: #2c3e50;
            margin-bottom: 10px;
        }
        .analysis-content {
            color: #7f8c8d;
            line-height: 1.7;
            background: #fafbfc;
            padding: 14px;
            border-radius: 6px;
            border-left: 4px solid #3498db;
        }
        .signal-table {
            margin-bottom: 18px;
        }
        .table-title {
            font-size: 14px;
            font-weight: 600;
            color: #2c3e50;
            margin-bottom: 12px;
        }
        .data-table {
            width: 100%;
            border-collapse: collapse;
            font-size: 13px;
            background: white;
            border: 1px solid #e3f2fd;
            border-radius: 6px;
            overflow: hidden;
        }
        .data-table th {
            background: #f8faff;
            padding: 12px 15px;
            text-align: left;
            font-weight: 600;
            color: #2980b9;
            border-bottom: 1px solid #e0e7ed;
        }
        .data-table td {
            padding: 12px 15px;
            border-bottom: 1px solid #f5f6fa;
            font-family: 'Consolas', monospace;
            color: #34495e;
        }
        .data-table tr:last-child td {
            border-bottom: none;
        }
        .signal-advice {
            background: linear-gradient(135deg, #e8f5e8 0%, #f0f8f0 100%);
            border: 1px solid #d5f4e6;
            border-radius: 8px;
            padding: 16px;
            position: relative;
        }
        .advice-title {
            font-size: 12px;
            font-weight: 600;
            color: #27ae60;
            position: absolute;
            top: -8px;
            left: 15px;
            background: white;
            padding: 0 8px;
        }
        .advice-content {
            color: #27ae60;
            font-size: 14px;
            line-height: 1.6;
            margin-top: 8px;
        }
        .footer {
            background: #34495e;
            padding: 25px;
            text-align: center;
            color: white;
        }
        .footer-disclaimer {
            margin-bottom: 15px;
            font-size: 13px;
            color: #bdc3c7;
            line-height: 1.6;
        }
        .footer-contact {
            font-size: 12px;
            color: #3498db;
            border-top: 1px solid #4a5f7a;
            padding-top: 12px;
        }
        .footer-contact a {
            color: #3498db;
            text-decoration: none;
        }
        /* 响应式设计 */
        @media (max-width: 600px) {
            body { padding: 10px; }
            .container { margin: 0; border-radius: 8px; }
            .header, .summary-section, .content { padding: 20px; }
            .header h1 { font-size: 22px; }
            .summary-stats { flex-direction: column; }
            .stat-item { margin: 5px 0; }
            .signal-header-top { flex-direction: column; align-items: flex-start; }
            .signal-asset { margin-left: 0; margin-top: 8px; }
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
                {{.HTMLMessage}}
            </div>
        </div>
        <div class="footer">
            <div class="footer-disclaimer">
                本报告仅供教育学习使用，所有信号不构成投资建议。数字货币投资有风险，请谨慎决策。
            </div>
            <div class="footer-contact">
                技术支持: <a href="mailto:yysfg666@gmail.com">yysfg666@gmail.com</a>
            </div>
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
	addr := net.JoinHostPort(e.config.SMTP.Host, fmt.Sprintf("%d", e.config.SMTP.Port))
	log.Printf("📧 开始发送邮件: %s -> %v (共 %d 个收件人)", e.config.From, e.config.To, len(e.config.To))

	var sendErr error
	if e.config.SMTP.TLS {
		sendErr = e.sendWithTLS(addr, msg)
	} else {
		sendErr = e.sendWithoutTLS(addr, msg)
	}

	if sendErr != nil {
		log.Printf("❌ 邮件发送失败: %v", sendErr)
		return sendErr
	}

	// 发送成功，记录日志
	log.Printf("📧 邮件发送成功: %s -> %v (主题: %s)", e.config.From, e.config.To, subject)

	// 保存HTML预览
	if err := e.saveHTMLPreview(subject, body); err != nil {
		log.Printf("⚠️ 保存HTML预览失败: %v", err)
	}

	return nil
}

// prepareEmail 准备邮件内容
func (e *EmailNotifier) prepareEmail(notification *Notification) (string, string, error) {
	// 准备模板数据
	// 设置时区为 UTC+8
	loc, _ := time.LoadLocation("Asia/Shanghai")

	data := struct {
		*Notification
		FormattedTime string
		HTMLMessage   template.HTML // 不会被HTML转义的消息内容
	}{
		Notification:  notification,
		FormattedTime: notification.Timestamp.In(loc).Format("2006-01-02 15:04:05 (UTC+8)"),
		HTMLMessage:   template.HTML(notification.Message), // 将消息转换为template.HTML类型
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

	// 构建To头部，确保格式正确
	toHeader := strings.Join(e.config.To, ", ")
	msg.WriteString(fmt.Sprintf("To: %s\r\n", toHeader))

	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	msg.WriteString("\r\n")

	// 邮件正文
	msg.WriteString(body)

	log.Printf("📧 邮件头部 To: %s (包含 %d 个收件人)", toHeader, len(e.config.To))
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
		log.Printf("📧 添加收件人: %s", to)
		if err := client.Rcpt(to); err != nil {
			return fmt.Errorf("failed to set recipient %s: %w", to, err)
		}
	}
	log.Printf("📧 所有收件人添加完成，共 %d 个", len(e.config.To))

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
	log.Printf("📧 使用 SendMail 发送到 %d 个收件人", len(e.config.To))
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
	addr := net.JoinHostPort(e.config.SMTP.Host, fmt.Sprintf("%d", e.config.SMTP.Port))

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

// saveHTMLPreview 保存HTML预览
func (e *EmailNotifier) saveHTMLPreview(subject, body string) error {
	// 创建预览目录
	dir := "email_previews"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create preview directory: %w", err)
	}

	// 生成带时间戳的文件名
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("email_preview_%s_%s.html", timestamp, hashString(subject)[:8])

	// 保存文件
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		return fmt.Errorf("failed to save HTML preview: %w", err)
	}

	// 同时保存最新的预览文件
	latestPath := filepath.Join(dir, "latest_email_preview.html")
	if err := os.WriteFile(latestPath, []byte(body), 0644); err != nil {
		log.Printf("⚠️ 保存最新预览文件失败: %v", err)
	}

	log.Printf("📄 HTML预览已保存: %s", path)
	return nil
}

// hashString 计算字符串的哈希值
func hashString(s string) string {
	h := fnv.New32a()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum32())
}

// PrepareEmailForTesting 为测试暴露的prepareEmail方法
func (e *EmailNotifier) PrepareEmailForTesting(notification *Notification) (string, string, error) {
	return e.prepareEmail(notification)
}

// BuildMessageForTesting 为测试暴露的buildMessage方法
func (e *EmailNotifier) BuildMessageForTesting(subject, body string) []byte {
	return e.buildMessage(subject, body)
}
