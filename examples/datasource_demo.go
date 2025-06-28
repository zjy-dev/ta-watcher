package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"ta-watcher/internal/config"
	"ta-watcher/internal/datasource"
	"ta-watcher/internal/strategy"
)

func main() {
	// 创建基础配置
	cfg := config.DefaultConfig()

	// 创建数据源工厂
	factory := datasource.NewFactory()

	// 创建Binance数据源
	binanceDS, err := factory.CreateDataSource("binance", cfg)
	if err != nil {
		log.Fatalf("Failed to create Binance data source: %v", err)
	}

	fmt.Printf("Created data source: %s\n", binanceDS.Name())

	// 测试获取K线数据
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	symbol := "BTCUSDT"
	timeframe := datasource.Timeframe1h
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour) // 最近24小时

	klines, err := binanceDS.GetKlines(ctx, symbol, timeframe, startTime, endTime, 24)
	if err != nil {
		log.Fatalf("Failed to get klines: %v", err)
	}

	fmt.Printf("Received %d klines for %s %s\n", len(klines), symbol, timeframe)
	if len(klines) > 0 {
		latest := klines[len(klines)-1]
		fmt.Printf("Latest kline: Open=%.2f High=%.2f Low=%.2f Close=%.2f Volume=%.2f\n",
			latest.Open, latest.High, latest.Low, latest.Close, latest.Volume)
	}

	// 创建市场数据结构用于策略评估
	marketData := &strategy.MarketData{
		Symbol:    symbol,
		Timeframe: timeframe,
		Klines:    klines,
		Timestamp: time.Now(),
	}

	// 创建RSI策略
	rsiStrategy := strategy.NewRSIStrategy(14, 70, 30)
	fmt.Printf("Created strategy: %s\n", rsiStrategy.Name())
	fmt.Printf("Description: %s\n", rsiStrategy.Description())
	fmt.Printf("Required data points: %d\n", rsiStrategy.RequiredDataPoints())

	// 检查是否有足够的数据点
	if len(klines) >= rsiStrategy.RequiredDataPoints() {
		result, err := rsiStrategy.Evaluate(marketData)
		if err != nil {
			log.Printf("Strategy evaluation error: %v", err)
		} else if result != nil {
			fmt.Printf("Strategy result: Signal=%s Strength=%s Confidence=%.2f Price=%.2f\n",
				result.Signal.String(), result.Strength.String(), result.Confidence, result.Price)
		} else {
			fmt.Println("No signal generated")
		}
	} else {
		fmt.Printf("Insufficient data points. Required: %d, Available: %d\n",
			rsiStrategy.RequiredDataPoints(), len(klines))
	}

	// 测试Coinbase数据源
	fmt.Println("\n--- Testing Coinbase Data Source ---")
	coinbaseDS, err := factory.CreateDataSource("coinbase", cfg)
	if err != nil {
		log.Fatalf("Failed to create Coinbase data source: %v", err)
	}

	fmt.Printf("Created data source: %s\n", coinbaseDS.Name())

	// 注意：Coinbase使用不同的交易对格式
	coinbaseSymbol := "BTC-USD"
	klines2, err := coinbaseDS.GetKlines(ctx, coinbaseSymbol, timeframe, startTime, endTime, 24)
	if err != nil {
		log.Printf("Failed to get Coinbase klines: %v", err)
	} else {
		fmt.Printf("Received %d klines from Coinbase for %s %s\n", len(klines2), coinbaseSymbol, timeframe)
		if len(klines2) > 0 {
			latest := klines2[len(klines2)-1]
			fmt.Printf("Latest Coinbase kline: Open=%.2f High=%.2f Low=%.2f Close=%.2f Volume=%.2f\n",
				latest.Open, latest.High, latest.Low, latest.Close, latest.Volume)
		}
	}
}
