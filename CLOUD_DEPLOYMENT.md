# TA Watcher 云部署指南

本项目已优化支持多种云平台的单次运行模式，通过定时任务触发，大幅节省运行成本。

## 🏆 推荐云平台排序

### 1. Railway.app ⭐⭐⭐⭐⭐

- **免费额度**: $5/月 免费额度
- **优势**:
  - 原生支持 cron jobs
  - 自动扩缩容
  - 简单部署，支持 GitHub 集成
  - 内置日志和监控
- **缺点**: 免费额度有限
- **适合**: 轻度使用，个人项目

### 2. Fly.io ⭐⭐⭐⭐

- **免费额度**: 每月免费运行时间
- **优势**:
  - 优秀的 cron 支持
  - 全球边缘部署
  - 资源利用率高
- **缺点**: 配置稍复杂
- **适合**: 需要低延迟的全球用户

### 3. Render.com ⭐⭐⭐⭐

- **免费额度**: 750 小时/月免费
- **优势**:
  - 简单易用
  - 支持 cron jobs
  - 自动 HTTPS
- **缺点**: 免费版有请求限制
- **适合**: 中小型项目

### 4. Google Cloud Run ⭐⭐⭐

- **免费额度**: 每月 200 万请求免费
- **优势**:
  - 按需付费，成本低
  - 强大的基础设施
  - 与 GCP 生态集成好
- **缺点**: 需要配置 Cloud Scheduler
- **适合**: 大规模使用

### 5. AWS Lambda ⭐⭐⭐

- **免费额度**: 每月 100 万请求免费
- **优势**: 成熟的 serverless 平台
- **缺点**: 配置复杂，需要多个服务配合
- **适合**: 已使用 AWS 生态的用户

### 6. Vercel ⭐⭐

- **免费额度**: 有限的执行时间
- **优势**: 部署简单
- **缺点**: 不支持原生 cron，需要外部触发
- **适合**: 前端项目，需要外部触发器

## 🚀 快速部署

### Railway.app (推荐)

1. Fork 这个项目到你的 GitHub
2. 注册 [Railway.app](https://railway.app/)
3. 连接 GitHub 仓库
4. 选择 `single-run` 分支
5. 设置环境变量：
   ```
   SMTP_HOST=smtp.gmail.com
   SMTP_USERNAME=your-email@gmail.com
   SMTP_PASSWORD=your-app-password
   FROM_EMAIL=your-email@gmail.com
   TO_EMAIL=recipient@example.com
   ```
6. 部署完成后，Railway 会自动按照 cron 配置执行

### Fly.io

1. 安装 flyctl CLI
2. 登录并初始化：
   ```bash
   flyctl auth login
   flyctl launch --dockerfile Dockerfile.cloud
   ```
3. 设置环境变量：
   ```bash
   flyctl secrets set SMTP_HOST=smtp.gmail.com
   flyctl secrets set SMTP_USERNAME=your-email@gmail.com
   flyctl secrets set SMTP_PASSWORD=your-app-password
   flyctl secrets set FROM_EMAIL=your-email@gmail.com
   flyctl secrets set TO_EMAIL=recipient@example.com
   ```
4. 部署：
   ```bash
   flyctl deploy
   ```

### Render.com

1. 连接 GitHub 仓库到 Render
2. 选择 "Cron Job" 服务类型
3. 使用 `Dockerfile.cloud`
4. 设置环境变量（在 Render 控制台）
5. 设置 cron 计划表达式：`*/5 * * * *`

## 📧 环境变量配置

### 必需的环境变量

```bash
# 邮件服务器配置
SMTP_HOST=smtp.gmail.com
SMTP_USERNAME=your-email@gmail.com
SMTP_PASSWORD=your-app-password  # Gmail 需要使用应用专用密码
FROM_EMAIL=your-email@gmail.com
TO_EMAIL=recipient@example.com

# 可选：Webhook 通知
FEISHU_WEBHOOK_URL=https://open.feishu.cn/open-apis/bot/v2/hook/xxx
WECHAT_WEBHOOK_URL=https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx
```

### Gmail 应用专用密码设置

1. 登录 Google 账户
2. 访问 [应用专用密码设置](https://myaccount.google.com/apppasswords)
3. 选择"邮件"应用和"其他"设备
4. 生成密码并复制
5. 在云平台设置 `SMTP_PASSWORD` 为生成的密码

## 💰 成本分析

基于每 5 分钟运行一次的频率：

| 平台          | 月运行次数 | 预估成本 | 免费额度       |
| ------------- | ---------- | -------- | -------------- |
| Railway.app   | 8,640 次   | $0-2     | $5 免费额度    |
| Fly.io        | 8,640 次   | $0-1     | 丰富免费额度   |
| Render.com    | 8,640 次   | $0       | 750 小时免费   |
| GCP Cloud Run | 8,640 次   | $0       | 200 万请求免费 |
| AWS Lambda    | 8,640 次   | $0       | 100 万请求免费 |

## 🔧 运行模式说明

### 持续运行模式 (原版)

```bash
./ta-watcher --config config.yaml
```

- 适合：VPS、本地部署
- 成本：固定成本，24/7 运行

### 单次运行模式 (云优化)

```bash
./ta-watcher --single-run --config config.yaml
```

- 适合：云函数、定时任务
- 成本：按执行次数付费，大幅节省

## 🏃‍♂️ 本地测试

测试单次运行模式：

```bash
# 构建
make build

# 测试单次运行
./bin/ta-watcher --single-run --config config.yaml

# Docker 测试
docker build -f Dockerfile.cloud -t ta-watcher-cloud .
docker run --rm -e SMTP_HOST=smtp.gmail.com ta-watcher-cloud
```

## 📊 监控和日志

大部分云平台都提供内置的日志和监控：

- **Railway**: 内置日志查看器
- **Fly.io**: `flyctl logs`
- **Render**: 控制台日志查看
- **GCP**: Cloud Logging
- **AWS**: CloudWatch

## 🔄 CI/CD 集成

所有平台都支持 GitHub 集成，代码推送自动部署：

1. 连接 GitHub 仓库
2. 选择 `single-run` 分支
3. 每次推送代码自动重新部署

## 🆘 故障排除

### 常见问题

1. **邮件发送失败**

   - 检查 Gmail 应用专用密码
   - 确认 SMTP 设置正确

2. **构建失败**

   - 确保使用 `Dockerfile.cloud`
   - 检查 Go 版本兼容性

3. **执行超时**

   - 单次运行模式通常在 30-60 秒内完成
   - 检查网络连接和 API 响应时间

4. **环境变量未设置**
   - 确保在云平台控制台设置了所有必需的环境变量
   - 注意区分大小写

## 🎯 优化建议

1. **频率调整**: 根据需要调整 cron 频率

   - 高频监控：`*/5 * * * *` (每 5 分钟)
   - 正常监控：`*/15 * * * *` (每 15 分钟)
   - 低频监控：`0 */4 * * *` (每 4 小时)

2. **资源优化**: 大部分场景下 512MB 内存足够
3. **通知优化**: 配置多种通知方式确保及时收到提醒
