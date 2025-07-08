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
	return fmt.Sprintf("RSI相对强弱指标策略\n• 指标: RSI-%d\n• 超买阈值: %.0f\n• 超卖阈值: %.0f\n• 说明: RSI > %.0f 为超买区域(卖出信号), RSI < %.0f 为超卖区域(买入信号)",
		s.period, s.overboughtLevel, s.oversoldLevel, s.overboughtLevel, s.oversoldLevel)
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
		Signal:    SignalNone,
		Strength:  StrengthNormal,
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
		Indicators: map[string]interface{}{
			"rsi":        latestRSI,
			"rsi_period": s.period,
			"price":      currentPrice,
		},
		Thresholds: map[string]interface{}{
			"overbought_level": s.overboughtLevel,
			"oversold_level":   s.oversoldLevel,
		},
	}

	// 生成指标摘要
	result.IndicatorSummary = fmt.Sprintf("RSI-%d: %.1f (超买>%.0f, 超卖<%.0f)",
		s.period, latestRSI, s.overboughtLevel, s.oversoldLevel)

	// 判断信号并生成描述
	if latestRSI >= s.overboughtLevel {
		// 超买，卖出信号
		result.Signal = SignalSell
		result.Message = fmt.Sprintf("🔴 RSI超买信号")
		result.DetailedAnalysis = fmt.Sprintf("RSI值 %.1f 已达到超买阈值 %.0f 以上，市场可能出现回调。<br/>RSI指标显示当前价格已被高估。",
			latestRSI, s.overboughtLevel)

		// 判断强度
		if latestRSI >= s.overboughtLevel+10 {
			result.Strength = StrengthStrong
			result.DetailedAnalysis += "<br/>📈 超买程度较为严重，信号强度: 强"
		} else if latestRSI >= s.overboughtLevel+5 {
			result.Strength = StrengthNormal
			result.DetailedAnalysis += "<br/>📊 超买程度适中，信号强度: 中等"
		} else {
			result.Strength = StrengthWeak
			result.DetailedAnalysis += "<br/>📉 刚进入超买区域，信号强度: 弱"
		}

	} else if latestRSI <= s.oversoldLevel {
		// 超卖，买入信号
		result.Signal = SignalBuy
		result.Message = fmt.Sprintf("🟢 RSI超卖信号")
		result.DetailedAnalysis = fmt.Sprintf("RSI值 %.1f 已降至超卖阈值 %.0f 以下，市场可能出现反弹。<br/>RSI指标显示当前价格已被低估。",
			latestRSI, s.oversoldLevel)

		// 判断强度
		if latestRSI <= s.oversoldLevel-10 {
			result.Strength = StrengthStrong
			result.DetailedAnalysis += "<br/>📈 超卖程度较为严重，信号强度: 强"
		} else if latestRSI <= s.oversoldLevel-5 {
			result.Strength = StrengthNormal
			result.DetailedAnalysis += "<br/>📊 超卖程度适中，信号强度: 中等"
		} else {
			result.Strength = StrengthWeak
			result.DetailedAnalysis += "<br/>📉 刚进入超卖区域，信号强度: 弱"
		}

	} else {
		// 中性区域
		result.Signal = SignalNone
		result.Message = fmt.Sprintf("⚪ RSI中性区域")
		result.DetailedAnalysis = fmt.Sprintf("RSI值 %.1f 处于中性区域 (%.0f-%.0f)，市场暂无明显超买超卖信号。<br/>建议继续观察或等待更明确的信号。",
			latestRSI, s.oversoldLevel, s.overboughtLevel)
	}

	// 添加趋势信息
	if len(rsiResult.Values) >= 2 {
		prevRSI := rsiResult.Values[len(rsiResult.Values)-2]
		rsiTrend := latestRSI - prevRSI
		result.Metadata["rsi_trend"] = rsiTrend
		result.Metadata["rsi_previous"] = prevRSI

		// 添加趋势描述
		trendDesc := ""
		if rsiTrend > 1 {
			trendDesc = "<br/>📈 RSI呈上升趋势"
		} else if rsiTrend < -1 {
			trendDesc = "<br/>📉 RSI呈下降趋势"
		} else {
			trendDesc = "<br/>➡️ RSI趋势平稳"
		}
		result.DetailedAnalysis += trendDesc
	}

	return result, nil
}
