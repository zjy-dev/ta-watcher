# 多数据源支持

## 🌍 支持的交易所

### ✅ Coinbase Pro (推荐用于美国 IP)

- **地理位置**: 美国本土，完全支持美国 IP
- **免费 API**: 无需认证的公开数据
- **币种支持**: BTC, ETH, ADA 等主流币种
- **稳定性**: 企业级稳定性

### ✅ Binance (全球主流)

- **地理位置**: 亚洲为主，美国 IP 受限
- **免费 API**: 丰富的公开数据
- **币种支持**: 最全面的币种覆盖
- **数据质量**: 行业标准

## 🔧 配置示例

### GitHub Actions 友好配置 (美国 IP)

```yaml
# config.example.yaml
datasource:
  primary: "coinbase" # 主数据源使用Coinbase
  fallback: "binance" # 备用数据源（如果需要）
  timeout: 30s
  max_retries: 3

coinbase:
  rate_limit:
    requests_per_minute: 600
    retry_delay: 3s
    max_retries: 3

binance:
  rate_limit:
    requests_per_minute: 1200
    retry_delay: 2s
    max_retries: 3
```

### 亚洲服务器配置 (Binance 优先)

```yaml
# config.example.yaml
datasource:
  primary: "binance" # 主数据源使用Binance
  fallback: "coinbase" # 备用数据源
  timeout: 30s
  max_retries: 3
```

## 🚀 自动切换机制

系统会自动处理数据源切换：

1. **正常情况**: 使用主数据源
2. **主数据源失败**: 自动切换到备用数据源
3. **智能重试**: 各数据源独立重试策略
4. **详细日志**: 记录切换原因和状态

## 📊 币种映射

### 自动转换

| 通用格式 | Binance | Coinbase Pro |
| -------- | ------- | ------------ |
| BTCUSDT  | BTCUSDT | BTC-USD      |
| ETHUSDT  | ETHUSDT | ETH-USD      |
| ADAUSDT  | ADAUSDT | ADA-USD      |

### 支持状态

- ✅ **BTC/USDT**: 两个平台都支持
- ✅ **ETH/USDT**: 两个平台都支持
- ✅ **ADA/USDT**: 两个平台都支持
- ⚠️ **BNB/USDT**: Coinbase 可能不支持，会自动切换到 Binance

## 🧪 测试结果

### Coinbase Pro 测试

```bash
=== 测试结果 ===
✅ BTC价格: $105,710.53
✅ 获取到 10 条K线数据
✅ 最新K线数据正常
✅ 美国IP访问成功
```

### 性能对比

| 数据源   | 响应时间 | 稳定性 | 美国 IP 支持 |
| -------- | -------- | ------ | ------------ |
| Coinbase | ~500ms   | 优秀   | ✅ 完全支持  |
| Binance  | ~300ms   | 优秀   | ❌ 受限      |

## 🔄 GitHub Actions 集成

更新后的 workflow 将自动使用 Coinbase：

```yaml
# .github/workflows/scheduled-run.yml (已自动更新)
- name: Run TA Watcher
  run: ./bin/ta-watcher --single-run --config config.example.yaml
  # 会自动使用 Coinbase Pro API，无需额外配置
```

## 📈 监控币种建议

基于 Coinbase 支持情况，推荐监控币种：

### 完全支持 (Coinbase + Binance)

- **BTC/USDT** - 比特币 ✅
- **ETH/USDT** - 以太坊 ✅
- **ADA/USDT** - 卡尔达诺 ✅
- **DOT/USDT** - 波卡 ✅
- **LINK/USDT** - 链接 ✅

### 需要切换 (仅 Binance 支持)

- **BNB/USDT** - 币安币 (自动切换到 Binance)

## 🛠️ 开发说明

### 添加新数据源

1. 实现 `DataSource` 接口
2. 创建对应的包装器
3. 在工厂方法中注册
4. 更新配置结构

### API 兼容性

所有数据源都遵循统一的接口：

```go
type DataSource interface {
    GetKlines(ctx context.Context, symbol string, interval string, limit int) ([]Kline, error)
    GetPrice(ctx context.Context, symbol string) (float64, error)
    Name() string
}
```

## ✨ 结论

通过多数据源支持：

- ✅ **解决地理限制**: GitHub Actions 可以正常使用
- ✅ **提高可靠性**: 主备切换机制
- ✅ **保持兼容性**: 现有配置继续有效
- ✅ **扩展能力**: 未来可以轻松添加更多数据源

现在可以在任何地理位置安全运行 TA Watcher！🎉
