package main

import (
	"fmt"
	"ta-watcher/internal/strategy"
)

func main() {
	factory := strategy.NewFactory()

	// 测试不带参数的RSI策略创建（应该使用新的默认值）
	rsiStrategy, err := factory.CreateStrategy("rsi")
	if err != nil {
		fmt.Printf("Error creating RSI strategy: %v\n", err)
		return
	}

	fmt.Printf("Default RSI strategy name: %s\n", rsiStrategy.Name())
	fmt.Printf("Default RSI strategy description: %s\n", rsiStrategy.Description())

	// 测试带参数的RSI策略创建
	rsiCustom, err := factory.CreateStrategy("rsi", 21, 75.0, 25.0)
	if err != nil {
		fmt.Printf("Error creating custom RSI strategy: %v\n", err)
		return
	}

	fmt.Printf("Custom RSI strategy name: %s\n", rsiCustom.Name())
	fmt.Printf("Custom RSI strategy description: %s\n", rsiCustom.Description())
}
