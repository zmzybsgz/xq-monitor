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
	// 不应包含变动段落
	if strings.Contains(html, "新增持仓") {
		t.Error("HTML should not contain 新增持仓 for empty diff")
	}
}
