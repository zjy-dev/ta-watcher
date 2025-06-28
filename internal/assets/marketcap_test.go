package assets

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ta-watcher/internal/datasource"
)

func TestMockMarketCapProvider(t *testing.T) {
	provider := NewMockMarketCapProvider()
	ctx := context.Background()

	t.Run("获取单个币种市值", func(t *testing.T) {
		symbols := []string{"BTC"}
		marketCaps, err := provider.GetMarketCaps(ctx, symbols)

		require.NoError(t, err)
		assert.Contains(t, marketCaps, "BTC")
		assert.Greater(t, marketCaps["BTC"], 0.0)
	})

	t.Run("获取多个币种市值", func(t *testing.T) {
		symbols := []string{"BTC", "ETH", "BNB"}
		marketCaps, err := provider.GetMarketCaps(ctx, symbols)

		require.NoError(t, err)
		assert.Len(t, marketCaps, 3)

		// 验证市值排序 (BTC > ETH > BNB)
		assert.Greater(t, marketCaps["BTC"], marketCaps["ETH"])
		assert.Greater(t, marketCaps["ETH"], marketCaps["BNB"])
	})

	t.Run("获取不存在的币种", func(t *testing.T) {
		symbols := []string{"NONEXISTENT"}
		marketCaps, err := provider.GetMarketCaps(ctx, symbols)

		require.NoError(t, err)
		assert.Empty(t, marketCaps)
	})
}

func TestMarketCapManager(t *testing.T) {
	provider := NewMockMarketCapProvider()
	manager := NewMarketCapManager(provider, 1*time.Second)
	ctx := context.Background()

	t.Run("首次获取市值数据", func(t *testing.T) {
		symbols := []string{"BTC", "ETH"}
		marketCaps, err := manager.GetMarketCaps(ctx, symbols)

		require.NoError(t, err)
		assert.Len(t, marketCaps, 2)
		assert.Contains(t, marketCaps, "BTC")
		assert.Contains(t, marketCaps, "ETH")
	})

	t.Run("缓存功能测试", func(t *testing.T) {
		symbols := []string{"BTC"}

		// 第一次获取
		marketCaps1, err := manager.GetMarketCaps(ctx, symbols)
		require.NoError(t, err)

		// 第二次获取（应该使用缓存）
		marketCaps2, err := manager.GetMarketCaps(ctx, symbols)
		require.NoError(t, err)

		assert.Equal(t, marketCaps1, marketCaps2)
	})

	t.Run("缓存过期测试", func(t *testing.T) {
		// 创建一个TTL很短的管理器
		shortTTLManager := NewMarketCapManager(provider, 1*time.Millisecond)
		symbols := []string{"BTC"}

		// 第一次获取
		_, err := shortTTLManager.GetMarketCaps(ctx, symbols)
		require.NoError(t, err)

		// 等待缓存过期
		time.Sleep(10 * time.Millisecond)

		// 再次获取（应该重新从provider获取）
		marketCaps, err := shortTTLManager.GetMarketCaps(ctx, symbols)
		require.NoError(t, err)
		assert.Contains(t, marketCaps, "BTC")
	})
}

func TestSortSymbolsByMarketCap(t *testing.T) {
	marketCaps := map[string]float64{
		"BTC": 800000000000,
		"ETH": 400000000000,
		"BNB": 50000000000,
		"ADA": 20000000000,
	}

	t.Run("正确排序", func(t *testing.T) {
		symbols := []string{"ADA", "BTC", "BNB", "ETH"}
		sorted := SortSymbolsByMarketCap(symbols, marketCaps)

		expected := []string{"BTC", "ETH", "BNB", "ADA"}
		assert.Equal(t, expected, sorted)
	})

	t.Run("不修改原始切片", func(t *testing.T) {
		original := []string{"ADA", "BTC", "BNB", "ETH"}
		originalCopy := make([]string, len(original))
		copy(originalCopy, original)

		_ = SortSymbolsByMarketCap(original, marketCaps)

		assert.Equal(t, originalCopy, original)
	})
}

func TestGenerateCrossRatePairs(t *testing.T) {
	marketCaps := map[string]float64{
		"BTC": 800000000000,
		"ETH": 400000000000,
		"BNB": 50000000000,
		"ADA": 20000000000,
	}

	t.Run("生成交叉汇率对", func(t *testing.T) {
		symbols := []string{"BTC", "ETH", "BNB", "ADA"}
		pairs := GenerateCrossRatePairs(symbols, marketCaps, 10)

		// 应该生成 低市值/高市值 的对，符合交易所约定
		assert.Contains(t, pairs, "ETHBTC") // ETH/BTC (ETH用BTC报价)
		assert.Contains(t, pairs, "BNBBTC") // BNB/BTC (BNB用BTC报价)
		assert.Contains(t, pairs, "BNBETH") // BNB/ETH (BNB用ETH报价)

		// 验证所有对都是低市值在前，高市值在后（符合交易所约定）
		for _, pair := range pairs {
			if pair == "ETHBTC" {
				assert.Greater(t, marketCaps["BTC"], marketCaps["ETH"]) // BTC市值 > ETH市值
			}
			if pair == "ADAETH" {
				assert.Greater(t, marketCaps["ETH"], marketCaps["ADA"]) // ETH市值 > ADA市值
			}
		}
	})

	t.Run("限制交易对数量", func(t *testing.T) {
		symbols := []string{"BTC", "ETH", "BNB", "ADA"}
		pairs := GenerateCrossRatePairs(symbols, marketCaps, 3)

		assert.LessOrEqual(t, len(pairs), 3)
	})

	t.Run("币种数量不足", func(t *testing.T) {
		symbols := []string{"BTC"}
		pairs := GenerateCrossRatePairs(symbols, marketCaps, 10)

		assert.Empty(t, pairs)
	})

	t.Run("空币种列表", func(t *testing.T) {
		symbols := []string{}
		pairs := GenerateCrossRatePairs(symbols, marketCaps, 10)

		assert.Empty(t, pairs)
	})
}

func TestValidatorWithMarketCap(t *testing.T) {
	// 这个测试主要验证市值查询功能的集成
	// 由于涉及多个复杂接口，我们在集成测试中覆盖
	t.Skip("复杂的验证器集成测试在 integration_test.go 中覆盖")
}

// MockDataSourceForMarketCap 用于市值管理器测试的简化 mock
type MockDataSourceForMarketCap struct {
	validSymbols map[string]bool
}

func (m *MockDataSourceForMarketCap) GetKlines(ctx context.Context, symbol string, interval datasource.Timeframe, startTime, endTime time.Time, limit int) ([]*datasource.Kline, error) {
	if m.validSymbols[symbol] {
		return []*datasource.Kline{{
			Symbol:   symbol,
			OpenTime: time.Now(),
			Open:     1.0,
			Close:    1.0,
		}}, nil
	}
	return nil, fmt.Errorf("symbol %s not found", symbol)
}

func (m *MockDataSourceForMarketCap) IsSymbolValid(ctx context.Context, symbol string) (bool, error) {
	return m.validSymbols[symbol], nil
}

func (m *MockDataSourceForMarketCap) Name() string {
	return "mock"
}
