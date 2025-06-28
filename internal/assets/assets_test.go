package assets

import (
	"context"
	"testing"
	"time"

	"ta-watcher/internal/config"
	"ta-watcher/internal/datasource"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockDataSource 模拟数据源
type MockDataSource struct {
	mock.Mock
}

func (m *MockDataSource) GetKlines(ctx context.Context, symbol string, interval datasource.Timeframe, startTime, endTime time.Time, limit int) ([]*datasource.Kline, error) {
	args := m.Called(ctx, symbol, interval, startTime, endTime, limit)
	return args.Get(0).([]*datasource.Kline), args.Error(1)
}

func (m *MockDataSource) IsSymbolValid(ctx context.Context, symbol string) (bool, error) {
	args := m.Called(ctx, symbol)
	return args.Bool(0), args.Error(1)
}

func (m *MockDataSource) Name() string {
	args := m.Called()
	return args.String(0)
}

func TestValidator_ValidateAssets(t *testing.T) {
	mockClient := new(MockDataSource)

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
	mockClient.On("IsSymbolValid", ctx, "BTCUSDT").Return(true, nil)

	// ETH/USDT 存在
	mockClient.On("IsSymbolValid", ctx, "ETHUSDT").Return(true, nil)

	// INVALID/USDT 不存在
	mockClient.On("IsSymbolValid", ctx, "INVALIDUSDT").Return(false, assert.AnError)

	// 交叉汇率对 ETHBTC 不存在（会被标记为计算汇率对）
	mockClient.On("IsSymbolValid", ctx, "ETHBTC").Return(false, assert.AnError)

	// 执行验证
	result, err := validator.ValidateAssets(ctx)

	// 验证结果
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, []string{"BTC", "ETH"}, result.ValidSymbols)
	assert.Equal(t, []string{"INVALID"}, result.MissingSymbols)
	assert.Equal(t, []string{"BTCUSDT", "ETHUSDT"}, result.ValidPairs)

	// 检查计算汇率对
	assert.Contains(t, result.CalculatedPairs, "ETHBTC")

	// 验证所有预期的mock调用都被执行了
	mockClient.AssertExpectations(t)
}

func TestValidator_ValidateAssets_EmptySymbols(t *testing.T) {
	mockClient := new(MockDataSource)

	assetsConfig := &config.AssetsConfig{
		Symbols:                 []string{},
		Timeframes:              []string{"1d"},
		BaseCurrency:            "USDT",
		MarketCapUpdateInterval: time.Hour,
	}

	validator := NewValidator(mockClient, assetsConfig)
	ctx := context.Background()

	result, err := validator.ValidateAssets(ctx)

	// 空符号列表应该返回错误
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "没有找到任何有效的币种")
}

func TestValidator_ValidateAssets_AllInvalid(t *testing.T) {
	mockClient := new(MockDataSource)

	assetsConfig := &config.AssetsConfig{
		Symbols:                 []string{"INVALID1", "INVALID2"},
		Timeframes:              []string{"1d"},
		BaseCurrency:            "USDT",
		MarketCapUpdateInterval: time.Hour,
	}

	validator := NewValidator(mockClient, assetsConfig)
	ctx := context.Background()

	// 所有币种都无效
	mockClient.On("IsSymbolValid", ctx, "INVALID1USDT").Return(false, assert.AnError)
	mockClient.On("IsSymbolValid", ctx, "INVALID2USDT").Return(false, assert.AnError)

	result, err := validator.ValidateAssets(ctx)

	// 当所有币种都无效时应该返回错误
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "没有找到任何有效的币种")

	mockClient.AssertExpectations(t)
}

func TestRateCalculator_CalculateRate(t *testing.T) {
	mockClient := new(MockDataSource)
	calculator := NewRateCalculator(mockClient)

	ctx := context.Background()

	// 创建模拟的K线数据
	now := time.Now()
	btcKlines := make([]*datasource.Kline, 30)
	ethKlines := make([]*datasource.Kline, 30)

	// 生成30个数据点的模拟K线数据
	for i := 0; i < 30; i++ {
		timestamp := now.Add(-time.Duration(29-i) * time.Hour * 24)
		btcKlines[i] = &datasource.Kline{
			Symbol:    "BTCUSDT",
			OpenTime:  timestamp,
			CloseTime: timestamp.Add(time.Hour * 24),
			Open:      50000.0 + float64(i*100),
			High:      51000.0 + float64(i*100),
			Low:       49000.0 + float64(i*100),
			Close:     50500.0 + float64(i*100),
			Volume:    1000.0,
		}
		ethKlines[i] = &datasource.Kline{
			Symbol:    "ETHUSDT",
			OpenTime:  timestamp,
			CloseTime: timestamp.Add(time.Hour * 24),
			Open:      3000.0 + float64(i*10),
			High:      3100.0 + float64(i*10),
			Low:       2900.0 + float64(i*10),
			Close:     3050.0 + float64(i*10),
			Volume:    2000.0,
		}
	}

	// 设置模拟响应
	mockClient.On("GetKlines", ctx, "BTCUSDT", datasource.Timeframe1d, time.Time{}, time.Time{}, 30).Return(btcKlines, nil)
	mockClient.On("GetKlines", ctx, "ETHUSDT", datasource.Timeframe1d, time.Time{}, time.Time{}, 30).Return(ethKlines, nil)

	// 计算ETH/BTC汇率
	result, err := calculator.CalculateRate(ctx, "ETH", "BTC", "USDT", datasource.Timeframe1d, 20)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result, 20) // 请求20个数据点

	// 验证汇率计算的正确性
	for _, kline := range result {
		assert.True(t, kline.Open > 0)
		assert.True(t, kline.Close > 0)
		assert.True(t, kline.High >= kline.Low)
		assert.Equal(t, "ETHBTC", kline.Symbol)
	}

	mockClient.AssertExpectations(t)
}

func TestRateCalculator_CalculateRate_InsufficientData(t *testing.T) {
	mockClient := new(MockDataSource)
	calculator := NewRateCalculator(mockClient)

	ctx := context.Background()

	// 创建不足的K线数据（只有5个数据点）
	now := time.Now()
	btcKlines := make([]*datasource.Kline, 5)
	ethKlines := make([]*datasource.Kline, 5)

	for i := 0; i < 5; i++ {
		timestamp := now.Add(-time.Duration(4-i) * time.Hour * 24)
		btcKlines[i] = &datasource.Kline{
			Symbol:    "BTCUSDT",
			OpenTime:  timestamp,
			CloseTime: timestamp.Add(time.Hour * 24),
			Open:      50000.0,
			High:      51000.0,
			Low:       49000.0,
			Close:     50500.0,
			Volume:    1000.0,
		}
		ethKlines[i] = &datasource.Kline{
			Symbol:    "ETHUSDT",
			OpenTime:  timestamp,
			CloseTime: timestamp.Add(time.Hour * 24),
			Open:      3000.0,
			High:      3100.0,
			Low:       2900.0,
			Close:     3050.0,
			Volume:    2000.0,
		}
	}

	// 设置模拟响应
	mockClient.On("GetKlines", ctx, "BTCUSDT", datasource.Timeframe1d, time.Time{}, time.Time{}, 30).Return(btcKlines, nil)
	mockClient.On("GetKlines", ctx, "ETHUSDT", datasource.Timeframe1d, time.Time{}, time.Time{}, 30).Return(ethKlines, nil)

	// 尝试计算汇率，应该失败因为数据不足
	_, err := calculator.CalculateRate(ctx, "ETH", "BTC", "USDT", datasource.Timeframe1d, 20)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient kline data for rate calculation")

	mockClient.AssertExpectations(t)
}

func TestRateCalculator_GetAvailableRatePairs(t *testing.T) {
	mockClient := new(MockDataSource)
	calculator := NewRateCalculator(mockClient)

	ctx := context.Background()

	// 设置模拟响应
	mockClient.On("GetKlines", ctx, "BTCUSDT", datasource.Timeframe1d, time.Time{}, time.Time{}, 1).Return([]*datasource.Kline{{}}, nil)
	mockClient.On("GetKlines", ctx, "ETHUSDT", datasource.Timeframe1d, time.Time{}, time.Time{}, 1).Return([]*datasource.Kline{{}}, nil)
	mockClient.On("GetKlines", ctx, "INVALIDUSDT", datasource.Timeframe1d, time.Time{}, time.Time{}, 1).Return([]*datasource.Kline{}, assert.AnError)

	available, unavailable, err := calculator.GetAvailableRatePairs(ctx, []string{"BTC", "ETH", "INVALID"}, "USDT")

	require.NoError(t, err)
	assert.Equal(t, []string{"BTC", "ETH"}, available)
	assert.Equal(t, []string{"INVALID"}, unavailable)

	mockClient.AssertExpectations(t)
}

func TestSafeDiv(t *testing.T) {
	tests := []struct {
		name     string
		a, b     float64
		expected float64
	}{
		{"正常除法", 10.0, 2.0, 5.0},
		{"除零", 10.0, 0.0, 0.0},
		{"被除数为零", 0.0, 5.0, 0.0},
		{"两者都为零", 0.0, 0.0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := safeDiv(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}
