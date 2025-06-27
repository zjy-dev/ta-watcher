package assets

import (
	"context"
	"fmt"
	"log"
	"math"

	"ta-watcher/internal/binance"
)

// RateCalculator 汇率计算器
type RateCalculator struct {
	client binance.DataSource
}

// NewRateCalculator 创建新的汇率计算器
func NewRateCalculator(client binance.DataSource) *RateCalculator {
	return &RateCalculator{
		client: client,
	}
}

// CalculateRate 计算两个币种之间的汇率
// 例如：CalculateRate("ETH", "BTC", "USDT") 计算 ETH/BTC 的汇率
// 意思是：1 ETH = ? BTC（用BTC报价ETH）
func (rc *RateCalculator) CalculateRate(ctx context.Context, baseSymbol, quoteSymbol, bridgeCurrency string, interval string, limit int) ([]*binance.KlineData, error) {
	log.Printf("计算 %s/%s 汇率，通过 %s 桥接", baseSymbol, quoteSymbol, bridgeCurrency)

	// 为了确保技术指标计算的准确性，我们需要更多的数据
	// 对于RSI-14，我们至少需要15个数据点，但为了计算稳定，我们获取更多
	requestLimit := limit
	if requestLimit < 200 {
		requestLimit = 200 // 确保获取足够的数据
	}

	// 获取基础币种对桥接货币的价格 (如 ETH/USDT)
	basePair := baseSymbol + bridgeCurrency
	baseKlines, err := rc.client.GetKlines(ctx, basePair, interval, requestLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s klines: %w", basePair, err)
	}

	// 获取报价币种对桥接货币的价格 (如 BTC/USDT)
	quotePair := quoteSymbol + bridgeCurrency
	quoteKlines, err := rc.client.GetKlines(ctx, quotePair, interval, requestLimit)
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
	// 对于RSI-14，我们至少需要15个数据点，但为了稳定性，建议30+
	if minLength < 30 {
		return nil, fmt.Errorf("insufficient kline data for rate calculation: need at least 30 data points, got %d", minLength)
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
func (rc *RateCalculator) alignKlinesByTime(baseKlines, quoteKlines []*binance.KlineData, baseSymbol, quoteSymbol string) ([]*binance.KlineData, error) {
	// 创建时间戳到K线的映射
	baseMap := make(map[int64]*binance.KlineData)
	quoteMap := make(map[int64]*binance.KlineData)

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
	rateKlines := make([]*binance.KlineData, 0, len(commonTimestamps))

	for _, timestamp := range commonTimestamps {
		baseK := baseMap[timestamp]
		quoteK := quoteMap[timestamp]

		// 计算汇率: base/quote = (base/bridge) / (quote/bridge)
		rateKline := &binance.KlineData{
			Symbol:    baseSymbol + quoteSymbol,
			Interval:  baseK.Interval,
			OpenTime:  baseK.OpenTime,
			CloseTime: baseK.CloseTime,
			Open:      safeDiv(baseK.Open, quoteK.Open),
			High:      safeDiv(baseK.High, quoteK.High), // 修正：base最高/quote最高
			Low:       safeDiv(baseK.Low, quoteK.Low),   // 修正：base最低/quote最低
			Close:     safeDiv(baseK.Close, quoteK.Close),
			Volume:    0, // 计算的汇率对没有实际交易量
		}

		// 验证数据有效性
		if rateKline.Open > 0 && rateKline.Close > 0 && rateKline.High > 0 && rateKline.Low > 0 {
			// 确保High >= Low，Open和Close在合理范围内
			if rateKline.High >= rateKline.Low {
				rateKlines = append(rateKlines, rateKline)
			}
		}
	}

	skippedCount := len(commonTimestamps) - len(rateKlines)
	if skippedCount > 0 {
		log.Printf("计算 %s/%s 汇率时跳过 %d 个无效数据点", baseSymbol, quoteSymbol, skippedCount)
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
		_, err := rc.client.GetKlines(ctx, pair, "1d", 1)
		if err != nil {
			log.Printf("币种 %s 对 %s 的交易对不存在: %v", symbol, bridgeCurrency, err)
			unavailableSymbols = append(unavailableSymbols, symbol)
		} else {
			availableSymbols = append(availableSymbols, symbol)
		}
	}

	return availableSymbols, unavailableSymbols, nil
}
