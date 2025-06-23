package watcher

import (
	"context"
	"fmt"
	"log"
	"time"

	"ta-watcher/internal/assets"
	"ta-watcher/internal/binance"
	"ta-watcher/internal/config"
	"ta-watcher/internal/notifiers"
	"ta-watcher/internal/strategy"
)

// New 创建新的 Watcher 实例
func New(cfg *config.Config) (*Watcher, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// 创建数据源
	dataSource, err := binance.NewClient(&cfg.Binance)
	if err != nil {
		return nil, fmt.Errorf("failed to create binance client: %w", err)
	}

	// 创建通知管理器
	notifier := notifiers.NewManager()

	// 创建策略管理器
	strategyManager := strategy.NewManager(strategy.DefaultManagerConfig())

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

	log.Printf("运行监控周期，监控 %d 个交易对", len(allPairs))

	// 处理每个验证的交易对的每个时间框架
	for _, pair := range allPairs {
		for _, timeframe := range w.config.Assets.Timeframes {
			w.processAssetTimeframe(pair, timeframe, strategies)
		}
	}
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
		klines, err = w.getCalculatedKlines(ctx, pair, timeframe, 100)
	} else {
		klines, err = w.dataSource.GetKlines(ctx, pair, timeframe, 100)
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
			log.Printf("Strategy %s failed for %s (%s): %v", strategyName, pair, timeframe, err)
			continue
		}

		// 如果有信号，发送通知
		if result != nil && result.Signal != strategy.SignalHold {
			w.sendNotification(pair, strategyName, result)
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
	// 解析交易对：例如 "BTCETH" -> "BTC", "ETH"
	baseSymbol, quoteSymbol := w.parseCrossRatePair(pair)
	if baseSymbol == "" || quoteSymbol == "" {
		return nil, fmt.Errorf("invalid cross rate pair: %s", pair)
	}

	// 使用汇率计算器
	calculator := assets.NewRateCalculator(w.dataSource)
	return calculator.CalculateRate(ctx, baseSymbol, quoteSymbol, w.config.Assets.BaseCurrency, timeframe, limit)
}

// parseCrossRatePair 解析交叉汇率对
// 例如 "BTCETH" -> ("BTC", "ETH")
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
func (w *Watcher) sendNotification(symbol, strategyName string, result *strategy.StrategyResult) {
	message := fmt.Sprintf("Signal detected for %s by %s: %s at %.6f",
		symbol, strategyName, result.Signal, result.Price)

	notification := &notifiers.Notification{
		ID:        fmt.Sprintf("%s-%s-%d", symbol, strategyName, time.Now().Unix()),
		Type:      notifiers.TypeStrategySignal,
		Level:     notifiers.LevelWarning,
		Asset:     symbol,
		Strategy:  strategyName,
		Title:     "Trading Signal",
		Message:   message,
		Timestamp: time.Now(),
	}

	err := w.notifier.Send(notification)
	if err != nil {
		log.Printf("Failed to send notification: %v", err)
		return
	}

	w.stats.mu.Lock()
	w.stats.NotificationsSent++
	w.stats.mu.Unlock()

	log.Printf("Notification sent: %s", message)
}
