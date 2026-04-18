package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// Portfolio 要监控的雪球组合
type Portfolio struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Config 运行时配置
type Config struct {
	Portfolios        []Portfolio `json:"portfolios"`
	XueqiuCookie      string      `json:"xueqiu_cookie"`
	PushplusToken     string      `json:"pushplus_token"`
	WeightChangeDelta float64     `json:"weight_change_delta"`
	TotalAmount       float64     `json:"total_amount"` // 总投资金额（元）
	Interval          int         `json:"interval"`     // 轮询间隔（秒），0 表示单次运行
}

var defaultPortfolios = []Portfolio{
	{ID: "ZH1004909", Name: "某大V组合"},
}

// Load 读取配置文件，env 变量覆盖敏感字段，空值补默认值
func Load(path string) Config {
	var cfg Config
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			log.Printf("[WARN] 解析 %s 失败: %v，使用默认值", path, err)
		}
	} else if !os.IsNotExist(err) {
		log.Printf("[WARN] 读取 %s 失败: %v", path, err)
	}

	if v := os.Getenv("XUEQIU_COOKIE"); v != "" {
		cfg.XueqiuCookie = v
	}
	if v := os.Getenv("PUSHPLUS_TOKEN"); v != "" {
		cfg.PushplusToken = v
	}

	if cfg.WeightChangeDelta <= 0 {
		cfg.WeightChangeDelta = 0.5
	}
	if len(cfg.Portfolios) == 0 {
		cfg.Portfolios = defaultPortfolios
	}
	return cfg
}

// TryReload 检查配置文件是否有变更，有则重新加载并打印变更摘要。
// lastMod 会被就地更新为文件的最新修改时间。
func TryReload(path string, lastMod *time.Time, old Config) (Config, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return old, false
	}
	modTime := info.ModTime()
	if !modTime.After(*lastMod) {
		return old, false
	}

	cfg := Load(path)
	*lastMod = modTime

	var changes []string
	if old.TotalAmount != cfg.TotalAmount {
		changes = append(changes, fmt.Sprintf("总金额 %.0f→%.0f", old.TotalAmount, cfg.TotalAmount))
	}
	if old.WeightChangeDelta != cfg.WeightChangeDelta {
		changes = append(changes, fmt.Sprintf("仓位阈值 %.2f→%.2f", old.WeightChangeDelta, cfg.WeightChangeDelta))
	}
	if old.Interval != cfg.Interval {
		changes = append(changes, fmt.Sprintf("轮询间隔 %d→%ds", old.Interval, cfg.Interval))
	}
	if len(old.Portfolios) != len(cfg.Portfolios) {
		changes = append(changes, fmt.Sprintf("组合数 %d→%d", len(old.Portfolios), len(cfg.Portfolios)))
	}
	if old.XueqiuCookie != cfg.XueqiuCookie {
		changes = append(changes, "雪球cookie已更新")
	}
	if old.PushplusToken != cfg.PushplusToken {
		changes = append(changes, "pushplus_token已更新")
	}
	if len(changes) > 0 {
		log.Printf("[HOT-RELOAD] 配置已更新：%s", strings.Join(changes, "，"))
	} else {
		log.Println("[HOT-RELOAD] 配置文件已变更（内容无实质差异）")
	}
	return cfg, true
}
