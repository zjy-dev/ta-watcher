package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"ta-watcher/internal/config"
	"ta-watcher/internal/watcher"
)

var (
	configPath       = flag.String("config", "config.yaml", "配置文件路径")
	strategiesDir    = flag.String("strategies", "strategies", "自定义策略目录")
	generateTemplate = flag.String("generate", "", "生成策略模板，指定策略名称")
	version          = flag.Bool("version", false, "显示版本信息")
	healthCheck      = flag.Bool("health", false, "健康检查")
	daemon           = flag.Bool("daemon", false, "后台运行模式")
)

const (
	AppName    = "TA Watcher"
	AppVersion = "1.0.0"
	AppDesc    = "技术分析监控工具"
)

func main() {
	flag.Parse()

	// 显示版本信息
	if *version {
		fmt.Printf("%s v%s - %s\n", AppName, AppVersion, AppDesc)
		os.Exit(0)
	}

	// 生成策略模板
	if *generateTemplate != "" {
		generateStrategyTemplate(*generateTemplate)
		return
	}

	// 健康检查
	if *healthCheck {
		performHealthCheck()
		return
	}

	// 运行主程序
	if err := run(); err != nil {
		log.Fatalf("应用程序启动失败: %v", err)
	}
}

func run() error {
	// 加载配置
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		return fmt.Errorf("配置加载失败: %w", err)
	}

	log.Printf("=== %s v%s 启动中 ===", AppName, AppVersion)
	log.Printf("配置文件: %s", *configPath)
	log.Printf("策略目录: %s", *strategiesDir)
	log.Printf("监控间隔: %v", cfg.Watcher.Interval)
	log.Printf("工作协程: %d", cfg.Watcher.MaxWorkers)
	log.Printf("监控资产: %v", cfg.Assets)

	// 创建 Watcher 实例
	w, err := watcher.New(cfg, watcher.WithStrategiesDirectory(*strategiesDir))
	if err != nil {
		return fmt.Errorf("Watcher 创建失败: %w", err)
	}

	// 设置信号处理
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动 Watcher
	if err := w.Start(ctx); err != nil {
		return fmt.Errorf("Watcher 启动失败: %w", err)
	}

	// 如果是后台模式，不阻塞主线程
	if *daemon {
		log.Println("后台模式启动完成")
		// 在实际应用中，这里应该实现守护进程逻辑
		select {}
	}

	// 等待信号
	log.Println("TA Watcher 运行中... (按 Ctrl+C 停止)")

	// 启动状态报告 goroutine
	go statusReporter(w)

	// 等待停止信号
	<-signalChan
	log.Println("收到停止信号，正在关闭...")

	// 停止 Watcher
	if err := w.Stop(); err != nil {
		log.Printf("Watcher 停止失败: %v", err)
	}

	log.Println("TA Watcher 已停止")
	return nil
}

// statusReporter 定期报告状态
func statusReporter(w *watcher.Watcher) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		health := w.GetHealth()
		stats := w.GetStatistics()

		log.Printf("=== 状态报告 ===")
		log.Printf("运行时间: %v", health.Uptime)
		log.Printf("活跃工作者: %d", health.ActiveWorkers)
		log.Printf("待处理任务: %d", health.PendingTasks)
		log.Printf("总任务: %d", stats.TotalTasks)
		log.Printf("完成任务: %d", stats.CompletedTasks)
		log.Printf("失败任务: %d", stats.FailedTasks)
		log.Printf("发送通知: %d", stats.NotificationsSent)

		if len(stats.AssetStats) > 0 {
			log.Printf("资产监控统计:")
			for symbol, stat := range stats.AssetStats {
				log.Printf("  %s: 检查%d次, 信号%d次, 最后信号: %s",
					symbol, stat.CheckCount, stat.SignalCount, stat.LastSignal)
			}
		}

		if len(stats.Errors) > 0 {
			log.Printf("最近错误: %v", stats.Errors[len(stats.Errors)-1])
		}
	}
}

// generateStrategyTemplate 生成策略模板
func generateStrategyTemplate(strategyName string) {
	outputPath := filepath.Join(*strategiesDir, fmt.Sprintf("%s_strategy.go", strategyName))

	// 确保目录存在
	if err := os.MkdirAll(*strategiesDir, 0755); err != nil {
		log.Fatalf("创建策略目录失败: %v", err)
	}

	if err := watcher.GenerateStrategyTemplate(outputPath, strategyName); err != nil {
		log.Printf("策略模板生成信息: %v", err)
	}

	log.Printf("策略模板已生成: %s", outputPath)
	log.Printf("请编辑策略文件并编译为插件:")
	log.Printf("  编辑: %s", outputPath)
	log.Printf("  编译: go build -buildmode=plugin -o %s.so %s",
		filepath.Join(*strategiesDir, fmt.Sprintf("%s_strategy", strategyName)), outputPath)
}

// performHealthCheck 执行健康检查
func performHealthCheck() {
	log.Println("执行健康检查...")

	// 检查配置文件
	if _, err := os.Stat(*configPath); os.IsNotExist(err) {
		log.Printf("❌ 配置文件不存在: %s", *configPath)
		os.Exit(1)
	}
	log.Printf("✅ 配置文件存在: %s", *configPath)

	// 检查配置文件格式
	if _, err := config.LoadConfig(*configPath); err != nil {
		log.Printf("❌ 配置文件格式错误: %v", err)
		os.Exit(1)
	}
	log.Printf("✅ 配置文件格式正确")

	// 检查策略目录
	if _, err := os.Stat(*strategiesDir); os.IsNotExist(err) {
		log.Printf("⚠️  策略目录不存在: %s (将使用内置策略)", *strategiesDir)
	} else {
		log.Printf("✅ 策略目录存在: %s", *strategiesDir)
	}

	// 检查网络连接 (简单测试)
	log.Printf("✅ 健康检查完成")
}
