package watcher

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"ta-watcher/internal/notifiers"
	"ta-watcher/internal/strategy"
)

// TestParseNotificationLevel 测试通知级别解析
func TestParseNotificationLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected notifiers.NotificationLevel
	}{
		{"info", notifiers.LevelInfo},
		{"INFO", notifiers.LevelInfo},
		{"Info", notifiers.LevelInfo},
		{"warning", notifiers.LevelWarning},
		{"warn", notifiers.LevelWarning},
		{"WARNING", notifiers.LevelWarning},
		{"WARN", notifiers.LevelWarning},
		{"error", notifiers.LevelError},
		{"ERROR", notifiers.LevelError},
		{"critical", notifiers.LevelCritical},
		{"CRITICAL", notifiers.LevelCritical},
		{"invalid", notifiers.LevelInfo}, // 默认值
		{"", notifiers.LevelInfo},        // 默认值
		{"unknown", notifiers.LevelInfo}, // 默认值
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseNotificationLevel(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestParseTimeframeEdgeCases 测试时间框架解析的边界情况
func TestParseTimeframeEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected strategy.Timeframe
		wantErr  bool
	}{
		{
			name:     "Valid lowercase",
			input:    "1m",
			expected: strategy.Timeframe1m,
			wantErr:  false,
		},
		{
			name:     "Valid uppercase",
			input:    "1M",
			expected: strategy.Timeframe1M,
			wantErr:  false,
		},
		{
			name:     "Mixed case input",
			input:    "1H",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "Invalid format",
			input:    "1minute",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "Invalid number",
			input:    "2m",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "Special characters",
			input:    "1m!",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "Spaces",
			input:    " 1m ",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseTimeframe(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, strategy.Timeframe(""), result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestTimeframeToStringComprehensive 测试时间框架到字符串的完整转换
func TestTimeframeToStringComprehensive(t *testing.T) {
	allTimeframes := []struct {
		timeframe strategy.Timeframe
		expected  string
	}{
		{strategy.Timeframe1m, "1m"},
		{strategy.Timeframe3m, "3m"},
		{strategy.Timeframe5m, "5m"},
		{strategy.Timeframe15m, "15m"},
		{strategy.Timeframe30m, "30m"},
		{strategy.Timeframe1h, "1h"},
		{strategy.Timeframe2h, "2h"},
		{strategy.Timeframe4h, "4h"},
		{strategy.Timeframe6h, "6h"},
		{strategy.Timeframe8h, "8h"},
		{strategy.Timeframe12h, "12h"},
		{strategy.Timeframe1d, "1d"},
		{strategy.Timeframe3d, "3d"},
		{strategy.Timeframe1w, "1w"},
		{strategy.Timeframe1M, "1M"},
	}

	for _, tt := range allTimeframes {
		t.Run(string(tt.timeframe), func(t *testing.T) {
			result := TimeframeToString(tt.timeframe)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestRoundTripTimeframeConversion 测试时间框架双向转换
func TestRoundTripTimeframeConversion(t *testing.T) {
	validTimeframes := []string{
		"1m", "3m", "5m", "15m", "30m",
		"1h", "2h", "4h", "6h", "8h", "12h",
		"1d", "3d", "1w", "1M",
	}

	for _, timeframeStr := range validTimeframes {
		t.Run(timeframeStr, func(t *testing.T) {
			// 字符串 -> Timeframe
			timeframe, err := ParseTimeframe(timeframeStr)
			assert.NoError(t, err)

			// Timeframe -> 字符串
			result := TimeframeToString(timeframe)
			assert.Equal(t, timeframeStr, result)
		})
	}
}

// TestNotificationLevelString 测试通知级别字符串表示
func TestNotificationLevelString(t *testing.T) {
	tests := []struct {
		level    notifiers.NotificationLevel
		expected string
	}{
		{notifiers.LevelInfo, "INFO"},
		{notifiers.LevelWarning, "WARNING"},
		{notifiers.LevelError, "ERROR"},
		{notifiers.LevelCritical, "CRITICAL"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.level.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// BenchmarkParseTimeframe 基准测试：时间框架解析
func BenchmarkParseTimeframe(b *testing.B) {
	timeframes := []string{"1m", "5m", "1h", "1d", "1w", "1M", "invalid"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		timeframe := timeframes[i%len(timeframes)]
		_, _ = ParseTimeframe(timeframe)
	}
}

// BenchmarkTimeframeToString 基准测试：时间框架转字符串
func BenchmarkTimeframeToString(b *testing.B) {
	timeframes := []strategy.Timeframe{
		strategy.Timeframe1m,
		strategy.Timeframe5m,
		strategy.Timeframe1h,
		strategy.Timeframe1d,
		strategy.Timeframe1w,
		strategy.Timeframe1M,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		timeframe := timeframes[i%len(timeframes)]
		_ = TimeframeToString(timeframe)
	}
}

// BenchmarkParseNotificationLevel 基准测试：通知级别解析
func BenchmarkParseNotificationLevel(b *testing.B) {
	levels := []string{"info", "warning", "error", "critical", "invalid"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		level := levels[i%len(levels)]
		_ = ParseNotificationLevel(level)
	}
}
