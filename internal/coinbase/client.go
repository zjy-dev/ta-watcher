package coinbase

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Client Coinbase Pro API客户端
type Client struct {
	baseURL    string
	httpClient *http.Client
	config     *Config
}

// Config Coinbase配置
type Config struct {
	RateLimit struct {
		RequestsPerMinute int           `yaml:"requests_per_minute"`
		RetryDelay        time.Duration `yaml:"retry_delay"`
		MaxRetries        int           `yaml:"max_retries"`
	} `yaml:"rate_limit"`
}

// NewClient 创建新的Coinbase客户端
func NewClient(config *Config) *Client {
	return &Client{
		baseURL: "https://api.exchange.coinbase.com",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		config: config,
	}
}

// GetKlines 获取K线数据（兼容Binance接口）
func (c *Client) GetKlines(ctx context.Context, symbol string, interval string, limit int) ([]Kline, error) {
	// 转换币种格式：BTCUSDT -> BTC-USD
	coinbaseSymbol := convertSymbol(symbol)

	// 转换时间间隔
	granularity, err := convertInterval(interval)
	if err != nil {
		return nil, err
	}

	// 构建请求URL
	url := fmt.Sprintf("%s/products/%s/candles?granularity=%d", c.baseURL, coinbaseSymbol, granularity)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API请求失败，状态码: %d", resp.StatusCode)
	}

	var rawCandles [][]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawCandles); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 转换为Kline格式
	klines := make([]Kline, 0, len(rawCandles))
	for _, candle := range rawCandles {
		if len(candle) != 6 {
			continue
		}

		kline, err := parseCandle(candle)
		if err != nil {
			continue
		}

		klines = append(klines, kline)
	}

	// Coinbase返回的数据是倒序的，需要反转
	reverseKlines(klines)

	// 限制返回数量
	if limit > 0 && len(klines) > limit {
		klines = klines[len(klines)-limit:]
	}

	return klines, nil
}

// GetPrice 获取当前价格
func (c *Client) GetPrice(ctx context.Context, symbol string) (float64, error) {
	coinbaseSymbol := convertSymbol(symbol)

	url := fmt.Sprintf("%s/products/%s/ticker", c.baseURL, coinbaseSymbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("API请求失败，状态码: %d", resp.StatusCode)
	}

	var ticker struct {
		Price string `json:"price"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ticker); err != nil {
		return 0, fmt.Errorf("解析响应失败: %w", err)
	}

	price, err := strconv.ParseFloat(ticker.Price, 64)
	if err != nil {
		return 0, fmt.Errorf("价格转换失败: %w", err)
	}

	return price, nil
}

// convertSymbol 转换币种格式：BTCUSDT -> BTC-USD
func convertSymbol(symbol string) string {
	// 如果已经是正确的 Coinbase 格式（包含破折号），直接返回
	if len(symbol) > 3 && symbol[len(symbol)-4] == '-' {
		return symbol
	}

	// 简单的转换逻辑，可以根据需要扩展
	switch symbol {
	case "BTCUSDT", "BTCUSD":
		return "BTC-USD"
	case "ETHUSDT", "ETHUSD":
		return "ETH-USD"
	case "BNBUSDT":
		return "BNB-USD" // Coinbase可能不支持BNB
	case "ADAUSDT", "ADAUSD":
		return "ADA-USD"
	case "DOTUSDT":
		return "DOT-USD"
	case "LINKUSDT":
		return "LINK-USD"
	default:
		// 通用转换：移除USDT或USD后缀，添加-USD
		if len(symbol) >= 4 && symbol[len(symbol)-4:] == "USDT" {
			return symbol[:len(symbol)-4] + "-USD"
		}
		if len(symbol) >= 3 && symbol[len(symbol)-3:] == "USD" {
			return symbol[:len(symbol)-3] + "-USD"
		}
		return symbol
	}
}

// convertInterval 转换时间间隔
func convertInterval(interval string) (int, error) {
	switch interval {
	case "1m":
		return 60, nil
	case "5m":
		return 300, nil
	case "15m":
		return 900, nil
	case "1h":
		return 3600, nil
	case "4h":
		return 14400, nil
	case "6h":
		return 21600, nil
	case "1d":
		return 86400, nil
	default:
		return 0, fmt.Errorf("不支持的时间间隔: %s", interval)
	}
}

// parseCandle 解析蜡烛图数据
func parseCandle(candle []interface{}) (Kline, error) {
	if len(candle) != 6 {
		return Kline{}, fmt.Errorf("无效的蜡烛图数据")
	}

	// Coinbase格式: [timestamp, low, high, open, close, volume]
	timestamp, ok := candle[0].(float64)
	if !ok {
		return Kline{}, fmt.Errorf("时间戳解析失败")
	}

	low, _ := parseFloat(candle[1])
	high, _ := parseFloat(candle[2])
	open, _ := parseFloat(candle[3])
	close, _ := parseFloat(candle[4])
	volume, _ := parseFloat(candle[5])

	return Kline{
		OpenTime:  int64(timestamp) * 1000, // 转换为毫秒
		Open:      fmt.Sprintf("%.8f", open),
		High:      fmt.Sprintf("%.8f", high),
		Low:       fmt.Sprintf("%.8f", low),
		Close:     fmt.Sprintf("%.8f", close),
		Volume:    fmt.Sprintf("%.8f", volume),
		CloseTime: int64(timestamp)*1000 + 59999, // 估算关闭时间
	}, nil
}

// GetProducts 获取产品列表
func (c *Client) GetProducts(ctx context.Context) ([]Product, error) {
	url := fmt.Sprintf("%s/products", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API请求失败，状态码: %d", resp.StatusCode)
	}

	var products []Product
	if err := json.NewDecoder(resp.Body).Decode(&products); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return products, nil
}

// GetTicker 获取价格行情 (使用24hr stats替代)
func (c *Client) GetTicker(ctx context.Context, symbol string) (*Ticker, error) {
	// 转换币种格式
	coinbaseSymbol := convertSymbol(symbol)

	// 使用stats端点替代ticker
	url := fmt.Sprintf("%s/products/%s/stats", c.baseURL, coinbaseSymbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API请求失败，状态码: %d", resp.StatusCode)
	}

	// 解析stats响应，转换为ticker格式
	var stats map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 构造ticker响应
	ticker := &Ticker{
		TradeID: 0, // stats中没有trade_id
		Price:   getString(stats, "last"),
		Size:    "0", // stats中没有size
		Time:    "",  // stats中没有time
		Bid:     "",  // stats中没有bid
		Ask:     "",  // stats中没有ask
		Volume:  getString(stats, "volume"),
	}

	return ticker, nil
}

// getString 从map中安全获取字符串值
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
		// 如果是数字，转换为字符串
		return fmt.Sprintf("%v", val)
	}
	return ""
}

// parseFloat 安全解析浮点数
func parseFloat(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		return 0, fmt.Errorf("无法解析浮点数")
	}
}

// reverseKlines 反转K线数组
func reverseKlines(klines []Kline) {
	for i, j := 0, len(klines)-1; i < j; i, j = i+1, j-1 {
		klines[i], klines[j] = klines[j], klines[i]
	}
}
