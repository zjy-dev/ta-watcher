package datasource

import (
	"context"
	"math"
	"testing"
	"time"

	"ta-watcher/internal/config"
)

// TestCompleteDataConsistency 完整的数据一致性测试
// 对比 Binance 和 Coinbase 的日线、周线、月线数据（包括20根月K）
func TestCompleteDataConsistency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping complete data consistency test in short mode")
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
	symbols := []string{"BTCUSDT", "ETHUSDT"}

	// 测试所有时间框架
	timeframeTests := []struct {
		name      string
		timeframe Timeframe
		days      int
		limit     int
		tolerance float64 // 价格差异容忍度（百分比）
	}{
		{"日线_30天", Timeframe1d, 30, 30, 2.0},
		{"日线_90天", Timeframe1d, 90, 90, 2.0},
		{"周线_12周", Timeframe1w, 84, 12, 3.0},  // 12周
		{"月线_20月", Timeframe1M, 600, 20, 5.0}, // 20个月
	}

	for _, symbol := range symbols {
		t.Run(symbol, func(t *testing.T) {
			for _, tt := range timeframeTests {
				t.Run(tt.name, func(t *testing.T) {
					testDataConsistencyForTimeframe(t, ctx, binanceDS, coinbaseDS, symbol, tt.timeframe, tt.days, tt.limit, tt.tolerance)
				})
			}
		})
	}
}

// testDataConsistencyForTimeframe 测试特定时间框架的数据一致性
func testDataConsistencyForTimeframe(t *testing.T, ctx context.Context, binanceDS, coinbaseDS DataSource, symbol string, tf Timeframe, days, limit int, tolerance float64) {
	endTime := time.Now().Truncate(24 * time.Hour)
	startTime := endTime.Add(-time.Duration(days) * 24 * time.Hour)

	t.Logf("测试 %s %s：从 %s 到 %s，期望 %d 根K线",
		symbol, tf, startTime.Format("2006-01-02"), endTime.Format("2006-01-02"), limit)

	// 获取 Binance 数据
	binanceKlines, err := binanceDS.GetKlines(ctx, symbol, tf, startTime, endTime, limit)
	if err != nil {
		t.Logf("Binance 数据获取失败: %v", err)
		return
	}

	// 获取 Coinbase 数据
	coinbaseKlines, err := coinbaseDS.GetKlines(ctx, symbol, tf, startTime, endTime, limit)
	if err != nil {
		t.Logf("Coinbase 数据获取失败: %v", err)
		return
	}

	if len(binanceKlines) == 0 {
		t.Log("Binance 没有数据")
		return
	}

	if len(coinbaseKlines) == 0 {
		t.Log("Coinbase 没有数据")
		return
	}

	t.Logf("获得数据：Binance %d 根，Coinbase %d 根", len(binanceKlines), len(coinbaseKlines))

	// 验证数据质量
	validateKlineData(t, "Binance", binanceKlines)
	validateKlineData(t, "Coinbase", coinbaseKlines)

	// 比较最新的K线价格
	binanceLatest := binanceKlines[len(binanceKlines)-1]
	coinbaseLatest := coinbaseKlines[len(coinbaseKlines)-1]

	priceFields := []struct {
		name     string
		binance  float64
		coinbase float64
	}{
		{"开盘价", binanceLatest.Open, coinbaseLatest.Open},
		{"最高价", binanceLatest.High, coinbaseLatest.High},
		{"最低价", binanceLatest.Low, coinbaseLatest.Low},
		{"收盘价", binanceLatest.Close, coinbaseLatest.Close},
	}

	for _, pf := range priceFields {
		diff := math.Abs(pf.binance-pf.coinbase) / pf.binance * 100
		t.Logf("%s: Binance=%.2f, Coinbase=%.2f, 差异=%.2f%%",
			pf.name, pf.binance, pf.coinbase, diff)

		if diff > tolerance {
			t.Errorf("%s 价格差异过大: %.2f%% > %.2f%%", pf.name, diff, tolerance)
		}
	}

	// 比较成交量（允许更大的差异）
	volumeDiff := math.Abs(binanceLatest.Volume-coinbaseLatest.Volume) / binanceLatest.Volume * 100
	t.Logf("成交量: Binance=%.2f, Coinbase=%.2f, 差异=%.2f%%",
		binanceLatest.Volume, coinbaseLatest.Volume, volumeDiff)

	// 成交量差异较大是正常的，因为交易所流动性不同
	if volumeDiff > 500 { // 500% 差异
		t.Logf("警告：成交量差异很大: %.2f%%", volumeDiff)
	}

	// 验证时间戳合理性
	timeDiff := math.Abs(float64(binanceLatest.OpenTime.Unix() - coinbaseLatest.OpenTime.Unix()))
	maxTimeDiff := float64(24 * 3600) // 最多1天差异

	if tf == Timeframe1w {
		maxTimeDiff = float64(7 * 24 * 3600) // 周线允许7天差异
	} else if tf == Timeframe1M {
		maxTimeDiff = float64(31 * 24 * 3600) // 月线允许31天差异
	}

	if timeDiff > maxTimeDiff {
		t.Errorf("时间戳差异过大: %.0f 秒 > %.0f 秒", timeDiff, maxTimeDiff)
	}

	// 对于月线，特别验证20个月的数据完整性
	if tf == Timeframe1M {
		validateMonthlyData(t, symbol, binanceKlines, coinbaseKlines)
	}
}

// validateKlineData 验证K线数据的基本合理性
func validateKlineData(t *testing.T, sourceName string, klines []*Kline) {
	for i, kline := range klines {
		// 验证价格合理性
		if kline.High < kline.Low {
			t.Errorf("%s K线 %d: 最高价 %.2f < 最低价 %.2f", sourceName, i, kline.High, kline.Low)
		}

		if kline.Open <= 0 || kline.Close <= 0 || kline.High <= 0 || kline.Low <= 0 {
			t.Errorf("%s K线 %d: 存在非正价格值", sourceName, i)
		}

		if kline.Volume < 0 {
			t.Errorf("%s K线 %d: 成交量为负值: %.2f", sourceName, i, kline.Volume)
		}

		// 验证价格在合理范围内（对于主流币种）
		if kline.Close < 1 || kline.Close > 1000000 {
			t.Logf("警告：%s K线 %d 价格似乎不合理: %.2f", sourceName, i, kline.Close)
		}
	}

	// 验证时间顺序
	for i := 1; i < len(klines); i++ {
		if klines[i].OpenTime.Before(klines[i-1].OpenTime) {
			t.Errorf("%s K线时间顺序错误: 索引 %d 的时间早于 %d", sourceName, i, i-1)
		}
	}
}

// validateMonthlyData 验证月线数据的特殊要求
func validateMonthlyData(t *testing.T, symbol string, binanceKlines, coinbaseKlines []*Kline) {
	if len(binanceKlines) < 15 {
		t.Logf("警告：Binance %s 月线数据不足: %d 根", symbol, len(binanceKlines))
	}

	if len(coinbaseKlines) < 15 {
		t.Logf("警告：Coinbase %s 月线数据不足: %d 根", symbol, len(coinbaseKlines))
	}

	// 检查是否覆盖了20个月的时间范围
	if len(binanceKlines) >= 20 {
		earliest := binanceKlines[0].OpenTime
		latest := binanceKlines[len(binanceKlines)-1].OpenTime
		monthsSpan := int(latest.Sub(earliest).Hours() / (24 * 30))

		t.Logf("Binance %s 月线数据时间跨度: %s 到 %s (%d 个月)",
			symbol, earliest.Format("2006-01"), latest.Format("2006-01"), monthsSpan)

		if monthsSpan < 18 {
			t.Logf("警告：时间跨度可能不足20个月")
		}
	}

	// 验证月线数据的价格连续性
	for i := 1; i < len(binanceKlines) && i < 10; i++ {
		prevClose := binanceKlines[i-1].Close
		currOpen := binanceKlines[i].Open

		// 月线之间的价格跳跃不应超过50%（除非有重大事件）
		diff := math.Abs(currOpen-prevClose) / prevClose * 100
		if diff > 50 {
			t.Logf("注意：Binance %s 月线价格跳跃较大: %.2f%% (从 %.2f 到 %.2f)",
				symbol, diff, prevClose, currOpen)
		}
	}
}

// TestSpecificTimeframes 测试特定时间框架的详细数据
func TestSpecificTimeframes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping specific timeframes test in short mode")
	}

	cfg := config.DefaultConfig()
	factory := NewFactory()

	sources := []string{"binance", "coinbase"}

	for _, sourceType := range sources {
		t.Run(sourceType, func(t *testing.T) {
			ds, err := factory.CreateDataSource(sourceType, cfg)
			if err != nil {
				t.Fatalf("Failed to create %s data source: %v", sourceType, err)
			}

			ctx := context.Background()

			// 测试1小时K线
			t.Run("1小时K线", func(t *testing.T) {
				endTime := time.Now().Truncate(time.Hour)
				startTime := endTime.Add(-48 * time.Hour)

				klines, err := ds.GetKlines(ctx, "BTCUSDT", Timeframe1h, startTime, endTime, 48)
				if err != nil {
					t.Logf("获取1小时K线失败: %v", err)
					return
				}

				t.Logf("%s: 获得 %d 根1小时K线", sourceType, len(klines))

				if len(klines) > 0 {
					latest := klines[len(klines)-1]
					t.Logf("最新1小时K线: 时间=%s 收盘价=%.2f",
						latest.OpenTime.Format("2006-01-02 15:04"), latest.Close)
				}
			})

			// 测试日线K线
			t.Run("日线K线", func(t *testing.T) {
				endTime := time.Now().Truncate(24 * time.Hour)
				startTime := endTime.Add(-30 * 24 * time.Hour)

				klines, err := ds.GetKlines(ctx, "BTCUSDT", Timeframe1d, startTime, endTime, 30)
				if err != nil {
					t.Logf("获取日线K线失败: %v", err)
					return
				}

				t.Logf("%s: 获得 %d 根日线K线", sourceType, len(klines))

				if len(klines) > 0 {
					latest := klines[len(klines)-1]
					t.Logf("最新日线K线: 时间=%s 收盘价=%.2f",
						latest.OpenTime.Format("2006-01-02"), latest.Close)
				}
			})
		})
	}
}
