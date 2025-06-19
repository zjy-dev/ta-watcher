package main

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"ta-watcher/internal/config"
	"ta-watcher/internal/notifiers"
)

func main() {
	fmt.Println("=== TA Watcher 邮件通知示例 ===")
	fmt.Println()

	// 1. 创建邮件配置（演示用，实际使用时从配置文件加载）
	emailConfig := &config.EmailConfig{
		Enabled: true,
		SMTP: config.SMTPConfig{
			Host:     "smtp.gmail.com",
			Port:     587,
			Username: "your_email@gmail.com", // 替换为您的邮箱
			Password: "your_app_password",    // 替换为您的应用专用密码
			TLS:      true,
		},
		From:     "your_email@gmail.com",                // 替换为您的邮箱
		To:       []string{"zhangjingyao666@gmail.com"}, // 您提供的邮箱
		Subject:  "TA Watcher Alert - {{.Asset}} {{.Level}}",
		Template: "", // 使用默认模板
	}

	fmt.Println("📧 邮件配置:")
	fmt.Printf("   启用: %v\n", emailConfig.Enabled)
	fmt.Printf("   SMTP: %s:%d (TLS: %v)\n",
		emailConfig.SMTP.Host, emailConfig.SMTP.Port, emailConfig.SMTP.TLS)
	fmt.Printf("   发送者: %s\n", emailConfig.From)
	fmt.Printf("   接收者: %v\n", emailConfig.To)
	fmt.Println()

	// 2. 创建邮件通知器
	fmt.Println("🔧 创建邮件通知器...")
	emailNotifier, err := notifiers.NewEmailNotifier(emailConfig)
	if err != nil {
		log.Fatal("创建邮件通知器失败:", err)
	}
	defer emailNotifier.Close()

	if !emailNotifier.IsEnabled() {
		fmt.Println("⚠️ 邮件通知器已禁用，启用用于演示...")
		emailNotifier.SetEnabled(true)
	}

	fmt.Printf("✅ 邮件通知器创建成功 (名称: %s, 启用: %v)\n",
		emailNotifier.Name(), emailNotifier.IsEnabled())
	fmt.Println()

	// 3. 创建通知管理器并添加邮件通知器
	fmt.Println("🔧 创建通知管理器...")
	manager := notifiers.NewManager()

	err = manager.AddNotifier(emailNotifier)
	if err != nil {
		log.Fatal("添加邮件通知器失败:", err)
	}

	fmt.Printf("✅ 通知管理器创建成功 (总数: %d, 启用: %d)\n",
		manager.TotalCount(), manager.EnabledCount())
	fmt.Println()

	// 4. 设置通知过滤器
	fmt.Println("🔧 设置通知过滤器...")
	filter := &notifiers.NotificationFilter{
		MinLevel: notifiers.LevelInfo, // 允许所有级别
		Types: []notifiers.NotificationType{
			notifiers.TypePriceAlert,
			notifiers.TypeStrategySignal,
			notifiers.TypeSystemAlert,
		},
		Assets: []string{"BTCUSDT", "ETHUSDT", "BNBUSDT"},
	}
	manager.SetFilter(filter)
	fmt.Println("✅ 过滤器设置完成")
	fmt.Println()

	// 5. 创建各种类型的测试通知
	notifications := []*notifiers.Notification{
		{
			ID:        "demo-price-alert-" + strconv.FormatInt(time.Now().Unix(), 10),
			Type:      notifiers.TypePriceAlert,
			Level:     notifiers.LevelWarning,
			Asset:     "BTCUSDT",
			Strategy:  "price_monitor",
			Title:     "比特币价格警报",
			Message:   "比特币价格突破关键阻力位 $105,000，建议密切关注后续走势。",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"current_price":    105125.50,
				"resistance":       105000.00,
				"support":          104000.00,
				"volume_24h":       "15,678 BTC",
				"change_24h":       "+3.25%",
				"market_cap":       "$2.1T",
				"fear_greed_index": 75,
			},
		},
		{
			ID:        "demo-strategy-signal-" + strconv.FormatInt(time.Now().Unix(), 10),
			Type:      notifiers.TypeStrategySignal,
			Level:     notifiers.LevelCritical,
			Asset:     "ETHUSDT",
			Strategy:  "golden_cross",
			Title:     "以太坊黄金交叉信号",
			Message:   "以太坊出现黄金交叉信号！50日移动平均线向上突破200日移动平均线，这是一个强烈的看涨信号。建议考虑建仓。",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"signal_type":    "GOLDEN_CROSS",
				"ma_50":          2520.45,
				"ma_200":         2518.30,
				"current_price":  2523.47,
				"confidence":     0.85,
				"recommendation": "BUY",
				"stop_loss":      2400.00,
				"target_price":   2700.00,
			},
		},
		{
			ID:        "demo-system-alert-" + strconv.FormatInt(time.Now().Unix(), 10),
			Type:      notifiers.TypeSystemAlert,
			Level:     notifiers.LevelInfo,
			Title:     "系统状态更新",
			Message:   "TA Watcher 系统运行正常。已成功监控 3 个资产，执行 2 个策略，发送 5 条通知。",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"uptime":             "2h 15m 30s",
				"monitored_assets":   3,
				"active_strategies":  2,
				"notifications_sent": 5,
				"memory_usage":       "45.2 MB",
				"cpu_usage":          "12.5%",
				"last_price_update":  time.Now().Add(-30 * time.Second).Format("15:04:05"),
			},
		},
	}

	// 6. 发送测试通知
	fmt.Println("📤 发送测试通知...")
	for i, notification := range notifications {
		fmt.Printf("   %d. 发送 %s 级别的 %s 通知",
			i+1, notification.Level.String(), notification.Type.String())

		if notification.Asset != "" {
			fmt.Printf(" (%s)", notification.Asset)
		}
		fmt.Println("...")

		err = manager.Send(notification)
		if err != nil {
			fmt.Printf("   ❌ 发送失败: %v\n", err)
		} else {
			fmt.Printf("   ✅ 发送成功\n")
		}

		// 添加延迟避免邮件发送过快
		time.Sleep(1 * time.Second)
	}

	fmt.Println()

	// 7. 演示单独发送到指定通知器
	fmt.Println("📤 演示发送到指定通知器...")
	heartbeatNotification := &notifiers.Notification{
		ID:        "demo-heartbeat-" + strconv.FormatInt(time.Now().Unix(), 10),
		Type:      notifiers.TypeHeartbeat,
		Level:     notifiers.LevelInfo,
		Title:     "TA Watcher 心跳检测",
		Message:   "这是 TA Watcher 的心跳检测消息，表示系统正在正常运行。",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"heartbeat_id":   time.Now().Unix(),
			"system_load":    0.65,
			"disk_usage":     "23.4%",
			"network_status": "connected",
		},
	}

	err = manager.SendTo("email", heartbeatNotification)
	if err != nil {
		fmt.Printf("❌ 发送到邮件通知器失败: %v\n", err)
	} else {
		fmt.Println("✅ 发送到邮件通知器成功")
	}

	fmt.Println()

	// 8. 显示统计信息
	fmt.Println("📊 统计信息:")
	fmt.Printf("   通知器总数: %d\n", manager.TotalCount())
	fmt.Printf("   启用的通知器: %d\n", manager.EnabledCount())
	fmt.Printf("   通知器列表: %v\n", manager.ListNotifierNames())

	if filter := manager.GetFilter(); filter != nil {
		fmt.Printf("   过滤器最小级别: %s\n", filter.MinLevel.String())
		fmt.Printf("   允许的类型: %d 种\n", len(filter.Types))
		fmt.Printf("   允许的资产: %v\n", filter.Assets)
	}

	fmt.Println()

	// 9. 关闭资源
	fmt.Println("🔧 关闭资源...")
	err = manager.Close()
	if err != nil {
		fmt.Printf("❌ 关闭管理器失败: %v\n", err)
	} else {
		fmt.Println("✅ 管理器关闭成功")
	}

	fmt.Println()
	fmt.Println("=== 邮件通知示例程序执行完成 ===")
	fmt.Println()
	fmt.Println("📬 请检查您的邮箱 (zhangjingyao666@gmail.com):")
	fmt.Println("   1. 比特币价格警报 (WARNING 级别)")
	fmt.Println("   2. 以太坊黄金交叉信号 (CRITICAL 级别)")
	fmt.Println("   3. 系统状态更新 (INFO 级别)")
	fmt.Println("   4. 心跳检测消息 (INFO 级别)")
	fmt.Println()
	fmt.Println("💡 提示:")
	fmt.Println("   - 如果没有收到邮件，请检查垃圾邮件文件夹")
	fmt.Println("   - 确保 SMTP 配置正确")
	fmt.Println("   - 对于 Gmail，需要使用应用专用密码")
	fmt.Println("   - 这是演示程序，实际使用时请从配置文件加载设置")
}
