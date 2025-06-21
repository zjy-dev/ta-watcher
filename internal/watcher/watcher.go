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
		config:     cfg,
		dataSource: dataSource,
		notifier:   notifier,
		strategy:   strategyManager,
		stats:      newStatistics(),
	}, nil
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

	// 处理每个资产
	for _, symbol := range w.config.Assets {
		w.processAsset(symbol, strategies)
	}
}

// processAsset 处理单个资产
func (w *Watcher) processAsset(symbol string, strategies []string) {
	w.stats.mu.Lock()
	w.stats.TotalTasks++
	w.stats.mu.Unlock()

	// 获取K线数据
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	klines, err := w.dataSource.GetKlines(ctx, symbol, "1h", 100)
	if err != nil {
		log.Printf("Failed to get klines for %s: %v", symbol, err)
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
			Symbol:    symbol,
			Timeframe: strategy.Timeframe1h,
			Klines:    klineData,
		})
		if err != nil {
			log.Printf("Strategy %s failed for %s: %v", strategyName, symbol, err)
			continue
		}

		// 如果有信号，发送通知
		if result != nil && result.Signal != strategy.SignalHold {
			w.sendNotification(symbol, strategyName, result)
		}
	}

	w.stats.mu.Lock()
	w.stats.CompletedTasks++
	w.stats.LastUpdate = time.Now()
	w.stats.mu.Unlock()
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
