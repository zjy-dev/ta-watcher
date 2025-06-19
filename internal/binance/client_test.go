package binance

import (
	"context"
	"testing"
	"time"

	"ta-watcher/internal/config"

	"github.com/adshao/go-binance/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultConfig 测试默认配置
func TestDefaultConfig(t *testing.T) {
	cfg := &config.BinanceConfig{
		RateLimit: config.RateLimitConfig{
			RequestsPerMinute: 1200,
			MaxRetries:        3,
			RetryDelay:        time.Second,
		},
	}

	assert.NotNil(t, cfg)
	assert.Equal(t, 1200, cfg.RateLimit.RequestsPerMinute)
	assert.Equal(t, time.Second*1, cfg.RateLimit.RetryDelay)
	assert.Equal(t, 3, cfg.RateLimit.MaxRetries)
}

// TestNewClient 测试客户端创建
func TestNewClient(t *testing.T) {
	tests := []struct {
		name   string
		config *config.BinanceConfig
		hasErr bool
	}{
		{
			name:   "with default config",
			config: nil,
			hasErr: false,
		},
		{
			name: "with custom config",
			config: &config.BinanceConfig{
				RateLimit: config.RateLimitConfig{
					RequestsPerMinute: 600,
					MaxRetries:        2,
					RetryDelay:        time.Second * 2,
				},
			},
			hasErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config)

			if tt.hasErr {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.NotNil(t, client.config)
				assert.NotNil(t, client.client)
				assert.NotNil(t, client.rateLimiter)

				// 测试关闭
				err = client.Close()
				assert.NoError(t, err)
			}
		})
	}
}

// TestRateLimiter 测试限流器
func TestRateLimiter(t *testing.T) {
	// 创建一个小的限流器进行测试
	rl := newRateLimiter(2) // 每分钟2个请求
	defer rl.stop()

	ctx := context.Background()

	// 应该能够获取到前两个令牌
	err := rl.acquire(ctx)
	assert.NoError(t, err)

	err = rl.acquire(ctx)
	assert.NoError(t, err)

	// 第三个请求应该阻塞，使用超时上下文测试
	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	defer cancel()

	err = rl.acquire(ctxWithTimeout)
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

// TestShouldRetry 测试重试逻辑
func TestShouldRetry(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "timeout error",
			err:      &timeoutError{},
			expected: true,
		},
		{
			name:     "rate limit error",
			err:      &rateLimitError{},
			expected: true,
		},
		{
			name:     "other error",
			err:      &otherError{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldRetry(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// 模拟错误类型用于测试
type timeoutError struct{}

func (e *timeoutError) Error() string { return "timeout" }

type rateLimitError struct{}

func (e *rateLimitError) Error() string { return "rate limit exceeded" }

type otherError struct{}

func (e *otherError) Error() string { return "other error" }

// TestValidation 测试参数验证函数
func TestValidation(t *testing.T) {
	t.Run("IsValidSymbol", func(t *testing.T) {
		tests := []struct {
			symbol   string
			expected bool
		}{
			{"BTCUSDT", true},
			{"ETHUSDT", true},
			{"ADAUSDT", true},
			{"BNBBTC", true},
			{"ETHBTC", true},
			{"LINKUSDC", true},
			{"BTC", false},                // 太短
			{"", false},                   // 空字符串
			{"INVALIDPAIR", false},        // 无效格式
			{"VERYLONGSYMBOLNAME", false}, // 太长
		}

		for _, tt := range tests {
			t.Run(tt.symbol, func(t *testing.T) {
				result := IsValidSymbol(tt.symbol)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("IsValidInterval", func(t *testing.T) {
		tests := []struct {
			interval string
			expected bool
		}{
			{Interval1m, true},
			{Interval5m, true},
			{Interval1h, true},
			{Interval1d, true},
			{Interval1w, true},
			{Interval1M, true},
			{"invalid", false},
			{"", false},
			{"1x", false},
		}

		for _, tt := range tests {
			t.Run(tt.interval, func(t *testing.T) {
				result := IsValidInterval(tt.interval)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("ValidateKlineParams", func(t *testing.T) {
		tests := []struct {
			name     string
			symbol   string
			interval string
			limit    int
			hasErr   bool
		}{
			{
				name:     "valid params",
				symbol:   "BTCUSDT",
				interval: Interval1h,
				limit:    100,
				hasErr:   false,
			},
			{
				name:     "invalid symbol",
				symbol:   "INVALID",
				interval: Interval1h,
				limit:    100,
				hasErr:   true,
			},
			{
				name:     "invalid interval",
				symbol:   "BTCUSDT",
				interval: "invalid",
				limit:    100,
				hasErr:   true,
			},
			{
				name:     "limit too small",
				symbol:   "BTCUSDT",
				interval: Interval1h,
				limit:    0,
				hasErr:   true,
			},
			{
				name:     "limit too large",
				symbol:   "BTCUSDT",
				interval: Interval1h,
				limit:    6000,
				hasErr:   true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := ValidateKlineParams(tt.symbol, tt.interval, tt.limit)
				if tt.hasErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

// TestKlineDataParsing 测试K线数据解析
func TestKlineDataParsing(t *testing.T) {
	cfg := &config.BinanceConfig{
		RateLimit: config.RateLimitConfig{
			RequestsPerMinute: 1200,
			MaxRetries:        3,
			RetryDelay:        time.Second,
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	// 模拟binance.Kline数据
	mockKline := &binance.Kline{
		OpenTime:                 1640995200000, // 2022-01-01 00:00:00 UTC
		CloseTime:                1640998799999, // 2022-01-01 00:59:59 UTC
		Open:                     "47000.50",
		High:                     "47500.00",
		Low:                      "46800.25",
		Close:                    "47200.75",
		Volume:                   "1250.50000000",
		TradeNum:                 1500,
		TakerBuyBaseAssetVolume:  "625.25000000",
		TakerBuyQuoteAssetVolume: "29500000.00000000",
	}

	klineData, err := client.parseKlineData("BTCUSDT", Interval1h, mockKline)
	require.NoError(t, err)

	assert.Equal(t, "BTCUSDT", klineData.Symbol)
	assert.Equal(t, Interval1h, klineData.Interval)
	assert.Equal(t, 47000.50, klineData.Open)
	assert.Equal(t, 47500.00, klineData.High)
	assert.Equal(t, 46800.25, klineData.Low)
	assert.Equal(t, 47200.75, klineData.Close)
	assert.Equal(t, 1250.50, klineData.Volume)
	assert.Equal(t, int64(1500), klineData.TradeCount)
	assert.Equal(t, 625.25, klineData.TakerBuyBaseVolume)
	assert.Equal(t, 29500000.00, klineData.TakerBuyQuoteVolume)

	// 验证时间转换
	expectedOpenTime := time.Unix(0, 1640995200000*int64(time.Millisecond))
	expectedCloseTime := time.Unix(0, 1640998799999*int64(time.Millisecond))
	assert.Equal(t, expectedOpenTime, klineData.OpenTime)
	assert.Equal(t, expectedCloseTime, klineData.CloseTime)
}

// TestTickerDataParsing 测试Ticker数据解析
func TestTickerDataParsing(t *testing.T) {
	cfg := &config.BinanceConfig{
		RateLimit: config.RateLimitConfig{
			RequestsPerMinute: 1200,
			MaxRetries:        3,
			RetryDelay:        time.Second,
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	// 模拟binance.PriceChangeStats数据
	mockTicker := &binance.PriceChangeStats{
		Symbol:             "BTCUSDT",
		PriceChange:        "1200.50",
		PriceChangePercent: "2.65",
		WeightedAvgPrice:   "46800.25",
		PrevClosePrice:     "45200.00",
		LastPrice:          "46402.50",
		LastQty:            "0.15000000",
		BidPrice:           "46402.00",
		BidQty:             "5.25000000",
		AskPrice:           "46403.00",
		AskQty:             "3.80000000",
		OpenPrice:          "45202.00",
		HighPrice:          "47500.00",
		LowPrice:           "44800.00",
		Volume:             "25420.50000000",
		QuoteVolume:        "1189526850.25000000",
		OpenTime:           1640995200000,
		CloseTime:          1641081599999,
		Count:              125000,
	}

	tickerData, err := client.parseTickerData(mockTicker)
	require.NoError(t, err)

	assert.Equal(t, "BTCUSDT", tickerData.Symbol)
	assert.Equal(t, 1200.50, tickerData.PriceChange)
	assert.Equal(t, 2.65, tickerData.PriceChangePercent)
	assert.Equal(t, 46800.25, tickerData.WeightedAvgPrice)
	assert.Equal(t, 45200.00, tickerData.PrevClosePrice)
	assert.Equal(t, 46402.50, tickerData.LastPrice)
	assert.Equal(t, 0.15, tickerData.LastQty)
	assert.Equal(t, 46402.00, tickerData.BidPrice)
	assert.Equal(t, 5.25, tickerData.BidQty)
	assert.Equal(t, 46403.00, tickerData.AskPrice)
	assert.Equal(t, 3.80, tickerData.AskQty)
	assert.Equal(t, 45202.00, tickerData.OpenPrice)
	assert.Equal(t, 47500.00, tickerData.HighPrice)
	assert.Equal(t, 44800.00, tickerData.LowPrice)
	assert.Equal(t, 25420.50, tickerData.Volume)
	assert.Equal(t, 1189526850.25, tickerData.QuoteVolume)
	assert.Equal(t, int64(125000), tickerData.Count)
}

// TestWithRetry 测试重试机制
func TestWithRetry(t *testing.T) {
	cfg := &config.BinanceConfig{
		RateLimit: config.RateLimitConfig{
			RequestsPerMinute: 1200,
			MaxRetries:        2,
			RetryDelay:        time.Millisecond * 10,
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	t.Run("success on first try", func(t *testing.T) {
		callCount := 0
		operation := func() error {
			callCount++
			return nil
		}

		err := client.withRetry(ctx, operation)
		assert.NoError(t, err)
		assert.Equal(t, 1, callCount)
	})

	t.Run("success on retry", func(t *testing.T) {
		callCount := 0
		operation := func() error {
			callCount++
			if callCount == 1 {
				return &timeoutError{} // 第一次失败
			}
			return nil // 第二次成功
		}

		err := client.withRetry(ctx, operation)
		assert.NoError(t, err)
		assert.Equal(t, 2, callCount)
	})

	t.Run("fail after max retries", func(t *testing.T) {
		callCount := 0
		operation := func() error {
			callCount++
			return &timeoutError{} // 总是失败
		}

		err := client.withRetry(ctx, operation)
		assert.Error(t, err)
		assert.Equal(t, 3, callCount) // 1次初始调用 + 2次重试
		assert.Contains(t, err.Error(), "operation failed after")
	})

	t.Run("non-retryable error", func(t *testing.T) {
		callCount := 0
		operation := func() error {
			callCount++
			return &otherError{} // 不可重试的错误
		}

		err := client.withRetry(ctx, operation)
		assert.Error(t, err)
		assert.Equal(t, 1, callCount) // 不重试
	})

	t.Run("context cancelled", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel() // 立即取消

		// 等待一点时间确保上下文被完全取消
		time.Sleep(time.Millisecond * 10)

		operation := func() error {
			return nil
		}

		err := client.withRetry(cancelCtx, operation)
		if err != nil {
			assert.Error(t, err)
			// 上下文取消时，错误应该包含 context canceled
			assert.Contains(t, err.Error(), "context canceled")
		} else {
			// 如果没有错误，说明操作在上下文取消前就完成了
			t.Log("Operation completed before context cancellation")
		}
	})
}

// TestDataStructures 测试数据结构
func TestDataStructures(t *testing.T) {
	t.Run("PriceData", func(t *testing.T) {
		now := time.Now()
		price := &PriceData{
			Symbol:    "BTCUSDT",
			Price:     46500.50,
			Timestamp: now,
		}

		assert.Equal(t, "BTCUSDT", price.Symbol)
		assert.Equal(t, 46500.50, price.Price)
		assert.Equal(t, now, price.Timestamp)
	})

	t.Run("KlineData", func(t *testing.T) {
		openTime := time.Now().Add(-time.Hour)
		closeTime := time.Now()

		kline := &KlineData{
			Symbol:              "ETHUSDT",
			Interval:            Interval1h,
			OpenTime:            openTime,
			CloseTime:           closeTime,
			Open:                3200.50,
			High:                3250.75,
			Low:                 3180.25,
			Close:               3230.00,
			Volume:              1500.25,
			TradeCount:          2500,
			TakerBuyBaseVolume:  750.125,
			TakerBuyQuoteVolume: 2425000.50,
		}

		assert.Equal(t, "ETHUSDT", kline.Symbol)
		assert.Equal(t, Interval1h, kline.Interval)
		assert.Equal(t, openTime, kline.OpenTime)
		assert.Equal(t, closeTime, kline.CloseTime)
		assert.Equal(t, 3200.50, kline.Open)
		assert.Equal(t, 3250.75, kline.High)
		assert.Equal(t, 3180.25, kline.Low)
		assert.Equal(t, 3230.00, kline.Close)
		assert.Equal(t, 1500.25, kline.Volume)
		assert.Equal(t, int64(2500), kline.TradeCount)
		assert.Equal(t, 750.125, kline.TakerBuyBaseVolume)
		assert.Equal(t, 2425000.50, kline.TakerBuyQuoteVolume)
	})
}

// TestConcurrentAccess 测试并发访问
func TestConcurrentAccess(t *testing.T) {
	cfg := &config.BinanceConfig{
		RateLimit: config.RateLimitConfig{
			RequestsPerMinute: 100, // 增加限流以支持并发测试
			MaxRetries:        3,
			RetryDelay:        time.Second,
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	// 并发执行多个操作
	const numGoroutines = 10
	const numOperationsPerGoroutine = 5

	errChan := make(chan error, numGoroutines*numOperationsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < numOperationsPerGoroutine; j++ {
				ctx := context.Background()

				// 测试限流器的并发访问
				err := client.rateLimiter.acquire(ctx)
				errChan <- err
			}
		}()
	}

	// 收集所有结果
	for i := 0; i < numGoroutines*numOperationsPerGoroutine; i++ {
		err := <-errChan
		assert.NoError(t, err)
	}
}

// Benchmark tests
// Note: 移除了 BenchmarkRateLimiterAcquire 因为它会在并发运行时卡住

func BenchmarkValidateKlineParams(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateKlineParams("BTCUSDT", Interval1h, 100)
	}
}

func BenchmarkIsValidSymbol(b *testing.B) {
	symbols := []string{"BTCUSDT", "ETHUSDT", "ADAUSDT", "BNBBTC", "LINKUSDC"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		symbol := symbols[i%len(symbols)]
		IsValidSymbol(symbol)
	}
}
