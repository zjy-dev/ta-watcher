package watcher

import (
	"context"
	"testing"
	"time"

	"ta-watcher/internal/config"
)

func TestNew(t *testing.T) {
	cfg := &config.Config{
		DataSource: config.DataSourceConfig{
			Primary: "binance",
		},
		Assets: config.AssetsConfig{
			Symbols:    []string{"BTCUSDT"},
			Timeframes: []string{"1h"},
		},
	}

	w, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if w == nil {
		t.Fatal("New() returned nil watcher")
	}

	// 检查数据源
	if w.dataSource == nil {
		t.Error("DataSource not initialized")
	}

	// 检查策略
	if len(w.strategies) == 0 {
		t.Error("No strategies initialized")
	}
}

func TestWatcher_Basic(t *testing.T) {
	cfg := &config.Config{
		DataSource: config.DataSourceConfig{
			Primary: "binance",
		},
		Assets: config.AssetsConfig{
			Symbols:    []string{"BTCUSDT"},
			Timeframes: []string{"1h"},
		},
	}

	w, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// 测试基本方法
	w.Stop()

	if !w.IsRunning() {
		t.Log("IsRunning() returned false as expected")
	}

	status := w.GetStatus()
	if status == nil {
		t.Error("GetStatus() returned nil")
	}

	if _, ok := status["running"]; !ok {
		t.Error("Status should contain 'running' field")
	}

	if _, ok := status["data_source"]; !ok {
		t.Error("Status should contain 'data_source' field")
	}
}

func TestWatcher_ContextCancellation(t *testing.T) {
	cfg := &config.Config{
		DataSource: config.DataSourceConfig{
			Primary: "binance",
		},
		Assets: config.AssetsConfig{
			Symbols:    []string{"BTCUSDT"},
			Timeframes: []string{"1h"},
		},
	}

	w, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start should return when context is cancelled
	err = w.Start(ctx)
	if err != nil && err != context.DeadlineExceeded {
		t.Errorf("Expected context deadline exceeded, got: %v", err)
	}
}
