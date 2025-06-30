package datasource

import (
	"testing"

	"ta-watcher/internal/config"
)

func TestFactory_New(t *testing.T) {
	factory := NewFactory()

	if factory == nil {
		t.Fatal("NewFactory() returned nil")
	}
}

func TestFactory_GetSupportedSources(t *testing.T) {
	factory := NewFactory()
	sources := factory.GetSupportedSources()

	expectedSources := []string{"binance", "coinbase"}

	if len(sources) != len(expectedSources) {
		t.Errorf("Expected %d sources, got %d", len(expectedSources), len(sources))
	}

	sourceMap := make(map[string]bool)
	for _, source := range sources {
		sourceMap[source] = true
	}

	for _, expected := range expectedSources {
		if !sourceMap[expected] {
			t.Errorf("Expected source '%s' not found in supported sources", expected)
		}
	}
}

func TestFactory_CreateDataSource_Interface(t *testing.T) {
	factory := NewFactory()
	cfg := &config.Config{}

	sources := []string{"binance", "coinbase"}

	for _, sourceType := range sources {
		t.Run(sourceType, func(t *testing.T) {
			ds, err := factory.CreateDataSource(sourceType, cfg)
			if err != nil {
				t.Fatalf("Failed to create %s data source: %v", sourceType, err)
			}

			if ds == nil {
				t.Fatal("DataSource is nil")
			}

			// 验证接口方法存在
			name := ds.Name()
			if name == "" {
				t.Error("DataSource name should not be empty")
			}

			t.Logf("Successfully created %s data source: %s", sourceType, name)
		})
	}
}
