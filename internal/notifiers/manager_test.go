package notifiers

import (
	"sync"
	"testing"

	"ta-watcher/internal/config"

	"github.com/stretchr/testify/assert"
)

// MockNotifier 模拟通知器，用于测试
type MockNotifier struct {
	name          string
	enabled       bool
	sendError     error
	closeError    error
	sentMessages  []*Notification
	sendCallCount int
	mu            sync.RWMutex // 添加互斥锁保证线程安全
}

func NewMockNotifier(name string, enabled bool) *MockNotifier {
	return &MockNotifier{
		name:         name,
		enabled:      enabled,
		sentMessages: make([]*Notification, 0),
	}
}

func (m *MockNotifier) Send(notification *Notification) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sendCallCount++
	if m.sendError != nil {
		return m.sendError
	}
	m.sentMessages = append(m.sentMessages, notification)
	return nil
}

func (m *MockNotifier) Close() error {
	return m.closeError
}

func (m *MockNotifier) IsEnabled() bool {
	return m.enabled
}

func (m *MockNotifier) Name() string {
	return m.name
}

func (m *MockNotifier) SetSendError(err error) {
	m.sendError = err
}

func (m *MockNotifier) SetCloseError(err error) {
	m.closeError = err
}

func (m *MockNotifier) GetSentMessages() []*Notification {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 返回副本避免竞态条件
	messages := make([]*Notification, len(m.sentMessages))
	copy(messages, m.sentMessages)
	return messages
}

func (m *MockNotifier) GetSendCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.sendCallCount
}

func TestNewManager(t *testing.T) {
	manager := NewManager()

	assert.NotNil(t, manager)
	assert.Equal(t, 0, manager.TotalCount())
	assert.Equal(t, 0, manager.EnabledCount())
	assert.Empty(t, manager.ListNotifierNames())
}

func TestManagerAddNotifier(t *testing.T) {
	manager := NewManager()

	// 测试添加有效通知器
	notifier1 := NewMockNotifier("test1", true)
	err := manager.AddNotifier(notifier1)
	assert.NoError(t, err)
	assert.Equal(t, 1, manager.TotalCount())
	assert.Equal(t, 1, manager.EnabledCount())

	// 测试添加另一个通知器
	notifier2 := NewMockNotifier("test2", false)
	err = manager.AddNotifier(notifier2)
	assert.NoError(t, err)
	assert.Equal(t, 2, manager.TotalCount())
	assert.Equal(t, 1, manager.EnabledCount()) // 只有一个启用

	// 测试添加 nil 通知器
	err = manager.AddNotifier(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "notifier cannot be nil")

	// 测试添加重复名称的通知器
	notifier3 := NewMockNotifier("test1", true) // 重复名称
	err = manager.AddNotifier(notifier3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestManagerRemoveNotifier(t *testing.T) {
	manager := NewManager()
	notifier := NewMockNotifier("test", true)

	// 添加通知器
	err := manager.AddNotifier(notifier)
	assert.NoError(t, err)
	assert.Equal(t, 1, manager.TotalCount())

	// 移除通知器
	err = manager.RemoveNotifier("test")
	assert.NoError(t, err)
	assert.Equal(t, 0, manager.TotalCount())

	// 移除不存在的通知器
	err = manager.RemoveNotifier("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManagerSend(t *testing.T) {
	manager := NewManager()

	// 添加通知器
	notifier1 := NewMockNotifier("test1", true)
	notifier2 := NewMockNotifier("test2", true)
	notifier3 := NewMockNotifier("test3", false) // 禁用的

	manager.AddNotifier(notifier1)
	manager.AddNotifier(notifier2)
	manager.AddNotifier(notifier3)

	notification := mockNotification()

	// 发送通知
	err := manager.Send(notification)
	assert.NoError(t, err)

	// 验证只有启用的通知器收到了消息
	assert.Len(t, notifier1.GetSentMessages(), 1)
	assert.Len(t, notifier2.GetSentMessages(), 1)
	assert.Len(t, notifier3.GetSentMessages(), 0) // 禁用的不应该收到

	assert.Equal(t, notification, notifier1.GetSentMessages()[0])
	assert.Equal(t, notification, notifier2.GetSentMessages()[0])
}

func TestManagerSendWithNilNotification(t *testing.T) {
	manager := NewManager()
	notifier := NewMockNotifier("test", true)
	manager.AddNotifier(notifier)

	err := manager.Send(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "notification cannot be nil")
}

func TestManagerSendWithFilter(t *testing.T) {
	manager := NewManager()
	notifier := NewMockNotifier("test", true)
	manager.AddNotifier(notifier)

	// 设置过滤器 - 只允许 WARNING 级别以上
	filter := &NotificationFilter{
		MinLevel: LevelWarning,
	}
	manager.SetFilter(filter)

	// 发送 INFO 级别的通知（应该被过滤）
	infoNotification := mockNotification()
	infoNotification.Level = LevelInfo

	err := manager.Send(infoNotification)
	assert.NoError(t, err)
	assert.Len(t, notifier.GetSentMessages(), 0) // 被过滤，不应该收到

	// 发送 WARNING 级别的通知（应该通过）
	warningNotification := mockNotification()
	warningNotification.Level = LevelWarning

	err = manager.Send(warningNotification)
	assert.NoError(t, err)
	assert.Len(t, notifier.GetSentMessages(), 1) // 应该收到
}

func TestManagerSendTo(t *testing.T) {
	manager := NewManager()

	notifier1 := NewMockNotifier("test1", true)
	notifier2 := NewMockNotifier("test2", true)

	manager.AddNotifier(notifier1)
	manager.AddNotifier(notifier2)

	notification := mockNotification()

	// 发送到指定通知器
	err := manager.SendTo("test1", notification)
	assert.NoError(t, err)

	// 验证只有指定的通知器收到了消息
	assert.Len(t, notifier1.GetSentMessages(), 1)
	assert.Len(t, notifier2.GetSentMessages(), 0)

	// 发送到不存在的通知器
	err = manager.SendTo("nonexistent", notification)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// 发送到禁用的通知器
	notifier3 := NewMockNotifier("test3", false)
	manager.AddNotifier(notifier3)

	err = manager.SendTo("test3", notification)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is disabled")
}

func TestManagerSendToWithNilNotification(t *testing.T) {
	manager := NewManager()
	notifier := NewMockNotifier("test", true)
	manager.AddNotifier(notifier)

	err := manager.SendTo("test", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "notification cannot be nil")
}

func TestManagerClose(t *testing.T) {
	manager := NewManager()

	notifier1 := NewMockNotifier("test1", true)
	notifier2 := NewMockNotifier("test2", true)

	manager.AddNotifier(notifier1)
	manager.AddNotifier(notifier2)

	assert.Equal(t, 2, manager.TotalCount())

	// 关闭管理器
	err := manager.Close()
	assert.NoError(t, err)
	assert.Equal(t, 0, manager.TotalCount())
}

func TestManagerGetNotifier(t *testing.T) {
	manager := NewManager()
	notifier := NewMockNotifier("test", true)
	manager.AddNotifier(notifier)

	// 获取存在的通知器
	found, exists := manager.GetNotifier("test")
	assert.True(t, exists)
	assert.Equal(t, notifier, found)

	// 获取不存在的通知器
	found, exists = manager.GetNotifier("nonexistent")
	assert.False(t, exists)
	assert.Nil(t, found)
}

func TestManagerListNotifierNames(t *testing.T) {
	manager := NewManager()

	// 空管理器
	names := manager.ListNotifierNames()
	assert.Empty(t, names)

	// 添加通知器
	manager.AddNotifier(NewMockNotifier("test1", true))
	manager.AddNotifier(NewMockNotifier("test2", false))

	names = manager.ListNotifierNames()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "test1")
	assert.Contains(t, names, "test2")
}

func TestManagerGetNotifiers(t *testing.T) {
	manager := NewManager()

	notifier1 := NewMockNotifier("test1", true)
	notifier2 := NewMockNotifier("test2", false)

	manager.AddNotifier(notifier1)
	manager.AddNotifier(notifier2)

	notifiers := manager.GetNotifiers()
	assert.Len(t, notifiers, 2)

	// 验证返回的是副本（修改不会影响原始数据）
	notifiers = append(notifiers, NewMockNotifier("test3", true))
	assert.Equal(t, 2, manager.TotalCount()) // 原始管理器不应该受影响
}

func TestManagerSetAndGetFilter(t *testing.T) {
	manager := NewManager()

	// 默认过滤器
	filter := manager.GetFilter()
	assert.NotNil(t, filter)

	// 设置新过滤器
	newFilter := &NotificationFilter{
		MinLevel: LevelWarning,
		Types:    []NotificationType{TypePriceAlert},
		Assets:   []string{"BTCUSDT"},
	}
	manager.SetFilter(newFilter)

	retrievedFilter := manager.GetFilter()
	assert.Equal(t, newFilter, retrievedFilter)

	// 设置 nil 过滤器（应该设置为默认过滤器）
	manager.SetFilter(nil)
	retrievedFilter = manager.GetFilter()
	assert.NotNil(t, retrievedFilter)
	assert.Equal(t, NotificationLevel(0), retrievedFilter.MinLevel)
	assert.Empty(t, retrievedFilter.Types)
	assert.Empty(t, retrievedFilter.Assets)
}

func TestManagerConcurrency(t *testing.T) {
	manager := NewManager()
	notifier := NewMockNotifier("test", true)
	manager.AddNotifier(notifier)

	notification := mockNotification()

	// 并发发送通知
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			manager.Send(notification)
			done <- true
		}()
	}

	// 等待所有协程完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证通知器收到了所有消息
	assert.Equal(t, 10, notifier.GetSendCallCount())
	assert.Len(t, notifier.GetSentMessages(), 10)
}

func TestManagerSendPartialFailure(t *testing.T) {
	manager := NewManager()

	notifier1 := NewMockNotifier("test1", true)
	notifier2 := NewMockNotifier("test2", true)
	notifier3 := NewMockNotifier("test3", true)

	// 设置一个通知器失败
	notifier2.SetSendError(assert.AnError)

	manager.AddNotifier(notifier1)
	manager.AddNotifier(notifier2)
	manager.AddNotifier(notifier3)

	notification := mockNotification()

	// 发送通知（部分失败）
	err := manager.Send(notification)
	assert.NoError(t, err) // 部分成功时不应该返回错误

	// 验证成功的通知器收到了消息
	assert.Len(t, notifier1.GetSentMessages(), 1)
	assert.Len(t, notifier2.GetSentMessages(), 0) // 失败的不应该收到
	assert.Len(t, notifier3.GetSentMessages(), 1)
}

func TestManagerSendAllFailure(t *testing.T) {
	manager := NewManager()

	notifier1 := NewMockNotifier("test1", true)
	notifier2 := NewMockNotifier("test2", true)

	// 设置所有通知器失败
	notifier1.SetSendError(assert.AnError)
	notifier2.SetSendError(assert.AnError)

	manager.AddNotifier(notifier1)
	manager.AddNotifier(notifier2)

	notification := mockNotification()

	// 发送通知（全部失败）
	err := manager.Send(notification)
	assert.Error(t, err) // 全部失败时应该返回错误
	assert.Contains(t, err.Error(), "all notifiers failed")
}

// 集成测试：创建真实的邮件通知器并添加到管理器
func TestManagerWithRealEmailNotifier(t *testing.T) {
	manager := NewManager()

	// 创建禁用的邮件通知器（避免实际发送邮件）
	emailConfig := &config.EmailConfig{
		Enabled: false,
	}

	emailNotifier, err := NewEmailNotifier(emailConfig)
	assert.NoError(t, err)

	err = manager.AddNotifier(emailNotifier)
	assert.NoError(t, err)

	assert.Equal(t, 1, manager.TotalCount())
	assert.Equal(t, 0, manager.EnabledCount()) // 禁用的

	notification := mockNotification()
	err = manager.Send(notification)
	assert.NoError(t, err) // 应该成功，因为被跳过了
}
