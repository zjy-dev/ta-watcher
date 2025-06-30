//go:build integration

package assets

import (
	"context"
	"testing"
	"time"

	"ta-watcher/internal/config"
	"ta-watcher/internal/datasource"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_AssetsValidationWorkflow 集成测试：完整的资产验证工作流
func TestIntegration_AssetsValidationWorkflow(t *testing.T) {
	// 跳过集成测试，除非显式指定
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	// 从配置文件加载配置
	cfg, err := config.LoadConfig("../../config.yaml")
	require.NoError(t, err, "应该能够加载配置文件")

	// 创建数据源客户端
	factory := datasource.NewFactory()
	dataSource, err := factory.CreateDataSource(cfg.DataSource.Primary, cfg)
	require.NoError(t, err, "应该能够创建数据源客户端")

	t.Logf("使用数据源: %s", dataSource.Name())

	// 测试连接
	ctx := context.Background()
	isValid, err := dataSource.IsSymbolValid(ctx, "BTCUSDT")
	require.NoError(t, err, "应该能够检查符号有效性")
	assert.True(t, isValid, "BTCUSDT 应该是有效的交易对")

	// 验证资产
	validator := NewValidator(dataSource, &cfg.Assets)
	result, err := validator.ValidateAssets(ctx)
	require.NoError(t, err, "资产验证应该成功")
	require.NotNil(t, result, "验证结果不应该为空")

	// 验证结果
	assert.NotEmpty(t, result.ValidSymbols, "应该有有效的币种")
	assert.NotEmpty(t, result.ValidPairs, "应该有有效的交易对")

	// 验证期望的交易对（根据config.yaml中的实际配置）
	expectedPairs := []string{"ADAUSDT", "SOLUSDT"}
	for _, pair := range expectedPairs {
		found := false
		for _, validPair := range result.ValidPairs {
			if validPair == pair {
				found = true
				break
			}
		}
		assert.True(t, found, "应该找到交易对: %s", pair)
	}

	t.Logf("验证完成: %d 个有效币种, %d 个有效交易对",
		len(result.ValidSymbols), len(result.ValidPairs))
}

// TestIntegration_RateCalculation 集成测试：汇率计算功能
func TestIntegration_RateCalculation(t *testing.T) {
	// 跳过集成测试，除非显式指定
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	// 从配置文件加载基础配置
	cfg, err := config.LoadConfig("../../config.yaml")
	require.NoError(t, err, "应该能够加载配置文件")

	// 创建数据源客户端
	factory := datasource.NewFactory()
	dataSource, err := factory.CreateDataSource(cfg.DataSource.Primary, cfg)
	require.NoError(t, err, "应该能够创建数据源客户端")

	// 测试各种配置场景
	// 测试用例统一使用 USDT 作为桥接货币
	// Coinbase 现在支持 USDT 交易对
	testCases := []struct {
		name         string
		baseSymbol   string
		quoteSymbol  string
		bridge       string
		shouldPass   bool
		minDataCount int
	}{
		{
			name:         "BTC/ETH 汇率计算",
			baseSymbol:   "BTC",
			quoteSymbol:  "ETH",
			bridge:       "USDT",
			shouldPass:   true,
			minDataCount: 20,
		},
		{
			name:         "ETH/BTC 汇率计算",
			baseSymbol:   "ETH",
			quoteSymbol:  "BTC",
			bridge:       "USDT",
			shouldPass:   true,
			minDataCount: 20,
		},
		{
			name:         "无效币种组合",
			baseSymbol:   "INVALID",
			quoteSymbol:  "BTC",
			bridge:       "USDT",
			shouldPass:   false,
			minDataCount: 0,
		},
	}

	ctx := context.Background()
	calculator := NewRateCalculator(dataSource)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 计算汇率
			now := time.Now()
			startTime := now.Add(-60 * 24 * time.Hour) // 60天前
			klines, err := calculator.CalculateRate(
				ctx,
				tc.baseSymbol,
				tc.quoteSymbol,
				tc.bridge,
				datasource.Timeframe1d,
				startTime,
				now,
				50,
			)

			if tc.shouldPass {
				require.NoError(t, err, "汇率计算应该成功")
				assert.GreaterOrEqual(t, len(klines), tc.minDataCount,
					"应该有足够的K线数据")

				// 验证K线数据的有效性
				for i, kline := range klines {
					assert.Positive(t, kline.Open, "第 %d 个K线的开盘价应该为正", i)
					assert.Positive(t, kline.Close, "第 %d 个K线的收盘价应该为正", i)
					assert.Positive(t, kline.High, "第 %d 个K线的最高价应该为正", i)
					assert.Positive(t, kline.Low, "第 %d 个K线的最低价应该为正", i)
					assert.GreaterOrEqual(t, kline.High, kline.Low,
						"第 %d 个K线的最高价应该不小于最低价", i)
					assert.GreaterOrEqual(t, kline.High, kline.Open,
						"第 %d 个K线的最高价应该不小于开盘价", i)
					assert.GreaterOrEqual(t, kline.High, kline.Close,
						"第 %d 个K线的最高价应该不小于收盘价", i)
					assert.LessOrEqual(t, kline.Low, kline.Open,
						"第 %d 个K线的最低价应该不大于开盘价", i)
					assert.LessOrEqual(t, kline.Low, kline.Close,
						"第 %d 个K线的最低价应该不大于收盘价", i)
				}

				t.Logf("%s: 计算成功，生成 %d 个K线数据", tc.name, len(klines))
			} else {
				assert.Error(t, err, "无效币种组合应该返回错误")
				t.Logf("%s: 预期错误，实际错误: %v", tc.name, err)
			}
		})
	}
}

// TestIntegration_MarketCapData 集成测试：市值数据功能
func TestIntegration_MarketCapData(t *testing.T) {
	// 跳过集成测试，除非显式指定
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	provider := NewMockMarketCapProvider()
	manager := NewMarketCapManager(provider, 5*time.Minute)

	ctx := context.Background()

	// 测试获取市值数据
	symbols := []string{"BTC", "ETH", "ADA"}
	marketCaps, err := manager.GetMarketCaps(ctx, symbols)
	require.NoError(t, err, "应该能够获取市值数据")
	assert.Equal(t, len(symbols), len(marketCaps), "市值数据数量应该匹配")

	// 验证市值数据
	for _, symbol := range symbols {
		marketCap, exists := marketCaps[symbol]
		assert.True(t, exists, "应该存在 %s 的市值数据", symbol)
		assert.Positive(t, marketCap, "%s 的市值应该为正数", symbol)
	}

	// 测试按市值排序
	sortedSymbols := SortSymbolsByMarketCap(symbols, marketCaps)
	assert.Equal(t, len(symbols), len(sortedSymbols), "排序后的符号数量应该一致")

	// 验证排序结果（降序）
	for i := 0; i < len(sortedSymbols)-1; i++ {
		currentMarketCap := marketCaps[sortedSymbols[i]]
		nextMarketCap := marketCaps[sortedSymbols[i+1]]
		assert.GreaterOrEqual(t, currentMarketCap, nextMarketCap,
			"市值应该按降序排列: %s (%.0f) >= %s (%.0f)",
			sortedSymbols[i], currentMarketCap,
			sortedSymbols[i+1], nextMarketCap)
	}

	t.Logf("市值排序结果: %v", sortedSymbols)
}

// TestIntegration_CompleteWorkflow 集成测试：完整的工作流程
func TestIntegration_CompleteWorkflow(t *testing.T) {
	// 跳过集成测试，除非显式指定
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	// 从配置文件加载配置
	cfg, err := config.LoadConfig("../../config.yaml")
	require.NoError(t, err, "应该能够加载配置文件")

	// 创建数据源客户端
	factory := datasource.NewFactory()
	dataSource, err := factory.CreateDataSource(cfg.DataSource.Primary, cfg)
	require.NoError(t, err, "应该能够创建数据源客户端")

	ctx := context.Background()

	// 步骤1：验证资产
	t.Log("步骤1：验证资产配置")
	validator := NewValidator(dataSource, &cfg.Assets)
	result, err := validator.ValidateAssets(ctx)
	require.NoError(t, err, "资产验证应该成功")

	t.Logf("验证结果: %d 个有效币种, %d 个有效交易对, %d 个计算汇率对",
		len(result.ValidSymbols), len(result.ValidPairs), len(result.CalculatedPairs))

	// 步骤2：测试汇率计算（如果有计算汇率对）
	if len(result.CalculatedPairs) > 0 {
		t.Log("步骤2：测试汇率计算")
		calculator := NewRateCalculator(dataSource)

		for _, pair := range result.CalculatedPairs {
			// 假设交易对格式为 BASEQUOTE（如 ETHBTC）
			// 这里简化处理，实际应该有更好的解析逻辑
			if len(pair) >= 6 {
				baseSymbol := pair[:3]
				quoteSymbol := pair[3:6]

				t.Logf("计算汇率: %s/%s", baseSymbol, quoteSymbol)

				now := time.Now()
				startTime := now.Add(-45 * 24 * time.Hour) // 45天前
				klines, err := calculator.CalculateRate(
					ctx,
					baseSymbol,
					quoteSymbol,
					"USDT",
					datasource.Timeframe1d,
					startTime,
					now,
					30,
				)

				if err != nil {
					t.Logf("汇率计算失败: %v", err)
				} else {
					assert.NotEmpty(t, klines, "应该有汇率数据")
					t.Logf("汇率计算成功: %d 个数据点", len(klines))
				}
			}
		}
	}

	t.Log("完整工作流程测试完成")
}
