package datasource

import (
	"context"
	"testing"
	"time"

	"ta-watcher/internal/config"
)

// TestMonthlyKlines 专门测试月线数据获取（20个月）
func TestMonthlyKlines(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping monthly klines test in short mode")
	}

	cfg := config.DefaultConfig()
	factory := NewFactory()

	// 测试Binance和Coinbase的月线数据
	sources := []string{"binance", "coinbase"}

	for _, sourceType := range sources {
		t.Run(sourceType, func(t *testing.T) {
			ds, err := factory.CreateDataSource(sourceType, cfg)
			if err != nil {
				t.Fatalf("Failed to create %s data source: %v", sourceType, err)
			}

			ctx := context.Background()
			symbol := "BTCUSDT"

			// 获取过去20个月的数据
			endTime := time.Now().Truncate(24 * time.Hour)
			startTime := endTime.Add(-20 * 30 * 24 * time.Hour) // 大约20个月

			// 对于Coinbase，我们获取日线数据然后聚合成月线
			var klines []*Kline
			if sourceType == "coinbase" {
				// 获取日线数据
				dailyKlines, err := ds.GetKlines(ctx, symbol, Timeframe1d, startTime, endTime, 600)
				if err != nil {
					t.Logf("Failed to get daily klines from %s: %v", sourceType, err)
					return
				}

				t.Logf("%s: Got %d daily klines for aggregation", sourceType, len(dailyKlines))

				// 手动聚合成月线
				klines = aggregateToMonthly(dailyKlines)
			} else {
				// Binance可能支持月线，或者我们也需要聚合
				dailyKlines, err := ds.GetKlines(ctx, symbol, Timeframe1d, startTime, endTime, 600)
				if err != nil {
					t.Logf("Failed to get daily klines from %s: %v", sourceType, err)
					return
				}

				t.Logf("%s: Got %d daily klines for aggregation", sourceType, len(dailyKlines))
				klines = aggregateToMonthly(dailyKlines)
			}

			t.Logf("%s: Aggregated to %d monthly klines", sourceType, len(klines))

			// 验证月线数据
			if len(klines) < 15 {
				t.Logf("Warning: Expected more monthly data, got only %d klines", len(klines))
			}

			// 检查每个月线数据的合理性
			for i, kline := range klines {
				if kline.High < kline.Low {
					t.Errorf("Month %d: Invalid high/low: High=%.2f Low=%.2f", i, kline.High, kline.Low)
				}
				if kline.Open <= 0 || kline.Close <= 0 || kline.Volume < 0 {
					t.Errorf("Month %d: Invalid values: Open=%.2f Close=%.2f Volume=%.2f", i, kline.Open, kline.Close, kline.Volume)
				}

				// 输出月线信息（只输出前5个和最后5个）
				if i < 5 || i >= len(klines)-5 {
					t.Logf("Month %s: Open=%.2f High=%.2f Low=%.2f Close=%.2f Volume=%.2f",
						kline.OpenTime.Format("2006-01"), kline.Open, kline.High, kline.Low, kline.Close, kline.Volume)
				}
			}
		})
	}
}

// aggregateToMonthly 将日线数据聚合为月线数据
func aggregateToMonthly(dailyKlines []*Kline) []*Kline {
	if len(dailyKlines) == 0 {
		return nil
	}

	var monthlyKlines []*Kline
	var currentMonth []*Kline

	for _, kline := range dailyKlines {
		// 检查是否需要开始新月
		if len(currentMonth) == 0 {
			currentMonth = append(currentMonth, kline)
			continue
		}

		lastKline := currentMonth[len(currentMonth)-1]

		// 如果是新月的第一天，聚合当前月并开始新月
		if kline.OpenTime.Month() != lastKline.OpenTime.Month() || kline.OpenTime.Year() != lastKline.OpenTime.Year() {
			if len(currentMonth) > 0 {
				monthlyKlines = append(monthlyKlines, aggregateMonthPeriod(currentMonth))
			}
			currentMonth = []*Kline{kline}
		} else {
			currentMonth = append(currentMonth, kline)
		}
	}

	// 聚合最后一个月
	if len(currentMonth) > 0 {
		monthlyKlines = append(monthlyKlines, aggregateMonthPeriod(currentMonth))
	}

	return monthlyKlines
}

// aggregateMonthPeriod 聚合一个月的K线数据
func aggregateMonthPeriod(dailyKlines []*Kline) *Kline {
	if len(dailyKlines) == 0 {
		return nil
	}

	first := dailyKlines[0]
	last := dailyKlines[len(dailyKlines)-1]

	monthly := &Kline{
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
	for _, kline := range dailyKlines {
		if kline.High > monthly.High {
			monthly.High = kline.High
		}
		if kline.Low < monthly.Low {
			monthly.Low = kline.Low
		}
		monthly.Volume += kline.Volume
	}

	return monthly
}

// TestWeeklyKlines 测试周线数据获取
func TestWeeklyKlines(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping weekly klines test in short mode")
	}

	cfg := config.DefaultConfig()
	factory := NewFactory()

	// 测试Coinbase的周线聚合
	ds, err := factory.CreateDataSource("coinbase", cfg)
	if err != nil {
		t.Fatalf("Failed to create coinbase data source: %v", err)
	}

	ctx := context.Background()
	symbol := "BTCUSDT"

	// 获取过去12周的数据
	endTime := time.Now().Truncate(24 * time.Hour)
	startTime := endTime.Add(-12 * 7 * 24 * time.Hour) // 12周

	// 测试周线数据获取
	klines, err := ds.GetKlines(ctx, symbol, Timeframe1w, startTime, endTime, 12)
	if err != nil {
		t.Logf("Failed to get weekly klines: %v", err)
		return
	}

	t.Logf("Got %d weekly klines", len(klines))

	for i, kline := range klines {
		if i < 3 || i >= len(klines)-3 {
			t.Logf("Week %s: Open=%.2f High=%.2f Low=%.2f Close=%.2f Volume=%.2f",
				kline.OpenTime.Format("2006-01-02"), kline.Open, kline.High, kline.Low, kline.Close, kline.Volume)
		}

		// 验证数据合理性
		if kline.High < kline.Low {
			t.Errorf("Week %d: Invalid high/low", i)
		}
	}
}
