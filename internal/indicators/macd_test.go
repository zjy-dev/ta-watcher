package indicators

import (
	"math"
	"testing"
)

// MACD测试数据 - 包含趋势变化
var macdTestPrices = []float64{
	459.99, 448.85, 446.06, 450.81, 442.80, 448.97, 444.57, 441.40, 430.47, 420.05,
	431.14, 425.66, 430.58, 431.72, 437.87, 428.43, 428.35, 432.50, 443.66, 455.72,
	454.49, 452.08, 452.73, 461.91, 463.58, 461.14, 452.08, 442.66, 428.91, 429.79,
	431.99, 427.72, 423.20, 426.21, 426.98, 435.69, 434.33, 429.80, 419.85, 426.24,
	402.80, 392.05, 390.53, 398.67, 406.13, 405.46, 408.38, 417.20, 430.12, 442.78,
}

func TestCalculateMACD(t *testing.T) {
	tests := []struct {
		name         string
		prices       []float64
		fastPeriod   int
		slowPeriod   int
		signalPeriod int
		wantErr      bool
	}{
		{
			name:         "正常计算MACD-默认参数",
			prices:       macdTestPrices,
			fastPeriod:   12,
			slowPeriod:   26,
			signalPeriod: 9,
			wantErr:      false,
		},
		{
			name:         "正常计算MACD-自定义参数",
			prices:       macdTestPrices,
			fastPeriod:   5,
			slowPeriod:   10,
			signalPeriod: 3,
			wantErr:      false,
		},
		{
			name:         "价格数据不足",
			prices:       []float64{1.0, 2.0, 3.0},
			fastPeriod:   12,
			slowPeriod:   26,
			signalPeriod: 9,
			wantErr:      true,
		},
		{
			name:         "快线周期为0",
			prices:       macdTestPrices,
			fastPeriod:   0,
			slowPeriod:   26,
			signalPeriod: 9,
			wantErr:      true,
		},
		{
			name:         "快线周期大于等于慢线周期",
			prices:       macdTestPrices,
			fastPeriod:   26,
			slowPeriod:   12,
			signalPeriod: 9,
			wantErr:      true,
		},
		{
			name:         "信号线周期为负数",
			prices:       macdTestPrices,
			fastPeriod:   12,
			slowPeriod:   26,
			signalPeriod: -1,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CalculateMACD(tt.prices, tt.fastPeriod, tt.slowPeriod, tt.signalPeriod)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CalculateMACD() 期望错误，但没有返回错误")
				}
				return
			}

			if err != nil {
				t.Errorf("CalculateMACD() 意外错误 = %v", err)
				return
			}

			if result == nil {
				t.Errorf("CalculateMACD() 返回nil结果")
				return
			}

			// 检查三个序列长度相等
			if len(result.MACD) != len(result.Signal) || len(result.Signal) != len(result.Histogram) {
				t.Errorf("CalculateMACD() MACD、Signal、Histogram长度不一致: %d, %d, %d",
					len(result.MACD), len(result.Signal), len(result.Histogram))
			}

			// 检查参数保存正确
			if result.FastPeriod != tt.fastPeriod {
				t.Errorf("CalculateMACD() FastPeriod = %v, 期望 %v", result.FastPeriod, tt.fastPeriod)
			}
			if result.SlowPeriod != tt.slowPeriod {
				t.Errorf("CalculateMACD() SlowPeriod = %v, 期望 %v", result.SlowPeriod, tt.slowPeriod)
			}
			if result.SignalPeriod != tt.signalPeriod {
				t.Errorf("CalculateMACD() SignalPeriod = %v, 期望 %v", result.SignalPeriod, tt.signalPeriod)
			}

			// 验证Histogram = MACD - Signal
			for i := 0; i < len(result.Histogram); i++ {
				expected := result.MACD[i] - result.Signal[i]
				if math.Abs(result.Histogram[i]-expected) > 0.0001 {
					t.Errorf("CalculateMACD() Histogram[%d] = %v, 期望 %v", i, result.Histogram[i], expected)
				}
			}
		})
	}
}

func TestCalculateDefaultMACD(t *testing.T) {
	result, err := CalculateDefaultMACD(macdTestPrices)

	if err != nil {
		t.Errorf("CalculateDefaultMACD() 错误 = %v", err)
		return
	}

	if result == nil {
		t.Errorf("CalculateDefaultMACD() 返回nil结果")
		return
	}

	// 检查使用的是默认参数
	if result.FastPeriod != DefaultFastPeriod {
		t.Errorf("CalculateDefaultMACD() FastPeriod = %v, 期望 %v", result.FastPeriod, DefaultFastPeriod)
	}
	if result.SlowPeriod != DefaultSlowPeriod {
		t.Errorf("CalculateDefaultMACD() SlowPeriod = %v, 期望 %v", result.SlowPeriod, DefaultSlowPeriod)
	}
	if result.SignalPeriod != DefaultSignalPeriod {
		t.Errorf("CalculateDefaultMACD() SignalPeriod = %v, 期望 %v", result.SignalPeriod, DefaultSignalPeriod)
	}
}

func TestMACDResult_GetLatest(t *testing.T) {
	result, err := CalculateDefaultMACD(macdTestPrices)
	if err != nil {
		t.Fatalf("CalculateDefaultMACD() 错误 = %v", err)
	}

	macd, signal, histogram := result.GetLatest()

	expectedMACD := result.MACD[len(result.MACD)-1]
	expectedSignal := result.Signal[len(result.Signal)-1]
	expectedHistogram := result.Histogram[len(result.Histogram)-1]

	if macd != expectedMACD {
		t.Errorf("GetLatest() MACD = %v, 期望 %v", macd, expectedMACD)
	}
	if signal != expectedSignal {
		t.Errorf("GetLatest() Signal = %v, 期望 %v", signal, expectedSignal)
	}
	if histogram != expectedHistogram {
		t.Errorf("GetLatest() Histogram = %v, 期望 %v", histogram, expectedHistogram)
	}
}

func TestMACDResult_GetLatestN(t *testing.T) {
	result, err := CalculateDefaultMACD(macdTestPrices)
	if err != nil {
		t.Fatalf("CalculateDefaultMACD() 错误 = %v", err)
	}

	tests := []struct {
		name string
		n    int
		want int
	}{
		{"获取最新3个值", 3, 3},
		{"获取所有值", len(result.MACD), len(result.MACD)},
		{"获取超过可用数量的值", len(result.MACD) + 5, len(result.MACD)},
		{"n为0", 0, 0},
		{"n为负数", -1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			macd, signal, histogram := result.GetLatestN(tt.n)
			if len(macd) != tt.want || len(signal) != tt.want || len(histogram) != tt.want {
				t.Errorf("GetLatestN(%v) 长度 = (%d, %d, %d), 期望 (%d, %d, %d)",
					tt.n, len(macd), len(signal), len(histogram), tt.want, tt.want, tt.want)
			}
		})
	}
}

func TestMACDResult_GetSignal(t *testing.T) {
	// 创建测试数据：MACD金叉 (MACD线从下方穿越信号线)
	macdValues := []float64{-1.0, -0.5, 0.2, 0.8}
	signalValues := []float64{-0.3, 0.0, 0.3, 0.5}
	histogramValues := []float64{-0.7, -0.5, -0.1, 0.3}

	result := &MACDResult{
		MACD:      macdValues,
		Signal:    signalValues,
		Histogram: histogramValues,
	}

	signal := result.GetSignal()
	if signal != MACDBuy {
		t.Errorf("GetSignal() = %v, 期望金叉买入信号", signal)
	}
	// 创建测试数据：MACD死叉 (MACD线从上方穿越信号线)
	// 前一个: MACD >= Signal, 当前: MACD < Signal
	macdValues2 := []float64{1.0, 0.8, 0.6, -0.1}
	signalValues2 := []float64{0.3, 0.5, 0.5, 0.2}

	result2 := &MACDResult{
		MACD:   macdValues2,
		Signal: signalValues2,
	}

	signal2 := result2.GetSignal()
	if signal2 != MACDSell {
		t.Errorf("GetSignal() = %v, 期望死叉卖出信号", signal2)
	}

	// 创建测试数据：无明显信号 (没有穿越)
	macdValues3 := []float64{1.0, 1.1, 1.2, 1.3}
	signalValues3 := []float64{0.8, 0.9, 1.0, 1.1}

	result3 := &MACDResult{
		MACD:   macdValues3,
		Signal: signalValues3,
	}

	signal3 := result3.GetSignal()
	if signal3 != MACDNeutral {
		t.Errorf("GetSignal() = %v, 期望中性信号", signal3)
	}
}

func TestMACDResult_IsGoldenCross(t *testing.T) {
	// 创建金叉测试数据：MACD从下方穿越信号线
	macdValues := []float64{-1.0, -0.5, 0.2, 0.8}
	signalValues := []float64{-0.3, 0.0, 0.3, 0.5}

	result := &MACDResult{
		MACD:   macdValues,
		Signal: signalValues,
	}

	if !result.IsGoldenCross() {
		t.Errorf("IsGoldenCross() = false, 期望 true")
	}
}

func TestMACDResult_IsDeathCross(t *testing.T) {
	// 创建死叉测试数据：MACD从上方穿越信号线
	// 前一个: MACD >= Signal, 当前: MACD < Signal
	macdValues := []float64{1.0, 0.8, 0.6, -0.1}
	signalValues := []float64{0.3, 0.5, 0.5, 0.2}

	result := &MACDResult{
		MACD:   macdValues,
		Signal: signalValues,
	}

	if !result.IsDeathCross() {
		t.Errorf("IsDeathCross() = false, 期望 true")
	}
}

func TestMACDResult_GetTrend(t *testing.T) {
	tests := []struct {
		name      string
		macd      float64
		signal    float64
		histogram float64
		expected  string
	}{
		{"强势上涨", 1.0, 0.5, 0.5, "强势上涨"},
		{"上涨减弱", 1.0, 0.5, -0.5, "上涨减弱"},
		{"强势下跌", -1.0, -0.5, -0.5, "强势下跌"},
		{"下跌减弱", -1.0, -0.5, 0.5, "下跌减弱"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &MACDResult{
				MACD:      []float64{tt.macd},
				Signal:    []float64{tt.signal},
				Histogram: []float64{tt.histogram},
			}

			trend := result.GetTrend()
			if trend != tt.expected {
				t.Errorf("GetTrend() = %v, 期望 %v", trend, tt.expected)
			}
		})
	}
}

func TestMACDResult_IsDivergence(t *testing.T) {
	// 创建足够长的数据用于背离检测
	prices := make([]float64, 30)
	for i := 0; i < 26; i++ {
		prices[i] = 50.0 + float64(i)*0.1 // 填充前26个数据点
	}
	// 最后4个点：价格下跌但我们希望MACD上升（看涨背离）
	prices[26] = 50.0
	prices[27] = 49.0
	prices[28] = 48.0
	prices[29] = 47.0

	// 创建看涨背离测试数据：MACD上升
	macdValues := []float64{-2.0, -1.5, -1.0, -0.5}

	result := &MACDResult{
		MACD:       macdValues,
		SlowPeriod: 26,
	}

	hasDivergence, isBullish := result.IsDivergence(prices)
	if !hasDivergence {
		t.Errorf("IsDivergence() 应该检测到背离")
	}
	if !isBullish {
		t.Errorf("IsDivergence() 应该是看涨背离")
	}

	// 创建看跌背离测试数据：价格上涨但MACD下降
	prices2 := make([]float64, 30)
	for i := 0; i < 26; i++ {
		prices2[i] = 45.0 + float64(i)*0.1
	}
	prices2[26] = 47.0
	prices2[27] = 48.0
	prices2[28] = 49.0
	prices2[29] = 50.0

	macdValues2 := []float64{2.0, 1.5, 1.0, 0.5}

	result2 := &MACDResult{
		MACD:       macdValues2,
		SlowPeriod: 26,
	}

	hasDivergence2, isBullish2 := result2.IsDivergence(prices2)
	if !hasDivergence2 {
		t.Errorf("IsDivergence() 应该检测到背离")
	}
	if isBullish2 {
		t.Errorf("IsDivergence() 应该是看跌背离")
	}
}

func TestMACDResult_GetHistogramTrend(t *testing.T) {
	tests := []struct {
		name     string
		current  float64
		prev     float64
		expected string
	}{
		{"动能增强", 1.0, 0.5, "动能增强"},
		{"跌势减弱", -0.5, -1.0, "跌势减弱"},
		{"动能减弱", 0.5, 1.0, "涨势减弱"},
		{"跌势减弱2", -1.0, -0.5, "动能减弱"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &MACDResult{
				Histogram: []float64{tt.prev, tt.current},
			}

			trend := result.GetHistogramTrend()
			if trend != tt.expected {
				t.Errorf("GetHistogramTrend() = %v, 期望 %v", trend, tt.expected)
			}
		})
	}
}

func TestMACDSignalToString(t *testing.T) {
	tests := []struct {
		signal   MACDSignal
		expected string
	}{
		{MACDBuy, "买入信号（金叉）"},
		{MACDSell, "卖出信号（死叉）"},
		{MACDNeutral, "中性"},
	}

	for _, tt := range tests {
		result := MACDSignalToString(tt.signal)
		if result != tt.expected {
			t.Errorf("MACDSignalToString(%v) = %v, 期望 %v", tt.signal, result, tt.expected)
		}
	}
}

func TestMACDEdgeCases(t *testing.T) {
	// 测试空数据
	emptyResult := &MACDResult{
		MACD:   []float64{},
		Signal: []float64{},
	}

	macd, signal, histogram := emptyResult.GetLatest()
	if macd != 0 || signal != 0 || histogram != 0 {
		t.Errorf("GetLatest() 空数据应该返回(0, 0, 0)，得到(%v, %v, %v)", macd, signal, histogram)
	}

	trend := emptyResult.GetTrend()
	if trend != "无数据" {
		t.Errorf("GetTrend() 空数据 = %v, 期望 '无数据'", trend)
	}

	histogramTrend := emptyResult.GetHistogramTrend()
	if histogramTrend != "无数据" {
		t.Errorf("GetHistogramTrend() 空数据 = %v, 期望 '无数据'", histogramTrend)
	}

	// 测试单个数据点
	singleResult := &MACDResult{
		MACD:   []float64{1.0},
		Signal: []float64{0.5},
	}

	signal2 := singleResult.GetSignal()
	if signal2 != MACDNeutral {
		t.Errorf("GetSignal() 单个数据点应该返回中性，得到 %v", signal2)
	}
}

func TestMACDAccuracy(t *testing.T) {
	// 验证MACD计算的基本准确性
	result, err := CalculateDefaultMACD(macdTestPrices)
	if err != nil {
		t.Fatalf("CalculateDefaultMACD() 错误 = %v", err)
	}

	// 验证数据完整性
	for i, macd := range result.MACD {
		if math.IsNaN(macd) || math.IsInf(macd, 0) {
			t.Errorf("MACD[%d] = %v 是无效数值", i, macd)
		}
	}

	for i, signal := range result.Signal {
		if math.IsNaN(signal) || math.IsInf(signal, 0) {
			t.Errorf("Signal[%d] = %v 是无效数值", i, signal)
		}
	}

	for i, histogram := range result.Histogram {
		if math.IsNaN(histogram) || math.IsInf(histogram, 0) {
			t.Errorf("Histogram[%d] = %v 是无效数值", i, histogram)
		}
	}
}

// 基准测试
func BenchmarkCalculateMACD(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CalculateDefaultMACD(macdTestPrices)
	}
}

func BenchmarkMACDGetSignal(b *testing.B) {
	result, _ := CalculateDefaultMACD(macdTestPrices)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result.GetSignal()
	}
}
