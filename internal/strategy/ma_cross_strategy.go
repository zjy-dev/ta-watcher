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

	return fmt.Sprintf("%s交叉策略\n• 快线: %s-%d\n• 慢线: %s-%d\n• 说明: 快线上穿慢线生成买入信号，快线下穿慢线生成卖出信号",
		typeName, typeName, s.fastPeriod, typeName, s.slowPeriod)
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
		Signal:    SignalNone,
		Strength:  StrengthNormal,
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
		Indicators: map[string]interface{}{
			"fast_ma":     fastCurrent,
			"slow_ma":     slowCurrent,
			"fast_period": s.fastPeriod,
			"slow_period": s.slowPeriod,
			"ma_type":     s.maType,
			"price":       currentPrice,
		},
		Thresholds: map[string]interface{}{
			"cross_threshold": 0.0, // 交叉阈值为0
		},
	}

	// 计算差值和差值变化
	currentDiff := fastCurrent - slowCurrent
	previousDiff := fastPrevious - slowPrevious
	diffChange := currentDiff - previousDiff

	result.Metadata["ma_diff"] = currentDiff
	result.Metadata["ma_diff_change"] = diffChange
	result.Metadata["ma_diff_percent"] = (currentDiff / slowCurrent) * 100

	// 生成指标摘要
	var maTypeName string
	switch s.maType {
	case indicators.EMA:
		maTypeName = "EMA"
	case indicators.WMA:
		maTypeName = "WMA"
	default:
		maTypeName = "SMA"
	}
	result.IndicatorSummary = fmt.Sprintf("%s交叉: 快线(%d)=%.2f, 慢线(%d)=%.2f",
		maTypeName, s.fastPeriod, fastCurrent, s.slowPeriod, slowCurrent)

	// 检测交叉并生成信号
	if previousDiff <= 0 && currentDiff > 0 {
		// 黄金交叉：快线上穿慢线，买入信号
		result.Signal = SignalBuy
		result.Message = "🟢 黄金交叉信号"
		result.DetailedAnalysis = fmt.Sprintf("快线 %.2f 上穿慢线 %.2f，形成黄金交叉。这通常预示着上升趋势的开始，建议考虑买入。当前价格差异为 %.2f%%。",
			fastCurrent, slowCurrent, (currentDiff/slowCurrent)*100)

		// 判断信号强度
		diffPercent := (currentDiff / slowCurrent) * 100
		if diffPercent > 2.0 {
			result.Strength = StrengthStrong
			result.DetailedAnalysis += " 📈 价格差异较大，信号强度: 强"
		} else if diffPercent > 1.0 {
			result.Strength = StrengthNormal
			result.DetailedAnalysis += " 📊 价格差异适中，信号强度: 中等"
		} else {
			result.Strength = StrengthWeak
			result.DetailedAnalysis += " 📉 价格差异较小，信号强度: 弱"
		}

	} else if previousDiff >= 0 && currentDiff < 0 {
		// 死亡交叉：快线下穿慢线，卖出信号
		result.Signal = SignalSell
		result.Message = "🔴 死亡交叉信号"
		result.DetailedAnalysis = fmt.Sprintf("快线 %.2f 下穿慢线 %.2f，形成死亡交叉。这通常预示着下降趋势的开始，建议考虑卖出。当前价格差异为 %.2f%%。",
			fastCurrent, slowCurrent, (currentDiff/slowCurrent)*100)

		// 判断信号强度
		diffPercent := (currentDiff / slowCurrent) * 100
		if diffPercent < -2.0 {
			result.Strength = StrengthStrong
			result.DetailedAnalysis += " 📈 价格差异较大，信号强度: 强"
		} else if diffPercent < -1.0 {
			result.Strength = StrengthNormal
			result.DetailedAnalysis += " 📊 价格差异适中，信号强度: 中等"
		} else {
			result.Strength = StrengthWeak
			result.DetailedAnalysis += " 📉 价格差异较小，信号强度: 弱"
		}

	} else {
		// 无交叉信号
		result.Signal = SignalNone
		result.Message = "⚪ 无交叉信号"
		if currentDiff > 0 {
			result.DetailedAnalysis = fmt.Sprintf("快线 %.2f 位于慢线 %.2f 之上，但未发生交叉。当前处于多头排列，价格差异为 %.2f%%。",
				fastCurrent, slowCurrent, (currentDiff/slowCurrent)*100)
		} else {
			result.DetailedAnalysis = fmt.Sprintf("快线 %.2f 位于慢线 %.2f 之下，但未发生交叉。当前处于空头排列，价格差异为 %.2f%%。",
				fastCurrent, slowCurrent, (currentDiff/slowCurrent)*100)
		}
	}

	// 添加趋势信息
	if len(fastMA.Values) >= 3 && len(slowMA.Values) >= 3 {
		// 计算趋势强度
		fastTrend := fastMA.Values[len(fastMA.Values)-1] - fastMA.Values[len(fastMA.Values)-3]
		slowTrend := slowMA.Values[len(slowMA.Values)-1] - slowMA.Values[len(slowMA.Values)-3]

		result.Metadata["fast_trend"] = fastTrend
		result.Metadata["slow_trend"] = slowTrend

		// 添加趋势描述
		trendDesc := ""
		if fastTrend > 0 && slowTrend > 0 {
			trendDesc = " 📈 双线均呈上升趋势"
		} else if fastTrend < 0 && slowTrend < 0 {
			trendDesc = " 📉 双线均呈下降趋势"
		} else {
			trendDesc = " ➡️ 趋势方向分歧"
		}
		result.DetailedAnalysis += trendDesc
	}

	return result, nil
}

// minInt 返回两个整数中较小的
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
