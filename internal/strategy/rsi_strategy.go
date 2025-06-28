package strategy

import (
	"fmt"
	"time"

	"ta-watcher/internal/datasource"
)

// RSIStrategy RSI策略
type RSIStrategy struct {
	name                string
	period              int
	overboughtLevel     float64
	oversoldLevel       float64
	supportedTimeframes []datasource.Timeframe
}

// NewRSIStrategy 创建RSI策略
func NewRSIStrategy(period int, overboughtLevel, oversoldLevel float64) *RSIStrategy {
	if period <= 0 {
		period = 14 // 默认周期
	}
	if overboughtLevel <= 0 {
		overboughtLevel = 70
	}
	if oversoldLevel <= 0 {
		oversoldLevel = 30
	}

	return &RSIStrategy{
		name:            fmt.Sprintf("RSI_%d_%.0f_%.0f", period, overboughtLevel, oversoldLevel),
		period:          period,
		overboughtLevel: overboughtLevel,
		oversoldLevel:   oversoldLevel,
		supportedTimeframes: []datasource.Timeframe{
			datasource.Timeframe5m, datasource.Timeframe15m, datasource.Timeframe30m,
			datasource.Timeframe1h, datasource.Timeframe2h, datasource.Timeframe4h,
			datasource.Timeframe6h, datasource.Timeframe12h,
			datasource.Timeframe1d, datasource.Timeframe3d, datasource.Timeframe1w, datasource.Timeframe1M,
		},
	}
}

// Name 返回策略名称
func (s *RSIStrategy) Name() string {
	return s.name
}

// Description 返回策略描述
func (s *RSIStrategy) Description() string {
	return fmt.Sprintf("RSI相对强弱指标策略 (周期:%d, 超买:%.0f, 超卖:%.0f)",
		s.period, s.overboughtLevel, s.oversoldLevel)
}

// RequiredDataPoints 返回所需数据点
func (s *RSIStrategy) RequiredDataPoints() int {
	// RSI需要足够的历史数据来计算稳定的平均涨跌幅
	// 通常需要 period * 5 个数据点来获得准确的RSI值
	// 对于RSI-14，至少需要 14 * 5 = 70 个数据点
	return s.period * 5
}

// SupportedTimeframes 返回支持的时间框架
func (s *RSIStrategy) SupportedTimeframes() []datasource.Timeframe {
	return s.supportedTimeframes
}

// Evaluate 评估策略
func (s *RSIStrategy) Evaluate(data *MarketData) (*StrategyResult, error) {
	ctx := NewIndicatorContext(data)

	// 计算RSI
	rsiResult, err := ctx.RSI(s.period)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate RSI: %w", err)
	}

	if len(rsiResult.Values) == 0 {
		return nil, fmt.Errorf("no RSI values calculated")
	}

	// 获取最新RSI值
	latestRSI := rsiResult.Values[len(rsiResult.Values)-1]
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
	result.Indicators["rsi"] = latestRSI
	result.Indicators["rsi_period"] = s.period
	result.Metadata["overbought_level"] = s.overboughtLevel
	result.Metadata["oversold_level"] = s.oversoldLevel

	// 判断信号
	if latestRSI >= s.overboughtLevel {
		// 超买，卖出信号
		result.Signal = SignalSell
		result.Confidence = calculateRSIConfidence(latestRSI, s.overboughtLevel, true)
		result.Message = fmt.Sprintf("RSI超买信号: %.2f >= %.0f", latestRSI, s.overboughtLevel)

		// 判断强度
		if latestRSI >= s.overboughtLevel+10 {
			result.Strength = StrengthStrong
		} else if latestRSI >= s.overboughtLevel+5 {
			result.Strength = StrengthNormal
		} else {
			result.Strength = StrengthWeak
		}

	} else if latestRSI <= s.oversoldLevel {
		// 超卖，买入信号
		result.Signal = SignalBuy
		result.Confidence = calculateRSIConfidence(latestRSI, s.oversoldLevel, false)
		result.Message = fmt.Sprintf("RSI超卖信号: %.2f <= %.0f", latestRSI, s.oversoldLevel)

		// 判断强度
		if latestRSI <= s.oversoldLevel-10 {
			result.Strength = StrengthStrong
		} else if latestRSI <= s.oversoldLevel-5 {
			result.Strength = StrengthNormal
		} else {
			result.Strength = StrengthWeak
		}

	} else {
		// 中性区域
		result.Signal = SignalNone
		result.Confidence = 0.0
		result.Message = fmt.Sprintf("RSI中性: %.2f (%.0f-%.0f)", latestRSI, s.oversoldLevel, s.overboughtLevel)
	}

	// 添加趋势信息
	if len(rsiResult.Values) >= 2 {
		prevRSI := rsiResult.Values[len(rsiResult.Values)-2]
		rsiTrend := latestRSI - prevRSI
		result.Metadata["rsi_trend"] = rsiTrend

		// 趋势一致性增加置信度
		if result.Signal == SignalBuy && rsiTrend > 0 {
			result.Confidence = minFloat64(result.Confidence*1.1, 1.0)
		} else if result.Signal == SignalSell && rsiTrend < 0 {
			result.Confidence = minFloat64(result.Confidence*1.1, 1.0)
		}
	}

	return result, nil
}

// calculateRSIConfidence 计算RSI置信度
func calculateRSIConfidence(rsi, threshold float64, isOverbought bool) float64 {
	var distance float64

	if isOverbought {
		// 超买：RSI越高，置信度越高
		if rsi < threshold {
			return 0.0
		}
		distance = rsi - threshold
		maxDistance := 100.0 - threshold
		return minFloat64(distance/maxDistance, 1.0)
	} else {
		// 超卖：RSI越低，置信度越高
		if rsi > threshold {
			return 0.0
		}
		distance = threshold - rsi
		maxDistance := threshold - 0.0
		return minFloat64(distance/maxDistance, 1.0)
	}
}

// minFloat64 返回两个数中较小的
func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
