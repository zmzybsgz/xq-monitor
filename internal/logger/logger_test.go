package logger

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const testPrefix = "xq-test-"

func writeFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("写入测试文件失败 %s: %v", path, err)
	}
}

func TestGzipFile_Atomic(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "sample.log")
	content := []byte("hello\nworld\n")
	writeFile(t, src, content)

	if err := gzipFile(src); err != nil {
		t.Fatalf("gzipFile 失败: %v", err)
	}

	// 原文件应已删除
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("原文件应被删除，err=%v", err)
	}
	// 临时文件不应残留
	if _, err := os.Stat(src + ".gz.tmp"); !os.IsNotExist(err) {
		t.Errorf(".gz.tmp 不应残留，err=%v", err)
	}
	// .gz 文件应存在且解压后内容一致
	gz := src + ".gz"
	f, err := os.Open(gz)
	if err != nil {
		t.Fatalf("打开 .gz 失败: %v", err)
	}
	defer f.Close()
	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip 解码失败: %v", err)
	}
	defer gr.Close()
	got, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("读取解压数据失败: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("解压内容 = %q, want %q", got, content)
	}
}

func TestGzipFile_SourceMissing(t *testing.T) {
	err := gzipFile(filepath.Join(t.TempDir(), "no-such.log"))
	if err == nil {
		t.Error("源文件不存在时应返回错误")
	}
}

func TestCompressOldFiles(t *testing.T) {
	dir := t.TempDir()
	today := "2026-05-01"

	todayLog := filepath.Join(dir, testPrefix+today+".log")
	yesterdayLog := filepath.Join(dir, testPrefix+"2026-04-30.log")
	otherLog := filepath.Join(dir, "other.log")
	writeFile(t, todayLog, []byte("today"))
	writeFile(t, yesterdayLog, []byte("yesterday"))
	writeFile(t, otherLog, []byte("other"))

	if err := compressOldFiles(dir, testPrefix, today); err != nil {
		t.Fatalf("compressOldFiles 失败: %v", err)
	}

	// 今天的应保持 .log
	if _, err := os.Stat(todayLog); err != nil {
		t.Errorf("今天的日志应保留: %v", err)
	}
	// 昨天的应被压缩
	if _, err := os.Stat(yesterdayLog + ".gz"); err != nil {
		t.Errorf("昨天的日志应被压缩为 .gz: %v", err)
	}
	if _, err := os.Stat(yesterdayLog); !os.IsNotExist(err) {
		t.Errorf("昨天的原 .log 应被删除，err=%v", err)
	}
	// 不带前缀的文件保持不动
	if _, err := os.Stat(otherLog); err != nil {
		t.Errorf("无关文件应保留: %v", err)
	}
}

func TestCompressOldFiles_DirNotExist(t *testing.T) {
	err := compressOldFiles(filepath.Join(t.TempDir(), "no-such-dir"), testPrefix, "2026-05-01")
	if err == nil {
		t.Error("目录不存在时应返回错误")
	}
}

func TestCleanOldFiles(t *testing.T) {
	dir := t.TempDir()

	oldLog := filepath.Join(dir, testPrefix+"2026-04-23.log")     // 8 天前
	oldGz := filepath.Join(dir, testPrefix+"2026-04-22.log.gz")   // 9 天前
	recentGz := filepath.Join(dir, testPrefix+"2026-04-26.log.gz") // 5 天前
	invalid := filepath.Join(dir, testPrefix+"invalid-name.log")
	other := filepath.Join(dir, "no-prefix.log")
	for _, p := range []string{oldLog, oldGz, recentGz, invalid, other} {
		writeFile(t, p, []byte("x"))
	}

	// cutoff 设为 2026-04-24，比它早的应被删
	cutoff := time.Date(2026, 4, 24, 0, 0, 0, 0, time.UTC)
	if err := cleanOldFiles(dir, testPrefix, cutoff); err != nil {
		t.Fatalf("cleanOldFiles 失败: %v", err)
	}

	if _, err := os.Stat(oldLog); !os.IsNotExist(err) {
		t.Errorf("8 天前的日志应被删，err=%v", err)
	}
	if _, err := os.Stat(oldGz); !os.IsNotExist(err) {
		t.Errorf("9 天前的 .gz 应被删，err=%v", err)
	}
	if _, err := os.Stat(recentGz); err != nil {
		t.Errorf("5 天前的 .gz 应保留: %v", err)
	}
	// 文件名日期解析失败的，不删（避免误删）
	if _, err := os.Stat(invalid); err != nil {
		t.Errorf("日期解析失败的文件应保留: %v", err)
	}
	// 不带前缀的文件不删
	if _, err := os.Stat(other); err != nil {
		t.Errorf("无关文件应保留: %v", err)
	}
}

func TestCleanOldFiles_BoundaryEqualsCutoff(t *testing.T) {
	dir := t.TempDir()
	// cutoff 与文件日期相等：t.Before(cutoff) 为 false，应保留
	equal := filepath.Join(dir, testPrefix+"2026-04-24.log")
	writeFile(t, equal, []byte("x"))

	cutoff := time.Date(2026, 4, 24, 0, 0, 0, 0, time.UTC)
	if err := cleanOldFiles(dir, testPrefix, cutoff); err != nil {
		t.Fatalf("cleanOldFiles 失败: %v", err)
	}
	if _, err := os.Stat(equal); err != nil {
		t.Errorf("与 cutoff 同日的文件应保留: %v", err)
	}
}

func TestCleanOldFiles_DirNotExist(t *testing.T) {
	err := cleanOldFiles(filepath.Join(t.TempDir(), "no-such-dir"), testPrefix, time.Now())
	if err == nil {
		t.Error("目录不存在时应返回错误")
	}
}
