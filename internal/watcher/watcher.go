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
	Symbol             string
	Timeframe          string
	Signal             strategy.Signal
	Strategy           string
	Timestamp          time.Time
	Message            string                   // 策略提供的简短消息
	IndicatorSummary   string                   // 指标摘要
	DetailedAnalysis   string                   // 详细分析
	AllIndicators      map[string]interface{}   // 所有指标值
	Thresholds         map[string]interface{}   // 策略阈值
	MultiTimeframeData map[string]TimeframeData // 多时间框架数据
}

// TimeframeData 时间框架数据
type TimeframeData struct {
	Timeframe        string
	Indicators       map[string]interface{}
	IndicatorSummary string
	DetailedAnalysis string
	HasSignal        bool
	SignalType       strategy.Signal
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
		// 如果直接获取失败，判断是否为交叉汇率对并尝试计算
		log.Printf("🔍 直接获取 %s 失败，判断是否为交叉汇率对: %v", symbol, err)
		isCrossRatePair := w.isCrossRatePair(symbol)

		if isCrossRatePair {
			log.Printf("🔄 %s 是交叉汇率对，尝试通过计算获取汇率数据", symbol)
			klines, err = w.getCrossRateKlines(ctx, symbol, timeframe, startTime, endTime, maxDataPoints*2)
			if err != nil {
				return fmt.Errorf("获取交叉汇率K线数据失败: %w", err)
			}
		} else {
			return fmt.Errorf("获取K线数据失败: %w", err)
		}
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

	// 收集该交易对在所有时间框架的数据
	multiTimeframeData := w.collectMultiTimeframeData(symbol, string(timeframe))

	// 添加信号到简单列表
	signal := SignalInfo{
		Symbol:             symbol,
		Timeframe:          string(timeframe),
		Signal:             result.Signal,
		Strategy:           strategyName,
		Timestamp:          time.Now(),
		Message:            result.Message,
		IndicatorSummary:   result.IndicatorSummary,
		DetailedAnalysis:   result.DetailedAnalysis,
		AllIndicators:      result.Indicators,
		Thresholds:         result.Thresholds,
		MultiTimeframeData: multiTimeframeData,
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

	// 生成 HTML 格式的邮件内容，使用传统中文风格
	var messageBuilder strings.Builder

	// 报告摘要 - 简洁传统风格
	messageBuilder.WriteString(`<div style="margin-bottom: 25px; padding: 20px; background: linear-gradient(135deg, #4a90e2 0%, #357abd 100%); border-radius: 8px; color: white; box-shadow: 0 4px 12px rgba(74, 144, 226, 0.2);">`)
	messageBuilder.WriteString(`<h2 style="margin: 0 0 12px 0; font-size: 22px; font-weight: 600;">📊 交易信号报告</h2>`)
	messageBuilder.WriteString(fmt.Sprintf(`<div style="font-size: 14px; opacity: 0.9; margin-bottom: 6px;">报告时间：%s</div>`, now.Format("2006-01-02 15:04:05")))
	messageBuilder.WriteString(fmt.Sprintf(`<div style="font-size: 14px; opacity: 0.9; margin-bottom: 15px;">触发原因：%s</div>`, reason))

	// 统计信息面板 - 简洁风格
	messageBuilder.WriteString(`<div style="display: flex; gap: 15px; flex-wrap: wrap; background: rgba(255,255,255,0.15); padding: 15px; border-radius: 6px;">`)
	messageBuilder.WriteString(fmt.Sprintf(`<div style="flex: 1; min-width: 100px; text-align: center;">
		<div style="font-size: 20px; font-weight: 600; color: white;">%d</div>
		<div style="font-size: 13px; opacity: 0.85;">总信号数</div>
	</div>`, len(w.signals)))
	messageBuilder.WriteString(fmt.Sprintf(`<div style="flex: 1; min-width: 100px; text-align: center;">
		<div style="font-size: 20px; font-weight: 600; color: #a8e6a3;">%d</div>
		<div style="font-size: 13px; opacity: 0.85;">买入信号</div>
	</div>`, buySignals))
	messageBuilder.WriteString(fmt.Sprintf(`<div style="flex: 1; min-width: 100px; text-align: center;">
		<div style="font-size: 20px; font-weight: 600; color: #ffb3ba;">%d</div>
		<div style="font-size: 13px; opacity: 0.85;">卖出信号</div>
	</div>`, sellSignals))
	messageBuilder.WriteString(`</div></div>`)

	// 信号汇总表 - 新增
	if len(w.signals) > 0 {
		messageBuilder.WriteString(`<div style="margin-bottom: 30px; padding: 20px; background: #ffffff; border: 1px solid #e5e5e5; border-radius: 6px;">`)
		messageBuilder.WriteString(`<h3 style="color: #2c3e50; margin-bottom: 15px; font-size: 18px; font-weight: 600; text-align: center;">📋 信号汇总</h3>`)
		messageBuilder.WriteString(`<div style="overflow-x: auto;">`)
		messageBuilder.WriteString(`<table style="width: 100%; border-collapse: collapse; font-size: 13px;">`)
		messageBuilder.WriteString(`<thead>
			<tr style="background: #f8f9fa;">
				<th style="padding: 12px 10px; text-align: left; font-weight: 600; color: #2c3e50; border-bottom: 2px solid #e5e5e5;">序号</th>
				<th style="padding: 12px 10px; text-align: left; font-weight: 600; color: #2c3e50; border-bottom: 2px solid #e5e5e5;">交易对</th>
				<th style="padding: 12px 10px; text-align: left; font-weight: 600; color: #2c3e50; border-bottom: 2px solid #e5e5e5;">时间框架</th>
				<th style="padding: 12px 10px; text-align: left; font-weight: 600; color: #2c3e50; border-bottom: 2px solid #e5e5e5;">信号类型</th>
				<th style="padding: 12px 10px; text-align: left; font-weight: 600; color: #2c3e50; border-bottom: 2px solid #e5e5e5;">核心指标</th>
				<th style="padding: 12px 10px; text-align: left; font-weight: 600; color: #2c3e50; border-bottom: 2px solid #e5e5e5;">触发时间</th>
			</tr>
		</thead>
		<tbody>`)

		for i, signal := range w.signals {
			// 信号类型样式
			signalColor := "#5cb85c"
			signalText := "买入"
			signalIcon := "📈"
			if signal.Signal == strategy.SignalSell {
				signalColor = "#d9534f"
				signalText = "卖出"
				signalIcon = "📉"
			}

			// 时间框架显示
			timeframeDisplay := signal.Timeframe
			switch signal.Timeframe {
			case "1d":
				timeframeDisplay = "日线"
			case "1w":
				timeframeDisplay = "周线"
			case "1M":
				timeframeDisplay = "月线"
			case "4h":
				timeframeDisplay = "4小时"
			case "1h":
				timeframeDisplay = "1小时"
			case "15m":
				timeframeDisplay = "15分钟"
			case "5m":
				timeframeDisplay = "5分钟"
			case "1m":
				timeframeDisplay = "1分钟"
			}

			// 核心指标简化显示 - 使用rune来正确处理中文字符
			coreIndicator := signal.IndicatorSummary
			if len([]rune(coreIndicator)) > 30 {
				runes := []rune(coreIndicator)
				coreIndicator = string(runes[:30]) + "..."
			}

			messageBuilder.WriteString(fmt.Sprintf(`<tr style="border-bottom: 1px solid #f0f0f0;">
				<td style="padding: 10px; font-weight: 600; color: #666;">%d</td>
				<td style="padding: 10px; font-weight: 600; color: #2c3e50; font-family: monospace;">%s</td>
				<td style="padding: 10px; color: #666;">%s</td>
				<td style="padding: 10px;">
					<span style="background: %s; color: white; padding: 4px 8px; border-radius: 12px; font-size: 12px; font-weight: 600;">
						%s %s
					</span>
				</td>
				<td style="padding: 10px; font-family: monospace; color: %s; font-size: 12px;">%s</td>
				<td style="padding: 10px; color: #666; font-family: monospace; font-size: 12px;">%s</td>
			</tr>`, i+1, signal.Symbol, timeframeDisplay, signalColor, signalIcon, signalText, signalColor, coreIndicator, signal.Timestamp.In(loc).Format("15:04:05")))
		}

		messageBuilder.WriteString(`</tbody></table></div></div>`)
	}

	// 信号详情部分 - 中文传统风格
	messageBuilder.WriteString(`<div style="margin-bottom: 30px;">`)
	messageBuilder.WriteString(`<h3 style="color: #2c3e50; margin-bottom: 20px; font-size: 20px; font-weight: 600; text-align: center; padding: 12px; background: linear-gradient(90deg, transparent, rgba(74, 144, 226, 0.1), transparent); border-radius: 6px;">📊 交易信号详情</h3>`)

	displayCount := len(w.signals)
	if displayCount > 10 {
		displayCount = 10 // 限制显示前10个信号
	}

	for i := 0; i < displayCount; i++ {
		signal := w.signals[i]

		// 信号方向颜色和图标 - 传统风格
		signalColor := "#5cb85c" // 蓝绿色 (买入)
		signalBgColor := "#f0f8ff"
		signalIcon := "↗"
		signalText := "买入"
		signalEmoji := "📈"
		if signal.Signal == strategy.SignalSell {
			signalColor = "#d9534f" // 红色 (卖出)
			signalBgColor = "#fff5f5"
			signalIcon = "↘"
			signalText = "卖出"
			signalEmoji = "📉"
		}

		messageBuilder.WriteString(`<div style="border: 1px solid #e5e5e5; border-radius: 6px; margin-bottom: 20px; overflow: hidden; box-shadow: 0 2px 8px rgba(0,0,0,0.06);">`)

		// 信号头部 - 传统风格
		// 时间框架友好显示
		timeframeDisplay := signal.Timeframe
		switch signal.Timeframe {
		case "1d":
			timeframeDisplay = "日线"
		case "1w":
			timeframeDisplay = "周线"
		case "1M":
			timeframeDisplay = "月线"
		case "4h":
			timeframeDisplay = "4小时"
		case "1h":
			timeframeDisplay = "1小时"
		case "15m":
			timeframeDisplay = "15分钟"
		case "5m":
			timeframeDisplay = "5分钟"
		case "1m":
			timeframeDisplay = "1分钟"
		}

		messageBuilder.WriteString(fmt.Sprintf(`<div style="padding: 15px; background: %s; border-bottom: 1px solid #e5e5e5;">
			<div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px;">
				<div style="display: flex; align-items: center; gap: 10px;">
					<div style="font-size: 14px; font-weight: 600; color: #666; background: rgba(0,0,0,0.05); padding: 2px 8px; border-radius: 12px; font-family: monospace;">%d</div>
					<div style="font-size: 20px; font-weight: 600; color: %s;">%s %s</div>
				</div>
				<div style="padding: 6px 12px; background: %s; color: white; border-radius: 16px; font-size: 13px; font-weight: 600;">%s %s</div>
			</div>
			<div style="font-size: 13px; color: #666; background: rgba(255,255,255,0.8); padding: 6px 10px; border-radius: 4px; display: inline-block;">
				📈 %s | 🔍 %s | ⏰ %s
			</div>
		</div>`, signalBgColor, i+1, signalColor, signalIcon, signal.Symbol, signalColor, signalText, signalEmoji, timeframeDisplay, signal.Strategy, signal.Timestamp.In(loc).Format("15:04:05")))

		// 信号内容区域 - 传统风格
		messageBuilder.WriteString(`<div style="padding: 20px; background: #ffffff;">`)

		// 指标摘要 - 传统风格突出显示
		messageBuilder.WriteString(fmt.Sprintf(`<div style="margin-bottom: 15px; padding: 15px; background: linear-gradient(135deg, rgba(74, 144, 226, 0.08) 0%%, rgba(53, 122, 189, 0.08) 100%%); border: 1px solid %s; border-radius: 6px; position: relative;">
			<div style="position: absolute; top: -8px; left: 12px; background: white; padding: 0 8px; font-size: 11px; font-weight: 600; color: %s;">核心指标</div>
			<div style="font-family: monospace; font-size: 14px; color: %s; font-weight: 600; text-align: center; margin-top: 3px;">%s</div>
		</div>`, signalColor, signalColor, signalColor, signal.IndicatorSummary))

		// 详细分析 - 传统风格
		if signal.DetailedAnalysis != "" {
			messageBuilder.WriteString(fmt.Sprintf(`<div style="margin-bottom: 15px;">
				<div style="font-weight: 600; color: #2c3e50; margin-bottom: 8px; display: flex; align-items: center; gap: 6px;">
					<span style="color: #4a90e2; font-size: 14px;">📋</span>
					技术分析
				</div>
				<div style="color: #555; line-height: 1.6; white-space: pre-wrap; word-wrap: break-word; overflow-wrap: break-word; background: #f8f9fa; padding: 12px; border-radius: 4px; border-left: 3px solid %s;">%s</div>
			</div>`, signalColor, signal.DetailedAnalysis))
		}

		// 关键指标值表格 - 传统风格
		if len(signal.AllIndicators) > 0 {
			messageBuilder.WriteString(`<div style="margin-bottom: 15px;">
				<div style="font-weight: 600; color: #2c3e50; margin-bottom: 8px; display: flex; align-items: center; gap: 6px;">
					<span style="color: #4a90e2; font-size: 14px;">📊</span>
					指标数值
				</div>
				<div style="background: #ffffff; border-radius: 6px; overflow: hidden; border: 1px solid #e5e5e5;">
				<table style="width: 100%; border-collapse: collapse; font-size: 13px;">`)

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

				messageBuilder.WriteString(fmt.Sprintf(`<tr style="border-bottom: 1px solid #f0f0f0;">
					<td style="padding: 10px 12px; background: #f8f9fa; font-weight: 600; color: #2c3e50; font-family: monospace;">%s</td>
					<td style="padding: 10px 12px; font-family: monospace; color: #333; font-weight: 500;">%s</td>
				</tr>`, displayKey, valueStr))
			}
			messageBuilder.WriteString(`</table></div></div>`)
		}

		// 多时间框架数据展示 - 传统风格
		if len(signal.MultiTimeframeData) > 0 {
			messageBuilder.WriteString(`<div style="margin-bottom: 15px;">
				<div style="font-weight: 600; color: #2c3e50; margin-bottom: 8px; display: flex; align-items: center; gap: 6px;">
					<span style="color: #4a90e2; font-size: 14px;">📈</span>
					多时间框架对比
				</div>
				<div style="background: #ffffff; border-radius: 6px; overflow: hidden; border: 1px solid #e5e5e5;">
				<table style="width: 100%; border-collapse: collapse; font-size: 12px;">
					<thead>
						<tr style="background: #f8f9fa;">
							<th style="padding: 10px 8px; text-align: left; font-weight: 600; color: #2c3e50; border-bottom: 2px solid #e5e5e5;">时间框架</th>
							<th style="padding: 10px 8px; text-align: left; font-weight: 600; color: #2c3e50; border-bottom: 2px solid #e5e5e5;">指标摘要</th>
							<th style="padding: 10px 8px; text-align: center; font-weight: 600; color: #2c3e50; border-bottom: 2px solid #e5e5e5;">信号状态</th>
							<th style="padding: 10px 8px; text-align: left; font-weight: 600; color: #2c3e50; border-bottom: 2px solid #e5e5e5;">详细分析</th>
						</tr>
					</thead>
					<tbody>`)

			// 按时间框架顺序排列：日线、周线、月线
			timeframeOrder := []string{"1d", "1w", "1M"}
			for _, tf := range timeframeOrder {
				if tfData, exists := signal.MultiTimeframeData[tf]; exists {
					// 信号状态指示器
					statusIndicator := "⚪ 无信号"
					statusColor := "#6c757d"
					if tfData.HasSignal {
						if tfData.SignalType == strategy.SignalBuy {
							statusIndicator = "🟢 买入"
							statusColor = "#5cb85c"
						} else if tfData.SignalType == strategy.SignalSell {
							statusIndicator = "🔴 卖出"
							statusColor = "#d9534f"
						}
					}

					// 指标摘要处理
					indicatorSummary := tfData.IndicatorSummary

					if len([]rune(indicatorSummary)) > 25 {
						runes := []rune(indicatorSummary)
						// 截断并添加省略号
						indicatorSummary = string(runes[:25]) + "..."
					}

					// 详细分析处理
					detailedAnalysis := tfData.DetailedAnalysis
					if len([]rune(detailedAnalysis)) > 40 {
						runes := []rune(detailedAnalysis)
						detailedAnalysis = string(runes[:40]) + "..."
					}

					messageBuilder.WriteString(fmt.Sprintf(`<tr style="border-bottom: 1px solid #f0f0f0;">
						<td style="padding: 8px; font-weight: 600; color: #2c3e50; font-family: monospace;">%s</td>
						<td style="padding: 8px; color: #333; font-family: monospace; font-size: 11px;">%s</td>
						<td style="padding: 8px; text-align: center;">
							<span style="color: %s; font-weight: 600; font-size: 11px;">%s</span>
						</td>
						<td style="padding: 8px; color: #666; font-size: 11px; line-height: 1.4;">%s</td>
					</tr>`, tfData.Timeframe, indicatorSummary, statusColor, statusIndicator, detailedAnalysis))
				}
			}

			messageBuilder.WriteString(`</tbody>
				</table></div>
				<div style="margin-top: 8px; padding: 8px; background: #f8f9fa; border-radius: 4px; font-size: 11px; color: #666; text-align: center;">
					💡 多时间框架分析有助于确认信号强度和趋势方向，建议综合考虑各时间维度的指标表现
				</div>
			</div>`)
		}

		// 交易建议 - 传统风格
		if signal.Message != "" {
			suggestionText := "继续关注市场指标变化"
			if signal.Signal == strategy.SignalBuy {
				suggestionText = "这可能是一个潜在的买入机会。请结合其他技术指标和市场情况进行综合分析。"
			} else if signal.Signal == strategy.SignalSell {
				suggestionText = "这可能是一个潜在的卖出机会。请结合其他技术指标和市场情况进行综合分析。"
			}

			messageBuilder.WriteString(fmt.Sprintf(`<div style="padding: 12px; background: linear-gradient(135deg, %s15, %s08); border: 1px solid %s; border-radius: 6px; margin-top: 12px; position: relative;">
				<div style="position: absolute; top: -8px; left: 12px; background: white; padding: 0 8px; font-size: 11px; font-weight: 600; color: %s;">操作建议</div>
				<div style="color: %s; font-size: 13px; line-height: 1.5; margin-top: 3px; font-weight: 500;">%s</div>
			</div>`, signalColor, signalColor, signalColor, signalColor, signalColor, suggestionText))
		}

		messageBuilder.WriteString(`</div>`) // 结束内容区域
		messageBuilder.WriteString(`</div>`) // 结束信号卡片
	}

	// 如果信号过多，显示提示
	if len(w.signals) > displayCount {
		messageBuilder.WriteString(fmt.Sprintf(`<div style="margin-top: 15px; text-align: center; padding: 15px; background-color: #fff3cd; border: 1px solid #ffeeba; border-radius: 6px; color: #856404;">
			<div style="font-size: 14px; font-weight: 600; margin-bottom: 4px;">📝 还有更多信号</div>
			<div style="font-size: 13px;">本次报告显示了前 %d 个信号，还有 %d 个信号未显示</div>
			<div style="font-size: 12px; margin-top: 8px;">完整信号详情请查看系统日志或下次报告</div>
		</div>`, displayCount, len(w.signals)-displayCount))
	}

	messageBuilder.WriteString(`</div>`) // 结束信号详情部分

	// 免责声明 - 传统风格
	messageBuilder.WriteString(`<div style="margin: 25px 0; padding: 20px; background: linear-gradient(135deg, #d9534f15, #c9302c15); border: 1px solid #d9534f; border-radius: 6px; position: relative;">
		<div style="position: absolute; top: -10px; left: 15px; background: white; padding: 4px 12px; font-size: 12px; font-weight: 600; color: #d9534f;">⚠️ 免责声明</div>
		<h4 style="margin: 12px 0 12px 0; color: #d63031; font-size: 16px;">📜 重要声明</h4>
		<div style="color: #666; line-height: 1.6; font-size: 14px;">
			<p style="margin: 0 0 10px 0;">• 所有交易信号不构成投资建议或推荐</p>
			<p style="margin: 0 0 10px 0;">• 加密货币投资具有高风险，可能损失全部本金</p>
			<p style="margin: 0;">• 请根据自身风险承受能力做出决策，并进行独立研究</p>
		</div>
	</div>`)

	// 页脚信息 - 传统风格简化版
	messageBuilder.WriteString(`<div style="margin-top: 30px; padding: 20px; background: linear-gradient(135deg, #4a90e2 0%, #357abd 100%); border-radius: 6px; text-align: center; color: white;">
		<div style="font-size: 15px; font-weight: 600; margin-bottom: 6px;">
			🤖 由 <strong>TA Watcher v1.0</strong> 提供技术支持
		</div>
		<div style="font-size: 12px; opacity: 0.9; margin-bottom: 1px;">
			报告生成时间：` + now.Format("2006-01-02 15:04:05") + ` (UTC+8)
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
		w.sendReport("单次检查发现交易信号")
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

// collectMultiTimeframeData 收集指定交易对在所有时间框架的数据
func (w *Watcher) collectMultiTimeframeData(symbol string, signalTimeframe string) map[string]TimeframeData {
	multiData := make(map[string]TimeframeData)

	// 定义要检查的时间框架
	timeframes := []datasource.Timeframe{datasource.Timeframe1d, datasource.Timeframe1w, datasource.Timeframe1M}

	// 计算所有策略需要的最大数据点数（与主逻辑保持一致）
	maxDataPoints := 50
	for _, strat := range w.strategies {
		if required := strat.RequiredDataPoints(); required > maxDataPoints {
			maxDataPoints = required
		}
	}

	// 判断是否为交叉汇率对
	log.Printf("🔍 开始判断 %s 是否为交叉汇率对...", symbol)
	isCrossRatePair := w.isCrossRatePair(symbol)
	log.Printf("📊 %s 判断结果: 交叉汇率对=%t", symbol, isCrossRatePair)

	for _, tf := range timeframes {
		tfStr := string(tf)

		// 时间框架显示名称
		timeframeDisplay := tfStr
		switch tfStr {
		case "1d":
			timeframeDisplay = "日线"
		case "1w":
			timeframeDisplay = "周线"
		case "1M":
			timeframeDisplay = "月线"
		case "4h":
			timeframeDisplay = "4小时"
		case "1h":
			timeframeDisplay = "1小时"
		}

		// 尝试获取数据并分析（使用与主逻辑相同的方式）
		ctx := context.Background()
		endTime := time.Now()

		// 根据时间框架计算正确的开始时间（与主逻辑保持一致）
		var duration time.Duration
		switch tf {
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

		// 获取K线数据
		var klines []*datasource.Kline
		var err error

		if isCrossRatePair {
			// 交叉汇率对，使用assets包的CalculateRate方法
			klines, err = w.getCrossRateKlines(ctx, symbol, tf, startTime, endTime, maxDataPoints*2)
		} else {
			// 普通交易对，直接获取K线数据
			klines, err = w.dataSource.GetKlines(ctx, symbol, tf, startTime, endTime, maxDataPoints*2)
		}

		if err != nil {
			// 数据获取失败，记录为无数据
			multiData[tfStr] = TimeframeData{
				Timeframe:        timeframeDisplay,
				Indicators:       make(map[string]interface{}),
				IndicatorSummary: "数据获取失败",
				DetailedAnalysis: fmt.Sprintf("无法获取K线数据: %v", err),
				HasSignal:        false,
				SignalType:       strategy.SignalNone,
			}
			continue
		}

		// 使用与主逻辑相同的数据充足性检查
		if len(klines) < maxDataPoints {
			// 数据不足，记录详细信息
			multiData[tfStr] = TimeframeData{
				Timeframe:        timeframeDisplay,
				Indicators:       make(map[string]interface{}),
				IndicatorSummary: fmt.Sprintf("数据不足 (%d/%d)", len(klines), maxDataPoints),
				DetailedAnalysis: "K线数据点数不足以进行分析",
				HasSignal:        false,
				SignalType:       strategy.SignalNone,
			}
			continue
		}

		// 准备市场数据
		marketData := &strategy.MarketData{
			Symbol:    symbol,
			Timeframe: tf,
			Klines:    klines,
			Timestamp: time.Now(),
		}

		// 分析所有策略
		var indicators map[string]interface{}
		var indicatorSummary string
		var detailedAnalysis string
		hasSignal := false
		signalType := strategy.SignalNone

		for _, strat := range w.strategies {
			result, err := strat.Evaluate(marketData)
			if err != nil {
				continue
			}

			if result != nil {
				indicators = result.Indicators
				indicatorSummary = result.IndicatorSummary
				detailedAnalysis = result.DetailedAnalysis

				// 检查是否有信号
				if result.ShouldNotify() {
					hasSignal = true
					signalType = result.Signal
				}

				// 通常只有一个策略，所以可以break
				break
			}
		}

		if indicators == nil {
			indicators = make(map[string]interface{})
		}
		if indicatorSummary == "" {
			indicatorSummary = "正常范围"
		}
		if detailedAnalysis == "" {
			detailedAnalysis = "指标在正常范围内"
		}

		multiData[tfStr] = TimeframeData{
			Timeframe:        timeframeDisplay,
			Indicators:       indicators,
			IndicatorSummary: indicatorSummary,
			DetailedAnalysis: detailedAnalysis,
			HasSignal:        hasSignal,
			SignalType:       signalType,
		}
	}

	return multiData
}

// isCrossRatePair 判断是否为交叉汇率对
func (w *Watcher) isCrossRatePair(symbol string) bool {
	log.Printf("🔍 [%s] 开始判断是否为交叉汇率对", symbol)

	// 首先检查是否包含常见稳定币后缀，如果是则不是交叉汇率对
	commonQuotes := []string{"USDT", "USD", "BUSD", "USDC", "DAI", "TUSD"}
	for _, quote := range commonQuotes {
		if strings.HasSuffix(symbol, quote) {
			log.Printf("✅ [%s] 包含稳定币后缀 %s，判定为直接交易对", symbol, quote)
			return false
		}
	}

	// 对于其他交易对，尝试直接获取少量数据来判断是否为真实交易对
	log.Printf("🔍 [%s] 不包含稳定币后缀，尝试获取数据验证", symbol)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // 增加到30秒
	defer cancel()

	// 尝试获取最近1小时的1个数据点来验证交易对是否存在
	endTime := time.Now()
	startTime := endTime.Add(-time.Hour)

	_, err := w.dataSource.GetKlines(ctx, symbol, datasource.Timeframe1h, startTime, endTime, 1)
	if err != nil {
		// 如果直接获取失败，则认为是交叉汇率对，需要通过计算获得
		log.Printf("🔍 [%s] 直接获取失败，判定为交叉汇率对: %v", symbol, err)
		return true
	}

	// 如果能直接获取数据，则是直接交易对，不需要计算
	log.Printf("✅ [%s] 直接获取成功，判定为直接交易对", symbol)
	return false
}

// getCrossRateKlines 获取交叉汇率对的K线数据
func (w *Watcher) getCrossRateKlines(ctx context.Context, symbol string, timeframe datasource.Timeframe, startTime, endTime time.Time, limit int) ([]*datasource.Kline, error) {
	// 解析交叉汇率对的基础货币和报价货币
	baseSymbol, quoteSymbol, err := w.parseCrossRatePair(symbol)
	if err != nil {
		return nil, fmt.Errorf("解析交叉汇率对失败: %w", err)
	}

	// 使用USDT作为桥接货币
	bridgeCurrency := "USDT"

	// 调用assets包的CalculateRate方法
	return w.rateCalculator.CalculateRate(ctx, baseSymbol, quoteSymbol, bridgeCurrency, timeframe, startTime, endTime, limit)
}

// parseCrossRatePair 解析交叉汇率对，返回基础货币和报价货币
func (w *Watcher) parseCrossRatePair(symbol string) (baseSymbol, quoteSymbol string, err error) {
	// 常见的加密货币符号，按市值排序（作为可能的分割点）
	knownSymbols := []string{"BTC", "ETH", "BNB", "ADA", "SOL", "DOT", "MATIC", "AVAX", "LINK", "UNI"}

	// 尝试从后往前匹配已知符号作为报价货币
	for _, quote := range knownSymbols {
		if strings.HasSuffix(symbol, quote) && len(symbol) > len(quote) {
			baseSymbol = symbol[:len(symbol)-len(quote)]
			quoteSymbol = quote

			// 验证基础货币也是已知符号
			for _, base := range knownSymbols {
				if baseSymbol == base {
					return baseSymbol, quoteSymbol, nil
				}
			}
		}
	}

	// 如果无法解析，尝试常见的3-3或4-3分割
	if len(symbol) == 6 {
		// 3-3分割，如ETHBTC
		return symbol[:3], symbol[3:], nil
	} else if len(symbol) == 7 {
		// 可能是4-3分割，如LINKBTC
		return symbol[:4], symbol[4:], nil
	}

	return "", "", fmt.Errorf("无法解析交叉汇率对: %s", symbol)
}
