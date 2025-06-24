//go:build integration

package coinbase

import (
	"context"
	"strconv"
	"testing"
	"time"

	"ta-watcher/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCoinbaseClientIntegration 集成测试（测试真实API连接）
func TestCoinbaseClientIntegration(t *testing.T) {
	if !config.IsIntegrationTestEnabled("COINBASE") {
		t.Skip("Skipping integration test. Set COINBASE_INTEGRATION_TEST=1 to run.")
	}

	cfg := &Config{
		RateLimit: struct {
			RequestsPerMinute int           `yaml:"requests_per_minute"`
			RetryDelay        time.Duration `yaml:"retry_delay"`
			MaxRetries        int           `yaml:"max_retries"`
		}{
			RequestsPerMinute: 10,
			MaxRetries:        3,
			RetryDelay:        time.Second,
		},
	}

	client := NewClient(cfg)
	require.NotNil(t, client)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	t.Run("connectivity test", func(t *testing.T) {
		// 测试获取产品列表来验证连接性
		products, err := client.GetProducts(ctx)
		assert.NoError(t, err, "Should be able to get products from Coinbase API")
		assert.NotEmpty(t, products, "Should have product data")
		t.Logf("✅ API connectivity test passed, got %d products", len(products))
	})

	t.Run("ticker test", func(t *testing.T) {
		ticker, err := client.GetTicker(ctx, "BTC-USD")
		require.NoError(t, err, "Should be able to get BTC-USD ticker")
		assert.NotNil(t, ticker)
		assert.NotEmpty(t, ticker.Price, "Price should not be empty") // 验证价格是有效的数字
		price, parseErr := testParseFloat(ticker.Price)
		assert.NoError(t, parseErr, "Price should be a valid number")
		assert.Greater(t, price, 0.0, "Price should be positive")

		t.Logf("✅ BTC-USD ticker - Price: $%s", ticker.Price)
	})

	t.Run("klines data test", func(t *testing.T) {
		// 测试获取BTC-USD的K线数据
		klines, err := client.GetKlines(ctx, "BTCUSD", "1h", 5)
		require.NoError(t, err, "Should be able to get klines")
		assert.NotNil(t, klines)
		assert.NotEmpty(t, klines, "Should have kline data")
		assert.LessOrEqual(t, len(klines), 5, "Should not exceed requested limit")

		for i, kline := range klines {
			assert.Greater(t, kline.OpenTime, int64(0), "OpenTime should be positive")
			assert.Greater(t, kline.CloseTime, int64(0), "CloseTime should be positive")
			assert.NotEmpty(t, kline.Open, "Open price should not be empty")
			assert.NotEmpty(t, kline.High, "High price should not be empty")
			assert.NotEmpty(t, kline.Low, "Low price should not be empty")
			assert.NotEmpty(t, kline.Close, "Close price should not be empty")
			assert.NotEmpty(t, kline.Volume, "Volume should not be empty")

			// 验证价格是有效数字
			open, err1 := testParseFloat(kline.Open)
			high, err2 := testParseFloat(kline.High)
			low, err3 := testParseFloat(kline.Low)
			close, err4 := testParseFloat(kline.Close)

			assert.NoError(t, err1, "Open price should be valid number")
			assert.NoError(t, err2, "High price should be valid number")
			assert.NoError(t, err3, "Low price should be valid number")
			assert.NoError(t, err4, "Close price should be valid number")

			assert.Greater(t, open, 0.0, "Open price should be positive")
			assert.Greater(t, high, 0.0, "High price should be positive")
			assert.Greater(t, low, 0.0, "Low price should be positive")
			assert.Greater(t, close, 0.0, "Close price should be positive")
			assert.GreaterOrEqual(t, high, low, "High should be >= Low")

			t.Logf("✅ Kline %d: Open=%s, High=%s, Low=%s, Close=%s",
				i+1, kline.Open, kline.High, kline.Low, kline.Close)
		}

		t.Logf("✅ Got %d klines for BTC-USD", len(klines))
	})

	t.Run("multiple symbols test", func(t *testing.T) {
		symbols := []string{"BTCUSD", "ETHUSD", "ADAUSD"}

		for _, symbol := range symbols {
			t.Run(symbol, func(t *testing.T) {
				klines, err := client.GetKlines(ctx, symbol, "1h", 2)
				if err != nil {
					t.Logf("⚠️  %s may not be available on Coinbase: %v", symbol, err)
					return
				}

				assert.NotEmpty(t, klines, "Should have kline data for %s", symbol)
				t.Logf("✅ %s: Got %d klines, latest close: %s",
					symbol, len(klines), klines[len(klines)-1].Close)
			})
		}
	})

	t.Run("different intervals test", func(t *testing.T) {
		intervals := []string{"1m", "5m", "1h", "1d"}

		for _, interval := range intervals {
			t.Run(interval, func(t *testing.T) {
				klines, err := client.GetKlines(ctx, "BTCUSD", interval, 2)
				require.NoError(t, err, "Should be able to get %s klines", interval)
				assert.NotEmpty(t, klines, "Should have kline data for %s", interval)

				t.Logf("✅ %s interval: Got %d klines", interval, len(klines))
			})
		}
	})
}

// TestCoinbaseClientPerformance 性能测试
func TestCoinbaseClientPerformance(t *testing.T) {
	if !config.IsIntegrationTestEnabled("COINBASE") {
		t.Skip("Skipping integration test. Set COINBASE_INTEGRATION_TEST=1 to run.")
	}

	cfg := &Config{
		RateLimit: struct {
			RequestsPerMinute int           `yaml:"requests_per_minute"`
			RetryDelay        time.Duration `yaml:"retry_delay"`
			MaxRetries        int           `yaml:"max_retries"`
		}{
			RequestsPerMinute: 10,
			MaxRetries:        3,
			RetryDelay:        time.Second,
		},
	}

	client := NewClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	t.Run("rate limiting test", func(t *testing.T) {
		const numRequests = 5
		start := time.Now()

		for i := 0; i < numRequests; i++ {
			_, err := client.GetTicker(ctx, "BTC-USD")
			if err != nil {
				t.Logf("Request %d failed: %v", i+1, err)
			}
		}

		duration := time.Since(start)
		t.Logf("✅ %d requests completed in %v (avg: %v per request)",
			numRequests, duration, duration/numRequests)
	})

	t.Run("concurrent requests test", func(t *testing.T) {
		const numGoroutines = 3
		results := make(chan error, numGoroutines)

		start := time.Now()
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				_, err := client.GetKlines(ctx, "BTCUSD", "1h", 10)
				if err != nil {
					t.Logf("Goroutine %d error: %v", id, err)
				}
				results <- err
			}(i)
		}

		// 等待所有请求完成
		for i := 0; i < numGoroutines; i++ {
			<-results
		}

		duration := time.Since(start)
		t.Logf("✅ %d concurrent requests completed in %v", numGoroutines, duration)
	})
}

// TestCoinbaseClientErrorHandling 错误处理测试
func TestCoinbaseClientErrorHandling(t *testing.T) {
	if !config.IsIntegrationTestEnabled("COINBASE") {
		t.Skip("Skipping integration test. Set COINBASE_INTEGRATION_TEST=1 to run.")
	}

	client := NewClient(nil)
	ctx := context.Background()

	t.Run("invalid symbol test", func(t *testing.T) {
		_, err := client.GetKlines(ctx, "INVALID_SYMBOL", "1h", 10)
		assert.Error(t, err, "Should return error for invalid symbol")
		t.Logf("✅ Invalid symbol error: %v", err)
	})

	t.Run("invalid interval test", func(t *testing.T) {
		_, err := client.GetKlines(ctx, "BTCUSD", "999m", 10)
		assert.Error(t, err, "Should return error for invalid interval")
		t.Logf("✅ Invalid interval error: %v", err)
	})

	t.Run("timeout test", func(t *testing.T) {
		// 创建一个极短的超时上下文
		shortCtx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
		defer cancel()

		_, err := client.GetKlines(shortCtx, "BTCUSD", "1h", 10)
		assert.Error(t, err, "Should return timeout error")
		assert.Contains(t, err.Error(), "context deadline exceeded")
		t.Logf("✅ Timeout error: %v", err)
	})
}

// testParseFloat 辅助函数，用于测试中解析浮点数
func testParseFloat(s string) (float64, error) {
	if s == "" {
		return 0, assert.AnError
	}
	// 使用标准库解析
	return strconv.ParseFloat(s, 64)
}
