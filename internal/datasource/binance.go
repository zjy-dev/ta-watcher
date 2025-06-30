package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

// BinanceClient Binance数据源实现
type BinanceClient struct {
	baseURL string
	client  *http.Client
}

// NewBinanceClient 创建Binance客户端
func NewBinanceClient() *BinanceClient {
	log.Printf("🔗 初始化 Binance 数据源")
	return &BinanceClient{
		baseURL: "https://api.binance.com",
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Name 返回数据源名称
func (b *BinanceClient) Name() string {
	return "binance"
}

// IsSymbolValid 检查交易对是否有效
func (b *BinanceClient) IsSymbolValid(ctx context.Context, symbol string) (bool, error) {
	url := fmt.Sprintf("%s/api/v3/ticker/price?symbol=%s", b.baseURL, symbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	valid := resp.StatusCode == http.StatusOK
	if !valid {
		log.Printf("❌ [Binance] %s 不存在", symbol)
	}

	return valid, nil
}

// GetKlines 获取K线数据
func (b *BinanceClient) GetKlines(ctx context.Context, symbol string, timeframe Timeframe, startTime, endTime time.Time, limit int) ([]*Kline, error) {
	if limit <= 0 {
		limit = 500
	}
	if limit > 1000 {
		limit = 1000 // Binance API限制
	}

	url := fmt.Sprintf("%s/api/v3/klines", b.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("symbol", symbol)
	q.Add("interval", string(timeframe))
	q.Add("limit", strconv.Itoa(limit))

	if !startTime.IsZero() {
		q.Add("startTime", strconv.FormatInt(startTime.UnixMilli(), 10))
	}
	if !endTime.IsZero() {
		q.Add("endTime", strconv.FormatInt(endTime.UnixMilli(), 10))
	}

	req.URL.RawQuery = q.Encode()
	// log.Printf("🌐 [Binance] 请求URL: %s", req.URL.String())

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binance API returned status: %d", resp.StatusCode)
	}

	var rawKlines [][]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawKlines); err != nil {
		return nil, err
	}

	klines := make([]*Kline, len(rawKlines))
	for i, raw := range rawKlines {
		kline, err := b.parseKline(symbol, raw)
		if err != nil {
			return nil, fmt.Errorf("failed to parse kline %d: %w", i, err)
		}
		klines[i] = kline
	}

	return klines, nil
}

// parseKline 解析Binance K线数据
func (b *BinanceClient) parseKline(symbol string, raw []interface{}) (*Kline, error) {
	if len(raw) < 11 {
		return nil, fmt.Errorf("invalid kline data length: %d", len(raw))
	}

	openTime, err := parseFloat64(raw[0])
	if err != nil {
		return nil, fmt.Errorf("invalid open time: %w", err)
	}

	closeTime, err := parseFloat64(raw[6])
	if err != nil {
		return nil, fmt.Errorf("invalid close time: %w", err)
	}

	open, err := parseFloat64FromString(raw[1])
	if err != nil {
		return nil, fmt.Errorf("invalid open price: %w", err)
	}

	high, err := parseFloat64FromString(raw[2])
	if err != nil {
		return nil, fmt.Errorf("invalid high price: %w", err)
	}

	low, err := parseFloat64FromString(raw[3])
	if err != nil {
		return nil, fmt.Errorf("invalid low price: %w", err)
	}

	close, err := parseFloat64FromString(raw[4])
	if err != nil {
		return nil, fmt.Errorf("invalid close price: %w", err)
	}

	volume, err := parseFloat64FromString(raw[5])
	if err != nil {
		return nil, fmt.Errorf("invalid volume: %w", err)
	}

	return &Kline{
		Symbol:    symbol,
		OpenTime:  time.UnixMilli(int64(openTime)),
		CloseTime: time.UnixMilli(int64(closeTime)),
		Open:      open,
		High:      high,
		Low:       low,
		Close:     close,
		Volume:    volume,
	}, nil
}

// parseFloat64 从interface{}解析float64
func parseFloat64(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		return 0, fmt.Errorf("cannot parse %T as float64", v)
	}
}

// parseFloat64FromString 从字符串解析float64
func parseFloat64FromString(v interface{}) (float64, error) {
	str, ok := v.(string)
	if !ok {
		return 0, fmt.Errorf("expected string, got %T", v)
	}
	return strconv.ParseFloat(str, 64)
}
