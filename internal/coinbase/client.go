package coinbase

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
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

	// 对于周线和月线，我们需要获取更多的日线数据进行聚合
	actualLimit := limit
	needsAggregation := false

	switch interval {
	case "1w":
		// 对于周线，获取大约7倍的日线数据
		actualLimit = limit * 7
		needsAggregation = true
	case "1M":
		// 对于月线，获取大约30倍的日线数据
		actualLimit = limit * 30
		needsAggregation = true
	}

	// 构建请求URL (使用actualLimit来获取足够的数据)
	url := fmt.Sprintf("%s/products/%s/candles?granularity=%d", c.baseURL, coinbaseSymbol, granularity)
	if actualLimit > 0 {
		// Coinbase API的limit参数（如果支持的话）
		// 注意：Coinbase API可能不支持limit参数，这种情况下我们在后面手动限制
	}

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

	// 对于周线和月线，进行数据聚合
	if needsAggregation {
		klines = c.aggregateKlines(klines, interval)
	}

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

// convertSymbol 转换币种格式：BTCUSDT -> BTC-USD, ADABTC -> ADA-BTC
func convertSymbol(symbol string) string {
	// 如果已经是正确的 Coinbase 格式（包含破折号），直接返回
	if len(symbol) > 3 && (symbol[len(symbol)-4] == '-' || symbol[3] == '-' || symbol[4] == '-') {
		return symbol
	}

	// 处理法币对 (USD/USDT/USDC/EUR/GBP)
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
	case "SOLUSDT", "SOLUSD":
		return "SOL-USD"
	case "MATICUSDT":
		return "MATIC-USD"
	case "AVAXUSDT":
		return "AVAX-USD"
	}

	// 处理交叉货币对 (crypto-to-crypto pairs)
	// 常见的基础货币：BTC, ETH, USD, USDT, USDC
	knownQuotes := []string{"BTC", "ETH", "USD", "USDT", "USDC", "EUR", "GBP"}

	for _, quote := range knownQuotes {
		if len(symbol) > len(quote) && symbol[len(symbol)-len(quote):] == quote {
			base := symbol[:len(symbol)-len(quote)]
			// 对于USD相关的，统一转换为USD
			if quote == "USDT" || quote == "USDC" {
				return base + "-USD"
			}
			return base + "-" + quote
		}
	}

	// 通用转换：如果没有匹配，尝试通用格式
	if len(symbol) >= 4 && symbol[len(symbol)-4:] == "USDT" {
		return symbol[:len(symbol)-4] + "-USD"
	}
	if len(symbol) >= 3 && symbol[len(symbol)-3:] == "USD" {
		return symbol[:len(symbol)-3] + "-USD"
	}

	// 如果都不匹配，返回原始值（可能需要人工检查）
	return symbol
}

// convertInterval 转换时间间隔 - 将Binance格式的interval转换为Coinbase支持的granularity
func convertInterval(interval string) (int, error) {
	switch interval {
	// 分钟级别
	case "1m":
		return 60, nil // 1分钟
	case "3m":
		return 300, nil // 5分钟 (Coinbase最接近的)
	case "5m":
		return 300, nil // 5分钟
	case "15m":
		return 900, nil // 15分钟
	case "30m":
		return 1800, nil // 30分钟 (如果Coinbase不支持，使用15分钟)
	// 小时级别
	case "1h":
		return 3600, nil // 1小时
	case "2h":
		return 7200, nil // 2小时 (如果Coinbase不支持，使用1小时)
	case "4h":
		return 14400, nil // 4小时 (如果Coinbase不支持，使用6小时)
	case "6h":
		return 21600, nil // 6小时
	case "8h":
		return 28800, nil // 8小时 (如果Coinbase不支持，使用6小时)
	case "12h":
		return 43200, nil // 12小时 (如果Coinbase不支持，使用6小时)
	// 日/周/月级别
	case "1d":
		return 86400, nil // 1天
	case "3d":
		return 86400, nil // 3天 (Coinbase不支持，使用1天，但需要在应用层处理采样)
	case "1w":
		return 86400, nil // 1周 (Coinbase不支持weekly，使用daily并在应用层采样)
	case "1M":
		return 86400, nil // 1月 (Coinbase不支持monthly，使用daily并在应用层采样)
	default:
		return 0, fmt.Errorf("不支持的时间间隔: %s，支持的间隔: 1m,3m,5m,15m,30m,1h,2h,4h,6h,8h,12h,1d,3d,1w,1M", interval)
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

// aggregateKlines 聚合K线数据，将日线数据转换为周线或月线
func (c *Client) aggregateKlines(dailyKlines []Kline, interval string) []Kline {
	if len(dailyKlines) == 0 {
		return dailyKlines
	}

	var aggregatedKlines []Kline

	switch interval {
	case "1w":
		// 按周聚合
		aggregatedKlines = c.aggregateByWeek(dailyKlines)
	case "1M":
		// 按月聚合
		aggregatedKlines = c.aggregateByMonth(dailyKlines)
	default:
		return dailyKlines
	}

	return aggregatedKlines
}

// aggregateByWeek 按周聚合日线数据
func (c *Client) aggregateByWeek(dailyKlines []Kline) []Kline {
	if len(dailyKlines) == 0 {
		return dailyKlines
	}

	var weeklyKlines []Kline
	var currentWeekStart time.Time
	var weekData []Kline

	for _, kline := range dailyKlines {
		klineTime := time.Unix(kline.OpenTime/1000, 0)

		// 获取当前K线所在周的开始时间（周一）
		year, week := klineTime.ISOWeek()
		weekStart := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		weekStart = weekStart.AddDate(0, 0, (week-1)*7-int(weekStart.Weekday())+1)

		// 如果是新的一周，处理上一周的数据
		if !currentWeekStart.IsZero() && !weekStart.Equal(currentWeekStart) {
			if len(weekData) > 0 {
				weeklyKline := c.aggregateKlineData(weekData, currentWeekStart.Unix()*1000)
				weeklyKlines = append(weeklyKlines, weeklyKline)
			}
			weekData = []Kline{}
		}

		currentWeekStart = weekStart
		weekData = append(weekData, kline)
	}

	// 处理最后一周的数据
	if len(weekData) > 0 {
		weeklyKline := c.aggregateKlineData(weekData, currentWeekStart.Unix()*1000)
		weeklyKlines = append(weeklyKlines, weeklyKline)
	}

	return weeklyKlines
}

// aggregateByMonth 按月聚合日线数据
func (c *Client) aggregateByMonth(dailyKlines []Kline) []Kline {
	if len(dailyKlines) == 0 {
		return dailyKlines
	}

	var monthlyKlines []Kline
	var currentMonth time.Time
	var monthData []Kline

	for _, kline := range dailyKlines {
		klineTime := time.Unix(kline.OpenTime/1000, 0)

		// 获取当前K线所在月的开始时间
		monthStart := time.Date(klineTime.Year(), klineTime.Month(), 1, 0, 0, 0, 0, time.UTC)

		// 如果是新的一月，处理上一月的数据
		if !currentMonth.IsZero() && !monthStart.Equal(currentMonth) {
			if len(monthData) > 0 {
				monthlyKline := c.aggregateKlineData(monthData, currentMonth.Unix()*1000)
				monthlyKlines = append(monthlyKlines, monthlyKline)
			}
			monthData = []Kline{}
		}

		currentMonth = monthStart
		monthData = append(monthData, kline)
	}

	// 处理最后一月的数据
	if len(monthData) > 0 {
		monthlyKline := c.aggregateKlineData(monthData, currentMonth.Unix()*1000)
		monthlyKlines = append(monthlyKlines, monthlyKline)
	}

	return monthlyKlines
}

// aggregateKlineData 聚合K线数据（开盘取第一个，收盘取最后一个，最高最低取极值，成交量累加）
func (c *Client) aggregateKlineData(klines []Kline, openTime int64) Kline {
	if len(klines) == 0 {
		return Kline{}
	}

	// 排序确保时间顺序正确
	sort.Slice(klines, func(i, j int) bool {
		return klines[i].OpenTime < klines[j].OpenTime
	})

	open, _ := strconv.ParseFloat(klines[0].Open, 64)
	close, _ := strconv.ParseFloat(klines[len(klines)-1].Close, 64)

	high := 0.0
	low := math.MaxFloat64
	volume := 0.0

	for _, kline := range klines {
		h, _ := strconv.ParseFloat(kline.High, 64)
		l, _ := strconv.ParseFloat(kline.Low, 64)
		v, _ := strconv.ParseFloat(kline.Volume, 64)

		if h > high {
			high = h
		}
		if l < low {
			low = l
		}
		volume += v
	}

	return Kline{
		OpenTime:  openTime,
		Open:      fmt.Sprintf("%.8f", open),
		High:      fmt.Sprintf("%.8f", high),
		Low:       fmt.Sprintf("%.8f", low),
		Close:     fmt.Sprintf("%.8f", close),
		Volume:    fmt.Sprintf("%.8f", volume),
		CloseTime: klines[len(klines)-1].CloseTime,
	}
}
