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
	Symbol           string
	Timeframe        string
	Signal           strategy.Signal
	Strategy         string
	Timestamp        time.Time
	Message          string                 // 策略提供的简短消息
	IndicatorSummary string                 // 指标摘要
	DetailedAnalysis string                 // 详细分析
	AllIndicators    map[string]interface{} // 所有指标值
	Thresholds       map[string]interface{} // 策略阈值
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
			// 使用策略提供的信息，而不是硬编码RSI
			if result.ShouldNotify() {
				// 触发信号时，使用策略提供的消息
				log.Printf("🚨 [%s %s] %s", symbol, timeframe, result.Message)
				// 记录信号
				w.recordSignal(symbol, timeframe, strat.Name(), result)
			} else {
				// 正常状态，显示简化信息
				if len(result.Message) > 0 {
					log.Printf("📗 [%s %s] %s", symbol, timeframe, result.Message)
				}
			}
		}
	}

	return nil
}

// recordSignal 将信号添加到信号列表并检查是否发送报告
func (w *Watcher) recordSignal(symbol string, timeframe datasource.Timeframe, strategyName string, result *strategy.StrategyResult) {
	if w.emailNotifier == nil {
		return
	}

	// 添加信号到简单列表
	signal := SignalInfo{
		Symbol:           symbol,
		Timeframe:        string(timeframe),
		Signal:           result.Signal,
		Strategy:         strategyName,
		Timestamp:        time.Now(),
		Message:          result.Message,
		IndicatorSummary: result.IndicatorSummary,
		DetailedAnalysis: result.DetailedAnalysis,
		AllIndicators:    result.Indicators,
		Thresholds:       result.Thresholds,
	}
	w.signals = append(w.signals, signal)

	log.Printf("📊 信号已记录: %s %s 信号 - %s",
		symbol, result.Signal.String(), result.IndicatorSummary)
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
	title := fmt.Sprintf("📊 TA Watcher 交易信号报告 - %d个信号", len(w.signals))

	// 设置时区
	loc, _ := time.LoadLocation("Asia/Shanghai") // 可以从配置中读取
	now := time.Now().In(loc)

	// 生成 HTML 格式的邮件内容
	var messageBuilder strings.Builder

	// 报告头部摘要
	messageBuilder.WriteString(`<div style="margin-bottom: 25px; padding: 20px; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); border-radius: 10px; color: white;">`)
	messageBuilder.WriteString(`<h2 style="margin: 0 0 15px 0; font-size: 24px;">📊 交易信号分析报告</h2>`)
	messageBuilder.WriteString(fmt.Sprintf(`<div style="font-size: 16px; opacity: 0.9;">🕐 生成时间: %s (UTC+8)</div>`, now.Format("2006-01-02 15:04:05")))
	messageBuilder.WriteString(fmt.Sprintf(`<div style="font-size: 16px; opacity: 0.9;">📝 触发原因: %s</div>`, reason))
	messageBuilder.WriteString(`</div>`)

	// 快速统计面板
	messageBuilder.WriteString(`<div style="display: flex; gap: 15px; margin-bottom: 25px; flex-wrap: wrap;">`)

	// 总信号数卡片
	messageBuilder.WriteString(fmt.Sprintf(`<div style="flex: 1; min-width: 120px; padding: 15px; background-color: #f8f9fa; border-left: 4px solid #007bff; border-radius: 5px;">
		<div style="font-size: 24px; font-weight: bold; color: #007bff;">%d</div>
		<div style="font-size: 14px; color: #6c757d;">总信号数</div>
	</div>`, len(w.signals)))

	// 买入信号卡片
	messageBuilder.WriteString(fmt.Sprintf(`<div style="flex: 1; min-width: 120px; padding: 15px; background-color: #f8f9fa; border-left: 4px solid #28a745; border-radius: 5px;">
		<div style="font-size: 24px; font-weight: bold; color: #28a745;">%d 🟢</div>
		<div style="font-size: 14px; color: #6c757d;">买入信号</div>
	</div>`, buySignals))

	// 卖出信号卡片
	messageBuilder.WriteString(fmt.Sprintf(`<div style="flex: 1; min-width: 120px; padding: 15px; background-color: #f8f9fa; border-left: 4px solid #dc3545; border-radius: 5px;">
		<div style="font-size: 24px; font-weight: bold; color: #dc3545;">%d 🔴</div>
		<div style="font-size: 14px; color: #6c757d;">卖出信号</div>
	</div>`, sellSignals))

	messageBuilder.WriteString(`</div>`)

	// 信号详情部分
	messageBuilder.WriteString(`<div style="margin-bottom: 25px;">`)
	messageBuilder.WriteString(`<h3 style="color: #495057; margin-bottom: 20px; font-size: 20px; border-bottom: 2px solid #e9ecef; padding-bottom: 10px;">📈 交易信号详情</h3>`)

	displayCount := len(w.signals)
	if displayCount > 10 {
		displayCount = 10 // 限制显示前10个信号
	}

	for i := 0; i < displayCount; i++ {
		signal := w.signals[i]

		// 信号方向颜色和图标
		signalColor := "#28a745" // 绿色 (买入)
		signalBgColor := "#d4edda"
		signalIcon := "🟢"
		signalText := "买入机会"
		if signal.Signal == strategy.SignalSell {
			signalColor = "#dc3545" // 红色 (卖出)
			signalBgColor = "#f8d7da"
			signalIcon = "🔴"
			signalText = "卖出机会"
		}

		messageBuilder.WriteString(`<div style="border: 1px solid #dee2e6; border-radius: 10px; margin-bottom: 20px; overflow: hidden; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">`)

		// 信号头部
		messageBuilder.WriteString(fmt.Sprintf(`<div style="padding: 15px; background-color: %s; border-bottom: 1px solid #dee2e6;">
			<div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px;">
				<div style="font-size: 20px; font-weight: bold; color: %s;">%s %s</div>
				<div style="padding: 6px 12px; background-color: %s; color: white; border-radius: 20px; font-size: 14px; font-weight: bold;">%s</div>
			</div>
			<div style="font-size: 14px; color: #6c757d;">时间框架: %s | 策略: %s | 时间: %s</div>
		</div>`, signalBgColor, signalColor, signalIcon, signal.Symbol, signalColor, signalText, signal.Timeframe, signal.Strategy, signal.Timestamp.In(loc).Format("15:04:05")))

		// 信号内容区域
		messageBuilder.WriteString(`<div style="padding: 20px; background-color: white;">`)

		// 指标摘要 - 突出显示
		messageBuilder.WriteString(fmt.Sprintf(`<div style="margin-bottom: 15px; padding: 12px; background-color: #f8f9fa; border-left: 4px solid %s; border-radius: 5px;">
			<div style="font-weight: bold; color: #495057; margin-bottom: 5px;">📊 核心指标</div>
			<div style="font-family: 'Courier New', monospace; font-size: 16px; color: %s; font-weight: bold;">%s</div>
		</div>`, signalColor, signalColor, signal.IndicatorSummary))

		// 详细分析
		if signal.DetailedAnalysis != "" {
			messageBuilder.WriteString(fmt.Sprintf(`<div style="margin-bottom: 15px;">
				<div style="font-weight: bold; color: #495057; margin-bottom: 8px;">💻 技术分析</div>
				<div style="color: #6c757d; line-height: 1.6;">%s</div>
			</div>`, signal.DetailedAnalysis))
		}

		// 关键指标值表格
		if len(signal.AllIndicators) > 0 {
			messageBuilder.WriteString(`<div style="margin-bottom: 15px;">
				<div style="font-weight: bold; color: #495057; margin-bottom: 8px;">📈 关键数据</div>
				<table style="width: 100%; border-collapse: collapse; font-size: 14px;">`)

			for key, value := range signal.AllIndicators {
				displayKey := key
				switch key {
				case "rsi":
					displayKey = "RSI指标"
				case "rsi_period":
					displayKey = "RSI周期"
				case "price":
					displayKey = "当前价格"
				case "sma_short":
					displayKey = "短期均线"
				case "sma_long":
					displayKey = "长期均线"
				case "macd":
					displayKey = "MACD"
				case "macd_signal":
					displayKey = "MACD信号线"
				case "macd_histogram":
					displayKey = "MACD柱状图"
				}

				valueStr := fmt.Sprintf("%v", value)
				if fVal, ok := value.(float64); ok {
					if fVal < 1 {
						valueStr = fmt.Sprintf("%.6f", fVal)
					} else {
						valueStr = fmt.Sprintf("%.2f", fVal)
					}
				}

				messageBuilder.WriteString(fmt.Sprintf(`<tr>
					<td style="padding: 8px; border: 1px solid #dee2e6; background-color: #f8f9fa; font-weight: bold;">%s</td>
					<td style="padding: 8px; border: 1px solid #dee2e6;">%s</td>
				</tr>`, displayKey, valueStr))
			}
			messageBuilder.WriteString(`</table>`)
			messageBuilder.WriteString(`</div>`)
		}

		// 阈值信息
		if len(signal.Thresholds) > 0 {
			messageBuilder.WriteString(`<div style="margin-bottom: 15px;">
				<div style="font-weight: bold; color: #495057; margin-bottom: 8px;">⚖️ 策略阈值</div>
				<div style="display: flex; gap: 15px; flex-wrap: wrap;">`)

			for key, value := range signal.Thresholds {
				displayKey := key
				switch key {
				case "overbought_level":
					displayKey = "超买阈值"
				case "oversold_level":
					displayKey = "超卖阈值"
				case "short_period":
					displayKey = "短周期"
				case "long_period":
					displayKey = "长周期"
				}

				messageBuilder.WriteString(fmt.Sprintf(`<span style="padding: 4px 8px; background-color: #e9ecef; border-radius: 4px; font-size: 12px;">
					<strong>%s:</strong> %v
				</span>`, displayKey, value))
			}
			messageBuilder.WriteString(`</div></div>`)
		}

		// 交易建议（如果有的话）
		if signal.Message != "" {
			suggestionText := "建议关注"
			if signal.Signal == strategy.SignalBuy {
				suggestionText = "💡 这可能是一个买入机会，但请结合其他技术指标和市场环境进行综合判断"
			} else if signal.Signal == strategy.SignalSell {
				suggestionText = "💡 这可能是一个卖出机会，但请结合其他技术指标和市场环境进行综合判断"
			}

			messageBuilder.WriteString(fmt.Sprintf(`<div style="padding: 10px; background-color: %s; border-radius: 5px; margin-top: 10px;">
				<div style="color: %s; font-size: 14px;">%s</div>
			</div>`, signalBgColor, signalColor, suggestionText))
		}

		messageBuilder.WriteString(`</div>`) // 结束内容区域
		messageBuilder.WriteString(`</div>`) // 结束信号卡片
	}

	// 如果信号过多，显示提示
	if len(w.signals) > displayCount {
		messageBuilder.WriteString(fmt.Sprintf(`<div style="margin-top: 20px; text-align: center; padding: 20px; background-color: #fff3cd; border: 1px solid #ffeeba; border-radius: 10px; color: #856404;">
			<div style="font-size: 16px; font-weight: bold; margin-bottom: 5px;">📝 还有更多信号</div>
			<div>本次报告显示了前 %d 个信号，还有 %d 个信号未显示</div>
			<div style="font-size: 14px; margin-top: 10px;">完整信号详情请查看系统日志或下次报告</div>
		</div>`, displayCount, len(w.signals)-displayCount))
	}

	messageBuilder.WriteString(`</div>`) // 结束信号详情部分

	// 市场提醒和建议
	messageBuilder.WriteString(`<div style="margin: 25px 0; padding: 20px; background-color: #e7f3ff; border-left: 4px solid #2196F3; border-radius: 5px;">
		<h4 style="margin: 0 0 10px 0; color: #1976D2;">💡 交易提醒</h4>
		<ul style="margin: 0; padding-left: 20px; color: #333;">
			<li>技术指标仅供参考，建议结合基本面分析</li>
			<li>请合理控制仓位，设置止损止盈</li>
			<li>关注市场新闻和重大事件影响</li>
			<li>避免频繁交易，保持冷静理性</li>
		</ul>
	</div>`)

	// 免责声明
	messageBuilder.WriteString(`<div style="margin: 25px 0; padding: 20px; background-color: #fff3cd; border-left: 4px solid #ffc107; border-radius: 5px;">
		<h4 style="margin: 0 0 10px 0; color: #856404;">⚠️ 重要免责声明</h4>
		<div style="color: #856404; line-height: 1.6;">
			<p style="margin: 0 0 10px 0;">• 本报告由技术分析系统自动生成，仅供参考学习</p>
			<p style="margin: 0 0 10px 0;">• 所有交易信号不构成投资建议或买卖推荐</p>
			<p style="margin: 0 0 10px 0;">• 数字货币投资存在高风险，可能导致本金损失</p>
			<p style="margin: 0;">• 请根据个人风险承受能力谨慎决策，独立承担投资风险</p>
		</div>
	</div>`)

	// 页脚信息
	messageBuilder.WriteString(`<div style="margin-top: 30px; padding: 20px; background-color: #f8f9fa; border-radius: 5px; text-align: center;">
		<div style="color: #6c757d; font-size: 14px; margin-bottom: 10px;">
			🤖 此报告由 <strong>TA Watcher v1.0.0</strong> 自动生成
		</div>
		<div style="color: #6c757d; font-size: 12px;">
			生成时间: ` + now.Format("2006-01-02 15:04:05") + ` (UTC+8) | 
			如有技术问题请联系系统管理员
		</div>
	</div>`)

	message := messageBuilder.String()

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
			"symbol":            signal.Symbol,
			"timeframe":         signal.Timeframe,
			"signal":            signal.Signal.String(),
			"message":           signal.Message,
			"indicator_summary": signal.IndicatorSummary,
			"detailed_analysis": signal.DetailedAnalysis,
			"strategy":          signal.Strategy,
			"timestamp":         signal.Timestamp,
			"indicators":        signal.AllIndicators,
			"thresholds":        signal.Thresholds,
		}
	}
	data["signals"] = signalData

	return &notifiers.Notification{
		ID:        fmt.Sprintf("trading-report-%d", time.Now().Unix()),
		Type:      notifiers.TypeStrategySignal,
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
• 各项指标: 在正常范围内波动
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

	// 单次检查结束后，强制发送报告（无论是否有信号）
	if len(w.signals) > 0 {
		log.Printf("📧 单次检查发现 %d 个信号，正在发送报告...", len(w.signals))
		w.sendReport("单次检查完成")
	} else {
		log.Printf("📭 单次检查未发现交易信号，发送无信号报告...")
		w.sendNoSignalReport()
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
