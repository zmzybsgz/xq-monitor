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
	entries, _ := os.ReadDir(logDir)
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, filePrefix) || !strings.HasSuffix(name, ".log") {
			continue
		}
		if strings.Contains(name, today) {
			continue
		}
		src := filepath.Join(logDir, name)
		if err := gzipFile(src); err != nil {
			log.Printf("[WARN] 压缩日志失败 %s: %v", name, err)
		}
	}
}

// cleanOld 删除超过 maxKeepDay 天的日志
func (l *Logger) cleanOld() {
	cutoff := time.Now().AddDate(0, 0, -maxKeepDay)
	entries, _ := os.ReadDir(logDir)
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, filePrefix) {
			continue
		}
		dateStr := strings.TrimPrefix(name, filePrefix)
		dateStr = strings.TrimSuffix(dateStr, ".gz")
		dateStr = strings.TrimSuffix(dateStr, ".log")
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		if t.Before(cutoff) {
			os.Remove(filepath.Join(logDir, name))
		}
	}
}

func gzipFile(src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(src + ".gz")
	if err != nil {
		return err
	}
	defer out.Close()

	gz := gzip.NewWriter(out)
	gz.Name = filepath.Base(src)
	if _, err := io.Copy(gz, in); err != nil {
		os.Remove(src + ".gz")
		return err
	}
	if err := gz.Close(); err != nil {
		os.Remove(src + ".gz")
		return err
	}

	return os.Remove(src)
}
