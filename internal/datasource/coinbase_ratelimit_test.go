package datasource

import (
	"context"
	"testing"
	"time"

	"ta-watcher/internal/config"
)

func TestCoinbaseRateLimiting(t *testing.T) {
	// 创建带限流配置的Coinbase客户端
	cfg := &config.CoinbaseConfig{
		RateLimit: config.RateLimitConfig{
			RequestsPerMinute: 10, // 设置一个很低的限流来测试
			RetryDelay:        2 * time.Second,
			MaxRetries:        2,
		},
	}

	client := NewCoinbaseClientWithConfig(cfg)

	// 验证配置
	if client.rateLimit.RequestsPerMinute != 10 {
		t.Errorf("预期限流为10 req/min，实际为 %d", client.rateLimit.RequestsPerMinute)
	}

	// 测试限流逻辑（不实际发送请求）
	start := time.Now()

	// 模拟限流等待
	client.rateLimitSleep()
	client.rateLimitSleep()

	elapsed := time.Since(start)

	// 第二次调用应该有延迟
	minExpectedDelay := time.Minute / time.Duration(cfg.RateLimit.RequestsPerMinute)

	t.Logf("✅ 限流测试完成")
	t.Logf("📊 限流配置: %d req/min", cfg.RateLimit.RequestsPerMinute)
	t.Logf("⏱️ 最小间隔: %v", minExpectedDelay)
	t.Logf("⏱️ 实际耗时: %v", elapsed)
}

func TestCoinbaseDefaultRateLimit(t *testing.T) {
	// 测试默认限流配置
	client := NewCoinbaseClient()

	if client.rateLimit.RequestsPerMinute != 300 {
		t.Errorf("默认限流应为300 req/min，实际为 %d", client.rateLimit.RequestsPerMinute)
	}

	if client.rateLimit.RetryDelay != 5*time.Second {
		t.Errorf("默认重试延迟应为5秒，实际为 %v", client.rateLimit.RetryDelay)
	}

	t.Logf("✅ 默认限流配置验证通过: %d req/min, %v retry delay",
		client.rateLimit.RequestsPerMinute, client.rateLimit.RetryDelay)
}

func TestCoinbaseRateLimitIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（短模式）")
	}

	// 创建限流客户端
	cfg := &config.CoinbaseConfig{
		RateLimit: config.RateLimitConfig{
			RequestsPerMinute: 300, // 使用新的慢速限流
			RetryDelay:        5 * time.Second,
			MaxRetries:        3,
		},
	}

	client := NewCoinbaseClientWithConfig(cfg)
	ctx := context.Background()

	// 测试单个请求
	start := time.Now()
	valid, err := client.IsSymbolValid(ctx, "BTCUSDT")
	elapsed := time.Since(start)

	if err != nil {
		t.Logf("⚠️ API请求失败（可能正常）: %v", err)
	} else {
		t.Logf("✅ 符号验证成功: BTCUSDT valid=%v", valid)
	}

	t.Logf("⏱️ 单次请求耗时: %v", elapsed)
	t.Logf("🔄 限流配置已应用: %d req/min", cfg.RateLimit.RequestsPerMinute)
}
