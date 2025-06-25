# 快速设置指南

## ✅ 2025 年 6 月更新 - Actions 版本升级

所有 GitHub Actions 已更新到最新稳定版本：

- ✅ `actions/upload-artifact@v4` - 修复 v3 废弃问题
- ✅ `actions/cache@v4` - 最新缓存策略
- ✅ `actions/checkout@v4` - 最新代码检出
- ✅ `actions/setup-go@v4` - Go 环境设置

### v4 主要改进：

- **不可变性**: 每个 artifact 都有唯一 ID，防止意外覆盖
- **更好的性能**: 优化的上传和下载速度
- **增强安全性**: 更严格的权限控制
- **动态命名**: 支持 `${{ github.run_number }}` 避免冲突

## 🚀 GitHub Actions CI/CD 流程

### 改进的 CI 流程包含：

1. **🧪 自动测试** - 每次 push 都会运行 `make test-all`
2. **🚀 立即运行** - push 到 single-run 分支后立即执行一次
3. **⏰ 定时任务** - 每天早晚 6 点（北京时间）自动运行

### 第一步：设置 Repository Secrets

1. **进入仓库设置**

   - 在 GitHub 仓库页面，点击 `Settings`
   - 左侧菜单选择 `Secrets and variables` -> `Actions`

2. **添加必需的 Secrets**
   点击 `New repository secret` 并添加以下 5 个 secrets：

   ```
   Name: SMTP_HOST
   Value: smtp.gmail.com  (或你的邮件服务器)

   Name: SMTP_USERNAME
   Value: your-email@gmail.com

   Name: SMTP_PASSWORD
   Value: your-app-password  (Gmail需要生成应用专用密码)

   Name: FROM_EMAIL
   Value: your-email@gmail.com

   Name: TO_EMAIL
   Value: alert-receiver@gmail.com  (接收通知的邮箱)
   ```

### 第二步：Gmail 应用专用密码设置

如果使用 Gmail：

1. **启用两步验证**

   - Google 账户 -> 安全 -> 两步验证

2. **生成应用专用密码**
   - Google 账户 -> 安全 -> 应用专用密码
   - 选择"邮件"应用类型
   - 复制 16 位密码作为 `SMTP_PASSWORD`

### 第三步：完整的 CI/CD 流程

现在的 CI 流程包含三个 job：

1. **🧪 自动测试** (每次 push 和 PR)

   - 运行 `make test-all`
   - 构建应用程序
   - 健康检查测试

2. **⚙️ 配置测试** (手动触发或提交消息包含 `[test-config]`)

   - 验证所有 secrets 配置状态
   - 测试健康检查和版本信息
   - 验证配置文件格式

3. **🚀 立即运行** (push 到 single-run 分支后)
   - 等待测试通过后立即执行一次 TA Watcher
   - 验证邮件通知功能
   - 确认整个流程正常

### 第四步：测试和 Push 流程

设置完 secrets 后有多种测试方式：

1. **手动触发配置测试**

   - Actions -> CI/CD Pipeline -> Run workflow

2. **提交触发配置测试**

   ```bash
   git commit -m "update config [test-config]"
   ```

3. **Push 触发完整流程**
   ```bash
   git push origin single-run  # 会依次触发：测试 -> 立即运行 -> 定时任务
   ```

### 第五步：验证运行结果

- 进入仓库的 `Actions` 标签
- 查看 `CI/CD Pipeline` 的运行状态
- 确保测试通过且立即运行成功

### 第四步：验证和监控

1. **手动测试** (可选)

   - Actions -> `Test TA Watcher Setup` -> `Run workflow`

2. **查看日志**
   - 每次运行的详细日志可在 Actions 页面下载
   - 包含测试结果、构建日志、运行输出

### 故障排除

如果遇到问题：

1. **检查 secrets 是否正确设置**

   - 名称必须完全匹配（区分大小写）
   - Gmail 必须使用应用专用密码，不是普通密码

2. **查看 Actions 日志**

   - Actions 标签页 -> 失败的 workflow -> 点击查看详细日志

3. **手动触发测试**
   - 使用 `workflow_dispatch` 手动运行进行调试

---

## 📧 邮件通知示例

成功运行后，你会收到类似这样的邮件通知：

```
主题: 🚨 TA Watcher Alert - BTC RSI Signal

BTC/USDT RSI信号触发
当前RSI: 75.2 (超买区间)
价格: $43,250.00
时间: 2025-06-24 18:00:00 UTC
```

## 🔧 自定义配置

如需调整监控参数，编辑 `config.example.yaml` 文件中的：

- 监控的币种列表
- RSI/MACD 等指标参数
- 通知阈值设置

### 🔄 CI/CD 流程说明

#### 3 个工作流程文件：

1. **`ci.yml`** - 主 CI/CD 流程
   - 每次 push 运行测试
   - single-run 分支 push 后立即执行一次
2. **`scheduled-run.yml`** - 定时任务
   - 北京时间早 6 点 (UTC 22:00)
   - 北京时间晚 6 点 (UTC 10:00)
3. **`test-setup.yml`** - 手动测试
   - 用于验证配置和调试

#### 运行时机：

- **测试**：每次 push 到 main 或 single-run 分支
- **立即运行**：仅在 push 到 single-run 分支时触发
- **定时运行**：每天两次，北京时间早晚 6 点
