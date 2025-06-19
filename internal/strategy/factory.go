package strategy

import (
	"fmt"
	"strings"

	"ta-watcher/internal/indicators"
)

// Factory 策略工厂
type Factory struct {
	presets map[string]func() Strategy
}

// NewFactory 创建策略工厂
func NewFactory() *Factory {
	factory := &Factory{
		presets: make(map[string]func() Strategy),
	}

	// 注册预设策略
	factory.registerDefaultPresets()

	return factory
}

// registerDefaultPresets 注册默认预设策略
func (f *Factory) registerDefaultPresets() {
	// RSI 策略预设
	f.presets["rsi_conservative"] = func() Strategy {
		return NewRSIStrategy(14, 75, 25) // 保守参数
	}
	f.presets["rsi_aggressive"] = func() Strategy {
		return NewRSIStrategy(14, 65, 35) // 激进参数
	}
	f.presets["rsi_scalping"] = func() Strategy {
		return NewRSIStrategy(7, 70, 30) // 短线参数
	}

	// 移动平均线策略预设
	f.presets["ma_golden_cross"] = func() Strategy {
		return NewMACrossStrategy(5, 20, indicators.SMA) // 黄金交叉
	}
	f.presets["ma_ema_cross"] = func() Strategy {
		return NewMACrossStrategy(12, 26, indicators.EMA) // EMA交叉
	}
	f.presets["ma_long_term"] = func() Strategy {
		return NewMACrossStrategy(20, 50, indicators.SMA) // 长期交叉
	}
	f.presets["ma_classic"] = func() Strategy {
		return NewMACrossStrategy(50, 200, indicators.SMA) // 经典50/200日均线
	}
	f.presets["ma_weekly"] = func() Strategy {
		return NewMACrossStrategy(10, 30, indicators.EMA) // 周线EMA策略
	}

	// MACD 策略预设
	f.presets["macd_standard"] = func() Strategy {
		return NewMACDStrategy(12, 26, 9) // 标准参数
	}
	f.presets["macd_fast"] = func() Strategy {
		return NewMACDStrategy(6, 13, 5) // 快速参数
	}
	f.presets["macd_slow"] = func() Strategy {
		return NewMACDStrategy(26, 52, 18) // 慢速参数
	}
	f.presets["macd_weekly"] = func() Strategy {
		return NewMACDStrategy(36, 72, 24) // 周线参数
	}
	f.presets["macd_monthly"] = func() Strategy {
		return NewMACDStrategy(60, 120, 36) // 月线参数
	}

	// 组合策略预设
	f.presets["balanced_combo"] = func() Strategy {
		combo := NewMultiStrategy("平衡组合", "RSI+MA+MACD平衡组合策略", CombineWeightedAverage)
		combo.AddSubStrategy(NewRSIStrategy(14, 70, 30), 1.0)
		combo.AddSubStrategy(NewMACrossStrategy(12, 26, indicators.EMA), 1.0)
		combo.AddSubStrategy(NewMACDStrategy(12, 26, 9), 1.0)
		return combo
	}

	f.presets["consensus_combo"] = func() Strategy {
		combo := NewMultiStrategy("共识组合", "多策略共识决策", CombineConsensus)
		combo.AddSubStrategy(NewRSIStrategy(14, 70, 30), 1.0)
		combo.AddSubStrategy(NewMACrossStrategy(5, 20, indicators.SMA), 1.0)
		combo.AddSubStrategy(NewMACDStrategy(12, 26, 9), 1.0)
		return combo
	}

	f.presets["scalping_combo"] = func() Strategy {
		combo := NewMultiStrategy("短线组合", "快速短线交易策略", CombineStrongest)
		combo.AddSubStrategy(NewRSIStrategy(7, 65, 35), 1.5)
		combo.AddSubStrategy(NewMACrossStrategy(5, 10, indicators.EMA), 1.0)
		combo.AddSubStrategy(NewMACDStrategy(6, 13, 5), 1.2)
		return combo
	}

	f.presets["weekly_combo"] = func() Strategy {
		combo := NewMultiStrategy("周线组合", "适合周线级别的策略组合", CombineWeightedAverage)
		combo.AddSubStrategy(NewRSIStrategy(14, 80, 20), 1.0)
		combo.AddSubStrategy(NewMACrossStrategy(10, 30, indicators.EMA), 1.5)
		combo.AddSubStrategy(NewMACDStrategy(36, 72, 24), 1.2)
		return combo
	}

	f.presets["monthly_combo"] = func() Strategy {
		combo := NewMultiStrategy("月线组合", "适合月线级别的价值投资策略", CombineConsensus)
		combo.AddSubStrategy(NewRSIStrategy(14, 85, 15), 1.0)
		combo.AddSubStrategy(NewMACrossStrategy(12, 36, indicators.SMA), 2.0)
		combo.AddSubStrategy(NewMACDStrategy(60, 120, 36), 1.0)
		return combo
	}

	f.presets["trend_following"] = func() Strategy {
		combo := NewMultiStrategy("趋势跟踪", "强趋势跟踪策略，适合中长期", CombineWeightedAverage)
		combo.AddSubStrategy(NewRSIStrategy(21, 75, 25), 1.0)
		combo.AddSubStrategy(NewMACrossStrategy(50, 200, indicators.SMA), 2.5) // 经典趋势线
		combo.AddSubStrategy(NewMACDStrategy(26, 52, 18), 1.5)
		return combo
	}
}

// CreateStrategy 创建策略
func (f *Factory) CreateStrategy(name string, params ...interface{}) (Strategy, error) {
	// 首先检查预设策略
	if creator, exists := f.presets[name]; exists {
		return creator(), nil
	}

	// 解析自定义策略
	return f.parseCustomStrategy(name, params...)
}

// parseCustomStrategy 解析自定义策略
func (f *Factory) parseCustomStrategy(name string, params ...interface{}) (Strategy, error) {
	parts := strings.Split(strings.ToLower(name), "_")
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid strategy name: %s", name)
	}

	strategyType := parts[0]

	switch strategyType {
	case "rsi":
		return f.createRSIStrategy(params...)
	case "ma", "sma", "ema", "wma":
		return f.createMAStrategy(strategyType, params...)
	case "macd":
		return f.createMACDStrategy(params...)
	case "multi", "combo":
		return f.createMultiStrategy(params...)
	default:
		return nil, fmt.Errorf("unknown strategy type: %s", strategyType)
	}
}

// createRSIStrategy 创建RSI策略
func (f *Factory) createRSIStrategy(params ...interface{}) (Strategy, error) {
	period := 14
	overbought := 70.0
	oversold := 30.0

	if len(params) >= 1 {
		if p, ok := params[0].(int); ok {
			period = p
		}
	}
	if len(params) >= 2 {
		if ob, ok := params[1].(float64); ok {
			overbought = ob
		}
	}
	if len(params) >= 3 {
		if os, ok := params[2].(float64); ok {
			oversold = os
		}
	}

	return NewRSIStrategy(period, overbought, oversold), nil
}

// createMAStrategy 创建移动平均线策略
func (f *Factory) createMAStrategy(maType string, params ...interface{}) (Strategy, error) {
	fastPeriod := 5
	slowPeriod := 20
	var avgType indicators.MovingAverageType

	switch maType {
	case "ema":
		avgType = indicators.EMA
	case "wma":
		avgType = indicators.WMA
	default:
		avgType = indicators.SMA
	}

	if len(params) >= 1 {
		if fp, ok := params[0].(int); ok {
			fastPeriod = fp
		}
	}
	if len(params) >= 2 {
		if sp, ok := params[1].(int); ok {
			slowPeriod = sp
		}
	}

	return NewMACrossStrategy(fastPeriod, slowPeriod, avgType), nil
}

// createMACDStrategy 创建MACD策略
func (f *Factory) createMACDStrategy(params ...interface{}) (Strategy, error) {
	fastPeriod := 12
	slowPeriod := 26
	signalPeriod := 9

	if len(params) >= 1 {
		if fp, ok := params[0].(int); ok {
			fastPeriod = fp
		}
	}
	if len(params) >= 2 {
		if sp, ok := params[1].(int); ok {
			slowPeriod = sp
		}
	}
	if len(params) >= 3 {
		if sig, ok := params[2].(int); ok {
			signalPeriod = sig
		}
	}

	return NewMACDStrategy(fastPeriod, slowPeriod, signalPeriod), nil
}

// createMultiStrategy 创建组合策略
func (f *Factory) createMultiStrategy(params ...interface{}) (Strategy, error) {
	name := "自定义组合"
	description := "自定义多策略组合"
	method := CombineWeightedAverage

	if len(params) >= 1 {
		if n, ok := params[0].(string); ok {
			name = n
		}
	}
	if len(params) >= 2 {
		if d, ok := params[1].(string); ok {
			description = d
		}
	}
	if len(params) >= 3 {
		if m, ok := params[2].(CombineMethod); ok {
			method = m
		}
	}

	return NewMultiStrategy(name, description, method), nil
}

// ListPresets 列出所有预设策略
func (f *Factory) ListPresets() []string {
	presets := make([]string, 0, len(f.presets))
	for name := range f.presets {
		presets = append(presets, name)
	}
	return presets
}

// GetPresetDescription 获取预设策略描述
func (f *Factory) GetPresetDescription(name string) string {
	descriptions := map[string]string{
		"rsi_conservative": "保守RSI策略 (14, 75/25) - 适合稳健投资",
		"rsi_aggressive":   "激进RSI策略 (14, 65/35) - 适合活跃交易",
		"rsi_scalping":     "短线RSI策略 (7, 70/30) - 适合快速进出",
		"ma_golden_cross":  "黄金交叉策略 (SMA 5/20) - 经典趋势跟踪",
		"ma_ema_cross":     "EMA交叉策略 (EMA 12/26) - 快速趋势响应",
		"ma_long_term":     "长期MA策略 (SMA 20/50) - 适合长期持有",
		"macd_standard":    "标准MACD策略 (12/26/9) - 经典动量指标",
		"macd_fast":        "快速MACD策略 (6/13/5) - 敏感信号捕捉",
		"macd_slow":        "慢速MACD策略 (26/52/18) - 过滤噪音",
		"balanced_combo":   "平衡组合策略 - RSI+MA+MACD均衡组合",
		"consensus_combo":  "共识组合策略 - 多策略投票决策",
		"scalping_combo":   "短线组合策略 - 快速交易优化组合",
	}

	if desc, exists := descriptions[name]; exists {
		return desc
	}
	return "未知策略"
}

// RegisterPreset 注册自定义预设策略
func (f *Factory) RegisterPreset(name string, creator func() Strategy) error {
	if _, exists := f.presets[name]; exists {
		return fmt.Errorf("preset '%s' already exists", name)
	}

	f.presets[name] = creator
	return nil
}

// UnregisterPreset 注销预设策略
func (f *Factory) UnregisterPreset(name string) error {
	if _, exists := f.presets[name]; !exists {
		return fmt.Errorf("preset '%s' not found", name)
	}

	delete(f.presets, name)
	return nil
}

// CreateRecommendedStrategy 根据时间框架创建推荐策略
func (f *Factory) CreateRecommendedStrategy(timeframe Timeframe) (Strategy, error) {
	switch timeframe {
	case Timeframe1m, Timeframe3m, Timeframe5m:
		// 超短线时间框架，使用快速响应策略
		return f.CreateStrategy("scalping_combo")

	case Timeframe15m, Timeframe30m:
		// 短线时间框架，使用平衡策略
		return f.CreateStrategy("balanced_combo")

	case Timeframe1h, Timeframe2h, Timeframe4h:
		// 中线时间框架，使用共识策略
		return f.CreateStrategy("consensus_combo")

	case Timeframe6h, Timeframe12h, Timeframe1d:
		// 中长线时间框架，使用稳健策略
		combo := NewMultiStrategy("中长线组合", "适合中长线投资的策略组合", CombineConsensus)
		combo.AddSubStrategy(NewRSIStrategy(14, 75, 25), 1.0)
		combo.AddSubStrategy(NewMACrossStrategy(20, 50, indicators.SMA), 1.5)
		combo.AddSubStrategy(NewMACDStrategy(12, 26, 9), 1.2)
		return combo, nil

	case Timeframe3d, Timeframe1w:
		// 长线时间框架，使用长期趋势策略
		combo := NewMultiStrategy("长线趋势组合", "适合长期趋势投资的策略组合", CombineWeightedAverage)
		combo.AddSubStrategy(NewRSIStrategy(14, 80, 20), 1.0)
		combo.AddSubStrategy(NewMACrossStrategy(50, 200, indicators.SMA), 2.0) // 经典50/200日均线
		combo.AddSubStrategy(NewMACDStrategy(26, 52, 18), 1.0)
		return combo, nil

	case Timeframe1M:
		// 月线时间框架，使用超长期价值投资策略
		combo := NewMultiStrategy("价值投资组合", "适合价值投资的超长期策略", CombineUnanimous)
		combo.AddSubStrategy(NewRSIStrategy(14, 85, 15), 1.0)                 // 更极端的超买超卖水平
		combo.AddSubStrategy(NewMACrossStrategy(12, 36, indicators.EMA), 1.5) // 月线EMA交叉
		combo.AddSubStrategy(NewMACDStrategy(36, 72, 24), 1.0)                // 月线MACD参数
		return combo, nil

	default:
		return f.CreateStrategy("balanced_combo")
	}
}
