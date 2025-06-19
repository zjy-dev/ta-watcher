// Package watcher provides the main monitoring service that combines data sources,
// strategies, and notifications to form an automated monitoring loop
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

// Watcher 主监控服务
type Watcher struct {
	// 核心组件
	config              *config.Config
	dataSource          binance.DataSource
	notifierManager     *notifiers.Manager
	strategyIntegration *strategy.WatcherIntegration

	// 监控状态
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.RWMutex

	// 工作管理
	workerPool chan struct{}
	resultChan chan *MonitorResult
	errorChan  chan error

	// 通知冷却管理
	cooldownTracker map[string]time.Time
	cooldownMu      sync.RWMutex

	// 统计信息
	stats *Statistics
}

// MonitorResult 监控结果
type MonitorResult struct {
	Symbol       string                   `json:"symbol"`
	Timeframe    strategy.Timeframe       `json:"timeframe"`
	StrategyName string                   `json:"strategy_name"`
	Decision     *strategy.DecisionResult `json:"decision"`
	Timestamp    time.Time                `json:"timestamp"`
	Error        error                    `json:"error,omitempty"`
}

// MonitorTask 监控任务
type MonitorTask struct {
	Symbol       string             `json:"symbol"`
	Timeframe    strategy.Timeframe `json:"timeframe"`
	StrategyName string             `json:"strategy_name"`
	Config       config.StrategyConfig
}

// Statistics 监控统计信息
type Statistics struct {
	mu                sync.RWMutex
	StartTime         time.Time             `json:"start_time"`
	TotalTasks        int64                 `json:"total_tasks"`
	CompletedTasks    int64                 `json:"completed_tasks"`
	FailedTasks       int64                 `json:"failed_tasks"`
	NotificationsSent int64                 `json:"notifications_sent"`
	LastUpdate        time.Time             `json:"last_update"`
	Errors            []string              `json:"recent_errors"`
	AssetStats        map[string]*AssetStat `json:"asset_stats"`
}

// AssetStat 单个资产的统计信息
type AssetStat struct {
	Symbol            string    `json:"symbol"`
	LastCheck         time.Time `json:"last_check"`
	CheckCount        int64     `json:"check_count"`
	SignalCount       int64     `json:"signal_count"`
	LastSignal        string    `json:"last_signal"`
	LastSignalTime    time.Time `json:"last_signal_time"`
	NotificationCount int64     `json:"notification_count"`
}

// WatcherOption 配置选项
type WatcherOption func(*Watcher) error

// HealthStatus 健康状态
type HealthStatus struct {
	Running         bool            `json:"running"`
	Uptime          time.Duration   `json:"uptime"`
	ActiveWorkers   int             `json:"active_workers"`
	PendingTasks    int             `json:"pending_tasks"`
	LastError       string          `json:"last_error,omitempty"`
	LastErrorTime   time.Time       `json:"last_error_time,omitempty"`
	ComponentStatus map[string]bool `json:"component_status"`
	Statistics      *Statistics     `json:"statistics"`
}

// Event 事件类型
type Event struct {
	Type      EventType   `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
	Error     error       `json:"error,omitempty"`
}

// EventType 事件类型枚举
type EventType string

const (
	EventTypeStarted          EventType = "started"
	EventTypeStopped          EventType = "stopped"
	EventTypeTaskCompleted    EventType = "task_completed"
	EventTypeTaskFailed       EventType = "task_failed"
	EventTypeSignalDetected   EventType = "signal_detected"
	EventTypeNotificationSent EventType = "notification_sent"
	EventTypeError            EventType = "error"
)

// EventHandler 事件处理器接口
type EventHandler interface {
	HandleEvent(event *Event)
}

// EventHandlerFunc 事件处理器函数类型
type EventHandlerFunc func(event *Event)

// HandleEvent 实现 EventHandler 接口
func (f EventHandlerFunc) HandleEvent(event *Event) {
	f(event)
}
