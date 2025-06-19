# TA Watcher - 技术分析监控工具

一个灵活、可扩展的技术分析监控工具，支持多种技术指标、时间粒度，并提供自动化监控、决策与通知功能。

## 🌟 特性

- **多策略支持**: 内置 RSI、MACD、移动平均线等经典策略
- **自定义策略**: 支持用户编写 Go 文件形式的自定义策略
- **多时间框架**: 支持 1m 到 1M 的各种时间粒度
- **实时监控**: 自动化监控多个交易对
- **智能通知**: 支持邮件、飞书、微信等多种通知方式
- **风险管理**: 内置风险评估和通知冷却机制
- **高性能**: 并发处理，支持大量交易对监控

## 🚀 快速开始

### 1. 环境准备

```bash
# 克隆项目
git clone <repository-url>
cd ta-watcher

# 快速设置并运行
make quick-start
```

### 2. 配置文件

复制并编辑配置文件：

```bash
cp config.example.yaml config.yaml
# 编辑 config.yaml 设置监控的交易对和通知配置
```

### 3. 运行监控

```bash
# 前台运行
make run

# 后台运行
make run-daemon

# 健康检查
make health
```

## 📝 自定义策略开发

### 生成策略模板

```bash
# 生成名为 "my_strategy" 的策略模板
make generate-strategy STRATEGY=my_strategy
```

这会在 `strategies/` 目录下生成 `my_strategy_strategy.go` 文件。

### 策略文件结构

```go
package main

import (
    "ta-watcher/internal/strategy"
    "ta-watcher/internal/binance"
)

// MyStrategy 自定义策略
type MyStrategy struct {
    name        string
    description string
    // 策略参数
    period      int
    threshold   float64
}

// NewStrategy 创建策略实例 (必须导出)
func NewStrategy() strategy.Strategy {
    return &MyStrategy{
        name:        "my_strategy",
        description: "我的自定义策略",
        period:      14,
        threshold:   0.02,
    }
}

// 实现 Strategy 接口的必要方法
func (s *MyStrategy) Name() string { return s.name }
func (s *MyStrategy) Description() string { return s.description }
func (s *MyStrategy) RequiredDataPoints() int { return s.period + 10 }
func (s *MyStrategy) SupportedTimeframes() []strategy.Timeframe {
    return []strategy.Timeframe{
        strategy.Timeframe5m,
        strategy.Timeframe1h,
        strategy.Timeframe1d,
    }
}

// Evaluate 策略核心逻辑
func (s *MyStrategy) Evaluate(data *strategy.MarketData) (*strategy.StrategyResult, error) {
    // 在这里实现你的策略逻辑
    // 返回买入/卖出/持有信号
    return &strategy.StrategyResult{
        Signal:     strategy.SignalBuy, // 或 SignalSell, SignalHold
        Strength:   strategy.StrengthNormal,
        Confidence: 0.8,
        Price:      data.Klines[len(data.Klines)-1].Close,
        Message:    "策略信号描述",
    }, nil
}
```

### 编译和使用策略

```bash
# 编译策略为插件
make compile-strategies

# 查看策略文件
make list-strategies

# 运行时会自动加载编译好的策略插件
make run
```

## 🔧 命令行工具

### 基本命令

```bash
# 显示帮助
./bin/ta-watcher -h

# 指定配置文件运行
./bin/ta-watcher -config my-config.yaml

# 指定策略目录
./bin/ta-watcher -strategies ./my-strategies

# 生成策略模板
./bin/ta-watcher -generate my_awesome_strategy

# 健康检查
./bin/ta-watcher -health

# 显示版本
./bin/ta-watcher -version
```

### Make 命令

```bash
# 开发相关
make build           # 构建应用程序
make run             # 运行应用程序
make health          # 健康检查
make clean           # 清理构建文件

# 策略相关
make generate-strategy STRATEGY=策略名  # 生成策略模板
make compile-strategies                 # 编译策略插件
make list-strategies                   # 列出策略文件

# 开发工具
make fmt             # 格式化代码
make vet             # 代码检查
make test            # 运行测试
make dev-setup       # 开发环境设置
```

## 📊 监控配置

### 资产配置

在 `config.yaml` 中配置要监控的交易对：

```yaml
assets:
  - "BTCUSDT"
  - "ETHUSDT"
  - "BNBUSDT"
  - "ADAUSDT"
```

### 策略配置

```yaml
strategies:
  - name: "rsi_strategy"
    enabled: true
    assets:
      - "BTCUSDT"
      - "ETHUSDT"
    interval: "1h"
    params:
      period: 14
      oversold: 30
      overbought: 70
```

### 通知配置

```yaml
notifiers:
  email:
    enabled: true
    smtp:
      host: "smtp.gmail.com"
      port: 587
      username: "${SMTP_USERNAME}"
      password: "${SMTP_PASSWORD}"
    from: "${FROM_EMAIL}"
    to:
      - "${TO_EMAIL}"
```

## 🎯 内置策略

- **RSI策略**: 基于相对强弱指数的超买超卖策略
- **MACD策略**: 基于 MACD 指标的趋势跟踪策略
- **移动平均线策略**: 基于 MA 交叉的趋势策略
- **复合策略**: 多策略组合决策

## 🔍 监控面板

程序运行时会定期输出状态报告：

```
=== 状态报告 ===
运行时间: 1h30m45s
活跃工作者: 8
待处理任务: 2
总任务: 1250
完成任务: 1248
失败任务: 2
发送通知: 15
资产监控统计:
  BTCUSDT: 检查125次, 信号8次, 最后信号: BUY
  ETHUSDT: 检查125次, 信号5次, 最后信号: SELL
```

## 🛠️ 开发指南

### 项目结构

```
ta-watcher/
├── cmd/watcher/          # 主程序入口
├── internal/
│   ├── strategy/         # 策略系统
│   ├── watcher/          # 监控服务
│   ├── binance/          # 数据源
│   ├── notifiers/        # 通知系统
│   └── config/           # 配置管理
├── strategies/           # 自定义策略目录
├── config.yaml          # 配置文件
└── Makefile             # 构建脚本
```

### 策略开发最佳实践

1. **参数化设计**: 将策略参数作为结构体字段，便于调整
2. **时间框架支持**: 明确策略支持的时间框架
3. **数据验证**: 检查输入数据的完整性
4. **错误处理**: 优雅处理异常情况
5. **性能优化**: 避免重复计算，缓存中间结果
6. **测试覆盖**: 为策略编写单元测试

### 扩展通知方式

可以在 `internal/notifiers/` 中添加新的通知器实现。

## 📈 性能优化

- 使用协程池限制并发数
- 智能缓存技术指标计算结果
- 通知冷却机制避免重复通知
- 批量处理减少 API 调用

## 🔐 风险管理

- 内置风险评估机制
- 止损止盈设置
- 最大持仓数量限制
- 通知频率控制

## 🐛 故障排除

### 常见问题

1. **配置文件错误**: 使用 `make health` 检查配置
2. **策略编译失败**: 检查策略文件语法和依赖
3. **网络连接问题**: 确认网络和代理设置
4. **权限问题**: 确保有读写策略目录的权限

### 日志分析

程序会输出详细的运行日志，包括：
- 策略评估结果
- 信号生成记录
- 通知发送状态
- 错误和警告信息

## 📄 许可证

MIT License

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

---

**开始你的技术分析监控之旅！** 🚀
