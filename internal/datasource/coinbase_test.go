package datasource

import (
	"context"
	"testing"
	"time"
)

func TestCoinbaseClient_New(t *testing.T) {
	client := NewCoinbaseClient()

	if client == nil {
		t.Fatal("NewCoinbaseClient() returned nil")
	}

	if client.Name() != "coinbase" {
		t.Errorf("Expected name 'coinbase', got '%s'", client.Name())
	}
}

func TestCoinbaseClient_IsSymbolValid(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Coinbase API test in short mode")
	}

	client := NewCoinbaseClient()
	ctx := context.Background()

	tests := []struct {
		name     string
		symbol   string
		expected bool
	}{
		{"Valid symbol", "BTCUSD", true},
		{"Valid symbol with USDT", "BTCUSDT", true}, // Coinbase 现在支持 USDT 交易对
		{"Invalid symbol", "INVALIDUSD", false},
		{"Empty symbol", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, err := client.IsSymbolValid(ctx, tt.symbol)
			if err != nil && tt.expected {
				t.Errorf("IsSymbolValid(%s) returned error: %v", tt.symbol, err)
			}
			if valid != tt.expected {
				t.Errorf("IsSymbolValid(%s) = %v, expected %v", tt.symbol, valid, tt.expected)
			}
		})
	}
}

func TestCoinbaseClient_GetKlines(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Coinbase API test in short mode")
	}

	client := NewCoinbaseClient()
	ctx := context.Background()

	tests := []struct {
		name      string
		symbol    string
		timeframe Timeframe
		limit     int
		wantErr   bool
	}{
		{"Valid daily klines", "BTCUSD", Timeframe1d, 10, false},
		{"Valid hourly klines", "ETHUSD", Timeframe1h, 5, false},
		{"Valid weekly klines", "BTCUSD", Timeframe1w, 5, false},
		{"Valid monthly klines", "BTCUSD", Timeframe1M, 3, false},
		{"Invalid symbol", "INVALIDUSD", Timeframe1d, 10, true},
		{"Zero limit", "BTCUSD", Timeframe1d, 0, false}, // Should default to reasonable limit
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			klines, err := client.GetKlines(ctx, tt.symbol, tt.timeframe, time.Time{}, time.Time{}, tt.limit)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetKlines() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(klines) == 0 {
					t.Error("GetKlines() returned empty result for valid symbol")
				}

				// Verify kline structure
				for i, kline := range klines {
					if kline.Symbol != tt.symbol {
						t.Errorf("Kline[%d].Symbol = %s, expected %s", i, kline.Symbol, tt.symbol)
					}
					if kline.Open <= 0 || kline.Close <= 0 || kline.High <= 0 || kline.Low <= 0 {
						t.Errorf("Kline[%d] has invalid OHLC values: O=%.2f H=%.2f L=%.2f C=%.2f",
							i, kline.Open, kline.High, kline.Low, kline.Close)
					}
					if kline.High < kline.Low {
						t.Errorf("Kline[%d] High < Low: H=%.2f L=%.2f", i, kline.High, kline.Low)
					}
				}
			}
		})
	}
}

func TestCoinbaseClient_TimeframeSupport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Coinbase API test in short mode")
	}

	client := NewCoinbaseClient()
	ctx := context.Background()

	timeframes := []Timeframe{
		Timeframe1m,
		Timeframe5m,
		Timeframe15m,
		Timeframe1h,
		Timeframe6h,
		Timeframe1d,
		Timeframe1w, // Aggregated from daily
		Timeframe1M, // Aggregated from daily
	}

	for _, tf := range timeframes {
		t.Run(string(tf), func(t *testing.T) {
			klines, err := client.GetKlines(ctx, "BTCUSD", tf, time.Time{}, time.Time{}, 5)
			if err != nil {
				t.Errorf("Timeframe %s not supported: %v", tf, err)
			}
			if len(klines) == 0 {
				t.Errorf("No data returned for timeframe %s", tf)
			}
		})
	}
}

func TestCoinbaseClient_Aggregation(t *testing.T) {
	client := NewCoinbaseClient()

	// 创建测试数据：14天的日线数据，应该聚合成2周的周线数据
	dailyKlines := []*Kline{}

	// 第一周：2025-06-23 (周一) 到 2025-06-29 (周日)，7天
	baseTime := time.Date(2025, 6, 23, 8, 0, 0, 0, time.UTC) // 周一
	for i := 0; i < 7; i++ {
		kline := &Kline{
			Symbol:    "BTCUSD",
			OpenTime:  baseTime.AddDate(0, 0, i),
			CloseTime: baseTime.AddDate(0, 0, i).Add(24 * time.Hour),
			Open:      float64(30000 + i*100),
			High:      float64(30500 + i*100),
			Low:       float64(29500 + i*100),
			Close:     float64(30200 + i*100),
			Volume:    1000.0,
		}
		dailyKlines = append(dailyKlines, kline)
	}

	// 第二周：2025-06-30 (周一) 到 2025-07-06 (周日)，7天
	baseTime2 := time.Date(2025, 6, 30, 8, 0, 0, 0, time.UTC) // 第二周周一
	for i := 0; i < 7; i++ {
		kline := &Kline{
			Symbol:    "BTCUSD",
			OpenTime:  baseTime2.AddDate(0, 0, i),
			CloseTime: baseTime2.AddDate(0, 0, i).Add(24 * time.Hour),
			Open:      float64(31000 + i*100),
			High:      float64(31500 + i*100),
			Low:       float64(30500 + i*100),
			Close:     float64(31200 + i*100),
			Volume:    1000.0,
		}
		dailyKlines = append(dailyKlines, kline)
	}

	// 聚合为周线
	weeklyKlines := client.aggregateKlines(dailyKlines, Timeframe1w)

	// 验证结果
	if len(weeklyKlines) != 2 {
		t.Errorf("期望得到 2 条周线数据，实际得到 %d 条", len(weeklyKlines))
	}

	if len(weeklyKlines) >= 1 {
		// 验证第一周数据
		week1 := weeklyKlines[0]
		if week1.Open != 30000 {
			t.Errorf("第一周开盘价期望 30000，实际 %.0f", week1.Open)
		}
		if week1.Close != 30800 { // 最后一天收盘价
			t.Errorf("第一周收盘价期望 30800，实际 %.0f", week1.Close)
		}
		if week1.Volume != 7000 { // 7天总量
			t.Errorf("第一周成交量期望 7000，实际 %.0f", week1.Volume)
		}
	}
}

func TestCoinbaseClient_MonthlyAggregation(t *testing.T) {
	client := NewCoinbaseClient()

	// 创建测试数据：跨越3个月的日线数据
	dailyKlines := []*Kline{}

	// 2025年4月的最后几天
	for day := 28; day <= 30; day++ {
		kline := &Kline{
			Symbol:    "ETHUSD",
			OpenTime:  time.Date(2025, 4, day, 8, 0, 0, 0, time.UTC),
			CloseTime: time.Date(2025, 4, day, 8, 0, 0, 0, time.UTC).Add(24 * time.Hour),
			Open:      float64(2000 + day),
			High:      float64(2100 + day),
			Low:       float64(1900 + day),
			Close:     float64(2050 + day),
			Volume:    500.0,
		}
		dailyKlines = append(dailyKlines, kline)
	}

	// 2025年5月整月
	for day := 1; day <= 31; day++ {
		kline := &Kline{
			Symbol:    "ETHUSD",
			OpenTime:  time.Date(2025, 5, day, 8, 0, 0, 0, time.UTC),
			CloseTime: time.Date(2025, 5, day, 8, 0, 0, 0, time.UTC).Add(24 * time.Hour),
			Open:      float64(2100 + day),
			High:      float64(2200 + day),
			Low:       float64(2000 + day),
			Close:     float64(2150 + day),
			Volume:    500.0,
		}
		dailyKlines = append(dailyKlines, kline)
	}

	// 2025年6月前几天
	for day := 1; day <= 5; day++ {
		kline := &Kline{
			Symbol:    "ETHUSD",
			OpenTime:  time.Date(2025, 6, day, 8, 0, 0, 0, time.UTC),
			CloseTime: time.Date(2025, 6, day, 8, 0, 0, 0, time.UTC).Add(24 * time.Hour),
			Open:      float64(2200 + day),
			High:      float64(2300 + day),
			Low:       float64(2100 + day),
			Close:     float64(2250 + day),
			Volume:    500.0,
		}
		dailyKlines = append(dailyKlines, kline)
	}

	// 聚合为月线
	monthlyKlines := client.aggregateKlines(dailyKlines, Timeframe1M)

	// 验证结果：应该得到3条月线数据（4月末+5月+6月初）
	if len(monthlyKlines) != 3 {
		t.Errorf("期望得到 3 条月线数据，实际得到 %d 条", len(monthlyKlines))
	}

	if len(monthlyKlines) >= 2 {
		// 验证5月份数据（完整月份）
		may := monthlyKlines[1]
		if may.Open != 2101 { // 5月1日开盘价
			t.Errorf("5月开盘价期望 2101，实际 %.0f", may.Open)
		}
		if may.Close != 2181 { // 5月31日收盘价
			t.Errorf("5月收盘价期望 2181，实际 %.0f", may.Close)
		}
		if may.Volume != 15500 { // 31天总量
			t.Errorf("5月成交量期望 15500，实际 %.0f", may.Volume)
		}
	}
}

func TestCoinbaseClient_AggregationEdgeCases(t *testing.T) {
	client := NewCoinbaseClient()

	// 测试空数据
	emptyResult := client.aggregateKlines([]*Kline{}, Timeframe1w)
	if len(emptyResult) != 0 {
		t.Errorf("空数据聚合应该返回空结果，实际得到 %d 条", len(emptyResult))
	}

	// 测试单条数据
	singleKline := []*Kline{
		{
			Symbol:    "BTCUSD",
			OpenTime:  time.Date(2025, 6, 23, 8, 0, 0, 0, time.UTC),
			CloseTime: time.Date(2025, 6, 23, 8, 0, 0, 0, time.UTC).Add(24 * time.Hour),
			Open:      30000,
			High:      30500,
			Low:       29500,
			Close:     30200,
			Volume:    1000,
		},
	}

	singleResult := client.aggregateKlines(singleKline, Timeframe1w)
	if len(singleResult) != 1 {
		t.Errorf("单条数据聚合应该返回1条结果，实际得到 %d 条", len(singleResult))
	}

	if len(singleResult) == 1 {
		result := singleResult[0]
		if result.Open != 30000 || result.Close != 30200 || result.Volume != 1000 {
			t.Errorf("单条数据聚合结果不正确: Open=%.0f Close=%.0f Volume=%.0f",
				result.Open, result.Close, result.Volume)
		}
	}
}
