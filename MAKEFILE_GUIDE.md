# TA Watcher - Makefile 使用指南

这个项目包含了一个全面的Makefile，提供一键运行各种测试和构建任务的功能。

## 快速开始

```bash
# 查看所有可用命令
make help

# 或者直接运行 make（默认显示帮助）
make
```

## 测试命令

### 🧪 单元测试
```bash
# 运行所有单元测试
make test-unit

# 快速测试（只运行关键模块）
make test-quick

# 详细模式运行测试
make test-verbose
```

### ⚡ 基准测试
```bash
# 运行所有基准测试
make test-bench
```

### 🔗 集成测试
```bash
# 运行集成测试（需要环境变量）
make test-integration

# 启用Binance API集成测试
BINANCE_INTEGRATION_TEST=1 make test-integration

# 启用邮件集成测试（需要邮件配置）
EMAIL_INTEGRATION_TEST=1 SMTP_HOST=smtp.gmail.com SMTP_PORT=587 \
EMAIL_USERNAME=your@email.com EMAIL_PASSWORD=your_password \
FROM_EMAIL=your@email.com TO_EMAIL=recipient@email.com \
make test-integration

# 同时启用两个集成测试
BINANCE_INTEGRATION_TEST=1 EMAIL_INTEGRATION_TEST=1 make test-integration
```

### 📊 覆盖率测试
```bash
# 生成测试覆盖率报告
make test-coverage
```

### 🧪 所有测试
```bash
# 运行所有测试（单元测试 + 集成测试）
make test-all
```

## 构建命令

```bash
# 构建项目
make build

# 清理构建文件
make clean

# 格式化代码
make fmt

# 运行代码检查
make vet
make lint

# 管理依赖
make deps
make deps-update
```

## 开发工作流

```bash
# 开发环境设置
make dev-setup

# 运行所有检查（格式化、vet、单元测试）
make check

# CI流水线（依赖、检查、覆盖率测试）
make ci
```

## 环境变量说明

### 集成测试相关
- `BINANCE_INTEGRATION_TEST=1` - 启用Binance API集成测试
- `EMAIL_INTEGRATION_TEST=1` - 启用邮件集成测试

### 邮件配置（用于邮件集成测试）
- `SMTP_HOST` - SMTP服务器地址 (如: smtp.gmail.com)
- `SMTP_PORT` - SMTP端口 (如: 587)
- `SMTP_USERNAME` - 邮件用户名
- `SMTP_PASSWORD` - 邮件密码 (对于Gmail建议使用应用专用密码)
- `FROM_EMAIL` - 发件人邮箱
- `TO_EMAIL` - 收件人邮箱

## 示例用法

```bash
# 日常开发检查
make check

# 提交前完整测试
make test-all

# 性能基准测试
make test-bench

# 生成覆盖率报告
make test-coverage

# CI环境运行
make ci
```

## 注意事项

1. **基准测试**: 移除了可能卡住的基准测试（如限流器测试），只保留纯计算性能测试
2. **集成测试**: 需要适当的环境变量才会运行，默认会跳过
3. **网络测试**: 所有需要网络连接的测试都设置了跳过机制
4. **覆盖率**: 生成的HTML报告可以在浏览器中查看详细覆盖情况

## 故障排除

如果遇到测试卡住的问题：
1. 确认没有运行需要网络的测试
2. 检查是否设置了不必要的集成测试环境变量
3. 使用 `make test-quick` 进行快速验证
