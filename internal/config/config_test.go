package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{
		"portfolios": [{"id": "ZH001", "name": "测试组合"}],
		"xueqiu_cookie": "test_cookie",
		"pushplus_token": "test_token",
		"weight_change_delta": 1.0,
		"total_amount": 300000,
		"interval": 60
	}`), 0644)

	cfg := Load(path)

	if len(cfg.Portfolios) != 1 || cfg.Portfolios[0].ID != "ZH001" {
		t.Errorf("portfolios = %v, want [{ZH001 测试组合}]", cfg.Portfolios)
	}
	if cfg.XueqiuCookie != "test_cookie" {
		t.Errorf("cookie = %q, want %q", cfg.XueqiuCookie, "test_cookie")
	}
	if cfg.PushplusToken != "test_token" {
		t.Errorf("token = %q, want %q", cfg.PushplusToken, "test_token")
	}
	if cfg.WeightChangeDelta != 1.0 {
		t.Errorf("delta = %v, want 1.0", cfg.WeightChangeDelta)
	}
	if cfg.TotalAmount != 300000 {
		t.Errorf("total_amount = %v, want 300000", cfg.TotalAmount)
	}
	if cfg.Interval != 60 {
		t.Errorf("interval = %v, want 60", cfg.Interval)
	}
}

func TestLoad_Defaults(t *testing.T) {
	cfg := Load("/nonexistent/config.json")

	if cfg.WeightChangeDelta != 0.5 {
		t.Errorf("default delta = %v, want 0.5", cfg.WeightChangeDelta)
	}
	if len(cfg.Portfolios) != 1 || cfg.Portfolios[0].ID != "ZH1004909" {
		t.Errorf("default portfolios = %v", cfg.Portfolios)
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{"xueqiu_cookie": "file_cookie"}`), 0644)

	t.Setenv("XUEQIU_COOKIE", "env_cookie")
	t.Setenv("PUSHPLUS_TOKEN", "env_token")

	cfg := Load(path)

	if cfg.XueqiuCookie != "env_cookie" {
		t.Errorf("cookie = %q, want %q (env should override file)", cfg.XueqiuCookie, "env_cookie")
	}
	if cfg.PushplusToken != "env_token" {
		t.Errorf("token = %q, want %q", cfg.PushplusToken, "env_token")
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{invalid json`), 0644)

	cfg := Load(path)

	// 应该回退到默认值
	if cfg.WeightChangeDelta != 0.5 {
		t.Errorf("delta = %v, want 0.5 (should use default on invalid JSON)", cfg.WeightChangeDelta)
	}
}

func TestTryReload_NoChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{"total_amount": 100000}`), 0644)

	info, _ := os.Stat(path)
	lastMod := info.ModTime()
	old := Load(path)

	cfg, reloaded := TryReload(path, &lastMod, old)
	if reloaded {
		t.Error("should not reload when file hasn't changed")
	}
	if cfg.TotalAmount != old.TotalAmount {
		t.Error("config should be unchanged")
	}
}

func TestTryReload_WithChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{"total_amount": 100000}`), 0644)

	info, _ := os.Stat(path)
	lastMod := info.ModTime()
	old := Load(path)

	// 修改文件（确保时间戳变化）
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(path, []byte(`{"total_amount": 200000}`), 0644)

	cfg, reloaded := TryReload(path, &lastMod, old)
	if !reloaded {
		t.Error("should reload when file has changed")
	}
	if cfg.TotalAmount != 200000 {
		t.Errorf("total_amount = %v, want 200000", cfg.TotalAmount)
	}
}
