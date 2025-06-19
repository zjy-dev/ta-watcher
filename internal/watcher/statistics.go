package watcher

import (
	"time"
)

// IncrementTotalTasks 增加总任务数
func (s *Statistics) IncrementTotalTasks(count int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalTasks += count
	s.LastUpdate = time.Now()
}

// IncrementCompletedTasks 增加完成任务数
func (s *Statistics) IncrementCompletedTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CompletedTasks++
	s.LastUpdate = time.Now()
}

// IncrementFailedTasks 增加失败任务数
func (s *Statistics) IncrementFailedTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.FailedTasks++
	s.LastUpdate = time.Now()
}

// IncrementNotificationsSent 增加发送通知数
func (s *Statistics) IncrementNotificationsSent() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.NotificationsSent++
	s.LastUpdate = time.Now()
}

// UpdateAssetStat 更新资产统计
func (s *Statistics) UpdateAssetStat(symbol string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stat, exists := s.AssetStats[symbol]
	if !exists {
		stat = &AssetStat{
			Symbol: symbol,
		}
		s.AssetStats[symbol] = stat
	}

	stat.LastCheck = time.Now()
	stat.CheckCount++
	s.LastUpdate = time.Now()
}

// UpdateSignalStat 更新信号统计
func (s *Statistics) UpdateSignalStat(symbol, signal string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stat, exists := s.AssetStats[symbol]
	if !exists {
		stat = &AssetStat{
			Symbol: symbol,
		}
		s.AssetStats[symbol] = stat
	}

	stat.SignalCount++
	stat.LastSignal = signal
	stat.LastSignalTime = time.Now()
	s.LastUpdate = time.Now()
}

// AddError 添加错误信息
func (s *Statistics) AddError(errorMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 保持最新的10个错误
	s.Errors = append(s.Errors, errorMsg)
	if len(s.Errors) > 10 {
		s.Errors = s.Errors[1:]
	}
	s.LastUpdate = time.Now()
}

// clone 克隆统计信息（线程安全）
func (s *Statistics) clone() *Statistics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cloned := &Statistics{
		StartTime:         s.StartTime,
		TotalTasks:        s.TotalTasks,
		CompletedTasks:    s.CompletedTasks,
		FailedTasks:       s.FailedTasks,
		NotificationsSent: s.NotificationsSent,
		LastUpdate:        s.LastUpdate,
		Errors:            make([]string, len(s.Errors)),
		AssetStats:        make(map[string]*AssetStat),
	}

	copy(cloned.Errors, s.Errors)

	for k, v := range s.AssetStats {
		cloned.AssetStats[k] = &AssetStat{
			Symbol:            v.Symbol,
			LastCheck:         v.LastCheck,
			CheckCount:        v.CheckCount,
			SignalCount:       v.SignalCount,
			LastSignal:        v.LastSignal,
			LastSignalTime:    v.LastSignalTime,
			NotificationCount: v.NotificationCount,
		}
	}

	return cloned
}
