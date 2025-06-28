package assets

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"ta-watcher/internal/config"
	"ta-watcher/internal/datasource"
)

// Validator 资产验证器
type Validator struct {
	client           datasource.DataSource
	config           *config.AssetsConfig
	marketCapManager *MarketCapManager
}

// NewValidator 创建新的资产验证器
func NewValidator(client datasource.DataSource, config *config.AssetsConfig) *Validator {
	// 创建市值管理器（使用模拟数据，生产环境可替换为真实API）
	marketCapProvider := NewMockMarketCapProvider()
	marketCapManager := NewMarketCapManager(marketCapProvider, config.MarketCapUpdateInterval)

	return &Validator{
		client:           client,
		config:           config,
		marketCapManager: marketCapManager,
	}
}

// ValidateAssets 验证所有配置的资产
func (v *Validator) ValidateAssets(ctx context.Context) (*ValidationResult, error) {
	log.Println("开始验证资产配置...")

	result := &ValidationResult{
		ValidSymbols:        make([]string, 0),
		ValidPairs:          make([]string, 0),
		CalculatedPairs:     make([]string, 0),
		MissingSymbols:      make([]string, 0),
		SupportedTimeframes: v.config.Timeframes,
	}

	// 1. 验证所有币种对基准货币的交易对
	log.Printf("验证币种对%s的交易对...", v.config.BaseCurrency)
	for _, symbol := range v.config.Symbols {
		pair := symbol + v.config.BaseCurrency
		if err := v.validateSymbolPair(ctx, pair); err != nil {
			log.Printf("警告: %s 不存在，跳过该币种: %v", pair, err)
			result.MissingSymbols = append(result.MissingSymbols, symbol)
			continue
		}
		result.ValidSymbols = append(result.ValidSymbols, symbol)
		result.ValidPairs = append(result.ValidPairs, pair)
		log.Printf("✓ %s 验证通过", pair)
	}

	if len(result.ValidSymbols) == 0 {
		return nil, fmt.Errorf("没有找到任何有效的币种")
	}

	// 2. 计算需要的汇率交易对
	log.Println("计算币种间汇率交易对...")
	ratePairs := v.calculateRatePairs(result.ValidSymbols)

	for _, pair := range ratePairs {
		if err := v.validateSymbolPair(ctx, pair); err != nil {
			// 如果直接交易对不存在，我们需要通过计算来获得汇率
			log.Printf("注意: %s 不存在，将通过计算获得汇率", pair)
			result.CalculatedPairs = append(result.CalculatedPairs, pair)
		} else {
			result.ValidPairs = append(result.ValidPairs, pair)
			log.Printf("✓ %s 汇率对验证通过", pair)
		}
	}

	// 3. 验证时间框架
	log.Println("验证时间框架支持...")
	for _, tf := range v.config.Timeframes {
		log.Printf("✓ 时间框架 %s 将被监控", tf)
	}

	log.Printf("资产验证完成: %d个有效币种, %d个直接交易对, %d个计算汇率对",
		len(result.ValidSymbols), len(result.ValidPairs)-len(ratePairs)+len(ratePairs)-len(result.CalculatedPairs), len(result.CalculatedPairs))

	return result, nil
}

// validateSymbolPair 验证单个交易对是否存在
func (v *Validator) validateSymbolPair(ctx context.Context, symbol string) error {
	// 使用 IsSymbolValid 方法来验证交易对是否存在
	valid, err := v.client.IsSymbolValid(ctx, symbol)
	if err != nil {
		return err
	}
	if !valid {
		return fmt.Errorf("symbol %s is not valid", symbol)
	}
	return nil
}

// calculateRatePairs 计算需要的汇率交易对
func (v *Validator) calculateRatePairs(validSymbols []string) []string {
	if len(validSymbols) < 2 {
		return []string{}
	}

	// 获取市值数据
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	marketCaps, err := v.marketCapManager.GetMarketCaps(ctx, validSymbols)
	if err != nil {
		log.Printf("警告: 无法获取市值数据，使用默认排序: %v", err)
		// 如果无法获取市值，使用固定顺序生成少量交易对
		pairs := make([]string, 0)
		if len(validSymbols) >= 2 {
			// 生成前几个币种的交叉对
			for i := 0; i < len(validSymbols) && i < 3; i++ {
				for j := i + 1; j < len(validSymbols) && j < 4; j++ {
					pair := validSymbols[i] + validSymbols[j]
					pairs = append(pairs, pair)
				}
			}
		}
		return pairs
	}

	// 基于市值生成交叉汇率对（限制数量避免过多请求）
	maxPairs := 10 // 最多生成10个交叉汇率对
	pairs := GenerateCrossRatePairs(validSymbols, marketCaps, maxPairs)

	log.Printf("基于市值生成 %d 个交叉汇率对", len(pairs))
	for _, pair := range pairs {
		log.Printf("- %s", pair)
	}

	return pairs
}

// ValidationResult 验证结果
type ValidationResult struct {
	// 有效的加密货币符号列表
	ValidSymbols []string

	// 有效的交易对列表（直接存在的）
	ValidPairs []string

	// 需要计算的汇率对列表（不直接存在的）
	CalculatedPairs []string

	// 缺失的币种列表
	MissingSymbols []string

	// 支持的时间框架
	SupportedTimeframes []string
}

// GetAllMonitoringPairs 获取所有需要监控的交易对
func (r *ValidationResult) GetAllMonitoringPairs() []string {
	all := make([]string, 0, len(r.ValidPairs)+len(r.CalculatedPairs))
	all = append(all, r.ValidPairs...)
	all = append(all, r.CalculatedPairs...)

	// 去重并排序
	uniquePairs := make(map[string]bool)
	for _, pair := range all {
		uniquePairs[pair] = true
	}

	result := make([]string, 0, len(uniquePairs))
	for pair := range uniquePairs {
		result = append(result, pair)
	}
	sort.Strings(result)

	return result
}

// Summary 返回验证结果摘要
func (r *ValidationResult) Summary() string {
	var summary strings.Builder

	summary.WriteString("资产验证结果:\n")
	summary.WriteString(fmt.Sprintf("- 有效币种: %d个 (%s)\n", len(r.ValidSymbols), strings.Join(r.ValidSymbols, ", ")))
	summary.WriteString(fmt.Sprintf("- 直接交易对: %d个\n", len(r.ValidPairs)))
	summary.WriteString(fmt.Sprintf("- 计算汇率对: %d个\n", len(r.CalculatedPairs)))
	summary.WriteString(fmt.Sprintf("- 时间框架: %s\n", strings.Join(r.SupportedTimeframes, ", ")))

	if len(r.MissingSymbols) > 0 {
		summary.WriteString(fmt.Sprintf("- 缺失币种: %s\n", strings.Join(r.MissingSymbols, ", ")))
	}

	return summary.String()
}
