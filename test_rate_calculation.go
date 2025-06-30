package main

import (
	"context"
	"log"
	"ta-watcher/internal/assets"
	"ta-watcher/internal/config"
	"ta-watcher/internal/datasource"
	"time"
)

func main() {
	// 测试实际的汇率计算逻辑
	log.Println("🧪 测试汇率计算逻辑...")

	// 加载配置
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 创建数据源
	factory := datasource.NewFactory()
	dataSource, err := factory.CreateDataSource(cfg.DataSource.Primary, cfg)
	if err != nil {
		log.Fatalf("创建数据源失败: %v", err)
	}

	// 创建汇率计算器
	calculator := assets.NewRateCalculator(dataSource)
	ctx := context.Background()

	// 测试不同时间框架
	testCases := []struct {
		name      string
		timeframe datasource.Timeframe
		limit     int
	}{
		{"日线", datasource.Timeframe1d, 30},
		{"周线", datasource.Timeframe1w, 15},
		{"月线", datasource.Timeframe1M, 10},
	}

	for _, tc := range testCases {
		log.Printf("\n📊 测试 %s 汇率计算...", tc.name)

		// 根据不同时间框架设置不同的时间范围
		now := time.Now()
		var startTime time.Time
		switch tc.timeframe {
		case datasource.Timeframe1d:
			startTime = now.Add(-time.Duration(tc.limit*2) * 24 * time.Hour)
		case datasource.Timeframe1w:
			startTime = now.Add(-time.Duration(tc.limit*2) * 7 * 24 * time.Hour)
		case datasource.Timeframe1M:
			startTime = now.Add(-time.Duration(tc.limit*2) * 30 * 24 * time.Hour)
		default:
			startTime = now.Add(-time.Duration(tc.limit*2) * time.Hour)
		}

		klines, err := calculator.CalculateRate(
			ctx,
			"ADA",
			"SOL",
			"USDT",
			tc.timeframe,
			startTime,
			now,
			tc.limit,
		)

		if err != nil {
			log.Printf("❌ %s 汇率计算失败: %v", tc.name, err)
		} else {
			log.Printf("✅ %s 汇率计算成功: 获得 %d 个数据点", tc.name, len(klines))
			if len(klines) > 0 {
				latest := klines[len(klines)-1]
				log.Printf("   最新汇率: 1 ADA = %.6f SOL", latest.Close)
			}
		}
	}

	log.Println("\n🎯 测试完成")
}
