package logger

import (
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	logDir     = "logs"
	maxKeepDay = 7
	filePrefix = "xq-monitor-"
)

// Logger 日志管理：按天切割、gzip 压缩、自动清理
type Logger struct {
	file *os.File
	date string
}

// Setup 初始化日志，输出到终端+文件，启动后台轮转协程
func Setup() (*Logger, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	l := &Logger{}
	if err := l.openToday(); err != nil {
		return nil, err
	}

	l.compressOld()
	l.cleanOld()

	go l.backgroundRotate()

	return l, nil
}

// Close 关闭日志文件
func (l *Logger) Close() {
	if l.file != nil {
		l.file.Close()
	}
}

func (l *Logger) openToday() error {
	today := time.Now().Format("2006-01-02")
	if l.date == today {
		return nil
	}

	if l.file != nil {
		l.file.Close()
	}

	filename := filepath.Join(logDir, filePrefix+today+".log")
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}

	l.file = f
	l.date = today
	log.SetOutput(io.MultiWriter(os.Stdout, f))

	return nil
}

func (l *Logger) backgroundRotate() {
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 1, 0, now.Location())
		time.Sleep(next.Sub(now))

		if err := l.openToday(); err != nil {
			log.Printf("[ERROR] 日志轮转失败: %v", err)
			continue
		}
		l.compressOld()
		l.cleanOld()
	}
}

// compressOld 压缩非今天的 .log 文件为 .log.gz
func (l *Logger) compressOld() {
	today := time.Now().Format("2006-01-02")
	if err := compressOldFiles(logDir, filePrefix, today); err != nil {
		log.Printf("[WARN] 读取日志目录失败: %v", err)
	}
}

// cleanOld 删除超过 maxKeepDay 天的日志
func (l *Logger) cleanOld() {
	cutoff := time.Now().AddDate(0, 0, -maxKeepDay)
	if err := cleanOldFiles(logDir, filePrefix, cutoff); err != nil {
		log.Printf("[WARN] 读取日志目录失败: %v", err)
	}
}

// compressOldFiles 压缩 dir 下所有匹配 prefix 且非 today 日期的 .log 文件为 .log.gz。
// 单个文件压缩失败仅打印 WARN，不中断后续文件，但读取目录失败会返回错误。
func compressOldFiles(dir, prefix, today string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".log") {
			continue
		}
		if strings.Contains(name, today) {
			continue
		}
		src := filepath.Join(dir, name)
		if err := gzipFile(src); err != nil {
			log.Printf("[WARN] 压缩日志失败 %s: %v", name, err)
		}
	}
	return nil
}

// cleanOldFiles 删除 dir 下匹配 prefix、文件名日期早于 cutoff 的日志文件。
// 单个文件删除失败仅打印 WARN，不中断后续文件，但读取目录失败会返回错误。
func cleanOldFiles(dir, prefix string, cutoff time.Time) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		dateStr := strings.TrimPrefix(name, prefix)
		dateStr = strings.TrimSuffix(dateStr, ".gz")
		dateStr = strings.TrimSuffix(dateStr, ".log")
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		if !t.Before(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			log.Printf("[WARN] 删除过期日志失败 %s: %v", name, err)
		}
	}
	return nil
}

func gzipFile(src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// 先写到临时文件，完成后原子 Rename，避免中途崩溃留下损坏的 .gz
	tmp := src + ".gz.tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}

	gz := gzip.NewWriter(out)
	gz.Name = filepath.Base(src)
	if _, err := io.Copy(gz, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := gz.Close(); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}

	if err := os.Rename(tmp, src+".gz"); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Remove(src)
}
