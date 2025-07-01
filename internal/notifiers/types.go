package notifiers

import (
	"time"
)

// NotificationType 通知类型
type NotificationType int

const (
	TypePriceAlert NotificationType = iota
	TypeStrategySignal
	TypeSystemAlert
	TypeHeartbeat
)

func (t NotificationType) String() string {
	switch t {
	case TypePriceAlert:
		return "PRICE_ALERT"
	case TypeStrategySignal:
		return "STRATEGY_SIGNAL"
	case TypeSystemAlert:
		return "SYSTEM_ALERT"
	case TypeHeartbeat:
		return "HEARTBEAT"
	default:
		return "UNKNOWN"
	}
}

// Notification 通知消息结构
type Notification struct {
	ID        string                 `json:"id"`        // 通知唯一标识
	Type      NotificationType       `json:"type"`      // 通知类型
	Asset     string                 `json:"asset"`     // 相关资产
	Strategy  string                 `json:"strategy"`  // 相关策略
	Title     string                 `json:"title"`     // 通知标题
	Message   string                 `json:"message"`   // 通知内容
	Data      map[string]interface{} `json:"data"`      // 附加数据
	Timestamp time.Time              `json:"timestamp"` // 时间戳
}

// Notifier 通知器接口
type Notifier interface {
	// Send 发送通知
	Send(notification *Notification) error

	// Close 关闭通知器
	Close() error

	// IsEnabled 是否启用
	IsEnabled() bool

	// Name 通知器名称
	Name() string
}

// NotificationManager 通知管理器接口
type NotificationManager interface {
	// AddNotifier 添加通知器
	AddNotifier(notifier Notifier) error

	// RemoveNotifier 移除通知器
	RemoveNotifier(name string) error

	// Send 发送通知到所有启用的通知器
	Send(notification *Notification) error

	// SendTo 发送通知到指定通知器
	SendTo(notifierName string, notification *Notification) error

	// Close 关闭所有通知器
	Close() error

	// GetNotifiers 获取所有通知器
	GetNotifiers() []Notifier
}
