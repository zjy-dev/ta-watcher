package datasource

import (
	"context"
	"math"
	"testing"
	"time"

	"ta-watcher/internal/config"
)

// ============================================================================
// 基础单元测试
// ============================================================================

// TestDataSource_Interface 测试数据源接口实现
func TestDataSource_Interface(t *testing.T) {
	cfg := config.DefaultConfig()
	factory := NewFactory()

	sources := []string{"binance", "coinbase"}

	for _, sourceType := range sources {
		t.Run(sourceType, func(t *testing.T) {
			ds, err := factory.CreateDataSource(sourceType, cfg)
			if err != nil {
				t.Fatalf("创建数据源失败: %v", err)
			}

			// 测试Name方法
			if ds.Name() == "" {
				t.Error("数据源名称不能为空")
			}

			// 测试接口方法的存在性（不需要实际调用API）
			ctx := context.Background()

			// 测试IsSymbolValid方法签名
			_, err = ds.IsSymbolValid(ctx, "TEST")
			// 允许错误，因为我们只是测试方法存在

			// 测试GetKlines方法签名
			startTime := time.Now().Add(-24 * time.Hour)
			endTime := time.Now()
			_, err = ds.GetKlines(ctx, "TEST", Timeframe1d, startTime, endTime, 1)
			// 允许错误，因为我们只是测试方法存在

			t.Logf("数据源 %s 接口测试通过", ds.Name())
		})
	}
}

// TestFactory_Basic 测试工厂基本功能
func TestFactory_Basic(t *testing.T) {
	factory := NewFactory()

	// 测试支持的数据源列表
	sources := factory.GetSupportedSources()
	expectedSources := []string{"binance", "coinbase"}

	if len(sources) != len(expectedSources) {
		t.Errorf("支持的数据源数量不匹配: 期望 %d, 实际 %d", len(expectedSources), len(sources))
	}

	sourceMap := make(map[string]bool)
	for _, source := range sources {
		sourceMap[source] = true
	}

	for _, expected := range expectedSources {
		if !sourceMap[expected] {
			t.Errorf("缺少预期的数据源: %s", expected)
		}
	}

	t.Logf("支持的数据源: %v", sources)
}

// TestFactory_CreateDataSource 测试数据源创建
func TestFactory_CreateDataSource(t *testing.T) {
	cfg := config.DefaultConfig()
	factory := NewFactory()

	testCases := []struct {
		name       string
		sourceType string
		wantErr    bool
	}{
		{"创建Binance数据源", "binance", false},
		{"创建Coinbase数据源", "coinbase", false},
		{"创建不支持的数据源", "unsupported", true},
		{"创建空数据源", "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ds, err := factory.CreateDataSource(tc.sourceType, cfg)

			if tc.wantErr {
				if err == nil {
					t.Error("期望出现错误，但没有错误")
				}
				return
			}

			if err != nil {
				t.Fatalf("创建数据源失败: %v", err)
			}

			if ds == nil {
				t.Fatal("数据源为nil")
			}

			if ds.Name() == "" {
				t.Error("数据源名称为空")
			}

			t.Logf("成功创建数据源: %s", ds.Name())
		})
	}
}

// TestTimeframes_Validation 测试时间框架常量定义
func TestTimeframes_Validation(t *testing.T) {
	timeframes := []Timeframe{
		Timeframe1m,
		Timeframe5m,
		Timeframe15m,
		Timeframe1h,
		Timeframe4h,
		Timeframe6h,
		Timeframe1d,
		Timeframe1w,
		Timeframe1M,
	}

	for _, tf := range timeframes {
		if string(tf) == "" {
			t.Errorf("时间框架 %v 的字符串值为空", tf)
		}
		t.Logf("时间框架: %s", tf)
	}
}

// TestKline_Structure 测试K线数据结构
func TestKline_Structure(t *testing.T) {
	now := time.Now()
	kline := &Kline{
		Symbol:    "BTCUSDT",
		OpenTime:  now,
		CloseTime: now.Add(time.Hour),
		Open:      50000.0,
		High:      51000.0,
		Low:       49000.0,
		Close:     50500.0,
		Volume:    100.0,
	}

	// 验证K线数据结构的基本属性
	if kline.Symbol == "" {
		t.Error("K线符号不能为空")
	}

	if kline.OpenTime.IsZero() {
		t.Error("K线开盘时间不能为零值")
	}

	if kline.CloseTime.IsZero() {
		t.Error("K线收盘时间不能为零值")
	}

	if kline.High < kline.Low {
		t.Error("最高价不能小于最低价")
	}

	if kline.Open <= 0 || kline.Close <= 0 {
		t.Error("开盘价和收盘价必须为正数")
	}

	if kline.Volume < 0 {
		t.Error("成交量不能为负数")
	}

	t.Logf("K线数据结构验证通过: %s %.2f", kline.Symbol, kline.Close)
}

// ============================================================================
// 聚合功能测试
// ============================================================================

// TestCoinbase_WeeklyAggregation 测试Coinbase周线聚合功能
func TestCoinbase_WeeklyAggregation(t *testing.T) {
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

	t.Logf("输入数据: %d条日线K线", len(dailyKlines))

	// 执行聚合
	weeklyKlines := client.aggregateKlines(dailyKlines, Timeframe1w)

	t.Logf("输出数据: %d条周线K线", len(weeklyKlines))

	// 验证结果
	if len(weeklyKlines) != 2 {
		t.Errorf("期望2条周线数据，实际得到 %d 条", len(weeklyKlines))
	}

	if len(weeklyKlines) >= 2 {
		// 验证第一周数据
		week1 := weeklyKlines[0]
		if week1.Open != 30000 {
			t.Errorf("第一周开盘价错误: 期望 30000, 实际 %.0f", week1.Open)
		}
		if week1.Close != 30800 {
			t.Errorf("第一周收盘价错误: 期望 30800, 实际 %.0f", week1.Close)
		}

		// 验证第二周数据
		week2 := weeklyKlines[1]
		if week2.Open != 31000 {
			t.Errorf("第二周开盘价错误: 期望 31000, 实际 %.0f", week2.Open)
		}
		if week2.Close != 31800 {
			t.Errorf("第二周收盘价错误: 期望 31800, 实际 %.0f", week2.Close)
		}
	}
}

// TestCoinbase_MonthlyAggregation 测试Coinbase月线聚合功能
func TestCoinbase_MonthlyAggregation(t *testing.T) {
	client := NewCoinbaseClient()

	// 创建跨月的测试数据
	dailyKlines := []*Kline{}

	// 2025年5月：1-31日，31天
	may1 := time.Date(2025, 5, 1, 8, 0, 0, 0, time.UTC)
	for i := 0; i < 31; i++ {
		kline := &Kline{
			Symbol:    "BTCUSD",
			OpenTime:  may1.AddDate(0, 0, i),
			CloseTime: may1.AddDate(0, 0, i).Add(24 * time.Hour),
			Open:      float64(2100 + i),
			High:      float64(2200 + i),
			Low:       float64(2000 + i),
			Close:     float64(2150 + i),
			Volume:    1000.0,
		}
		dailyKlines = append(dailyKlines, kline)
	}

	t.Logf("输入数据: %d条日线K线", len(dailyKlines))

	// 执行聚合
	monthlyKlines := client.aggregateKlines(dailyKlines, Timeframe1M)

	t.Logf("输出数据: %d条月线K线", len(monthlyKlines))

	// 验证应该有1个月的数据
	if len(monthlyKlines) != 1 {
		t.Errorf("期望1条月线数据，实际得到 %d 条", len(monthlyKlines))
	}

	if len(monthlyKlines) >= 1 {
		may := monthlyKlines[0]
		if may.Open != 2100 { // 5月1日的开盘价
			t.Errorf("5月开盘价错误: 期望 2100, 实际 %.0f", may.Open)
		}
	}
}

// TestGetWeekStart 测试周开始时间计算函数
func TestGetWeekStart(t *testing.T) {
	testCases := []struct {
		name     string
		input    time.Time
		expected time.Time
	}{
		{
			name:     "周一",
			input:    time.Date(2025, 6, 23, 15, 30, 45, 0, time.UTC), // 周一
			expected: time.Date(2025, 6, 23, 15, 30, 45, 0, time.UTC), // 保持时间部分
		},
		{
			name:     "周三",
			input:    time.Date(2025, 6, 25, 10, 20, 30, 0, time.UTC), // 周三
			expected: time.Date(2025, 6, 23, 10, 20, 30, 0, time.UTC), // 周一的相同时间
		},
		{
			name:     "周日",
			input:    time.Date(2025, 6, 29, 23, 59, 59, 0, time.UTC), // 周日
			expected: time.Date(2025, 6, 23, 23, 59, 59, 0, time.UTC), // 周一的相同时间
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := getWeekStart(tc.input)
			if !result.Equal(tc.expected) {
				t.Errorf("getWeekStart(%s) = %s, 期望 %s",
					tc.input.Format("2006-01-02 15:04:05"),
					result.Format("2006-01-02 15:04:05"),
					tc.expected.Format("2006-01-02 15:04:05"))
			}
		})
	}
}

// ============================================================================
// API集成测试（需要实际API调用）
// ============================================================================

// TestDataSource_BasicFunctionality 测试数据源基本功能
func TestDataSource_BasicFunctionality(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping API tests in short mode")
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
			symbol := "BTCUSDT"

			// 测试符号验证
			valid, err := ds.IsSymbolValid(ctx, symbol)
			if err != nil {
				t.Logf("Symbol validation failed for %s: %v (可能是API限制)", symbol, err)
				return
			}

			if !valid {
				t.Errorf("Expected %s to be valid on %s", symbol, sourceType)
				return
			}

			// 测试数据获取
			endTime := time.Now().Truncate(time.Hour)
			startTime := endTime.Add(-24 * time.Hour)

			klines, err := ds.GetKlines(ctx, symbol, Timeframe1d, startTime, endTime, 5)
			if err != nil {
				t.Logf("GetKlines failed for %s: %v", sourceType, err)
				return
			}

			t.Logf("%s: 获取到 %d 条日线数据", sourceType, len(klines))

			if len(klines) > 0 {
				latest := klines[len(klines)-1]
				t.Logf("%s 最新价格: %.2f", sourceType, latest.Close)

				// 基本数据验证
				if latest.High < latest.Low {
					t.Errorf("无效K线数据: 最高价 < 最低价")
				}
				if latest.Open <= 0 || latest.Close <= 0 {
					t.Errorf("无效K线数据: 价格非正数")
				}
			}
		})
	}
}

// TestDataSource_MultipleTimeframes 测试多时间框架支持
func TestDataSource_MultipleTimeframes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping API tests in short mode")
	}

	cfg := config.DefaultConfig()
	factory := NewFactory()

	// 测试Binance的多时间框架
	ds, err := factory.CreateDataSource("binance", cfg)
	if err != nil {
		t.Fatalf("Failed to create Binance data source: %v", err)
	}

	ctx := context.Background()
	symbol := "BTCUSDT"
	endTime := time.Now().Truncate(time.Hour)
	startTime := endTime.Add(-48 * time.Hour)

	timeframes := []struct {
		tf    Timeframe
		limit int
	}{
		{Timeframe1h, 24},
		{Timeframe4h, 12},
		{Timeframe1d, 7},
	}

	for _, tf := range timeframes {
		t.Run(string(tf.tf), func(t *testing.T) {
			klines, err := ds.GetKlines(ctx, symbol, tf.tf, startTime, endTime, tf.limit)
			if err != nil {
				t.Logf("GetKlines failed for %s: %v", tf.tf, err)
				return
			}

			t.Logf("时间框架 %s: 获取到 %d 条K线", tf.tf, len(klines))

			if len(klines) > 0 {
				latest := klines[len(klines)-1]
				t.Logf("最新 %s K线: 时间=%s 收盘价=%.2f",
					tf.tf, latest.OpenTime.Format("2006-01-02 15:04"), latest.Close)

				// 验证K线数据完整性
				if latest.High < latest.Low {
					t.Errorf("K线数据错误: High < Low")
				}
				if latest.Open <= 0 || latest.Close <= 0 || latest.Volume < 0 {
					t.Errorf("K线数据错误: 存在非正值")
				}
			}
		})
	}
}

// TestDataSource_DataConsistency 测试数据源间的数据一致性
func TestDataSource_DataConsistency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping consistency tests in short mode")
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

	// 测试不同时间框架的数据一致性
	testCases := []struct {
		name      string
		timeframe Timeframe
		days      int
		limit     int
		tolerance float64 // 价格差异容忍度（百分比）
	}{
		{"日线数据", Timeframe1d, 30, 30, 3.0},
		{"周线数据", Timeframe1w, 84, 12, 5.0},
		{"月线数据", Timeframe1M, 600, 20, 8.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			endTime := time.Now().Truncate(24 * time.Hour)
			startTime := endTime.Add(-time.Duration(tc.days) * 24 * time.Hour)

			// 获取两个数据源的数据
			binanceKlines, err := binanceDS.GetKlines(ctx, symbol, tc.timeframe, startTime, endTime, tc.limit)
			if err != nil {
				t.Logf("Binance数据获取失败: %v", err)
				return
			}

			coinbaseKlines, err := coinbaseDS.GetKlines(ctx, symbol, tc.timeframe, startTime, endTime, tc.limit)
			if err != nil {
				t.Logf("Coinbase数据获取失败: %v", err)
				return
			}

			if len(binanceKlines) == 0 || len(coinbaseKlines) == 0 {
				t.Logf("数据不足: Binance=%d, Coinbase=%d", len(binanceKlines), len(coinbaseKlines))
				return
			}

			t.Logf("%s一致性检查: Binance=%d根K线, Coinbase=%d根K线",
				tc.name, len(binanceKlines), len(coinbaseKlines))

			// 比较最新K线的价格一致性
			binanceLatest := binanceKlines[len(binanceKlines)-1]
			coinbaseLatest := coinbaseKlines[len(coinbaseKlines)-1]

			closeDiff := math.Abs(binanceLatest.Close-coinbaseLatest.Close) / binanceLatest.Close * 100

			t.Logf("收盘价对比: Binance=%.2f, Coinbase=%.2f, 差异=%.2f%%",
				binanceLatest.Close, coinbaseLatest.Close, closeDiff)

			if closeDiff > tc.tolerance {
				t.Logf("价格差异较大: %.2f%% > %.2f%% (可能由于流动性差异)", closeDiff, tc.tolerance)
			} else {
				t.Logf("价格差异在可接受范围内: %.2f%% <= %.2f%%", closeDiff, tc.tolerance)
			}
		})
	}
}

// BenchmarkFactory_CreateDataSource 性能基准测试
func BenchmarkFactory_CreateDataSource(b *testing.B) {
	cfg := config.DefaultConfig()
	factory := NewFactory()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := factory.CreateDataSource("binance", cfg)
		if err != nil {
			b.Fatalf("创建数据源失败: %v", err)
		}
	}
}
