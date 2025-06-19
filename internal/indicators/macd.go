package indicators

import (
	"errors"
	"math"
)

// MACDResult MACD指标计算结果
type MACDResult struct {
	MACD         []float64 // MACD线：快线EMA - 慢线EMA
	Signal       []float64 // 信号线：MACD的EMA
	Histogram    []float64 // 柱状图：MACD - Signal
	FastPeriod   int       // 快线周期
	SlowPeriod   int       // 慢线周期
	SignalPeriod int       // 信号线周期
}

// MACDSignal MACD信号类型
type MACDSignal int

const (
	MACDNeutral MACDSignal = iota // 中性
	MACDBuy                       // 买入信号
	MACDSell                      // 卖出信号
)

// MACD默认参数
const (
	DefaultFastPeriod   = 12 // 默认快线周期
	DefaultSlowPeriod   = 26 // 默认慢线周期
	DefaultSignalPeriod = 9  // 默认信号线周期
)

// CalculateMACD 计算MACD指标
// prices: 价格序列（通常是收盘价）
// fastPeriod: 快线EMA周期，通常为12
// slowPeriod: 慢线EMA周期，通常为26
// signalPeriod: 信号线EMA周期，通常为9
func CalculateMACD(prices []float64, fastPeriod, slowPeriod, signalPeriod int) (*MACDResult, error) {
	if len(prices) < slowPeriod {
		return nil, errors.New("价格数据不足，无法计算MACD指标")
	}

	if fastPeriod <= 0 || slowPeriod <= 0 || signalPeriod <= 0 {
		return nil, errors.New("MACD周期参数必须大于0")
	}

	if fastPeriod >= slowPeriod {
		return nil, errors.New("快线周期必须小于慢线周期")
	}

	// 计算快线和慢线EMA
	fastEMA, err := CalculateEMA(prices, fastPeriod)
	if err != nil {
		return nil, err
	}

	slowEMA, err := CalculateEMA(prices, slowPeriod)
	if err != nil {
		return nil, err
	}

	// 由于慢线周期更长，需要对齐数据
	// 慢线EMA的第一个值对应的是第slowPeriod个价格
	// 快线EMA的第一个值对应的是第fastPeriod个价格
	// 所以需要从慢线开始的位置开始计算MACD

	alignOffset := slowPeriod - fastPeriod
	var macdLine []float64

	// 计算MACD线（快线EMA - 慢线EMA）
	for i := 0; i < len(slowEMA.Values); i++ {
		fastValue := fastEMA.Values[i+alignOffset]
		slowValue := slowEMA.Values[i]
		macdLine = append(macdLine, fastValue-slowValue)
	}

	// 计算信号线（MACD的EMA）
	signalEMA, err := CalculateEMA(macdLine, signalPeriod)
	if err != nil {
		return nil, err
	}

	// 计算柱状图（MACD - Signal）
	// 需要再次对齐，因为信号线又比MACD线短了
	var histogram []float64
	signalOffset := signalPeriod - 1

	for i := 0; i < len(signalEMA.Values); i++ {
		macdValue := macdLine[i+signalOffset]
		signalValue := signalEMA.Values[i]
		histogram = append(histogram, macdValue-signalValue)
	}

	// 对齐所有序列到相同长度（以最短的为准）
	finalLength := len(histogram)
	finalMACDOffset := len(macdLine) - finalLength
	finalSignalOffset := len(signalEMA.Values) - finalLength

	return &MACDResult{
		MACD:         macdLine[finalMACDOffset:],
		Signal:       signalEMA.Values[finalSignalOffset:],
		Histogram:    histogram,
		FastPeriod:   fastPeriod,
		SlowPeriod:   slowPeriod,
		SignalPeriod: signalPeriod,
	}, nil
}

// CalculateDefaultMACD 使用默认参数计算MACD
func CalculateDefaultMACD(prices []float64) (*MACDResult, error) {
	return CalculateMACD(prices, DefaultFastPeriod, DefaultSlowPeriod, DefaultSignalPeriod)
}

// GetLatest 获取最新的MACD值
func (m *MACDResult) GetLatest() (macd, signal, histogram float64) {
	if len(m.MACD) == 0 {
		return 0, 0, 0
	}

	idx := len(m.MACD) - 1
	return m.MACD[idx], m.Signal[idx], m.Histogram[idx]
}

// GetLatestN 获取最新的N个MACD值
func (m *MACDResult) GetLatestN(n int) (macd, signal, histogram []float64) {
	if n <= 0 || len(m.MACD) == 0 {
		return []float64{}, []float64{}, []float64{}
	}

	start := int(math.Max(0, float64(len(m.MACD)-n)))
	return m.MACD[start:], m.Signal[start:], m.Histogram[start:]
}

// GetSignal 获取MACD交易信号
func (m *MACDResult) GetSignal() MACDSignal {
	if len(m.MACD) < 2 || len(m.Signal) < 2 {
		return MACDNeutral
	}

	// 当前值
	currentMACD := m.MACD[len(m.MACD)-1]
	currentSignal := m.Signal[len(m.Signal)-1]

	// 前一个值
	prevMACD := m.MACD[len(m.MACD)-2]
	prevSignal := m.Signal[len(m.Signal)-2]

	// 金叉：MACD线上穿信号线
	if currentMACD > currentSignal && prevMACD <= prevSignal {
		return MACDBuy
	}

	// 死叉：MACD线下穿信号线
	if currentMACD < currentSignal && prevMACD >= prevSignal {
		return MACDSell
	}

	return MACDNeutral
}

// IsGoldenCross 检查MACD是否发生金叉
func (m *MACDResult) IsGoldenCross() bool {
	return m.GetSignal() == MACDBuy
}

// IsDeathCross 检查MACD是否发生死叉
func (m *MACDResult) IsDeathCross() bool {
	return m.GetSignal() == MACDSell
}

// GetTrend 获取MACD趋势
func (m *MACDResult) GetTrend() string {
	if len(m.MACD) == 0 {
		return "无数据"
	}

	macd, signal, histogram := m.GetLatest()

	if macd > signal {
		if histogram > 0 {
			return "强势上涨"
		} else {
			return "上涨减弱"
		}
	} else {
		if histogram < 0 {
			return "强势下跌"
		} else {
			return "下跌减弱"
		}
	}
}

// IsDivergence 检测MACD背离
// prices: 对应的价格序列
// 返回：是否存在背离，背离类型（看涨/看跌）
func (m *MACDResult) IsDivergence(prices []float64) (bool, bool) {
	if len(m.MACD) < 4 || len(prices) < len(m.MACD)+m.SlowPeriod {
		return false, false
	}

	// 获取最近的数据
	recentMACD := m.MACD[len(m.MACD)-4:]
	recentPrices := prices[len(prices)-4:]

	// 检查看涨背离：价格创新低，但MACD没有创新低
	if recentPrices[3] < recentPrices[1] && recentMACD[3] > recentMACD[1] {
		return true, true // 看涨背离
	}

	// 检查看跌背离：价格创新高，但MACD没有创新高
	if recentPrices[3] > recentPrices[1] && recentMACD[3] < recentMACD[1] {
		return true, false // 看跌背离
	}

	return false, false
}

// GetHistogramTrend 获取柱状图趋势
func (m *MACDResult) GetHistogramTrend() string {
	if len(m.Histogram) < 2 {
		return "无数据"
	}

	current := m.Histogram[len(m.Histogram)-1]
	prev := m.Histogram[len(m.Histogram)-2]

	if current > prev {
		if current > 0 {
			return "动能增强"
		} else {
			return "跌势减弱"
		}
	} else {
		if current < 0 {
			return "动能减弱"
		} else {
			return "涨势减弱"
		}
	}
}

// MACDSignalToString 将MACD信号转换为字符串
func MACDSignalToString(signal MACDSignal) string {
	switch signal {
	case MACDBuy:
		return "买入信号（金叉）"
	case MACDSell:
		return "卖出信号（死叉）"
	default:
		return "中性"
	}
}
