// Package strategy provides the high-level interface for watcher integration
package strategy

import (
	"context"
	"fmt"
	"time"

	"ta-watcher/internal/binance"
	"ta-watcher/internal/notifiers"
)

// WatcherIntegration 为 Watcher 提供的高级策略接口
type WatcherIntegration struct {
	manager *Manager
	factory *Factory
	client  binance.DataSource
	config  *WatcherConfig
}

// WatcherConfig Watcher 集成配置
type WatcherConfig struct {
	// 默认策略名称
	DefaultStrategy string

	// 数据获取配置
	DataLimit       int           // 获取的K线数量
	RefreshInterval time.Duration // 数据刷新间隔

	// 通知配置
	EnableNotifications  bool
	NotificationCooldown time.Duration // 同一策略通知冷却时间

	// 风险管理
	MaxPositions      int     // 最大持仓数量
	RiskLevel         float64 // 风险水平 (0.0-1.0)
	StopLossPercent   float64 // 止损百分比
	TakeProfitPercent float64 // 止盈百分比
}

// DefaultWatcherConfig 默认 Watcher 配置
func DefaultWatcherConfig() *WatcherConfig {
	return &WatcherConfig{
		DefaultStrategy:      "balanced_combo",
		DataLimit:            100,
		RefreshInterval:      5 * time.Minute,
		EnableNotifications:  true,
		NotificationCooldown: 15 * time.Minute,
		MaxPositions:         5,
		RiskLevel:            0.5,
		StopLossPercent:      5.0,
		TakeProfitPercent:    10.0,
	}
}

// NewWatcherIntegration 创建 Watcher 集成实例
func NewWatcherIntegration(client binance.DataSource, config *WatcherConfig) *WatcherIntegration {
	if config == nil {
		config = DefaultWatcherConfig()
	}

	return &WatcherIntegration{
		manager: NewManager(DefaultManagerConfig()),
		factory: NewFactory(),
		client:  client,
		config:  config,
	}
}

// DecisionRequest 决策请求
type DecisionRequest struct {
	Symbol       string    // 交易对
	Timeframe    Timeframe // 时间框架
	StrategyName string    // 策略名称（可选，使用默认策略如果为空）
	Context      context.Context
}

// DecisionResult 决策结果
type DecisionResult struct {
	// 基本信息
	Symbol    string    `json:"symbol"`
	Timeframe Timeframe `json:"timeframe"`
	Timestamp time.Time `json:"timestamp"`

	// 策略信息
	StrategyName string `json:"strategy_name"`
	StrategyDesc string `json:"strategy_description"`

	// 信号信息
	Signal     Signal   `json:"signal"`
	Strength   Strength `json:"strength"`
	Confidence float64  `json:"confidence"`
	Price      float64  `json:"price"`
	Message    string   `json:"message"`

	// 通知决策
	ShouldNotify      bool   `json:"should_notify"`
	NotificationLevel string `json:"notification_level"`

	// 风险管理
	RiskAssessment *RiskAssessment `json:"risk_assessment,omitempty"`

	// 技术指标
	Indicators map[string]interface{} `json:"indicators"`

	// 元数据
	Metadata map[string]interface{} `json:"metadata"`

	// 执行时间
	ExecutionTime time.Duration `json:"execution_time"`
}

// RiskAssessment 风险评估
type RiskAssessment struct {
	RiskLevel       string   `json:"risk_level"`        // HIGH, MEDIUM, LOW
	RiskScore       float64  `json:"risk_score"`        // 0.0-1.0
	MaxPosition     float64  `json:"max_position"`      // 建议最大仓位百分比
	StopLossPrice   float64  `json:"stop_loss_price"`   // 建议止损价
	TakeProfitPrice float64  `json:"take_profit_price"` // 建议止盈价
	Warnings        []string `json:"warnings"`          // 风险警告
}

// MakeDecision 做出交易决策
func (w *WatcherIntegration) MakeDecision(req *DecisionRequest) (*DecisionResult, error) {
	startTime := time.Now()

	if req.Context == nil {
		req.Context = context.Background()
	}

	// 1. 获取市场数据
	marketData, err := w.fetchMarketData(req.Symbol, req.Timeframe)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch market data: %w", err)
	}

	// 2. 确定使用的策略
	strategyName := req.StrategyName
	if strategyName == "" {
		strategyName = w.config.DefaultStrategy
	}

	strategy, err := w.getOrCreateStrategy(strategyName, req.Timeframe)
	if err != nil {
		return nil, fmt.Errorf("failed to get strategy: %w", err)
	}

	// 3. 评估策略
	strategyResult, err := strategy.Evaluate(marketData)
	if err != nil {
		return nil, fmt.Errorf("strategy evaluation failed: %w", err)
	}

	// 4. 风险评估
	riskAssessment := w.assessRisk(strategyResult, marketData)

	// 5. 构建决策结果
	result := &DecisionResult{
		Symbol:            req.Symbol,
		Timeframe:         req.Timeframe,
		Timestamp:         time.Now(),
		StrategyName:      strategy.Name(),
		StrategyDesc:      strategy.Description(),
		Signal:            strategyResult.Signal,
		Strength:          strategyResult.Strength,
		Confidence:        strategyResult.Confidence,
		Price:             strategyResult.Price,
		Message:           strategyResult.Message,
		ShouldNotify:      w.shouldNotify(strategyResult, riskAssessment),
		NotificationLevel: w.getNotificationLevel(strategyResult, riskAssessment),
		RiskAssessment:    riskAssessment,
		Indicators:        strategyResult.Indicators,
		Metadata:          w.enrichMetadata(strategyResult.Metadata, marketData),
		ExecutionTime:     time.Since(startTime),
	}

	return result, nil
}

// MakeDecisionForMultipleSymbols 为多个交易对做决策
func (w *WatcherIntegration) MakeDecisionForMultipleSymbols(symbols []string, timeframe Timeframe, strategyName string) ([]*DecisionResult, error) {
	results := make([]*DecisionResult, 0, len(symbols))

	for _, symbol := range symbols {
		req := &DecisionRequest{
			Symbol:       symbol,
			Timeframe:    timeframe,
			StrategyName: strategyName,
			Context:      context.Background(),
		}

		result, err := w.MakeDecision(req)
		if err != nil {
			// 记录错误但继续处理其他交易对
			result = &DecisionResult{
				Symbol:    symbol,
				Timeframe: timeframe,
				Timestamp: time.Now(),
				Signal:    SignalNone,
				Message:   fmt.Sprintf("Error: %v", err),
			}
		}

		results = append(results, result)
	}

	return results, nil
}

// GetRecommendedTimeframes 获取推荐的时间框架
func (w *WatcherIntegration) GetRecommendedTimeframes(tradingStyle string) []Timeframe {
	switch tradingStyle {
	case "scalping":
		return []Timeframe{Timeframe1m, Timeframe3m, Timeframe5m}
	case "day_trading":
		return []Timeframe{Timeframe5m, Timeframe15m, Timeframe30m, Timeframe1h}
	case "swing_trading":
		return []Timeframe{Timeframe1h, Timeframe4h, Timeframe1d}
	case "long_term":
		return []Timeframe{Timeframe1d, Timeframe3d, Timeframe1w}
	default:
		return []Timeframe{Timeframe15m, Timeframe1h, Timeframe4h, Timeframe1d}
	}
}

// ConvertToNotification 将决策结果转换为通知
func (w *WatcherIntegration) ConvertToNotification(result *DecisionResult) *notifiers.Notification {
	if !result.ShouldNotify {
		return nil
	}

	// 确定通知类型
	var notificationType notifiers.NotificationType
	switch result.Signal {
	case SignalBuy:
		notificationType = notifiers.TypeStrategySignal
	case SignalSell:
		notificationType = notifiers.TypeStrategySignal
	default:
		notificationType = notifiers.TypePriceAlert
	}

	// 确定通知级别
	var level notifiers.NotificationLevel
	switch result.NotificationLevel {
	case "critical":
		level = notifiers.LevelCritical
	case "warning":
		level = notifiers.LevelWarning
	default:
		level = notifiers.LevelInfo
	}

	notification := &notifiers.Notification{
		ID:        fmt.Sprintf("strategy_%s_%s_%d", result.Symbol, result.StrategyName, result.Timestamp.Unix()),
		Type:      notificationType,
		Level:     level,
		Title:     fmt.Sprintf("%s %s信号", result.Symbol, result.Signal.String()),
		Message:   result.Message,
		Timestamp: result.Timestamp,
		Data: map[string]interface{}{
			"symbol":     result.Symbol,
			"timeframe":  result.Timeframe,
			"strategy":   result.StrategyName,
			"signal":     result.Signal.String(),
			"strength":   result.Strength.String(),
			"confidence": result.Confidence,
			"price":      result.Price,
			"indicators": result.Indicators,
			"risk_level": result.RiskAssessment.RiskLevel,
		},
	}

	return notification
}

// 内部方法

// fetchMarketData 获取市场数据
func (w *WatcherIntegration) fetchMarketData(symbol string, timeframe Timeframe) (*MarketData, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	klines, err := w.client.GetKlines(ctx, symbol, string(timeframe), w.config.DataLimit)
	if err != nil {
		return nil, err
	}

	return &MarketData{
		Symbol:    symbol,
		Timeframe: timeframe,
		Klines:    convertKlinePointers(klines),
		Timestamp: time.Now(),
	}, nil
}

// convertKlinePointers 转换指针数组为值数组
func convertKlinePointers(klines []*binance.KlineData) []binance.KlineData {
	result := make([]binance.KlineData, len(klines))
	for i, kline := range klines {
		if kline != nil {
			result[i] = *kline
		}
	}
	return result
}

// getOrCreateStrategy 获取或创建策略
func (w *WatcherIntegration) getOrCreateStrategy(name string, timeframe Timeframe) (Strategy, error) {
	// 首先尝试从管理器获取已注册的策略
	if strategy, err := w.manager.GetStrategy(name); err == nil {
		return strategy, nil
	}

	// 尝试从工厂创建策略
	if name == "recommended" {
		strategy, err := w.factory.CreateRecommendedStrategy(timeframe)
		if err != nil {
			return nil, err
		}

		// 注册到管理器以供后续使用
		w.manager.RegisterStrategy(strategy)
		return strategy, nil
	}

	strategy, err := w.factory.CreateStrategy(name)
	if err != nil {
		return nil, err
	}

	// 注册到管理器
	w.manager.RegisterStrategy(strategy)
	return strategy, nil
}

// assessRisk 评估风险
func (w *WatcherIntegration) assessRisk(result *StrategyResult, data *MarketData) *RiskAssessment {
	ctx := NewIndicatorContext(data)
	currentPrice := result.Price

	// 计算价格波动率
	priceChange := ctx.PriceChange(20) // 20周期价格变化
	volatility := absFloat64(priceChange)

	// 基础风险评分
	var riskScore float64
	if volatility > 10 {
		riskScore = 0.8 // 高风险
	} else if volatility > 5 {
		riskScore = 0.5 // 中风险
	} else {
		riskScore = 0.2 // 低风险
	}

	// 考虑信号置信度
	if result.Confidence < 0.5 {
		riskScore += 0.2
	}

	// 限制在 0-1 范围
	if riskScore > 1.0 {
		riskScore = 1.0
	}

	// 风险等级
	var riskLevel string
	if riskScore > 0.7 {
		riskLevel = "HIGH"
	} else if riskScore > 0.4 {
		riskLevel = "MEDIUM"
	} else {
		riskLevel = "LOW"
	}

	// 计算建议仓位
	maxPosition := (1.0 - riskScore) * w.config.RiskLevel * 100 // 百分比

	// 计算止损止盈价格
	var stopLossPrice, takeProfitPrice float64
	if result.Signal == SignalBuy {
		stopLossPrice = currentPrice * (1 - w.config.StopLossPercent/100)
		takeProfitPrice = currentPrice * (1 + w.config.TakeProfitPercent/100)
	} else if result.Signal == SignalSell {
		stopLossPrice = currentPrice * (1 + w.config.StopLossPercent/100)
		takeProfitPrice = currentPrice * (1 - w.config.TakeProfitPercent/100)
	}

	// 生成警告
	var warnings []string
	if volatility > 15 {
		warnings = append(warnings, "价格波动极大，谨慎操作")
	}
	if result.Confidence < 0.3 {
		warnings = append(warnings, "信号置信度较低")
	}
	if riskScore > 0.8 {
		warnings = append(warnings, "高风险操作，建议减少仓位")
	}

	return &RiskAssessment{
		RiskLevel:       riskLevel,
		RiskScore:       riskScore,
		MaxPosition:     maxPosition,
		StopLossPrice:   stopLossPrice,
		TakeProfitPrice: takeProfitPrice,
		Warnings:        warnings,
	}
}

// shouldNotify 判断是否应该发送通知
func (w *WatcherIntegration) shouldNotify(result *StrategyResult, risk *RiskAssessment) bool {
	if !w.config.EnableNotifications {
		return false
	}

	// 基础通知条件
	if !result.ShouldNotify() {
		return false
	}

	// 高风险时降低通知门槛
	if risk.RiskLevel == "HIGH" && result.Confidence > 0.3 {
		return true
	}

	// 强信号时增加通知可能性
	if result.Strength == StrengthStrong && result.Confidence > 0.6 {
		return true
	}

	// 一般情况需要较高置信度
	return result.Confidence > 0.7
}

// getNotificationLevel 获取通知级别
func (w *WatcherIntegration) getNotificationLevel(result *StrategyResult, risk *RiskAssessment) string {
	baseLevel := result.GetNotificationLevel()

	// 高风险时提升通知级别
	if risk.RiskLevel == "HIGH" {
		if baseLevel == "info" {
			return "warning"
		} else if baseLevel == "warning" {
			return "critical"
		}
	}

	return baseLevel
}

// enrichMetadata 丰富元数据
func (w *WatcherIntegration) enrichMetadata(original map[string]interface{}, data *MarketData) map[string]interface{} {
	if original == nil {
		original = make(map[string]interface{})
	}

	// 添加市场信息
	original["market_data_count"] = len(data.Klines)
	original["data_timestamp"] = data.Timestamp
	original["watcher_config"] = map[string]interface{}{
		"risk_level":          w.config.RiskLevel,
		"stop_loss_percent":   w.config.StopLossPercent,
		"take_profit_percent": w.config.TakeProfitPercent,
	}

	return original
}
