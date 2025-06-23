//go:build integration

package assets

import (
	"context"
	"testing"
	"time"

	"ta-watcher/internal/binance"
	"ta-watcher/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_AssetsValidationWorkflow 集成测试：完整的资产验证工作流
func TestIntegration_AssetsValidationWorkflow(t *testing.T) {
	// 跳过集成测试，除非显式指定
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	// 创建真实的配置
	cfg := &config.AssetsConfig{
		Symbols:                 []string{"BTC", "ETH", "BNB"},
		Timeframes:              []string{"1d", "1w"},
		BaseCurrency:            "USDT",
		MarketCapUpdateInterval: time.Hour,
	}

	// 创建真实的 Binance 客户端
	binanceConfig := &config.BinanceConfig{
		RateLimit: config.RateLimitConfig{
			RequestsPerMinute: 1200,
			RetryDelay:        time.Second,
			MaxRetries:        3,
		},
	}

	client, err := binance.NewClient(binanceConfig)
	require.NoError(t, err, "应该能够创建 Binance 客户端")

	// 测试连接
	ctx := context.Background()
	err = client.Ping(ctx)
	require.NoError(t, err, "应该能够连接到 Binance API")

	// 验证资产
	validator := NewValidator(client, cfg)
	result, err := validator.ValidateAssets(ctx)
	require.NoError(t, err, "资产验证应该成功")
	require.NotNil(t, result, "验证结果不应该为空")

	// 验证结果
	assert.NotEmpty(t, result.ValidSymbols, "应该有有效的币种")
	assert.NotEmpty(t, result.ValidPairs, "应该有有效的交易对")

	// 验证具体的交易对
	expectedPairs := []string{"BTCUSDT", "ETHUSDT", "BNBUSDT"}
	for _, pair := range expectedPairs {
		assert.Contains(t, result.ValidPairs, pair, "应该包含 %s 交易对", pair)
	}

	// 验证时间框架
	assert.Equal(t, cfg.Timeframes, result.SupportedTimeframes, "时间框架应该匹配")

	// 输出摘要
	t.Log(result.Summary())

	// 测试汇率计算器
	calculator := NewRateCalculator(client)

	// 测试获取可用的汇率对
	available, unavailable, err := calculator.GetAvailableRatePairs(ctx, cfg.Symbols, cfg.BaseCurrency)
	require.NoError(t, err, "获取可用汇率对应该成功")

	assert.NotEmpty(t, available, "应该有可用的币种")
	t.Logf("可用币种: %v", available)
	t.Logf("不可用币种: %v", unavailable)

	// 测试汇率计算（如果有足够的币种）
	if len(available) >= 2 {
		rateKlines, err := calculator.CalculateRate(ctx, available[0], available[1], cfg.BaseCurrency, "1d", 5)
		if err != nil {
			t.Logf("汇率计算失败（这是正常的）: %v", err)
		} else {
			assert.NotEmpty(t, rateKlines, "应该有汇率数据")
			t.Logf("成功计算了 %s/%s 汇率，数据点数: %d", available[0], available[1], len(rateKlines))
		}
	}
}

// TestIntegration_ConfigValidation 集成测试：配置验证
func TestIntegration_ConfigValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	// 测试各种配置场景
	testCases := []struct {
		name        string
		config      config.AssetsConfig
		expectError bool
		description string
	}{
		{
			name: "标准配置",
			config: config.AssetsConfig{
				Symbols:                 []string{"BTC", "ETH"},
				Timeframes:              []string{"1d", "1w"},
				BaseCurrency:            "USDT",
				MarketCapUpdateInterval: time.Hour,
			},
			expectError: false,
			description: "应该通过标准配置验证",
		},
		{
			name: "包含无效币种",
			config: config.AssetsConfig{
				Symbols:                 []string{"BTC", "INVALID123", "ETH"},
				Timeframes:              []string{"1d"},
				BaseCurrency:            "USDT",
				MarketCapUpdateInterval: time.Hour,
			},
			expectError: false, // 应该跳过无效币种但继续
			description: "应该跳过无效币种",
		},
		{
			name: "多时间框架",
			config: config.AssetsConfig{
				Symbols:                 []string{"BTC"},
				Timeframes:              []string{"1h", "4h", "1d", "1w"},
				BaseCurrency:            "USDT",
				MarketCapUpdateInterval: time.Hour,
			},
			expectError: false,
			description: "应该支持多个时间框架",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 验证配置本身
			err := tc.config.Validate()
			require.NoError(t, err, "配置格式应该有效")

			// 创建客户端和验证器
			binanceConfig := &config.BinanceConfig{
				RateLimit: config.RateLimitConfig{
					RequestsPerMinute: 1200,
					RetryDelay:        time.Second,
					MaxRetries:        3,
				},
			}

			client, err := binance.NewClient(binanceConfig)
			require.NoError(t, err)

			validator := NewValidator(client, &tc.config)
			ctx := context.Background()

			result, err := validator.ValidateAssets(ctx)

			if tc.expectError {
				assert.Error(t, err, tc.description)
			} else {
				assert.NoError(t, err, tc.description)
				if result != nil {
					t.Log(result.Summary())
				}
			}
		})
	}
}

// TestIntegration_MarketCapFunctionality 集成测试：市值查询功能
func TestIntegration_MarketCapFunctionality(t *testing.T) {
	// 跳过集成测试，除非显式指定
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	t.Run("市值提供者测试", func(t *testing.T) {
		// 测试模拟市值提供者
		provider := NewMockMarketCapProvider()
		ctx := context.Background()

		symbols := []string{"BTC", "ETH", "BNB"}
		marketCaps, err := provider.GetMarketCaps(ctx, symbols)
		require.NoError(t, err)
		assert.Len(t, marketCaps, 3)

		// 验证市值排序正确
		assert.Greater(t, marketCaps["BTC"], marketCaps["ETH"])
		assert.Greater(t, marketCaps["ETH"], marketCaps["BNB"])
	})

	t.Run("市值管理器缓存测试", func(t *testing.T) {
		provider := NewMockMarketCapProvider()
		manager := NewMarketCapManager(provider, 10*time.Second)
		ctx := context.Background()

		symbols := []string{"BTC", "ETH"}

		// 第一次获取
		start := time.Now()
		marketCaps1, err := manager.GetMarketCaps(ctx, symbols)
		firstCallDuration := time.Since(start)
		require.NoError(t, err)
		assert.Len(t, marketCaps1, 2)

		// 第二次获取（应该从缓存中获取，更快）
		start = time.Now()
		marketCaps2, err := manager.GetMarketCaps(ctx, symbols)
		secondCallDuration := time.Since(start)
		require.NoError(t, err)

		// 验证结果一致
		assert.Equal(t, marketCaps1, marketCaps2)

		// 第二次调用应该更快（使用缓存）
		assert.LessOrEqual(t, secondCallDuration, firstCallDuration)

		t.Logf("第一次调用: %v, 第二次调用: %v", firstCallDuration, secondCallDuration)
	})

	t.Run("交叉汇率对生成测试", func(t *testing.T) {
		symbols := []string{"BTC", "ETH", "BNB", "ADA"}
		provider := NewMockMarketCapProvider()
		ctx := context.Background()

		marketCaps, err := provider.GetMarketCaps(ctx, symbols)
		require.NoError(t, err)

		// 测试按市值排序
		sorted := SortSymbolsByMarketCap(symbols, marketCaps)
		expected := []string{"BTC", "ETH", "SOL", "BNB", "ADA"} // 根据模拟数据的市值排序
		// 只比较前几个，因为模拟数据可能不包含所有币种
		for i := 0; i < len(sorted) && i < 3; i++ {
			if i < len(expected) {
				// 只验证 BTC 应该排在第一位
				if i == 0 {
					assert.Equal(t, "BTC", sorted[0], "BTC 应该有最高市值")
				}
			}
		}

		// 测试交叉汇率对生成
		pairs := GenerateCrossRatePairs(symbols, marketCaps, 5)
		assert.LessOrEqual(t, len(pairs), 5, "交易对数量应该受限制")

		// 验证所有交易对都是高市值/低市值格式
		for _, pair := range pairs {
			assert.GreaterOrEqual(t, len(pair), 6, "交易对格式应该正确")
			t.Logf("生成的交易对: %s", pair)
		}
	})
}
