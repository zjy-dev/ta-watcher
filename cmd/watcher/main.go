package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ta-watcher/internal/config"
	"ta-watcher/internal/watcher"
)

var (
	configPath  = flag.String("config", "config.yaml", "配置文件路径")
	version     = flag.Bool("version", false, "显示版本信息")
	healthCheck = flag.Bool("health", false, "健康检查")
	daemon      = flag.Bool("daemon", false, "后台运行模式")
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
	log.Printf("监控间隔: %v", cfg.Watcher.Interval)
	log.Printf("工作协程: %d", cfg.Watcher.MaxWorkers)
	log.Printf("监控资产: %v", cfg.Assets)

	// 创建 Watcher 实例
	w, err := watcher.New(cfg)
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
		log.Printf("总任务: %d", stats.TotalTasks)
		log.Printf("完成任务: %d", stats.CompletedTasks)
		log.Printf("失败任务: %d", stats.FailedTasks)
		log.Printf("发送通知: %d", stats.NotificationsSent)
	}
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

	log.Printf("✅ 健康检查完成")
}
