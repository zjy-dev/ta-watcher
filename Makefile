# TA Watcher Makefile
# 提供一键运行各种测试和构建任务的功能

# 项目配置
PROJECT_NAME := ta-watcher
BINARY_NAME := ta-watcher
GO_VERSION := 1.21

# 目录
CMD_DIR := ./cmd
INTERNAL_DIR := ./internal
EXAMPLES_DIR := ./examples

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
	@echo "  $(COLOR_GREEN)test-integration$(COLOR_RESET)     只运行集成测试"
	@echo "  $(COLOR_GREEN)test-bench$(COLOR_RESET)           运行基准测试"
	@echo "  $(COLOR_GREEN)test-coverage$(COLOR_RESET)        运行测试并生成覆盖率报告"
	@echo ""
	@echo "$(COLOR_BOLD)构建命令:$(COLOR_RESET)"
	@echo "  $(COLOR_GREEN)build$(COLOR_RESET)               构建项目"
	@echo "  $(COLOR_GREEN)clean$(COLOR_RESET)               清理构建文件"

.PHONY: test-all
test-all: ## 运行所有测试（包括集成测试）
	@echo "$(COLOR_BOLD)🧪 运行所有测试...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)===============================================$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)📋 运行单元测试...$(COLOR_RESET)"
	@$(MAKE) test-unit
	@echo ""
	@echo "$(COLOR_YELLOW)🔗 运行集成测试...$(COLOR_RESET)"
	@BINANCE_INTEGRATION_TEST=1 EMAIL_INTEGRATION_TEST=1 $(MAKE) test-integration
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
	@echo "$(COLOR_YELLOW)⚠️  注意: 集成测试需要设置相应的环境变量$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)   - Binance API测试: BINANCE_INTEGRATION_TEST=1$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)   - 邮件测试: EMAIL_INTEGRATION_TEST=1 + 邮件配置$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BLUE)🔍 运行Binance集成测试...$(COLOR_RESET)"
	@if [ "$$BINANCE_INTEGRATION_TEST" = "1" ]; then \
		go test -v -timeout $(TEST_TIMEOUT) -tags=integration ./internal/binance/...; \
	else \
		echo "$(COLOR_YELLOW)⏭️  跳过Binance集成测试 (设置 BINANCE_INTEGRATION_TEST=1 启用)$(COLOR_RESET)"; \
	fi
	@echo ""
	@echo "$(COLOR_BLUE)📧 运行邮件集成测试...$(COLOR_RESET)"
	@if [ "$$EMAIL_INTEGRATION_TEST" = "1" ]; then \
		go test -v -timeout $(TEST_TIMEOUT) -tags=integration -run ".*Integration.*" ./internal/notifiers/...; \
	else \
		echo "$(COLOR_YELLOW)⏭️  跳过邮件集成测试 (设置 EMAIL_INTEGRATION_TEST=1 启用)$(COLOR_RESET)"; \
	fi
	@echo ""
	@echo "$(COLOR_GREEN)✅ 集成测试完成！$(COLOR_RESET)"

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

.PHONY: test-quick
test-quick: ## 快速测试（仅运行关键测试）
	@echo "$(COLOR_BOLD)⚡ 快速测试...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)===============================================$(COLOR_RESET)"
	@go test -short -timeout 30s ./internal/config ./internal/binance ./internal/notifiers
	@echo ""
	@echo "$(COLOR_GREEN)✅ 快速测试完成！$(COLOR_RESET)"

.PHONY: test-verbose
test-verbose: ## 详细模式运行所有单元测试
	@echo "$(COLOR_BOLD)🔍 详细模式运行测试...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)===============================================$(COLOR_RESET)"
	@go test -v -timeout $(TEST_TIMEOUT) -short -count=1 ./internal/...
	@echo ""
	@echo "$(COLOR_GREEN)✅ 详细测试完成！$(COLOR_RESET)"

.PHONY: build
build: ## 构建项目
	@echo "$(COLOR_BOLD)🔨 构建项目...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)===============================================$(COLOR_RESET)"
	@go mod tidy
	@go mod verify
	@if [ -d "$(CMD_DIR)" ]; then \
		go build -o bin/$(BINARY_NAME) $(CMD_DIR)/...; \
		echo "$(COLOR_GREEN)✅ 构建完成: bin/$(BINARY_NAME)$(COLOR_RESET)"; \
	else \
		go build ./...; \
		echo "$(COLOR_GREEN)✅ 库构建完成$(COLOR_RESET)"; \
	fi

.PHONY: clean
clean: ## 清理构建文件和测试文件
	@echo "$(COLOR_BOLD)🧹 清理文件...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)===============================================$(COLOR_RESET)"
	@rm -rf bin/
	@rm -f $(COVERAGE_FILE)
	@rm -f $(COVERAGE_HTML)
	@go clean ./...
	@echo "$(COLOR_GREEN)✅ 清理完成！$(COLOR_RESET)"

.PHONY: fmt
fmt: ## 格式化代码
	@echo "$(COLOR_BOLD)✨ 格式化代码...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)===============================================$(COLOR_RESET)"
	@go fmt ./...
	@echo "$(COLOR_GREEN)✅ 代码格式化完成！$(COLOR_RESET)"

.PHONY: lint
lint: ## 运行代码检查
	@echo "$(COLOR_BOLD)🔍 运行代码检查...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)===============================================$(COLOR_RESET)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
		echo "$(COLOR_GREEN)✅ 代码检查完成！$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_YELLOW)⚠️  golangci-lint 未安装，跳过代码检查$(COLOR_RESET)"; \
		echo "$(COLOR_YELLOW)   安装方法: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest$(COLOR_RESET)"; \
	fi

.PHONY: vet
vet: ## 运行 go vet 检查
	@echo "$(COLOR_BOLD)🔍 运行 go vet 检查...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)===============================================$(COLOR_RESET)"
	@go vet ./...
	@echo "$(COLOR_GREEN)✅ go vet 检查完成！$(COLOR_RESET)"

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

.PHONY: check
check: fmt vet test-unit ## 运行所有检查（格式化、vet、单元测试）
	@echo ""
	@echo "$(COLOR_GREEN)✅ 所有检查完成！$(COLOR_RESET)"

.PHONY: ci
ci: deps check test-coverage ## CI流水线（依赖、检查、覆盖率测试）
	@echo ""
	@echo "$(COLOR_GREEN)✅ CI流水线完成！$(COLOR_RESET)"

.PHONY: dev-setup
dev-setup: ## 开发环境设置
	@echo "$(COLOR_BOLD)🛠️  设置开发环境...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)===============================================$(COLOR_RESET)"
	@go mod download
	@echo "$(COLOR_YELLOW)💡 推荐安装的工具:$(COLOR_RESET)"
	@echo "  - golangci-lint: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
	@echo "  - gofumpt: go install mvdan.cc/gofumpt@latest"
	@echo ""
	@echo "$(COLOR_YELLOW)🧪 运行集成测试需要设置环境变量:$(COLOR_RESET)"
	@echo "  - BINANCE_INTEGRATION_TEST=1 (启用Binance API测试)"
	@echo "  - EMAIL_INTEGRATION_TEST=1 (启用邮件测试)"
	@echo "  - EMAIL_SMTP_HOST, EMAIL_SMTP_PORT, EMAIL_USERNAME, EMAIL_PASSWORD (邮件配置)"
	@echo ""
	@echo "$(COLOR_GREEN)✅ 开发环境设置完成！$(COLOR_RESET)"

.PHONY: examples
examples: ## 运行示例代码
	@echo "$(COLOR_BOLD)📚 运行示例...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)===============================================$(COLOR_RESET)"
	@if [ -d "$(EXAMPLES_DIR)" ]; then \
		for example in $(EXAMPLES_DIR)/*.go; do \
			if [ -f "$$example" ]; then \
				echo "$(COLOR_YELLOW)运行示例: $$example$(COLOR_RESET)"; \
				go run "$$example"; \
				echo ""; \
			fi; \
		done; \
		echo "$(COLOR_GREEN)✅ 示例运行完成！$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_YELLOW)⚠️  未找到示例目录$(COLOR_RESET)"; \
	fi

# 默认目标
.DEFAULT_GOAL := help
