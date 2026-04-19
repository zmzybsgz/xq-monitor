package main

import (
	"log"
	"os"
	"time"

	"xueqiu-monitor/internal/config"
	"xueqiu-monitor/internal/logger"
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

// monitorPortfolio 检测单个组合变化。preloaded 非 nil 时跳过 API 抓取，直接使用已有数据。
// 返回 error 仅当 GetHoldings 失败，供 poll 汇总告警。
func monitorPortfolio(cfg config.Config, client *xueqiu.Client, p config.Portfolio, preloaded map[string][]model.Holding) error {
	log.Printf("--- 检测 %s（%s）---", p.Name, p.ID)

	var current []model.Holding
	if hs, ok := preloaded[p.ID]; ok {
		current = hs
		log.Printf("当前持仓：%d 只（复用启动数据）", len(current))
	} else {
		var err error
		current, err = client.GetHoldings(p.ID)
		if err != nil {
			log.Printf("[ERROR] 抓取失败: %v", err)
			return err
		}
		log.Printf("当前持仓：%d 只", len(current))
	}

	previous, err := snapshot.Load(snapshotDir, p.ID)
	if err != nil {
		log.Printf("[ERROR] 读取快照失败: %v", err)
		return nil
	}

	if len(current) == 0 && len(previous) > 0 {
		log.Printf("[ERROR] 抓到空持仓但历史非空（%d 只），疑似 cookie 失效，本次跳过", len(previous))
		return nil
	}

	if previous == nil {
		if len(current) == 0 {
			log.Println("首次运行但抓到空持仓，跳过")
			return nil
		}
		log.Println("首次运行，保存快照，无需通知")
		if err := snapshot.Save(snapshotDir, p.ID, current); err != nil {
			log.Printf("[ERROR] 保存快照失败: %v", err)
		}
		return nil
	}

	if snapshot.Fingerprint(previous) == snapshot.Fingerprint(current) {
		log.Println("持仓无变化")
		return nil
	}

	diff := snapshot.Diff(previous, current, cfg.WeightChangeDelta)
	if !diff.HasChanges() {
		log.Println("持仓无显著变化（仓位微调低于阈值）")
		if err := snapshot.Save(snapshotDir, p.ID, current); err != nil {
			log.Printf("[ERROR] 保存快照失败: %v", err)
		}
		return nil
	}

	log.Printf("发现调仓！新增=%d 移除=%d 调仓=%d",
		len(diff.Added), len(diff.Removed), len(diff.Changed))

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
		log.Printf("[ERROR] 发送通知失败，跳过快照更新以确保下次重试: %v", err)
		return nil
	}

	if err := snapshot.Save(snapshotDir, p.ID, current); err != nil {
		log.Printf("[ERROR] 保存快照失败: %v", err)
	}
	return nil
}

// poll 执行一轮检测。preloaded 非 nil 时对应组合跳过 API 抓取（仅首轮复用启动数据）。
// 收集所有抓取失败的组合，全局限速后统一发一条汇总告警。
func poll(cfg config.Config, client *xueqiu.Client, lastErrNotify *time.Time, preloaded map[string][]model.Holding) {
	fetchErrors := make(map[string]string)
	for i, p := range cfg.Portfolios {
		if err := monitorPortfolio(cfg, client, p, preloaded); err != nil {
			fetchErrors[p.Name+"（"+p.ID+"）"] = err.Error()
		}
		if i < len(cfg.Portfolios)-1 {
			time.Sleep(2 * time.Second)
		}
	}

	if len(fetchErrors) > 0 && time.Since(*lastErrNotify) >= time.Hour {
		if err := notify.SendFetchErrors(cfg.PushplusToken, fetchErrors); err != nil {
			log.Printf("[ERROR] 发送抓取失败汇总警报失败: %v", err)
		} else {
			*lastErrNotify = time.Now()
		}
	}
}

func sendDailyHeartbeat(cfg config.Config, client *xueqiu.Client) {
	log.Println("--- 每日持仓播报 ---")
	holdings := make(map[string][]model.Holding)
	fetchErrors := make(map[string]string)
	for _, p := range cfg.Portfolios {
		if hs, err := client.GetHoldings(p.ID); err == nil {
			holdings[p.Name+"（"+p.ID+"）"] = hs
		} else {
			fetchErrors[p.Name+"（"+p.ID+"）"] = err.Error()
			log.Printf("[WARN] 每日播报获取 %s 持仓失败: %v", p.Name, err)
		}
	}
	if err := notify.SendDailySummary(cfg.PushplusToken, holdings, fetchErrors); err != nil {
		log.Printf("[ERROR] 每日持仓播报推送失败: %v", err)
	}
}

func main() {
	l, err := logger.Setup()
	if err != nil {
		log.Fatalf("日志初始化失败: %v", err)
	}
	defer l.Close()

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

	// 启动时抓取持仓：一方面推送概览通知，另一方面缓存数据供第一次 poll 复用，避免重复请求
	startupHoldings := make(map[string][]model.Holding) // keyed by portfolio ID，供 poll 复用
	summaryHoldings := make(map[string][]model.Holding) // keyed by display name，供通知展示
	for _, p := range cfg.Portfolios {
		if hs, err := client.GetHoldings(p.ID); err == nil {
			startupHoldings[p.ID] = hs
			summaryHoldings[p.Name+"（"+p.ID+"）"] = hs
			log.Printf("获取 %s 持仓 %d 只", p.Name, len(hs))
		} else {
			log.Printf("[WARN] 获取 %s 持仓失败: %v", p.Name, err)
		}
	}
	if cfg.PushplusToken != "" {
		if err := notify.SendStartupSummary(cfg.PushplusToken, summaryHoldings); err != nil {
			log.Printf("[ERROR] 启动概览推送失败: %v", err)
		}
	}

	var lastErrNotify time.Time

	// 程序启动时已在 8 点后，跳过当天心跳（启动通知已覆盖）；启动时在 8 点前，当天 8 点正常触发
	lastHeartbeatDate := ""
	if time.Now().Hour() >= 8 {
		lastHeartbeatDate = time.Now().Format("2006-01-02")
	}

	log.Printf("进入常驻模式，每 %d 秒检测一次（修改 config.json 自动热加载）", cfg.Interval)

	// 第一次 poll 复用启动数据，后续正常抓取
	poll(cfg, client, &lastErrNotify, startupHoldings)

	for {
		log.Printf("--- 下次检测：%s ---", time.Now().Add(time.Duration(cfg.Interval)*time.Second).Format("15:04:05"))
		time.Sleep(time.Duration(cfg.Interval) * time.Second)

		if newCfg, reloaded := config.TryReload(configFile, &configLastMod, cfg); reloaded {
			cfg = newCfg
			client = xueqiu.NewClient(cfg.XueqiuCookie)
		}

		// 每日早 8 点推送持仓播报
		if now := time.Now(); now.Hour() >= 8 && now.Format("2006-01-02") != lastHeartbeatDate {
			lastHeartbeatDate = now.Format("2006-01-02")
			sendDailyHeartbeat(cfg, client)
		}

		poll(cfg, client, &lastErrNotify, nil)
	}
}
