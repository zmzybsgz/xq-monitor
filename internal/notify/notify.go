package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"xueqiu-monitor/internal/model"
)

var pushplusClient = &http.Client{Timeout: 30 * time.Second}

func sendToPushplus(token, title, content string) error {
	payload := map[string]string{
		"token":    token,
		"title":    title,
		"content":  content,
		"template": "html",
	}
	body, _ := json.Marshal(payload)

	resp, err := pushplusClient.Post("https://www.pushplus.plus/send", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("PushPlus 请求失败: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("解析 PushPlus 响应失败: %w", err)
	}
	if code, ok := result["code"].(float64); !ok || int(code) != 200 {
		return fmt.Errorf("PushPlus 发送失败：%v", result)
	}
	return nil
}

// SendWechat 通过 PushPlus 发送微信通知
func SendWechat(token string, portfolioID, portfolioName string, diff model.Diff, advices []model.TradeAdvice, totalAmount float64) error {
	if token == "" {
		log.Println("[WARN] pushplus_token 未配置，跳过通知")
		return nil
	}

	content := BuildHTML(portfolioID, portfolioName, diff, advices, totalAmount)
	if err := sendToPushplus(token, fmt.Sprintf("📈 %s 发现调仓！", portfolioName), content); err != nil {
		return err
	}
	log.Printf("[OK] 微信通知已发送：%s", portfolioName)
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

// buildHoldingsHTML 构建持仓概览 HTML 表格，fetchErrors 为抓取失败的组合及原因
func buildHoldingsHTML(holdings map[string][]model.Holding, fetchErrors map[string]string) string {
	var sb strings.Builder

	names := make([]string, 0, len(holdings))
	for name := range holdings {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		hs := holdings[name]
		sb.WriteString(fmt.Sprintf("<h4>📊 %s（%d 只）</h4>", name, len(hs)))
		sb.WriteString("<table border='1' cellpadding='5' cellspacing='0' style='border-collapse:collapse'>")
		sb.WriteString("<tr><th>股票</th><th>代码</th><th>仓位</th></tr>")
		for _, h := range hs {
			sb.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%s</td><td>%.2f%%</td></tr>",
				h.Name, h.Symbol, h.Weight))
		}
		sb.WriteString("</table>")
	}

	if len(fetchErrors) > 0 {
		sb.WriteString("<h4>⚠️ 以下组合抓取失败（Cookie 可能已失效）</h4><ul>")
		errNames := make([]string, 0, len(fetchErrors))
		for name := range fetchErrors {
			errNames = append(errNames, name)
		}
		sort.Strings(errNames)
		for _, name := range errNames {
			sb.WriteString(fmt.Sprintf("<li>%s：%s</li>", name, fetchErrors[name]))
		}
		sb.WriteString("</ul>")
	}

	return sb.String()
}

// SendStartupSummary 启动时推送当前持仓概览
func SendStartupSummary(token string, holdings map[string][]model.Holding) error {
	if token == "" {
		return nil
	}
	now := time.Now().Format("2006-01-02 15:04")
	var sb strings.Builder
	sb.WriteString("<h3>🚀 雪球组合监控已启动</h3>")
	sb.WriteString(fmt.Sprintf("<p>🕐 启动时间：%s</p>", now))
	sb.WriteString(buildHoldingsHTML(holdings, nil))
	if err := sendToPushplus(token, "🚀 雪球组合监控已启动", sb.String()); err != nil {
		return err
	}
	log.Println("[OK] 启动概览通知已发送")
	return nil
}

// SendDailySummary 每日早 8 点推送持仓概览及程序运行状态
func SendDailySummary(token string, holdings map[string][]model.Holding, fetchErrors map[string]string) error {
	if token == "" {
		return nil
	}
	now := time.Now().Format("2006-01-02 15:04")
	var sb strings.Builder
	sb.WriteString("<h3>📅 每日持仓播报</h3>")
	sb.WriteString(fmt.Sprintf("<p>🕐 播报时间：%s</p>", now))
	sb.WriteString(fmt.Sprintf("<p>✅ 程序运行正常</p>"))
	sb.WriteString(buildHoldingsHTML(holdings, fetchErrors))
	title := "📅 每日持仓播报"
	if len(fetchErrors) > 0 {
		title = "⚠️ 每日播报（有组合抓取失败，请检查 Cookie）"
	}
	if err := sendToPushplus(token, title, sb.String()); err != nil {
		return err
	}
	log.Println("[OK] 每日持仓播报已发送")
	return nil
}

// SendFetchErrors 汇总推送本轮所有抓取失败的组合（Cookie 失效提醒）
func SendFetchErrors(token string, fetchErrors map[string]string) error {
	if token == "" || len(fetchErrors) == 0 {
		return nil
	}
	now := time.Now().Format("2006-01-02 15:04")
	var sb strings.Builder
	sb.WriteString("<h3>⚠️ 持仓抓取失败</h3>")
	sb.WriteString(fmt.Sprintf("<p>🕐 时间：%s</p>", now))
	sb.WriteString("<p>以下组合本轮抓取失败，请检查 xueqiu_cookie 是否已失效：</p><ul>")
	names := make([]string, 0, len(fetchErrors))
	for name := range fetchErrors {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		sb.WriteString(fmt.Sprintf("<li>%s：%s</li>", name, fetchErrors[name]))
	}
	sb.WriteString("</ul>")
	sb.WriteString("<p>👉 更新 config.json 中的 xueqiu_cookie 后自动生效（无需重启）</p>")
	title := fmt.Sprintf("⚠️ %d 个组合抓取失败，请更新 Cookie", len(fetchErrors))
	if err := sendToPushplus(token, title, sb.String()); err != nil {
		return err
	}
	log.Printf("[OK] 抓取失败汇总警报已发送，涉及 %d 个组合", len(fetchErrors))
	return nil
}

func formatAdviceLine(action string, a model.TradeAdvice) string {
	if a.Shares > 0 {
		return fmt.Sprintf("<br>　→ 建议<b>%s %d 股</b>，现价 %.2f，金额 %.0f 元",
			action, a.Shares, a.Price, a.Amount)
	}
	return fmt.Sprintf("<br>　→ 目标%s %.0f 元，现价 %.2f（不足100股，约 %.1f 股）",
		action, a.TargetAmount, a.Price, a.TargetAmount/a.Price)
}
