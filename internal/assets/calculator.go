package assets

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

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
// 例如：CalculateRate("BTC", "ETH", "USDT") 计算 BTC/ETH 的汇率
func (rc *RateCalculator) CalculateRate(ctx context.Context, baseSymbol, quoteSymbol, bridgeCurrency string, interval string, limit int) ([]*binance.KlineData, error) {
	log.Printf("计算 %s/%s 汇率，通过 %s 桥接", baseSymbol, quoteSymbol, bridgeCurrency)

	// 获取基础币种对桥接货币的价格 (如 BTC/USDT)
	basePair := baseSymbol + bridgeCurrency
	baseKlines, err := rc.client.GetKlines(ctx, basePair, interval, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s klines: %w", basePair, err)
	}

	// 获取报价币种对桥接货币的价格 (如 ETH/USDT)
	quotePair := quoteSymbol + bridgeCurrency
	quoteKlines, err := rc.client.GetKlines(ctx, quotePair, interval, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s klines: %w", quotePair, err)
	}

	// 确保两个数据集有相同的长度
	minLength := len(baseKlines)
	if len(quoteKlines) < minLength {
		minLength = len(quoteKlines)
	}

	if minLength == 0 {
		return nil, fmt.Errorf("no kline data available for rate calculation")
	}

	// 计算汇率 K线数据
	rateKlines := make([]*binance.KlineData, minLength)

	for i := 0; i < minLength; i++ {
		baseK := baseKlines[i]
		quoteK := quoteKlines[i]

		// 验证时间戳是否匹配（允许一定误差）
		timeDiff := baseK.OpenTime.Sub(quoteK.OpenTime)
		if timeDiff < 0 {
			timeDiff = -timeDiff
		}
		if timeDiff > time.Minute { // 1分钟误差
			log.Printf("警告: 时间戳不匹配 %s vs %s，跳过此数据点",
				baseK.OpenTime.Format("2006-01-02 15:04:05"),
				quoteK.OpenTime.Format("2006-01-02 15:04:05"))
			continue
		}

		// 计算汇率: base/quote = (base/bridge) / (quote/bridge)
		rateKlines[i] = &binance.KlineData{
			Symbol:    baseSymbol + quoteSymbol, // 生成的汇率对符号
			Interval:  baseK.Interval,
			OpenTime:  baseK.OpenTime,
			CloseTime: baseK.CloseTime,
			Open:      safeDiv(baseK.Open, quoteK.Open),
			High:      safeDiv(baseK.High, quoteK.Low), // 最高点：base最高/quote最低
			Low:       safeDiv(baseK.Low, quoteK.High), // 最低点：base最低/quote最高
			Close:     safeDiv(baseK.Close, quoteK.Close),
			Volume:    0, // 计算的汇率对没有实际交易量
		}
	}

	// 过滤掉无效的数据点
	validKlines := make([]*binance.KlineData, 0, len(rateKlines))
	for _, k := range rateKlines {
		if k != nil && k.Open > 0 && k.Close > 0 {
			validKlines = append(validKlines, k)
		}
	}

	if len(validKlines) == 0 {
		return nil, fmt.Errorf("no valid rate data could be calculated")
	}

	log.Printf("成功计算 %s/%s 汇率，生成 %d 个数据点", baseSymbol, quoteSymbol, len(validKlines))
	return validKlines, nil
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
