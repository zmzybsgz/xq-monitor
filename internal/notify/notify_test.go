package notify

import (
	"strings"
	"testing"

	"xueqiu-monitor/internal/model"
)

func TestBuildHTML_Added(t *testing.T) {
	diff := model.Diff{
		Added: []model.Holding{
			{Symbol: "SZ002812", Name: "恩捷股份", Weight: 5.96},
		},
	}

	html := BuildHTML("ZH001", "测试组合", diff, nil, 0)

	if !strings.Contains(html, "测试组合") {
		t.Error("HTML should contain portfolio name")
	}
	if !strings.Contains(html, "新增持仓") {
		t.Error("HTML should contain 新增持仓 section")
	}
	if !strings.Contains(html, "恩捷股份") {
		t.Error("HTML should contain stock name")
	}
	if !strings.Contains(html, "5.96%") {
		t.Error("HTML should contain weight")
	}
}

func TestBuildHTML_Removed(t *testing.T) {
	diff := model.Diff{
		Removed: []model.Holding{
			{Symbol: "SH600000", Name: "浦发银行", Weight: 10.0},
		},
	}

	html := BuildHTML("ZH001", "测试组合", diff, nil, 0)

	if !strings.Contains(html, "移除持仓") {
		t.Error("HTML should contain 移除持仓 section")
	}
	if !strings.Contains(html, "浦发银行") {
		t.Error("HTML should contain removed stock name")
	}
}

func TestBuildHTML_Changed(t *testing.T) {
	diff := model.Diff{
		Changed: []model.HoldingChange{
			{
				Holding:   model.Holding{Symbol: "SH601857", Name: "中国石油", Weight: 55},
				OldWeight: 50,
				Delta:     5.0,
			},
			{
				Holding:   model.Holding{Symbol: "SH601919", Name: "中远海控", Weight: 30},
				OldWeight: 35,
				Delta:     -5.0,
			},
		},
	}

	html := BuildHTML("ZH001", "测试组合", diff, nil, 0)

	if !strings.Contains(html, "仓位调整") {
		t.Error("HTML should contain 仓位调整 section")
	}
	if !strings.Contains(html, "↑") {
		t.Error("HTML should contain ↑ for increase")
	}
	if !strings.Contains(html, "↓") {
		t.Error("HTML should contain ↓ for decrease")
	}
}

func TestBuildHTML_WithAdvice(t *testing.T) {
	diff := model.Diff{
		Added: []model.Holding{
			{Symbol: "SH600000", Name: "浦发银行", Weight: 10.0},
		},
	}
	advices := []model.TradeAdvice{
		{Symbol: "SH600000", Name: "浦发银行", Action: "买入", Price: 8.5, Shares: 1100, Amount: 9350, TargetAmount: 10000},
	}

	html := BuildHTML("ZH001", "测试组合", diff, advices, 100000)

	if !strings.Contains(html, "1100 股") {
		t.Error("HTML should contain share count")
	}
	if !strings.Contains(html, "100000 元") {
		t.Error("HTML should contain total amount")
	}
}

func TestBuildHTML_WithAdvice_NotEnough(t *testing.T) {
	diff := model.Diff{
		Added: []model.Holding{
			{Symbol: "SZ002812", Name: "恩捷股份", Weight: 5.0},
		},
	}
	advices := []model.TradeAdvice{
		{Symbol: "SZ002812", Name: "恩捷股份", Action: "买入", Price: 70, Shares: 0, Amount: 0, TargetAmount: 5000},
	}

	html := BuildHTML("ZH001", "测试组合", diff, advices, 100000)

	if !strings.Contains(html, "不足100股") {
		t.Error("HTML should indicate not enough for 100 shares")
	}
	if !strings.Contains(html, "5000 元") {
		t.Error("HTML should contain target amount")
	}
}

func TestBuildHTML_EmptyDiff(t *testing.T) {
	diff := model.Diff{}
	html := BuildHTML("ZH001", "测试组合", diff, nil, 0)

	if !strings.Contains(html, "测试组合") {
		t.Error("HTML should still contain portfolio name")
	}
	if strings.Contains(html, "新增持仓") {
		t.Error("HTML should not contain 新增持仓 for empty diff")
	}
}

func TestBuildHoldingsHTML_Normal(t *testing.T) {
	holdings := map[string][]model.Holding{
		"组合A（ZH001）": {
			{Symbol: "SH601857", Name: "中国石油", Weight: 50.0},
		},
	}
	html := buildHoldingsHTML(holdings, nil)
	if !strings.Contains(html, "中国石油") {
		t.Error("should contain stock name")
	}
	if !strings.Contains(html, "50.00%") {
		t.Error("should contain weight")
	}
	if strings.Contains(html, "抓取失败") {
		t.Error("should not contain error section when no errors")
	}
}

func TestBuildHoldingsHTML_WithFetchErrors(t *testing.T) {
	holdings := map[string][]model.Holding{}
	fetchErrors := map[string]string{
		"组合B（ZH002）": "HTTP 401",
	}
	html := buildHoldingsHTML(holdings, fetchErrors)
	if !strings.Contains(html, "抓取失败") {
		t.Error("should contain error section")
	}
	if !strings.Contains(html, "HTTP 401") {
		t.Error("should contain error message")
	}
}

func TestBuildHoldingsHTML_SortedOrder(t *testing.T) {
	holdings := map[string][]model.Holding{
		"Z组合": {{Symbol: "SH000001", Name: "Z股", Weight: 10}},
		"A组合": {{Symbol: "SH000002", Name: "A股", Weight: 20}},
	}
	html := buildHoldingsHTML(holdings, nil)
	posA := strings.Index(html, "A组合")
	posZ := strings.Index(html, "Z组合")
	if posA > posZ {
		t.Error("portfolios should be sorted alphabetically")
	}
}

func TestSendFetchErrors_EmptyToken(t *testing.T) {
	err := SendFetchErrors("", map[string]string{"组合A（ZH001）": "HTTP 401"})
	if err != nil {
		t.Errorf("empty token should return nil, got %v", err)
	}
}

func TestSendFetchErrors_EmptyErrors(t *testing.T) {
	err := SendFetchErrors("any_token", nil)
	if err != nil {
		t.Errorf("empty fetchErrors should return nil, got %v", err)
	}
	err = SendFetchErrors("any_token", map[string]string{})
	if err != nil {
		t.Errorf("empty fetchErrors map should return nil, got %v", err)
	}
}

func TestSendFetchErrors_HTMLContent(t *testing.T) {
	// 验证 buildHoldingsHTML 的错误段与 SendFetchErrors 使用的格式一致
	fetchErrors := map[string]string{
		"组合A（ZH001）": "HTTP 401",
		"组合B（ZH002）": "HTTP 403",
	}
	// 通过 buildHoldingsHTML 间接验证错误内容格式
	html := buildHoldingsHTML(nil, fetchErrors)
	if !strings.Contains(html, "组合A（ZH001）") {
		t.Error("should contain portfolio name")
	}
	if !strings.Contains(html, "HTTP 401") {
		t.Error("should contain error message")
	}
	// 验证两个组合都出现，且 A 在 B 之前（排序）
	posA := strings.Index(html, "组合A")
	posB := strings.Index(html, "组合B")
	if posA < 0 || posB < 0 || posA > posB {
		t.Error("fetch errors should be sorted and both present")
	}
}
