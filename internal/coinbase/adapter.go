package coinbase

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"ta-watcher/internal/binance"
)

// BinanceAdapter 将 Coinbase 客户端适配为 binance.DataSource 接口
type BinanceAdapter struct {
	client *Client
}

// NewBinanceAdapter 创建 Coinbase 到 Binance 接口的适配器
func NewBinanceAdapter(client *Client) *BinanceAdapter {
	return &BinanceAdapter{
		client: client,
	}
}

// 辅助函数：字符串转float64
func parseFloat64(s string) float64 {
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

// 辅助函数：Unix时间戳转time.Time
func unixToTime(unix int64) time.Time {
	return time.Unix(unix, 0)
}

// GetPrice 获取单个交易对的当前价格
func (a *BinanceAdapter) GetPrice(ctx context.Context, symbol string) (*binance.PriceData, error) {
	ticker, err := a.client.GetTicker(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("获取价格失败: %w", err)
	}

	price := parseFloat64(ticker.Price)

	return &binance.PriceData{
		Symbol:    symbol,
		Price:     price,
		Timestamp: time.Now(),
	}, nil
}

// GetPrices 获取多个交易对的当前价格
func (a *BinanceAdapter) GetPrices(ctx context.Context, symbols []string) ([]*binance.PriceData, error) {
	var prices []*binance.PriceData

	for _, symbol := range symbols {
		price, err := a.GetPrice(ctx, symbol)
		if err != nil {
			return nil, fmt.Errorf("获取 %s 价格失败: %w", symbol, err)
		}
		prices = append(prices, price)
	}

	return prices, nil
}

// GetKlines 获取K线数据
func (a *BinanceAdapter) GetKlines(ctx context.Context, symbol, interval string, limit int) ([]*binance.KlineData, error) {
	coinbaseKlines, err := a.client.GetKlines(ctx, symbol, interval, limit)
	if err != nil {
		return nil, fmt.Errorf("获取K线数据失败: %w", err)
	}

	var binanceKlines []*binance.KlineData
	for _, k := range coinbaseKlines {
		binanceKlines = append(binanceKlines, &binance.KlineData{
			Symbol:              symbol,
			Interval:            interval,
			OpenTime:            unixToTime(k.OpenTime),
			CloseTime:           unixToTime(k.CloseTime),
			Open:                parseFloat64(k.Open),
			High:                parseFloat64(k.High),
			Low:                 parseFloat64(k.Low),
			Close:               parseFloat64(k.Close),
			Volume:              parseFloat64(k.Volume),
			TradeCount:          0, // Coinbase 不提供交易次数
			TakerBuyBaseVolume:  0, // Coinbase 不提供此数据
			TakerBuyQuoteVolume: 0, // Coinbase 不提供此数据
		})
	}

	return binanceKlines, nil
}

// GetKlinesWithTimeRange 获取指定时间范围的K线数据
func (a *BinanceAdapter) GetKlinesWithTimeRange(ctx context.Context, symbol, interval string, startTime, endTime time.Time) ([]*binance.KlineData, error) {
	// Coinbase 当前实现不支持时间范围查询，使用默认limit
	return a.GetKlines(ctx, symbol, interval, 1000)
}

// GetTicker24hr 获取24小时价格变动统计
func (a *BinanceAdapter) GetTicker24hr(ctx context.Context, symbol string) (*binance.TickerData, error) {
	ticker, err := a.client.GetTicker(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("获取ticker失败: %w", err)
	}

	// 解析所有价格字段
	price := parseFloat64(ticker.Price)
	volume := parseFloat64(ticker.Volume)
	lastQty := parseFloat64(ticker.Size)
	bidPrice := parseFloat64(ticker.Bid)
	askPrice := parseFloat64(ticker.Ask)

	return &binance.TickerData{
		Symbol:             symbol,
		PriceChange:        0, // Coinbase stats中没有直接的价格变化
		PriceChangePercent: 0, // Coinbase stats中没有直接的百分比变化
		WeightedAvgPrice:   price,
		PrevClosePrice:     price, // 近似值
		LastPrice:          price,
		LastQty:            lastQty,
		BidPrice:           bidPrice,
		BidQty:             0, // Coinbase 不提供bid数量
		AskPrice:           askPrice,
		AskQty:             0,     // Coinbase 不提供ask数量
		OpenPrice:          price, // 近似值
		HighPrice:          price, // 近似值
		LowPrice:           price, // 近似值
		Volume:             volume,
		QuoteVolume:        price * volume,
		OpenTime:           time.Now().Add(-24 * time.Hour), // 24小时前
		CloseTime:          time.Now(),
		Count:              0,
	}, nil
}

// GetAllTickers24hr 获取所有交易对的24小时价格变动统计
func (a *BinanceAdapter) GetAllTickers24hr(ctx context.Context) ([]*binance.TickerData, error) {
	// 获取所有产品
	products, err := a.client.GetProducts(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取产品列表失败: %w", err)
	}

	var tickers []*binance.TickerData

	// 只处理前50个产品以避免过多的API调用
	limit := 50
	if len(products) < limit {
		limit = len(products)
	}

	for i, product := range products[:limit] {
		if i >= limit {
			break
		}

		// 只处理USD交易对
		if len(product.ID) > 4 && product.ID[len(product.ID)-4:] == "-USD" {
			ticker, err := a.GetTicker24hr(ctx, product.ID)
			if err != nil {
				continue // 跳过错误的交易对
			}
			tickers = append(tickers, ticker)
		}
	}

	return tickers, nil
}

// GetExchangeInfo 获取交易所信息
func (a *BinanceAdapter) GetExchangeInfo(ctx context.Context) (*binance.ExchangeInfo, error) {
	products, err := a.client.GetProducts(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取交易所信息失败: %w", err)
	}

	var symbols []*binance.SymbolInfo
	for _, product := range products {
		if product.Status == "online" {
			symbols = append(symbols, &binance.SymbolInfo{
				Symbol:              product.ID,
				Status:              "TRADING",
				BaseAsset:           product.BaseCurrency,
				BaseAssetPrecision:  8,
				QuoteAsset:          product.QuoteCurrency,
				QuoteAssetPrecision: 8,
			})
		}
	}

	// 转换为值切片
	symbolsSlice := make([]binance.SymbolInfo, len(symbols))
	for i, s := range symbols {
		symbolsSlice[i] = *s
	}

	return &binance.ExchangeInfo{
		Timezone:   "UTC",
		ServerTime: time.Now(),
		Symbols:    symbolsSlice,
	}, nil
}

// Ping 检查连接状态
func (a *BinanceAdapter) Ping(ctx context.Context) error {
	// 通过获取产品列表来检查连接
	_, err := a.client.GetProducts(ctx)
	if err != nil {
		return fmt.Errorf("Coinbase连接检查失败: %w", err)
	}
	return nil
}

// GetServerTime 获取服务器时间
func (a *BinanceAdapter) GetServerTime(ctx context.Context) (time.Time, error) {
	// Coinbase没有专门的服务器时间端点，返回当前时间
	return time.Now(), nil
}
