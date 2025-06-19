//go:build integration

package binance

import (
	"context"
	"testing"
	"time"

	"ta-watcher/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBinanceClientIntegration 集成测试（测试真实API连接）
func TestBinanceClientIntegration(t *testing.T) {
	if !config.IsIntegrationTestEnabled("BINANCE") {
		t.Skip("Skipping integration test. Set BINANCE_INTEGRATION_TEST=1 to run.")
	}

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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	t.Run("connectivity test", func(t *testing.T) {
		err := client.Ping(ctx)
		assert.NoError(t, err, "Should be able to ping Binance API")
		t.Log("✅ API connectivity test passed")
	})

	t.Run("server time test", func(t *testing.T) {
		serverTime, err := client.GetServerTime(ctx)
		require.NoError(t, err, "Should be able to get server time")
		assert.False(t, serverTime.IsZero(), "Server time should not be zero")

		timeDiff := time.Since(serverTime)
		if timeDiff < 0 {
			timeDiff = -timeDiff
		}
		assert.True(t, timeDiff < time.Minute*2, "Server time difference should be reasonable")

		t.Logf("✅ Server time: %v, Difference: %v", serverTime.Format("2006-01-02 15:04:05"), timeDiff)
	})

	t.Run("price data test", func(t *testing.T) {
		priceData, err := client.GetPrice(ctx, "BTCUSDT")
		require.NoError(t, err, "Should be able to get BTCUSDT price")
		assert.NotNil(t, priceData)
		assert.Equal(t, "BTCUSDT", priceData.Symbol)
		assert.Greater(t, priceData.Price, 0.0, "Price should be positive")

		t.Logf("✅ BTCUSDT price: $%.2f", priceData.Price)
	})

	t.Run("klines data test", func(t *testing.T) {
		klines, err := client.GetKlines(ctx, "BTCUSDT", Interval1h, 5)
		require.NoError(t, err, "Should be able to get klines")
		assert.NotNil(t, klines)
		assert.NotEmpty(t, klines, "Should have kline data")

		for _, kline := range klines {
			assert.Equal(t, "BTCUSDT", kline.Symbol)
			assert.Equal(t, Interval1h, kline.Interval)
			assert.Greater(t, kline.High, 0.0, "High price should be positive")
			assert.Greater(t, kline.Low, 0.0, "Low price should be positive")
			assert.GreaterOrEqual(t, kline.High, kline.Low, "High should be >= Low")
		}

		t.Logf("✅ Got %d klines, latest close: $%.2f", len(klines), klines[len(klines)-1].Close)
	})

	t.Run("24hr ticker test", func(t *testing.T) {
		ticker, err := client.GetTicker24hr(ctx, "BTCUSDT")
		require.NoError(t, err, "Should be able to get 24hr ticker")
		assert.NotNil(t, ticker)
		assert.Equal(t, "BTCUSDT", ticker.Symbol)
		assert.Greater(t, ticker.LastPrice, 0.0, "Last price should be positive")

		t.Logf("✅ 24hr ticker - Price: $%.2f, Change: %.2f%%",
			ticker.LastPrice, ticker.PriceChangePercent)
	})
}

// TestBinanceClientPerformance 性能测试
func TestBinanceClientPerformance(t *testing.T) {
	if !config.IsIntegrationTestEnabled("BINANCE") {
		t.Skip("Skipping integration test. Set BINANCE_INTEGRATION_TEST=1 to run.")
	}

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

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	t.Run("response latency test", func(t *testing.T) {
		const numRequests = 3
		var totalLatency time.Duration

		for i := 0; i < numRequests; i++ {
			start := time.Now()
			err := client.Ping(ctx)
			latency := time.Since(start)
			totalLatency += latency

			require.NoError(t, err, "Ping should succeed")
			t.Logf("Ping %d latency: %v", i+1, latency)
		}

		avgLatency := totalLatency / numRequests
		t.Logf("✅ Average ping latency: %v", avgLatency)
		assert.Less(t, avgLatency, time.Second*5, "Average latency should be reasonable")
	})
}
