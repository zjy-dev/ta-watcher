# TA Watcher - 技术分析监控工具

一个简洁、高效的加密货币技术分析自动监控系统，实时监控交易信号并发送通知。

## 🎯 项目特点

- **极简设计**: 删除了复杂的中间层，watcher 直接调用 strategy 接口
- **高度可扩展**: 支持自定义策略，内置多种经典技术指标
- **实时监控**: 定时获取市场数据，自动执行技术分析
- **多种通知**: 支持邮件、飞书、微信等多种通知方式
- **健壮可靠**: 完整的错误处理和优雅停止机制

## 📁 项目结构

```
ta-watcher/
├── cmd/watcher/           # 主程序入口
├── internal/
│   ├── binance/          # 币安数据源模块
│   ├── config/           # 配置管理模块
│   ├── indicators/       # 技术指标计算模块
│   ├── notifiers/        # 通知管理模块  
│   ├── strategy/         # 策略管理模块
│   └── watcher/          # 核心监控模块 (极简设计)
├── config.yaml          # 配置文件
└── Makefile             # 构建脚本
```

## 🏗️ 核心模块设计

### 1. Watcher 监控模块 (internal/watcher/)

**设计思路**: 这是整个系统的核心协调器，采用极简设计，直接调用各个组件接口，避免过度抽象。

**核心结构**:
```go
type Watcher struct {
    config     *config.Config        // 配置
    dataSource binance.DataSource    // 数据源
    notifier   *notifiers.Manager    // 通知管理器
    strategy   *strategy.Manager     // 策略管理器
    
    running bool                     // 运行状态
    stats   *Statistics             // 统计信息
}
```

**工作流程**:
1. 定时获取配置中指定的交易对K线数据
2. 对每个策略执行技术分析
3. 如果检测到买入/卖出信号，发送通知
4. 更新统计信息

**文件说明**:
- `types.go` (70行): 核心类型定义，保持最简
- `watcher.go` (242行): 主要业务逻辑，直接调用各模块接口
- `watcher_test.go` (141行): 单元测试

### 2. Strategy 策略模块 (internal/strategy/)

**设计思路**: 提供策略接口和管理器，支持内置策略和自定义策略扩展。

**核心接口**:
```go
type Strategy interface {
    Name() string                                    // 策略名称
    Evaluate(data *MarketData) (*StrategyResult, error) // 执行分析
    RequiredDataPoints() int                         // 所需数据点数
}

type Manager struct {
    strategies map[string]Strategy // 策略注册表
}
```

**信号类型**:
```go
type Signal int
const (
    SignalNone Signal = iota  // 无信号
    SignalBuy                 // 买入信号  
    SignalSell                // 卖出信号
    SignalHold                // 持有信号
)
```

**内置策略**:
- **RSI策略**: 基于相对强弱指数，超卖时买入，超买时卖出
- **MACD策略**: 基于移动平均收敛背离，金叉买入，死叉卖出  
- **均线交叉策略**: 短期均线上穿长期均线时买入
- **多策略组合**: 专为通知系统设计，任何子策略触发信号都会发送通知，避免复杂的投票或加权逻辑

### 3. Binance 数据源模块 (internal/binance/)

**设计思路**: 封装币安API调用，提供统一的数据接口，内置限流和错误处理。

**核心接口**:
```go
type DataSource interface {
    GetKlines(ctx context.Context, symbol, interval string, limit int) ([]*KlineData, error)
}

type KlineData struct {
    OpenTime   int64   // 开盘时间
    Open       float64 // 开盘价
    High       float64 // 最高价
    Low        float64 // 最低价  
    Close      float64 // 收盘价
    Volume     float64 // 交易量
}
```

**特性**:
- 自动限流控制，避免触发API限制
- 重试机制，提高数据获取稳定性
- 上下文支持，可控制超时

### 4. Notifiers 通知模块 (internal/notifiers/)

**设计思路**: 统一通知接口，支持多种通知方式并行发送。

**核心接口**:
```go
type Notifier interface {
    Send(notification *Notification) error // 发送通知
    Name() string                          // 通知器名称
    IsEnabled() bool                       // 是否启用
}

type Notification struct {
    Type      NotificationType       // 通知类型
    Level     NotificationLevel      // 通知级别  
    Asset     string                 // 相关资产
    Strategy  string                 // 相关策略
    Message   string                 // 通知内容
    Timestamp time.Time              // 时间戳
}
```

**支持的通知方式**:
- 邮件通知 (SMTP)
- 飞书机器人
- 微信通知
- 控制台输出 (调试用)

### 5. Indicators 技术指标模块 (internal/indicators/)

**设计思路**: 提供常用技术指标的计算函数，供策略模块调用。

**核心函数**:
```go
func RSI(prices []float64, period int) []float64        // RSI指标
func SMA(prices []float64, period int) []float64        // 简单移动平均
func EMA(prices []float64, period int) []float64        // 指数移动平均
func MACD(prices []float64) ([]float64, []float64, []float64) // MACD指标
```

### 6. Config 配置模块 (internal/config/)

**设计思路**: 简化配置结构，只保留必要的配置项。

**配置结构**:
```go
type Config struct {
    Binance   BinanceConfig   // 币安配置
    Watcher   WatcherConfig   // 监控配置  
    Notifiers NotifiersConfig // 通知配置
    Assets    []string        // 监控的交易对
}
```

## 🚀 快速开始

### 1. 安装依赖

```bash
go mod tidy
```

### 2. 配置文件

复制并编辑配置文件:
```bash
cp config.example.yaml config.yaml
```

配置示例:
```yaml
watcher:
  interval: 1m          # 监控间隔
  max_workers: 5        # 最大并发数
  
assets:                 # 监控的交易对
  - "BTCUSDT"
  - "ETHUSDT"

notifiers:
  email:
    enabled: true
    smtp_host: "smtp.gmail.com"
    # ... 其他邮件配置
```

### 3. 运行

```bash
# 构建
make build

# 运行
./watcher

# 后台运行
./watcher --daemon

# 健康检查
./watcher --health
```

## 🔧 自定义策略开发

创建自定义策略只需实现 `Strategy` 接口:

```go
type MyStrategy struct{}

func (s *MyStrategy) Name() string {
    return "my_custom_strategy"
}

func (s *MyStrategy) Evaluate(data *strategy.MarketData) (*strategy.StrategyResult, error) {
    // 实现你的交易逻辑
    // 返回买入/卖出/持有信号
    
    return &strategy.StrategyResult{
        Signal:     strategy.SignalBuy,
        Confidence: 0.8,
        Price:      data.Klines[len(data.Klines)-1].Close,
        Message:    "检测到买入信号",
    }, nil
}

func (s *MyStrategy) RequiredDataPoints() int {
    return 20  // 需要20个数据点
}
```

然后在初始化时注册策略:
```go
manager.RegisterStrategy(&MyStrategy{})
```

## 📊 监控和统计

系统提供详细的运行统计:
- 总任务数、成功/失败数
- 通知发送统计  
- 运行时间和健康状态

可通过健康检查接口获取实时状态:
```bash
./watcher --health
```

## 🛠️ 开发说明

### 设计原则

1. **极简优先**: 删除不必要的抽象层，直接调用接口
2. **单一职责**: 每个模块只负责自己的核心功能  
3. **接口导向**: 通过接口解耦，便于测试和扩展
4. **错误友好**: 完善的错误处理，不会因单个错误停止整个系统

### 测试

```bash
# 运行所有测试
make test

# 运行特定模块测试
go test ./internal/watcher/ -v
go test ./internal/strategy/ -v
```

### 架构优势

相比复杂的微服务架构，我们选择了单体但模块化的设计:
- **简单**: 无需复杂的服务发现和通信
- **高效**: 内存中直接调用，性能更好
- **可靠**: 减少了网络调用的不确定性
- **易维护**: 代码集中，便于调试和修改

## 🔄 重构历程

本项目经历了从复杂到简单的演进过程:

### v0.x 复杂设计阶段
- 有多层抽象: `StrategyAdapter`、`TaskGenerator` 等中间层
- 文件众多: 10+ 个文件，上千行代码
- 接口复杂: 多重嵌套的调用关系

### v1.0 极简设计阶段  
- **删除中间层**: 移除 `StrategyAdapter`，watcher 直接调用 strategy
- **精简文件**: 只保留 3 个核心文件，总共 ~450 行代码
- **直接调用**: 清晰的调用链，无需复杂的适配器模式

### 重构原则
> "Less is More" - 复杂的设计往往是过度设计的结果

## 📝 更新日志

- **v1.0.0**: 极简架构设计，删除复杂中间层，直接接口调用
- 完整的单元测试覆盖  
- 支持优雅停止和健康检查
- 内置多种经典技术分析策略

## 🚦 使用示例

```bash
# 查看版本
./watcher --version

# 健康检查
./watcher --health

# 运行监控
./watcher

# 后台运行
./watcher --daemon
```

---

**注意**: 本项目仅用于学习和研究目的，不构成投资建议。加密货币交易有风险，请谨慎投资。
