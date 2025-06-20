package watcher

import (
	"context"
	"fmt"
	"log"
	"time"

	"ta-watcher/internal/binance"
	"ta-watcher/internal/config"
	"ta-watcher/internal/notifiers"
	"ta-watcher/internal/strategy"
)

// New 创建新的 Watcher 实例
func New(cfg *config.Config, options ...WatcherOption) (*Watcher, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// 创建数据源
	dataSource, err := binance.NewClient(&cfg.Binance)
	if err != nil {
		return nil, fmt.Errorf("failed to create binance client: %w", err)
	}

	// 创建通知管理器
	notifierManager := notifiers.NewManager()

	// 创建策略集成
	watcherConfig := &strategy.WatcherConfig{
		DefaultStrategy:      "balanced_combo",
		DataLimit:            200,
		RefreshInterval:      cfg.Watcher.Interval,
		EnableNotifications:  true,
		NotificationCooldown: 15 * time.Minute,
		MaxPositions:         5,
		RiskLevel:            0.5,
		StopLossPercent:      5.0,
		TakeProfitPercent:    10.0,
	}
	strategyIntegration := strategy.NewWatcherIntegration(dataSource, watcherConfig)

	w := &Watcher{
		config:              cfg,
		dataSource:          dataSource,
		notifierManager:     notifierManager,
		strategyIntegration: strategyIntegration,
		workerPool:          make(chan struct{}, cfg.Watcher.MaxWorkers),
		resultChan:          make(chan *MonitorResult, cfg.Watcher.BufferSize),
		errorChan:           make(chan error, cfg.Watcher.BufferSize),
		cooldownTracker:     make(map[string]time.Time),
		stats:               newStatistics(),
	}

	// 应用选项
	for _, option := range options {
		if err := option(w); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	return w, nil
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

	// 启动结果处理器
	w.wg.Add(1)
	go w.resultProcessor()

	// 启动错误处理器
	w.wg.Add(1)
	go w.errorProcessor()

	// 启动主监控循环
	w.wg.Add(1)
	go w.monitorLoop()

	log.Printf("TA Watcher started with %d workers", cap(w.workerPool))
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

	// 等待所有 goroutine 完成
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("TA Watcher stopped gracefully")
	case <-time.After(30 * time.Second):
		log.Println("TA Watcher stop timeout, forcing shutdown")
	}

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
		Running:       w.running,
		Uptime:        uptime,
		ActiveWorkers: len(w.workerPool),
		PendingTasks:  len(w.resultChan),
		ComponentStatus: map[string]bool{
			"data_source": w.dataSource != nil,
			"notifier":    w.notifierManager != nil,
			"strategy":    w.strategyIntegration != nil,
		},
		Statistics: w.stats.clone(),
	}
}

// GetStatistics 获取统计信息
func (w *Watcher) GetStatistics() *Statistics {
	return w.stats.clone()
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
			w.executeMonitoringCycle()
		}
	}
}

// executeMonitoringCycle 执行一次监控周期
func (w *Watcher) executeMonitoringCycle() {
	tasks := w.generateMonitorTasks()

	log.Printf("Starting monitoring cycle with %d tasks", len(tasks))
	w.stats.IncrementTotalTasks(int64(len(tasks)))

	for _, task := range tasks {
		select {
		case <-w.ctx.Done():
			return
		case w.workerPool <- struct{}{}:
			w.wg.Add(1)
			go w.executeMonitorTask(task)
		}
	}
}

// generateMonitorTasks 生成监控任务
func (w *Watcher) generateMonitorTasks() []*MonitorTask {
	var tasks []*MonitorTask

	// 内置的策略名称
	strategyNames := []string{
		"rsi_strategy",
		"macd_strategy",
		"ma_cross_strategy",
	}

	// 预定义的时间框架组合
	timeframes := []strategy.Timeframe{
		strategy.Timeframe1h, // 1小时
		strategy.Timeframe4h, // 4小时
		strategy.Timeframe1d, // 1天
	}

	// 为每个策略、时间框架和资产组合创建任务
	for _, strategyName := range strategyNames {
		for _, timeframe := range timeframes {
			for _, symbol := range w.config.Assets {
				tasks = append(tasks, &MonitorTask{
					Symbol:       symbol,
					Timeframe:    timeframe,
					StrategyName: strategyName,
				})
			}
		}
	}

	return tasks
}

// executeMonitorTask 执行单个监控任务
func (w *Watcher) executeMonitorTask(task *MonitorTask) {
	defer func() {
		<-w.workerPool // 释放 worker slot
		w.wg.Done()
	}()

	result := &MonitorResult{
		Symbol:       task.Symbol,
		Timeframe:    task.Timeframe,
		StrategyName: task.StrategyName,
		Timestamp:    time.Now(),
	}

	// 更新资产统计
	w.stats.UpdateAssetStat(task.Symbol)

	// 执行策略决策
	decision, err := w.strategyIntegration.MakeDecision(&strategy.DecisionRequest{
		Symbol:       task.Symbol,
		Timeframe:    task.Timeframe,
		StrategyName: task.StrategyName,
		Context:      w.ctx,
	})

	if err != nil {
		result.Error = err
		w.stats.IncrementFailedTasks()
		w.errorChan <- fmt.Errorf("strategy decision failed for %s: %w", task.Symbol, err)
	} else {
		result.Decision = decision
		w.stats.IncrementCompletedTasks()

		// 更新信号统计
		if decision.Signal != strategy.SignalNone {
			w.stats.UpdateSignalStat(task.Symbol, decision.Signal.String())
		}
	}

	w.resultChan <- result
}

// resultProcessor 处理监控结果
func (w *Watcher) resultProcessor() {
	defer w.wg.Done()

	for {
		select {
		case <-w.ctx.Done():
			return
		case result := <-w.resultChan:
			w.processResult(result)
		}
	}
}

// processResult 处理单个结果
func (w *Watcher) processResult(result *MonitorResult) {
	if result.Error != nil {
		log.Printf("Monitor task failed for %s: %v", result.Symbol, result.Error)
		return
	}

	decision := result.Decision
	if decision == nil {
		return
	}

	// 检查是否需要发送通知
	if decision.ShouldNotify && w.shouldSendNotification(result) {
		go w.sendNotification(result)
	}

	// 记录信号
	if decision.Signal != strategy.SignalNone {
		log.Printf("Signal detected: %s %s %s (%.2f%% confidence)",
			result.Symbol, decision.Signal, decision.Strength, decision.Confidence*100)
	}
}

// shouldSendNotification 检查是否应该发送通知
func (w *Watcher) shouldSendNotification(result *MonitorResult) bool {
	key := fmt.Sprintf("%s:%s:%s", result.Symbol, result.StrategyName, result.Decision.Signal)

	w.cooldownMu.Lock()
	defer w.cooldownMu.Unlock()

	lastNotification, exists := w.cooldownTracker[key]
	if !exists || time.Since(lastNotification) > 15*time.Minute {
		w.cooldownTracker[key] = time.Now()
		return true
	}

	return false
}

// sendNotification 发送通知
func (w *Watcher) sendNotification(result *MonitorResult) {
	decision := result.Decision

	notification := &notifiers.Notification{
		Level:     ParseNotificationLevel(decision.NotificationLevel),
		Title:     fmt.Sprintf("TA Signal: %s %s", result.Symbol, decision.Signal),
		Message:   decision.Message,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"symbol":     result.Symbol,
			"signal":     decision.Signal.String(),
			"strength":   decision.Strength.String(),
			"confidence": decision.Confidence,
			"price":      decision.Price,
			"strategy":   result.StrategyName,
			"timeframe":  TimeframeToString(result.Timeframe),
		},
	}

	if err := w.notifierManager.Send(notification); err != nil {
		w.errorChan <- fmt.Errorf("failed to send notification for %s: %w", result.Symbol, err)
	} else {
		w.stats.IncrementNotificationsSent()
		log.Printf("Notification sent for %s: %s", result.Symbol, decision.Signal)
	}
}

// errorProcessor 处理错误
func (w *Watcher) errorProcessor() {
	defer w.wg.Done()

	for {
		select {
		case <-w.ctx.Done():
			return
		case err := <-w.errorChan:
			w.handleError(err)
		}
	}
}

// handleError 处理错误
func (w *Watcher) handleError(err error) {
	log.Printf("Error: %v", err)
	w.stats.AddError(err.Error())
}

// newStatistics 创建新的统计实例
func newStatistics() *Statistics {
	return &Statistics{
		StartTime:  time.Now(),
		AssetStats: make(map[string]*AssetStat),
		Errors:     make([]string, 0),
		LastUpdate: time.Now(),
	}
}

// WithStrategiesDirectory 设置自定义策略目录选项
func WithStrategiesDirectory(dir string) WatcherOption {
	return func(w *Watcher) error {
		if dir == "" {
			return nil
		}

		// 创建策略加载器
		factory := strategy.NewFactory()
		loader := NewStrategyLoader(dir, factory)

		// 加载自定义策略
		if err := loader.LoadStrategiesFromDirectory(); err != nil {
			log.Printf("Failed to load custom strategies: %v", err)
			// 不返回错误，只是记录日志，让程序继续运行使用内置策略
		}

		return nil
	}
}
