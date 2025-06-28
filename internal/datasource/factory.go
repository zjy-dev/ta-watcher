package datasource

import (
	"fmt"

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
	switch sourceType {
	case "binance":
		client := NewBinanceClient()
		return client, nil
	case "coinbase":
		client := NewCoinbaseClient()
		return client, nil
	default:
		return nil, fmt.Errorf("unsupported data source type: %s", sourceType)
	}
}

// GetSupportedSources 获取支持的数据源列表
func (f *Factory) GetSupportedSources() []string {
	return []string{"binance", "coinbase"}
}
