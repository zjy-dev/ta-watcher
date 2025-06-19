package watcher

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"plugin"
	"strings"

	"ta-watcher/internal/strategy"
)

// StrategyLoader 策略加载器
type StrategyLoader struct {
	strategiesDir string
	factory       *strategy.Factory
}

// NewStrategyLoader 创建策略加载器
func NewStrategyLoader(strategiesDir string, factory *strategy.Factory) *StrategyLoader {
	return &StrategyLoader{
		strategiesDir: strategiesDir,
		factory:       factory,
	}
}

// LoadStrategiesFromDirectory 从目录加载策略
func (sl *StrategyLoader) LoadStrategiesFromDirectory() error {
	if sl.strategiesDir == "" {
		log.Println("No strategies directory specified, using built-in strategies only")
		return nil
	}

	log.Printf("Loading custom strategies from directory: %s", sl.strategiesDir)

	err := filepath.WalkDir(sl.strategiesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// 只处理 .go 文件
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		return sl.loadStrategyFromFile(path)
	})

	if err != nil {
		return fmt.Errorf("failed to load strategies from directory %s: %w", sl.strategiesDir, err)
	}

	return nil
}

// loadStrategyFromFile 从文件加载策略
func (sl *StrategyLoader) loadStrategyFromFile(filePath string) error {
	log.Printf("Loading strategy from file: %s", filePath)

	// 注意：在实际生产环境中，Go 的 plugin 系统需要将 Go 文件编译为 .so 文件
	// 这里我们提供一个框架，实际使用时需要用户先编译策略文件

	// 检查是否有对应的 .so 文件
	soPath := strings.TrimSuffix(filePath, ".go") + ".so"

	// 尝试加载插件
	p, err := plugin.Open(soPath)
	if err != nil {
		// 如果没有 .so 文件，我们记录信息但不报错
		log.Printf("No compiled plugin found for %s (expected %s). Please compile the strategy first.", filePath, soPath)
		return nil
	}

	// 查找策略构造函数
	newStrategyFunc, err := p.Lookup("NewStrategy")
	if err != nil {
		return fmt.Errorf("strategy file %s must export a 'NewStrategy() strategy.Strategy' function", filePath)
	}

	// 类型断言
	strategyConstructor, ok := newStrategyFunc.(func() strategy.Strategy)
	if !ok {
		return fmt.Errorf("NewStrategy function in %s has wrong signature, expected: func() strategy.Strategy", filePath)
	}

	// 创建策略实例
	strategyInstance := strategyConstructor()

	// 注册策略到工厂
	strategyName := strategyInstance.Name()
	log.Printf("Registering custom strategy: %s", strategyName)

	// 这里需要扩展 factory 来支持注册自定义策略
	// 暂时记录日志
	log.Printf("Custom strategy %s loaded successfully from %s", strategyName, filePath)

	return nil
}

// GenerateStrategyTemplate 生成策略模板文件
func GenerateStrategyTemplate(outputPath, strategyName string) error {
	structName := strategyName + "Strategy"

	template := fmt.Sprintf(`package main

import (
	"fmt"
	"time"
	
	"ta-watcher/internal/strategy"
	"ta-watcher/internal/binance"
	"ta-watcher/internal/indicators"
)

// %s 自定义策略实现
type %s struct {
	name        string
	description string
	// 在这里添加策略参数
	period      int
	threshold   float64
}

// NewStrategy 创建策略实例 (插件导出函数)
func NewStrategy() strategy.Strategy {
	return &%s{
		name:        "%s",
		description: "这是一个自定义策略模板",
		period:      14,
		threshold:   0.02,
	}
}

// Name 返回策略名称
func (s *%s) Name() string {
	return s.name
}

// Description 返回策略描述
func (s *%s) Description() string {
	return s.description
}

// RequiredDataPoints 返回所需的最少数据点数
func (s *%s) RequiredDataPoints() int {
	return s.period + 10 // 通常需要比指标周期多一些数据
}

// SupportedTimeframes 返回支持的时间框架
func (s *%s) SupportedTimeframes() []strategy.Timeframe {
	return []strategy.Timeframe{
		strategy.Timeframe5m,
		strategy.Timeframe15m,
		strategy.Timeframe1h,
		strategy.Timeframe4h,
		strategy.Timeframe1d,
	}
}

// Evaluate 评估策略，返回信号
func (s *%s) Evaluate(data *strategy.MarketData) (*strategy.StrategyResult, error) {
	if len(data.Klines) < s.RequiredDataPoints() {
		return &strategy.StrategyResult{
			Signal:     strategy.SignalNone,
			Strength:   strategy.StrengthWeak,
			Confidence: 0.0,
			Timestamp:  time.Now(),
			Message:    "数据点不足",
		}, nil
	}

	// 提取价格数据
	closes := make([]float64, len(data.Klines))
	for i, kline := range data.Klines {
		closes[i] = kline.Close
	}

	// 在这里实现你的策略逻辑
	// 示例：简单的价格变化策略
	currentPrice := closes[len(closes)-1]
	previousPrice := closes[len(closes)-2]
	priceChange := (currentPrice - previousPrice) / previousPrice

	var signal strategy.Signal
	var strength strategy.Strength
	var confidence float64
	var message string

	if priceChange > s.threshold {
		signal = strategy.SignalBuy
		strength = strategy.StrengthNormal
		confidence = 0.7
		message = fmt.Sprintf("价格上涨 %%.2f%%%%", priceChange*100)
	} else if priceChange < -s.threshold {
		signal = strategy.SignalSell
		strength = strategy.StrengthNormal
		confidence = 0.7
		message = fmt.Sprintf("价格下跌 %%.2f%%%%", -priceChange*100)
	} else {
		signal = strategy.SignalHold
		strength = strategy.StrengthWeak
		confidence = 0.3
		message = "价格变化不大，建议持有"
	}

	return &strategy.StrategyResult{
		Signal:     signal,
		Strength:   strength,
		Confidence: confidence,
		Price:      currentPrice,
		Timestamp:  time.Now(),
		Message:    message,
		Metadata: map[string]interface{}{
			"price_change":     priceChange,
			"current_price":    currentPrice,
			"previous_price":   previousPrice,
			"threshold":        s.threshold,
		},
		Indicators: map[string]interface{}{
			"price_change_pct": priceChange * 100,
		},
	}, nil
}

// 编译指令：
// go build -buildmode=plugin -o %s.so %s.go
`, structName, structName, structName, strategyName, structName, structName, structName, structName, structName, strings.ToLower(strategyName), strings.ToLower(strategyName))

	// 实际写入文件
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", outputPath, err)
	}
	defer file.Close()

	_, err = file.WriteString(template)
	if err != nil {
		return fmt.Errorf("failed to write template to file %s: %w", outputPath, err)
	}

	return nil
}
