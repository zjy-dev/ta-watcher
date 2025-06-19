package notifiers

import (
	"time"
)

// NotificationLevel 通知级别
type NotificationLevel int

const (
	LevelInfo NotificationLevel = iota
	LevelWarning
	LevelError
	LevelCritical
)

func (l NotificationLevel) String() string {
	switch l {
	case LevelInfo:
		return "INFO"
	case LevelWarning:
		return "WARNING"
	case LevelError:
		return "ERROR"
	case LevelCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

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
	Level     NotificationLevel      `json:"level"`     // 通知级别
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

// NotificationFilter 通知过滤器
type NotificationFilter struct {
	MinLevel NotificationLevel  // 最小级别
	Types    []NotificationType // 允许的类型
	Assets   []string           // 允许的资产
}

// ShouldNotify 判断是否应该发送通知
func (f *NotificationFilter) ShouldNotify(notification *Notification) bool {
	// 检查级别
	if notification.Level < f.MinLevel {
		return false
	}

	// 检查类型
	if len(f.Types) > 0 {
		found := false
		for _, t := range f.Types {
			if t == notification.Type {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 检查资产
	if len(f.Assets) > 0 && notification.Asset != "" {
		found := false
		for _, asset := range f.Assets {
			if asset == notification.Asset {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}
