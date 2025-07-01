package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"ta-watcher/internal/config"
)

// CoinbaseClient Coinbase数据源实现
type CoinbaseClient struct {
	baseURL      string
	client       *http.Client
	rateLimit    *config.RateLimitConfig
	lastRequest  time.Time
	requestMutex sync.Mutex
}

// NewCoinbaseClient 创建Coinbase客户端
func NewCoinbaseClient() *CoinbaseClient {
	return &CoinbaseClient{
		baseURL: "https://api.exchange.coinbase.com",
		client:  &http.Client{Timeout: 30 * time.Second},
		rateLimit: &config.RateLimitConfig{
			RequestsPerMinute: 300, // 默认限流：每分钟300请求
			RetryDelay:        5 * time.Second,
			MaxRetries:        3,
		},
	}
}

// NewCoinbaseClientWithConfig 使用配置创建Coinbase客户端
func NewCoinbaseClientWithConfig(cfg *config.CoinbaseConfig) *CoinbaseClient {
	client := &CoinbaseClient{
		baseURL: "https://api.exchange.coinbase.com",
		client:  &http.Client{Timeout: 30 * time.Second},
	}

	if cfg != nil {
		client.rateLimit = &cfg.RateLimit
	} else {
		client.rateLimit = &config.RateLimitConfig{
			RequestsPerMinute: 300,
			RetryDelay:        5 * time.Second,
			MaxRetries:        3,
		}
	}

	return client
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

	resp, err := c.executeWithRateLimit(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	valid := resp.StatusCode == http.StatusOK
	return valid, nil
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

	// 设置默认时间范围
	if startTime.IsZero() || endTime.IsZero() {
		now := time.Now()
		if endTime.IsZero() {
			endTime = now
		}
		if startTime.IsZero() {
			// 根据请求的数据量设置开始时间
			duration := time.Duration(limit*baseGranularity) * time.Second
			startTime = endTime.Add(-duration)
		}
	}

	var allKlines []*Kline
	batchSize := 300 // Coinbase API限制
	batchCount := 0

	// 分批获取数据
	for currentEnd := endTime; currentEnd.After(startTime); {
		batchCount++
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

	if len(allKlines) > 0 {
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

	resp, err := c.executeWithRateLimit(req)
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
			// 周线：计算周开始日期（以周一为起始）
			lastWeekStart := getWeekStart(lastKline.OpenTime)
			currentWeekStart := getWeekStart(kline.OpenTime)
			if !lastWeekStart.Equal(currentWeekStart) {
				shouldStartNew = true
			}
		case Timeframe1M:
			// 月线：不同月份开始新周期
			if kline.OpenTime.Year() != lastKline.OpenTime.Year() ||
				kline.OpenTime.Month() != lastKline.OpenTime.Month() {
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

// getWeekStart 获取给定时间所在周的周一
func getWeekStart(t time.Time) time.Time {
	// Go中 Sunday=0, Monday=1, ..., Saturday=6
	weekday := int(t.Weekday())
	if weekday == 0 { // Sunday
		weekday = 7 // 转换为 Monday=1, ..., Sunday=7
	}
	// 计算到周一的天数差
	daysFromMonday := weekday - 1
	// 返回周一的日期（保持时间部分）
	return t.AddDate(0, 0, -daysFromMonday)
}

// rateLimitSleep 根据限流配置进行休眠
func (c *CoinbaseClient) rateLimitSleep() {
	c.requestMutex.Lock()
	defer c.requestMutex.Unlock()

	if c.rateLimit.RequestsPerMinute <= 0 {
		return // 没有限流
	}

	// 计算每个请求之间的最小间隔
	minInterval := time.Minute / time.Duration(c.rateLimit.RequestsPerMinute)

	// 计算距离上次请求的时间
	elapsed := time.Since(c.lastRequest)

	// 如果还没有达到最小间隔，则等待
	if elapsed < minInterval {
		sleepTime := minInterval - elapsed
		time.Sleep(sleepTime)
	}

	c.lastRequest = time.Now()
}

// executeWithRateLimit 执行带限流的HTTP请求
func (c *CoinbaseClient) executeWithRateLimit(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt <= c.rateLimit.MaxRetries; attempt++ {
		// 应用限流
		c.rateLimitSleep()

		// 执行请求
		resp, err = c.client.Do(req)

		// 如果成功或者非限流错误，直接返回
		if err == nil && resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}

		// 如果是限流错误，等待更长时间再重试
		if resp != nil && resp.StatusCode == http.StatusTooManyRequests {
			if attempt < c.rateLimit.MaxRetries {
				time.Sleep(c.rateLimit.RetryDelay)
			}
			resp.Body.Close()
			continue
		}

		// 其他错误，等待重试延迟
		if attempt < c.rateLimit.MaxRetries {
			time.Sleep(c.rateLimit.RetryDelay)
		}
	}

	return resp, err
}
