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
	emailNotifier   *notifiers.EmailNotifier
	rateCalculator  *assets.RateCalculator
	signals         []SignalInfo // 简单存储信号信息
	lastReportTime  time.Time
}

// SignalInfo 简单的信号信息结构
type SignalInfo struct {
	Symbol     string
	Timeframe  string
	Signal     strategy.Signal
	RSI        float64
	Price      float64
	Confidence float64
	Strategy   string
	Timestamp  time.Time
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
	var emailNotifier *notifiers.EmailNotifier

	// 添加邮件通知器
	if cfg.Notifiers.Email.Enabled {
		log.Printf("🔔 启用邮件通知器: %s", cfg.Notifiers.Email.SMTP.Password)
		emailNotifier, err = notifiers.NewEmailNotifier(&cfg.Notifiers.Email)
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
		emailNotifier:   emailNotifier,
		rateCalculator:  rateCalculator,
		signals:         make([]SignalInfo, 0),
		lastReportTime:  time.Now(),
	}, nil
}

// Start 启动监控
func (w *Watcher) Start(ctx context.Context) error {
	symbols := []string{"BTCUSDT", "ETHUSDT"}
	timeframes := []datasource.Timeframe{datasource.Timeframe1h, datasource.Timeframe4h}

	// 创建一个带有取消功能的上下文
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, symbol := range symbols {
		for _, tf := range timeframes {
			go w.Watch(cancelCtx, symbol, tf)
		}
	}

	// 创建定时报告发送器（每10分钟检查一次是否需要发送报告）
	reportTicker := time.NewTicker(10 * time.Minute)
	defer reportTicker.Stop()

	go func() {
		for {
			select {
			case <-cancelCtx.Done():
				return
			case <-reportTicker.C:
				w.checkAndSendReport()
			}
		}
	}()

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
					// 记录信号
					if rsiVal, ok := rsiValue.(float64); ok {
						w.recordSymbol(symbol, timeframe, strat.Name(), result, rsiVal)
					} else {
						w.recordSymbol(symbol, timeframe, strat.Name(), result, 0)
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

// recordSymbol 将信号添加到信号列表并检查是否发送报告
func (w *Watcher) recordSymbol(symbol string, timeframe datasource.Timeframe, strategyName string, result *strategy.StrategyResult, rsiValue float64) {
	if w.emailNotifier == nil {
		return
	}

	// 获取当前价格（从策略结果的指标中获取，如果有的话）
	var price float64
	if closePrice, exists := result.Indicators["close"]; exists {
		if p, ok := closePrice.(float64); ok {
			price = p
		}
	}

	// 添加信号到简单列表
	signal := SignalInfo{
		Symbol:     symbol,
		Timeframe:  string(timeframe),
		Signal:     result.Signal,
		RSI:        rsiValue,
		Price:      price,
		Confidence: result.Confidence,
		Strategy:   strategyName,
		Timestamp:  time.Now(),
	}
	w.signals = append(w.signals, signal)

	log.Printf("📊 信号已记录: %s %s 信号 (置信度: %.1f%%)",
		symbol, result.Signal.String(), result.Confidence*100)
}

// checkAndSendReport 检查并发送报告
func (w *Watcher) checkAndSendReport() {
	if w.emailNotifier == nil {
		return
	}

	// 发送条件：有信号且距离上次报告超过1分钟，或者信号数量达到3个
	now := time.Now()
	timeSinceLastReport := now.Sub(w.lastReportTime)
	signalCount := len(w.signals)

	shouldSend := false
	reason := ""

	if signalCount >= 3 {
		shouldSend = true
		reason = "信号数量达到3个"
	} else if signalCount > 0 && timeSinceLastReport >= 1*time.Minute {
		shouldSend = true
		reason = "距离上次报告超过1分钟"
	}

	if shouldSend {
		w.sendReport(reason)
	}
}

// sendReport 发送报告
func (w *Watcher) sendReport(reason string) {
	if w.emailNotifier == nil {
		return
	}

	if len(w.signals) == 0 {
		return
	}

	// 创建交易报告通知
	notification := w.createTradingReportNotification(reason)

	// 发送通知
	if err := w.emailNotifier.Send(notification); err != nil {
		log.Printf("❌ 发送交易报告失败: %v", err)
	} else {
		log.Printf("📧 交易报告已发送: %d个信号 (%s)",
			len(w.signals), reason)
	}

	// 重置信号列表和更新时间
	w.signals = make([]SignalInfo, 0)
	w.lastReportTime = time.Now()
}

// createTradingReportNotification 创建交易报告通知
func (w *Watcher) createTradingReportNotification(reason string) *notifiers.Notification {
	// 统计信号
	buySignals := 0
	sellSignals := 0
	for _, signal := range w.signals {
		switch signal.Signal {
		case strategy.SignalBuy:
			buySignals++
		case strategy.SignalSell:
			sellSignals++
		}
	}

	// 生成通知标题
	title := fmt.Sprintf("TA Watcher 交易报告 - %d个信号", len(w.signals))

	// 生成通知消息
	message := fmt.Sprintf(`🚀 TA Watcher 交易分析报告

📊 报告摘要:
• 总信号数: %d
• 买入信号: %d  
• 卖出信号: %d
• 生成时间: %s
• 触发原因: %s

📈 信号详情:`,
		len(w.signals),
		buySignals,
		sellSignals,
		time.Now().Format("2006-01-02 15:04:05"),
		reason)

	// 添加信号详情
	for i, signal := range w.signals {
		if i >= 10 { // 限制显示前10个信号
			message += fmt.Sprintf("\n... 还有 %d 个信号", len(w.signals)-10)
			break
		}

		message += fmt.Sprintf(`
%d. %s (%s) - %s
   • RSI: %.1f
   • 价格: %.6f  
   • 置信度: %.1f%%
   • 策略: %s
   • 时间: %s`,
			i+1,
			signal.Symbol,
			signal.Timeframe,
			signal.Signal.String(),
			signal.RSI,
			signal.Price,
			signal.Confidence*100,
			signal.Strategy,
			signal.Timestamp.Format("15:04:05"))
	}

	message += `

⚠️ 免责声明: 本报告仅供参考，不构成投资建议。投资有风险，入市需谨慎。

---
🤖 此报告由 TA Watcher v1.0.0 自动生成`

	// 创建附加数据
	data := make(map[string]interface{})
	data["total_signals"] = len(w.signals)
	data["buy_signals"] = buySignals
	data["sell_signals"] = sellSignals
	data["generated_at"] = time.Now()
	data["reason"] = reason

	// 添加信号数据
	signalData := make([]map[string]interface{}, len(w.signals))
	for i, signal := range w.signals {
		signalData[i] = map[string]interface{}{
			"symbol":     signal.Symbol,
			"timeframe":  signal.Timeframe,
			"signal":     signal.Signal.String(),
			"rsi":        signal.RSI,
			"price":      signal.Price,
			"confidence": signal.Confidence,
			"strategy":   signal.Strategy,
			"timestamp":  signal.Timestamp,
		}
	}
	data["signals"] = signalData

	return &notifiers.Notification{
		ID:        fmt.Sprintf("trading-report-%d", time.Now().Unix()),
		Type:      notifiers.TypeStrategySignal,
		Level:     notifiers.LevelWarning,
		Title:     title,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// sendNoSignalReport 发送无信号报告
func (w *Watcher) sendNoSignalReport() {
	if w.emailNotifier == nil {
		return
	}

	// 创建无信号通知
	notification := &notifiers.Notification{
		ID:    fmt.Sprintf("no-signal-report-%d", time.Now().Unix()),
		Type:  notifiers.TypeSystemAlert,
		Level: notifiers.LevelInfo,
		Title: "TA Watcher 分析报告 - 未发现交易信号",
		Message: `🔍 TA Watcher 市场分析完成

📊 分析摘要:
• 交易信号: 0 个
• 分析时间: ` + time.Now().Format("2006-01-02 15:04:05") + `
• 分析状态: 完成

💡 市场状况:
市场分析已完成，当前市场处于观望状态，未发现明显的交易机会。
建议继续关注市场动态，等待更好的交易时机。

📈 技术分析:
• RSI 指标: 在正常范围内波动
• 市场趋势: 相对稳定
• 交易建议: 保持观望

⚠️ 免责声明: 
本报告仅供参考，不构成投资建议。投资有风险，入市需谨慎。

---
🤖 此报告由 TA Watcher v1.0.0 自动生成`,
		Data: map[string]interface{}{
			"total_signals":  0,
			"analysis_time":  time.Now(),
			"market_status":  "stable",
			"recommendation": "hold",
		},
		Timestamp: time.Now(),
	}

	// 发送报告
	if err := w.emailNotifier.Send(notification); err != nil {
		log.Printf("❌ 发送无信号报告失败: %v", err)
	} else {
		log.Printf("📧 无信号分析报告已发送")
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

	// 单次检查结束后，强制发送所有累积的信号报告
	if len(w.signals) > 0 {
		log.Printf("📧 单次检查发现 %d 个信号，正在发送报告...", len(w.signals))
		// log.Printf("邮箱配置: %v", w.emailNotifier.Config().Email)
		w.sendReport("单次检查完成")
	} else {
		log.Printf("📭 单次检查未发现交易信号")
	}

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
