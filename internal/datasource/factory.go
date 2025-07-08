package datasource

import (
	"fmt"
	"log"

	"ta-watcher/internal/config"
)

// Factory 数据源工厂
type Factory struct{}

// NewFactory 创建数据源工厂
func NewFactory() *Factory {
	return &Factory{}
}

// CreateDataSource 根据配置创建数据源
func (f *Factory) CreateDataSource(sourceType string, cfg *config.Config) (DataSource, error) {
	log.Printf("🏭 创建数据源: %s", sourceType)

	switch sourceType {
	case "binance":
		log.Printf("🔧 Binance 限流配置:")
		log.Printf("   ├── 每分钟请求数: %d", cfg.DataSource.Binance.RateLimit.RequestsPerMinute)
		log.Printf("   ├── 重试延迟: %v", cfg.DataSource.Binance.RateLimit.RetryDelay)
		log.Printf("   └── 最大重试: %d", cfg.DataSource.Binance.RateLimit.MaxRetries)
		client := NewBinanceClientWithConfig(&cfg.DataSource.Binance)
		return client, nil
	case "coinbase":
		log.Printf("🔧 Coinbase 限流配置:")
		log.Printf("   ├── 每分钟请求数: %d", cfg.DataSource.Coinbase.RateLimit.RequestsPerMinute)
		log.Printf("   ├── 重试延迟: %v", cfg.DataSource.Coinbase.RateLimit.RetryDelay)
		log.Printf("   └── 最大重试: %d", cfg.DataSource.Coinbase.RateLimit.MaxRetries)
		client := NewCoinbaseClientWithConfig(&cfg.DataSource.Coinbase)
		return client, nil
	default:
		log.Printf("❌ 不支持的数据源类型: %s", sourceType)
		return nil, fmt.Errorf("unsupported data source type: %s", sourceType)
	}
}

// GetSupportedSources 获取支持的数据源列表
func (f *Factory) GetSupportedSources() []string {
	return []string{"binance", "coinbase"}
}
