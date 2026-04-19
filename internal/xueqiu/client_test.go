package xueqiu

import (
	"testing"
)

func TestParseCurrentResponse_OK(t *testing.T) {
	body := []byte(`{
		"last_rb": {
			"cash": 0.0,
			"holdings": [
				{"stock_symbol": "SH601857", "stock_name": "中国石油", "weight": 50.27},
				{"stock_symbol": "SH601919", "stock_name": "中远海控", "weight": 34.41},
				{"stock_symbol": "SH600941", "stock_name": "中国移动", "weight": 5.08}
			]
		}
	}`)

	holdings, err := parseCurrentResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(holdings) != 3 {
		t.Fatalf("got %d holdings, want 3", len(holdings))
	}

	// 应该按 symbol 排序
	if holdings[0].Symbol != "SH600941" {
		t.Errorf("first symbol = %s, want SH600941 (sorted)", holdings[0].Symbol)
	}
	if holdings[2].Symbol != "SH601919" {
		t.Errorf("last symbol = %s, want SH601919 (sorted)", holdings[2].Symbol)
	}

	// weight 四舍五入到两位
	if holdings[0].Weight != 5.08 {
		t.Errorf("weight = %v, want 5.08", holdings[0].Weight)
	}
}

func TestParseCurrentResponse_StringErrorCode(t *testing.T) {
	body := []byte(`{
		"error_code": "10026",
		"error_description": "遇到错误，请刷新页面后重试"
	}`)

	_, err := parseCurrentResponse(body)
	if err == nil {
		t.Fatal("expected error for string error_code")
	}
}

func TestParseCurrentResponse_NumericErrorCode(t *testing.T) {
	body := []byte(`{
		"error_code": 401,
		"error_description": "未登录"
	}`)

	_, err := parseCurrentResponse(body)
	if err == nil {
		t.Fatal("expected error for numeric error_code")
	}
}

func TestParseCurrentResponse_EmptyHoldings(t *testing.T) {
	body := []byte(`{"last_rb": {"cash": 100, "holdings": []}}`)

	holdings, err := parseCurrentResponse(body)
	if err != nil {
		t.Fatalf("empty holdings should not return error, got %v", err)
	}
	if holdings != nil {
		t.Fatalf("empty holdings should return nil slice, got %v", holdings)
	}
}

func TestParseCurrentResponse_InvalidJSON(t *testing.T) {
	_, err := parseCurrentResponse([]byte(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseQuoteResponse_OK(t *testing.T) {
	body := []byte(`{
		"data": [
			{"symbol": "SH601857", "current": 11.53},
			{"symbol": "SZ000408", "current": 84.85}
		]
	}`)

	prices, err := parseQuoteResponse(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prices["SH601857"] != 11.53 {
		t.Errorf("SH601857 price = %v, want 11.53", prices["SH601857"])
	}
	if prices["SZ000408"] != 84.85 {
		t.Errorf("SZ000408 price = %v, want 84.85", prices["SZ000408"])
	}
}

func TestParseQuoteResponse_InvalidJSON(t *testing.T) {
	_, err := parseQuoteResponse([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("truncate short = %q", got)
	}
	if got := truncate("hello world", 5); got != "hello..." {
		t.Errorf("truncate long = %q, want %q", got, "hello...")
	}
}
