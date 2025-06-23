package assets

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
)

// MarketCapProvider 市值数据提供者接口
type MarketCapProvider interface {
	GetMarketCaps(ctx context.Context, symbols []string) (map[string]float64, error)
}

// CoinGeckoProvider CoinGecko API 市值数据提供者
type CoinGeckoProvider struct {
	client  *http.Client
	baseURL string
	apiKey  string // 可选的API密钥
}

// NewCoinGeckoProvider 创建新的 CoinGecko 提供者
func NewCoinGeckoProvider(apiKey string) *CoinGeckoProvider {
	return &CoinGeckoProvider{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://api.coingecko.com/api/v3",
		apiKey:  apiKey,
	}
}

// CoinGeckoResponse CoinGecko API 响应结构
type CoinGeckoResponse []struct {
	ID                 string  `json:"id"`
	Symbol             string  `json:"symbol"`
	Name               string  `json:"name"`
	MarketCap          float64 `json:"market_cap"`
	MarketCapRank      int     `json:"market_cap_rank"`
	CurrentPrice       float64 `json:"current_price"`
	PriceChange24h     float64 `json:"price_change_24h"`
	PriceChangePerc24h float64 `json:"price_change_percentage_24h"`
}

// GetMarketCaps 获取指定币种的市值数据
func (p *CoinGeckoProvider) GetMarketCaps(ctx context.Context, symbols []string) (map[string]float64, error) {
	// 将币种符号转换为小写，CoinGecko API 使用小写
	lowerSymbols := make([]string, len(symbols))
	for i, symbol := range symbols {
		lowerSymbols[i] = strings.ToLower(symbol)
	}

	// 构建API URL
	symbolsParam := strings.Join(lowerSymbols, ",")
	url := fmt.Sprintf("%s/coins/markets?vs_currency=usd&ids=%s&order=market_cap_desc&per_page=%d&page=1",
		p.baseURL, symbolsParam, len(symbols))

	// 如果有API密钥，添加到请求中
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if p.apiKey != "" {
		req.Header.Set("x-cg-demo-api-key", p.apiKey)
	}

	req.Header.Set("User-Agent", "TA-Watcher/1.0")
	req.Header.Set("Accept", "application/json")

	// 发送请求
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch market cap data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	// 解析响应
	var coinData CoinGeckoResponse
	if err := json.NewDecoder(resp.Body).Decode(&coinData); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// 构建结果映射
	result := make(map[string]float64)
	for _, coin := range coinData {
		symbol := strings.ToUpper(coin.Symbol)
		result[symbol] = coin.MarketCap
		log.Printf("市值数据: %s = $%.0f (排名: %d)", symbol, coin.MarketCap, coin.MarketCapRank)
	}

	return result, nil
}

// MockMarketCapProvider 模拟市值数据提供者（用于测试）
type MockMarketCapProvider struct {
	data map[string]float64
}

// NewMockMarketCapProvider 创建模拟市值数据提供者
func NewMockMarketCapProvider() *MockMarketCapProvider {
	return &MockMarketCapProvider{
		data: map[string]float64{
			"BTC":   800000000000, // 8000亿美元
			"ETH":   400000000000, // 4000亿美元
			"BNB":   50000000000,  // 500亿美元
			"ADA":   20000000000,  // 200亿美元
			"SOL":   30000000000,  // 300亿美元
			"DOT":   10000000000,  // 100亿美元
			"MATIC": 8000000000,   // 80亿美元
			"AVAX":  12000000000,  // 120亿美元
		},
	}
}

// GetMarketCaps 获取模拟市值数据
func (p *MockMarketCapProvider) GetMarketCaps(ctx context.Context, symbols []string) (map[string]float64, error) {
	result := make(map[string]float64)
	for _, symbol := range symbols {
		if marketCap, exists := p.data[symbol]; exists {
			result[symbol] = marketCap
		}
	}
	return result, nil
}

// MarketCapManager 市值管理器
type MarketCapManager struct {
	provider   MarketCapProvider
	cache      map[string]float64
	lastUpdate time.Time
	ttl        time.Duration
}

// NewMarketCapManager 创建新的市值管理器
func NewMarketCapManager(provider MarketCapProvider, ttl time.Duration) *MarketCapManager {
	return &MarketCapManager{
		provider: provider,
		cache:    make(map[string]float64),
		ttl:      ttl,
	}
}

// GetMarketCaps 获取市值数据（带缓存）
func (m *MarketCapManager) GetMarketCaps(ctx context.Context, symbols []string) (map[string]float64, error) {
	// 检查缓存是否过期
	if time.Since(m.lastUpdate) > m.ttl {
		log.Println("市值数据缓存过期，重新获取...")
		data, err := m.provider.GetMarketCaps(ctx, symbols)
		if err != nil {
			// 如果获取失败但有缓存数据，使用缓存
			if len(m.cache) > 0 {
				log.Printf("使用缓存的市值数据 (获取失败: %v)", err)
				return m.cache, nil
			}
			return nil, err
		}
		m.cache = data
		m.lastUpdate = time.Now()
		log.Printf("市值数据已更新，缓存 %d 个币种", len(data))
	}

	// 返回请求的币种数据
	result := make(map[string]float64)
	for _, symbol := range symbols {
		if marketCap, exists := m.cache[symbol]; exists {
			result[symbol] = marketCap
		}
	}

	return result, nil
}

// SortSymbolsByMarketCap 按市值排序币种符号
func SortSymbolsByMarketCap(symbols []string, marketCaps map[string]float64) []string {
	// 创建副本避免修改原始切片
	sorted := make([]string, len(symbols))
	copy(sorted, symbols)

	// 按市值降序排序
	sort.Slice(sorted, func(i, j int) bool {
		marketCapI := marketCaps[sorted[i]]
		marketCapJ := marketCaps[sorted[j]]
		return marketCapI > marketCapJ
	})

	return sorted
}

// GenerateCrossRatePairs 基于市值生成交叉汇率对
// 返回格式为 "SYMBOL1SYMBOL2" 的交易对，遵循交易所约定：
// 市值较高的币种作为报价货币（在后），市值较低的币种作为基础货币（在前）
// 例如：ETHBTC 表示用BTC买ETH，符合主流交易所约定
func GenerateCrossRatePairs(symbols []string, marketCaps map[string]float64, maxPairs int) []string {
	if len(symbols) < 2 {
		return []string{}
	}

	// 按市值排序
	sortedSymbols := SortSymbolsByMarketCap(symbols, marketCaps)

	pairs := make([]string, 0)

	// 生成符合交易所约定的交易对：市值低的在前（基础货币），市值高的在后（报价货币）
	for i := 0; i < len(sortedSymbols) && len(pairs) < maxPairs; i++ {
		for j := i + 1; j < len(sortedSymbols) && len(pairs) < maxPairs; j++ {
			pair := sortedSymbols[j] + sortedSymbols[i] // 低市值+高市值，如ETHBTC
			pairs = append(pairs, pair)
		}
	}

	return pairs
}
