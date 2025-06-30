package datasource

import (
	"context"
	"testing"
	"time"
)

func TestBinanceClient_New(t *testing.T) {
	client := NewBinanceClient()

	if client == nil {
		t.Fatal("NewBinanceClient() returned nil")
	}

	if client.Name() != "binance" {
		t.Errorf("Expected name 'binance', got '%s'", client.Name())
	}
}

func TestBinanceClient_IsSymbolValid(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Binance API test in short mode")
	}

	client := NewBinanceClient()
	ctx := context.Background()

	tests := []struct {
		name     string
		symbol   string
		expected bool
	}{
		{"Valid symbol", "BTCUSDT", true},
		{"Invalid symbol", "INVALIDUSDT", false},
		{"Empty symbol", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, err := client.IsSymbolValid(ctx, tt.symbol)
			if err != nil && tt.expected {
				t.Errorf("IsSymbolValid(%s) returned error: %v", tt.symbol, err)
			}
			if valid != tt.expected {
				t.Errorf("IsSymbolValid(%s) = %v, expected %v", tt.symbol, valid, tt.expected)
			}
		})
	}
}

func TestBinanceClient_GetKlines(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Binance API test in short mode")
	}

	client := NewBinanceClient()
	ctx := context.Background()

	tests := []struct {
		name      string
		symbol    string
		timeframe Timeframe
		limit     int
		wantErr   bool
	}{
		{"Valid daily klines", "BTCUSDT", Timeframe1d, 10, false},
		{"Valid hourly klines", "ETHUSDT", Timeframe1h, 5, false},
		{"Invalid symbol", "INVALIDUSDT", Timeframe1d, 10, true},
		{"Zero limit", "BTCUSDT", Timeframe1d, 0, false}, // Should default to reasonable limit
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			klines, err := client.GetKlines(ctx, tt.symbol, tt.timeframe, time.Time{}, time.Time{}, tt.limit)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetKlines() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(klines) == 0 {
					t.Error("GetKlines() returned empty result for valid symbol")
				}

				// Verify kline structure
				for i, kline := range klines {
					if kline.Symbol != tt.symbol {
						t.Errorf("Kline[%d].Symbol = %s, expected %s", i, kline.Symbol, tt.symbol)
					}
					if kline.Open <= 0 || kline.Close <= 0 || kline.High <= 0 || kline.Low <= 0 {
						t.Errorf("Kline[%d] has invalid OHLC values: O=%.2f H=%.2f L=%.2f C=%.2f",
							i, kline.Open, kline.High, kline.Low, kline.Close)
					}
					if kline.High < kline.Low {
						t.Errorf("Kline[%d] High < Low: H=%.2f L=%.2f", i, kline.High, kline.Low)
					}
				}
			}
		})
	}
}

func TestBinanceClient_TimeframeSupport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Binance API test in short mode")
	}

	client := NewBinanceClient()
	ctx := context.Background()

	timeframes := []Timeframe{
		Timeframe1m,
		Timeframe5m,
		Timeframe15m,
		Timeframe1h,
		Timeframe4h,
		Timeframe1d,
		Timeframe1w,
		Timeframe1M,
	}

	for _, tf := range timeframes {
		t.Run(string(tf), func(t *testing.T) {
			klines, err := client.GetKlines(ctx, "BTCUSDT", tf, time.Time{}, time.Time{}, 5)
			if err != nil {
				t.Errorf("Timeframe %s not supported: %v", tf, err)
			}
			if len(klines) == 0 {
				t.Errorf("No data returned for timeframe %s", tf)
			}
		})
	}
}
