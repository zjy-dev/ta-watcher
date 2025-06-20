package watcher

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"ta-watcher/internal/config"
	"ta-watcher/internal/strategy"
)

// MockDataSource 模拟数据源
type MockDataSource struct {
	mock.Mock
}

func (m *MockDataSource) GetKlineData(symbol string, interval string, limit int) ([]interface{}, error) {
	args := m.Called(symbol, interval, limit)
	return args.Get(0).([]interface{}), args.Error(1)
}

// MockNotifierManager 模拟通知管理器
type MockNotifierManager struct {
	mock.Mock
}

func (m *MockNotifierManager) Send(notification interface{}) error {
	args := m.Called(notification)
	return args.Error(0)
}

func (m *MockNotifierManager) AddNotifier(notifier interface{}) error {
	args := m.Called(notifier)
	return args.Error(0)
}

func (m *MockNotifierManager) Close() error {
	args := m.Called()
	return args.Error(0)
}

// TestWatcherCreation 测试 Watcher 创建
func TestWatcherCreation(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.Config
		wantErr bool
	}{
		{
			name: "Valid config",
			config: &config.Config{
				Watcher: config.WatcherConfig{
					Interval:   5 * time.Minute,
					MaxWorkers: 10,
					BufferSize: 100,
				},
				Binance:   config.BinanceConfig{},
				Notifiers: config.NotifiersConfig{},
				Assets:    []string{"BTCUSDT"},
			},
			wantErr: false,
		},
		{
			name:    "Nil config",
			config:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			watcher, err := New(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, watcher)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, watcher)
				assert.False(t, watcher.IsRunning())
			}
		})
	}
}

// TestWatcherLifecycle 测试 Watcher 生命周期
func TestWatcherLifecycle(t *testing.T) {
	cfg := &config.Config{
		Watcher: config.WatcherConfig{
			Interval:   100 * time.Millisecond, // 短间隔便于测试
			MaxWorkers: 2,
			BufferSize: 10,
		},
		Binance:   config.BinanceConfig{},
		Notifiers: config.NotifiersConfig{},
		Assets:    []string{"BTCUSDT"},
	}

	w, err := New(cfg)
	require.NoError(t, err)
	require.NotNil(t, w)

	// 测试初始状态
	assert.False(t, w.IsRunning())

	// 测试启动
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = w.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, w.IsRunning())

	// 让 watcher 运行一小段时间
	time.Sleep(300 * time.Millisecond)

	// 测试停止
	err = w.Stop()
	assert.NoError(t, err)
	assert.False(t, w.IsRunning())

	// 测试重复停止
	err = w.Stop()
	assert.Error(t, err)
}

// TestWatcherHealthStatus 测试健康状态
func TestWatcherHealthStatus(t *testing.T) {
	cfg := &config.Config{
		Watcher: config.WatcherConfig{
			Interval:   1 * time.Second,
			MaxWorkers: 5,
			BufferSize: 20,
		},
		Binance:   config.BinanceConfig{},
		Notifiers: config.NotifiersConfig{},
		Assets:    []string{"BTCUSDT", "ETHUSDT"},
	}

	w, err := New(cfg)
	require.NoError(t, err)

	// 测试停止状态的健康检查
	health := w.GetHealth()
	assert.False(t, health.Running)
	assert.Equal(t, time.Duration(0), health.Uptime)
	assert.Equal(t, 0, health.ActiveWorkers)
	assert.NotNil(t, health.ComponentStatus)
	assert.NotNil(t, health.Statistics)

	// 启动并测试运行状态
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err = w.Start(ctx)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	health = w.GetHealth()
	assert.True(t, health.Running)
	assert.Greater(t, health.Uptime, time.Duration(0))
	assert.True(t, health.ComponentStatus["data_source"])
	assert.True(t, health.ComponentStatus["notifier"])
	assert.True(t, health.ComponentStatus["strategy"])

	err = w.Stop()
	assert.NoError(t, err)
}

// TestWatcherStatistics 测试统计功能
func TestWatcherStatistics(t *testing.T) {
	cfg := &config.Config{
		Watcher: config.WatcherConfig{
			Interval:   50 * time.Millisecond,
			MaxWorkers: 3,
			BufferSize: 10,
		},
		Binance:   config.BinanceConfig{},
		Notifiers: config.NotifiersConfig{},
		Assets:    []string{"BTCUSDT"},
	}

	w, err := New(cfg)
	require.NoError(t, err)

	// 获取初始统计
	stats := w.GetStatistics()
	assert.Equal(t, int64(0), stats.TotalTasks)
	assert.Equal(t, int64(0), stats.CompletedTasks)
	assert.Equal(t, int64(0), stats.FailedTasks)
	assert.Equal(t, int64(0), stats.NotificationsSent)
	assert.Empty(t, stats.AssetStats)

	// 启动 watcher 让它执行一些任务
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err = w.Start(ctx)
	require.NoError(t, err)

	// 等待执行一些监控周期
	time.Sleep(200 * time.Millisecond)

	// 检查统计是否更新
	stats = w.GetStatistics()
	// 由于可能会有网络错误，我们只检查任务是否被创建
	assert.Greater(t, stats.TotalTasks, int64(0))

	err = w.Stop()
	assert.NoError(t, err)
}

// TestParseTimeframe 测试时间框架解析
func TestParseTimeframe(t *testing.T) {
	tests := []struct {
		input    string
		expected strategy.Timeframe
		wantErr  bool
	}{
		{"1m", strategy.Timeframe1m, false},
		{"5m", strategy.Timeframe5m, false},
		{"1h", strategy.Timeframe1h, false},
		{"1d", strategy.Timeframe1d, false},
		{"1w", strategy.Timeframe1w, false},
		{"1M", strategy.Timeframe1M, false},
		{"invalid", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ParseTimeframe(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestTimeframeToString 测试时间框架转字符串
func TestTimeframeToString(t *testing.T) {
	tests := []struct {
		input    strategy.Timeframe
		expected string
	}{
		{strategy.Timeframe1m, "1m"},
		{strategy.Timeframe5m, "5m"},
		{strategy.Timeframe1h, "1h"},
		{strategy.Timeframe1d, "1d"},
		{strategy.Timeframe1w, "1w"},
		{strategy.Timeframe1M, "1M"},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			result := TimeframeToString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestWatcherOptions 测试 Watcher 选项
func TestWatcherOptions(t *testing.T) {
	cfg := &config.Config{
		Watcher: config.WatcherConfig{
			Interval:   1 * time.Second,
			MaxWorkers: 5,
			BufferSize: 10,
		},
		Binance:   config.BinanceConfig{},
		Notifiers: config.NotifiersConfig{},
		Assets:    []string{"BTCUSDT"},
	}

	// 测试带策略目录选项
	w, err := New(cfg, WithStrategiesDirectory("./test_strategies"))
	assert.NoError(t, err)
	assert.NotNil(t, w)

	// 测试空策略目录
	w, err = New(cfg, WithStrategiesDirectory(""))
	assert.NoError(t, err)
	assert.NotNil(t, w)
}

// TestWatcherConcurrentOperations 测试并发操作
func TestWatcherConcurrentOperations(t *testing.T) {
	cfg := &config.Config{
		Watcher: config.WatcherConfig{
			Interval:   100 * time.Millisecond,
			MaxWorkers: 3,
			BufferSize: 10,
		},
		Binance:   config.BinanceConfig{},
		Notifiers: config.NotifiersConfig{},
		Assets:    []string{"BTCUSDT"},
	}

	w, err := New(cfg)
	require.NoError(t, err)

	// 并发调用 IsRunning
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = w.IsRunning()
				_ = w.GetHealth()
				_ = w.GetStatistics()
			}
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 测试在并发访问时启动和停止
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err = w.Start(ctx)
	assert.NoError(t, err)

	// 继续并发访问
	go func() {
		for i := 0; i < 50; i++ {
			_ = w.IsRunning()
			time.Sleep(1 * time.Millisecond)
		}
	}()

	time.Sleep(50 * time.Millisecond)

	err = w.Stop()
	assert.NoError(t, err)
}
