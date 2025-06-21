package watcher

import (
	"context"
	"testing"
	"time"

	"ta-watcher/internal/config"
)

func TestNew(t *testing.T) {
	cfg := &config.Config{
		Binance: config.BinanceConfig{
			RateLimit: config.RateLimitConfig{
				RequestsPerMinute: 100,
			},
		},
		Watcher: config.WatcherConfig{
			Interval: time.Second,
		},
		Assets: []string{"BTCUSDT"},
	}

	w, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if w == nil {
		t.Fatal("New() returned nil watcher")
	}

	if w.config != cfg {
		t.Error("Config not set correctly")
	}

	if w.stats == nil {
		t.Error("Statistics not initialized")
	}
}

func TestWatcher_StartStop(t *testing.T) {
	cfg := &config.Config{
		Binance: config.BinanceConfig{
			RateLimit: config.RateLimitConfig{
				RequestsPerMinute: 100,
			},
		},
		Watcher: config.WatcherConfig{
			Interval: 100 * time.Millisecond,
		},
		Assets: []string{"BTCUSDT"},
	}

	w, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Test start
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if !w.IsRunning() {
		t.Error("Watcher should be running after Start()")
	}

	// Wait a bit
	time.Sleep(200 * time.Millisecond)

	// Test stop
	err = w.Stop()
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	if w.IsRunning() {
		t.Error("Watcher should not be running after Stop()")
	}
}

func TestWatcher_GetHealth(t *testing.T) {
	cfg := &config.Config{
		Binance: config.BinanceConfig{
			RateLimit: config.RateLimitConfig{
				RequestsPerMinute: 100,
			},
		},
		Watcher: config.WatcherConfig{
			Interval: time.Second,
		},
		Assets: []string{"BTCUSDT"},
	}

	w, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	health := w.GetHealth()
	if health == nil {
		t.Fatal("GetHealth() returned nil")
	}

	if health.Running {
		t.Error("Health should show not running initially")
	}
}

func TestWatcher_GetStatistics(t *testing.T) {
	cfg := &config.Config{
		Binance: config.BinanceConfig{
			RateLimit: config.RateLimitConfig{
				RequestsPerMinute: 100,
			},
		},
		Watcher: config.WatcherConfig{
			Interval: time.Second,
		},
		Assets: []string{"BTCUSDT"},
	}

	w, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	stats := w.GetStatistics()
	if stats == nil {
		t.Fatal("GetStatistics() returned nil")
	}

	if stats.TotalTasks != 0 {
		t.Error("Initial total tasks should be 0")
	}
}
