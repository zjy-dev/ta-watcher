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

	"ta-watcher/internal/assets"
	"ta-watcher/internal/binance"
	"ta-watcher/internal/coinbase"
	"ta-watcher/internal/config"
	"ta-watcher/internal/watcher"
)

var (
	configPath  = flag.String("config", "config.yaml", "配置文件路径")
	version     = flag.Bool("version", false, "显示版本信息")
	healthCheck = flag.Bool("health", false, "健康检查")
	daemon      = flag.Bool("daemon", false, "后台运行模式")
	singleRun   = flag.Bool("single-run", false, "单次运行模式（用于定时任务/云函数）")
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
	log.Printf("配置的币种: %v", cfg.Assets.Symbols)
	log.Printf("监控时间框架: %v", cfg.Assets.Timeframes)

	// 根据配置创建适当的数据源
	log.Println("正在初始化数据源...")

	// 默认使用 Binance 数据源
	var dataSource binance.DataSource

	// 检查是否配置了数据源选择
	primarySource := "binance" // 默认值
	if cfg.DataSource.Primary != "" {
		primarySource = cfg.DataSource.Primary
	}

	switch primarySource {
	case "binance":
		log.Println("使用 Binance 数据源")
		binanceClient, err := binance.NewClient(&cfg.Binance)
		if err != nil {
			return fmt.Errorf("Binance 客户端创建失败: %w", err)
		}
		dataSource = binanceClient

	case "coinbase":
		log.Println("使用 Coinbase 数据源（通过适配器）")
		// 转换配置格式
		coinbaseConfig := &coinbase.Config{
			RateLimit: struct {
				RequestsPerMinute int           `yaml:"requests_per_minute"`
				RetryDelay        time.Duration `yaml:"retry_delay"`
				MaxRetries        int           `yaml:"max_retries"`
			}{
				RequestsPerMinute: cfg.DataSource.Coinbase.RateLimit.RequestsPerMinute,
				RetryDelay:        cfg.DataSource.Coinbase.RateLimit.RetryDelay,
				MaxRetries:        cfg.DataSource.Coinbase.RateLimit.MaxRetries,
			},
		}
		coinbaseClient := coinbase.NewClient(coinbaseConfig)
		// 使用适配器将 Coinbase 客户端包装为 binance.DataSource 接口
		dataSource = coinbase.NewBinanceAdapter(coinbaseClient)

	default:
		return fmt.Errorf("不支持的数据源: %s", primarySource)
	}

	// 预检查：验证所有配置的资产
	log.Println("开始资产预检查...")
	validator := assets.NewValidator(dataSource, &cfg.Assets)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	validationResult, err := validator.ValidateAssets(ctx)
	if err != nil {
		return fmt.Errorf("资产验证失败: %w", err)
	}

	// 显示验证结果
	log.Println(validationResult.Summary())

	// 如果有缺失的币种，给出警告但继续运行
	if len(validationResult.MissingSymbols) > 0 {
		log.Printf("警告: 以下币种将被跳过: %v", validationResult.MissingSymbols)
	}

	// 确保至少有一个有效币种
	if len(validationResult.ValidSymbols) == 0 {
		return fmt.Errorf("没有找到任何有效的监控币种，请检查配置")
	}

	log.Printf("资产预检查完成，将监控 %d 个币种", len(validationResult.ValidSymbols))

	// 创建 Watcher 实例，并传入验证结果
	w, err := watcher.NewWithValidationResult(cfg, validationResult)
	if err != nil {
		return fmt.Errorf("Watcher 创建失败: %w", err)
	}

	// 设置信号处理
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动 Watcher
	if *singleRun {
		// 单次运行模式：执行一次检查后退出
		log.Println("=== 单次运行模式 ===")
		return runSingleCheck(ctx2, w)
	}

	if err := w.Start(ctx2); err != nil {
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

// runSingleCheck 执行单次检查
func runSingleCheck(ctx context.Context, w *watcher.Watcher) error {
	log.Println("开始执行单次检查...")

	// 创建一个短期context，确保检查不会无限期运行
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// 执行一次完整的检查周期
	if err := w.RunSingleCheck(checkCtx); err != nil {
		return fmt.Errorf("单次检查失败: %w", err)
	}

	// 获取统计信息
	stats := w.GetStatistics()
	log.Printf("=== 单次检查完成 ===")
	log.Printf("处理任务: %d", stats.TotalTasks)
	log.Printf("完成任务: %d", stats.CompletedTasks)
	log.Printf("失败任务: %d", stats.FailedTasks)
	log.Printf("发送通知: %d", stats.NotificationsSent)

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
