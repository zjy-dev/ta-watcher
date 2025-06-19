package watcher

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestStatisticsBasicOperations 测试统计基本操作
func TestStatisticsBasicOperations(t *testing.T) {
	stats := newStatistics()

	// 测试初始状态
	assert.Equal(t, int64(0), stats.TotalTasks)
	assert.Equal(t, int64(0), stats.CompletedTasks)
	assert.Equal(t, int64(0), stats.FailedTasks)
	assert.Equal(t, int64(0), stats.NotificationsSent)
	assert.Empty(t, stats.AssetStats)
	assert.Empty(t, stats.Errors)

	// 测试增加任务
	stats.IncrementTotalTasks(10)
	assert.Equal(t, int64(10), stats.TotalTasks)

	stats.IncrementTotalTasks(5)
	assert.Equal(t, int64(15), stats.TotalTasks)

	// 测试完成任务
	stats.IncrementCompletedTasks()
	assert.Equal(t, int64(1), stats.CompletedTasks)

	// 测试失败任务
	stats.IncrementFailedTasks()
	assert.Equal(t, int64(1), stats.FailedTasks)

	// 测试通知发送
	stats.IncrementNotificationsSent()
	assert.Equal(t, int64(1), stats.NotificationsSent)
}

// TestStatisticsAssetOperations 测试资产统计操作
func TestStatisticsAssetOperations(t *testing.T) {
	stats := newStatistics()

	// 测试新资产
	stats.UpdateAssetStat("BTCUSDT")
	assert.Contains(t, stats.AssetStats, "BTCUSDT")

	assetStat := stats.AssetStats["BTCUSDT"]
	assert.Equal(t, "BTCUSDT", assetStat.Symbol)
	assert.Equal(t, int64(1), assetStat.CheckCount)
	assert.WithinDuration(t, time.Now(), assetStat.LastCheck, time.Second)

	// 测试重复更新
	stats.UpdateAssetStat("BTCUSDT")
	assert.Equal(t, int64(2), assetStat.CheckCount)

	// 测试信号统计
	stats.UpdateSignalStat("BTCUSDT", "BUY")
	assert.Equal(t, int64(1), assetStat.SignalCount)
	assert.Equal(t, "BUY", assetStat.LastSignal)
	assert.WithinDuration(t, time.Now(), assetStat.LastSignalTime, time.Second)

	stats.UpdateSignalStat("BTCUSDT", "SELL")
	assert.Equal(t, int64(2), assetStat.SignalCount)
	assert.Equal(t, "SELL", assetStat.LastSignal)

	// 测试多个资产
	stats.UpdateAssetStat("ETHUSDT")
	assert.Contains(t, stats.AssetStats, "ETHUSDT")
	assert.Len(t, stats.AssetStats, 2)
}

// TestStatisticsErrorTracking 测试错误追踪
func TestStatisticsErrorTracking(t *testing.T) {
	stats := newStatistics()

	// 添加错误
	stats.AddError("Test error 1")
	assert.Len(t, stats.Errors, 1)
	assert.Equal(t, "Test error 1", stats.Errors[0])

	// 添加更多错误
	for i := 2; i <= 15; i++ {
		stats.AddError(fmt.Sprintf("Test error %d", i))
	}

	// 应该只保留最新的10个错误
	assert.Len(t, stats.Errors, 10)
	assert.Equal(t, "Test error 6", stats.Errors[0])  // 最早的应该是第6个
	assert.Equal(t, "Test error 15", stats.Errors[9]) // 最新的应该是第15个
}

// TestStatisticsClone 测试统计克隆
func TestStatisticsClone(t *testing.T) {
	stats := newStatistics()

	// 准备测试数据
	stats.IncrementTotalTasks(5)
	stats.IncrementCompletedTasks()
	stats.IncrementFailedTasks()
	stats.IncrementNotificationsSent()
	stats.UpdateAssetStat("BTCUSDT")
	stats.UpdateSignalStat("BTCUSDT", "BUY")
	stats.AddError("Test error")

	// 克隆
	cloned := stats.clone()

	// 验证克隆的数据
	assert.Equal(t, stats.TotalTasks, cloned.TotalTasks)
	assert.Equal(t, stats.CompletedTasks, cloned.CompletedTasks)
	assert.Equal(t, stats.FailedTasks, cloned.FailedTasks)
	assert.Equal(t, stats.NotificationsSent, cloned.NotificationsSent)
	assert.Len(t, cloned.AssetStats, len(stats.AssetStats))
	assert.Len(t, cloned.Errors, len(stats.Errors))

	// 验证是深拷贝，修改原始数据不影响克隆
	stats.IncrementTotalTasks(10)
	stats.AddError("Another error")

	assert.NotEqual(t, stats.TotalTasks, cloned.TotalTasks)
	assert.NotEqual(t, len(stats.Errors), len(cloned.Errors))

	// 验证 AssetStats 是深拷贝
	originalAsset := stats.AssetStats["BTCUSDT"]
	clonedAsset := cloned.AssetStats["BTCUSDT"]

	assert.Equal(t, originalAsset.Symbol, clonedAsset.Symbol)
	assert.Equal(t, originalAsset.CheckCount, clonedAsset.CheckCount)

	// 修改原始数据不应影响克隆
	stats.UpdateAssetStat("BTCUSDT")
	assert.NotEqual(t, originalAsset.CheckCount, clonedAsset.CheckCount)
}

// TestStatisticsConcurrentAccess 测试并发访问
func TestStatisticsConcurrentAccess(t *testing.T) {
	stats := newStatistics()
	var wg sync.WaitGroup

	// 并发执行各种操作
	numGoroutines := 10
	operationsPerGoroutine := 100

	// 并发增加任务
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				stats.IncrementTotalTasks(1)
				stats.IncrementCompletedTasks()
				stats.IncrementFailedTasks()
				stats.IncrementNotificationsSent()
			}
		}()
	}

	// 并发更新资产统计
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			symbol := fmt.Sprintf("ASSET%d", id%3) // 使用3个不同的资产
			for j := 0; j < operationsPerGoroutine; j++ {
				stats.UpdateAssetStat(symbol)
				if j%10 == 0 {
					stats.UpdateSignalStat(symbol, "BUY")
				}
			}
		}(i)
	}

	// 并发添加错误
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				stats.AddError(fmt.Sprintf("Error from goroutine %d iteration %d", id, j))
			}
		}(i)
	}

	// 并发读取操作
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				_ = stats.clone()
			}
		}()
	}

	wg.Wait()

	// 验证最终结果
	expectedTotal := int64(numGoroutines * operationsPerGoroutine)
	assert.Equal(t, expectedTotal, stats.TotalTasks)
	assert.Equal(t, expectedTotal, stats.CompletedTasks)
	assert.Equal(t, expectedTotal, stats.FailedTasks)
	assert.Equal(t, expectedTotal, stats.NotificationsSent)

	// 验证资产统计
	assert.Len(t, stats.AssetStats, 3) // 应该有3个不同的资产
	for symbol, assetStat := range stats.AssetStats {
		assert.Contains(t, []string{"ASSET0", "ASSET1", "ASSET2"}, symbol)
		assert.Greater(t, assetStat.CheckCount, int64(0))
	}

	// 验证错误列表长度（应该不超过10）
	assert.LessOrEqual(t, len(stats.Errors), 10)
}

// TestStatisticsTimestamps 测试时间戳更新
func TestStatisticsTimestamps(t *testing.T) {
	stats := newStatistics()
	startTime := stats.StartTime

	// 等待一小段时间确保时间戳会变化
	time.Sleep(10 * time.Millisecond)

	originalLastUpdate := stats.LastUpdate

	// 执行操作应该更新 LastUpdate
	stats.IncrementTotalTasks(1)
	assert.True(t, stats.LastUpdate.After(originalLastUpdate))

	stats.IncrementCompletedTasks()
	lastUpdate1 := stats.LastUpdate

	stats.UpdateAssetStat("BTCUSDT")
	assert.True(t, stats.LastUpdate.After(lastUpdate1) || stats.LastUpdate.Equal(lastUpdate1))

	// StartTime 不应该改变
	assert.Equal(t, startTime, stats.StartTime)
}
