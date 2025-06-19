// Package indicators provides technical analysis indicator calculations
package indicators

import (
	"errors"
	"math"
)

// MovingAverageType 移动平均线类型
type MovingAverageType int

const (
	SMA MovingAverageType = iota // Simple Moving Average 简单移动平均线
	EMA                          // Exponential Moving Average 指数移动平均线
	WMA                          // Weighted Moving Average 加权移动平均线
)

// MAResult 移动平均线计算结果
type MAResult struct {
	Values []float64 // MA 值序列
	Period int       // 周期
	Type   MovingAverageType
}

// CalculateSMA 计算简单移动平均线
// prices: 价格序列（通常是收盘价）
// period: 移动平均周期
func CalculateSMA(prices []float64, period int) (*MAResult, error) {
	if len(prices) < period {
		return nil, errors.New("价格数据不足，无法计算指定周期的移动平均线")
	}

	if period <= 0 {
		return nil, errors.New("移动平均周期必须大于0")
	}

	var smaValues []float64

	// 计算每个位置的SMA值
	for i := period - 1; i < len(prices); i++ {
		sum := 0.0
		// 计算period个价格的平均值
		for j := i - period + 1; j <= i; j++ {
			sum += prices[j]
		}
		smaValues = append(smaValues, sum/float64(period))
	}

	return &MAResult{
		Values: smaValues,
		Period: period,
		Type:   SMA,
	}, nil
}

// CalculateEMA 计算指数移动平均线
// prices: 价格序列
// period: 移动平均周期
func CalculateEMA(prices []float64, period int) (*MAResult, error) {
	if len(prices) < period {
		return nil, errors.New("价格数据不足，无法计算指定周期的指数移动平均线")
	}

	if period <= 0 {
		return nil, errors.New("移动平均周期必须大于0")
	}

	// 计算平滑系数
	multiplier := 2.0 / (float64(period) + 1.0)
	var emaValues []float64

	// 第一个EMA值使用SMA作为初始值
	firstSMA := 0.0
	for i := 0; i < period; i++ {
		firstSMA += prices[i]
	}
	firstSMA /= float64(period)
	emaValues = append(emaValues, firstSMA)

	// 计算后续的EMA值
	for i := period; i < len(prices); i++ {
		ema := (prices[i] * multiplier) + emaValues[len(emaValues)-1]*(1-multiplier)
		emaValues = append(emaValues, ema)
	}

	return &MAResult{
		Values: emaValues,
		Period: period,
		Type:   EMA,
	}, nil
}

// CalculateWMA 计算加权移动平均线
// prices: 价格序列
// period: 移动平均周期
func CalculateWMA(prices []float64, period int) (*MAResult, error) {
	if len(prices) < period {
		return nil, errors.New("价格数据不足，无法计算指定周期的加权移动平均线")
	}

	if period <= 0 {
		return nil, errors.New("移动平均周期必须大于0")
	}

	var wmaValues []float64

	// 计算权重总和
	weightSum := float64(period * (period + 1) / 2)

	// 计算每个位置的WMA值
	for i := period - 1; i < len(prices); i++ {
		weightedSum := 0.0
		// 计算加权总和
		for j := 0; j < period; j++ {
			weight := float64(j + 1)
			weightedSum += prices[i-period+1+j] * weight
		}
		wmaValues = append(wmaValues, weightedSum/weightSum)
	}

	return &MAResult{
		Values: wmaValues,
		Period: period,
		Type:   WMA,
	}, nil
}

// GetLatest 获取最新的MA值
func (m *MAResult) GetLatest() float64 {
	if len(m.Values) == 0 {
		return 0
	}
	return m.Values[len(m.Values)-1]
}

// GetLatestN 获取最新的N个MA值
func (m *MAResult) GetLatestN(n int) []float64 {
	if n <= 0 || len(m.Values) == 0 {
		return []float64{}
	}

	start := int(math.Max(0, float64(len(m.Values)-n)))
	return m.Values[start:]
}

// IsGoldenCross 检查是否发生金叉（短期MA上穿长期MA）
func IsGoldenCross(shortMA, longMA *MAResult) bool {
	if len(shortMA.Values) < 2 || len(longMA.Values) < 2 {
		return false
	}

	// 当前值：短期MA > 长期MA
	currentShort := shortMA.GetLatest()
	currentLong := longMA.GetLatest()

	// 前一个值：短期MA <= 长期MA
	prevShort := shortMA.Values[len(shortMA.Values)-2]
	prevLong := longMA.Values[len(longMA.Values)-2]

	return currentShort > currentLong && prevShort <= prevLong
}

// IsDeathCross 检查是否发生死叉（短期MA下穿长期MA）
func IsDeathCross(shortMA, longMA *MAResult) bool {
	if len(shortMA.Values) < 2 || len(longMA.Values) < 2 {
		return false
	}

	// 当前值：短期MA < 长期MA
	currentShort := shortMA.GetLatest()
	currentLong := longMA.GetLatest()

	// 前一个值：短期MA >= 长期MA
	prevShort := shortMA.Values[len(shortMA.Values)-2]
	prevLong := longMA.Values[len(longMA.Values)-2]

	return currentShort < currentLong && prevShort >= prevLong
}
