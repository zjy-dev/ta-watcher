package strategy

import (
	"fmt"
	"time"
)

// WeightedSubStrategy 加权子策略
type WeightedSubStrategy struct {
	Strategy Strategy
	Weight   float64
}

// MultiStrategy 多策略组合
type MultiStrategy struct {
	name                string
	description         string
	subStrategies       map[string]*WeightedSubStrategy
	combineMethod       CombineMethod
	minConfidence       float64
	supportedTimeframes []Timeframe
}

// CombineMethod 组合方法
type CombineMethod int

const (
	CombineWeightedAverage CombineMethod = iota // 加权平均
	CombineConsensus                            // 共识（多数决定）
	CombineStrongest                            // 最强信号
	CombineUnanimous                            // 一致性（全部同意）
)

// NewMultiStrategy 创建多策略组合
func NewMultiStrategy(name, description string, combineMethod CombineMethod) *MultiStrategy {
	return &MultiStrategy{
		name:          name,
		description:   description,
		subStrategies: make(map[string]*WeightedSubStrategy),
		combineMethod: combineMethod,
		minConfidence: 0.5, // 默认最小置信度
		supportedTimeframes: []Timeframe{
			Timeframe5m, Timeframe15m, Timeframe30m,
			Timeframe1h, Timeframe2h, Timeframe4h, Timeframe6h, Timeframe12h,
			Timeframe1d, Timeframe3d, Timeframe1w, Timeframe1M,
		},
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

// RequiredDataPoints 返回所需数据点（取所有子策略的最大值）
func (s *MultiStrategy) RequiredDataPoints() int {
	maxPoints := 0
	for _, subStrategy := range s.subStrategies {
		points := subStrategy.Strategy.RequiredDataPoints()
		if points > maxPoints {
			maxPoints = points
		}
	}
	return maxPoints
}

// SupportedTimeframes 返回支持的时间框架
func (s *MultiStrategy) SupportedTimeframes() []Timeframe {
	return s.supportedTimeframes
}

// AddSubStrategy 添加子策略
func (s *MultiStrategy) AddSubStrategy(strategy Strategy, weight float64) {
	if weight <= 0 {
		weight = 1.0
	}

	s.subStrategies[strategy.Name()] = &WeightedSubStrategy{
		Strategy: strategy,
		Weight:   weight,
	}
}

// RemoveSubStrategy 移除子策略
func (s *MultiStrategy) RemoveSubStrategy(name string) {
	delete(s.subStrategies, name)
}

// GetSubStrategies 获取所有子策略
func (s *MultiStrategy) GetSubStrategies() map[string]Strategy {
	strategies := make(map[string]Strategy)
	for name, weighted := range s.subStrategies {
		strategies[name] = weighted.Strategy
	}
	return strategies
}

// SetMinConfidence 设置最小置信度
func (s *MultiStrategy) SetMinConfidence(confidence float64) {
	if confidence >= 0 && confidence <= 1 {
		s.minConfidence = confidence
	}
}

// SubStrategyResult 子策略结果
type SubStrategyResult struct {
	Name   string
	Result *StrategyResult
	Weight float64
	Error  error
}

// Evaluate 评估策略
func (s *MultiStrategy) Evaluate(data *MarketData) (*StrategyResult, error) {
	if len(s.subStrategies) == 0 {
		return nil, fmt.Errorf("no sub-strategies defined")
	}

	// 评估所有子策略
	subResults := make([]*SubStrategyResult, 0, len(s.subStrategies))
	validResults := make([]*SubStrategyResult, 0, len(s.subStrategies))

	for name, weighted := range s.subStrategies {
		result, err := weighted.Strategy.Evaluate(data)
		subResult := &SubStrategyResult{
			Name:   name,
			Result: result,
			Weight: weighted.Weight,
			Error:  err,
		}
		subResults = append(subResults, subResult)

		if err == nil && result != nil {
			validResults = append(validResults, subResult)
		}
	}

	if len(validResults) == 0 {
		return nil, fmt.Errorf("no valid sub-strategy results")
	}

	// 根据组合方法合并结果
	var finalResult *StrategyResult
	var err error

	switch s.combineMethod {
	case CombineWeightedAverage:
		finalResult, err = s.combineWeightedAverage(validResults, data)
	case CombineConsensus:
		finalResult, err = s.combineConsensus(validResults, data)
	case CombineStrongest:
		finalResult, err = s.combineStrongest(validResults, data)
	case CombineUnanimous:
		finalResult, err = s.combineUnanimous(validResults, data)
	default:
		finalResult, err = s.combineWeightedAverage(validResults, data)
	}

	if err != nil {
		return nil, err
	}

	// 添加子策略结果到元数据
	finalResult.Metadata["sub_strategies"] = s.formatSubResults(subResults)
	finalResult.Metadata["combine_method"] = s.combineMethod
	finalResult.Metadata["valid_strategies"] = len(validResults)
	finalResult.Metadata["total_strategies"] = len(subResults)

	return finalResult, nil
}

// combineWeightedAverage 加权平均组合
func (s *MultiStrategy) combineWeightedAverage(results []*SubStrategyResult, data *MarketData) (*StrategyResult, error) {
	var totalWeight, totalConfidence float64
	signalWeights := make(map[Signal]float64)
	strengthWeights := make(map[Strength]float64)
	allIndicators := make(map[string]interface{})

	for _, result := range results {
		weight := result.Weight
		totalWeight += weight

		// 累积置信度
		totalConfidence += result.Result.Confidence * weight

		// 累积信号权重
		signalWeights[result.Result.Signal] += weight

		// 累积强度权重
		strengthWeights[result.Result.Strength] += weight

		// 合并指标
		for k, v := range result.Result.Indicators {
			allIndicators[fmt.Sprintf("%s_%s", result.Name, k)] = v
		}
	}

	// 计算最终信号（权重最高的）
	finalSignal := s.getMaxWeightSignal(signalWeights)
	finalStrength := s.getMaxWeightStrength(strengthWeights)
	finalConfidence := totalConfidence / totalWeight

	// 应用最小置信度过滤
	if finalConfidence < s.minConfidence {
		finalSignal = SignalNone
		finalConfidence = 0.0
	}

	ctx := NewIndicatorContext(data)

	result := &StrategyResult{
		Signal:     finalSignal,
		Strength:   finalStrength,
		Confidence: finalConfidence,
		Price:      ctx.LatestPrice(),
		Timestamp:  time.Now(),
		Message:    s.formatCombinedMessage(finalSignal, finalStrength, finalConfidence, results),
		Metadata:   make(map[string]interface{}),
		Indicators: allIndicators,
	}

	result.Metadata["signal_weights"] = signalWeights
	result.Metadata["total_weight"] = totalWeight

	return result, nil
}

// combineConsensus 共识组合（多数决定）
func (s *MultiStrategy) combineConsensus(results []*SubStrategyResult, data *MarketData) (*StrategyResult, error) {
	signalCounts := make(map[Signal]int)
	signalConfidences := make(map[Signal][]float64)

	for _, result := range results {
		signal := result.Result.Signal
		signalCounts[signal]++
		signalConfidences[signal] = append(signalConfidences[signal], result.Result.Confidence)
	}

	// 找到最多投票的信号
	maxCount := 0
	var finalSignal Signal
	for signal, count := range signalCounts {
		if count > maxCount {
			maxCount = count
			finalSignal = signal
		}
	}

	// 计算该信号的平均置信度
	var totalConfidence float64
	confidences := signalConfidences[finalSignal]
	for _, conf := range confidences {
		totalConfidence += conf
	}
	finalConfidence := totalConfidence / float64(len(confidences))

	// 需要超过半数支持
	requiredVotes := len(results)/2 + 1
	if maxCount < requiredVotes {
		finalSignal = SignalNone
		finalConfidence = 0.0
	}

	ctx := NewIndicatorContext(data)

	return &StrategyResult{
		Signal:     finalSignal,
		Strength:   StrengthNormal,
		Confidence: finalConfidence,
		Price:      ctx.LatestPrice(),
		Timestamp:  time.Now(),
		Message:    s.formatConsensusMessage(finalSignal, maxCount, len(results)),
		Metadata:   map[string]interface{}{"signal_votes": signalCounts},
		Indicators: make(map[string]interface{}),
	}, nil
}

// combineStrongest 最强信号组合
func (s *MultiStrategy) combineStrongest(results []*SubStrategyResult, data *MarketData) (*StrategyResult, error) {
	var strongest *SubStrategyResult
	maxScore := 0.0

	for _, result := range results {
		// 计算综合得分（强度 + 置信度）
		strengthScore := float64(result.Result.Strength) + 1.0
		score := strengthScore * result.Result.Confidence

		if score > maxScore {
			maxScore = score
			strongest = result
		}
	}

	if strongest == nil {
		return nil, fmt.Errorf("no strongest strategy found")
	}

	// 复制最强策略的结果
	result := &StrategyResult{
		Signal:     strongest.Result.Signal,
		Strength:   strongest.Result.Strength,
		Confidence: strongest.Result.Confidence,
		Price:      strongest.Result.Price,
		Timestamp:  time.Now(),
		Message:    fmt.Sprintf("最强信号来自%s: %s", strongest.Name, strongest.Result.Message),
		Metadata:   make(map[string]interface{}),
		Indicators: strongest.Result.Indicators,
	}

	result.Metadata["strongest_strategy"] = strongest.Name
	result.Metadata["score"] = maxScore

	return result, nil
}

// combineUnanimous 一致性组合（全部同意）
func (s *MultiStrategy) combineUnanimous(results []*SubStrategyResult, data *MarketData) (*StrategyResult, error) {
	if len(results) == 0 {
		return nil, fmt.Errorf("no results to combine")
	}

	// 检查是否所有策略都给出相同信号
	firstSignal := results[0].Result.Signal
	allSame := true
	var totalConfidence float64

	for _, result := range results {
		if result.Result.Signal != firstSignal {
			allSame = false
			break
		}
		totalConfidence += result.Result.Confidence
	}

	ctx := NewIndicatorContext(data)

	if allSame && firstSignal != SignalNone {
		// 全部一致且不是无信号
		avgConfidence := totalConfidence / float64(len(results))

		return &StrategyResult{
			Signal:     firstSignal,
			Strength:   StrengthStrong,                     // 一致性强度高
			Confidence: minFloat64(avgConfidence*1.2, 1.0), // 一致性加成
			Price:      ctx.LatestPrice(),
			Timestamp:  time.Now(),
			Message:    fmt.Sprintf("全体一致%s信号 (%d个策略)", firstSignal.String(), len(results)),
			Metadata:   map[string]interface{}{"unanimous": true},
			Indicators: make(map[string]interface{}),
		}, nil
	} else {
		// 没有一致性
		return &StrategyResult{
			Signal:     SignalNone,
			Strength:   StrengthWeak,
			Confidence: 0.0,
			Price:      ctx.LatestPrice(),
			Timestamp:  time.Now(),
			Message:    fmt.Sprintf("策略分歧，无一致性信号 (%d个策略)", len(results)),
			Metadata:   map[string]interface{}{"unanimous": false},
			Indicators: make(map[string]interface{}),
		}, nil
	}
}

// 辅助方法
func (s *MultiStrategy) getMaxWeightSignal(weights map[Signal]float64) Signal {
	var maxSignal Signal
	var maxWeight float64

	for signal, weight := range weights {
		if weight > maxWeight {
			maxWeight = weight
			maxSignal = signal
		}
	}

	return maxSignal
}

func (s *MultiStrategy) getMaxWeightStrength(weights map[Strength]float64) Strength {
	var maxStrength Strength
	var maxWeight float64

	for strength, weight := range weights {
		if weight > maxWeight {
			maxWeight = weight
			maxStrength = strength
		}
	}

	return maxStrength
}

func (s *MultiStrategy) formatCombinedMessage(signal Signal, strength Strength, confidence float64, results []*SubStrategyResult) string {
	return fmt.Sprintf("组合策略%s信号 (强度:%s, 置信度:%.2f, %d个子策略)",
		signal.String(), strength.String(), confidence, len(results))
}

func (s *MultiStrategy) formatConsensusMessage(signal Signal, votes, total int) string {
	if signal == SignalNone {
		return fmt.Sprintf("共识失败：无足够投票 (%d/%d)", votes, total)
	}
	return fmt.Sprintf("共识%s信号 (%d/%d投票)", signal.String(), votes, total)
}

func (s *MultiStrategy) formatSubResults(results []*SubStrategyResult) []map[string]interface{} {
	formatted := make([]map[string]interface{}, len(results))
	for i, result := range results {
		item := map[string]interface{}{
			"name":   result.Name,
			"weight": result.Weight,
		}

		if result.Error != nil {
			item["error"] = result.Error.Error()
		} else if result.Result != nil {
			item["signal"] = result.Result.Signal.String()
			item["strength"] = result.Result.Strength.String()
			item["confidence"] = result.Result.Confidence
		}

		formatted[i] = item
	}
	return formatted
}
