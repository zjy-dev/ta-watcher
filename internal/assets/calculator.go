package assets

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"ta-watcher/internal/datasource"
)

// 汇率计算的最小数据点数
// 汇率计算需要更多数据点来确保准确性和稳定性
const MinRateCalculationDataPoints = 30

// RateCalculator 汇率计算器
type RateCalculator struct {
	client datasource.DataSource
}

// NewRateCalculator 创建新的汇率计算器
func NewRateCalculator(client datasource.DataSource) *RateCalculator {
	return &RateCalculator{
		client: client,
	}
}

// CalculateRate 计算两个币种之间的汇率
// 例如：CalculateRate("ETH", "BTC", "USDT") 计算 ETH/BTC 的汇率
// 意思是：1 ETH = ? BTC（用BTC报价ETH）
func (rc *RateCalculator) CalculateRate(ctx context.Context, baseSymbol, quoteSymbol, bridgeCurrency string, interval datasource.Timeframe, limit int) ([]*datasource.Kline, error) {
	log.Printf("计算 %s/%s 汇率，通过 %s 桥接", baseSymbol, quoteSymbol, bridgeCurrency)

	// 为了确保技术指标计算的准确性，我们需要更多的数据
	// 汇率计算需要足够的数据点来确保稳定性
	requestLimit := limit
	if requestLimit < MinRateCalculationDataPoints {
		requestLimit = MinRateCalculationDataPoints
	}

	log.Printf("计算 %s/%s 汇率，通过 %s 桥接，需要 %d 个数据点", baseSymbol, quoteSymbol, bridgeCurrency, requestLimit)

	// 获取基础币种对桥接货币的价格 (如 ETH/USDT)
	basePair := baseSymbol + bridgeCurrency
	baseKlines, err := rc.client.GetKlines(ctx, basePair, interval, time.Time{}, time.Time{}, requestLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s klines: %w", basePair, err)
	}

	// 获取报价币种对桥接货币的价格 (如 BTC/USDT)
	quotePair := quoteSymbol + bridgeCurrency
	quoteKlines, err := rc.client.GetKlines(ctx, quotePair, interval, time.Time{}, time.Time{}, requestLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s klines: %w", quotePair, err)
	}

	// 检查数据是否足够
	if len(baseKlines) == 0 || len(quoteKlines) == 0 {
		return nil, fmt.Errorf("no kline data available for rate calculation")
	}

	// 取两个币种中数据较少的那个作为基准
	minLength := len(baseKlines)
	if len(quoteKlines) < minLength {
		minLength = len(quoteKlines)
	}

	// 检查是否有足够的数据来计算技术指标
	// 汇率计算需要足够的数据点来保证准确性
	if minLength < MinRateCalculationDataPoints {
		return nil, fmt.Errorf("insufficient kline data for rate calculation: need at least %d data points, got %d", MinRateCalculationDataPoints, minLength)
	}

	// 按时间戳对齐数据
	alignedData, err := rc.alignKlinesByTime(baseKlines, quoteKlines, baseSymbol, quoteSymbol)
	if err != nil {
		return nil, fmt.Errorf("failed to align klines by time: %w", err)
	}

	// 检查对齐后的数据是否足够
	if len(alignedData) < 30 {
		return nil, fmt.Errorf("insufficient aligned kline data: need at least 30 data points, got %d after alignment", len(alignedData))
	}

	// 限制返回的数据量
	if len(alignedData) > limit {
		alignedData = alignedData[len(alignedData)-limit:]
	}

	log.Printf("成功计算 %s/%s 汇率，生成 %d 个数据点", baseSymbol, quoteSymbol, len(alignedData))
	return alignedData, nil
}

// alignKlinesByTime 按时间戳对齐两个K线数据集并计算汇率
func (rc *RateCalculator) alignKlinesByTime(baseKlines, quoteKlines []*datasource.Kline, baseSymbol, quoteSymbol string) ([]*datasource.Kline, error) {
	// 创建时间戳到K线的映射
	baseMap := make(map[int64]*datasource.Kline)
	quoteMap := make(map[int64]*datasource.Kline)

	for _, kline := range baseKlines {
		timestamp := kline.OpenTime.Unix()
		baseMap[timestamp] = kline
	}

	for _, kline := range quoteKlines {
		timestamp := kline.OpenTime.Unix()
		quoteMap[timestamp] = kline
	}

	// 找到共同的时间戳
	var commonTimestamps []int64
	for timestamp := range baseMap {
		if _, exists := quoteMap[timestamp]; exists {
			commonTimestamps = append(commonTimestamps, timestamp)
		}
	}

	log.Printf("时间戳对齐：共同时间戳%d个", len(commonTimestamps))

	if len(commonTimestamps) == 0 {
		return nil, fmt.Errorf("no matching timestamps found between %s and %s", baseSymbol, quoteSymbol)
	}

	// 按时间排序
	for i := 0; i < len(commonTimestamps)-1; i++ {
		for j := i + 1; j < len(commonTimestamps); j++ {
			if commonTimestamps[i] > commonTimestamps[j] {
				commonTimestamps[i], commonTimestamps[j] = commonTimestamps[j], commonTimestamps[i]
			}
		}
	}

	// 计算汇率K线数据
	rateKlines := make([]*datasource.Kline, 0, len(commonTimestamps))

	for _, timestamp := range commonTimestamps {
		baseK := baseMap[timestamp]
		quoteK := quoteMap[timestamp]

		// 计算汇率: base/quote = (base/bridge) / (quote/bridge)
		// 对于OHLC，我们需要正确计算汇率的高低点
		open := safeDiv(baseK.Open, quoteK.Open)
		close := safeDiv(baseK.Close, quoteK.Close)

		// 计算所有可能的汇率点，找到真正的最高和最低
		rates := []float64{
			safeDiv(baseK.Open, quoteK.Open),
			safeDiv(baseK.Open, quoteK.Close),
			safeDiv(baseK.High, quoteK.Low), // base最高/quote最低 = 汇率最高
			safeDiv(baseK.Low, quoteK.High), // base最低/quote最高 = 汇率最低
			safeDiv(baseK.Close, quoteK.Open),
			safeDiv(baseK.Close, quoteK.Close),
		}

		// 过滤有效汇率值
		validRates := make([]float64, 0)
		for _, rate := range rates {
			if rate > 0 && !math.IsNaN(rate) && !math.IsInf(rate, 0) {
				validRates = append(validRates, rate)
			}
		}

		if len(validRates) < 2 {
			continue // 跳过无效数据
		}

		// 计算真实的最高和最低汇率
		high := validRates[0]
		low := validRates[0]
		for _, rate := range validRates {
			if rate > high {
				high = rate
			}
			if rate < low {
				low = rate
			}
		}

		rateKline := &datasource.Kline{
			Symbol:    baseSymbol + quoteSymbol,
			OpenTime:  baseK.OpenTime,
			CloseTime: baseK.CloseTime,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    0, // 计算的汇率对没有实际交易量
		}

		// 验证数据有效性：确保Open、Close、High、Low都为正数且High>=Low
		if rateKline.Open <= 0 || rateKline.Close <= 0 || rateKline.High <= 0 || rateKline.Low <= 0 {
			log.Printf("跳过数据点 %s - 价格非正数: Open=%.8f, High=%.8f, Low=%.8f, Close=%.8f",
				rateKline.OpenTime.Format("2006-01-02 15:04:05"),
				rateKline.Open, rateKline.High, rateKline.Low, rateKline.Close)
			continue
		}

		// 确保High >= Low，这是K线数据的基本要求
		if rateKline.High < rateKline.Low {
			log.Printf("跳过数据点 %s - High < Low: High=%.8f, Low=%.8f",
				rateKline.OpenTime.Format("2006-01-02 15:04:05"),
				rateKline.High, rateKline.Low)
			continue
		}

		// 进一步检查汇率是否在合理范围内（避免极端异常值）
		if rateKline.High/rateKline.Low > 10.0 { // 单根K线内汇率波动不应超过10倍
			continue
		}

		rateKlines = append(rateKlines, rateKline)
	}

	skippedCount := len(commonTimestamps) - len(rateKlines)
	if skippedCount > 0 {
		log.Printf("计算 %s/%s 汇率时过滤了 %d 个异常数据点", baseSymbol, quoteSymbol, skippedCount)
	}

	return rateKlines, nil
}

// safeDiv 安全除法，避免除零
func safeDiv(a, b float64) float64 {
	if b == 0 || math.IsNaN(b) || math.IsInf(b, 0) {
		return 0
	}
	result := a / b
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return 0
	}
	return result
}

// GetAvailableRatePairs 获取可用的汇率对
// 检查哪些币种对可以通过给定的桥接货币进行汇率计算
func (rc *RateCalculator) GetAvailableRatePairs(ctx context.Context, symbols []string, bridgeCurrency string) ([]string, []string, error) {
	availableSymbols := make([]string, 0)
	unavailableSymbols := make([]string, 0)

	for _, symbol := range symbols {
		pair := symbol + bridgeCurrency
		_, err := rc.client.GetKlines(ctx, pair, datasource.Timeframe1d, time.Time{}, time.Time{}, 1)
		if err != nil {
			log.Printf("币种 %s 对 %s 的交易对不存在: %v", symbol, bridgeCurrency, err)
			unavailableSymbols = append(unavailableSymbols, symbol)
		} else {
			availableSymbols = append(availableSymbols, symbol)
		}
	}

	return availableSymbols, unavailableSymbols, nil
}
