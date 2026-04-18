package snapshot

import (
	"testing"

	"xueqiu-monitor/internal/model"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	holdings := []model.Holding{
		{Symbol: "SH601857", Name: "中国石油", Weight: 50.0},
		{Symbol: "SH601919", Name: "中远海控", Weight: 35.0},
	}

	if err := Save(dir, "ZH001", holdings); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(dir, "ZH001")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("loaded %d holdings, want 2", len(loaded))
	}
	if loaded[0].Symbol != "SH601857" || loaded[0].Weight != 50.0 {
		t.Errorf("first holding = %+v", loaded[0])
	}
}

func TestLoad_NotExist(t *testing.T) {
	holdings, err := Load(t.TempDir(), "NONEXIST")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if holdings != nil {
		t.Errorf("expected nil for non-existent snapshot, got %v", holdings)
	}
}

func TestFingerprint_Consistent(t *testing.T) {
	h := []model.Holding{{Symbol: "A", Weight: 10}, {Symbol: "B", Weight: 20}}
	fp1 := Fingerprint(h)
	fp2 := Fingerprint(h)
	if fp1 != fp2 {
		t.Errorf("fingerprint not consistent: %s vs %s", fp1, fp2)
	}
}

func TestFingerprint_DiffersOnChange(t *testing.T) {
	h1 := []model.Holding{{Symbol: "A", Weight: 10}}
	h2 := []model.Holding{{Symbol: "A", Weight: 11}}
	if Fingerprint(h1) == Fingerprint(h2) {
		t.Error("fingerprint should differ when weight changes")
	}
}

func TestDiff_Added(t *testing.T) {
	old := []model.Holding{
		{Symbol: "A", Name: "股票A", Weight: 50},
	}
	new := []model.Holding{
		{Symbol: "A", Name: "股票A", Weight: 50},
		{Symbol: "B", Name: "股票B", Weight: 30},
	}

	d := Diff(old, new, 0.5)

	if len(d.Added) != 1 || d.Added[0].Symbol != "B" {
		t.Errorf("Added = %v, want [B]", d.Added)
	}
	if len(d.Removed) != 0 {
		t.Errorf("Removed = %v, want empty", d.Removed)
	}
	if len(d.Changed) != 0 {
		t.Errorf("Changed = %v, want empty", d.Changed)
	}
}

func TestDiff_Removed(t *testing.T) {
	old := []model.Holding{
		{Symbol: "A", Name: "股票A", Weight: 50},
		{Symbol: "B", Name: "股票B", Weight: 30},
	}
	new := []model.Holding{
		{Symbol: "A", Name: "股票A", Weight: 50},
	}

	d := Diff(old, new, 0.5)

	if len(d.Removed) != 1 || d.Removed[0].Symbol != "B" {
		t.Errorf("Removed = %v, want [B]", d.Removed)
	}
	if len(d.Added) != 0 {
		t.Errorf("Added = %v, want empty", d.Added)
	}
}

func TestDiff_Changed_AboveThreshold(t *testing.T) {
	old := []model.Holding{
		{Symbol: "A", Name: "股票A", Weight: 50.0},
	}
	new := []model.Holding{
		{Symbol: "A", Name: "股票A", Weight: 52.0},
	}

	d := Diff(old, new, 0.5)

	if len(d.Changed) != 1 {
		t.Fatalf("Changed count = %d, want 1", len(d.Changed))
	}
	if d.Changed[0].Delta != 2.0 {
		t.Errorf("Delta = %v, want 2.0", d.Changed[0].Delta)
	}
	if d.Changed[0].OldWeight != 50.0 {
		t.Errorf("OldWeight = %v, want 50.0", d.Changed[0].OldWeight)
	}
}

func TestDiff_Changed_BelowThreshold(t *testing.T) {
	old := []model.Holding{
		{Symbol: "A", Name: "股票A", Weight: 50.0},
	}
	new := []model.Holding{
		{Symbol: "A", Name: "股票A", Weight: 50.3},
	}

	d := Diff(old, new, 0.5)

	if len(d.Changed) != 0 {
		t.Errorf("Changed = %v, want empty (below threshold)", d.Changed)
	}
}

func TestDiff_Mixed(t *testing.T) {
	old := []model.Holding{
		{Symbol: "A", Name: "股票A", Weight: 30},
		{Symbol: "B", Name: "股票B", Weight: 40},
		{Symbol: "C", Name: "股票C", Weight: 30},
	}
	new := []model.Holding{
		{Symbol: "A", Name: "股票A", Weight: 35}, // 调仓 +5
		{Symbol: "C", Name: "股票C", Weight: 30}, // 不变
		{Symbol: "D", Name: "股票D", Weight: 35}, // 新增
	}
	// B 被移除

	d := Diff(old, new, 0.5)

	if len(d.Added) != 1 || d.Added[0].Symbol != "D" {
		t.Errorf("Added = %v, want [D]", d.Added)
	}
	if len(d.Removed) != 1 || d.Removed[0].Symbol != "B" {
		t.Errorf("Removed = %v, want [B]", d.Removed)
	}
	if len(d.Changed) != 1 || d.Changed[0].Symbol != "A" {
		t.Errorf("Changed = %v, want [A]", d.Changed)
	}
	if !d.HasChanges() {
		t.Error("HasChanges() should be true")
	}
}

func TestDiff_NoChanges(t *testing.T) {
	h := []model.Holding{
		{Symbol: "A", Weight: 50},
		{Symbol: "B", Weight: 50},
	}

	d := Diff(h, h, 0.5)

	if d.HasChanges() {
		t.Error("HasChanges() should be false for identical holdings")
	}
}
