package strategy

import (
	"fmt"
	"time"

	"ta-watcher/internal/datasource"
	"ta-watcher/internal/indicators"
)

// MACrossStrategy 移动平均线交叉策略
type MACrossStrategy struct {
	name                string
	fastPeriod          int
	slowPeriod          int
	maType              indicators.MovingAverageType
	supportedTimeframes []datasource.Timeframe
}

// NewMACrossStrategy 创建移动平均线交叉策略
func NewMACrossStrategy(fastPeriod, slowPeriod int, maType indicators.MovingAverageType) *MACrossStrategy {
	if fastPeriod >= slowPeriod {
		// 确保快线周期小于慢线周期
		fastPeriod, slowPeriod = slowPeriod/2, fastPeriod
	}

	if fastPeriod <= 0 {
		fastPeriod = 5
	}
	if slowPeriod <= 0 {
		slowPeriod = 20
	}

	var typeName string
	switch maType {
	case indicators.EMA:
		typeName = "EMA"
	case indicators.WMA:
		typeName = "WMA"
	default:
		typeName = "SMA"
		maType = indicators.SMA
	}

	return &MACrossStrategy{
		name:       fmt.Sprintf("%s_Cross_%d_%d", typeName, fastPeriod, slowPeriod),
		fastPeriod: fastPeriod,
		slowPeriod: slowPeriod,
		maType:     maType,
		supportedTimeframes: []datasource.Timeframe{
			datasource.Timeframe5m, datasource.Timeframe15m, datasource.Timeframe30m,
			datasource.Timeframe1h, datasource.Timeframe2h, datasource.Timeframe4h, datasource.Timeframe6h, datasource.Timeframe12h,
			datasource.Timeframe1d, datasource.Timeframe3d, datasource.Timeframe1w, datasource.Timeframe1M,
		},
	}
}

// Name 返回策略名称
func (s *MACrossStrategy) Name() string {
	return s.name
}

// Description 返回策略描述
func (s *MACrossStrategy) Description() string {
	var typeName string
	switch s.maType {
	case indicators.EMA:
		typeName = "指数移动平均线"
	case indicators.WMA:
		typeName = "加权移动平均线"
	default:
		typeName = "简单移动平均线"
	}

	return fmt.Sprintf("%s交叉策略 (快线:%d, 慢线:%d)", typeName, s.fastPeriod, s.slowPeriod)
}

// RequiredDataPoints 返回所需数据点
func (s *MACrossStrategy) RequiredDataPoints() int {
	return s.slowPeriod + 2 // 需要额外数据点来检测交叉
}

// SupportedTimeframes 返回支持的时间框架
func (s *MACrossStrategy) SupportedTimeframes() []datasource.Timeframe {
	return s.supportedTimeframes
}

// Evaluate 评估策略
func (s *MACrossStrategy) Evaluate(data *MarketData) (*StrategyResult, error) {
	ctx := NewIndicatorContext(data)

	// 计算快线和慢线
	var fastMA, slowMA *indicators.MAResult
	var err error

	switch s.maType {
	case indicators.EMA:
		fastMA, err = ctx.EMA(s.fastPeriod)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate fast EMA: %w", err)
		}
		slowMA, err = ctx.EMA(s.slowPeriod)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate slow EMA: %w", err)
		}
	case indicators.WMA:
		fastMA, err = indicators.CalculateWMA(ctx.ClosePrices(), s.fastPeriod)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate fast WMA: %w", err)
		}
		slowMA, err = indicators.CalculateWMA(ctx.ClosePrices(), s.slowPeriod)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate slow WMA: %w", err)
		}
	default:
		fastMA, err = ctx.SMA(s.fastPeriod)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate fast SMA: %w", err)
		}
		slowMA, err = ctx.SMA(s.slowPeriod)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate slow SMA: %w", err)
		}
	}

	// 检查数据长度
	minLen := minInt(len(fastMA.Values), len(slowMA.Values))
	if minLen < 2 {
		return nil, fmt.Errorf("insufficient MA data points")
	}

	// 获取最新和前一个值
	fastCurrent := fastMA.Values[len(fastMA.Values)-1]
	fastPrevious := fastMA.Values[len(fastMA.Values)-2]
	slowCurrent := slowMA.Values[len(slowMA.Values)-1]
	slowPrevious := slowMA.Values[len(slowMA.Values)-2]

	currentPrice := ctx.LatestPrice()

	// 初始化结果
	result := &StrategyResult{
		Signal:     SignalNone,
		Strength:   StrengthNormal,
		Confidence: 0.0,
		Price:      currentPrice,
		Timestamp:  time.Now(),
		Metadata:   make(map[string]interface{}),
		Indicators: make(map[string]interface{}),
	}

	// 设置指标值
	result.Indicators["fast_ma"] = fastCurrent
	result.Indicators["slow_ma"] = slowCurrent
	result.Indicators["fast_period"] = s.fastPeriod
	result.Indicators["slow_period"] = s.slowPeriod
	result.Indicators["ma_type"] = s.maType

	// 计算差值和差值变化
	currentDiff := fastCurrent - slowCurrent
	previousDiff := fastPrevious - slowPrevious
	diffChange := currentDiff - previousDiff

	result.Metadata["ma_diff"] = currentDiff
	result.Metadata["ma_diff_change"] = diffChange

	// 检测交叉
	var signal Signal
	var message string
	confidence := 0.0

	if previousDiff <= 0 && currentDiff > 0 {
		// 黄金交叉：快线上穿慢线，买入信号
		signal = SignalBuy
		message = fmt.Sprintf("黄金交叉: 快线(%.2f)上穿慢线(%.2f)", fastCurrent, slowCurrent)
		confidence = calculateCrossConfidence(currentDiff, slowCurrent, true)

	} else if previousDiff >= 0 && currentDiff < 0 {
		// 死亡交叉：快线下穿慢线，卖出信号
		signal = SignalSell
		message = fmt.Sprintf("死亡交叉: 快线(%.2f)下穿慢线(%.2f)", fastCurrent, slowCurrent)
		confidence = calculateCrossConfidence(currentDiff, slowCurrent, false)

	} else if currentDiff > 0 {
		// 快线在慢线上方，持有
		signal = SignalHold
		message = fmt.Sprintf("多头趋势: 快线(%.2f)高于慢线(%.2f)", fastCurrent, slowCurrent)
		confidence = 0.3

	} else if currentDiff < 0 {
		// 快线在慢线下方，空头
		signal = SignalHold
		message = fmt.Sprintf("空头趋势: 快线(%.2f)低于慢线(%.2f)", fastCurrent, slowCurrent)
		confidence = 0.3

	} else {
		// 平行状态
		signal = SignalNone
		message = fmt.Sprintf("平行状态: 快线(%.2f)与慢线(%.2f)接近", fastCurrent, slowCurrent)
		confidence = 0.0
	}

	result.Signal = signal
	result.Message = message
	result.Confidence = confidence

	// 计算强度
	if signal == SignalBuy || signal == SignalSell {
		diffPercent := absFloat64(currentDiff) / slowCurrent * 100
		if diffPercent >= 2.0 {
			result.Strength = StrengthStrong
		} else if diffPercent >= 1.0 {
			result.Strength = StrengthNormal
		} else {
			result.Strength = StrengthWeak
		}

		// 考虑价格趋势一致性
		priceChange := ctx.PriceChange(s.fastPeriod)
		if (signal == SignalBuy && priceChange > 0) || (signal == SignalSell && priceChange < 0) {
			result.Confidence = minFloat64(result.Confidence*1.2, 1.0)
		}
	}

	return result, nil
}

// calculateCrossConfidence 计算交叉置信度
func calculateCrossConfidence(diff, slowMA float64, isBullish bool) float64 {
	// 基于差值相对于慢线的百分比计算置信度
	diffPercent := absFloat64(diff) / slowMA * 100

	// 差值越大，置信度越高
	confidence := minFloat64(diffPercent/5.0, 1.0) // 5%差值对应满置信度

	// 最小置信度
	if confidence < 0.5 {
		confidence = 0.5
	}

	return confidence
}

// absFloat64 返回绝对值
func absFloat64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// minInt 返回两个整数中较小的
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
