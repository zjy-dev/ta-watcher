package watcher

import (
	"fmt"
	"strings"

	"ta-watcher/internal/notifiers"
	"ta-watcher/internal/strategy"
)

// ParseTimeframe 解析时间框架字符串
func ParseTimeframe(timeframeStr string) (strategy.Timeframe, error) {
	// 直接匹配字符串，保持大小写敏感
	switch timeframeStr {
	case "1m":
		return strategy.Timeframe1m, nil
	case "3m":
		return strategy.Timeframe3m, nil
	case "5m":
		return strategy.Timeframe5m, nil
	case "15m":
		return strategy.Timeframe15m, nil
	case "30m":
		return strategy.Timeframe30m, nil
	case "1h":
		return strategy.Timeframe1h, nil
	case "2h":
		return strategy.Timeframe2h, nil
	case "4h":
		return strategy.Timeframe4h, nil
	case "6h":
		return strategy.Timeframe6h, nil
	case "8h":
		return strategy.Timeframe8h, nil
	case "12h":
		return strategy.Timeframe12h, nil
	case "1d":
		return strategy.Timeframe1d, nil
	case "3d":
		return strategy.Timeframe3d, nil
	case "1w":
		return strategy.Timeframe1w, nil
	case "1M":
		return strategy.Timeframe1M, nil
	default:
		return "", fmt.Errorf("unsupported timeframe: %s", timeframeStr)
	}
}

// TimeframeToString 将 Timeframe 转换为字符串
func TimeframeToString(t strategy.Timeframe) string {
	return string(t)
}

// ParseNotificationLevel 解析通知级别
func ParseNotificationLevel(level string) notifiers.NotificationLevel {
	switch strings.ToLower(level) {
	case "info":
		return notifiers.LevelInfo
	case "warning", "warn":
		return notifiers.LevelWarning
	case "error":
		return notifiers.LevelError
	case "critical":
		return notifiers.LevelCritical
	default:
		return notifiers.LevelInfo
	}
}
