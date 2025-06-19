package binance

import (
	"context"
	"errors"
	"time"
)

// PriceData 价格数据结构
type PriceData struct {
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	Timestamp time.Time `json:"timestamp"`
}

// KlineData K线数据结构
type KlineData struct {
	Symbol              string    `json:"symbol"`
	Interval            string    `json:"interval"`
	OpenTime            time.Time `json:"open_time"`
	CloseTime           time.Time `json:"close_time"`
	Open                float64   `json:"open"`
	High                float64   `json:"high"`
	Low                 float64   `json:"low"`
	Close               float64   `json:"close"`
	Volume              float64   `json:"volume"`
	TradeCount          int64     `json:"trade_count"`
	TakerBuyBaseVolume  float64   `json:"taker_buy_base_volume"`
	TakerBuyQuoteVolume float64   `json:"taker_buy_quote_volume"`
}

// TickerData 24小时价格变动数据
type TickerData struct {
	Symbol             string    `json:"symbol"`
	PriceChange        float64   `json:"price_change"`
	PriceChangePercent float64   `json:"price_change_percent"`
	WeightedAvgPrice   float64   `json:"weighted_avg_price"`
	PrevClosePrice     float64   `json:"prev_close_price"`
	LastPrice          float64   `json:"last_price"`
	LastQty            float64   `json:"last_qty"`
	BidPrice           float64   `json:"bid_price"`
	BidQty             float64   `json:"bid_qty"`
	AskPrice           float64   `json:"ask_price"`
	AskQty             float64   `json:"ask_qty"`
	OpenPrice          float64   `json:"open_price"`
	HighPrice          float64   `json:"high_price"`
	LowPrice           float64   `json:"low_price"`
	Volume             float64   `json:"volume"`
	QuoteVolume        float64   `json:"quote_volume"`
	OpenTime           time.Time `json:"open_time"`
	CloseTime          time.Time `json:"close_time"`
	Count              int64     `json:"count"`
}

// ExchangeInfo 交易所信息
type ExchangeInfo struct {
	Timezone   string       `json:"timezone"`
	ServerTime time.Time    `json:"server_time"`
	Symbols    []SymbolInfo `json:"symbols"`
}

// SymbolInfo 交易对信息
type SymbolInfo struct {
	Symbol                     string                   `json:"symbol"`
	Status                     string                   `json:"status"`
	BaseAsset                  string                   `json:"base_asset"`
	BaseAssetPrecision         int                      `json:"base_asset_precision"`
	QuoteAsset                 string                   `json:"quote_asset"`
	QuoteAssetPrecision        int                      `json:"quote_asset_precision"`
	OrderTypes                 []string                 `json:"order_types"`
	IcebergAllowed             bool                     `json:"iceberg_allowed"`
	OcoAllowed                 bool                     `json:"oco_allowed"`
	QuoteOrderQtyMarketAllowed bool                     `json:"quote_order_qty_market_allowed"`
	AllowTrailingStop          bool                     `json:"allow_trailing_stop"`
	CancelReplaceAllowed       bool                     `json:"cancel_replace_allowed"`
	IsSpotTradingAllowed       bool                     `json:"is_spot_trading_allowed"`
	IsMarginTradingAllowed     bool                     `json:"is_margin_trading_allowed"`
	Filters                    []map[string]interface{} `json:"filters"`
	Permissions                []string                 `json:"permissions"`
}

// DataSource 数据源接口定义
type DataSource interface {
	// GetPrice 获取单个交易对的当前价格
	GetPrice(ctx context.Context, symbol string) (*PriceData, error)

	// GetPrices 获取多个交易对的当前价格
	GetPrices(ctx context.Context, symbols []string) ([]*PriceData, error)

	// GetKlines 获取K线数据
	GetKlines(ctx context.Context, symbol, interval string, limit int) ([]*KlineData, error)

	// GetKlinesWithTimeRange 获取指定时间范围的K线数据
	GetKlinesWithTimeRange(ctx context.Context, symbol, interval string, startTime, endTime time.Time) ([]*KlineData, error)

	// GetTicker24hr 获取24小时价格变动统计
	GetTicker24hr(ctx context.Context, symbol string) (*TickerData, error)

	// GetAllTickers24hr 获取所有交易对的24小时价格变动统计
	GetAllTickers24hr(ctx context.Context) ([]*TickerData, error)

	// GetExchangeInfo 获取交易所信息
	GetExchangeInfo(ctx context.Context) (*ExchangeInfo, error)

	// Ping 检查连接状态
	Ping(ctx context.Context) error

	// GetServerTime 获取服务器时间
	GetServerTime(ctx context.Context) (time.Time, error)
}

// 支持的K线间隔
const (
	Interval1m  = "1m"
	Interval3m  = "3m"
	Interval5m  = "5m"
	Interval15m = "15m"
	Interval30m = "30m"
	Interval1h  = "1h"
	Interval2h  = "2h"
	Interval4h  = "4h"
	Interval6h  = "6h"
	Interval8h  = "8h"
	Interval12h = "12h"
	Interval1d  = "1d"
	Interval3d  = "3d"
	Interval1w  = "1w"
	Interval1M  = "1M"
)

// 常见错误定义
var (
	ErrInvalidSymbol     = errors.New("invalid symbol")
	ErrInvalidInterval   = errors.New("invalid interval")
	ErrInvalidLimit      = errors.New("invalid limit")
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	ErrServerError       = errors.New("server error")
	ErrNetworkError      = errors.New("network error")
	ErrInvalidTimeRange  = errors.New("invalid time range")
)

// IsValidInterval 检查K线间隔是否有效
func IsValidInterval(interval string) bool {
	validIntervals := []string{
		Interval1m, Interval3m, Interval5m, Interval15m, Interval30m,
		Interval1h, Interval2h, Interval4h, Interval6h, Interval8h, Interval12h,
		Interval1d, Interval3d, Interval1w, Interval1M,
	}

	for _, valid := range validIntervals {
		if interval == valid {
			return true
		}
	}
	return false
}

// IsValidSymbol 基本的交易对格式验证
func IsValidSymbol(symbol string) bool {
	if len(symbol) < 6 || len(symbol) > 20 {
		return false
	}

	// 简单检查是否包含常见的报价货币
	commonQuotes := []string{"USDT", "BTC", "ETH", "BNB", "USDC", "BUSD"}
	for _, quote := range commonQuotes {
		if len(symbol) > len(quote) && symbol[len(symbol)-len(quote):] == quote {
			return true
		}
	}

	return false
}

// ValidateKlineParams 验证K线参数
func ValidateKlineParams(symbol, interval string, limit int) error {
	if !IsValidSymbol(symbol) {
		return ErrInvalidSymbol
	}

	if !IsValidInterval(interval) {
		return ErrInvalidInterval
	}

	if limit <= 0 || limit > 5000 {
		return ErrInvalidLimit
	}

	return nil
}
