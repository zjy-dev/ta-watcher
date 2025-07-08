package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"ta-watcher/internal/assets"
	"ta-watcher/internal/config"
	"ta-watcher/internal/datasource"
	"ta-watcher/internal/watcher"

	"gopkg.in/yaml.v3"
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

	// 设置日志输出
	setupLogging()

	// 打印启动横幅
	printBanner()

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
	log.Printf("📂 加载配置: %s", *configPath)
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Printf("❌ 配置加载失败: %v", err)
		return fmt.Errorf("配置加载失败: %w", err)
	}

	// 打印配置概要
	printConfigSummary(cfg)

	// 创建新架构的 Watcher
	w, err := watcher.New(cfg)
	if err != nil {
		log.Printf("❌ 监控器创建失败: %v", err)
		return fmt.Errorf("Watcher 创建失败: %w", err)
	}

	// 设置信号处理
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动 Watcher
	if *singleRun {
		// 单次运行模式：执行一次检查后退出
		log.Printf("🔄 单次运行模式")
		return performSingleRun(ctx, w, cfg)
	}

	log.Printf("🔄 守护进程模式")
	// 启动监控
	go func() {
		if err := w.Start(ctx); err != nil {
			log.Printf("Watcher 运行错误: %v", err)
		}
	}()

	// 如果是后台模式，不阻塞主线程
	if *daemon {
		log.Println("后台模式启动完成")
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
	w.Stop()

	log.Println("TA Watcher 已停止")
	return nil
}

// performSingleRun 执行单次检查
func performSingleRun(ctx context.Context, w *watcher.Watcher, cfg *config.Config) error {
	log.Println("🔍 开始执行单次检查...")

	// 创建一个一小时后超时的context
	checkCtx, cancel := context.WithTimeout(ctx, 60*time.Minute)
	defer cancel()

	// 1. 首先进行资产验证
	log.Println("📋 开始资产验证...")
	factory := datasource.NewFactory()
	dataSource, err := factory.CreateDataSource(cfg.DataSource.Primary, cfg)
	if err != nil {
		return fmt.Errorf("创建数据源失败: %w", err)
	}

	validator := assets.NewValidator(dataSource, &cfg.Assets)
	validationResult, err := validator.ValidateAssets(checkCtx)
	if err != nil {
		log.Printf("❌ 资产验证失败: %v", err)
	} else {
		// 输出详细的资产验证日志
		log.Printf("✅ 资产验证完成:")
		log.Printf("  - 有效币种: %d 个 %v", len(validationResult.ValidSymbols), validationResult.ValidSymbols)
		log.Printf("  - 有效交易对: %d 个 %v", len(validationResult.ValidPairs), validationResult.ValidPairs)
		log.Printf("  - 计算得出的对: %d 个 %v", len(validationResult.CalculatedPairs), validationResult.CalculatedPairs)
		if len(validationResult.MissingSymbols) > 0 {
			log.Printf("  - 缺失币种: %d 个 %v", len(validationResult.MissingSymbols), validationResult.MissingSymbols)
		}
		log.Printf("  - 支持的时间框架: %v", validationResult.SupportedTimeframes)
	}

	// 2. 解析时间框架
	var timeframes []datasource.Timeframe
	for _, tfStr := range cfg.Assets.Timeframes {
		tf := datasource.Timeframe(tfStr)
		// 简单验证时间框架是否有效
		if isValidTimeframe(tf) {
			timeframes = append(timeframes, tf)
		} else {
			log.Printf("⚠️ 无效的时间框架: %s", tfStr)
		}
	}

	if len(timeframes) == 0 {
		log.Println("⚠️ 没有有效的时间框架，使用默认值")
		timeframes = []datasource.Timeframe{datasource.Timeframe1h}
	}

	// 3. 使用验证过的交易对进行策略分析
	symbols := cfg.Assets.Symbols
	if validationResult != nil {
		// 使用所有验证通过的交易对，包括基础货币对和币币交易对
		symbols = make([]string, 0)

		// 添加基础货币对（如 BTCUSDT）
		for _, symbol := range validationResult.ValidSymbols {
			basePair := symbol + cfg.Assets.BaseCurrency
			symbols = append(symbols, basePair)
		}

		// 添加所有验证通过的币币交易对（如 ETHBTC）
		for _, pair := range validationResult.ValidPairs {
			// 避免重复添加基础货币对
			if !strings.HasSuffix(pair, cfg.Assets.BaseCurrency) {
				symbols = append(symbols, pair)
			}
		}

		// 添加所有计算得出的汇率对（如 ADASOL）
		for _, pair := range validationResult.CalculatedPairs {
			symbols = append(symbols, pair)
		}

		log.Printf("📊 策略分析将包含：")
		log.Printf("  - 基础货币对: %d 个", len(validationResult.ValidSymbols))
		log.Printf("  - 币币交易对: %d 个", len(validationResult.ValidPairs)-len(validationResult.ValidSymbols))
		log.Printf("  - 计算汇率对: %d 个", len(validationResult.CalculatedPairs))
		log.Printf("  - 总交易对: %d 个", len(symbols))
	}

	log.Printf("🎯 开始策略分析 - %d 个交易对，%d 个时间框架", len(symbols), len(timeframes))

	// 4. 调用 watcher 的 RunSingleCheck 进行策略分析
	if err := w.RunSingleCheck(checkCtx, symbols, timeframes); err != nil {
		return fmt.Errorf("策略分析失败: %w", err)
	}

	// 5. 等待一点时间确保所有信号都被处理
	time.Sleep(3 * time.Second)

	log.Println("=== 单次检查完成 ===")
	return nil
}

// statusReporter 定期报告状态
func statusReporter(w *watcher.Watcher) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		status := w.GetStatus()
		log.Printf("=== 状态报告 ===")
		log.Printf("运行状态: %t", status["running"])
		log.Printf("数据源: %s", status["data_source"])
		log.Printf("策略数量: %d", status["strategies"])
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

	// 检查配置文件格式（跳过环境变量验证）
	if _, err := loadConfigForHealthCheck(*configPath); err != nil {
		log.Printf("❌ 配置文件格式错误: %v", err)
		os.Exit(1)
	} else {
		log.Printf("✅ 配置文件格式正确")
	}

	log.Printf("✅ 健康检查完成")
}

// setupLogging 设置日志输出到文件和控制台
func setupLogging() {
	// 创建 logs 目录
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		log.Printf("警告: 无法创建日志目录: %v", err)
		return
	}

	// 生成日志文件名（包含时间戳）
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logFileName := filepath.Join(logsDir, fmt.Sprintf("ta-watcher_%s.log", timestamp))

	// 打开日志文件
	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("警告: 无法创建日志文件 %s: %v", logFileName, err)
		return
	}

	// 设置多重输出：同时输出到控制台和文件
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)

	// 设置日志格式
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)

	log.Printf("📁 日志文件: %s", logFileName)
}

// loadConfigForHealthCheck 为健康检查加载配置（跳过环境变量验证）
func loadConfigForHealthCheck(filename string) (*config.Config, error) {
	// 检查文件是否存在
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", filename)
	}

	// 读取文件内容
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 解析 YAML
	cfg := config.DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg, nil
}

// isValidTimeframe 检查时间框架是否有效
func isValidTimeframe(tf datasource.Timeframe) bool {
	validTimeframes := []datasource.Timeframe{
		datasource.Timeframe1m,
		datasource.Timeframe3m,
		datasource.Timeframe5m,
		datasource.Timeframe15m,
		datasource.Timeframe30m,
		datasource.Timeframe1h,
		datasource.Timeframe2h,
		datasource.Timeframe4h,
		datasource.Timeframe6h,
		datasource.Timeframe8h,
		datasource.Timeframe12h,
		datasource.Timeframe1d,
		datasource.Timeframe3d,
		datasource.Timeframe1w,
		datasource.Timeframe1M,
	}

	for _, valid := range validTimeframes {
		if tf == valid {
			return true
		}
	}
	return false
}

// printBanner 打印启动横幅
func printBanner() {
	log.Printf("════════════════════════════════════════")
	log.Printf("🚀 %s v%s", AppName, AppVersion)
	log.Printf("════════════════════════════════════════")
}

// printConfigSummary 打印配置概要
func printConfigSummary(cfg *config.Config) {
	log.Printf("📊 配置: %s数据源, %d币种, %d时间框架, 邮件:%t",
		cfg.DataSource.Primary,
		len(cfg.Assets.Symbols),
		len(cfg.Assets.Timeframes),
		cfg.Notifiers.Email.Enabled)
}
