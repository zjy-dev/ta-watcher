package datasource

import (
	"context"
	"testing"

	"ta-watcher/internal/config"
)

// TestDataSourceInterface 测试数据源接口一致性
func TestDataSourceInterface(t *testing.T) {
	cfg := config.DefaultConfig()
	factory := NewFactory()

	// 测试支持的数据源类型
	sources := []string{"binance", "coinbase"}

	for _, sourceType := range sources {
		t.Run(sourceType, func(t *testing.T) {
			ds, err := factory.CreateDataSource(sourceType, cfg)
			if err != nil {
				t.Fatalf("Failed to create %s data source: %v", sourceType, err)
			}

			// 检查名称
			name := ds.Name()
			if name == "" {
				t.Errorf("Data source name should not be empty")
			}

			// 检查是否实现了所有接口方法
			ctx := context.Background()

			// 测试符号验证
			valid, err := ds.IsSymbolValid(ctx, "BTCUSDT")
			if err != nil {
				t.Logf("Symbol validation failed for %s: %v (this might be expected for API rate limits)", sourceType, err)
			} else {
				t.Logf("%s: BTCUSDT valid = %t", sourceType, valid)
			}
		})
	}
}

// TestFactoryGetSupportedSources 测试工厂支持的数据源列表
func TestFactoryGetSupportedSources(t *testing.T) {
	factory := NewFactory()
	sources := factory.GetSupportedSources()

	expectedSources := map[string]bool{
		"binance":  true,
		"coinbase": true,
	}

	if len(sources) != len(expectedSources) {
		t.Errorf("Expected %d sources, got %d", len(expectedSources), len(sources))
	}

	for _, source := range sources {
		if !expectedSources[source] {
			t.Errorf("Unexpected source: %s", source)
		}
	}
}

// TestUnsupportedDataSource 测试不支持的数据源
func TestUnsupportedDataSource(t *testing.T) {
	cfg := config.DefaultConfig()
	factory := NewFactory()

	_, err := factory.CreateDataSource("invalid_source", cfg)
	if err == nil {
		t.Error("Expected error for unsupported data source")
	}
}
