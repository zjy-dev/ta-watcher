package watcher

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ta-watcher/internal/config"
)

// TestWatcherIntegration 集成测试
func TestWatcherIntegration(t *testing.T) {
	// 跳过集成测试，除非明确启用
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=1 to run.")
	}

	// 创建测试配置
	cfg := &config.Config{
		Watcher: config.WatcherConfig{
			Interval:      200 * time.Millisecond,
			MaxWorkers:    3,
			BufferSize:    10,
			LogLevel:      "info",
			EnableMetrics: true,
		},
		Binance: config.BinanceConfig{
			RateLimit: config.RateLimitConfig{
				RequestsPerMinute: 100,
				RetryDelay:        1 * time.Second,
				MaxRetries:        3,
			},
		},
		Notifiers: config.NotifiersConfig{
			Email: config.EmailConfig{
				Enabled: false, // 测试时禁用通知
			},
		},
		Assets: []string{"BTCUSDT"},
	}

	// 创建 Watcher
	w, err := New(cfg)
	require.NoError(t, err)
	require.NotNil(t, w)

	// 启动 Watcher
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = w.Start(ctx)
	require.NoError(t, err)

	// 等待几个监控周期
	time.Sleep(1 * time.Second)

	// 检查健康状态
	health := w.GetHealth()
	assert.True(t, health.Running)
	assert.Greater(t, health.Uptime, time.Duration(0))
	assert.True(t, health.ComponentStatus["data_source"])
	assert.True(t, health.ComponentStatus["strategy"])

	// 检查统计信息
	stats := w.GetStatistics()
	assert.Greater(t, stats.TotalTasks, int64(0))

	// 停止 Watcher
	err = w.Stop()
	assert.NoError(t, err)
	assert.False(t, w.IsRunning())
}

// TestWatcherWithCustomStrategy 测试自定义策略集成
func TestWatcherWithCustomStrategy(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=1 to run.")
	}

	// 创建临时策略目录
	tempDir, err := os.MkdirTemp("", "integration_strategy_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// 生成测试策略
	strategyPath := tempDir + "/test_integration_strategy.go"
	err = GenerateStrategyTemplate(strategyPath, "test_integration")
	require.NoError(t, err)

	// 创建配置
	cfg := &config.Config{
		Watcher: config.WatcherConfig{
			Interval:   500 * time.Millisecond,
			MaxWorkers: 2,
			BufferSize: 5,
		},
		Binance: config.BinanceConfig{
			RateLimit: config.RateLimitConfig{
				RequestsPerMinute: 50,
				RetryDelay:        1 * time.Second,
				MaxRetries:        2,
			},
		},
		Notifiers: config.NotifiersConfig{
			Email: config.EmailConfig{Enabled: false},
		},
		Assets: []string{"BTCUSDT"},
	}

	// 创建带自定义策略目录的 Watcher
	w, err := New(cfg, WithStrategiesDirectory(tempDir))
	require.NoError(t, err)

	// 短时间运行测试
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = w.Start(ctx)
	require.NoError(t, err)

	time.Sleep(800 * time.Millisecond)

	// 验证运行状态
	assert.True(t, w.IsRunning())

	stats := w.GetStatistics()
	assert.Greater(t, stats.TotalTasks, int64(0))

	err = w.Stop()
	assert.NoError(t, err)
}

// TestWatcherStressTest 压力测试
func TestWatcherStressTest(t *testing.T) {
	if os.Getenv("STRESS_TEST") == "" {
		t.Skip("Skipping stress test. Set STRESS_TEST=1 to run.")
	}

	cfg := &config.Config{
		Watcher: config.WatcherConfig{
			Interval:   50 * time.Millisecond, // 高频率
			MaxWorkers: 20,                    // 更多工作协程
			BufferSize: 100,
		},
		Binance: config.BinanceConfig{
			RateLimit: config.RateLimitConfig{
				RequestsPerMinute: 1000,
				RetryDelay:        100 * time.Millisecond,
				MaxRetries:        5,
			},
		},
		Notifiers: config.NotifiersConfig{
			Email: config.EmailConfig{Enabled: false},
		},
		Assets: []string{
			"BTCUSDT", "ETHUSDT", "BNBUSDT", "ADAUSDT", "SOLUSDT",
			"DOTUSDT", "LINKUSDT", "LTCUSDT", "BCUSDT", "XLMUSDT",
		},
	}

	w, err := New(cfg)
	require.NoError(t, err)

	// 运行压力测试
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = w.Start(ctx)
	require.NoError(t, err)

	// 并发访问统计信息
	done := make(chan bool)
	go func() {
		for i := 0; i < 1000; i++ {
			_ = w.GetHealth()
			_ = w.GetStatistics()
			time.Sleep(5 * time.Millisecond)
		}
		done <- true
	}()

	// 让 watcher 运行一段时间
	time.Sleep(5 * time.Second)

	// 检查运行状态
	health := w.GetHealth()
	assert.True(t, health.Running)
	assert.LessOrEqual(t, health.ActiveWorkers, 20) // 不应超过最大工作协程数

	stats := w.GetStatistics()
	assert.Greater(t, stats.TotalTasks, int64(100)) // 应该处理了很多任务
	t.Logf("Processed %d total tasks, %d completed, %d failed",
		stats.TotalTasks, stats.CompletedTasks, stats.FailedTasks)

	err = w.Stop()
	assert.NoError(t, err)

	// 等待并发访问完成
	select {
	case <-done:
		// 成功完成
	case <-time.After(2 * time.Second):
		t.Error("Concurrent access didn't complete in time")
	}
}

// TestWatcherRecovery 错误恢复测试
func TestWatcherRecovery(t *testing.T) {
	if os.Getenv("RECOVERY_TEST") == "" {
		t.Skip("Skipping recovery test. Set RECOVERY_TEST=1 to run.")
	}

	cfg := &config.Config{
		Watcher: config.WatcherConfig{
			Interval:   100 * time.Millisecond,
			MaxWorkers: 5,
			BufferSize: 20,
		},
		Binance: config.BinanceConfig{
			RateLimit: config.RateLimitConfig{
				RequestsPerMinute: 10, // 很低的限制，容易触发错误
				RetryDelay:        100 * time.Millisecond,
				MaxRetries:        2,
			},
		},
		Notifiers: config.NotifiersConfig{
			Email: config.EmailConfig{Enabled: false},
		},
		Assets: []string{"BTCUSDT", "INVALID_SYMBOL"}, // 包含无效交易对
	}

	w, err := New(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = w.Start(ctx)
	require.NoError(t, err)

	// 让它运行并遇到错误
	time.Sleep(2 * time.Second)

	// 即使有错误，watcher 应该仍在运行
	assert.True(t, w.IsRunning())

	stats := w.GetStatistics()
	// 应该有一些失败的任务
	assert.Greater(t, stats.FailedTasks, int64(0))
	// 应该记录了一些错误
	assert.Greater(t, len(stats.Errors), 0)

	t.Logf("Failed tasks: %d, Errors: %v", stats.FailedTasks, stats.Errors)

	err = w.Stop()
	assert.NoError(t, err)
}
