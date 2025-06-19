package indicators

import (
	"errors"
	"math"
)

// RSIResult RSI指标计算结果
type RSIResult struct {
	Values []float64 // RSI 值序列 (0-100)
	Period int       // 计算周期
}

// RSISignal RSI信号类型
type RSISignal int

const (
	RSINeutral RSISignal = iota // 中性
	RSIBuy                      // 买入信号（超卖）
	RSISell                     // 卖出信号（超买）
)

// RSI默认参数
const (
	DefaultRSIPeriod       = 14   // 默认RSI周期
	DefaultOverboughtLevel = 70.0 // 默认超买水平
	DefaultOversoldLevel   = 30.0 // 默认超卖水平
)

// CalculateRSI 计算相对强弱指标
// prices: 价格序列（通常是收盘价）
// period: RSI计算周期，通常为14
func CalculateRSI(prices []float64, period int) (*RSIResult, error) {
	if len(prices) < period+1 {
		return nil, errors.New("价格数据不足，无法计算RSI指标")
	}

	if period <= 0 {
		return nil, errors.New("RSI周期必须大于0")
	}

	// 计算价格变化
	var gains, losses []float64
	for i := 1; i < len(prices); i++ {
		change := prices[i] - prices[i-1]
		if change > 0 {
			gains = append(gains, change)
			losses = append(losses, 0)
		} else {
			gains = append(gains, 0)
			losses = append(losses, -change)
		}
	}

	if len(gains) < period {
		return nil, errors.New("价格变化数据不足，无法计算RSI指标")
	}

	var rsiValues []float64

	// 计算第一个RSI值 - 使用简单平均
	firstAvgGain := 0.0
	firstAvgLoss := 0.0
	for i := 0; i < period; i++ {
		firstAvgGain += gains[i]
		firstAvgLoss += losses[i]
	}
	firstAvgGain /= float64(period)
	firstAvgLoss /= float64(period)

	// 避免除零错误
	if firstAvgLoss == 0 {
		rsiValues = append(rsiValues, 100.0)
	} else {
		rs := firstAvgGain / firstAvgLoss
		rsi := 100.0 - (100.0 / (1.0 + rs))
		rsiValues = append(rsiValues, rsi)
	}

	// 计算后续RSI值 - 使用指数平滑
	avgGain := firstAvgGain
	avgLoss := firstAvgLoss

	for i := period; i < len(gains); i++ {
		// 威尔德平滑（Wilder's smoothing）
		avgGain = ((avgGain * float64(period-1)) + gains[i]) / float64(period)
		avgLoss = ((avgLoss * float64(period-1)) + losses[i]) / float64(period)

		// 计算RSI
		if avgLoss == 0 {
			rsiValues = append(rsiValues, 100.0)
		} else {
			rs := avgGain / avgLoss
			rsi := 100.0 - (100.0 / (1.0 + rs))
			rsiValues = append(rsiValues, rsi)
		}
	}

	return &RSIResult{
		Values: rsiValues,
		Period: period,
	}, nil
}

// GetLatest 获取最新的RSI值
func (r *RSIResult) GetLatest() float64 {
	if len(r.Values) == 0 {
		return 0
	}
	return r.Values[len(r.Values)-1]
}

// GetLatestN 获取最新的N个RSI值
func (r *RSIResult) GetLatestN(n int) []float64 {
	if n <= 0 || len(r.Values) == 0 {
		return []float64{}
	}

	start := int(math.Max(0, float64(len(r.Values)-n)))
	return r.Values[start:]
}

// GetSignal 根据RSI值获取交易信号
func (r *RSIResult) GetSignal(overboughtLevel, oversoldLevel float64) RSISignal {
	if len(r.Values) == 0 {
		return RSINeutral
	}

	latestRSI := r.GetLatest()

	if latestRSI >= overboughtLevel {
		return RSISell // 超买，考虑卖出
	} else if latestRSI <= oversoldLevel {
		return RSIBuy // 超卖，考虑买入
	}

	return RSINeutral
}

// GetDefaultSignal 使用默认阈值获取交易信号
func (r *RSIResult) GetDefaultSignal() RSISignal {
	return r.GetSignal(DefaultOverboughtLevel, DefaultOversoldLevel)
}

// IsDivergence 检测RSI背离
// prices: 对应的价格序列
// 返回：是否存在背离，背离类型（看涨/看跌）
func (r *RSIResult) IsDivergence(prices []float64) (bool, bool) {
	if len(r.Values) < 4 || len(prices) < len(r.Values)+r.Period {
		return false, false
	}

	// 获取最近4个周期的数据进行简单背离检测
	recentRSI := r.GetLatestN(4)
	recentPrices := prices[len(prices)-4:]

	// 检查看涨背离：价格创新低，但RSI没有创新低
	if recentPrices[3] < recentPrices[1] && recentRSI[3] > recentRSI[1] {
		return true, true // 看涨背离
	}

	// 检查看跌背离：价格创新高，但RSI没有创新高
	if recentPrices[3] > recentPrices[1] && recentRSI[3] < recentRSI[1] {
		return true, false // 看跌背离
	}

	return false, false
}

// GetStrength 获取RSI强度描述
func (r *RSIResult) GetStrength() string {
	if len(r.Values) == 0 {
		return "无数据"
	}

	rsi := r.GetLatest()

	switch {
	case rsi >= 80:
		return "极度超买"
	case rsi >= 70:
		return "超买"
	case rsi >= 60:
		return "强势"
	case rsi >= 40:
		return "中性"
	case rsi >= 30:
		return "弱势"
	case rsi >= 20:
		return "超卖"
	default:
		return "极度超卖"
	}
}

// RSISignalToString 将RSI信号转换为字符串
func RSISignalToString(signal RSISignal) string {
	switch signal {
	case RSIBuy:
		return "买入信号"
	case RSISell:
		return "卖出信号"
	default:
		return "中性"
	}
}
