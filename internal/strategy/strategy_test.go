package strategy

import (
	"testing"
	"time"

	"ta-watcher/internal/binance"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestMarketData 创建测试市场数据
func createTestMarketData(symbol string, timeframe Timeframe, prices []float64) *MarketData {
	klines := make([]binance.KlineData, len(prices))
	baseTime := time.Now().Add(-time.Duration(len(prices)) * time.Hour)

	for i, price := range prices {
		klines[i] = binance.KlineData{
			Symbol:    symbol,
			Interval:  string(timeframe),
			OpenTime:  baseTime.Add(time.Duration(i) * time.Hour),
			CloseTime: baseTime.Add(time.Duration(i+1) * time.Hour),
			Open:      price * 0.999, // 稍微低一点的开盘价
			High:      price * 1.002, // 稍微高一点的最高价
			Low:       price * 0.998, // 稍微低一点的最低价
			Close:     price,
			Volume:    1000.0,
		}
	}

	return &MarketData{
		Symbol:    symbol,
		Timeframe: timeframe,
		Klines:    klines,
		Timestamp: time.Now(),
	}
}

func TestRSIStrategy(t *testing.T) {
	strategy := NewRSIStrategy(14, 70, 30)

	t.Run("Basic Properties", func(t *testing.T) {
		assert.Equal(t, "RSI_14_70_30", strategy.Name())
		assert.Contains(t, strategy.Description(), "RSI")
		assert.Equal(t, 15, strategy.RequiredDataPoints()) // 14 + 1
		assert.NotEmpty(t, strategy.SupportedTimeframes())
	})

	t.Run("Oversold Signal", func(t *testing.T) {
		// 创建下降趋势数据，应该产生超卖信号
		prices := []float64{100, 98, 96, 94, 92, 90, 88, 86, 84, 82, 80, 78, 76, 74, 72, 70, 68}
		data := createTestMarketData("BTCUSDT", Timeframe1h, prices)

		result, err := strategy.Evaluate(data)
		require.NoError(t, err)
		require.NotNil(t, result)

		// 应该产生买入信号（超卖）
		assert.Equal(t, SignalBuy, result.Signal)
		assert.Greater(t, result.Confidence, 0.0)
		assert.Contains(t, result.Message, "超卖")
	})

	t.Run("Insufficient Data", func(t *testing.T) {
		prices := []float64{100, 101, 102} // 数据不足
		data := createTestMarketData("BTCUSDT", Timeframe1h, prices)

		result, err := strategy.Evaluate(data)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestMACrossStrategy(t *testing.T) {
	strategy := NewMACrossStrategy(5, 20, 0) // SMA

	t.Run("Basic Properties", func(t *testing.T) {
		assert.Equal(t, "SMA_Cross_5_20", strategy.Name())
		assert.Contains(t, strategy.Description(), "简单移动平均线")
		assert.Equal(t, 22, strategy.RequiredDataPoints()) // 20 + 2
	})

	t.Run("Golden Cross", func(t *testing.T) {
		// 创建上升趋势，快线上穿慢线
		prices := make([]float64, 25)
		for i := 0; i < 25; i++ {
			if i < 20 {
				prices[i] = 100.0 + float64(i)*0.5 // 缓慢上升
			} else {
				prices[i] = 100.0 + float64(i)*2.0 // 快速上升，触发金叉
			}
		}

		data := createTestMarketData("BTCUSDT", Timeframe1h, prices)

		result, err := strategy.Evaluate(data)
		require.NoError(t, err)
		require.NotNil(t, result)

		// 可能产生买入信号（金叉）或持有信号
		assert.True(t, result.Signal == SignalBuy || result.Signal == SignalHold)
		if result.Signal == SignalBuy {
			assert.Contains(t, result.Message, "金叉")
		}
	})
}

func TestMACDStrategy(t *testing.T) {
	strategy := NewMACDStrategy(12, 26, 9)

	t.Run("Basic Properties", func(t *testing.T) {
		assert.Equal(t, "MACD_12_26_9", strategy.Name())
		assert.Contains(t, strategy.Description(), "MACD")
		assert.Equal(t, 45, strategy.RequiredDataPoints()) // 26 + 9 + 10
	})

	t.Run("Valid Evaluation", func(t *testing.T) {
		// 创建足够的数据
		prices := make([]float64, 50)
		for i := 0; i < 50; i++ {
			prices[i] = 100.0 + float64(i%10) // 波动数据
		}

		data := createTestMarketData("BTCUSDT", Timeframe1h, prices)

		result, err := strategy.Evaluate(data)
		require.NoError(t, err)
		require.NotNil(t, result)

		// 验证结果结构
		assert.NotEmpty(t, result.Message)
		assert.Contains(t, result.Indicators, "macd")
		assert.Contains(t, result.Indicators, "signal")
		assert.Contains(t, result.Indicators, "histogram")
	})
}

func TestMultiStrategy(t *testing.T) {
	combo := NewMultiStrategy("测试组合", "测试用组合策略", CombineWeightedAverage)

	// 添加子策略
	combo.AddSubStrategy(NewRSIStrategy(14, 70, 30), 1.0)
	combo.AddSubStrategy(NewMACrossStrategy(5, 20, 0), 1.0)

	t.Run("Basic Properties", func(t *testing.T) {
		assert.Equal(t, "测试组合", combo.Name())
		assert.Equal(t, "测试用组合策略", combo.Description())

		subStrategies := combo.GetSubStrategies()
		assert.Len(t, subStrategies, 2)
	})

	t.Run("Strategy Management", func(t *testing.T) {
		// 测试添加和移除策略
		macdStrategy := NewMACDStrategy(12, 26, 9)
		combo.AddSubStrategy(macdStrategy, 1.5)

		subStrategies := combo.GetSubStrategies()
		assert.Len(t, subStrategies, 3)

		combo.RemoveSubStrategy(macdStrategy.Name())
		subStrategies = combo.GetSubStrategies()
		assert.Len(t, subStrategies, 2)
	})

	t.Run("Evaluation", func(t *testing.T) {
		// 创建足够的数据
		prices := make([]float64, 50)
		for i := 0; i < 50; i++ {
			prices[i] = 100.0 + float64(i)*0.1 // 上升趋势
		}

		data := createTestMarketData("BTCUSDT", Timeframe1h, prices)

		result, err := combo.Evaluate(data)
		require.NoError(t, err)
		require.NotNil(t, result)

		// 验证组合结果
		assert.NotEmpty(t, result.Message)
		assert.Contains(t, result.Metadata, "sub_strategies")
		assert.Contains(t, result.Metadata, "combine_method")
	})
}

func TestStrategyManager(t *testing.T) {
	manager := NewManager(DefaultManagerConfig())

	t.Run("Strategy Registration", func(t *testing.T) {
		rsiStrategy := NewRSIStrategy(14, 70, 30)

		err := manager.RegisterStrategy(rsiStrategy)
		assert.NoError(t, err)

		// 重复注册应该失败
		err = manager.RegisterStrategy(rsiStrategy)
		assert.Error(t, err)

		// 验证策略列表
		strategies := manager.ListStrategies()
		assert.Contains(t, strategies, rsiStrategy.Name())

		// 获取策略
		retrieved, err := manager.GetStrategy(rsiStrategy.Name())
		assert.NoError(t, err)
		assert.Equal(t, rsiStrategy.Name(), retrieved.Name())

		// 注销策略
		err = manager.UnregisterStrategy(rsiStrategy.Name())
		assert.NoError(t, err)

		strategies = manager.ListStrategies()
		assert.NotContains(t, strategies, rsiStrategy.Name())
	})

	t.Run("Evaluation", func(t *testing.T) {
		// 注册多个策略
		manager.RegisterStrategy(NewRSIStrategy(14, 70, 30))
		manager.RegisterStrategy(NewMACrossStrategy(5, 20, 0))

		// 创建测试数据
		prices := make([]float64, 50)
		for i := 0; i < 50; i++ {
			prices[i] = 100.0 + float64(i)*0.1
		}
		data := createTestMarketData("BTCUSDT", Timeframe1h, prices)

		// 评估所有策略
		summary, err := manager.EvaluateAll(data)
		require.NoError(t, err)
		require.NotNil(t, summary)

		assert.Equal(t, 2, len(summary.Results))
		assert.Equal(t, 2, summary.SuccessCount)
		assert.Equal(t, 0, summary.ErrorCount)
	})

	t.Run("Data Validation", func(t *testing.T) {
		// 测试数据验证
		err := manager.ValidateData(nil)
		assert.Error(t, err)

		invalidData := &MarketData{}
		err = manager.ValidateData(invalidData)
		assert.Error(t, err)

		validData := createTestMarketData("BTCUSDT", Timeframe1h, []float64{100, 101, 102})
		err = manager.ValidateData(validData)
		assert.NoError(t, err)
	})
}

func TestStrategyFactory(t *testing.T) {
	factory := NewFactory()

	t.Run("Preset Strategies", func(t *testing.T) {
		presets := factory.ListPresets()
		assert.NotEmpty(t, presets)
		assert.Contains(t, presets, "rsi_conservative")
		assert.Contains(t, presets, "balanced_combo")

		// 创建预设策略
		strategy, err := factory.CreateStrategy("rsi_conservative")
		assert.NoError(t, err)
		assert.NotNil(t, strategy)
		assert.Contains(t, strategy.Name(), "RSI")
	})

	t.Run("Custom Strategies", func(t *testing.T) {
		// 创建自定义RSI策略
		strategy, err := factory.CreateStrategy("rsi", 21, 80.0, 20.0)
		assert.NoError(t, err)
		assert.NotNil(t, strategy)

		// 创建自定义MA策略
		strategy, err = factory.CreateStrategy("ema", 10, 30)
		assert.NoError(t, err)
		assert.NotNil(t, strategy)
		assert.Contains(t, strategy.Name(), "EMA")
	})

	t.Run("Recommended Strategies", func(t *testing.T) {
		// 为不同时间框架获取推荐策略
		strategy, err := factory.CreateRecommendedStrategy(Timeframe5m)
		assert.NoError(t, err)
		assert.NotNil(t, strategy)

		strategy, err = factory.CreateRecommendedStrategy(Timeframe1h)
		assert.NoError(t, err)
		assert.NotNil(t, strategy)

		strategy, err = factory.CreateRecommendedStrategy(Timeframe1d)
		assert.NoError(t, err)
		assert.NotNil(t, strategy)
	})

	t.Run("Custom Preset Registration", func(t *testing.T) {
		// 注册自定义预设
		err := factory.RegisterPreset("custom_test", func() Strategy {
			return NewRSIStrategy(21, 75, 25)
		})
		assert.NoError(t, err)

		// 创建自定义预设策略
		strategy, err := factory.CreateStrategy("custom_test")
		assert.NoError(t, err)
		assert.NotNil(t, strategy)

		// 注销预设
		err = factory.UnregisterPreset("custom_test")
		assert.NoError(t, err)

		// 再次创建应该失败
		strategy, err = factory.CreateStrategy("custom_test")
		assert.Error(t, err)
		assert.Nil(t, strategy)
	})
}
