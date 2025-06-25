package coinbase

import "fmt"

// Kline K线数据结构（兼容Binance格式）
type Kline struct {
	OpenTime  int64  `json:"openTime"`  // 开盘时间
	Open      string `json:"open"`      // 开盘价
	High      string `json:"high"`      // 最高价
	Low       string `json:"low"`       // 最低价
	Close     string `json:"close"`     // 收盘价
	Volume    string `json:"volume"`    // 成交量
	CloseTime int64  `json:"closeTime"` // 收盘时间
}

// Product Coinbase产品信息
type Product struct {
	ID              string `json:"id"`
	DisplayName     string `json:"display_name"`
	BaseCurrency    string `json:"base_currency"`
	QuoteCurrency   string `json:"quote_currency"`
	Status          string `json:"status"`
	TradingDisabled bool   `json:"trading_disabled"`
}

// Ticker 价格行情
type Ticker struct {
	TradeID int    `json:"trade_id"`
	Price   string `json:"price"`
	Size    string `json:"size"`
	Time    string `json:"time"`
	Bid     string `json:"bid"`
	Ask     string `json:"ask"`
	Volume  string `json:"volume"`
}

// APIError API错误响应
type APIError struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

func (e APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("Coinbase API错误 [%s]: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("Coinbase API错误: %s", e.Message)
}
