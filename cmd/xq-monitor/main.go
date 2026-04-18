package main

import (
	"log"
	"os"
	"time"

	"xueqiu-monitor/internal/config"
	"xueqiu-monitor/internal/model"
	"xueqiu-monitor/internal/notify"
	"xueqiu-monitor/internal/snapshot"
	"xueqiu-monitor/internal/trade"
	"xueqiu-monitor/internal/xueqiu"
)

const (
	configFile  = "config.json"
	snapshotDir = "snapshots"
)

func monitorPortfolio(cfg config.Config, client *xueqiu.Client, p config.Portfolio) {
	log.Printf("--- 检测 %s（%s）---", p.Name, p.ID)

	current, err := client.GetHoldings(p.ID)
	if err != nil {
		log.Printf("[ERROR] 抓取失败: %v", err)
		return
	}
	log.Printf("当前持仓：%d 只", len(current))

	previous, err := snapshot.Load(snapshotDir, p.ID)
	if err != nil {
		log.Printf("[ERROR] 读取快照失败: %v", err)
		return
	}

	if len(current) == 0 && len(previous) > 0 {
		log.Printf("[ERROR] 抓到空持仓但历史非空（%d 只），疑似 cookie 失效，本次跳过", len(previous))
		return
	}

	if previous == nil {
		if len(current) == 0 {
			log.Println("首次运行但抓到空持仓，跳过")
			return
		}
		log.Println("首次运行，保存快照，无需通知")
		if err := snapshot.Save(snapshotDir, p.ID, current); err != nil {
			log.Printf("[ERROR] 保存快照失败: %v", err)
		}
		return
	}

	if snapshot.Fingerprint(previous) == snapshot.Fingerprint(current) {
		log.Println("持仓无变化")
		return
	}

	diff := snapshot.Diff(previous, current, cfg.WeightChangeDelta)
	if !diff.HasChanges() {
		log.Println("持仓无显著变化（仓位微调低于阈值）")
		if err := snapshot.Save(snapshotDir, p.ID, current); err != nil {
			log.Printf("[ERROR] 保存快照失败: %v", err)
		}
		return
	}

	log.Printf("发现调仓！新增=%d 移除=%d 调仓=%d",
		len(diff.Added), len(diff.Removed), len(diff.Changed))

	// 获取股价并计算买卖建议
	var advices []model.TradeAdvice
	if cfg.TotalAmount > 0 {
		var symbols []string
		for _, h := range diff.Added {
			symbols = append(symbols, h.Symbol)
		}
		for _, h := range diff.Removed {
			symbols = append(symbols, h.Symbol)
		}
		for _, h := range diff.Changed {
			symbols = append(symbols, h.Symbol)
		}
		prices, err := client.GetStockPrices(symbols)
		if err != nil {
			log.Printf("[WARN] 获取股价失败，跳过买卖建议: %v", err)
		} else {
			advices = trade.CalcAdvices(diff, prices, cfg.TotalAmount)
			for _, a := range advices {
				if a.Shares > 0 {
					log.Printf("  💰 %s %s（%s）现价 %.2f，建议 %d 股，金额 %.0f 元",
						a.Action, a.Name, a.Symbol, a.Price, a.Shares, a.Amount)
				} else {
					log.Printf("  💰 %s %s（%s）现价 %.2f，目标金额 %.0f 元（不足100股，需自行决定）",
						a.Action, a.Name, a.Symbol, a.Price, a.TargetAmount)
				}
			}
		}
	}

	if err := notify.SendWechat(cfg.PushplusToken, p.ID, p.Name, diff, advices, cfg.TotalAmount); err != nil {
		log.Printf("[ERROR] 发送通知失败: %v", err)
	}

	if err := snapshot.Save(snapshotDir, p.ID, current); err != nil {
		log.Printf("[ERROR] 保存快照失败: %v", err)
	}
}

func runOnce(cfg config.Config, client *xueqiu.Client) {
	for i, p := range cfg.Portfolios {
		monitorPortfolio(cfg, client, p)
		if i < len(cfg.Portfolios)-1 {
			time.Sleep(2 * time.Second)
		}
	}
}

func main() {
	once := len(os.Args) > 1 && os.Args[1] == "--once"

	cfg := config.Load(configFile)
	var configLastMod time.Time
	if info, err := os.Stat(configFile); err == nil {
		configLastMod = info.ModTime()
	}

	log.Printf("=== 雪球组合监控启动 ===")
	log.Printf("组合 %d 个，仓位阈值 %.2f%%，总金额 %.0f 元，轮询间隔 %ds",
		len(cfg.Portfolios), cfg.WeightChangeDelta, cfg.TotalAmount, cfg.Interval)
	if cfg.XueqiuCookie == "" {
		log.Println("[WARN] xueqiu_cookie 未配置！匿名 cookie 大概率被雪球拒绝")
	}
	if cfg.PushplusToken == "" {
		log.Println("[WARN] pushplus_token 未配置，检测到调仓也不会推送微信")
	}

	client := xueqiu.NewClient(cfg.XueqiuCookie)

	if once || cfg.Interval <= 0 {
		runOnce(cfg, client)
		log.Println("=== 监控完成 ===")
		return
	}

	log.Printf("进入常驻模式，每 %d 秒检测一次（修改 config.json 自动热加载）", cfg.Interval)
	for {
		runOnce(cfg, client)
		log.Printf("--- 下次检测：%s ---", time.Now().Add(time.Duration(cfg.Interval)*time.Second).Format("15:04:05"))
		time.Sleep(time.Duration(cfg.Interval) * time.Second)
		if newCfg, reloaded := config.TryReload(configFile, &configLastMod, cfg); reloaded {
			cfg = newCfg
			client = xueqiu.NewClient(cfg.XueqiuCookie)
		}
	}
}
