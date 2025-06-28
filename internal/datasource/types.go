package datasource

import (
	"context"
	"time"
)

// Timeframe 时间框架
type Timeframe string

const (
	Timeframe1m  Timeframe = "1m"
	Timeframe3m  Timeframe = "3m"
	Timeframe5m  Timeframe = "5m"
	Timeframe15m Timeframe = "15m"
	Timeframe30m Timeframe = "30m"
	Timeframe1h  Timeframe = "1h"
	Timeframe2h  Timeframe = "2h"
	Timeframe4h  Timeframe = "4h"
	Timeframe6h  Timeframe = "6h"
	Timeframe8h  Timeframe = "8h"
	Timeframe12h Timeframe = "12h"
	Timeframe1d  Timeframe = "1d"
	Timeframe3d  Timeframe = "3d"
	Timeframe1w  Timeframe = "1w"
	Timeframe1M  Timeframe = "1M"
)

// Kline K线数据
type Kline struct {
	Symbol    string    `json:"symbol"`
	OpenTime  time.Time `json:"open_time"`
	CloseTime time.Time `json:"close_time"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
}

// DataSource 数据源接口
type DataSource interface {
	// GetKlines 获取K线数据
	// symbol: 交易对符号，例如 "BTCUSDT"
	// timeframe: 时间框架
	// startTime: 开始时间
	// endTime: 结束时间
	// limit: 最大返回数量（可选，默认500）
	GetKlines(ctx context.Context, symbol string, timeframe Timeframe, startTime, endTime time.Time, limit int) ([]*Kline, error)

	// IsSymbolValid 检查交易对是否有效
	IsSymbolValid(ctx context.Context, symbol string) (bool, error)

	// Name 返回数据源名称
	Name() string
}
