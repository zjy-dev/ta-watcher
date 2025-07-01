// Package strategy provides trading strategy evaluation and signal generation
package strategy

import (
	"time"

	"ta-watcher/internal/datasource"
	"ta-watcher/internal/indicators"
)

// Signal 策略信号类型
type Signal int

const (
	SignalNone Signal = iota // 无信号
	SignalBuy                // 买入信号
	SignalSell               // 卖出信号
	SignalHold               // 持有信号
)

// String 返回信号的字符串表示
func (s Signal) String() string {
	switch s {
	case SignalBuy:
		return "BUY"
	case SignalSell:
		return "SELL"
	case SignalHold:
		return "HOLD"
	default:
		return "NONE"
	}
}

// Strength 信号强度
type Strength int

const (
	StrengthWeak   Strength = iota // 弱信号
	StrengthNormal                 // 普通信号
	StrengthStrong                 // 强信号
)

// String 返回强度的字符串表示
func (s Strength) String() string {
	switch s {
	case StrengthWeak:
		return "WEAK"
	case StrengthStrong:
		return "STRONG"
	default:
		return "NORMAL"
	}
}

// MarketData 市场数据
type MarketData struct {
	Symbol    string               // 交易对
	Timeframe datasource.Timeframe // 时间框架
	Klines    []*datasource.Kline  // K线数据
	Timestamp time.Time            // 数据时间戳
}

// StrategyResult 策略评估结果
type StrategyResult struct {
	Signal           Signal                 // 信号类型
	Strength         Strength               // 信号强度
	Timestamp        time.Time              // 信号时间
	Message          string                 // 信号描述消息
	IndicatorSummary string                 // 指标摘要描述（包含指标名称、阈值、当前值）
	DetailedAnalysis string                 // 详细分析描述
	Indicators       map[string]interface{} // 指标原始值
	Thresholds       map[string]interface{} // 策略阈值
	Metadata         map[string]interface{} // 额外元数据
}

// ShouldNotify 判断是否应该发送通知
func (r *StrategyResult) ShouldNotify() bool {
	// 只有明确的买入或卖出信号才发送通知
	return r.Signal == SignalBuy || r.Signal == SignalSell
}

// GetNotificationLevel 获取通知级别
func (r *StrategyResult) GetNotificationLevel() string {
	if r.Signal == SignalNone || r.Signal == SignalHold {
		return "info"
	}

	switch r.Strength {
	case StrengthStrong:
		return "critical"
	case StrengthNormal:
		return "warning"
	default:
		return "info"
	}
}

// Strategy 策略接口
type Strategy interface {
	// Name 返回策略名称
	Name() string

	// Description 返回策略描述
	Description() string

	// Evaluate 评估策略，返回信号
	Evaluate(data *MarketData) (*StrategyResult, error)

	// RequiredDataPoints 返回所需的最少数据点数
	RequiredDataPoints() int

	// SupportedTimeframes 返回支持的时间框架
	SupportedTimeframes() []datasource.Timeframe
}

// CompositeStrategy 复合策略接口 - 简化版本，专为通知系统设计
type CompositeStrategy interface {
	Strategy

	// AddSubStrategy 添加子策略
	AddSubStrategy(strategy Strategy)

	// RemoveSubStrategy 移除子策略
	RemoveSubStrategy(name string)

	// GetSubStrategies 获取所有子策略
	GetSubStrategies() map[string]Strategy
}

// IndicatorContext 指标上下文，提供计算指标的便捷方法
type IndicatorContext struct {
	data *MarketData
}

// NewIndicatorContext 创建指标上下文
func NewIndicatorContext(data *MarketData) *IndicatorContext {
	return &IndicatorContext{data: data}
}

// ClosePrices 获取收盘价序列
func (ctx *IndicatorContext) ClosePrices() []float64 {
	prices := make([]float64, len(ctx.data.Klines))
	for i, kline := range ctx.data.Klines {
		prices[i] = kline.Close
	}
	return prices
}

// HighPrices 获取最高价序列
func (ctx *IndicatorContext) HighPrices() []float64 {
	prices := make([]float64, len(ctx.data.Klines))
	for i, kline := range ctx.data.Klines {
		prices[i] = kline.High
	}
	return prices
}

// LowPrices 获取最低价序列
func (ctx *IndicatorContext) LowPrices() []float64 {
	prices := make([]float64, len(ctx.data.Klines))
	for i, kline := range ctx.data.Klines {
		prices[i] = kline.Low
	}
	return prices
}

// Volumes 获取成交量序列
func (ctx *IndicatorContext) Volumes() []float64 {
	volumes := make([]float64, len(ctx.data.Klines))
	for i, kline := range ctx.data.Klines {
		volumes[i] = kline.Volume
	}
	return volumes
}

// SMA 计算简单移动平均线
func (ctx *IndicatorContext) SMA(period int) (*indicators.MAResult, error) {
	return indicators.CalculateSMA(ctx.ClosePrices(), period)
}

// EMA 计算指数移动平均线
func (ctx *IndicatorContext) EMA(period int) (*indicators.MAResult, error) {
	return indicators.CalculateEMA(ctx.ClosePrices(), period)
}

// RSI 计算RSI指标
func (ctx *IndicatorContext) RSI(period int) (*indicators.RSIResult, error) {
	return indicators.CalculateRSI(ctx.ClosePrices(), period)
}

// MACD 计算MACD指标
func (ctx *IndicatorContext) MACD(fastPeriod, slowPeriod, signalPeriod int) (*indicators.MACDResult, error) {
	return indicators.CalculateMACD(ctx.ClosePrices(), fastPeriod, slowPeriod, signalPeriod)
}

// LatestPrice 获取最新价格
func (ctx *IndicatorContext) LatestPrice() float64 {
	if len(ctx.data.Klines) == 0 {
		return 0
	}
	return ctx.data.Klines[len(ctx.data.Klines)-1].Close
}

// PriceChange 计算价格变化百分比（相对于指定周期前）
func (ctx *IndicatorContext) PriceChange(periods int) float64 {
	if len(ctx.data.Klines) < periods+1 {
		return 0
	}

	current := ctx.data.Klines[len(ctx.data.Klines)-1].Close
	previous := ctx.data.Klines[len(ctx.data.Klines)-1-periods].Close

	if previous == 0 {
		return 0
	}

	return (current - previous) / previous * 100
}
