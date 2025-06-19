package binance

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"ta-watcher/internal/config"

	"github.com/adshao/go-binance/v2"
)

// Client Binance客户端实现
type Client struct {
	config      *config.BinanceConfig
	client      *binance.Client
	rateLimiter *rateLimiter
	mu          sync.RWMutex
}

// rateLimiter 简单的限流器
type rateLimiter struct {
	requests    chan struct{}
	ticker      *time.Ticker
	maxRequests int
	mu          sync.Mutex
}

// newRateLimiter 创建限流器
func newRateLimiter(requestsPerMinute int) *rateLimiter {
	rl := &rateLimiter{
		requests:    make(chan struct{}, requestsPerMinute),
		ticker:      time.NewTicker(time.Minute),
		maxRequests: requestsPerMinute,
	}

	// 初始填充
	for i := 0; i < requestsPerMinute; i++ {
		rl.requests <- struct{}{}
	}

	// 定期补充令牌
	go func() {
		for range rl.ticker.C {
			rl.mu.Lock()
			// 清空并重新填充
			for len(rl.requests) > 0 {
				<-rl.requests
			}
			for i := 0; i < rl.maxRequests; i++ {
				select {
				case rl.requests <- struct{}{}:
				default:
				}
			}
			rl.mu.Unlock()
		}
	}()

	return rl
}

// acquire 获取令牌
func (rl *rateLimiter) acquire(ctx context.Context) error {
	select {
	case <-rl.requests:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// stop 停止限流器
func (rl *rateLimiter) stop() {
	if rl.ticker != nil {
		rl.ticker.Stop()
	}
}

// NewClient 创建新的Binance客户端
func NewClient(cfg *config.BinanceConfig) (*Client, error) {
	if cfg == nil {
		// 使用默认配置
		cfg = &config.BinanceConfig{
			RateLimit: config.RateLimitConfig{
				RequestsPerMinute: 1200,
				MaxRetries:        3,
				RetryDelay:        time.Second,
			},
		}
	}

	// 创建只读客户端（不需要API密钥）
	client := binance.NewClient("", "")

	// 创建限流器
	rateLimiter := newRateLimiter(cfg.RateLimit.RequestsPerMinute)

	c := &Client{
		config:      cfg,
		client:      client,
		rateLimiter: rateLimiter,
	}

	return c, nil
}

// Close 关闭客户端
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.rateLimiter != nil {
		c.rateLimiter.stop()
	}

	return nil
}

// withRetry 带重试的请求执行
func (c *Client) withRetry(ctx context.Context, operation func() error) error {
	var lastErr error

	for i := 0; i <= c.config.RateLimit.MaxRetries; i++ {
		// 限流
		if err := c.rateLimiter.acquire(ctx); err != nil {
			return err
		}

		// 执行操作
		if err := operation(); err != nil {
			lastErr = err

			// 判断是否应该重试
			if !shouldRetry(err) {
				return err
			}

			// 等待后重试
			if i < c.config.RateLimit.MaxRetries {
				select {
				case <-time.After(c.config.RateLimit.RetryDelay):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			continue
		}

		return nil
	}

	return fmt.Errorf("operation failed after %d retries: %w", c.config.RateLimit.MaxRetries, lastErr)
}

// shouldRetry 判断错误是否应该重试
func shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// 网络相关错误可以重试
	retryableErrors := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"temporary failure",
		"rate limit",
		"too many requests",
		"internal server error",
		"bad gateway",
		"service unavailable",
		"gateway timeout",
	}

	for _, retryable := range retryableErrors {
		if contains(errStr, retryable) {
			return true
		}
	}

	return false
}

// contains 检查字符串是否包含子字符串（忽略大小写）
func contains(s, substr string) bool {
	// 转换为小写进行比较
	sLower := strings.ToLower(s)
	substrLower := strings.ToLower(substr)
	return strings.Contains(sLower, substrLower)
}

// GetPrice 获取单个交易对的当前价格
func (c *Client) GetPrice(ctx context.Context, symbol string) (*PriceData, error) {
	if !IsValidSymbol(symbol) {
		return nil, ErrInvalidSymbol
	}

	var priceData *PriceData
	var err error

	operation := func() error {
		price, opErr := c.client.NewListPricesService().Symbol(symbol).Do(ctx)
		if opErr != nil {
			return opErr
		}

		if len(price) == 0 {
			return fmt.Errorf("no price data for symbol %s", symbol)
		}

		priceFloat, parseErr := strconv.ParseFloat(price[0].Price, 64)
		if parseErr != nil {
			return fmt.Errorf("failed to parse price: %w", parseErr)
		}

		priceData = &PriceData{
			Symbol:    symbol,
			Price:     priceFloat,
			Timestamp: time.Now(),
		}

		return nil
	}

	if err = c.withRetry(ctx, operation); err != nil {
		return nil, fmt.Errorf("failed to get price for %s: %w", symbol, err)
	}

	return priceData, nil
}

// GetPrices 获取多个交易对的当前价格
func (c *Client) GetPrices(ctx context.Context, symbols []string) ([]*PriceData, error) {
	if len(symbols) == 0 {
		return nil, fmt.Errorf("no symbols provided")
	}

	// 验证所有交易对
	for _, symbol := range symbols {
		if !IsValidSymbol(symbol) {
			return nil, fmt.Errorf("invalid symbol: %s", symbol)
		}
	}

	var priceDataList []*PriceData
	var err error

	operation := func() error {
		prices, opErr := c.client.NewListPricesService().Do(ctx)
		if opErr != nil {
			return opErr
		}

		// 创建symbol映射以提高查找效率
		symbolMap := make(map[string]bool)
		for _, symbol := range symbols {
			symbolMap[symbol] = true
		}

		priceDataList = make([]*PriceData, 0, len(symbols))
		timestamp := time.Now()

		for _, price := range prices {
			if symbolMap[price.Symbol] {
				priceFloat, parseErr := strconv.ParseFloat(price.Price, 64)
				if parseErr != nil {
					return fmt.Errorf("failed to parse price for %s: %w", price.Symbol, parseErr)
				}

				priceDataList = append(priceDataList, &PriceData{
					Symbol:    price.Symbol,
					Price:     priceFloat,
					Timestamp: timestamp,
				})
			}
		}

		return nil
	}

	if err = c.withRetry(ctx, operation); err != nil {
		return nil, fmt.Errorf("failed to get prices: %w", err)
	}

	return priceDataList, nil
}

// GetKlines 获取K线数据
func (c *Client) GetKlines(ctx context.Context, symbol, interval string, limit int) ([]*KlineData, error) {
	if err := ValidateKlineParams(symbol, interval, limit); err != nil {
		return nil, err
	}

	var klineDataList []*KlineData
	var err error

	operation := func() error {
		klines, opErr := c.client.NewKlinesService().
			Symbol(symbol).
			Interval(interval).
			Limit(limit).
			Do(ctx)
		if opErr != nil {
			return opErr
		}

		klineDataList = make([]*KlineData, 0, len(klines))

		for _, kline := range klines {
			klineData, parseErr := c.parseKlineData(symbol, interval, kline)
			if parseErr != nil {
				return parseErr
			}
			klineDataList = append(klineDataList, klineData)
		}

		return nil
	}

	if err = c.withRetry(ctx, operation); err != nil {
		return nil, fmt.Errorf("failed to get klines for %s: %w", symbol, err)
	}

	return klineDataList, nil
}

// GetKlinesWithTimeRange 获取指定时间范围的K线数据
func (c *Client) GetKlinesWithTimeRange(ctx context.Context, symbol, interval string, startTime, endTime time.Time) ([]*KlineData, error) {
	if !IsValidSymbol(symbol) {
		return nil, ErrInvalidSymbol
	}

	if !IsValidInterval(interval) {
		return nil, ErrInvalidInterval
	}

	if startTime.After(endTime) {
		return nil, ErrInvalidTimeRange
	}

	var klineDataList []*KlineData
	var err error

	operation := func() error {
		klines, opErr := c.client.NewKlinesService().
			Symbol(symbol).
			Interval(interval).
			StartTime(startTime.UnixMilli()).
			EndTime(endTime.UnixMilli()).
			Do(ctx)
		if opErr != nil {
			return opErr
		}

		klineDataList = make([]*KlineData, 0, len(klines))

		for _, kline := range klines {
			klineData, parseErr := c.parseKlineData(symbol, interval, kline)
			if parseErr != nil {
				return parseErr
			}
			klineDataList = append(klineDataList, klineData)
		}

		return nil
	}

	if err = c.withRetry(ctx, operation); err != nil {
		return nil, fmt.Errorf("failed to get klines for %s with time range: %w", symbol, err)
	}

	return klineDataList, nil
}

// parseKlineData 解析K线数据
func (c *Client) parseKlineData(symbol, interval string, kline *binance.Kline) (*KlineData, error) {
	open, err := strconv.ParseFloat(kline.Open, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse open price: %w", err)
	}

	high, err := strconv.ParseFloat(kline.High, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse high price: %w", err)
	}

	low, err := strconv.ParseFloat(kline.Low, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse low price: %w", err)
	}

	closePrice, err := strconv.ParseFloat(kline.Close, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse close price: %w", err)
	}

	volume, err := strconv.ParseFloat(kline.Volume, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse volume: %w", err)
	}

	takerBuyBaseVolume, err := strconv.ParseFloat(kline.TakerBuyBaseAssetVolume, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse taker buy base volume: %w", err)
	}

	takerBuyQuoteVolume, err := strconv.ParseFloat(kline.TakerBuyQuoteAssetVolume, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse taker buy quote volume: %w", err)
	}

	return &KlineData{
		Symbol:              symbol,
		Interval:            interval,
		OpenTime:            time.Unix(0, kline.OpenTime*int64(time.Millisecond)),
		CloseTime:           time.Unix(0, kline.CloseTime*int64(time.Millisecond)),
		Open:                open,
		High:                high,
		Low:                 low,
		Close:               closePrice,
		Volume:              volume,
		TradeCount:          kline.TradeNum,
		TakerBuyBaseVolume:  takerBuyBaseVolume,
		TakerBuyQuoteVolume: takerBuyQuoteVolume,
	}, nil
}

// Ping 检查连接状态
func (c *Client) Ping(ctx context.Context) error {
	var err error

	operation := func() error {
		return c.client.NewPingService().Do(ctx)
	}

	if err = c.withRetry(ctx, operation); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	return nil
}

// GetServerTime 获取服务器时间
func (c *Client) GetServerTime(ctx context.Context) (time.Time, error) {
	var serverTime time.Time
	var err error

	operation := func() error {
		timeRes, opErr := c.client.NewServerTimeService().Do(ctx)
		if opErr != nil {
			return opErr
		}

		serverTime = time.Unix(0, timeRes*int64(time.Millisecond))
		return nil
	}

	if err = c.withRetry(ctx, operation); err != nil {
		return time.Time{}, fmt.Errorf("failed to get server time: %w", err)
	}

	return serverTime, nil
}

// GetTicker24hr 获取24小时价格变动统计
func (c *Client) GetTicker24hr(ctx context.Context, symbol string) (*TickerData, error) {
	if !IsValidSymbol(symbol) {
		return nil, ErrInvalidSymbol
	}

	var tickerData *TickerData
	var err error

	operation := func() error {
		ticker, opErr := c.client.NewListPriceChangeStatsService().Symbol(symbol).Do(ctx)
		if opErr != nil {
			return opErr
		}

		if len(ticker) == 0 {
			return fmt.Errorf("no ticker data for symbol %s", symbol)
		}

		data, parseErr := c.parseTickerData(ticker[0])
		if parseErr != nil {
			return parseErr
		}

		tickerData = data
		return nil
	}

	if err = c.withRetry(ctx, operation); err != nil {
		return nil, fmt.Errorf("failed to get 24hr ticker for %s: %w", symbol, err)
	}

	return tickerData, nil
}

// GetAllTickers24hr 获取所有交易对的24小时价格变动统计
func (c *Client) GetAllTickers24hr(ctx context.Context) ([]*TickerData, error) {
	var tickerDataList []*TickerData
	var err error

	operation := func() error {
		tickers, opErr := c.client.NewListPriceChangeStatsService().Do(ctx)
		if opErr != nil {
			return opErr
		}

		tickerDataList = make([]*TickerData, 0, len(tickers))

		for _, ticker := range tickers {
			data, parseErr := c.parseTickerData(ticker)
			if parseErr != nil {
				return parseErr
			}
			tickerDataList = append(tickerDataList, data)
		}

		return nil
	}

	if err = c.withRetry(ctx, operation); err != nil {
		return nil, fmt.Errorf("failed to get all 24hr tickers: %w", err)
	}

	return tickerDataList, nil
}

// parseTickerData 解析Ticker数据
func (c *Client) parseTickerData(ticker *binance.PriceChangeStats) (*TickerData, error) {
	// 解析各个字段
	priceChange, err := strconv.ParseFloat(ticker.PriceChange, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse price change: %w", err)
	}

	priceChangePercent, err := strconv.ParseFloat(ticker.PriceChangePercent, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse price change percent: %w", err)
	}

	weightedAvgPrice, err := strconv.ParseFloat(ticker.WeightedAvgPrice, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse weighted avg price: %w", err)
	}

	prevClosePrice, err := strconv.ParseFloat(ticker.PrevClosePrice, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse prev close price: %w", err)
	}

	lastPrice, err := strconv.ParseFloat(ticker.LastPrice, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse last price: %w", err)
	}

	lastQty, err := strconv.ParseFloat(ticker.LastQty, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse last qty: %w", err)
	}

	bidPrice, err := strconv.ParseFloat(ticker.BidPrice, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse bid price: %w", err)
	}

	bidQty, err := strconv.ParseFloat(ticker.BidQty, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse bid qty: %w", err)
	}

	askPrice, err := strconv.ParseFloat(ticker.AskPrice, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ask price: %w", err)
	}

	askQty, err := strconv.ParseFloat(ticker.AskQty, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ask qty: %w", err)
	}

	openPrice, err := strconv.ParseFloat(ticker.OpenPrice, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse open price: %w", err)
	}

	highPrice, err := strconv.ParseFloat(ticker.HighPrice, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse high price: %w", err)
	}

	lowPrice, err := strconv.ParseFloat(ticker.LowPrice, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse low price: %w", err)
	}

	volume, err := strconv.ParseFloat(ticker.Volume, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse volume: %w", err)
	}

	quoteVolume, err := strconv.ParseFloat(ticker.QuoteVolume, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse quote volume: %w", err)
	}

	return &TickerData{
		Symbol:             ticker.Symbol,
		PriceChange:        priceChange,
		PriceChangePercent: priceChangePercent,
		WeightedAvgPrice:   weightedAvgPrice,
		PrevClosePrice:     prevClosePrice,
		LastPrice:          lastPrice,
		LastQty:            lastQty,
		BidPrice:           bidPrice,
		BidQty:             bidQty,
		AskPrice:           askPrice,
		AskQty:             askQty,
		OpenPrice:          openPrice,
		HighPrice:          highPrice,
		LowPrice:           lowPrice,
		Volume:             volume,
		QuoteVolume:        quoteVolume,
		OpenTime:           time.Unix(0, ticker.OpenTime*int64(time.Millisecond)),
		CloseTime:          time.Unix(0, ticker.CloseTime*int64(time.Millisecond)),
		Count:              ticker.Count,
	}, nil
}

// GetExchangeInfo 获取交易所信息
func (c *Client) GetExchangeInfo(ctx context.Context) (*ExchangeInfo, error) {
	var exchangeInfo *ExchangeInfo
	var err error

	operation := func() error {
		info, opErr := c.client.NewExchangeInfoService().Do(ctx)
		if opErr != nil {
			return opErr
		}

		symbols := make([]SymbolInfo, 0, len(info.Symbols))
		for _, symbol := range info.Symbols {
			// 转换过滤器
			filters := make([]map[string]interface{}, 0, len(symbol.Filters))
			for range symbol.Filters {
				filterMap := make(map[string]interface{})
				// 这里需要根据实际的过滤器结构进行转换
				// 简化处理，实际使用时需要根据binance库的具体结构来实现
				filters = append(filters, filterMap)
			}

			symbolInfo := SymbolInfo{
				Symbol:                     symbol.Symbol,
				Status:                     symbol.Status,
				BaseAsset:                  symbol.BaseAsset,
				BaseAssetPrecision:         symbol.BaseAssetPrecision,
				QuoteAsset:                 symbol.QuoteAsset,
				QuoteAssetPrecision:        symbol.QuotePrecision,
				OrderTypes:                 symbol.OrderTypes,
				IcebergAllowed:             symbol.IcebergAllowed,
				OcoAllowed:                 symbol.OcoAllowed,
				QuoteOrderQtyMarketAllowed: symbol.QuoteOrderQtyMarketAllowed,
				AllowTrailingStop:          false, // 字段可能不存在，设置默认值
				CancelReplaceAllowed:       false, // 字段可能不存在，设置默认值
				IsSpotTradingAllowed:       symbol.IsSpotTradingAllowed,
				IsMarginTradingAllowed:     symbol.IsMarginTradingAllowed,
				Filters:                    filters,
				Permissions:                symbol.Permissions,
			}
			symbols = append(symbols, symbolInfo)
		}

		exchangeInfo = &ExchangeInfo{
			Timezone:   info.Timezone,
			ServerTime: time.Unix(0, info.ServerTime*int64(time.Millisecond)),
			Symbols:    symbols,
		}

		return nil
	}

	if err = c.withRetry(ctx, operation); err != nil {
		return nil, fmt.Errorf("failed to get exchange info: %w", err)
	}

	return exchangeInfo, nil
}
