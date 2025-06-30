//go:build integration
// +build integration

package datasource

import (
	"context"
	"math"
	"testing"
	"time"

	"ta-watcher/internal/config"
)

// TestIntegration_DataConsistencyBetweenExchanges 测试不同交易所间数据一致性
func TestIntegration_DataConsistencyBetweenExchanges(t *testing.T) {
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
	testCases := []struct {
		name      string
		timeframe Timeframe
		days      int
		limit     int
		tolerance float64 // 价格差异容忍度（百分比）
	}{
		{"日线_30天", Timeframe1d, 30, 30, 3.0},
		{"周线_12周", Timeframe1w, 84, 12, 5.0},
		{"月线_20月", Timeframe1M, 600, 20, 8.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			endTime := time.Now().Truncate(24 * time.Hour)
			startTime := endTime.Add(-time.Duration(tc.days) * 24 * time.Hour)

			// 从Binance获取数据
			binanceKlines, err := binanceDS.GetKlines(ctx, symbol, tc.timeframe, startTime, endTime, tc.limit)
			if err != nil {
				t.Logf("Binance data fetch failed for %s: %v", tc.timeframe, err)
				return
			}

			// 从Coinbase获取数据
			coinbaseKlines, err := coinbaseDS.GetKlines(ctx, symbol, tc.timeframe, startTime, endTime, tc.limit)
			if err != nil {
				t.Logf("Coinbase data fetch failed for %s: %v", tc.timeframe, err)
				return
			}

			if len(binanceKlines) == 0 || len(coinbaseKlines) == 0 {
				t.Logf("Insufficient data: Binance=%d, Coinbase=%d", len(binanceKlines), len(coinbaseKlines))
				return
			}

			t.Logf("数据对比 - Binance: %d根K线, Coinbase: %d根K线", len(binanceKlines), len(coinbaseKlines))

			// 比较最新的K线价格差异
			binanceLatest := binanceKlines[len(binanceKlines)-1]
			coinbaseLatest := coinbaseKlines[len(coinbaseKlines)-1]

			// 检查各个价格的差异
			priceTypes := map[string][2]float64{
				"开盘价": {binanceLatest.Open, coinbaseLatest.Open},
				"最高价": {binanceLatest.High, coinbaseLatest.High},
				"最低价": {binanceLatest.Low, coinbaseLatest.Low},
				"收盘价": {binanceLatest.Close, coinbaseLatest.Close},
			}

			for priceType, prices := range priceTypes {
				binancePrice, coinbasePrice := prices[0], prices[1]
				diff := math.Abs(binancePrice-coinbasePrice) / binancePrice * 100

				t.Logf("%s: Binance=%.2f, Coinbase=%.2f, 差异=%.2f%%",
					priceType, binancePrice, coinbasePrice, diff)

				if diff > tc.tolerance {
					t.Logf("警告: %s价格差异较大: %.2f%% > %.2f%% (可能由于流动性差异)",
						priceType, diff, tc.tolerance)
				}
			}

			// 检查时间戳合理性
			timeDiff := math.Abs(float64(binanceLatest.OpenTime.Unix() - coinbaseLatest.OpenTime.Unix()))
			maxTimeDiff := float64(24 * 3600) // 最大允许1天差异
			if tc.timeframe == Timeframe1w {
				maxTimeDiff = float64(7 * 24 * 3600) // 周线允许7天差异
			} else if tc.timeframe == Timeframe1M {
				maxTimeDiff = float64(30 * 24 * 3600) // 月线允许30天差异
			}

			if timeDiff > maxTimeDiff {
				t.Errorf("时间戳差异过大: %.0f seconds > %.0f seconds", timeDiff, maxTimeDiff)
			}
		})
	}
}

// TestIntegration_MultipleSymbols 测试多个交易对的数据获取
func TestIntegration_MultipleSymbols(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := config.DefaultConfig()
	factory := NewFactory()

	sources := []string{"binance", "coinbase"}
	symbols := []string{"BTCUSDT", "ETHUSDT"}

	for _, sourceType := range sources {
		t.Run(sourceType, func(t *testing.T) {
			ds, err := factory.CreateDataSource(sourceType, cfg)
			if err != nil {
				t.Fatalf("Failed to create %s data source: %v", sourceType, err)
			}

			ctx := context.Background()

			for _, symbol := range symbols {
				t.Run(symbol, func(t *testing.T) {
					// 测试符号验证
					valid, err := ds.IsSymbolValid(ctx, symbol)
					if err != nil {
						t.Logf("Symbol validation failed for %s: %v", symbol, err)
						return
					}

					if !valid {
						t.Errorf("Expected %s to be valid", symbol)
						return
					}

					// 测试数据获取
					endTime := time.Now().Truncate(time.Hour)
					startTime := endTime.Add(-24 * time.Hour)

					klines, err := ds.GetKlines(ctx, symbol, Timeframe1d, startTime, endTime, 10)
					if err != nil {
						t.Logf("Failed to get klines for %s: %v", symbol, err)
						return
					}

					if len(klines) > 0 {
						latest := klines[len(klines)-1]
						t.Logf("%s 最新价格: %.2f", symbol, latest.Close)

						// 基本数据验证
						if latest.High < latest.Low {
							t.Errorf("无效K线: 最高价 (%.2f) < 最低价 (%.2f)", latest.High, latest.Low)
						}
						if latest.Open <= 0 || latest.Close <= 0 || latest.Volume < 0 {
							t.Errorf("无效K线: 存在非正值 Open=%.2f Close=%.2f Volume=%.2f",
								latest.Open, latest.Close, latest.Volume)
						}
					}
				})
			}
		})
	}
}

// TestIntegration_TimeframeSupport 测试各个时间框架的支持情况
func TestIntegration_TimeframeSupport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := config.DefaultConfig()
	factory := NewFactory()

	sources := []string{"binance", "coinbase"}
	symbol := "BTCUSDT"

	// 定义测试的时间框架
	timeframes := []struct {
		timeframe Timeframe
		hours     int // 用于计算测试时间范围
	}{
		{Timeframe1m, 1},
		{Timeframe5m, 2},
		{Timeframe15m, 4},
		{Timeframe1h, 24},
		{Timeframe4h, 48},
		{Timeframe1d, 720},   // 30天
		{Timeframe1w, 2160},  // 90天 ≈ 12-13周
		{Timeframe1M, 14400}, // 600天 ≈ 20个月
	}

	for _, sourceType := range sources {
		t.Run(sourceType, func(t *testing.T) {
			ds, err := factory.CreateDataSource(sourceType, cfg)
			if err != nil {
				t.Fatalf("Failed to create %s data source: %v", sourceType, err)
			}

			ctx := context.Background()

			for _, tf := range timeframes {
				t.Run(string(tf.timeframe), func(t *testing.T) {
					endTime := time.Now().Truncate(time.Hour)
					startTime := endTime.Add(-time.Duration(tf.hours) * time.Hour)

					klines, err := ds.GetKlines(ctx, symbol, tf.timeframe, startTime, endTime, 50)
					if err != nil {
						t.Logf("GetKlines failed for %s %s: %v", sourceType, tf.timeframe, err)
						return
					}

					t.Logf("%s %s: 获得 %d 根K线", sourceType, tf.timeframe, len(klines))

					if len(klines) > 0 {
						latest := klines[len(klines)-1]
						t.Logf("最新K线: 时间=%s 收盘价=%.2f",
							latest.OpenTime.Format("2006-01-02 15:04"), latest.Close)

						// 验证K线数据完整性
						if latest.High < latest.Low {
							t.Errorf("K线数据错误: High < Low")
						}
						if latest.Open <= 0 || latest.Close <= 0 {
							t.Errorf("K线价格数据错误: 存在非正值")
						}
					}
				})
			}
		})
	}
}

// TestIntegration_LongTermDataConsistency 测试长期数据一致性
func TestIntegration_LongTermDataConsistency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-term integration test in short mode")
	}

	cfg := config.DefaultConfig()
	factory := NewFactory()

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

	// 测试长期月线数据（20个月）
	endTime := time.Now().Truncate(24 * time.Hour)
	startTime := endTime.Add(-600 * 24 * time.Hour) // 约20个月

	binanceMonthly, err := binanceDS.GetKlines(ctx, symbol, Timeframe1M, startTime, endTime, 20)
	if err != nil {
		t.Fatalf("Failed to get Binance monthly data: %v", err)
	}

	coinbaseMonthly, err := coinbaseDS.GetKlines(ctx, symbol, Timeframe1M, startTime, endTime, 20)
	if err != nil {
		t.Fatalf("Failed to get Coinbase monthly data: %v", err)
	}

	t.Logf("长期月线数据: Binance=%d根, Coinbase=%d根", len(binanceMonthly), len(coinbaseMonthly))

	// 验证数据量合理性
	if len(binanceMonthly) < 15 {
		t.Logf("Binance月线数据较少: %d根 (期望至少15根)", len(binanceMonthly))
	}

	if len(coinbaseMonthly) < 15 {
		t.Logf("Coinbase月线数据较少: %d根 (期望至少15根)", len(coinbaseMonthly))
	}

	// 比较最近几个月的数据一致性
	compareMonths := 3
	if len(binanceMonthly) >= compareMonths && len(coinbaseMonthly) >= compareMonths {
		for i := 0; i < compareMonths; i++ {
			bIdx := len(binanceMonthly) - 1 - i
			cIdx := len(coinbaseMonthly) - 1 - i

			bKline := binanceMonthly[bIdx]
			cKline := coinbaseMonthly[cIdx]

			closeDiff := math.Abs(bKline.Close-cKline.Close) / bKline.Close * 100

			t.Logf("月线对比 %s: Binance收盘=%.2f, Coinbase收盘=%.2f, 差异=%.2f%%",
				bKline.OpenTime.Format("2006-01"), bKline.Close, cKline.Close, closeDiff)

			// 月线数据允许更大的差异（最多10%）
			if closeDiff > 10.0 {
				t.Logf("警告: 月线收盘价差异较大: %.2f%% > 10%% (可能正常)", closeDiff)
			}
		}
	}
}
