package trade

import (
	"testing"

	"xueqiu-monitor/internal/model"
)

func TestCalcAdvices_Buy(t *testing.T) {
	diff := model.Diff{
		Added: []model.Holding{
			{Symbol: "SH600000", Name: "浦发银行", Weight: 10.0},
		},
	}
	prices := map[string]float64{"SH600000": 8.50}

	advices := CalcAdvices(diff, prices, 500000)

	if len(advices) != 1 {
		t.Fatalf("got %d advices, want 1", len(advices))
	}
	a := advices[0]
	if a.Action != "买入" {
		t.Errorf("Action = %q, want 买入", a.Action)
	}
	// 500000 * 10% = 50000, 50000/8.5 = 5882 股, 向下取整到 5800
	if a.Shares != 5800 {
		t.Errorf("Shares = %d, want 5800", a.Shares)
	}
	if a.Amount != 5800*8.50 {
		t.Errorf("Amount = %.0f, want %.0f", a.Amount, 5800*8.50)
	}
}

func TestCalcAdvices_Sell(t *testing.T) {
	diff := model.Diff{
		Removed: []model.Holding{
			{Symbol: "SZ000001", Name: "平安银行", Weight: 5.0},
		},
	}
	prices := map[string]float64{"SZ000001": 12.0}

	advices := CalcAdvices(diff, prices, 200000)

	if len(advices) != 1 {
		t.Fatalf("got %d advices, want 1", len(advices))
	}
	a := advices[0]
	if a.Action != "卖出" {
		t.Errorf("Action = %q, want 卖出", a.Action)
	}
	// 200000 * 5% = 10000, 10000/12 = 833 股, 向下取整到 800
	if a.Shares != 800 {
		t.Errorf("Shares = %d, want 800", a.Shares)
	}
}

func TestCalcAdvices_Increase(t *testing.T) {
	diff := model.Diff{
		Changed: []model.HoldingChange{
			{
				Holding:   model.Holding{Symbol: "SH601857", Name: "中国石油", Weight: 55},
				OldWeight: 50,
				Delta:     5.0,
			},
		},
	}
	prices := map[string]float64{"SH601857": 11.50}

	advices := CalcAdvices(diff, prices, 500000)

	if len(advices) != 1 {
		t.Fatalf("got %d advices, want 1", len(advices))
	}
	a := advices[0]
	if a.Action != "加仓" {
		t.Errorf("Action = %q, want 加仓", a.Action)
	}
	// 500000 * 5% = 25000, 25000/11.5 = 2173 股, 向下取整到 2100
	if a.Shares != 2100 {
		t.Errorf("Shares = %d, want 2100", a.Shares)
	}
}

func TestCalcAdvices_Decrease(t *testing.T) {
	diff := model.Diff{
		Changed: []model.HoldingChange{
			{
				Holding:   model.Holding{Symbol: "SH601919", Name: "中远海控", Weight: 30},
				OldWeight: 35,
				Delta:     -5.0,
			},
		},
	}
	prices := map[string]float64{"SH601919": 15.0}

	advices := CalcAdvices(diff, prices, 500000)

	if len(advices) != 1 {
		t.Fatalf("got %d advices, want 1", len(advices))
	}
	a := advices[0]
	if a.Action != "减仓" {
		t.Errorf("Action = %q, want 减仓", a.Action)
	}
	// 500000 * 5% = 25000, 25000/15 = 1666 股, 向下取整到 1600
	if a.Shares != 1600 {
		t.Errorf("Shares = %d, want 1600", a.Shares)
	}
}

func TestCalcAdvices_NotEnoughFor100Shares(t *testing.T) {
	diff := model.Diff{
		Added: []model.Holding{
			{Symbol: "SZ002812", Name: "恩捷股份", Weight: 5.0},
		},
	}
	prices := map[string]float64{"SZ002812": 70.0}

	advices := CalcAdvices(diff, prices, 100000)

	if len(advices) != 1 {
		t.Fatalf("got %d advices, want 1", len(advices))
	}
	a := advices[0]
	// 100000 * 5% = 5000, 5000/70 = 71.4 股, 向下取整到 0
	if a.Shares != 0 {
		t.Errorf("Shares = %d, want 0 (not enough for 100 shares)", a.Shares)
	}
	if a.TargetAmount != 5000 {
		t.Errorf("TargetAmount = %.0f, want 5000", a.TargetAmount)
	}
}

func TestCalcAdvices_ZeroPrice(t *testing.T) {
	diff := model.Diff{
		Added: []model.Holding{
			{Symbol: "SH000001", Name: "未知", Weight: 10.0},
		},
	}
	prices := map[string]float64{"SH000001": 0}

	advices := CalcAdvices(diff, prices, 500000)

	if len(advices) != 0 {
		t.Errorf("got %d advices, want 0 (zero price should be skipped)", len(advices))
	}
}

func TestCalcAdvices_EmptyDiff(t *testing.T) {
	diff := model.Diff{}
	prices := map[string]float64{}

	advices := CalcAdvices(diff, prices, 500000)

	if len(advices) != 0 {
		t.Errorf("got %d advices, want 0", len(advices))
	}
}
