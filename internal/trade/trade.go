package trade

import (
	"math"

	"xueqiu-monitor/internal/model"
)

// CalcAdvices 根据持仓变动和总金额计算具体买卖建议，股数向下取整到 100 股
func CalcAdvices(diff model.Diff, prices map[string]float64, totalAmount float64) []model.TradeAdvice {
	var advices []model.TradeAdvice

	for _, h := range diff.Added {
		price := prices[h.Symbol]
		if price <= 0 {
			continue
		}
		targetAmount := totalAmount * h.Weight / 100
		shares := int(targetAmount/price/100) * 100
		advices = append(advices, model.TradeAdvice{
			Symbol: h.Symbol, Name: h.Name, Action: "买入",
			Price: price, Shares: shares, Amount: float64(shares) * price,
			TargetAmount: targetAmount,
		})
	}

	for _, h := range diff.Removed {
		price := prices[h.Symbol]
		if price <= 0 {
			continue
		}
		targetAmount := totalAmount * h.Weight / 100
		shares := int(targetAmount/price/100) * 100
		advices = append(advices, model.TradeAdvice{
			Symbol: h.Symbol, Name: h.Name, Action: "卖出",
			Price: price, Shares: shares, Amount: float64(shares) * price,
			TargetAmount: targetAmount,
		})
	}

	for _, h := range diff.Changed {
		price := prices[h.Symbol]
		if price <= 0 {
			continue
		}
		deltaAmount := totalAmount * math.Abs(h.Delta) / 100
		shares := int(deltaAmount/price/100) * 100
		action := "加仓"
		if h.Delta < 0 {
			action = "减仓"
		}
		advices = append(advices, model.TradeAdvice{
			Symbol: h.Symbol, Name: h.Name, Action: action,
			Price: price, Shares: shares, Amount: float64(shares) * price,
			TargetAmount: deltaAmount,
		})
	}

	return advices
}
