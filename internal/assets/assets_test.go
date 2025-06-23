package assets

import (
	"context"
	"testing"
	"time"

	"ta-watcher/internal/binance"
	"ta-watcher/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockDataSource 模拟数据源
type MockDataSource struct {
	mock.Mock
}

func (m *MockDataSource) GetKlines(ctx context.Context, symbol, interval string, limit int) ([]*binance.KlineData, error) {
	args := m.Called(ctx, symbol, interval, limit)
	return args.Get(0).([]*binance.KlineData), args.Error(1)
}

func (m *MockDataSource) GetPrice(ctx context.Context, symbol string) (*binance.PriceData, error) {
	args := m.Called(ctx, symbol)
	return args.Get(0).(*binance.PriceData), args.Error(1)
}

func (m *MockDataSource) GetPrices(ctx context.Context, symbols []string) ([]*binance.PriceData, error) {
	args := m.Called(ctx, symbols)
	return args.Get(0).([]*binance.PriceData), args.Error(1)
}

func (m *MockDataSource) GetKlinesWithTimeRange(ctx context.Context, symbol, interval string, startTime, endTime time.Time) ([]*binance.KlineData, error) {
	args := m.Called(ctx, symbol, interval, startTime, endTime)
	return args.Get(0).([]*binance.KlineData), args.Error(1)
}

func (m *MockDataSource) GetTicker24hr(ctx context.Context, symbol string) (*binance.TickerData, error) {
	args := m.Called(ctx, symbol)
	return args.Get(0).(*binance.TickerData), args.Error(1)
}

func (m *MockDataSource) GetAllTickers24hr(ctx context.Context) ([]*binance.TickerData, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*binance.TickerData), args.Error(1)
}

func (m *MockDataSource) GetExchangeInfo(ctx context.Context) (*binance.ExchangeInfo, error) {
	args := m.Called(ctx)
	return args.Get(0).(*binance.ExchangeInfo), args.Error(1)
}

func (m *MockDataSource) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockDataSource) GetServerTime(ctx context.Context) (time.Time, error) {
	args := m.Called(ctx)
	return args.Get(0).(time.Time), args.Error(1)
}

func TestValidator_ValidateAssets(t *testing.T) {
	// 创建模拟数据源
	mockClient := new(MockDataSource)

	// 配置资产
	assetsConfig := &config.AssetsConfig{
		Symbols:                 []string{"BTC", "ETH", "INVALID"},
		Timeframes:              []string{"1d", "1w"},
		BaseCurrency:            "USDT",
		MarketCapUpdateInterval: time.Hour,
	}

	validator := NewValidator(mockClient, assetsConfig)

	// 设置模拟响应
	ctx := context.Background()

	// BTC/USDT 存在
	mockClient.On("GetKlines", ctx, "BTCUSDT", "1d", 1).Return([]*binance.KlineData{
		{
			Symbol:    "BTCUSDT",
			OpenTime:  time.Now(),
			CloseTime: time.Now(),
			Open:      50000.0,
			Close:     51000.0,
		},
	}, nil)

	// ETH/USDT 存在
	mockClient.On("GetKlines", ctx, "ETHUSDT", "1d", 1).Return([]*binance.KlineData{
		{
			Symbol:    "ETHUSDT",
			OpenTime:  time.Now(),
			CloseTime: time.Now(),
			Open:      3000.0,
			Close:     3100.0,
		},
	}, nil)

	// INVALID/USDT 不存在
	mockClient.On("GetKlines", ctx, "INVALIDUSDT", "1d", 1).Return([]*binance.KlineData{},
		assert.AnError)

	// 新增：交叉汇率对 BTC/ETH 不存在（将被标记为计算汇率对）
	mockClient.On("GetKlines", ctx, "BTCETH", "1d", 1).Return([]*binance.KlineData{},
		assert.AnError)

	// 执行验证
	result, err := validator.ValidateAssets(ctx)

	// 验证结果
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, []string{"BTC", "ETH"}, result.ValidSymbols)
	assert.Contains(t, result.ValidPairs, "BTCUSDT")
	assert.Contains(t, result.ValidPairs, "ETHUSDT")
	assert.Equal(t, []string{"INVALID"}, result.MissingSymbols)
	assert.Equal(t, []string{"1d", "1w"}, result.SupportedTimeframes)

	// 验证生成了计算汇率对
	assert.Contains(t, result.CalculatedPairs, "BTCETH")

	// 验证摘要
	summary := result.Summary()
	assert.Contains(t, summary, "有效币种: 2个")
	assert.Contains(t, summary, "缺失币种: INVALID")

	mockClient.AssertExpectations(t)
}

func TestValidator_ValidateAssets_NoValidSymbols(t *testing.T) {
	mockClient := new(MockDataSource)

	assetsConfig := &config.AssetsConfig{
		Symbols:                 []string{"INVALID1", "INVALID2"},
		Timeframes:              []string{"1d"},
		BaseCurrency:            "USDT",
		MarketCapUpdateInterval: time.Hour,
	}

	validator := NewValidator(mockClient, assetsConfig)
	ctx := context.Background()

	// 所有交易对都不存在
	mockClient.On("GetKlines", ctx, "INVALID1USDT", "1d", 1).Return([]*binance.KlineData{}, assert.AnError)
	mockClient.On("GetKlines", ctx, "INVALID2USDT", "1d", 1).Return([]*binance.KlineData{}, assert.AnError)

	// 执行验证
	result, err := validator.ValidateAssets(ctx)

	// 应该返回错误
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "没有找到任何有效的币种")

	mockClient.AssertExpectations(t)
}

func TestRateCalculator_CalculateRate(t *testing.T) {
	mockClient := new(MockDataSource)
	calculator := NewRateCalculator(mockClient)

	ctx := context.Background()
	baseTime := time.Now().Truncate(time.Hour)

	// 模拟 BTC/USDT 数据
	btcKlines := []*binance.KlineData{
		{
			Symbol:    "BTCUSDT",
			OpenTime:  baseTime,
			CloseTime: baseTime.Add(time.Hour),
			Open:      50000.0,
			High:      52000.0,
			Low:       49000.0,
			Close:     51000.0,
		},
	}

	// 模拟 ETH/USDT 数据
	ethKlines := []*binance.KlineData{
		{
			Symbol:    "ETHUSDT",
			OpenTime:  baseTime,
			CloseTime: baseTime.Add(time.Hour),
			Open:      2500.0,
			High:      2600.0,
			Low:       2400.0,
			Close:     2550.0,
		},
	}

	mockClient.On("GetKlines", ctx, "BTCUSDT", "1h", 1).Return(btcKlines, nil)
	mockClient.On("GetKlines", ctx, "ETHUSDT", "1h", 1).Return(ethKlines, nil)

	// 执行汇率计算
	result, err := calculator.CalculateRate(ctx, "BTC", "ETH", "USDT", "1h", 1)

	// 验证结果
	require.NoError(t, err)
	require.Len(t, result, 1)

	rate := result[0]
	assert.Equal(t, "BTCETH", rate.Symbol)
	assert.Equal(t, baseTime, rate.OpenTime)

	// 验证汇率计算: BTC/ETH = (BTC/USDT) / (ETH/USDT)
	expectedOpen := 50000.0 / 2500.0  // 20.0
	expectedClose := 51000.0 / 2550.0 // 约20.0

	assert.InDelta(t, expectedOpen, rate.Open, 0.01)
	assert.InDelta(t, expectedClose, rate.Close, 0.01)

	mockClient.AssertExpectations(t)
}

func TestRateCalculator_GetAvailableRatePairs(t *testing.T) {
	mockClient := new(MockDataSource)
	calculator := NewRateCalculator(mockClient)

	ctx := context.Background()
	symbols := []string{"BTC", "ETH", "INVALID"}

	// BTC/USDT 存在
	mockClient.On("GetKlines", ctx, "BTCUSDT", "1d", 1).Return([]*binance.KlineData{{}}, nil)

	// ETH/USDT 存在
	mockClient.On("GetKlines", ctx, "ETHUSDT", "1d", 1).Return([]*binance.KlineData{{}}, nil)

	// INVALID/USDT 不存在
	mockClient.On("GetKlines", ctx, "INVALIDUSDT", "1d", 1).Return([]*binance.KlineData{}, assert.AnError)

	// 执行检查
	available, unavailable, err := calculator.GetAvailableRatePairs(ctx, symbols, "USDT")

	// 验证结果
	require.NoError(t, err)
	assert.Equal(t, []string{"BTC", "ETH"}, available)
	assert.Equal(t, []string{"INVALID"}, unavailable)

	mockClient.AssertExpectations(t)
}

func TestAssetsConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      config.AssetsConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: config.AssetsConfig{
				Symbols:                 []string{"BTC", "ETH"},
				Timeframes:              []string{"1d", "1w"},
				BaseCurrency:            "USDT",
				MarketCapUpdateInterval: time.Hour,
			},
			expectError: false,
		},
		{
			name: "empty symbols",
			config: config.AssetsConfig{
				Symbols:                 []string{},
				Timeframes:              []string{"1d"},
				BaseCurrency:            "USDT",
				MarketCapUpdateInterval: time.Hour,
			},
			expectError: true,
			errorMsg:    "symbols list cannot be empty",
		},
		{
			name: "empty timeframes",
			config: config.AssetsConfig{
				Symbols:                 []string{"BTC"},
				Timeframes:              []string{},
				BaseCurrency:            "USDT",
				MarketCapUpdateInterval: time.Hour,
			},
			expectError: true,
			errorMsg:    "timeframes list cannot be empty",
		},
		{
			name: "invalid timeframe",
			config: config.AssetsConfig{
				Symbols:                 []string{"BTC"},
				Timeframes:              []string{"1x"},
				BaseCurrency:            "USDT",
				MarketCapUpdateInterval: time.Hour,
			},
			expectError: true,
			errorMsg:    "invalid timeframe: 1x",
		},
		{
			name: "empty base currency",
			config: config.AssetsConfig{
				Symbols:                 []string{"BTC"},
				Timeframes:              []string{"1d"},
				BaseCurrency:            "",
				MarketCapUpdateInterval: time.Hour,
			},
			expectError: true,
			errorMsg:    "base_currency cannot be empty",
		},
		{
			name: "invalid update interval",
			config: config.AssetsConfig{
				Symbols:                 []string{"BTC"},
				Timeframes:              []string{"1d"},
				BaseCurrency:            "USDT",
				MarketCapUpdateInterval: 0,
			},
			expectError: true,
			errorMsg:    "market_cap_update_interval must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
