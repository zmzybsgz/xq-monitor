package snapshot

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"xueqiu-monitor/internal/model"
)

// Load 读取快照文件，文件不存在返回 (nil, nil)
func Load(dir, portfolioID string) ([]model.Holding, error) {
	data, err := os.ReadFile(filepath.Join(dir, portfolioID+".json"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var holdings []model.Holding
	err = json.Unmarshal(data, &holdings)
	return holdings, err
}

// Save 保存快照文件
func Save(dir, portfolioID string, holdings []model.Holding) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(holdings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, portfolioID+".json"), data, 0644)
}

// Fingerprint 计算持仓数据的 MD5 指纹
func Fingerprint(holdings []model.Holding) string {
	data, _ := json.Marshal(holdings)
	return fmt.Sprintf("%x", md5.Sum(data))
}

// Diff 对比新旧持仓，返回变动。threshold 为仓位变动阈值（%）。
func Diff(old, new []model.Holding, threshold float64) model.Diff {
	oldMap := make(map[string]model.Holding, len(old))
	for _, h := range old {
		oldMap[h.Symbol] = h
	}
	newMap := make(map[string]model.Holding, len(new))
	for _, h := range new {
		newMap[h.Symbol] = h
	}

	var diff model.Diff

	for sym, h := range newMap {
		if prev, exists := oldMap[sym]; !exists {
			diff.Added = append(diff.Added, h)
		} else {
			delta := math.Round((h.Weight-prev.Weight)*100) / 100
			if math.Abs(delta) >= threshold {
				diff.Changed = append(diff.Changed, model.HoldingChange{
					Holding:   h,
					OldWeight: prev.Weight,
					Delta:     delta,
				})
			}
		}
	}

	for sym, h := range oldMap {
		if _, exists := newMap[sym]; !exists {
			diff.Removed = append(diff.Removed, h)
		}
	}

	return diff
}
