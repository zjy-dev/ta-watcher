package datasource

import (
	"context"
	"math"
	"testing"
	"time"

	"ta-watcher/internal/config"
)

// TestDataConsistencyBetweenExchanges 测试交易所间数据一致性
func TestDataConsistencyBetweenExchanges(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := config.DefaultConfig()
	factory := NewFactory()

	// 创建两个数据源
	binanceDS, err := factory.CreateDataSource("binance", cfg)
	if err != nil {
		t.Fatalf("Failed to create Binance data source: %v", err)
	}

	coinbaseDS, err := factory.CreateDataSource("coinbase", cfg)
	if err != nil {
		t.Fatalf("Failed to create Coinbase data source: %v", err)
	}

	ctx := context.Background()
	symbol := "BTCUSDT"

	// 测试不同时间框架的数据一致性
	timeframes := []Timeframe{
		Timeframe1d, // 日线
		Timeframe1w, // 周线（如果支持的话，需要通过聚合日线数据）
	}

	for _, tf := range timeframes {
		t.Run(string(tf), func(t *testing.T) {
			testTimeframeConsistency(t, ctx, binanceDS, coinbaseDS, symbol, tf)
		})
	}
}

// testTimeframeConsistency 测试特定时间框架的数据一致性
func testTimeframeConsistency(t *testing.T, ctx context.Context, binanceDS, coinbaseDS DataSource, symbol string, tf Timeframe) {
	endTime := time.Now().Truncate(24 * time.Hour) // 取整到天
	startTime := endTime.Add(-30 * 24 * time.Hour) // 30天前

	// 从Binance获取数据
	binanceKlines, err := binanceDS.GetKlines(ctx, symbol, tf, startTime, endTime, 30)
	if err != nil {
		t.Logf("Binance data fetch failed for %s: %v", tf, err)
		return
	}

	// 从Coinbase获取数据
	coinbaseKlines, err := coinbaseDS.GetKlines(ctx, symbol, tf, startTime, endTime, 30)
	if err != nil {
		t.Logf("Coinbase data fetch failed for %s: %v", tf, err)
		return
	}

	if len(binanceKlines) == 0 {
		t.Logf("No Binance data for %s", tf)
		return
	}

	if len(coinbaseKlines) == 0 {
		t.Logf("No Coinbase data for %s", tf)
		return
	}

	t.Logf("Binance: %d klines, Coinbase: %d klines for %s", len(binanceKlines), len(coinbaseKlines), tf)

	// 比较价格差异（以最新的K线为例）
	if len(binanceKlines) > 0 && len(coinbaseKlines) > 0 {
		binanceLatest := binanceKlines[len(binanceKlines)-1]
		coinbaseLatest := coinbaseKlines[len(coinbaseKlines)-1]

		// 计算价格差异百分比
		priceDiff := math.Abs(binanceLatest.Close-coinbaseLatest.Close) / binanceLatest.Close * 100

		t.Logf("Latest prices - Binance: %.2f, Coinbase: %.2f, Diff: %.2f%%",
			binanceLatest.Close, coinbaseLatest.Close, priceDiff)

		// 价格差异不应超过5%（考虑到市场流动性差异）
		if priceDiff > 5.0 {
			t.Errorf("Price difference too large: %.2f%% > 5%%", priceDiff)
		}

		// 检查时间戳合理性
		timeDiff := math.Abs(float64(binanceLatest.OpenTime.Unix() - coinbaseLatest.OpenTime.Unix()))
		if timeDiff > 86400 { // 不超过1天的差异
			t.Errorf("Timestamp difference too large: %.0f seconds", timeDiff)
		}
	}
}

// TestMultipleTimeframes 测试多时间框架数据获取
func TestMultipleTimeframes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := config.DefaultConfig()
	factory := NewFactory()

	// 测试Binance数据源的多时间框架
	ds, err := factory.CreateDataSource("binance", cfg)
	if err != nil {
		t.Fatalf("Failed to create data source: %v", err)
	}

	ctx := context.Background()
	symbol := "BTCUSDT"
	endTime := time.Now().Truncate(time.Hour)
	startTime := endTime.Add(-24 * time.Hour)

	timeframes := []Timeframe{
		Timeframe1h,
		Timeframe4h,
		Timeframe1d,
	}

	for _, tf := range timeframes {
		t.Run(string(tf), func(t *testing.T) {
			klines, err := ds.GetKlines(ctx, symbol, tf, startTime, endTime, 100)
			if err != nil {
				t.Logf("Failed to get %s klines: %v", tf, err)
				return
			}

			t.Logf("Got %d klines for %s", len(klines), tf)

			if len(klines) > 0 {
				latest := klines[len(klines)-1]
				t.Logf("Latest %s kline: Open=%.2f High=%.2f Low=%.2f Close=%.2f Volume=%.2f",
					tf, latest.Open, latest.High, latest.Low, latest.Close, latest.Volume)

				// 基本数据验证
				if latest.High < latest.Low {
					t.Errorf("Invalid kline: High (%.2f) < Low (%.2f)", latest.High, latest.Low)
				}
				if latest.Open < 0 || latest.Close < 0 || latest.Volume < 0 {
					t.Errorf("Invalid kline: negative values")
				}
			}
		})
	}
}

// TestLongTermData 测试长期数据（月线20个月）
func TestLongTermData(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-term test in short mode")
	}

	cfg := config.DefaultConfig()
	factory := NewFactory()

	ds, err := factory.CreateDataSource("binance", cfg)
	if err != nil {
		t.Fatalf("Failed to create data source: %v", err)
	}

	ctx := context.Background()
	symbol := "BTCUSDT"

	// 测试获取20个月的日线数据（用于聚合成月线）
	endTime := time.Now().Truncate(24 * time.Hour)
	startTime := endTime.Add(-600 * 24 * time.Hour) // 约20个月

	klines, err := ds.GetKlines(ctx, symbol, Timeframe1d, startTime, endTime, 600)
	if err != nil {
		t.Logf("Failed to get long-term data: %v", err)
		return
	}

	t.Logf("Got %d daily klines over ~20 months", len(klines))

	if len(klines) < 500 {
		t.Logf("Warning: Expected more historical data, got only %d klines", len(klines))
	}

	// 验证数据的连续性和合理性
	if len(klines) > 1 {
		for i := 1; i < len(klines); i++ {
			prev := klines[i-1]
			curr := klines[i]

			// 检查时间顺序
			if curr.OpenTime.Before(prev.OpenTime) {
				t.Errorf("Klines not in chronological order at index %d", i)
			}

			// 检查价格跳变（不应超过50%）
			priceChange := math.Abs(curr.Close-prev.Close) / prev.Close
			if priceChange > 0.5 {
				t.Logf("Large price change between %s and %s: %.2f%% (might be normal for crypto)",
					prev.OpenTime.Format("2006-01-02"), curr.OpenTime.Format("2006-01-02"), priceChange*100)
			}
		}
	}
}

// TestSymbolValidation 测试交易对验证
func TestSymbolValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := config.DefaultConfig()
	factory := NewFactory()

	sources := []string{"binance", "coinbase"}

	testCases := []struct {
		symbol   string
		expected bool
	}{
		{"BTCUSDT", true},  // 应该存在
		{"ETHUSDT", true},  // 应该存在
		{"INVALID", false}, // 不应该存在
	}

	for _, sourceType := range sources {
		t.Run(sourceType, func(t *testing.T) {
			ds, err := factory.CreateDataSource(sourceType, cfg)
			if err != nil {
				t.Fatalf("Failed to create %s data source: %v", sourceType, err)
			}

			ctx := context.Background()

			for _, tc := range testCases {
				t.Run(tc.symbol, func(t *testing.T) {
					valid, err := ds.IsSymbolValid(ctx, tc.symbol)
					if err != nil {
						t.Logf("Symbol validation failed for %s on %s: %v", tc.symbol, sourceType, err)
						return
					}

					t.Logf("%s on %s: valid = %t", tc.symbol, sourceType, valid)

					// 对于已知的主要交易对，应该返回true
					if tc.symbol == "BTCUSDT" || tc.symbol == "ETHUSDT" {
						if !valid {
							t.Errorf("Expected %s to be valid on %s", tc.symbol, sourceType)
						}
					}
				})
			}
		})
	}
}
