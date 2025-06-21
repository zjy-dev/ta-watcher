package watcher

import (
	"context"
	"sync"
	"time"

	"ta-watcher/internal/binance"
	"ta-watcher/internal/config"
	"ta-watcher/internal/notifiers"
	"ta-watcher/internal/strategy"
)

// Watcher 监控服务
type Watcher struct {
	config     *config.Config
	dataSource binance.DataSource
	notifier   *notifiers.Manager
	strategy   *strategy.Manager

	running bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.RWMutex

	stats *Statistics
}

// MonitorResult 监控结果
type MonitorResult struct {
	Symbol    string                   `json:"symbol"`
	Timeframe strategy.Timeframe       `json:"timeframe"`
	Strategy  string                   `json:"strategy"`
	Result    *strategy.StrategyResult `json:"result"`
	Timestamp time.Time                `json:"timestamp"`
}

// HealthStatus 健康状态
type HealthStatus struct {
	Running    bool          `json:"running"`
	Uptime     time.Duration `json:"uptime"`
	TasksTotal int64         `json:"tasks_total"`
	TasksOK    int64         `json:"tasks_ok"`
	TasksError int64         `json:"tasks_error"`
	StartTime  time.Time     `json:"start_time"`
}

// Statistics 统计信息
type Statistics struct {
	mu sync.RWMutex

	StartTime         time.Time `json:"start_time"`
	TotalTasks        int64     `json:"total_tasks"`
	CompletedTasks    int64     `json:"completed_tasks"`
	FailedTasks       int64     `json:"failed_tasks"`
	NotificationsSent int64     `json:"notifications_sent"`
	LastUpdate        time.Time `json:"last_update"`
}

// WatcherOption 配置选项
type WatcherOption func(*Watcher) error

// newStatistics 创建统计实例
func newStatistics() *Statistics {
	return &Statistics{
		StartTime:  time.Now(),
		LastUpdate: time.Now(),
	}
}
