package main

import (
	"fmt"
	"time"
	
	"ta-watcher/internal/strategy"
	"ta-watcher/internal/binance"
	"ta-watcher/internal/indicators"
)

// test_strategyStrategy 自定义策略实现
type test_strategyStrategy struct {
	name        string
	description string
	// 在这里添加策略参数
	period      int
	threshold   float64
}

// NewStrategy 创建策略实例 (插件导出函数)
func NewStrategy() strategy.Strategy {
	return &test_strategyStrategy{
		name:        "test_strategy",
		description: "这是一个自定义策略模板",
		period:      14,
		threshold:   0.02,
	}
}

// Name 返回策略名称
func (s *test_strategyStrategy) Name() string {
	return s.name
}

// Description 返回策略描述
func (s *test_strategyStrategy) Description() string {
	return s.description
}

// RequiredDataPoints 返回所需的最少数据点数
func (s *test_strategyStrategy) RequiredDataPoints() int {
	return s.period + 10 // 通常需要比指标周期多一些数据
}

// SupportedTimeframes 返回支持的时间框架
func (s *test_strategyStrategy) SupportedTimeframes() []strategy.Timeframe {
	return []strategy.Timeframe{
		strategy.Timeframe5m,
		strategy.Timeframe15m,
		strategy.Timeframe1h,
		strategy.Timeframe4h,
		strategy.Timeframe1d,
	}
}

// Evaluate 评估策略，返回信号
func (s *test_strategyStrategy) Evaluate(data *strategy.MarketData) (*strategy.StrategyResult, error) {
	if len(data.Klines) < s.RequiredDataPoints() {
		return &strategy.StrategyResult{
			Signal:     strategy.SignalNone,
			Strength:   strategy.StrengthWeak,
			Confidence: 0.0,
			Timestamp:  time.Now(),
			Message:    "数据点不足",
		}, nil
	}

	// 提取价格数据
	closes := make([]float64, len(data.Klines))
	for i, kline := range data.Klines {
		closes[i] = kline.Close
	}

	// 在这里实现你的策略逻辑
	// 示例：简单的价格变化策略
	currentPrice := closes[len(closes)-1]
	previousPrice := closes[len(closes)-2]
	priceChange := (currentPrice - previousPrice) / previousPrice

	var signal strategy.Signal
	var strength strategy.Strength
	var confidence float64
	var message string

	if priceChange > s.threshold {
		signal = strategy.SignalBuy
		strength = strategy.StrengthNormal
		confidence = 0.7
		message = fmt.Sprintf("价格上涨 %!f(string=te)%", priceChange*100)
	} else if priceChange < -s.threshold {
		signal = strategy.SignalSell
		strength = strategy.StrengthNormal
		confidence = 0.7
		message = fmt.Sprintf("价格下跌 %!f(string=te)%", -priceChange*100)
	} else {
		signal = strategy.SignalHold
		strength = strategy.StrengthWeak
		confidence = 0.3
		message = "价格变化不大，建议持有"
	}

	return &strategy.StrategyResult{
		Signal:     signal,
		Strength:   strength,
		Confidence: confidence,
		Price:      currentPrice,
		Timestamp:  time.Now(),
		Message:    message,
		Metadata: map[string]interface{}{
			"price_change":     priceChange,
			"current_price":    currentPrice,
			"previous_price":   previousPrice,
			"threshold":        s.threshold,
		},
		Indicators: map[string]interface{}{
			"price_change_pct": priceChange * 100,
		},
	}, nil
}

// 编译指令：
// go build -buildmode=plugin -o %!s(MISSING).so %!s(MISSING).go
