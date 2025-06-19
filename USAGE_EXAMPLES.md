# TA Watcher 使用示例

## 快速开始示例

### 1. 基本运行

```bash
# 构建项目
make build

# 运行健康检查
make health

# 启动监控（前台运行）
make run
```

### 2. 生成并使用自定义策略

```bash
# 生成策略模板
make generate-strategy STRATEGY=my_trend_following

# 编辑生成的策略文件
# vim strategies/my_trend_following_strategy.go

# 编译策略为插件
make compile-strategies

# 运行时会自动加载策略
make run
```

### 3. 示例配置文件

创建 `config.yaml`：

```yaml
# 基本配置
watcher:
  interval: 5m          # 监控间隔
  max_workers: 10       # 最大工作协程
  buffer_size: 100      # 缓冲区大小

# 监控的交易对
assets:
  - "BTCUSDT"
  - "ETHUSDT"
  - "BNBUSDT"

# 策略配置
strategies:
  # 使用内置 RSI 策略
  - name: "rsi_strategy"
    enabled: true
    assets: ["BTCUSDT", "ETHUSDT"]
    interval: "1h"
    params:
      period: 14
      oversold: 30
      overbought: 70

  # 使用内置 MACD 策略
  - name: "macd_strategy"
    enabled: true
    assets: ["BTCUSDT"]
    interval: "4h"
    params:
      fast_period: 12
      slow_period: 26
      signal_period: 9

# 通知配置（可选）
notifiers:
  email:
    enabled: false  # 开发阶段建议设为 false
```

### 4. 示例自定义策略

文件：`strategies/momentum_strategy.go`

```go
package main

import (
	"fmt"
	"math"
	"time"
	
	"ta-watcher/internal/strategy"
)

// MomentumStrategy 动量策略
type MomentumStrategy struct {
	name        string
	period      int     // 动量计算周期
	threshold   float64 // 动量阈值
}

// NewStrategy 创建策略实例
func NewStrategy() strategy.Strategy {
	return &MomentumStrategy{
		name:      "momentum_strategy",
		period:    10,
		threshold: 0.05, // 5% 动量阈值
	}
}

func (s *MomentumStrategy) Name() string {
	return s.name
}

func (s *MomentumStrategy) Description() string {
	return fmt.Sprintf("动量策略，周期：%d，阈值：%.1f%%", s.period, s.threshold*100)
}

func (s *MomentumStrategy) RequiredDataPoints() int {
	return s.period + 5
}

func (s *MomentumStrategy) SupportedTimeframes() []strategy.Timeframe {
	return []strategy.Timeframe{
		strategy.Timeframe15m,
		strategy.Timeframe1h,
		strategy.Timeframe4h,
		strategy.Timeframe1d,
	}
}

func (s *MomentumStrategy) Evaluate(data *strategy.MarketData) (*strategy.StrategyResult, error) {
	if len(data.Klines) < s.RequiredDataPoints() {
		return &strategy.StrategyResult{
			Signal:     strategy.SignalNone,
			Confidence: 0.0,
			Message:    "数据不足",
			Timestamp:  time.Now(),
		}, nil
	}

	// 计算价格动量 (当前价格相对于N周期前的变化率)
	currentPrice := data.Klines[len(data.Klines)-1].Close
	pastPrice := data.Klines[len(data.Klines)-1-s.period].Close
	momentum := (currentPrice - pastPrice) / pastPrice

	// 计算成交量动量
	currentVolume := data.Klines[len(data.Klines)-1].Volume
	avgVolume := 0.0
	for i := len(data.Klines) - s.period; i < len(data.Klines); i++ {
		avgVolume += data.Klines[i].Volume
	}
	avgVolume /= float64(s.period)
	volumeRatio := currentVolume / avgVolume

	// 生成信号
	var signal strategy.Signal
	var strength strategy.Strength
	var confidence float64
	var message string

	// 强动量 + 成交量放大 = 强信号
	if math.Abs(momentum) > s.threshold {
		if momentum > 0 {
			signal = strategy.SignalBuy
			message = fmt.Sprintf("上涨动量 %.2f%%, 买入信号", momentum*100)
		} else {
			signal = strategy.SignalSell
			message = fmt.Sprintf("下跌动量 %.2f%%, 卖出信号", -momentum*100)
		}

		// 成交量确认强度
		if volumeRatio > 1.5 {
			strength = strategy.StrengthStrong
			confidence = 0.85
		} else {
			strength = strategy.StrengthNormal
			confidence = 0.65
		}
	} else {
		signal = strategy.SignalHold
		strength = strategy.StrengthWeak
		confidence = 0.3
		message = "动量不足，持有"
	}

	return &strategy.StrategyResult{
		Signal:     signal,
		Strength:   strength,
		Confidence: confidence,
		Price:      currentPrice,
		Timestamp:  time.Now(),
		Message:    message,
		Metadata: map[string]interface{}{
			"momentum":      momentum,
			"volume_ratio":  volumeRatio,
			"current_price": currentPrice,
			"past_price":    pastPrice,
		},
		Indicators: map[string]interface{}{
			"momentum_pct":   momentum * 100,
			"volume_ratio":   volumeRatio,
		},
	}, nil
}
```

### 5. 编译和运行自定义策略

```bash
# 编译策略为插件
go build -buildmode=plugin -o strategies/momentum_strategy.so strategies/momentum_strategy.go

# 运行时会自动加载
./bin/ta-watcher -config config.yaml -strategies strategies
```

### 6. 监控输出示例

```
2025/06/19 22:20:46 === TA Watcher v1.0.0 启动中 ===
2025/06/19 22:20:46 配置文件: config.yaml
2025/06/19 22:20:46 策略目录: strategies
2025/06/19 22:20:46 监控间隔: 5m0s
2025/06/19 22:20:46 工作协程: 10
2025/06/19 22:20:46 监控资产: [BTCUSDT ETHUSDT BNBUSDT]
2025/06/19 22:20:46 Loading custom strategies from directory: strategies
2025/06/19 22:20:46 Custom strategy momentum_strategy loaded successfully
2025/06/19 22:20:46 Starting TA Watcher...
2025/06/19 22:20:46 TA Watcher started with 10 workers
2025/06/19 22:20:46 Monitor loop started, interval: 5m0s

2025/06/19 22:25:46 Starting monitoring cycle with 6 tasks
2025/06/19 22:25:47 Signal detected: BTCUSDT BUY STRONG (85% confidence)
2025/06/19 22:25:47 Signal detected: ETHUSDT SELL NORMAL (65% confidence)

=== 状态报告 ===
运行时间: 5m0s
活跃工作者: 0
待处理任务: 0
总任务: 6
完成任务: 6
失败任务: 0
发送通知: 2
资产监控统计:
  BTCUSDT: 检查1次, 信号1次, 最后信号: BUY
  ETHUSDT: 检查1次, 信号1次, 最后信号: SELL
  BNBUSDT: 检查1次, 信号0次, 最后信号: 
```

### 7. 开发调试

```bash
# 查看策略文件
make list-strategies

# 格式化代码
make fmt

# 代码检查
make vet

# 运行所有测试
make test

# 重新设置开发环境
make dev-setup
```

### 8. 常用命令速查

```bash
# 项目管理
make build                           # 构建项目
make clean                          # 清理构建文件
make dev-setup                      # 开发环境设置

# 运行相关
make run                            # 前台运行
make run-daemon                     # 后台运行
make health                         # 健康检查

# 策略开发
make generate-strategy STRATEGY=名称  # 生成策略模板
make compile-strategies             # 编译策略插件
make list-strategies               # 列出策略文件

# 开发工具
make fmt                           # 代码格式化
make vet                           # 代码检查
make test                          # 运行测试
```

## 常见问题

### Q: 如何添加新的技术指标？
A: 在 `internal/indicators/` 目录下添加新的指标实现，然后在策略中引用。

### Q: 如何调整监控频率？
A: 修改 `config.yaml` 中的 `watcher.interval` 配置。

### Q: 策略编译失败怎么办？
A: 检查策略文件语法，确保实现了所有必需的接口方法。

### Q: 如何停止监控？
A: 按 `Ctrl+C` 或发送 SIGTERM 信号。

### Q: 如何查看详细日志？
A: 设置 `config.yaml` 中的 `watcher.log_level: "debug"`。

这就是 TA Watcher 的完整使用指南！🚀
