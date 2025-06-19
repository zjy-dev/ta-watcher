package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// 环境变量模式匹配
var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// 全局环境变量管理器
var globalEnvManager *EnvManager

// EnvManager 环境变量管理器
type EnvManager struct {
	envVars map[string]string // 缓存的环境变量
	loaded  bool              // 是否已经加载过环境变量
}

// NewEnvManager 创建新的环境变量管理器
func NewEnvManager() *EnvManager {
	return &EnvManager{
		envVars: make(map[string]string),
		loaded:  false,
	}
}

// LoadEnvFile 加载指定的 .env 文件
func (em *EnvManager) LoadEnvFile(envFile string) error {
	// 如果文件不存在，不报错，使用系统环境变量
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		fmt.Printf("%v not exist!\n", envFile)
		em.loadSystemEnvVars()
		return nil
	}

	file, err := os.Open(envFile)
	if err != nil {
		return fmt.Errorf("failed to open env file %s: %w", envFile, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 解析 KEY=VALUE 格式
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// 移除引号
		if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
			(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
			value = value[1 : len(value)-1]
		}

		em.envVars[key] = value
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read env file %s: %w", envFile, err)
	}

	em.loaded = true
	return nil
}

// loadSystemEnvVars 加载系统环境变量
func (em *EnvManager) loadSystemEnvVars() {
	// 加载系统环境变量中与配置相关的变量
	relevantEnvKeys := []string{
		"SMTP_HOST", "SMTP_PORT", "SMTP_USERNAME", "SMTP_PASSWORD", "SMTP_TLS",
		"FROM_EMAIL", "TO_EMAIL",
		"FEISHU_WEBHOOK_URL", "FEISHU_SECRET",
		"WECHAT_WEBHOOK_URL",
		"EMAIL_INTEGRATION_TEST", "BINANCE_INTEGRATION_TEST",
		"TEST_TIMEOUT",
	}

	for _, key := range relevantEnvKeys {
		if value := os.Getenv(key); value != "" {
			em.envVars[key] = value
		}
	}

	em.loaded = true
}

// GetEnv 获取环境变量值（优先级：系统环境变量 > .env文件）
func (em *EnvManager) GetEnv(key string) string {
	if !em.loaded {
		em.loadSystemEnvVars()
	}

	// 优先检查系统环境变量（包括命令行传递的）
	if value := os.Getenv(key); value != "" {
		return value
	}

	// 然后检查.env文件中的变量
	return em.envVars[key]
}

// GetEnvWithDefault 获取环境变量值，如果不存在则返回默认值
func (em *EnvManager) GetEnvWithDefault(key, defaultValue string) string {
	if value := em.GetEnv(key); value != "" {
		return value
	}
	return defaultValue
}

// SetEnv 设置环境变量（主要用于测试）
func (em *EnvManager) SetEnv(key, value string) {
	if !em.loaded {
		em.envVars = make(map[string]string)
		em.loaded = true
	}
	em.envVars[key] = value
}

// IsIntegrationTestEnabled 检查指定类型的集成测试是否启用
func IsIntegrationTestEnabled(testType string) bool {
	envMgr := GetEnvManager()
	return envMgr.GetEnv(testType+"_INTEGRATION_TEST") == "1"
}

// GetIntegrationTestTimeout 获取集成测试超时时间
func GetIntegrationTestTimeout() string {
	envMgr := GetEnvManager()
	return envMgr.GetEnvWithDefault("TEST_TIMEOUT", "30s")
}

// ===== 环境变量展开功能 =====

// InitEnvManager 初始化环境变量管理器
func InitEnvManager(envFile string) error {
	globalEnvManager = NewEnvManager()
	return globalEnvManager.LoadEnvFile(envFile)
}

// GetEnvManager 获取环境变量管理器
func GetEnvManager() *EnvManager {
	if globalEnvManager == nil {
		globalEnvManager = NewEnvManager()
		// 自动确定环境文件
		envFile := DetermineEnvFile()
		globalEnvManager.LoadEnvFile(envFile)
	}
	return globalEnvManager
}

// expandEnvVars 递归展开配置中的环境变量
func expandEnvVars(config *Config) error {
	// 确保环境变量管理器已初始化
	envMgr := GetEnvManager()

	if err := expandEmailConfig(&config.Notifiers.Email, envMgr); err != nil {
		return fmt.Errorf("email config: %w", err)
	}

	if err := expandFeishuConfig(&config.Notifiers.Feishu, envMgr); err != nil {
		return fmt.Errorf("feishu config: %w", err)
	}

	if err := expandWechatConfig(&config.Notifiers.Wechat, envMgr); err != nil {
		return fmt.Errorf("wechat config: %w", err)
	}

	return nil
}

// expandEmailConfig 展开邮件配置中的环境变量
func expandEmailConfig(config *EmailConfig, envMgr *EnvManager) error {
	config.SMTP.Host = expandStringEnvVar(config.SMTP.Host, envMgr)
	config.SMTP.Username = expandStringEnvVar(config.SMTP.Username, envMgr)
	config.SMTP.Password = expandStringEnvVar(config.SMTP.Password, envMgr)
	config.From = expandStringEnvVar(config.From, envMgr)

	// 展开收件人列表
	for i, to := range config.To {
		config.To[i] = expandStringEnvVar(to, envMgr)
	}

	config.Subject = expandStringEnvVar(config.Subject, envMgr)
	config.Template = expandStringEnvVar(config.Template, envMgr)

	// 展开端口（如果是字符串格式的环境变量）
	if portStr := envMgr.GetEnv("SMTP_PORT"); portStr != "" && config.SMTP.Port == 0 {
		if port, err := strconv.Atoi(portStr); err == nil {
			config.SMTP.Port = port
		}
	}

	// 展开TLS设置
	if tlsStr := envMgr.GetEnv("SMTP_TLS"); tlsStr != "" {
		if tls, err := strconv.ParseBool(tlsStr); err == nil {
			config.SMTP.TLS = tls
		}
	}

	return nil
}

// expandFeishuConfig 展开飞书配置中的环境变量
func expandFeishuConfig(config *FeishuConfig, envMgr *EnvManager) error {
	config.WebhookURL = expandStringEnvVar(config.WebhookURL, envMgr)
	config.Secret = expandStringEnvVar(config.Secret, envMgr)
	config.Template = expandStringEnvVar(config.Template, envMgr)
	return nil
}

// expandWechatConfig 展开微信配置中的环境变量
func expandWechatConfig(config *WechatConfig, envMgr *EnvManager) error {
	config.WebhookURL = expandStringEnvVar(config.WebhookURL, envMgr)
	config.Template = expandStringEnvVar(config.Template, envMgr)
	return nil
}

// expandStringEnvVar 展开字符串中的环境变量
func expandStringEnvVar(value string, envMgr *EnvManager) string {
	if value == "" {
		return value
	}

	return envVarPattern.ReplaceAllStringFunc(value, func(match string) string {
		// 提取变量名（去掉 ${ 和 }）
		varName := match[2 : len(match)-1]

		// 检查是否有默认值 VAR_NAME:default_value
		parts := strings.SplitN(varName, ":", 2)
		envName := parts[0]
		defaultValue := ""
		if len(parts) == 2 {
			defaultValue = parts[1]
		}

		// 获取环境变量值
		if envValue := envMgr.GetEnv(envName); envValue != "" {
			return envValue
		}

		// 返回默认值（可能为空）
		return defaultValue
	})
}

// ===== 辅助函数 =====

// DetermineEnvFile 根据运行环境确定应该使用的 .env 文件
func DetermineEnvFile() string {
	// 查找项目根目录
	projectRoot := FindProjectRoot()
	if projectRoot == "" {
		// 如果找不到项目根目录，回退到当前目录
		projectRoot = "."
	}

	// 检查是否强制使用 .env（用于真实集成测试）
	if os.Getenv("USE_REAL_ENV") == "1" {
		envPath := filepath.Join(projectRoot, ".env")
		if _, err := os.Stat(envPath); err == nil {
			return envPath
		}
	}

	// 检查是否在测试环境中
	if isRunningTests() {
		// 集成测试优先使用 .env.example
		envExamplePath := filepath.Join(projectRoot, ".env.example")
		if _, err := os.Stat(envExamplePath); err == nil {
			return envExamplePath
		}
		// 如果没有 .env.example，使用 .env.integration
		envIntegrationPath := filepath.Join(projectRoot, ".env.integration")
		if _, err := os.Stat(envIntegrationPath); err == nil {
			return envIntegrationPath
		}
	}

	// 开发环境使用 .env
	envPath := filepath.Join(projectRoot, ".env")
	if _, err := os.Stat(envPath); err == nil {
		return envPath
	}

	// 如果都没有，返回空字符串，将使用系统环境变量
	return ""
}

// isRunningTests 检查是否在运行测试
func isRunningTests() bool {
	// 检查命令行参数或测试相关的环境变量
	for _, arg := range os.Args {
		if strings.Contains(arg, "test") || strings.Contains(arg, ".test") {
			return true
		}
	}

	// 检查测试相关的环境变量
	if os.Getenv("GO_TESTING") == "1" ||
		os.Getenv("EMAIL_INTEGRATION_TEST") == "1" ||
		os.Getenv("BINANCE_INTEGRATION_TEST") == "1" {
		return true
	}

	return false
}

// FindProjectRoot 查找项目根目录
func FindProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		// 检查是否存在 go.mod 文件
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		// 检查是否存在 config.example.yaml 文件
		if _, err := os.Stat(filepath.Join(dir, "config.example.yaml")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// 已经到达根目录
			break
		}
		dir = parent
	}

	return ""
}

// ===== 向后兼容的辅助函数 =====

// getEnvDuration 从环境变量获取时间间隔
func getEnvDuration(key string, defaultValue time.Duration, envMgr *EnvManager) time.Duration {
	if value := envMgr.GetEnv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// getEnvInt 从环境变量获取整数值
func getEnvInt(key string, defaultValue int, envMgr *EnvManager) int {
	if value := envMgr.GetEnv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvBool 从环境变量获取布尔值
func getEnvBool(key string, defaultValue bool, envMgr *EnvManager) bool {
	if value := envMgr.GetEnv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// getEnvString 从环境变量获取字符串值
func getEnvString(key string, defaultValue string, envMgr *EnvManager) string {
	if value := envMgr.GetEnv(key); value != "" {
		return value
	}
	return defaultValue
}
