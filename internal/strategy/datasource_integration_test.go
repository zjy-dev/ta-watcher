package strategy

import (
	"context"
	"testing"
	"time"

	"ta-watcher/internal/config"
	"ta-watcher/internal/datasource"
)

// TestStrategyWithDataSources 测试策略与数据源的集成
func TestStrategyWithDataSources(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping strategy integration test in short mode")
	}

	cfg := config.DefaultConfig()
	factory := datasource.NewFactory()

	// 测试两个数据源
	sources := []string{"binance", "coinbase"}

	for _, sourceType := range sources {
		t.Run(sourceType, func(t *testing.T) {
			ds, err := factory.CreateDataSource(sourceType, cfg)
			if err != nil {
				t.Fatalf("Failed to create %s data source: %v", sourceType, err)
			}

			ctx := context.Background()
			symbol := "BTCUSDT"
			timeframe := datasource.Timeframe1h

			// 获取数据
			endTime := time.Now().Truncate(time.Hour)
			startTime := endTime.Add(-50 * time.Hour)

			klines, err := ds.GetKlines(ctx, symbol, timeframe, startTime, endTime, 50)
			if err != nil {
				t.Logf("Failed to get klines from %s: %v", sourceType, err)
				return
			}

			if len(klines) < 30 {
				t.Logf("Insufficient data from %s: got %d klines", sourceType, len(klines))
				return
			}

			// 创建市场数据结构
			marketData := &MarketData{
				Symbol:    symbol,
				Timeframe: timeframe,
				Klines:    klines,
				Timestamp: time.Now(),
			}

			// 测试策略工厂
			strategyFactory := NewFactory()

			// 测试不同策略
			testCases := []struct {
				name           string
				strategyName   string
				expectedResult bool
			}{
				{"RSI保守策略", "rsi_conservative", true},
				{"RSI激进策略", "rsi_aggressive", true},
				{"MA黄金交叉", "ma_golden_cross", true},
				{"MACD标准", "macd_standard", true},
				{"平衡组合", "balanced_combo", true},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					strat, err := strategyFactory.CreateStrategy(tc.strategyName)
					if err != nil {
						t.Fatalf("Failed to create strategy %s: %v", tc.strategyName, err)
					}

					// 检查数据点需求
					requiredPoints := strat.RequiredDataPoints()
					if len(klines) < requiredPoints {
						t.Logf("Insufficient data for %s: need %d, got %d", tc.strategyName, requiredPoints, len(klines))
						return
					}

					// 执行策略评估
					result, err := strat.Evaluate(marketData)
					if err != nil {
						t.Errorf("Strategy %s evaluation failed: %v", tc.strategyName, err)
						return
					}

					if result == nil {
						t.Errorf("Strategy %s returned nil result", tc.strategyName)
						return
					}

					t.Logf("%s on %s: Signal=%s Message=%s Summary=%s",
						tc.strategyName, sourceType, result.Signal.String(), result.Message, result.IndicatorSummary)

					// 基本验证
					if price, exists := result.Indicators["price"]; exists {
						if p, ok := price.(float64); ok && p <= 0 {
							t.Errorf("Invalid price in result: %.2f", p)
						}
					}

					if result.IndicatorSummary == "" {
						t.Errorf("Missing indicator summary")
					}
				})
			}
		})
	}
}
