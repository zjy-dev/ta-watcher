package indicators

import (
	"math"
	"testing"
)

// 测试数据：模拟价格序列
var testPrices = []float64{
	44.5, 44.2, 44.4, 44.9, 44.5, 44.6, 44.8, 44.2, 44.6, 44.8,
	45.1, 45.3, 45.5, 45.4, 45.2, 45.4, 45.6, 45.8, 46.0, 46.0,
	46.2, 46.4, 46.6, 46.8, 47.0, 47.2, 47.4, 47.6, 47.8, 48.0,
}

func TestCalculateSMA(t *testing.T) {
	tests := []struct {
		name     string
		prices   []float64
		period   int
		wantErr  bool
		expected float64 // 期望的第一个SMA值
	}{
		{
			name:     "正常计算SMA-5周期",
			prices:   testPrices,
			period:   5,
			wantErr:  false,
			expected: 44.5, // (44.5+44.2+44.4+44.9+44.5)/5 = 44.5
		},
		{
			name:    "价格数据不足",
			prices:  []float64{1.0, 2.0},
			period:  5,
			wantErr: true,
		},
		{
			name:    "周期为0",
			prices:  testPrices,
			period:  0,
			wantErr: true,
		},
		{
			name:    "周期为负数",
			prices:  testPrices,
			period:  -1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CalculateSMA(tt.prices, tt.period)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CalculateSMA() 期望错误，但没有返回错误")
				}
				return
			}

			if err != nil {
				t.Errorf("CalculateSMA() 意外错误 = %v", err)
				return
			}

			if result == nil {
				t.Errorf("CalculateSMA() 返回nil结果")
				return
			}

			if len(result.Values) == 0 {
				t.Errorf("CalculateSMA() 返回空值序列")
				return
			}

			// 检查第一个计算值
			if math.Abs(result.Values[0]-tt.expected) > 0.001 {
				t.Errorf("CalculateSMA() 第一个值 = %v, 期望 %v", result.Values[0], tt.expected)
			}

			// 检查结果长度
			expectedLength := len(tt.prices) - tt.period + 1
			if len(result.Values) != expectedLength {
				t.Errorf("CalculateSMA() 结果长度 = %v, 期望 %v", len(result.Values), expectedLength)
			}

			// 检查属性
			if result.Period != tt.period {
				t.Errorf("CalculateSMA() 周期 = %v, 期望 %v", result.Period, tt.period)
			}

			if result.Type != SMA {
				t.Errorf("CalculateSMA() 类型 = %v, 期望 %v", result.Type, SMA)
			}
		})
	}
}

func TestCalculateEMA(t *testing.T) {
	tests := []struct {
		name    string
		prices  []float64
		period  int
		wantErr bool
	}{
		{
			name:    "正常计算EMA-5周期",
			prices:  testPrices,
			period:  5,
			wantErr: false,
		},
		{
			name:    "正常计算EMA-10周期",
			prices:  testPrices,
			period:  10,
			wantErr: false,
		},
		{
			name:    "价格数据不足",
			prices:  []float64{1.0, 2.0},
			period:  5,
			wantErr: true,
		},
		{
			name:    "周期为0",
			prices:  testPrices,
			period:  0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CalculateEMA(tt.prices, tt.period)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CalculateEMA() 期望错误，但没有返回错误")
				}
				return
			}

			if err != nil {
				t.Errorf("CalculateEMA() 意外错误 = %v", err)
				return
			}

			if result == nil || len(result.Values) == 0 {
				t.Errorf("CalculateEMA() 返回空结果")
				return
			}

			// EMA的第一个值应该等于前period个价格的SMA
			firstSMA := 0.0
			for i := 0; i < tt.period; i++ {
				firstSMA += tt.prices[i]
			}
			firstSMA /= float64(tt.period)

			if math.Abs(result.Values[0]-firstSMA) > 0.001 {
				t.Errorf("CalculateEMA() 第一个值 = %v, 期望 %v", result.Values[0], firstSMA)
			}

			// 检查结果长度
			expectedLength := len(tt.prices) - tt.period + 1
			if len(result.Values) != expectedLength {
				t.Errorf("CalculateEMA() 结果长度 = %v, 期望 %v", len(result.Values), expectedLength)
			}

			if result.Type != EMA {
				t.Errorf("CalculateEMA() 类型 = %v, 期望 %v", result.Type, EMA)
			}
		})
	}
}

func TestCalculateWMA(t *testing.T) {
	tests := []struct {
		name    string
		prices  []float64
		period  int
		wantErr bool
	}{
		{
			name:    "正常计算WMA-5周期",
			prices:  testPrices,
			period:  5,
			wantErr: false,
		},
		{
			name:    "价格数据不足",
			prices:  []float64{1.0, 2.0},
			period:  5,
			wantErr: true,
		},
		{
			name:    "周期为0",
			prices:  testPrices,
			period:  0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CalculateWMA(tt.prices, tt.period)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CalculateWMA() 期望错误，但没有返回错误")
				}
				return
			}

			if err != nil {
				t.Errorf("CalculateWMA() 意外错误 = %v", err)
				return
			}

			if result == nil || len(result.Values) == 0 {
				t.Errorf("CalculateWMA() 返回空结果")
				return
			}

			// 检查结果长度
			expectedLength := len(tt.prices) - tt.period + 1
			if len(result.Values) != expectedLength {
				t.Errorf("CalculateWMA() 结果长度 = %v, 期望 %v", len(result.Values), expectedLength)
			}

			if result.Type != WMA {
				t.Errorf("CalculateWMA() 类型 = %v, 期望 %v", result.Type, WMA)
			}
		})
	}
}

func TestMAResult_GetLatest(t *testing.T) {
	result, _ := CalculateSMA(testPrices, 5)

	latest := result.GetLatest()
	expected := result.Values[len(result.Values)-1]

	if latest != expected {
		t.Errorf("GetLatest() = %v, 期望 %v", latest, expected)
	}
}

func TestMAResult_GetLatestN(t *testing.T) {
	result, _ := CalculateSMA(testPrices, 5)

	tests := []struct {
		name string
		n    int
		want int
	}{
		{"获取最新3个值", 3, 3},
		{"获取所有值", len(result.Values), len(result.Values)},
		{"获取超过可用数量的值", len(result.Values) + 5, len(result.Values)},
		{"n为0", 0, 0},
		{"n为负数", -1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := result.GetLatestN(tt.n)
			if len(got) != tt.want {
				t.Errorf("GetLatestN(%v) 长度 = %v, 期望 %v", tt.n, len(got), tt.want)
			}
		})
	}
}

func TestIsGoldenCross(t *testing.T) {
	// 创建测试数据：短期MA从下方穿越长期MA
	// 前一个值：短期MA <= 长期MA，当前值：短期MA > 长期MA
	shortMAValues := []float64{10.0, 10.1, 10.2, 10.1, 10.3}
	longMAValues := []float64{10.2, 10.2, 10.2, 10.2, 10.2}

	shortMA := &MAResult{Values: shortMAValues, Period: 5, Type: SMA}
	longMA := &MAResult{Values: longMAValues, Period: 10, Type: SMA}

	// 应该检测到金叉
	if !IsGoldenCross(shortMA, longMA) {
		t.Errorf("IsGoldenCross() = false, 期望 true")
	}
}

func TestIsDeathCross(t *testing.T) {
	// 创建测试数据：短期MA从上方穿越长期MA
	shortMAValues := []float64{10.5, 10.4, 10.3, 10.2, 10.0}
	longMAValues := []float64{10.2, 10.2, 10.2, 10.2, 10.2}

	shortMA := &MAResult{Values: shortMAValues, Period: 5, Type: SMA}
	longMA := &MAResult{Values: longMAValues, Period: 10, Type: SMA}

	// 应该检测到死叉
	if !IsDeathCross(shortMA, longMA) {
		t.Errorf("IsDeathCross() = false, 期望 true")
	}
}

func TestCrossDetectionEdgeCases(t *testing.T) {
	// 测试边界情况

	// 空数据
	emptyMA := &MAResult{Values: []float64{}, Period: 5, Type: SMA}
	normalMA := &MAResult{Values: []float64{10.0, 10.1}, Period: 5, Type: SMA}

	if IsGoldenCross(emptyMA, normalMA) {
		t.Errorf("IsGoldenCross() 空数据应该返回 false")
	}

	if IsDeathCross(emptyMA, normalMA) {
		t.Errorf("IsDeathCross() 空数据应该返回 false")
	}

	// 只有一个数据点
	singleMA := &MAResult{Values: []float64{10.0}, Period: 5, Type: SMA}

	if IsGoldenCross(singleMA, normalMA) {
		t.Errorf("IsGoldenCross() 单个数据点应该返回 false")
	}
}

// 基准测试
func BenchmarkCalculateSMA(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CalculateSMA(testPrices, 5)
	}
}

func BenchmarkCalculateEMA(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CalculateEMA(testPrices, 5)
	}
}

func BenchmarkCalculateWMA(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CalculateWMA(testPrices, 5)
	}
}
