# GitHub Actions 配置指南

## 概述

TA Watcher 已配置为使用 GitHub Actions 进行定时运行。配置为每天两次：

- **北京时间早上 6 点** (UTC 22:00)
- **北京时间晚上 6 点** (UTC 10:00)

## 配置步骤

### 1. 设置 Repository Secrets

在你的 GitHub 仓库中设置以下 secrets：

1. 进入仓库页面
2. 点击 `Settings` -> `Secrets and variables` -> `Actions`
3. 添加以下 secrets：

#### 必需的 Email 配置

```
SMTP_HOST=smtp.gmail.com          # 或其他SMTP服务器
SMTP_USERNAME=your-email@gmail.com
SMTP_PASSWORD=your-app-password   # Gmail需要使用应用专用密码
FROM_EMAIL=your-email@gmail.com
TO_EMAIL=alert-email@gmail.com    # 接收通知的邮箱
```

#### 可选的 Webhook 配置

```
FEISHU_WEBHOOK_URL=https://...    # 飞书群机器人webhook
WECHAT_WEBHOOK_URL=https://...    # 企业微信群机器人webhook
```

### 2. Gmail 配置指南

如果使用 Gmail SMTP：

1. **启用 2FA（两步验证）**

   - 进入 Google 账户设置
   - 安全 -> 两步验证

2. **生成应用专用密码**

   - Google 账户 -> 安全 -> 应用专用密码
   - 选择"邮件"和设备类型
   - 复制生成的 16 位密码作为 `SMTP_PASSWORD`

3. **验证配置**
   ```yaml
   SMTP_HOST=smtp.gmail.com
   SMTP_USERNAME=your-email@gmail.com
   SMTP_PASSWORD=generated-app-password  # 16位应用专用密码
   FROM_EMAIL=your-email@gmail.com
   TO_EMAIL=alert-email@gmail.com
   ```

### 3. 其他邮箱服务配置

#### Outlook/Hotmail

```yaml
SMTP_HOST=smtp-mail.outlook.com
SMTP_USERNAME=your-email@outlook.com
SMTP_PASSWORD=your-password
```

#### QQ 邮箱

```yaml
SMTP_HOST=smtp.qq.com
SMTP_USERNAME=your-email@qq.com
SMTP_PASSWORD=your-authorization-code # 需要开启SMTP服务
```

#### 163 邮箱

```yaml
SMTP_HOST=smtp.163.com
SMTP_USERNAME=your-email@163.com
SMTP_PASSWORD=your-authorization-code # 需要开启SMTP服务
```

### 4. 验证配置

#### 手动触发测试

1. 进入仓库的 `Actions` 页面
2. 选择 `TA Watcher Scheduled Run` workflow
3. 点击 `Run workflow` -> `Run workflow`
4. 查看运行日志确认配置正确

#### 查看运行状态

- 定时运行会自动在设定时间执行
- 可以在 `Actions` 页面查看运行历史和日志
- 失败的运行会在 `Actions` 页面显示红色标记

### 5. 监控配置

当前监控的资产（可在 `config.example.yaml` 中修改）：

- BTC/USDT - RSI 策略
- ETH/USDT - RSI 策略
- BNB/USDT - MA 交叉策略
- ADA/USDT - MACD 策略

### 6. 日志收集

每次运行的日志会自动收集并存储 7 天：

- 运行完成后可在 `Actions` 页面下载日志文件
- 日志包含详细的运行信息和错误诊断

### 7. 时间配置说明

```yaml
# 当前时间设置
schedule:
  - cron: "0 10,22 * * *" # UTC时间
```

对应的北京时间：

- `10 UTC` = `18:00 北京时间` (晚上 6 点)
- `22 UTC` = `06:00 北京时间` (早上 6 点)

### 8. 故障排除

#### 常见问题

1. **邮件发送失败**

   - 检查 SMTP 配置是否正确
   - 确认邮箱服务已启用 SMTP
   - 验证应用专用密码（Gmail）

2. **构建失败**

   - 检查 Go 版本兼容性
   - 确认依赖模块可用

3. **运行超时**
   - GitHub Actions 免费账户有运行时长限制
   - 优化监控资产数量

#### 调试步骤

1. 查看 Actions 运行日志
2. 检查 secrets 配置
3. 手动触发测试运行
4. 联系维护者获取支持

### 9. 成本说明

- **GitHub Actions**: 免费账户每月 2000 分钟
- **预估使用**: 每次运行约 2-3 分钟，每天两次约 6 分钟
- **月度使用**: 约 180 分钟，完全在免费额度内

### 10. 进阶配置

#### 自定义监控频率

修改 `.github/workflows/scheduled-run.yml` 中的 cron 表达式：

```yaml
schedule:
  - cron: "0 6,18 * * *" # 每天14:00和02:00北京时间
  - cron: "0 */6 * * *" # 每6小时运行一次
  - cron: "0 9 * * 1-5" # 工作日上午5点北京时间
```

#### 添加更多资产监控

编辑 `config.example.yaml`，在 `assets` 部分添加新的监控对象。

## 立即开始

1. 设置上述 repository secrets
2. 推送代码到 `single-run` 分支（如果有修改）
3. 手动触发一次测试运行
4. 等待定时任务自动运行

配置完成后，系统将自动在每天早晚 6 点（北京时间）运行技术分析监控，并通过邮件发送警报通知。
