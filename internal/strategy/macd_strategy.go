package strategy

import (
	"fmt"
	"ta-watcher/internal/datasource"
	"time"
)

// MACDStrategy MACD策略
type MACDStrategy struct {
	name                string
	fastPeriod          int
	slowPeriod          int
	signalPeriod        int
	supportedTimeframes []datasource.Timeframe
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
		supportedTimeframes: []datasource.Timeframe{
			datasource.Timeframe15m, datasource.Timeframe30m, datasource.Timeframe1h, datasource.Timeframe2h,
			datasource.Timeframe4h, datasource.Timeframe6h, datasource.Timeframe12h,
			datasource.Timeframe1d, datasource.Timeframe3d, datasource.Timeframe1w, datasource.Timeframe1M,
		},
	}
}

// Name 返回策略名称
func (s *MACDStrategy) Name() string {
	return s.name
}

// Description 返回策略描述
func (s *MACDStrategy) Description() string {
	return fmt.Sprintf("MACD指标策略\n• 快线EMA: %d\n• 慢线EMA: %d\n• 信号线EMA: %d\n• 说明: MACD线上穿信号线生成买入信号，下穿信号线生成卖出信号",
		s.fastPeriod, s.slowPeriod, s.signalPeriod)
}

// RequiredDataPoints 返回所需数据点
func (s *MACDStrategy) RequiredDataPoints() int {
	return s.slowPeriod + s.signalPeriod + 10 // 额外缓冲
}

// SupportedTimeframes 返回支持的时间框架
func (s *MACDStrategy) SupportedTimeframes() []datasource.Timeframe {
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
		Signal:    SignalNone,
		Strength:  StrengthNormal,
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
		Indicators: map[string]interface{}{
			"macd":          macdCurrent,
			"signal":        signalCurrent,
			"histogram":     histCurrent,
			"fast_period":   s.fastPeriod,
			"slow_period":   s.slowPeriod,
			"signal_period": s.signalPeriod,
			"price":         currentPrice,
		},
		Thresholds: map[string]interface{}{
			"cross_threshold": 0.0, // MACD交叉阈值为0
		},
	}

	// 生成指标摘要
	result.IndicatorSummary = fmt.Sprintf("MACD(%d,%d,%d): MACD=%.4f, Signal=%.4f, Hist=%.4f",
		s.fastPeriod, s.slowPeriod, s.signalPeriod, macdCurrent, signalCurrent, histCurrent)

	// 计算趋势和动量
	macdTrend := macdCurrent - macdPrev
	histTrend := histCurrent - histPrev
	result.Metadata["macd_trend"] = macdTrend
	result.Metadata["hist_trend"] = histTrend
	result.Metadata["macd_previous"] = macdPrev
	result.Metadata["hist_previous"] = histPrev

	// 检测MACD交叉信号
	if macdPrev <= signalPrev && macdCurrent > signalCurrent {
		// MACD线上穿信号线，买入信号
		result.Signal = SignalBuy
		result.Message = "🟢 MACD金叉信号"
		result.DetailedAnalysis = fmt.Sprintf("MACD线 %.4f 上穿信号线 %.4f，形成金叉。<br/>柱状图值为 %.4f。这通常预示着上升趋势的开始，建议考虑买入。",
			macdCurrent, signalCurrent, histCurrent)

		// 判断信号强度
		crossStrength := macdCurrent - signalCurrent
		if crossStrength > 0.002 {
			result.Strength = StrengthStrong
			result.DetailedAnalysis += "<br/>📈 交叉强度较大，信号强度: 强"
		} else if crossStrength > 0.001 {
			result.Strength = StrengthNormal
			result.DetailedAnalysis += "<br/>📊 交叉强度适中，信号强度: 中等"
		} else {
			result.Strength = StrengthWeak
			result.DetailedAnalysis += "<br/>📉 交叉强度较小，信号强度: 弱"
		}

	} else if macdPrev >= signalPrev && macdCurrent < signalCurrent {
		// MACD线下穿信号线，卖出信号
		result.Signal = SignalSell
		result.Message = "🔴 MACD死叉信号"
		result.DetailedAnalysis = fmt.Sprintf("MACD线 %.4f 下穿信号线 %.4f，形成死叉。<br/>柱状图值为 %.4f。这通常预示着下降趋势的开始，建议考虑卖出。",
			macdCurrent, signalCurrent, histCurrent)

		// 判断信号强度
		crossStrength := signalCurrent - macdCurrent
		if crossStrength > 0.002 {
			result.Strength = StrengthStrong
			result.DetailedAnalysis += "<br/>📈 交叉强度较大，信号强度: 强"
		} else if crossStrength > 0.001 {
			result.Strength = StrengthNormal
			result.DetailedAnalysis += "<br/>📊 交叉强度适中，信号强度: 中等"
		} else {
			result.Strength = StrengthWeak
			result.DetailedAnalysis += "<br/>📉 交叉强度较小，信号强度: 弱"
		}

	} else {
		// 无交叉信号
		result.Signal = SignalNone
		result.Message = "⚪ MACD无交叉信号"
		if macdCurrent > signalCurrent {
			result.DetailedAnalysis = fmt.Sprintf("MACD线 %.4f 位于信号线 %.4f 之上，但未发生交叉。<br/>柱状图值为 %.4f，当前处于多头状态。",
				macdCurrent, signalCurrent, histCurrent)
		} else {
			result.DetailedAnalysis = fmt.Sprintf("MACD线 %.4f 位于信号线 %.4f 之下，但未发生交叉。<br/>柱状图值为 %.4f，当前处于空头状态。",
				macdCurrent, signalCurrent, histCurrent)
		}
	}

	// 添加趋势信息
	if len(macdResult.MACD) >= 3 {
		// 计算趋势强度
		macdTrend3 := macdCurrent - macdResult.MACD[len(macdResult.MACD)-3]
		histTrend3 := histCurrent - macdResult.Histogram[len(macdResult.Histogram)-3]

		result.Metadata["macd_trend_3"] = macdTrend3
		result.Metadata["hist_trend_3"] = histTrend3

		// 添加趋势描述
		trendDesc := ""
		if macdTrend3 > 0 && histTrend3 > 0 {
			trendDesc = "<br/>📈 MACD和柱状图均呈上升趋势"
		} else if macdTrend3 < 0 && histTrend3 < 0 {
			trendDesc = "<br/>📉 MACD和柱状图均呈下降趋势"
		} else {
			trendDesc = "<br/>➡️ MACD趋势方向分歧"
		}
		result.DetailedAnalysis += trendDesc
	}

	return result, nil
}
