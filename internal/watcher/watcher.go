package watcher

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"ta-watcher/internal/assets"
	"ta-watcher/internal/config"
	"ta-watcher/internal/datasource"
	"ta-watcher/internal/notifiers"
	"ta-watcher/internal/strategy"
)

// Watcher 重构后的监控器
type Watcher struct {
	dataSource      datasource.DataSource
	strategies      []strategy.Strategy
	notifierManager *notifiers.Manager
	rateCalculator  *assets.RateCalculator
}

// New 创建新的监控器
func New(cfg *config.Config) (*Watcher, error) {
	factory := datasource.NewFactory()
	ds, err := factory.CreateDataSource(cfg.DataSource.Primary, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create data source: %w", err)
	}

	strategyFactory := strategy.NewFactory()
	strategies := []strategy.Strategy{}

	rsiStrategy, err := strategyFactory.CreateStrategy("rsi_oversold")
	if err == nil {
		strategies = append(strategies, rsiStrategy)
	}

	// 创建通知管理器
	notifierManager := notifiers.NewManager()

	// 添加邮件通知器
	if cfg.Notifiers.Email.Enabled {
		emailNotifier, err := notifiers.NewEmailNotifier(&cfg.Notifiers.Email)
		if err == nil {
			if err := notifierManager.AddNotifier(emailNotifier); err == nil {
				log.Printf("✅ 邮件通知器已启用")
			}
		}
	}

	// 创建汇率计算器
	rateCalculator := assets.NewRateCalculator(ds)

	return &Watcher{
		dataSource:      ds,
		strategies:      strategies,
		notifierManager: notifierManager,
		rateCalculator:  rateCalculator,
	}, nil
}

// Start 启动监控
func (w *Watcher) Start(ctx context.Context) error {
	symbols := []string{"BTCUSDT", "ETHUSDT"}
	timeframes := []datasource.Timeframe{datasource.Timeframe1h, datasource.Timeframe4h}

	for _, symbol := range symbols {
		for _, tf := range timeframes {
			go w.Watch(ctx, symbol, tf)
		}
	}

	<-ctx.Done()
	return nil
}

// Watch 监控单个交易对
func (w *Watcher) Watch(ctx context.Context, symbol string, timeframe datasource.Timeframe) error {
	maxDataPoints := 50
	for _, strat := range w.strategies {
		if required := strat.RequiredDataPoints(); required > maxDataPoints {
			maxDataPoints = required
		}
	}

	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := w.analyzeSymbol(ctx, symbol, timeframe, maxDataPoints); err != nil {
				log.Printf("❌ 分析 %s 时出错: %v", symbol, err)
			}
		}
	}
}

// analyzeSymbol 分析交易对
func (w *Watcher) analyzeSymbol(ctx context.Context, symbol string, timeframe datasource.Timeframe, maxDataPoints int) error {
	endTime := time.Now()

	// 根据时间框架计算正确的开始时间
	var duration time.Duration
	switch timeframe {
	case datasource.Timeframe1m:
		duration = time.Duration(maxDataPoints*2) * time.Minute
	case datasource.Timeframe3m:
		duration = time.Duration(maxDataPoints*2) * 3 * time.Minute
	case datasource.Timeframe5m:
		duration = time.Duration(maxDataPoints*2) * 5 * time.Minute
	case datasource.Timeframe15m:
		duration = time.Duration(maxDataPoints*2) * 15 * time.Minute
	case datasource.Timeframe30m:
		duration = time.Duration(maxDataPoints*2) * 30 * time.Minute
	case datasource.Timeframe1h:
		duration = time.Duration(maxDataPoints*2) * time.Hour
	case datasource.Timeframe2h:
		duration = time.Duration(maxDataPoints*2) * 2 * time.Hour
	case datasource.Timeframe4h:
		duration = time.Duration(maxDataPoints*2) * 4 * time.Hour
	case datasource.Timeframe6h:
		duration = time.Duration(maxDataPoints*2) * 6 * time.Hour
	case datasource.Timeframe8h:
		duration = time.Duration(maxDataPoints*2) * 8 * time.Hour
	case datasource.Timeframe12h:
		duration = time.Duration(maxDataPoints*2) * 12 * time.Hour
	case datasource.Timeframe1d:
		duration = time.Duration(maxDataPoints*2) * 24 * time.Hour
	case datasource.Timeframe3d:
		duration = time.Duration(maxDataPoints*2) * 3 * 24 * time.Hour
	case datasource.Timeframe1w:
		duration = time.Duration(maxDataPoints*2) * 7 * 24 * time.Hour
	case datasource.Timeframe1M:
		duration = time.Duration(maxDataPoints*2) * 30 * 24 * time.Hour
	default:
		// 默认按小时计算
		duration = time.Duration(maxDataPoints*2) * time.Hour
	}

	startTime := endTime.Add(-duration)

	// 尝试直接获取K线数据
	klines, err := w.dataSource.GetKlines(ctx, symbol, timeframe, startTime, endTime, maxDataPoints*2)
	if err != nil {
		// 如果直接获取失败，尝试计算汇率对
		calculatedKlines, calcErr := w.tryCalculateRatePair(ctx, symbol, timeframe, startTime, endTime, maxDataPoints*2)
		if calcErr != nil {
			return fmt.Errorf("获取K线数据失败，计算汇率也失败: 原始错误=%v, 计算错误=%v", err, calcErr)
		}
		klines = calculatedKlines
	}

	if len(klines) < maxDataPoints {
		log.Printf("⚠️ [%s %s] 数据不足: %d/%d", symbol, timeframe, len(klines), maxDataPoints)
		return fmt.Errorf("数据点不足: 需要 %d，实际 %d", maxDataPoints, len(klines))
	}

	marketData := &strategy.MarketData{
		Symbol:    symbol,
		Timeframe: timeframe,
		Klines:    klines,
		Timestamp: time.Now(),
	}

	for _, strat := range w.strategies {
		result, err := strat.Evaluate(marketData)
		if err != nil {
			log.Printf("❌ [%s %s] 策略错误: %v", symbol, timeframe, err)
			continue
		}

		if result != nil {
			// 只显示RSI结果和信号
			if rsiValue, exists := result.Indicators["rsi"]; exists {
				if result.ShouldNotify() {
					// 触发信号时
					log.Printf("🚨 [%s %s] RSI:%.1f %s", symbol, timeframe, rsiValue, result.Signal.String())
					// 发送邮件通知
					if rsiVal, ok := rsiValue.(float64); ok {
						w.sendNotification(symbol, timeframe, strat.Name(), result, rsiVal)
					} else {
						w.sendNotification(symbol, timeframe, strat.Name(), result, 0)
					}
				} else {
					// 正常状态
					log.Printf("📗 [%s %s] RSI:%.1f", symbol, timeframe, rsiValue)
				}
			}
		}
	}

	return nil
}

// sendNotification 发送通知
func (w *Watcher) sendNotification(symbol string, timeframe datasource.Timeframe, strategyName string, result *strategy.StrategyResult, rsiValue float64) {
	if w.notifierManager == nil {
		return
	}

	// 构建通知数据
	var level notifiers.NotificationLevel
	var message string
	var signalIcon string

	switch result.Signal {
	case strategy.SignalBuy:
		level = notifiers.LevelWarning
		signalIcon = "📈 买入信号"
	case strategy.SignalSell:
		level = notifiers.LevelWarning
		signalIcon = "📉 卖出信号"
	default:
		level = notifiers.LevelInfo
		signalIcon = "ℹ️ 信息"
	}

	// 构建详细消息
	if rsiValue > 0 {
		message = fmt.Sprintf("%s\n\n交易对: %s\n时间框架: %s\n策略: %s\nRSI值: %.1f\n信号类型: %s\n置信度: %.1f%%",
			signalIcon, symbol, timeframe, strategyName, rsiValue, result.Signal.String(), result.Confidence*100)
	} else {
		message = fmt.Sprintf("%s\n\n交易对: %s\n时间框架: %s\n策略: %s\n信号类型: %s\n置信度: %.1f%%",
			signalIcon, symbol, timeframe, strategyName, result.Signal.String(), result.Confidence*100)
	}

	// 构建数据字典
	data := map[string]interface{}{
		"Symbol":     symbol,
		"Timeframe":  string(timeframe),
		"Strategy":   strategyName,
		"Signal":     result.Signal.String(),
		"Confidence": fmt.Sprintf("%.1f%%", result.Confidence*100),
	}

	if rsiValue > 0 {
		data["RSI"] = fmt.Sprintf("%.1f", rsiValue)
	}

	// 添加所有指标数据
	for key, value := range result.Indicators {
		data[key] = fmt.Sprintf("%.2f", value)
	}

	notification := &notifiers.Notification{
		Level:     level,
		Type:      notifiers.TypeStrategySignal,
		Asset:     symbol,
		Strategy:  strategyName,
		Title:     fmt.Sprintf("TA Watcher - %s %s 信号", symbol, result.Signal.String()),
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}

	// 发送通知
	if err := w.notifierManager.Send(notification); err != nil {
		log.Printf("❌ 发送通知失败: %v", err)
	} else {
		log.Printf("📧 通知已发送: %s %s 信号", symbol, result.Signal.String())
	}
}

// RunSingleCheck 执行单次检查所有交易对
func (w *Watcher) RunSingleCheck(ctx context.Context, symbols []string, timeframes []datasource.Timeframe) error {
	log.Printf("🔍 开始单次检查 - %d 个交易对，%d 个时间框架", len(symbols), len(timeframes))

	// 计算所有策略需要的最大数据点数
	maxDataPoints := 0
	for _, strat := range w.strategies {
		required := strat.RequiredDataPoints()
		if required > maxDataPoints {
			maxDataPoints = required
		}
	}

	// 设置合理的最小值
	if maxDataPoints < 20 {
		maxDataPoints = 20
	}

	checkCount := 0
	for _, symbol := range symbols {
		for _, tf := range timeframes {
			log.Printf("📊 分析 %s (%s)...", symbol, tf)
			if err := w.analyzeSymbol(ctx, symbol, tf, maxDataPoints); err != nil {
				log.Printf("❌ %s (%s): %v", symbol, tf, err)
				continue
			}
			checkCount++
		}
	}

	log.Printf("✅ 单次检查完成 - 成功检查了 %d 个组合", checkCount)
	return nil
}

// Stop 停止监控 (兼容接口)
func (w *Watcher) Stop() {}

// IsRunning 检查运行状态 (兼容接口)
func (w *Watcher) IsRunning() bool {
	return true
}

// GetStatus 获取状态 (兼容接口)
func (w *Watcher) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"running":     true,
		"data_source": w.dataSource.Name(),
		"strategies":  len(w.strategies),
	}
}

// tryCalculateRatePair 尝试计算汇率对
func (w *Watcher) tryCalculateRatePair(ctx context.Context, symbol string, timeframe datasource.Timeframe, startTime, endTime time.Time, limit int) ([]*datasource.Kline, error) {
	// 检查是否是已知的计算汇率对
	// 目前支持的计算汇率对模式：ADASOL、BTCETH 等
	if len(symbol) < 6 {
		return nil, fmt.Errorf("symbol too short for rate calculation: %s", symbol)
	}

	// 尝试不同的拆分方式来识别基础币种和报价币种
	possibleSplits := []struct {
		base  string
		quote string
	}{
		// 3+3 模式 (如 ADASOL)
		{symbol[:3], symbol[3:]},
		// 3+4 模式 (如 BTCUSDT 已经有直接交易对，不应该到这里)
		{symbol[:3], symbol[3:]},
		// 4+3 模式 (如 ATOMBTC)
		{symbol[:4], symbol[4:]},
	}

	bridgeCurrency := "USDT" // 使用 USDT 作为桥接货币

	for _, split := range possibleSplits {
		baseSymbol := split.base
		quoteSymbol := split.quote

		// 验证基础币种和报价币种是否都是有效的加密货币
		if w.isValidCryptoSymbol(baseSymbol) && w.isValidCryptoSymbol(quoteSymbol) {
			log.Printf("💱 尝试计算 %s/%s 汇率，通过 %s 桥接", baseSymbol, quoteSymbol, bridgeCurrency)

			klines, err := w.rateCalculator.CalculateRate(ctx, baseSymbol, quoteSymbol, bridgeCurrency, timeframe, startTime, endTime, limit)
			if err == nil && len(klines) > 0 {
				return klines, nil
			}
			log.Printf("⚠️ 计算 %s/%s 汇率失败: %v", baseSymbol, quoteSymbol, err)
		}
	}

	return nil, fmt.Errorf("无法计算 %s 的汇率", symbol)
}

// isValidCryptoSymbol 检查是否是有效的加密货币符号
func (w *Watcher) isValidCryptoSymbol(symbol string) bool {
	// 常见的加密货币符号列表
	validSymbols := map[string]bool{
		"BTC":   true,
		"ETH":   true,
		"BNB":   true,
		"ADA":   true,
		"SOL":   true,
		"DOT":   true,
		"LINK":  true,
		"MATIC": true,
		"AVAX":  true,
		"ATOM":  true,
		"XRP":   true,
		"DOGE":  true,
		"LTC":   true,
		"BCH":   true,
		"UNI":   true,
		"AAVE":  true,
		"SUSHI": true,
		"COMP":  true,
		"MKR":   true,
		"YFI":   true,
		"USDT":  true,
		"USDC":  true,
		"BUSD":  true,
		"DAI":   true,
	}

	return validSymbols[strings.ToUpper(symbol)]
}
