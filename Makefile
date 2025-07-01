# TA Watcher Makefile
# 提供一键运行各种测试和构建任务的功能

# 项目配置
PROJECT_NAME := ta-watcher
BINARY_NAME := ta-watcher
GO_VERSION := 1.24

# 目录
CMD_DIR := ./cmd/watcher
INTERNAL_DIR := ./internal
STRATEGIES_DIR := ./strategies

# 测试配置
TEST_TIMEOUT := 5m
BENCH_TIME := 1s
COVERAGE_FILE := coverage.out
COVERAGE_HTML := coverage.html

# 颜色输出
COLOR_RESET := \033[0m
COLOR_BOLD := \033[1m
COLOR_GREEN := \033[32m
COLOR_BLUE := \033[34m
COLOR_YELLOW := \033[33m
COLOR_RED := \033[31m

.PHONY: help
help: ## 显示帮助信息
	@echo "$(COLOR_BOLD)$(PROJECT_NAME) Makefile$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BOLD)可用的命令:$(COLOR_RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_GREEN)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(COLOR_BOLD)测试命令:$(COLOR_RESET)"
	@echo "  $(COLOR_GREEN)test-all$(COLOR_RESET)             运行所有测试（单元测试 + 集成测试）"
	@echo "  $(COLOR_GREEN)test-unit$(COLOR_RESET)            只运行单元测试"
	@echo "  $(COLOR_GREEN)test-integration$(COLOR_RESET)     运行集成测试"
	@echo "  $(COLOR_GREEN)test-bench$(COLOR_RESET)           运行基准测试"
	@echo "  $(COLOR_GREEN)test-coverage$(COLOR_RESET)        运行测试并生成覆盖率报告"
	@echo ""
	@echo "$(COLOR_BOLD)构建命令:$(COLOR_RESET)"
	@echo "  $(COLOR_GREEN)build$(COLOR_RESET)               构建项目"
	@echo "  $(COLOR_GREEN)clean$(COLOR_RESET)               清理构建文件"
	@echo ""
	@echo "$(COLOR_BOLD)Docker 命令:$(COLOR_RESET)"
	@echo "  $(COLOR_GREEN)docker-build$(COLOR_RESET)        构建 Docker 镜像（依赖单元测试）"
	@echo "  $(COLOR_GREEN)compose-up$(COLOR_RESET)          启动 Docker Compose 服务"
	@echo "  $(COLOR_GREEN)compose-down$(COLOR_RESET)        停止 Docker Compose 服务"
	@echo "  $(COLOR_GREEN)compose-logs$(COLOR_RESET)        查看服务日志"
	@echo "  $(COLOR_GREEN)compose-ps$(COLOR_RESET)          查看服务状态"
	@echo "  $(COLOR_GREEN)compose-clean$(COLOR_RESET)        清理 Docker 资源"
	@echo ""
	@echo "$(COLOR_BOLD)云部署命令:$(COLOR_RESET)"
	@echo "  $(COLOR_GREEN)build-cloud$(COLOR_RESET)         构建云优化版本"
	@echo "  $(COLOR_GREEN)test-single-run$(COLOR_RESET)     测试单次运行模式"
	@echo "  $(COLOR_GREEN)docker-test-single-run$(COLOR_RESET) Docker 测试单次运行模式"
	@echo "  $(COLOR_GREEN)cloud-compose-test$(COLOR_RESET)   测试云部署 Docker Compose"

.PHONY: test-all
test-all: ## 运行所有测试（包括集成测试）
	@echo "$(COLOR_BOLD)🧪 运行所有测试...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)===============================================$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)📋 运行单元测试...$(COLOR_RESET)"
	@$(MAKE) test-unit
	@echo ""
	@echo "$(COLOR_YELLOW)🔗 运行集成测试...$(COLOR_RESET)"
	@$(MAKE) test-integration
	@echo ""
	@echo "$(COLOR_YELLOW)📊 运行覆盖率测试...$(COLOR_RESET)"
	@$(MAKE) test-coverage
	@echo ""
	@echo "$(COLOR_GREEN)✅ 所有测试完成！$(COLOR_RESET)"

.PHONY: test-unit
test-unit: ## 运行单元测试
	@echo "$(COLOR_BOLD)🧪 运行单元测试...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)===============================================$(COLOR_RESET)"
	@go test -v -timeout $(TEST_TIMEOUT) -short ./internal/...
	@echo ""
	@echo "$(COLOR_GREEN)✅ 单元测试完成！$(COLOR_RESET)"

.PHONY: test-integration
test-integration: ## 运行集成测试
	@echo "$(COLOR_BOLD)🔗 运行集成测试...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)===============================================$(COLOR_RESET)"
	@go test -v -timeout $(TEST_TIMEOUT) -tags=integration -run "Integration" ./internal/...
	@echo ""
	@echo "$(COLOR_GREEN)✅ 集成测试完成！$(COLOR_RESET)"

.PHONY: test-integration-real
test-integration-real: ## 运行真实集成测试（使用.env配置）
	@echo "$(COLOR_BOLD)🔗 运行真实集成测试（使用.env配置）...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)===============================================$(COLOR_RESET)"
	@if [ -f .env ]; then \
		echo "$(COLOR_YELLOW)📋 从 .env 文件加载真实配置...$(COLOR_RESET)"; \
		export USE_REAL_ENV=1 && \
		go test -v -timeout $(TEST_TIMEOUT) -tags=integration -run "Integration" ./internal/...; \
	else \
		echo "$(COLOR_RED)❌ 未找到 .env 文件$(COLOR_RESET)"; \
		echo "$(COLOR_YELLOW)💡 提示: 复制 .env.example 到 .env 并配置真实邮件信息$(COLOR_RESET)"; \
		echo "$(COLOR_YELLOW)   命令: cp .env.example .env$(COLOR_RESET)"; \
		exit 1; \
	fi
	@echo ""
	@echo "$(COLOR_GREEN)✅ 真实集成测试完成！$(COLOR_RESET)"

.PHONY: test-bench
test-bench: ## 运行基准测试
	@echo "$(COLOR_BOLD)⚡ 运行基准测试...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)===============================================$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)📊 运行Binance模块基准测试...$(COLOR_RESET)"
	@go test -bench=. -benchtime=$(BENCH_TIME) -benchmem ./internal/binance
	@echo ""
	@echo "$(COLOR_BLUE)📊 运行指标计算基准测试...$(COLOR_RESET)"
	@go test -bench=. -benchtime=$(BENCH_TIME) -benchmem ./internal/indicators
	@echo ""
	@echo "$(COLOR_GREEN)✅ 基准测试完成！$(COLOR_RESET)"

.PHONY: test-coverage
test-coverage: ## 运行测试并生成覆盖率报告
	@echo "$(COLOR_BOLD)📊 生成测试覆盖率报告...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)===============================================$(COLOR_RESET)"
	@go test -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./internal/...
	@go tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@go tool cover -func=$(COVERAGE_FILE) | tail -1
	@echo ""
	@echo "$(COLOR_GREEN)✅ 覆盖率报告已生成:$(COLOR_RESET)"
	@echo "  - 文本报告: $(COVERAGE_FILE)"
	@echo "  - HTML报告: $(COVERAGE_HTML)"

.PHONY: test-verbose
test-verbose: ## 详细模式运行所有单元测试
	@echo "$(COLOR_BOLD)🔍 详细模式运行测试...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)===============================================$(COLOR_RESET)"
	@go test -v -timeout $(TEST_TIMEOUT) -short -count=1 ./internal/...
	@echo ""
	@echo "$(COLOR_GREEN)✅ 详细测试完成！$(COLOR_RESET)"

# 模块特定测试（用于单独测试某个模块）
.PHONY: test-watcher
test-watcher: ## 运行 watcher 模块测试
	@echo "$(COLOR_BLUE)运行 watcher 模块测试...$(COLOR_RESET)"
	@go test -v ./internal/watcher/

.PHONY: test-strategy
test-strategy: ## 运行 strategy 模块测试
	@echo "$(COLOR_BLUE)运行 strategy 模块测试...$(COLOR_RESET)"
	@go test -v ./internal/strategy/

.PHONY: test-config
test-config: ## 运行 config 模块测试
	@echo "$(COLOR_BLUE)运行 config 模块测试...$(COLOR_RESET)"
	@go test -v ./internal/config/

.PHONY: test-assets
test-assets: ## 运行 assets 模块测试
	@echo "$(COLOR_BLUE)运行 assets 模块测试...$(COLOR_RESET)"
	@go test -v ./internal/assets/

.PHONY: benchmark
benchmark: ## 运行基准测试
	@echo "$(COLOR_BLUE)运行基准测试...$(COLOR_RESET)"
	@go test -bench=. -benchmem ./internal/watcher/

# 构建和运行
.PHONY: build
build: ## 构建应用程序
	@echo "$(COLOR_BLUE)构建 $(BINARY_NAME)...$(COLOR_RESET)"
	@go build -o bin/$(BINARY_NAME) $(CMD_DIR)
	@echo "$(COLOR_GREEN)构建完成: bin/$(BINARY_NAME)$(COLOR_RESET)"

.PHONY: run
run: ## 运行应用程序
	@echo "$(COLOR_BLUE)运行 $(BINARY_NAME)...$(COLOR_RESET)"
	@go run $(CMD_DIR) -config config.yaml -strategies $(STRATEGIES_DIR)

.PHONY: run-daemon
run-daemon: ## 后台运行应用程序
	@echo "$(COLOR_BLUE)后台运行 $(BINARY_NAME)...$(COLOR_RESET)"
	@go run $(CMD_DIR) -config config.yaml -strategies $(STRATEGIES_DIR) -daemon

.PHONY: health
health: ## 健康检查
	@echo "$(COLOR_BLUE)执行健康检查...$(COLOR_RESET)"
	@go run $(CMD_DIR) -health

.PHONY: generate-strategy
generate-strategy: ## 生成策略模板 (用法: make generate-strategy STRATEGY=my_strategy)
	@if [ -z "$(STRATEGY)" ]; then \
		echo "$(COLOR_RED)请指定策略名称: make generate-strategy STRATEGY=策略名称$(COLOR_RESET)"; \
		exit 1; \
	fi
	@echo "$(COLOR_BLUE)生成策略模板: $(STRATEGY)...$(COLOR_RESET)"
	@mkdir -p $(STRATEGIES_DIR)
	@go run $(CMD_DIR) -generate $(STRATEGY) -strategies $(STRATEGIES_DIR)

.PHONY: clean
clean: ## 清理构建文件
	@echo "$(COLOR_YELLOW)清理构建文件...$(COLOR_RESET)"
	@rm -rf bin/
	@rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)
	@echo "$(COLOR_GREEN)清理完成$(COLOR_RESET)"

# 策略相关
.PHONY: compile-strategies
compile-strategies: ## 编译自定义策略为插件
	@echo "$(COLOR_BLUE)编译策略插件...$(COLOR_RESET)"
	@if [ -d "$(STRATEGIES_DIR)" ]; then \
		for file in $(STRATEGIES_DIR)/*_strategy.go; do \
			if [ -f "$$file" ]; then \
				name=$$(basename $$file .go); \
				echo "编译策略: $$name"; \
				go build -buildmode=plugin -o $(STRATEGIES_DIR)/$$name.so $$file; \
			fi; \
		done; \
		echo "$(COLOR_GREEN)策略编译完成$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_YELLOW)策略目录不存在: $(STRATEGIES_DIR)$(COLOR_RESET)"; \
	fi

.PHONY: list-strategies
list-strategies: ## 列出策略文件
	@echo "$(COLOR_BLUE)策略文件列表:$(COLOR_RESET)"
	@if [ -d "$(STRATEGIES_DIR)" ]; then \
		ls -la $(STRATEGIES_DIR)/*.go $(STRATEGIES_DIR)/*.so 2>/dev/null || echo "没有找到策略文件"; \
	else \
		echo "策略目录不存在: $(STRATEGIES_DIR)"; \
	fi

# 开发工具
.PHONY: fmt
fmt: ## 格式化代码
	@echo "$(COLOR_BLUE)格式化代码...$(COLOR_RESET)"
	@go fmt ./...

.PHONY: vet
vet: ## 代码检查
	@echo "$(COLOR_BLUE)代码检查...$(COLOR_RESET)"
	@go vet ./...

.PHONY: mod-tidy
mod-tidy: ## 整理依赖
	@echo "$(COLOR_BLUE)整理依赖...$(COLOR_RESET)"
	@go mod tidy

.PHONY: dev-setup
dev-setup: mod-tidy ## 开发环境设置
	@echo "$(COLOR_BLUE)设置开发环境...$(COLOR_RESET)"
	@if [ ! -f "config.yaml" ] && [ -f "config.example.yaml" ]; then \
		cp config.example.yaml config.yaml; \
		echo "$(COLOR_GREEN)已复制配置文件模板$(COLOR_RESET)"; \
	fi
	@mkdir -p $(STRATEGIES_DIR)
	@mkdir -p bin/
	@echo "$(COLOR_GREEN)开发环境设置完成$(COLOR_RESET)"

.PHONY: quick-start
quick-start: dev-setup build ## 快速开始 (设置环境并运行)
	@echo "$(COLOR_GREEN)快速开始 TA Watcher...$(COLOR_RESET)"
	@./bin/$(BINARY_NAME) -config config.yaml -strategies $(STRATEGIES_DIR)

.PHONY: ci
ci: deps check test-coverage ## CI流水线（依赖、检查、覆盖率测试）
	@echo ""
	@echo "$(COLOR_GREEN)✅ CI流水线完成！$(COLOR_RESET)"

.PHONY: check
check: fmt vet test-unit ## 运行所有检查（格式化、vet、单元测试）
	@echo ""
	@echo "$(COLOR_GREEN)✅ 所有检查完成！$(COLOR_RESET)"

.PHONY: deps
deps: ## 安装和更新依赖
	@echo "$(COLOR_BOLD)📦 管理依赖...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)===============================================$(COLOR_RESET)"
	@go mod download
	@go mod tidy
	@echo "$(COLOR_GREEN)✅ 依赖管理完成！$(COLOR_RESET)"

.PHONY: deps-update
deps-update: ## 更新所有依赖到最新版本
	@echo "$(COLOR_BOLD)📦 更新依赖...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)===============================================$(COLOR_RESET)"
	@go get -u ./...
	@go mod tidy
	@echo "$(COLOR_GREEN)✅ 依赖更新完成！$(COLOR_RESET)"

# ===== Docker 相关命令 =====

.PHONY: docker-build
docker-build: test-unit ## 构建 Docker 镜像（依赖单元测试）
	@echo "$(COLOR_BOLD)🐳 构建 Docker 镜像...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)===============================================$(COLOR_RESET)"
	@docker build -t $(PROJECT_NAME):latest .
	@echo "$(COLOR_GREEN)✅ Docker 镜像构建完成！$(COLOR_RESET)"

.PHONY: compose-up
compose-up: ## 使用 Docker Compose 启动服务
	@echo "$(COLOR_BOLD)🐳 启动 Docker Compose 服务...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)===============================================$(COLOR_RESET)"
	@if [ ! -f ".env" ] && [ -f ".env.example" ]; then \
		echo "$(COLOR_YELLOW)⚠️  .env 文件不存在，复制 .env.example 到 .env$(COLOR_RESET)"; \
		cp .env.example .env; \
		echo "$(COLOR_YELLOW)⚠️  请编辑 .env 文件配置环境变量$(COLOR_RESET)"; \
		echo "$(COLOR_YELLOW)⚠️  配置完成后请重新运行此命令$(COLOR_RESET)"; \
		exit 1; \
	fi
	@if [ ! -f "config.yaml" ] && [ -f "config.example.yaml" ]; then \
		cp config.example.yaml config.yaml; \
		echo "$(COLOR_GREEN)✅ 已复制配置文件模板$(COLOR_RESET)"; \
	fi
	@docker-compose up -d --build
	@echo "$(COLOR_GREEN)✅ Docker Compose 服务已启动！$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BOLD)有用的命令:$(COLOR_RESET)"
	@echo "  make compose-logs     - 查看实时日志"
	@echo "  make compose-ps       - 查看服务状态"
	@echo "  make compose-down     - 停止服务"

.PHONY: compose-down
compose-down: ## 停止 Docker Compose 服务
	@echo "$(COLOR_BOLD)🐳 停止 Docker Compose 服务...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)===============================================$(COLOR_RESET)"
	@docker-compose down
	@echo "$(COLOR_GREEN)✅ Docker Compose 服务已停止！$(COLOR_RESET)"

.PHONY: compose-logs
compose-logs: ## 查看 Docker Compose 日志
	@echo "$(COLOR_BOLD)🐳 查看 Docker Compose 日志...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)===============================================$(COLOR_RESET)"
	@docker-compose logs -f

.PHONY: compose-ps
compose-ps: ## 查看 Docker Compose 服务状态
	@echo "$(COLOR_BOLD)🐳 查看 Docker Compose 服务状态...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)===============================================$(COLOR_RESET)"
	@docker-compose ps

.PHONY: compose-restart
compose-restart: ## 重启 Docker Compose 服务
	@echo "$(COLOR_BOLD)🐳 重启 Docker Compose 服务...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)===============================================$(COLOR_RESET)"
	@docker-compose restart
	@echo "$(COLOR_GREEN)✅ Docker Compose 服务已重启！$(COLOR_RESET)"

.PHONY: compose-clean
compose-clean: ## 清理 Docker Compose 资源
	@echo "$(COLOR_BOLD)🐳 清理 Docker 资源...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)===============================================$(COLOR_RESET)"
	@docker-compose down -v --remove-orphans || true
	@docker system prune -f
	@echo "$(COLOR_GREEN)✅ Docker 资源清理完成！$(COLOR_RESET)"

# 云部署相关命令
.PHONY: build-cloud
build-cloud: ## 构建云优化版本
	@echo "$(COLOR_BOLD)☁️ 构建云部署版本...$(COLOR_RESET)"
	@docker build -f Dockerfile.cloud -t ta-watcher-cloud:latest .
	@echo "$(COLOR_GREEN)✅ 云版本构建完成！$(COLOR_RESET)"

.PHONY: docker-test-single-run
docker-test-single-run: build-cloud ## Docker 测试单次运行模式
	@echo "$(COLOR_BOLD)🐳 Docker 单次运行测试...$(COLOR_RESET)"
	@docker run --rm -v $(PWD)/config.yaml:/config.yaml ta-watcher-cloud:latest
	@echo "$(COLOR_GREEN)✅ Docker 单次运行测试完成！$(COLOR_RESET)"

.PHONY: cloud-compose-test
cloud-compose-test: ## 测试云部署 Docker Compose
	@echo "$(COLOR_BOLD)🌩️ 测试云部署配置...$(COLOR_RESET)"
	@docker-compose -f docker-compose.cloud.yml run --rm ta-watcher-cron
	@echo "$(COLOR_GREEN)✅ 云部署测试完成！$(COLOR_RESET)"

# 默认目标
.DEFAULT_GOAL := help
