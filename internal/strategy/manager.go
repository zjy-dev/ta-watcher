package strategy

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Manager 策略管理器
type Manager struct {
	strategies map[string]Strategy
	mu         sync.RWMutex
	config     *ManagerConfig
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	// 并发执行策略的最大数量
	MaxConcurrentStrategies int

	// 策略执行超时时间
	ExecutionTimeout time.Duration

	// 是否启用调试模式
	DebugMode bool
}

// DefaultManagerConfig 默认管理器配置
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		MaxConcurrentStrategies: 10,
		ExecutionTimeout:        30 * time.Second,
		DebugMode:               false,
	}
}

// NewManager 创建新的策略管理器
func NewManager(config *ManagerConfig) *Manager {
	if config == nil {
		config = DefaultManagerConfig()
	}

	return &Manager{
		strategies: make(map[string]Strategy),
		config:     config,
	}
}

// RegisterStrategy 注册策略
func (m *Manager) RegisterStrategy(strategy Strategy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := strategy.Name()
	if _, exists := m.strategies[name]; exists {
		return fmt.Errorf("strategy '%s' already registered", name)
	}

	m.strategies[name] = strategy
	return nil
}

// UnregisterStrategy 注销策略
func (m *Manager) UnregisterStrategy(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.strategies[name]; !exists {
		return fmt.Errorf("strategy '%s' not found", name)
	}

	delete(m.strategies, name)
	return nil
}

// GetStrategy 获取策略
func (m *Manager) GetStrategy(name string) (Strategy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	strategy, exists := m.strategies[name]
	if !exists {
		return nil, fmt.Errorf("strategy '%s' not found", name)
	}

	return strategy, nil
}

// ListStrategies 列出所有注册的策略
func (m *Manager) ListStrategies() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.strategies))
	for name := range m.strategies {
		names = append(names, name)
	}

	return names
}

// EvaluationRequest 评估请求
type EvaluationRequest struct {
	StrategyNames []string    // 要评估的策略名称列表，空则评估所有策略
	Data          *MarketData // 市场数据
	Context       context.Context
}

// EvaluationResult 评估结果
type EvaluationResult struct {
	StrategyName string
	Result       *StrategyResult
	Error        error
	Duration     time.Duration
}

// EvaluationSummary 评估汇总
type EvaluationSummary struct {
	Results             []*EvaluationResult
	TotalDuration       time.Duration
	SuccessCount        int
	ErrorCount          int
	NotificationResults []*EvaluationResult // 需要发送通知的结果
}

// ShouldNotify 判断汇总结果是否需要发送通知
func (s *EvaluationSummary) ShouldNotify() bool {
	return len(s.NotificationResults) > 0
}

// GetStrongestSignal 获取最强的信号
func (s *EvaluationSummary) GetStrongestSignal() *EvaluationResult {
	if len(s.NotificationResults) == 0 {
		return nil
	}

	strongest := s.NotificationResults[0]
	for _, result := range s.NotificationResults[1:] {
		if result.Result.Strength > strongest.Result.Strength ||
			(result.Result.Strength == strongest.Result.Strength &&
				result.Result.Timestamp.After(strongest.Result.Timestamp)) {
			strongest = result
		}
	}

	return strongest
}

// EvaluateAll 评估所有策略
func (m *Manager) EvaluateAll(data *MarketData) (*EvaluationSummary, error) {
	return m.Evaluate(&EvaluationRequest{
		Data:    data,
		Context: context.Background(),
	})
}

// EvaluateStrategy 评估单个策略
func (m *Manager) EvaluateStrategy(strategyName string, data *MarketData) (*EvaluationResult, error) {
	summary, err := m.Evaluate(&EvaluationRequest{
		StrategyNames: []string{strategyName},
		Data:          data,
		Context:       context.Background(),
	})

	if err != nil {
		return nil, err
	}

	if len(summary.Results) == 0 {
		return nil, fmt.Errorf("no results returned for strategy '%s'", strategyName)
	}

	return summary.Results[0], nil
}

// Evaluate 评估策略
func (m *Manager) Evaluate(req *EvaluationRequest) (*EvaluationSummary, error) {
	if req.Data == nil {
		return nil, fmt.Errorf("market data is required")
	}

	if req.Context == nil {
		req.Context = context.Background()
	}

	// 添加超时控制
	ctx, cancel := context.WithTimeout(req.Context, m.config.ExecutionTimeout)
	defer cancel()

	startTime := time.Now()

	// 确定要评估的策略
	m.mu.RLock()
	strategiesToEvaluate := make(map[string]Strategy)

	if len(req.StrategyNames) == 0 {
		// 评估所有策略
		for name, strategy := range m.strategies {
			strategiesToEvaluate[name] = strategy
		}
	} else {
		// 评估指定策略
		for _, name := range req.StrategyNames {
			if strategy, exists := m.strategies[name]; exists {
				strategiesToEvaluate[name] = strategy
			}
		}
	}
	m.mu.RUnlock()

	if len(strategiesToEvaluate) == 0 {
		return &EvaluationSummary{
			Results:       []*EvaluationResult{},
			TotalDuration: time.Since(startTime),
		}, nil
	}

	// 并发评估策略
	resultsChan := make(chan *EvaluationResult, len(strategiesToEvaluate))
	semaphore := make(chan struct{}, m.config.MaxConcurrentStrategies)

	var wg sync.WaitGroup
	for name, strategy := range strategiesToEvaluate {
		wg.Add(1)
		go func(strategyName string, s Strategy) {
			defer wg.Done()

			// 限制并发数
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				resultsChan <- &EvaluationResult{
					StrategyName: strategyName,
					Error:        fmt.Errorf("strategy evaluation cancelled: %w", ctx.Err()),
				}
				return
			}

			result := m.evaluateStrategy(ctx, strategyName, s, req.Data)
			resultsChan <- result
		}(name, strategy)
	}

	// 等待所有策略评估完成
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// 收集结果
	var results []*EvaluationResult
	var notificationResults []*EvaluationResult
	successCount := 0
	errorCount := 0

	for result := range resultsChan {
		results = append(results, result)

		if result.Error != nil {
			errorCount++
		} else {
			successCount++
			if result.Result != nil && result.Result.ShouldNotify() {
				notificationResults = append(notificationResults, result)
			}
		}
	}

	return &EvaluationSummary{
		Results:             results,
		TotalDuration:       time.Since(startTime),
		SuccessCount:        successCount,
		ErrorCount:          errorCount,
		NotificationResults: notificationResults,
	}, nil
}

// evaluateStrategy 评估单个策略（内部方法）
func (m *Manager) evaluateStrategy(ctx context.Context, name string, strategy Strategy, data *MarketData) *EvaluationResult {
	startTime := time.Now()

	// 检查数据是否充足
	if len(data.Klines) < strategy.RequiredDataPoints() {
		return &EvaluationResult{
			StrategyName: name,
			Error: fmt.Errorf("insufficient data points: required %d, got %d",
				strategy.RequiredDataPoints(), len(data.Klines)),
			Duration: time.Since(startTime),
		}
	}

	// 检查时间框架是否支持
	supportedTimeframes := strategy.SupportedTimeframes()
	if len(supportedTimeframes) > 0 {
		supported := false
		for _, tf := range supportedTimeframes {
			if tf == data.Timeframe {
				supported = true
				break
			}
		}
		if !supported {
			return &EvaluationResult{
				StrategyName: name,
				Error:        fmt.Errorf("unsupported timeframe: %s", data.Timeframe),
				Duration:     time.Since(startTime),
			}
		}
	}

	// 执行策略评估
	resultChan := make(chan *StrategyResult, 1)
	errorChan := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				errorChan <- fmt.Errorf("strategy panic: %v", r)
			}
		}()

		result, err := strategy.Evaluate(data)
		if err != nil {
			errorChan <- err
		} else {
			resultChan <- result
		}
	}()

	// 等待结果或超时
	select {
	case result := <-resultChan:
		return &EvaluationResult{
			StrategyName: name,
			Result:       result,
			Duration:     time.Since(startTime),
		}
	case err := <-errorChan:
		return &EvaluationResult{
			StrategyName: name,
			Error:        err,
			Duration:     time.Since(startTime),
		}
	case <-ctx.Done():
		return &EvaluationResult{
			StrategyName: name,
			Error:        fmt.Errorf("strategy evaluation timeout: %w", ctx.Err()),
			Duration:     time.Since(startTime),
		}
	}
}

// ValidateData 验证市场数据
func (m *Manager) ValidateData(data *MarketData) error {
	if data == nil {
		return fmt.Errorf("market data is nil")
	}

	if data.Symbol == "" {
		return fmt.Errorf("symbol is required")
	}

	if len(data.Klines) == 0 {
		return fmt.Errorf("no kline data provided")
	}

	if data.Timeframe == "" {
		return fmt.Errorf("timeframe is required")
	}

	return nil
}
