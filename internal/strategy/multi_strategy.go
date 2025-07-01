package strategy

import (
	"fmt"
	"time"

	"ta-watcher/internal/datasource"
)

// MultiStrategy 多策略组合 - 专为通知系统设计
type MultiStrategy struct {
	name          string
	description   string
	subStrategies map[string]Strategy
}

// NewMultiStrategy 创建多策略组合
func NewMultiStrategy(name, description string) *MultiStrategy {
	return &MultiStrategy{
		name:          name,
		description:   description,
		subStrategies: make(map[string]Strategy),
	}
}

// Name 返回策略名称
func (s *MultiStrategy) Name() string {
	return s.name
}

// Description 返回策略描述
func (s *MultiStrategy) Description() string {
	return s.description
}

// AddSubStrategy 添加子策略
func (s *MultiStrategy) AddSubStrategy(strategy Strategy) {
	s.subStrategies[strategy.Name()] = strategy
}

// RemoveSubStrategy 移除子策略
func (s *MultiStrategy) RemoveSubStrategy(name string) {
	delete(s.subStrategies, name)
}

// GetSubStrategies 获取所有子策略
func (s *MultiStrategy) GetSubStrategies() map[string]Strategy {
	strategies := make(map[string]Strategy)
	for name, strategy := range s.subStrategies {
		strategies[name] = strategy
	}
	return strategies
}

// RequiredDataPoints 返回所需的最少数据点数（取所有子策略的最大值）
func (s *MultiStrategy) RequiredDataPoints() int {
	maxPoints := 0
	for _, strategy := range s.subStrategies {
		if points := strategy.RequiredDataPoints(); points > maxPoints {
			maxPoints = points
		}
	}
	return maxPoints
}

// SupportedTimeframes 返回支持的时间框架（所有子策略的交集）
func (s *MultiStrategy) SupportedTimeframes() []datasource.Timeframe {
	if len(s.subStrategies) == 0 {
		return []datasource.Timeframe{}
	}

	// 取第一个策略的时间框架作为基准
	var baseTimeframes []datasource.Timeframe
	for _, strategy := range s.subStrategies {
		baseTimeframes = strategy.SupportedTimeframes()
		break
	}

	// 求交集
	supported := make([]datasource.Timeframe, 0)
	for _, tf := range baseTimeframes {
		allSupport := true
		for _, strategy := range s.subStrategies {
			if !contains(strategy.SupportedTimeframes(), tf) {
				allSupport = false
				break
			}
		}
		if allSupport {
			supported = append(supported, tf)
		}
	}

	return supported
}

// Evaluate 评估策略 - 通知器逻辑：任何一个策略触发都返回信号
func (s *MultiStrategy) Evaluate(data *MarketData) (*StrategyResult, error) {
	if len(s.subStrategies) == 0 {
		return nil, fmt.Errorf("no sub-strategies defined")
	}

	var triggeredResults []*StrategyResult
	var allResults []string

	// 评估所有子策略
	for name, strategy := range s.subStrategies {
		result, err := strategy.Evaluate(data)
		if err != nil {
			allResults = append(allResults, fmt.Sprintf("%s: Error(%v)", name, err))
			continue
		}

		if result == nil {
			allResults = append(allResults, fmt.Sprintf("%s: No signal", name))
			continue
		}

		allResults = append(allResults, fmt.Sprintf("%s: %s",
			name, result.Signal.String()))

		// 只有买入/卖出信号才算触发（忽略Hold和None）
		if result.Signal == SignalBuy || result.Signal == SignalSell {
			triggeredResults = append(triggeredResults, result)
		}
	}

	// 如果没有任何策略触发，返回无信号
	if len(triggeredResults) == 0 {
		return &StrategyResult{
			Signal:           SignalNone,
			Strength:         StrengthWeak,
			Timestamp:        time.Now(),
			Message:          fmt.Sprintf("组合策略 %s: 无触发信号", s.name),
			IndicatorSummary: fmt.Sprintf("组合策略(%d个子策略): 无信号", len(s.subStrategies)),
			DetailedAnalysis: fmt.Sprintf("组合策略 %s 包含 %d 个子策略，当前无任何策略触发买入或卖出信号。", s.name, len(s.subStrategies)),
			Indicators:       map[string]interface{}{"price": getCurrentPrice(data)},
			Thresholds:       map[string]interface{}{},
			Metadata: map[string]interface{}{
				"sub_results":      allResults,
				"triggered_count":  0,
				"total_strategies": len(s.subStrategies),
			},
		}, nil
	}

	// 选择信号强度最高的信号作为代表
	bestResult := triggeredResults[0]
	for _, result := range triggeredResults[1:] {
		if result.Strength > bestResult.Strength ||
			(result.Strength == bestResult.Strength &&
				result.Timestamp.After(bestResult.Timestamp)) {
			bestResult = result
		}
	}

	// 构造组合结果
	return &StrategyResult{
		Signal:           bestResult.Signal,
		Strength:         bestResult.Strength,
		Timestamp:        time.Now(),
		Message:          s.formatNotificationMessage(triggeredResults),
		IndicatorSummary: fmt.Sprintf("组合策略(%d个子策略): %d个触发", len(s.subStrategies), len(triggeredResults)),
		DetailedAnalysis: s.formatDetailedAnalysis(triggeredResults, allResults),
		Indicators:       bestResult.Indicators,
		Thresholds:       bestResult.Thresholds,
		Metadata: map[string]interface{}{
			"sub_results":          allResults,
			"triggered_count":      len(triggeredResults),
			"total_strategies":     len(s.subStrategies),
			"triggered_strategies": s.getTriggeredNames(triggeredResults),
		},
	}, nil
}

// formatDetailedAnalysis 格式化详细分析
func (s *MultiStrategy) formatDetailedAnalysis(triggered []*StrategyResult, allResults []string) string {
	analysis := fmt.Sprintf("组合策略 %s 包含 %d 个子策略，其中 %d 个触发了信号:\n",
		s.name, len(s.subStrategies), len(triggered))

	for i, result := range triggered {
		analysis += fmt.Sprintf("  %d. %s: %s\n", i+1,
			s.getStrategyNameForResult(result), result.Message)
	}

	if len(triggered) > 1 {
		analysis += fmt.Sprintf("\n选择了信号强度最高的策略作为组合信号。")
	}

	return analysis
}

// getStrategyNameForResult 获取结果对应的策略名称
func (s *MultiStrategy) getStrategyNameForResult(result *StrategyResult) string {
	// 这里需要根据实际情况实现，暂时返回通用名称
	return "子策略"
}

// formatNotificationMessage 格式化通知消息
func (s *MultiStrategy) formatNotificationMessage(triggered []*StrategyResult) string {
	if len(triggered) == 1 {
		return fmt.Sprintf("🔄 组合策略 %s: %s信号",
			s.name, triggered[0].Signal.String())
	}

	return fmt.Sprintf("组合策略 %s: 检测到%d个信号触发", s.name, len(triggered))
}

// getTriggeredNames 获取触发的策略名称
func (s *MultiStrategy) getTriggeredNames(triggered []*StrategyResult) []string {
	names := make([]string, 0, len(triggered))
	for _, result := range triggered {
		// 从metadata中获取策略名称，如果没有就用信号类型
		if name, ok := result.Metadata["strategy_name"].(string); ok {
			names = append(names, name)
		} else {
			names = append(names, result.Signal.String())
		}
	}
	return names
}

// getCurrentPrice 获取当前价格
func getCurrentPrice(data *MarketData) float64 {
	if len(data.Klines) == 0 {
		return 0.0
	}
	return data.Klines[len(data.Klines)-1].Close
}

// contains 检查切片是否包含元素
func contains(slice []datasource.Timeframe, item datasource.Timeframe) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
