package strategy

import (
	"fmt"
	"time"
)

// MACDStrategy MACD策略
type MACDStrategy struct {
	name                string
	fastPeriod          int
	slowPeriod          int
	signalPeriod        int
	supportedTimeframes []Timeframe
}

// NewMACDStrategy 创建MACD策略
func NewMACDStrategy(fastPeriod, slowPeriod, signalPeriod int) *MACDStrategy {
	if fastPeriod <= 0 {
		fastPeriod = 12
	}
	if slowPeriod <= 0 {
		slowPeriod = 26
	}
	if signalPeriod <= 0 {
		signalPeriod = 9
	}

	// 确保快周期小于慢周期
	if fastPeriod >= slowPeriod {
		fastPeriod, slowPeriod = 12, 26
	}

	return &MACDStrategy{
		name:         fmt.Sprintf("MACD_%d_%d_%d", fastPeriod, slowPeriod, signalPeriod),
		fastPeriod:   fastPeriod,
		slowPeriod:   slowPeriod,
		signalPeriod: signalPeriod,
		supportedTimeframes: []Timeframe{
			Timeframe15m, Timeframe30m, Timeframe1h, Timeframe2h,
			Timeframe4h, Timeframe6h, Timeframe12h,
			Timeframe1d, Timeframe3d, Timeframe1w, Timeframe1M,
		},
	}
}

// Name 返回策略名称
func (s *MACDStrategy) Name() string {
	return s.name
}

// Description 返回策略描述
func (s *MACDStrategy) Description() string {
	return fmt.Sprintf("MACD指标策略 (快线:%d, 慢线:%d, 信号线:%d)",
		s.fastPeriod, s.slowPeriod, s.signalPeriod)
}

// RequiredDataPoints 返回所需数据点
func (s *MACDStrategy) RequiredDataPoints() int {
	return s.slowPeriod + s.signalPeriod + 10 // 额外缓冲
}

// SupportedTimeframes 返回支持的时间框架
func (s *MACDStrategy) SupportedTimeframes() []Timeframe {
	return s.supportedTimeframes
}

// Evaluate 评估策略
func (s *MACDStrategy) Evaluate(data *MarketData) (*StrategyResult, error) {
	ctx := NewIndicatorContext(data)

	// 计算MACD
	macdResult, err := ctx.MACD(s.fastPeriod, s.slowPeriod, s.signalPeriod)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate MACD: %w", err)
	}

	if len(macdResult.MACD) < 2 || len(macdResult.Signal) < 2 || len(macdResult.Histogram) < 2 {
		return nil, fmt.Errorf("insufficient MACD data points")
	}

	// 获取最新值
	latestIdx := len(macdResult.MACD) - 1
	prevIdx := latestIdx - 1

	macdCurrent := macdResult.MACD[latestIdx]
	macdPrev := macdResult.MACD[prevIdx]
	signalCurrent := macdResult.Signal[latestIdx]
	signalPrev := macdResult.Signal[prevIdx]
	histCurrent := macdResult.Histogram[latestIdx]
	histPrev := macdResult.Histogram[prevIdx]

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
	result.Indicators["macd"] = macdCurrent
	result.Indicators["signal"] = signalCurrent
	result.Indicators["histogram"] = histCurrent
	result.Indicators["fast_period"] = s.fastPeriod
	result.Indicators["slow_period"] = s.slowPeriod
	result.Indicators["signal_period"] = s.signalPeriod

	// 计算趋势和动量
	macdTrend := macdCurrent - macdPrev
	histTrend := histCurrent - histPrev

	result.Metadata["macd_trend"] = macdTrend
	result.Metadata["hist_trend"] = histTrend

	// 分析信号
	signal, confidence, strength, message := s.analyzeMACD(
		macdCurrent, macdPrev, signalCurrent, signalPrev,
		histCurrent, histPrev, macdTrend, histTrend,
	)

	result.Signal = signal
	result.Confidence = confidence
	result.Strength = strength
	result.Message = message

	// 考虑价格确认
	priceChange := ctx.PriceChange(s.signalPeriod)
	if (signal == SignalBuy && priceChange > 0) || (signal == SignalSell && priceChange < 0) {
		result.Confidence = minFloat64(result.Confidence*1.15, 1.0)
		result.Metadata["price_confirmation"] = true
	} else {
		result.Metadata["price_confirmation"] = false
	}

	return result, nil
}

// analyzeMACD 分析MACD信号
func (s *MACDStrategy) analyzeMACD(
	macdCurrent, macdPrev, signalCurrent, signalPrev,
	histCurrent, histPrev, macdTrend, histTrend float64,
) (Signal, float64, Strength, string) {

	// 1. MACD线与信号线交叉
	if macdPrev <= signalPrev && macdCurrent > signalCurrent {
		// MACD上穿信号线，买入信号
		confidence := calculateMACDConfidence(histCurrent, macdCurrent, true)
		strength := calculateMACDStrength(histCurrent, histTrend, true)
		message := fmt.Sprintf("MACD金叉: MACD(%.4f)上穿信号线(%.4f)", macdCurrent, signalCurrent)
		return SignalBuy, confidence, strength, message

	} else if macdPrev >= signalPrev && macdCurrent < signalCurrent {
		// MACD下穿信号线，卖出信号
		confidence := calculateMACDConfidence(histCurrent, macdCurrent, false)
		strength := calculateMACDStrength(histCurrent, histTrend, false)
		message := fmt.Sprintf("MACD死叉: MACD(%.4f)下穿信号线(%.4f)", macdCurrent, signalCurrent)
		return SignalSell, confidence, strength, message
	}

	// 2. 零轴交叉
	if macdPrev <= 0 && macdCurrent > 0 {
		confidence := 0.7
		strength := StrengthNormal
		if histCurrent > 0 && histTrend > 0 {
			confidence = 0.8
			strength = StrengthStrong
		}
		message := fmt.Sprintf("MACD零轴金叉: MACD(%.4f)上穿零轴", macdCurrent)
		return SignalBuy, confidence, strength, message

	} else if macdPrev >= 0 && macdCurrent < 0 {
		confidence := 0.7
		strength := StrengthNormal
		if histCurrent < 0 && histTrend < 0 {
			confidence = 0.8
			strength = StrengthStrong
		}
		message := fmt.Sprintf("MACD零轴死叉: MACD(%.4f)下穿零轴", macdCurrent)
		return SignalSell, confidence, strength, message
	}

	// 3. 柱状图背离（简化版）
	if histCurrent > 0 && histPrev > 0 && histTrend > 0 {
		// 正向柱状图增强
		if macdCurrent > signalCurrent {
			confidence := minFloat64(absFloat64(histTrend)*100, 0.6)
			message := fmt.Sprintf("MACD多头增强: 柱状图(%.4f)持续扩大", histCurrent)
			return SignalHold, confidence, StrengthNormal, message
		}
	} else if histCurrent < 0 && histPrev < 0 && histTrend < 0 {
		// 负向柱状图增强
		if macdCurrent < signalCurrent {
			confidence := minFloat64(absFloat64(histTrend)*100, 0.6)
			message := fmt.Sprintf("MACD空头增强: 柱状图(%.4f)持续扩大", histCurrent)
			return SignalHold, confidence, StrengthNormal, message
		}
	}

	// 4. 趋势持续
	if macdCurrent > signalCurrent && macdCurrent > 0 {
		message := fmt.Sprintf("MACD多头趋势: MACD(%.4f) > 信号线(%.4f) > 0", macdCurrent, signalCurrent)
		return SignalHold, 0.4, StrengthWeak, message
	} else if macdCurrent < signalCurrent && macdCurrent < 0 {
		message := fmt.Sprintf("MACD空头趋势: MACD(%.4f) < 信号线(%.4f) < 0", macdCurrent, signalCurrent)
		return SignalHold, 0.4, StrengthWeak, message
	}

	// 默认中性
	message := fmt.Sprintf("MACD中性: MACD(%.4f), 信号线(%.4f)", macdCurrent, signalCurrent)
	return SignalNone, 0.0, StrengthNormal, message
}

// calculateMACDConfidence 计算MACD置信度
func calculateMACDConfidence(histogram, macd float64, isBullish bool) float64 {
	// 基于柱状图和MACD值计算置信度
	histAbs := absFloat64(histogram)
	macdAbs := absFloat64(macd)

	// 柱状图越大，置信度越高
	histConfidence := minFloat64(histAbs*500, 0.7) // 放大系数

	// MACD值的大小也影响置信度
	macdConfidence := minFloat64(macdAbs*100, 0.3)

	totalConfidence := histConfidence + macdConfidence

	// 确保在合理范围内
	if totalConfidence < 0.5 {
		totalConfidence = 0.5
	}

	return minFloat64(totalConfidence, 1.0)
}

// calculateMACDStrength 计算MACD强度
func calculateMACDStrength(histogram, histTrend float64, isBullish bool) Strength {
	histAbs := absFloat64(histogram)
	trendAbs := absFloat64(histTrend)

	// 柱状图大且趋势强烈
	if histAbs > 0.01 && trendAbs > 0.005 {
		return StrengthStrong
	} else if histAbs > 0.005 || trendAbs > 0.002 {
		return StrengthNormal
	} else {
		return StrengthWeak
	}
}
