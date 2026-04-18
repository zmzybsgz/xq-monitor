package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"xueqiu-monitor/internal/model"
)

// SendWechat 通过 PushPlus 发送微信通知
func SendWechat(token string, portfolioID, portfolioName string, diff model.Diff, advices []model.TradeAdvice, totalAmount float64) error {
	if token == "" {
		log.Println("[WARN] pushplus_token 未配置，跳过通知")
		return nil
	}

	content := BuildHTML(portfolioID, portfolioName, diff, advices, totalAmount)

	payload := map[string]string{
		"token":    token,
		"title":    fmt.Sprintf("📈 %s 发现调仓！", portfolioName),
		"content":  content,
		"template": "html",
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post("http://www.pushplus.plus/send", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("PushPlus 请求失败: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("解析 PushPlus 响应失败: %w", err)
	}

	if code, ok := result["code"].(float64); ok && int(code) == 200 {
		log.Printf("[OK] 微信通知已发送：%s", portfolioName)
	} else {
		return fmt.Errorf("PushPlus 发送失败：%v", result)
	}
	return nil
}

// BuildHTML 生成通知 HTML 内容
func BuildHTML(portfolioID, portfolioName string, diff model.Diff, advices []model.TradeAdvice, totalAmount float64) string {
	adviceMap := make(map[string]model.TradeAdvice, len(advices))
	for _, a := range advices {
		adviceMap[a.Symbol] = a
	}

	now := time.Now().Format("2006-01-02 15:04")
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("<h3>📊 %s（%s）发现调仓</h3>", portfolioName, portfolioID))
	sb.WriteString(fmt.Sprintf("<p>🕐 检测时间：%s</p>", now))
	if totalAmount > 0 {
		sb.WriteString(fmt.Sprintf("<p>💰 按总金额 %.0f 元计算（股数向下取整到100股）</p>", totalAmount))
	}

	if len(diff.Added) > 0 {
		sb.WriteString("<h4>🟢 新增持仓</h4><ul>")
		for _, s := range diff.Added {
			line := fmt.Sprintf("%s（%s）仓位 %.2f%%", s.Name, s.Symbol, s.Weight)
			if a, ok := adviceMap[s.Symbol]; ok {
				line += formatAdviceLine("买入", a)
			}
			sb.WriteString(fmt.Sprintf("<li>%s</li>", line))
		}
		sb.WriteString("</ul>")
	}

	if len(diff.Removed) > 0 {
		sb.WriteString("<h4>🔴 移除持仓</h4><ul>")
		for _, s := range diff.Removed {
			line := fmt.Sprintf("%s（%s）原仓位 %.2f%%", s.Name, s.Symbol, s.Weight)
			if a, ok := adviceMap[s.Symbol]; ok {
				line += formatAdviceLine("卖出", a)
			}
			sb.WriteString(fmt.Sprintf("<li>%s</li>", line))
		}
		sb.WriteString("</ul>")
	}

	if len(diff.Changed) > 0 {
		sb.WriteString("<h4>🟡 仓位调整</h4><ul>")
		for _, s := range diff.Changed {
			arrow := "↑"
			if s.Delta < 0 {
				arrow = "↓"
			}
			line := fmt.Sprintf("%s（%s）%.2f%% → %.2f%%（%s%.2f%%）",
				s.Name, s.Symbol, s.OldWeight, s.Weight, arrow, math.Abs(s.Delta))
			if a, ok := adviceMap[s.Symbol]; ok {
				line += formatAdviceLine(a.Action, a)
			}
			sb.WriteString(fmt.Sprintf("<li>%s</li>", line))
		}
		sb.WriteString("</ul>")
	}

	return sb.String()
}

func formatAdviceLine(action string, a model.TradeAdvice) string {
	if a.Shares > 0 {
		return fmt.Sprintf("<br>　→ 建议<b>%s %d 股</b>，现价 %.2f，金额 %.0f 元",
			action, a.Shares, a.Price, a.Amount)
	}
	return fmt.Sprintf("<br>　→ 目标%s %.0f 元，现价 %.2f（不足100股，约 %.1f 股）",
		action, a.TargetAmount, a.Price, a.TargetAmount/a.Price)
}
