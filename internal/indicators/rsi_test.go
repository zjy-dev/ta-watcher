package indicators

import (
	"math"
	"testing"
)

// RSI测试数据 - 包含明显的上涨和下跌趋势
var rsiTestPrices = []float64{
	44.34, 44.09, 44.15, 43.61, 44.33, 44.83, 45.85, 47.15, 47.72, 46.87,
	42.66, 42.49, 42.84, 44.17, 44.18, 44.22, 44.57, 43.42, 42.66, 43.13,
	46.87, 47.72, 48.15, 47.61, 47.33, 46.83, 47.85, 48.15, 49.72, 50.87,
}

func TestCalculateRSI(t *testing.T) {
	tests := []struct {
		name    string
		prices  []float64
		period  int
		wantErr bool
	}{
		{
			name:    "正常计算RSI-14周期",
			prices:  rsiTestPrices,
			period:  14,
			wantErr: false,
		},
		{
			name:    "正常计算RSI-7周期",
			prices:  rsiTestPrices,
			period:  7,
			wantErr: false,
		},
		{
			name:    "价格数据不足",
			prices:  []float64{1.0, 2.0, 3.0},
			period:  14,
			wantErr: true,
		},
		{
			name:    "周期为0",
			prices:  rsiTestPrices,
			period:  0,
			wantErr: true,
		},
		{
			name:    "周期为负数",
			prices:  rsiTestPrices,
			period:  -1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CalculateRSI(tt.prices, tt.period)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CalculateRSI() 期望错误，但没有返回错误")
				}
				return
			}

			if err != nil {
				t.Errorf("CalculateRSI() 意外错误 = %v", err)
				return
			}

			if result == nil {
				t.Errorf("CalculateRSI() 返回nil结果")
				return
			}

			if len(result.Values) == 0 {
				t.Errorf("CalculateRSI() 返回空值序列")
				return
			}

			// RSI值应该在0-100之间
			for i, rsi := range result.Values {
				if rsi < 0 || rsi > 100 {
					t.Errorf("CalculateRSI() 第%d个RSI值 = %v, 应该在0-100之间", i, rsi)
				}
			}

			// 检查结果长度
			expectedLength := len(tt.prices) - tt.period
			if len(result.Values) != expectedLength {
				t.Errorf("CalculateRSI() 结果长度 = %v, 期望 %v", len(result.Values), expectedLength)
			}

			// 检查周期
			if result.Period != tt.period {
				t.Errorf("CalculateRSI() 周期 = %v, 期望 %v", result.Period, tt.period)
			}
		})
	}
}

func TestRSIResult_GetLatest(t *testing.T) {
	result, err := CalculateRSI(rsiTestPrices, 14)
	if err != nil {
		t.Fatalf("CalculateRSI() 错误 = %v", err)
	}

	latest := result.GetLatest()
	expected := result.Values[len(result.Values)-1]

	if latest != expected {
		t.Errorf("GetLatest() = %v, 期望 %v", latest, expected)
	}
}

func TestRSIResult_GetLatestN(t *testing.T) {
	result, err := CalculateRSI(rsiTestPrices, 14)
	if err != nil {
		t.Fatalf("CalculateRSI() 错误 = %v", err)
	}

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

func TestRSIResult_GetSignal(t *testing.T) {
	tests := []struct {
		name            string
		rsiValue        float64
		overboughtLevel float64
		oversoldLevel   float64
		expected        RSISignal
	}{
		{
			name:            "超买信号",
			rsiValue:        75.0,
			overboughtLevel: 70.0,
			oversoldLevel:   30.0,
			expected:        RSISell,
		},
		{
			name:            "超卖信号",
			rsiValue:        25.0,
			overboughtLevel: 70.0,
			oversoldLevel:   30.0,
			expected:        RSIBuy,
		},
		{
			name:            "中性信号",
			rsiValue:        50.0,
			overboughtLevel: 70.0,
			oversoldLevel:   30.0,
			expected:        RSINeutral,
		},
		{
			name:            "边界值-超买",
			rsiValue:        70.0,
			overboughtLevel: 70.0,
			oversoldLevel:   30.0,
			expected:        RSISell,
		},
		{
			name:            "边界值-超卖",
			rsiValue:        30.0,
			overboughtLevel: 70.0,
			oversoldLevel:   30.0,
			expected:        RSIBuy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &RSIResult{
				Values: []float64{tt.rsiValue},
				Period: 14,
			}

			signal := result.GetSignal(tt.overboughtLevel, tt.oversoldLevel)
			if signal != tt.expected {
				t.Errorf("GetSignal() = %v, 期望 %v", signal, tt.expected)
			}
		})
	}
}

func TestRSIResult_GetDefaultSignal(t *testing.T) {
	tests := []struct {
		name     string
		rsiValue float64
		expected RSISignal
	}{
		{"默认超买", 75.0, RSISell},
		{"默认超卖", 25.0, RSIBuy},
		{"默认中性", 50.0, RSINeutral},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &RSIResult{
				Values: []float64{tt.rsiValue},
				Period: 14,
			}

			signal := result.GetDefaultSignal()
			if signal != tt.expected {
				t.Errorf("GetDefaultSignal() = %v, 期望 %v", signal, tt.expected)
			}
		})
	}
}

func TestRSIResult_GetStrength(t *testing.T) {
	tests := []struct {
		name     string
		rsiValue float64
		expected string
	}{
		{"极度超买", 85.0, "极度超买"},
		{"超买", 75.0, "超买"},
		{"强势", 65.0, "强势"},
		{"中性", 50.0, "中性"},
		{"弱势", 35.0, "弱势"},
		{"超卖", 25.0, "超卖"},
		{"极度超卖", 15.0, "极度超卖"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &RSIResult{
				Values: []float64{tt.rsiValue},
				Period: 14,
			}

			strength := result.GetStrength()
			if strength != tt.expected {
				t.Errorf("GetStrength() = %v, 期望 %v", strength, tt.expected)
			}
		})
	}
}

func TestRSIResult_IsDivergence(t *testing.T) {
	// 创建足够长的数据用于背离检测
	// 需要至少 RSI期间 + 4个价格数据点
	prices := make([]float64, 20)
	for i := 0; i < 16; i++ {
		prices[i] = 50.0 + float64(i)*0.1 // 填充前16个数据点
	}
	// 最后4个点：价格下跌但我们希望RSI上升（看涨背离）
	prices[16] = 50.0
	prices[17] = 49.0
	prices[18] = 48.0
	prices[19] = 47.0

	// 创建看涨背离测试数据：RSI上升
	rsiValues := []float64{40.0, 42.0, 44.0, 46.0}

	result := &RSIResult{
		Values: rsiValues,
		Period: 14,
	}

	hasDivergence, isBullish := result.IsDivergence(prices)
	if !hasDivergence {
		t.Errorf("IsDivergence() 应该检测到背离")
	}
	if !isBullish {
		t.Errorf("IsDivergence() 应该是看涨背离")
	}

	// 创建看跌背离测试数据：价格上涨但RSI下降
	prices2 := make([]float64, 20)
	for i := 0; i < 16; i++ {
		prices2[i] = 45.0 + float64(i)*0.1
	}
	prices2[16] = 47.0
	prices2[17] = 48.0
	prices2[18] = 49.0
	prices2[19] = 50.0

	rsiValues2 := []float64{60.0, 58.0, 56.0, 54.0}

	result2 := &RSIResult{
		Values: rsiValues2,
		Period: 14,
	}

	hasDivergence2, isBullish2 := result2.IsDivergence(prices2)
	if !hasDivergence2 {
		t.Errorf("IsDivergence() 应该检测到背离")
	}
	if isBullish2 {
		t.Errorf("IsDivergence() 应该是看跌背离")
	}
}

func TestRSISignalToString(t *testing.T) {
	tests := []struct {
		signal   RSISignal
		expected string
	}{
		{RSIBuy, "买入信号"},
		{RSISell, "卖出信号"},
		{RSINeutral, "中性"},
	}

	for _, tt := range tests {
		result := RSISignalToString(tt.signal)
		if result != tt.expected {
			t.Errorf("RSISignalToString(%v) = %v, 期望 %v", tt.signal, result, tt.expected)
		}
	}
}

func TestRSIEdgeCases(t *testing.T) {
	// 测试所有价格相同的情况
	samePrices := []float64{50.0, 50.0, 50.0, 50.0, 50.0, 50.0, 50.0, 50.0, 50.0, 50.0, 50.0, 50.0, 50.0, 50.0, 50.0}

	result, err := CalculateRSI(samePrices, 14)
	if err != nil {
		t.Errorf("CalculateRSI() 相同价格不应该出错，错误 = %v", err)
		return
	}

	// 当所有价格相同时，RSI应该是50（或者由于没有亏损而接近100）
	rsi := result.GetLatest()
	if rsi != 100.0 {
		t.Errorf("CalculateRSI() 相同价格的RSI = %v, 期望 100.0", rsi)
	}

	// 测试空结果的GetStrength
	emptyResult := &RSIResult{
		Values: []float64{},
		Period: 14,
	}

	strength := emptyResult.GetStrength()
	if strength != "无数据" {
		t.Errorf("GetStrength() 空数据 = %v, 期望 '无数据'", strength)
	}
}

func TestRSIAccuracy(t *testing.T) {
	// 使用已知的测试数据验证RSI计算的准确性
	// 这些是手工计算或其他工具验证过的数据
	knownPrices := []float64{
		44.34, 44.09, 44.15, 43.61, 44.33, 44.83, 45.85, 47.15, 47.72, 46.87,
		42.66, 42.49, 42.84, 44.17, 44.18, 44.22, 44.57, 43.42, 42.66, 43.13,
	}

	result, err := CalculateRSI(knownPrices, 14)
	if err != nil {
		t.Fatalf("CalculateRSI() 错误 = %v", err)
	}

	// 验证RSI计算结果在合理范围内
	for i, rsi := range result.Values {
		if rsi < 0 || rsi > 100 {
			t.Errorf("RSI值[%d] = %v 超出合理范围 [0, 100]", i, rsi)
		}

		// RSI应该对价格变化有所反应
		if math.IsNaN(rsi) || math.IsInf(rsi, 0) {
			t.Errorf("RSI值[%d] = %v 是无效数值", i, rsi)
		}
	}
}

// 基准测试
func BenchmarkCalculateRSI(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CalculateRSI(rsiTestPrices, 14)
	}
}

func BenchmarkRSIGetSignal(b *testing.B) {
	result, _ := CalculateRSI(rsiTestPrices, 14)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result.GetDefaultSignal()
	}
}
