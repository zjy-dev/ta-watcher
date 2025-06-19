package notifiers

import (
	"fmt"
	"sync"
)

// Manager 通知管理器实现
type Manager struct {
	notifiers map[string]Notifier
	filter    *NotificationFilter
	mu        sync.RWMutex
}

// NewManager 创建新的通知管理器
func NewManager() *Manager {
	return &Manager{
		notifiers: make(map[string]Notifier),
		filter:    &NotificationFilter{}, // 默认无过滤
	}
}

// AddNotifier 添加通知器
func (m *Manager) AddNotifier(notifier Notifier) error {
	if notifier == nil {
		return fmt.Errorf("notifier cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	name := notifier.Name()
	if _, exists := m.notifiers[name]; exists {
		return fmt.Errorf("notifier with name '%s' already exists", name)
	}

	m.notifiers[name] = notifier
	return nil
}

// RemoveNotifier 移除通知器
func (m *Manager) RemoveNotifier(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	notifier, exists := m.notifiers[name]
	if !exists {
		return fmt.Errorf("notifier with name '%s' not found", name)
	}

	// 关闭通知器
	if err := notifier.Close(); err != nil {
		return fmt.Errorf("failed to close notifier '%s': %w", name, err)
	}

	delete(m.notifiers, name)
	return nil
}

// Send 发送通知到所有启用的通知器
func (m *Manager) Send(notification *Notification) error {
	if notification == nil {
		return fmt.Errorf("notification cannot be nil")
	}

	// 检查过滤器
	if !m.filter.ShouldNotify(notification) {
		return nil // 被过滤，不发送
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var errors []error
	successCount := 0

	// 发送到所有启用的通知器
	for name, notifier := range m.notifiers {
		if !notifier.IsEnabled() {
			continue
		}

		if err := notifier.Send(notification); err != nil {
			errors = append(errors, fmt.Errorf("notifier '%s': %w", name, err))
		} else {
			successCount++
		}
	}

	// 如果所有通知器都失败了，返回错误
	if len(errors) > 0 && successCount == 0 {
		return fmt.Errorf("all notifiers failed: %v", errors)
	}

	// 如果部分失败，记录但不返回错误
	if len(errors) > 0 {
		// 这里可以添加日志记录
	}

	return nil
}

// SendTo 发送通知到指定通知器
func (m *Manager) SendTo(notifierName string, notification *Notification) error {
	if notification == nil {
		return fmt.Errorf("notification cannot be nil")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	notifier, exists := m.notifiers[notifierName]
	if !exists {
		return fmt.Errorf("notifier with name '%s' not found", notifierName)
	}

	if !notifier.IsEnabled() {
		return fmt.Errorf("notifier '%s' is disabled", notifierName)
	}

	return notifier.Send(notification)
}

// Close 关闭所有通知器
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []error

	for name, notifier := range m.notifiers {
		if err := notifier.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close notifier '%s': %w", name, err))
		}
	}

	// 清空通知器映射
	m.notifiers = make(map[string]Notifier)

	if len(errors) > 0 {
		return fmt.Errorf("failed to close some notifiers: %v", errors)
	}

	return nil
}

// GetNotifiers 获取所有通知器
func (m *Manager) GetNotifiers() []Notifier {
	m.mu.RLock()
	defer m.mu.RUnlock()

	notifiers := make([]Notifier, 0, len(m.notifiers))
	for _, notifier := range m.notifiers {
		notifiers = append(notifiers, notifier)
	}

	return notifiers
}

// SetFilter 设置通知过滤器
func (m *Manager) SetFilter(filter *NotificationFilter) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if filter == nil {
		m.filter = &NotificationFilter{} // 默认无过滤
	} else {
		m.filter = filter
	}
}

// GetFilter 获取当前过滤器
func (m *Manager) GetFilter() *NotificationFilter {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.filter
}

// GetNotifier 获取指定名称的通知器
func (m *Manager) GetNotifier(name string) (Notifier, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	notifier, exists := m.notifiers[name]
	return notifier, exists
}

// ListNotifierNames 列出所有通知器名称
func (m *Manager) ListNotifierNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.notifiers))
	for name := range m.notifiers {
		names = append(names, name)
	}

	return names
}

// EnabledCount 返回启用的通知器数量
func (m *Manager) EnabledCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, notifier := range m.notifiers {
		if notifier.IsEnabled() {
			count++
		}
	}

	return count
}

// TotalCount 返回总通知器数量
func (m *Manager) TotalCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.notifiers)
}
