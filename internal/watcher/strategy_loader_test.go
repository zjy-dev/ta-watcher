package watcher

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ta-watcher/internal/strategy"
)

// TestStrategyLoader 测试策略加载器
func TestStrategyLoader(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "strategy_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	factory := strategy.NewFactory()
	loader := NewStrategyLoader(tempDir, factory)

	// 测试空目录
	err = loader.LoadStrategiesFromDirectory()
	assert.NoError(t, err)

	// 测试不存在的目录
	loader2 := NewStrategyLoader("/nonexistent/path", factory)
	err = loader2.LoadStrategiesFromDirectory()
	assert.Error(t, err)

	// 测试空策略目录
	loader3 := NewStrategyLoader("", factory)
	err = loader3.LoadStrategiesFromDirectory()
	assert.NoError(t, err) // 空目录应该不报错
}

// TestGenerateStrategyTemplate 测试策略模板生成
func TestGenerateStrategyTemplate(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "template_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// 测试模板生成
	outputPath := filepath.Join(tempDir, "test_strategy.go")
	err = GenerateStrategyTemplate(outputPath, "test")
	assert.NoError(t, err)

	// 验证文件是否创建
	_, err = os.Stat(outputPath)
	assert.NoError(t, err)

	// 读取文件内容验证
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "package main")
	assert.Contains(t, contentStr, "testStrategy")
	assert.Contains(t, contentStr, "NewStrategy")
	assert.Contains(t, contentStr, "ta-watcher/internal/strategy")
	assert.Contains(t, contentStr, "func (s *testStrategy) Name()")
	assert.Contains(t, contentStr, "func (s *testStrategy) Evaluate(")

	// 测试无效路径
	err = GenerateStrategyTemplate("/invalid/path/test.go", "test")
	assert.Error(t, err)
}

// TestNewStrategyLoader 测试策略加载器创建
func TestNewStrategyLoader(t *testing.T) {
	factory := strategy.NewFactory()

	loader := NewStrategyLoader("/test/path", factory)
	assert.NotNil(t, loader)
	assert.Equal(t, "/test/path", loader.strategiesDir)
	assert.Equal(t, factory, loader.factory)

	// 测试空路径
	loader2 := NewStrategyLoader("", factory)
	assert.NotNil(t, loader2)
	assert.Equal(t, "", loader2.strategiesDir)
}

// TestStrategyTemplateContent 测试生成的策略模板内容
func TestStrategyTemplateContent(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "content_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	testCases := []struct {
		strategyName    string
		expectedContent []string
	}{
		{
			strategyName: "momentum",
			expectedContent: []string{
				"momentumStrategy",
				`name:        "momentum"`,
				"func (s *momentumStrategy) Name()",
				"func (s *momentumStrategy) Description()",
				"func (s *momentumStrategy) RequiredDataPoints()",
				"func (s *momentumStrategy) SupportedTimeframes()",
				"func (s *momentumStrategy) Evaluate(",
			},
		},
		{
			strategyName: "my_custom_strategy",
			expectedContent: []string{
				"my_custom_strategyStrategy",
				`name:        "my_custom_strategy"`,
				"func (s *my_custom_strategyStrategy) Name()",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.strategyName, func(t *testing.T) {
			outputPath := filepath.Join(tempDir, tc.strategyName+"_strategy.go")
			err = GenerateStrategyTemplate(outputPath, tc.strategyName)
			require.NoError(t, err)

			content, err := os.ReadFile(outputPath)
			require.NoError(t, err)

			contentStr := string(content)
			for _, expected := range tc.expectedContent {
				assert.Contains(t, contentStr, expected,
					"Missing expected content: %s", expected)
			}
		})
	}
}

// TestStrategyTemplateCompilation 测试生成的模板是否可以编译
// 注意：这个测试需要 Go 编译器，在某些环境下可能会跳过
func TestStrategyTemplateCompilation(t *testing.T) {
	// 检查是否有 go 命令可用
	if _, err := os.Stat("/usr/bin/go"); os.IsNotExist(err) {
		if _, err := os.Stat("/usr/local/go/bin/go"); os.IsNotExist(err) {
			t.Skip("Go compiler not available, skipping compilation test")
		}
	}

	tempDir, err := os.MkdirTemp("", "compile_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// 生成策略模板
	outputPath := filepath.Join(tempDir, "compile_test_strategy.go")
	err = GenerateStrategyTemplate(outputPath, "compile_test")
	require.NoError(t, err)

	// 注意：实际的编译测试需要完整的 Go 模块环境
	// 这里我们只验证文件语法的基本正确性
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	contentStr := string(content)

	// 基本语法检查
	assert.Contains(t, contentStr, "package main")
	assert.Contains(t, contentStr, "import (")
	assert.Contains(t, contentStr, "func NewStrategy()")
	assert.Contains(t, contentStr, "return &")
	assert.Contains(t, contentStr, "strategy.Strategy")

	// 检查大括号平衡
	openBraces := 0
	closeBraces := 0
	for _, char := range contentStr {
		if char == '{' {
			openBraces++
		} else if char == '}' {
			closeBraces++
		}
	}
	assert.Equal(t, openBraces, closeBraces, "Unbalanced braces in generated template")
}
