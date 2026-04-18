package model

// Holding 单只持仓
type Holding struct {
	Symbol string  `json:"symbol"`
	Name   string  `json:"name"`
	Weight float64 `json:"weight"` // 仓位占比 %
	Price  float64 `json:"price"`
}

// HoldingChange 仓位变动
type HoldingChange struct {
	Holding
	OldWeight float64 `json:"old_weight"`
	Delta     float64 `json:"delta"`
}

// Diff 持仓变动结果
type Diff struct {
	Added   []Holding       `json:"added"`
	Removed []Holding       `json:"removed"`
	Changed []HoldingChange `json:"changed"`
}

func (d *Diff) HasChanges() bool {
	return len(d.Added) > 0 || len(d.Removed) > 0 || len(d.Changed) > 0
}

// TradeAdvice 单只股票的买卖建议
type TradeAdvice struct {
	Symbol       string
	Name         string
	Action       string  // "买入" / "卖出" / "加仓" / "减仓"
	Price        float64 // 当前股价
	Shares       int     // 建议股数（已向下取整到 100 股）
	Amount       float64 // 实际金额 = Shares * Price
	TargetAmount float64 // 目标金额（按权重计算）
}
