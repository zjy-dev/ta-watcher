package coinbase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDefaultConfig 测试默认配置
func TestDefaultConfig(t *testing.T) {
	cfg := &Config{
		RateLimit: struct {
			RequestsPerMinute int           `yaml:"requests_per_minute"`
			RetryDelay        time.Duration `yaml:"retry_delay"`
			MaxRetries        int           `yaml:"max_retries"`
		}{
			RequestsPerMinute: 10,
			MaxRetries:        3,
			RetryDelay:        time.Second,
		},
	}

	assert.NotNil(t, cfg)
	assert.Equal(t, 10, cfg.RateLimit.RequestsPerMinute)
	assert.Equal(t, time.Second*1, cfg.RateLimit.RetryDelay)
	assert.Equal(t, 3, cfg.RateLimit.MaxRetries)
}

// TestNewClient 测试客户端创建
func TestNewClient(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		hasErr bool
	}{
		{
			name:   "with default config",
			config: nil,
			hasErr: false,
		},
		{
			name: "with custom config",
			config: &Config{
				RateLimit: struct {
					RequestsPerMinute int           `yaml:"requests_per_minute"`
					RetryDelay        time.Duration `yaml:"retry_delay"`
					MaxRetries        int           `yaml:"max_retries"`
				}{
					RequestsPerMinute: 5,
					MaxRetries:        2,
					RetryDelay:        time.Second * 2,
				},
			},
			hasErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.config)
			assert.NotNil(t, client)
			assert.NotNil(t, client.httpClient)
			assert.Equal(t, "https://api.exchange.coinbase.com", client.baseURL)
		})
	}
}

// TestConvertSymbol 测试符号转换
func TestConvertSymbol(t *testing.T) {
	tests := []struct {
		binanceSymbol  string
		coinbaseSymbol string
	}{
		{"BTCUSDT", "BTC-USD"},
		{"BTCUSD", "BTC-USD"},
		{"ETHUSDT", "ETH-USD"},
		{"ETHUSD", "ETH-USD"},
		{"ADAUSDT", "ADA-USD"},
		{"BNBUSDT", "BNB-USD"},
		{"DOTUSDT", "DOT-USD"},
		{"LINKUSDT", "LINK-USD"},
	}

	for _, tt := range tests {
		t.Run(tt.binanceSymbol, func(t *testing.T) {
			result := convertSymbol(tt.binanceSymbol)
			assert.Equal(t, tt.coinbaseSymbol, result)
		})
	}
}

// TestConvertInterval 测试时间间隔转换
func TestConvertInterval(t *testing.T) {
	tests := []struct {
		binanceInterval     string
		coinbaseGranularity int
	}{
		{"1m", 60},
		{"5m", 300},
		{"15m", 900},
		{"1h", 3600},
		{"4h", 14400},
		{"1d", 86400},
	}

	for _, tt := range tests {
		t.Run(tt.binanceInterval, func(t *testing.T) {
			result, err := convertInterval(tt.binanceInterval)
			assert.NoError(t, err)
			assert.Equal(t, tt.coinbaseGranularity, result)
		})
	}
}

// TestKlineDataStructure 测试K线数据结构
func TestKlineDataStructure(t *testing.T) {
	kline := Kline{
		OpenTime:  1640995200000,
		Open:      "47000.00",
		High:      "48000.00",
		Low:       "46500.00",
		Close:     "47500.00",
		Volume:    "100.5",
		CloseTime: 1640995260000,
	}

	assert.Equal(t, int64(1640995200000), kline.OpenTime)
	assert.Equal(t, "47000.00", kline.Open)
	assert.Equal(t, "48000.00", kline.High)
	assert.Equal(t, "46500.00", kline.Low)
	assert.Equal(t, "47500.00", kline.Close)
	assert.Equal(t, "100.5", kline.Volume)
	assert.Equal(t, int64(1640995260000), kline.CloseTime)
}

// TestTickerDataStructure 测试价格行情数据结构
func TestTickerDataStructure(t *testing.T) {
	ticker := Ticker{
		TradeID: 12345,
		Price:   "47500.00",
		Size:    "0.1",
		Time:    "2021-12-31T16:00:00.000000Z",
		Bid:     "47450.00",
		Ask:     "47550.00",
		Volume:  "1000.5",
	}

	assert.Equal(t, 12345, ticker.TradeID)
	assert.Equal(t, "47500.00", ticker.Price)
	assert.Equal(t, "0.1", ticker.Size)
	assert.Equal(t, "2021-12-31T16:00:00.000000Z", ticker.Time)
	assert.Equal(t, "47450.00", ticker.Bid)
	assert.Equal(t, "47550.00", ticker.Ask)
	assert.Equal(t, "1000.5", ticker.Volume)
}

// TestAPIError 测试API错误结构
func TestAPIError(t *testing.T) {
	tests := []struct {
		name     string
		err      APIError
		expected string
	}{
		{
			name: "error with code",
			err: APIError{
				Message: "Invalid request",
				Code:    "400",
			},
			expected: "Coinbase API错误 [400]: Invalid request",
		},
		{
			name: "error without code",
			err: APIError{
				Message: "Server error",
			},
			expected: "Coinbase API错误: Server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

// TestClientTimeout 测试客户端超时设置
func TestClientTimeout(t *testing.T) {
	client := NewClient(nil)
	assert.Equal(t, 30*time.Second, client.httpClient.Timeout)
}

// TestContextCancellation 测试上下文取消
func TestContextCancellation(t *testing.T) {
	client := NewClient(nil)

	// 创建一个立即取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// 这个请求应该因为上下文取消而失败
	_, err := client.GetKlines(ctx, "BTCUSD", "1h", 10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

// TestInvalidSymbol 测试无效符号处理
func TestInvalidSymbol(t *testing.T) {
	client := NewClient(nil)
	ctx := context.Background()

	// 测试无效符号
	_, err := client.GetKlines(ctx, "INVALID", "1h", 10)
	assert.Error(t, err)
}

// TestInvalidInterval 测试无效时间间隔
func TestInvalidInterval(t *testing.T) {
	client := NewClient(nil)
	ctx := context.Background()

	// 测试无效时间间隔
	_, err := client.GetKlines(ctx, "BTCUSD", "999m", 10)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不支持的时间间隔")
}

// TestRateLimitConfig 测试限流配置
func TestRateLimitConfig(t *testing.T) {
	cfg := &Config{
		RateLimit: struct {
			RequestsPerMinute int           `yaml:"requests_per_minute"`
			RetryDelay        time.Duration `yaml:"retry_delay"`
			MaxRetries        int           `yaml:"max_retries"`
		}{
			RequestsPerMinute: 60,
			MaxRetries:        5,
			RetryDelay:        time.Second * 3,
		},
	}

	client := NewClient(cfg)
	assert.NotNil(t, client.config)
	assert.Equal(t, 60, client.config.RateLimit.RequestsPerMinute)
	assert.Equal(t, 5, client.config.RateLimit.MaxRetries)
	assert.Equal(t, time.Second*3, client.config.RateLimit.RetryDelay)
}
