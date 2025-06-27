package watcher

import (
	"context"
	"fmt"
	"log"
	"time"

	"ta-watcher/internal/assets"
	"ta-watcher/internal/binance"
	"ta-watcher/internal/coinbase"
	"ta-watcher/internal/config"
	"ta-watcher/internal/notifiers"
	"ta-watcher/internal/strategy"
)

// New 创建新的 Watcher 实例
func New(cfg *config.Config) (*Watcher, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// 创建数据源（根据配置选择）
	var dataSource binance.DataSource
	var err error

	// 根据配置的主数据源创建对应的客户端
	primarySource := cfg.DataSource.Primary
	switch primarySource {
	case "binance":
		dataSource, err = binance.NewClient(&cfg.Binance)
		if err != nil {
			return nil, fmt.Errorf("failed to create binance client: %w", err)
		}
	case "coinbase":
		// 创建 Coinbase 适配器
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
		dataSource = coinbase.NewBinanceAdapter(coinbaseClient)
		log.Println("✅ Watcher 内部使用 Coinbase 数据源（通过适配器）")
	default:
		return nil, fmt.Errorf("unsupported data source: %s", primarySource)
	}

	return newWatcherWithDataSource(cfg, dataSource)
}

// NewWithDataSource 使用指定的数据源创建 Watcher
func NewWithDataSource(cfg *config.Config, dataSource binance.DataSource) (*Watcher, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if dataSource == nil {
		return nil, fmt.Errorf("dataSource cannot be nil")
	}

	return newWatcherWithDataSource(cfg, dataSource)
}

// newWatcherWithDataSource 内部函数：使用数据源创建 Watcher
func newWatcherWithDataSource(cfg *config.Config, dataSource binance.DataSource) (*Watcher, error) {

	// 创建通知管理器
	notifier := notifiers.NewManager()

	// 如果启用了邮件通知，创建并添加邮件通知器
	if cfg.Notifiers.Email.Enabled {
		emailNotifier, err := notifiers.NewEmailNotifier(&cfg.Notifiers.Email)
		if err != nil {
			return nil, fmt.Errorf("failed to create email notifier: %w", err)
		}
		if err := notifier.AddNotifier(emailNotifier); err != nil {
			return nil, fmt.Errorf("failed to add email notifier: %w", err)
		}
		log.Printf("✅ 邮件通知器已启用 -> %v", cfg.Notifiers.Email.To)
	} else {
		log.Printf("⚠️ 邮件通知器未启用")
	}

	// 创建策略管理器
	strategyManager := strategy.NewManager(strategy.DefaultManagerConfig())

	// 注册RSI策略 - 通知系统使用敏感参数
	// 参数调整：65超买/35超卖，更合理的阈值适合通知系统
	rsiStrategy := strategy.NewRSIStrategy(14, 65, 35)
	if err := strategyManager.RegisterStrategy(rsiStrategy); err != nil {
		return nil, fmt.Errorf("failed to register RSI strategy: %w", err)
	}
	log.Printf("✅ 已注册RSI策略: %s", rsiStrategy.Description())
	log.Printf("📊 策略参数: RSI周期=%d, 超买阈值=%.0f, 超卖阈值=%.0f (通知系统优化)", 14, 65.0, 35.0)

	return &Watcher{
		config:           cfg,
		dataSource:       dataSource,
		notifier:         notifier,
		strategy:         strategyManager,
		validationResult: nil, // 需要通过 SetValidationResult 设置
		stats:            newStatistics(),
	}, nil
}

// NewWithValidationResult 创建带有验证结果的 Watcher 实例
func NewWithValidationResult(cfg *config.Config, validationResult *assets.ValidationResult) (*Watcher, error) {
	watcher, err := New(cfg)
	if err != nil {
		return nil, err
	}
	watcher.validationResult = validationResult
	return watcher, nil
}

// NewWithValidationResultAndDataSource 创建带有验证结果和指定数据源的 Watcher 实例
func NewWithValidationResultAndDataSource(cfg *config.Config, validationResult *assets.ValidationResult, dataSource binance.DataSource) (*Watcher, error) {
	watcher, err := NewWithDataSource(cfg, dataSource)
	if err != nil {
		return nil, err
	}
	watcher.validationResult = validationResult
	return watcher, nil
}

// SetValidationResult 设置验证结果
func (w *Watcher) SetValidationResult(result *assets.ValidationResult) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.validationResult = result
}

// Start 启动监控服务
func (w *Watcher) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		return fmt.Errorf("watcher is already running")
	}

	w.ctx, w.cancel = context.WithCancel(ctx)
	w.running = true
	w.stats.StartTime = time.Now()

	log.Println("Starting TA Watcher...")

	// 显示当前注册的策略
	strategies := w.strategy.ListStrategies()
	log.Printf("🎯 当前注册的策略数量: %d", len(strategies))
	for i, strategyName := range strategies {
		if strategyObj, err := w.strategy.GetStrategy(strategyName); err == nil {
			log.Printf("   %d. %s - %s", i+1, strategyName, strategyObj.Description())
		}
	}

	// 启动监控循环
	w.wg.Add(1)
	go w.monitorLoop()

	log.Println("TA Watcher started")
	return nil
}

// Stop 停止监控服务
func (w *Watcher) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return fmt.Errorf("watcher is not running")
	}

	log.Println("Stopping TA Watcher...")

	w.cancel()
	w.running = false
	w.wg.Wait()

	log.Println("TA Watcher stopped")
	return nil
}

// IsRunning 检查服务是否运行中
func (w *Watcher) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// RunSingleCheck 执行单次检查 - 用于云函数/定时任务模式
func (w *Watcher) RunSingleCheck(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		return fmt.Errorf("watcher is already running in continuous mode")
	}

	// 设置单次运行状态
	w.ctx = ctx
	w.stats.StartTime = time.Now()

	log.Println("🎯 开始单次检查模式...")

	// 显示当前注册的策略
	strategies := w.strategy.ListStrategies()
	log.Printf("🎯 当前注册的策略数量: %d", len(strategies))
	for i, strategyName := range strategies {
		if strategyObj, err := w.strategy.GetStrategy(strategyName); err == nil {
			log.Printf("   %d. %s - %s", i+1, strategyName, strategyObj.Description())
		}
	}

	// 执行一次监控周期
	w.runMonitorCycle()

	log.Println("✅ 单次检查完成")
	return nil
}

// GetHealth 获取健康状态
func (w *Watcher) GetHealth() *HealthStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var uptime time.Duration
	if w.running {
		uptime = time.Since(w.stats.StartTime)
	}

	return &HealthStatus{
		Running:    w.running,
		Uptime:     uptime,
		TasksTotal: w.stats.TotalTasks,
		TasksOK:    w.stats.CompletedTasks,
		TasksError: w.stats.FailedTasks,
		StartTime:  w.stats.StartTime,
	}
}

// GetStatistics 获取统计信息
func (w *Watcher) GetStatistics() *Statistics {
	w.stats.mu.RLock()
	defer w.stats.mu.RUnlock()
	return &Statistics{
		StartTime:         w.stats.StartTime,
		TotalTasks:        w.stats.TotalTasks,
		CompletedTasks:    w.stats.CompletedTasks,
		FailedTasks:       w.stats.FailedTasks,
		NotificationsSent: w.stats.NotificationsSent,
		LastUpdate:        w.stats.LastUpdate,
	}
}

// monitorLoop 主监控循环
func (w *Watcher) monitorLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.config.Watcher.Interval)
	defer ticker.Stop()

	log.Printf("Monitor loop started, interval: %v", w.config.Watcher.Interval)

	// 启动时立即执行一次监控周期
	log.Println("🚀 启动时立即执行策略检查...")
	w.runMonitorCycle()

	for {
		select {
		case <-w.ctx.Done():
			log.Println("Monitor loop stopped")
			return
		case <-ticker.C:
			w.runMonitorCycle()
		}
	}
}

// runMonitorCycle 运行一次监控周期
func (w *Watcher) runMonitorCycle() {
	// 获取策略列表
	strategies := w.strategy.ListStrategies()
	if len(strategies) == 0 {
		log.Println("No strategies available")
		return
	}

	// 如果有验证结果，使用验证的交易对；否则使用传统方法
	if w.validationResult != nil {
		w.runValidatedMonitorCycle(strategies)
	} else {
		w.runLegacyMonitorCycle(strategies)
	}
}

// runValidatedMonitorCycle 使用验证结果运行监控周期
func (w *Watcher) runValidatedMonitorCycle(strategies []string) {
	allPairs := w.validationResult.GetAllMonitoringPairs()

	log.Printf("🚀 开始监控周期 - 监控 %d 个交易对，使用 %d 个策略，%d 个时间框架",
		len(allPairs), len(strategies), len(w.config.Assets.Timeframes))

	// 处理每个验证的交易对的每个时间框架
	for _, pair := range allPairs {
		for _, timeframe := range w.config.Assets.Timeframes {
			w.processAssetTimeframe(pair, timeframe, strategies)
		}
	}

	log.Printf("✅ 监控周期完成")
}

// runLegacyMonitorCycle 使用传统方法运行监控周期（向后兼容）
func (w *Watcher) runLegacyMonitorCycle(strategies []string) {
	// 处理每个资产的每个时间框架
	for _, symbol := range w.config.Assets.Symbols {
		for _, timeframe := range w.config.Assets.Timeframes {
			// 构建交易对（币种 + 基准货币）
			pair := symbol + w.config.Assets.BaseCurrency
			w.processAssetTimeframe(pair, timeframe, strategies)
		}
	}
}

// processAssetTimeframe 处理单个资产的特定时间框架
func (w *Watcher) processAssetTimeframe(pair, timeframe string, strategies []string) {
	w.stats.mu.Lock()
	w.stats.TotalTasks++
	w.stats.mu.Unlock()

	// 获取K线数据
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var klines []*binance.KlineData
	var err error

	// 检查是否为计算汇率对
	if w.validationResult != nil && w.isCalculatedPair(pair) {
		klines, err = w.getCalculatedKlines(ctx, pair, timeframe, 200) // 增加到200以确保足够数据
	} else {
		klines, err = w.dataSource.GetKlines(ctx, pair, timeframe, 200) // 增加到200以确保足够数据
	}

	if err != nil {
		log.Printf("Failed to get klines for %s (%s): %v", pair, timeframe, err)
		w.stats.mu.Lock()
		w.stats.FailedTasks++
		w.stats.mu.Unlock()
		return
	}

	// 对每个策略进行检查
	for _, strategyName := range strategies {
		strategyObj, err := w.strategy.GetStrategy(strategyName)
		if err != nil {
			log.Printf("Failed to get strategy %s: %v", strategyName, err)
			continue
		}

		// 检查数据点是否足够
		requiredDataPoints := strategyObj.RequiredDataPoints()
		if len(klines) < requiredDataPoints {
			log.Printf("⚠️ %s [%s] 数据不足，需要 %d 个数据点，实际只有 %d 个，跳过策略 %s",
				pair, timeframe, requiredDataPoints, len(klines), strategyName)
			continue
		}

		log.Printf("🔍 分析 %s [%s] 使用策略 %s", pair, timeframe, strategyName)

		// 转换数据格式
		klineData := make([]binance.KlineData, len(klines))
		for i, kline := range klines {
			klineData[i] = *kline
		}

		result, err := strategyObj.Evaluate(&strategy.MarketData{
			Symbol:    pair,
			Timeframe: strategy.Timeframe(timeframe),
			Klines:    klineData,
		})
		if err != nil {
			log.Printf("❌ 策略 %s 分析 %s [%s] 失败: %v", strategyName, pair, timeframe, err)
			continue
		}

		// 记录分析结果（包括无信号的情况，用于调试）
		if result != nil {
			log.Printf("📊 %s [%s] %s策略结果: %s | 价格: $%.6f | %s",
				pair, timeframe, strategyName, result.Signal.String(), result.Price, result.Message)
		}

		// 只有买入和卖出信号才发送通知，忽略无信号和持有信号
		if result != nil && (result.Signal == strategy.SignalBuy || result.Signal == strategy.SignalSell) {
			w.sendNotification(pair, timeframe, strategyName, result)
		}
	}

	w.stats.mu.Lock()
	w.stats.CompletedTasks++
	w.stats.LastUpdate = time.Now()
	w.stats.mu.Unlock()
}

// isCalculatedPair 检查是否为计算汇率对
func (w *Watcher) isCalculatedPair(pair string) bool {
	if w.validationResult == nil {
		return false
	}

	for _, calculatedPair := range w.validationResult.CalculatedPairs {
		if calculatedPair == pair {
			return true
		}
	}
	return false
}

// getCalculatedKlines 获取计算的K线数据
func (w *Watcher) getCalculatedKlines(ctx context.Context, pair, timeframe string, limit int) ([]*binance.KlineData, error) {
	// 解析交易对：例如 "ETHBTC" -> "ETH", "BTC"
	baseSymbol, quoteSymbol := w.parseCrossRatePair(pair)
	if baseSymbol == "" || quoteSymbol == "" {
		return nil, fmt.Errorf("invalid cross rate pair: %s", pair)
	}

	// 使用汇率计算器
	calculator := assets.NewRateCalculator(w.dataSource)
	return calculator.CalculateRate(ctx, baseSymbol, quoteSymbol, w.config.Assets.BaseCurrency, timeframe, limit)
}

// parseCrossRatePair 解析交叉汇率对
// 例如 "ETHBTC" -> ("ETH", "BTC")
func (w *Watcher) parseCrossRatePair(pair string) (string, string) {
	// 这是一个简化的解析器，假设按市值排序的交易对
	// 在实际实现中，可能需要更复杂的逻辑来正确分割

	// 尝试匹配已知的币种
	validSymbols := w.config.Assets.Symbols

	for _, symbol1 := range validSymbols {
		if len(pair) > len(symbol1) && pair[:len(symbol1)] == symbol1 {
			remaining := pair[len(symbol1):]
			for _, symbol2 := range validSymbols {
				if remaining == symbol2 {
					return symbol1, symbol2
				}
			}
		}
	}

	return "", ""
}

// sendNotification 发送通知
func (w *Watcher) sendNotification(symbol, timeframe, strategyName string, result *strategy.StrategyResult) {
	// 确定价格类型和信号描述
	var signalDesc, priceType string
	switch result.Signal {
	case strategy.SignalBuy:
		signalDesc = "🟢 买入信号"
		priceType = "建议买入价"
	case strategy.SignalSell:
		signalDesc = "🔴 卖出信号"
		priceType = "建议卖出价"
	default:
		signalDesc = result.Signal.String()
		priceType = "当前价格"
	}

	// 构建详细的消息
	message := fmt.Sprintf("%s | %s [%s] | %s策略 | %s: $%.6f",
		signalDesc, symbol, timeframe, strategyName, priceType, result.Price)

	// 如果有置信度信息，添加到消息中
	if result.Confidence > 0 {
		message += fmt.Sprintf(" | 置信度: %.1f%%", result.Confidence*100)
	}

	// 如果有额外消息，添加到消息中
	if result.Message != "" {
		message += fmt.Sprintf(" | %s", result.Message)
	}

	notification := &notifiers.Notification{
		ID:        fmt.Sprintf("%s-%s-%s-%d", symbol, timeframe, strategyName, time.Now().Unix()),
		Type:      notifiers.TypeStrategySignal,
		Level:     notifiers.LevelWarning,
		Asset:     symbol,
		Strategy:  strategyName,
		Title:     fmt.Sprintf("%s - %s", signalDesc, symbol),
		Message:   message,
		Timestamp: time.Now(),
	}

	err := w.notifier.Send(notification)
	if err != nil {
		log.Printf("❌ 邮件发送失败: %v", err)
		log.Printf("📧 %s (邮件发送失败)", message)
		return
	}

	w.stats.mu.Lock()
	w.stats.NotificationsSent++
	w.stats.mu.Unlock()

	// 获取邮件收件人信息用于日志
	recipients := ""
	if len(w.config.Notifiers.Email.To) > 0 {
		recipients = w.config.Notifiers.Email.To[0]
		if len(w.config.Notifiers.Email.To) > 1 {
			recipients += fmt.Sprintf(" 等%d个收件人", len(w.config.Notifiers.Email.To))
		}
	}

	log.Printf("📧 %s", message)
	if recipients != "" {
		log.Printf("✅ 邮件已成功发送到: %s", recipients)
	} else {
		log.Printf("✅ 邮件已成功发送")
	}
}
