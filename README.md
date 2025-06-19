# 📈 TA Watcher

> 🤖 一个智能的加密货币技术分析监控器，让您再也不用熬夜盯盘！

## 🎯 项目背景

还在为看不过来那么多技术指标而头疼吗？😵‍💫

想象一下：
- 📊 追踪 10 个资产
- 📅 每天查看日线、周线、月线
- 💱 监控 10 个资产之间的汇率关系
- 📈 关注 3 个关键技术指标

**数学计算：** `(10 + C(10,2)) × 3 × 3 = (10 + 45) × 9 = 495` 个数据点！🤯

而且很多汇率交易对在交易所根本没有，需要程序自动计算。TA Watcher 就是为了解决这个痛点而生！

## ✨ 核心功能

- 🔄 **自动监控**：24/7 监控您关注的加密货币资产
- 📊 **技术指标计算**：MA、MACD、RSI 等主流技术指标
- 💱 **汇率计算**：自动计算交易所没有的交易对汇率
- 📧 **多渠道通知**：支持邮件、飞书、微信通知
- 🎯 **买卖建议**：基于技术分析给出操作建议
- ⚙️ **策略可配置**：支持自定义交易策略

## 🏗️ 项目结构

```
ta-watcher/
├── README.md                    # 📖 项目说明文档
├── cmd/
│   └── main.go                  # 🚀 应用程序入口
├── config.yaml                 # ⚙️ 配置文件
├── go.mod                       # 📦 Go 模块定义
├── go.sum                       # 🔒 依赖版本锁定
├── internal/                    # 🏠 核心业务逻辑
│   ├── binance/                 # 🔗 币安 API 客户端
│   │   ├── client.go
│   │   └── client_test.go
│   ├── config/                  # ⚙️ 配置管理
│   ├── indicators/              # 📊 技术指标计算
│   │   ├── ma.go               # 移动平均线
│   │   ├── macd.go             # MACD 指标
│   │   └── rsi.go              # RSI 指标
│   ├── notifiers/               # 📢 通知服务
│   │   ├── email.go            # 📧 邮件通知
│   │   ├── feishu.go           # 🚀 飞书通知
│   │   └── wechat.go           # 💬 微信通知
│   ├── strategy/                # 🎯 策略接口
│   │   └── interface.go
│   └── watcher/                 # 👀 监控核心
│       └── watcher.go
└── strategies/                  # 📈 交易策略
    ├── examples/                # 🎨 示例策略
    │   ├── rsi_strategy.go      # RSI 策略
    │   ├── macd_strategy.go     # MACD 策略
    │   └── golden_cross.go      # 金叉策略
    └── template.go              # 📝 策略模板
```

## 🚀 快速开始

### 环境要求

- 🐹 Go 1.21+

### 安装步骤

1. **克隆项目**
   ```bash
   git clone https://github.com/your-username/ta-watcher.git
   cd ta-watcher
   ```

2. **安装依赖**
   ```bash
   go mod download
   ```

3. **配置文件**
   ```bash
   cp config.yaml.example config.yaml
   # 编辑 config.yaml，配置监控资产和通知方式
   ```

4. **运行程序**
   ```bash
   go run cmd/main.go
   ```

## ⚙️ 配置说明

```yaml
# Binance 配置（使用公开API，无需密钥）
binance:
  rate_limit:
    requests_per_minute: 1200
    retry_delay: 2s
    max_retries: 3

# 监控资产列表
assets:
  - "BTCUSDT"
  - "ETHUSDT"
  - "ADAUSDT"
  # ... 更多资产

# 技术指标配置
indicators:
  ma_periods: [20, 50, 200]
  rsi_period: 14
  macd_config:
    fast_period: 12
    slow_period: 26
    signal_period: 9

# 通知配置
notifications:
  email:
    enabled: true
    smtp_host: "smtp.gmail.com"
    smtp_port: 587
    username: "your_email@gmail.com"
    password: "your_password"
    
  feishu:
    enabled: true
    webhook_url: "https://open.feishu.cn/open-apis/bot/v2/hook/your_webhook"
    
  wechat:
    enabled: false
    # 微信配置...
```

## 📊 支持的技术指标

- 📈 **MA (Moving Average)**: 移动平均线
- 📉 **MACD**: 指数平滑异同移动平均线
- ⚡ **RSI**: 相对强弱指标
- 🎯 **Golden Cross**: 金叉死叉策略
- 🔧 **自定义指标**: 支持扩展更多指标

## 🎯 策略示例

### RSI 策略
```go
// 当 RSI < 30 时建议买入
// 当 RSI > 70 时建议卖出
```

### MACD 策略
```go
// 当 MACD 线上穿信号线时建议买入
// 当 MACD 线下穿信号线时建议卖出
```

### 金叉策略
```go
// 当短期MA上穿长期MA时建议买入（金叉）
// 当短期MA下穿长期MA时建议卖出（死叉）
```

## 📱 通知渠道

### 📧 邮件通知
- 支持 SMTP 协议
- 可配置收件人列表
- HTML 格式的精美报告

### 🚀 飞书通知
- 支持飞书机器人 Webhook
- 实时推送交易建议
- 支持富文本消息

### 💬 微信通知
- 支持企业微信机器人
- 支持微信公众号模板消息
- 移动端即时接收

## 🔧 开发指南

### 添加新的技术指标

1. 在 `internal/indicators/` 目录下创建新文件
2. 实现指标计算逻辑
3. 在配置文件中添加相应配置

### 添加新的通知渠道

1. 在 `internal/notifiers/` 目录下创建新文件
2. 实现通知接口
3. 在配置文件中添加相应配置

### 创建自定义策略

1. 在 `strategies/` 目录下创建新策略文件
2. 参考 `strategies/template.go` 实现策略接口
3. 在配置文件中启用新策略

## 🤝 贡献指南

欢迎提交 Issue 和 Pull Request！🎉

1. Fork 这个项目
2. 创建您的特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交您的更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 打开一个 Pull Request

## 📄 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情

## ⚠️ 免责声明

**重要提醒：** 📢
- 本工具仅供学习和研究使用
- 所有交易建议仅供参考，不构成投资建议
- 加密货币投资有风险，请谨慎决策
- 作者不对任何投资损失承担责任

## 🙏 致谢

- [go-binance](https://github.com/adshao/go-binance) - 优秀的币安 Go SDK
- [techanalysis](https://github.com/cinar/indicator) - 技术指标计算库
- 所有为开源社区做出贡献的开发者们 ❤️

---

**🌟 如果这个项目对您有帮助，请给个 Star 支持一下！**

📧 **联系方式**: [your-email@example.com](mailto:your-email@example.com)

🐛 **Bug 报告**: [GitHub Issues](https://github.com/your-username/ta-watcher/issues)

💡 **功能建议**: [GitHub Discussions](https://github.com/your-username/ta-watcher/discussions)
