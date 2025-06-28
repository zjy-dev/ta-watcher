package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"
)

// CoinbaseClient Coinbase数据源实现
type CoinbaseClient struct {
	baseURL string
	client  *http.Client
}

// NewCoinbaseClient 创建Coinbase客户端
func NewCoinbaseClient() *CoinbaseClient {
	return &CoinbaseClient{
		baseURL: "https://api.exchange.coinbase.com",
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Name 返回数据源名称
func (c *CoinbaseClient) Name() string {
	return "coinbase"
}

// IsSymbolValid 检查交易对是否有效
func (c *CoinbaseClient) IsSymbolValid(ctx context.Context, symbol string) (bool, error) {
	// 转换为Coinbase格式 (BTCUSDT -> BTC-USDT)
	coinbaseSymbol := c.convertToCoinbaseSymbol(symbol)

	url := fmt.Sprintf("%s/products/%s/ticker", c.baseURL, coinbaseSymbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// GetKlines 获取K线数据（支持分页和聚合）
func (c *CoinbaseClient) GetKlines(ctx context.Context, symbol string, timeframe Timeframe, startTime, endTime time.Time, limit int) ([]*Kline, error) {
	if limit <= 0 {
		limit = 300
	}

	// 转换为Coinbase格式
	coinbaseSymbol := c.convertToCoinbaseSymbol(symbol)
	granularity := c.convertTimeframeToGranularity(timeframe)

	if granularity == 0 {
		return nil, fmt.Errorf("unsupported timeframe: %s", timeframe)
	}

	// 对于日线以上的时间框架，可能需要获取更多数据进行聚合
	baseGranularity := granularity
	if timeframe == Timeframe1w || timeframe == Timeframe1M {
		baseGranularity = 86400 // 使用日线数据进行聚合
	}

	var allKlines []*Kline
	batchSize := 300 // Coinbase API限制

	// 分批获取数据
	for currentEnd := endTime; currentEnd.After(startTime); {
		currentStart := currentEnd.Add(-time.Duration(batchSize) * time.Second * time.Duration(baseGranularity))
		if currentStart.Before(startTime) {
			currentStart = startTime
		}

		klines, err := c.fetchKlinesBatch(ctx, coinbaseSymbol, baseGranularity, currentStart, currentEnd)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch batch: %w", err)
		}

		// 转换为标准格式
		for _, raw := range klines {
			kline, err := c.parseCandle(symbol, raw)
			if err != nil {
				continue // 跳过无法解析的数据
			}
			allKlines = append(allKlines, kline)
		}

		currentEnd = currentStart

		// 避免无限循环
		if len(klines) == 0 {
			break
		}
	}

	// 按时间排序（Coinbase返回的数据可能是倒序）
	sortKlinesByTime(allKlines)

	// 如果需要聚合（周线、月线）
	if timeframe == Timeframe1w || timeframe == Timeframe1M {
		allKlines = c.aggregateKlines(allKlines, timeframe)
	}

	// 限制返回数量
	if len(allKlines) > limit {
		allKlines = allKlines[len(allKlines)-limit:]
	}

	return allKlines, nil
}

// parseCandle 解析Coinbase蜡烛图数据
// Coinbase格式: [timestamp, low, high, open, close, volume]
func (c *CoinbaseClient) parseCandle(symbol string, raw []float64) (*Kline, error) {
	if len(raw) < 6 {
		return nil, fmt.Errorf("invalid candle data length: %d", len(raw))
	}

	timestamp := time.Unix(int64(raw[0]), 0)

	return &Kline{
		Symbol:    symbol,
		OpenTime:  timestamp,
		CloseTime: timestamp.Add(time.Minute), // 简化处理，实际应根据timeframe计算
		Open:      raw[3],
		High:      raw[2],
		Low:       raw[1],
		Close:     raw[4],
		Volume:    raw[5],
	}, nil
}

// convertToCoinbaseSymbol 转换为Coinbase交易对格式
// BTCUSDT -> BTC-USDT
func (c *CoinbaseClient) convertToCoinbaseSymbol(symbol string) string {
	// 简化处理，假设都是对USDT或USD的交易对
	if len(symbol) >= 6 {
		if symbol[len(symbol)-4:] == "USDT" {
			base := symbol[:len(symbol)-4]
			return base + "-USDT"
		} else if symbol[len(symbol)-3:] == "USD" {
			base := symbol[:len(symbol)-3]
			return base + "-USD"
		}
	}

	// 默认处理：假设最后3个字符是quote currency
	if len(symbol) > 3 {
		base := symbol[:len(symbol)-3]
		quote := symbol[len(symbol)-3:]
		return base + "-" + quote
	}

	return symbol
}

// convertTimeframeToGranularity 转换时间框架为Coinbase粒度（秒）
func (c *CoinbaseClient) convertTimeframeToGranularity(tf Timeframe) int {
	switch tf {
	case Timeframe1m:
		return 60
	case Timeframe5m:
		return 300
	case Timeframe15m:
		return 900
	case Timeframe1h:
		return 3600
	case Timeframe6h:
		return 21600
	case Timeframe1d:
		return 86400
	case Timeframe1w, Timeframe1M:
		return 86400 // 使用日线数据进行聚合
	default:
		return 0 // 不支持的时间框架
	}
}

// fetchKlinesBatch 获取单批K线数据
func (c *CoinbaseClient) fetchKlinesBatch(ctx context.Context, coinbaseSymbol string, granularity int, startTime, endTime time.Time) ([][]float64, error) {
	url := fmt.Sprintf("%s/products/%s/candles", c.baseURL, coinbaseSymbol)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("granularity", strconv.Itoa(granularity))

	if !startTime.IsZero() {
		q.Add("start", startTime.Format(time.RFC3339))
	}
	if !endTime.IsZero() {
		q.Add("end", endTime.Format(time.RFC3339))
	}

	req.URL.RawQuery = q.Encode()

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("coinbase API returned status: %d", resp.StatusCode)
	}

	var rawCandles [][]float64
	if err := json.NewDecoder(resp.Body).Decode(&rawCandles); err != nil {
		return nil, err
	}

	return rawCandles, nil
}

// sortKlinesByTime 按时间排序K线数据
func sortKlinesByTime(klines []*Kline) {
	sort.Slice(klines, func(i, j int) bool {
		return klines[i].OpenTime.Before(klines[j].OpenTime)
	})
}

// aggregateKlines 聚合K线数据（日线->周线/月线）
func (c *CoinbaseClient) aggregateKlines(dailyKlines []*Kline, targetTimeframe Timeframe) []*Kline {
	if len(dailyKlines) == 0 {
		return dailyKlines
	}

	var aggregated []*Kline
	var currentPeriod []*Kline

	for _, kline := range dailyKlines {
		// 判断是否需要开始新的聚合周期
		if len(currentPeriod) == 0 {
			currentPeriod = append(currentPeriod, kline)
			continue
		}

		lastKline := currentPeriod[len(currentPeriod)-1]
		shouldStartNew := false

		switch targetTimeframe {
		case Timeframe1w:
			// 周线：周一开始新周期
			if kline.OpenTime.Weekday() == time.Monday && kline.OpenTime.After(lastKline.OpenTime.Add(6*24*time.Hour)) {
				shouldStartNew = true
			}
		case Timeframe1M:
			// 月线：月初开始新周期
			if kline.OpenTime.Day() == 1 && kline.OpenTime.Month() != lastKline.OpenTime.Month() {
				shouldStartNew = true
			}
		}

		if shouldStartNew {
			// 聚合当前周期并开始新周期
			if len(currentPeriod) > 0 {
				aggregated = append(aggregated, c.aggregatePeriod(currentPeriod))
			}
			currentPeriod = []*Kline{kline}
		} else {
			currentPeriod = append(currentPeriod, kline)
		}
	}

	// 聚合最后一个周期
	if len(currentPeriod) > 0 {
		aggregated = append(aggregated, c.aggregatePeriod(currentPeriod))
	}

	return aggregated
}

// aggregatePeriod 聚合一个周期的K线数据
func (c *CoinbaseClient) aggregatePeriod(klines []*Kline) *Kline {
	if len(klines) == 0 {
		return nil
	}

	first := klines[0]
	last := klines[len(klines)-1]

	aggregated := &Kline{
		Symbol:    first.Symbol,
		OpenTime:  first.OpenTime,
		CloseTime: last.CloseTime,
		Open:      first.Open,
		Close:     last.Close,
		High:      first.High,
		Low:       first.Low,
		Volume:    0,
	}

	// 计算最高价、最低价和总成交量
	for _, kline := range klines {
		if kline.High > aggregated.High {
			aggregated.High = kline.High
		}
		if kline.Low < aggregated.Low {
			aggregated.Low = kline.Low
		}
		aggregated.Volume += kline.Volume
	}

	return aggregated
}
