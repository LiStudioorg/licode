package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// spillOutput 把超长的工具结果“溢出”落盘，只把有界预览放回上下文。
// 这是三层裁剪管道里的第 1 层（采集时即有界，防止单次巨量输出污染整个会话历史）。
// 返回内容长度 <= limit（外加固定开销）。limit<=0 或内容未超限时原样返回。
//
// 落盘确定性：同一 (dir, 内容) 得到同一路径（内容哈希命名），便于重复读取；
// 不写时间戳，不依赖 locale。dir 为空时落到系统临时目录下的 licode-spills。
func spillOutput(dir, toolName, content string, limit int) string {
	if limit <= 0 || len(content) <= limit {
		return content
	}
	path := writeSpill(dir, content)
	head := limit / 2
	tail := limit - head
	var sb strings.Builder
	sb.WriteString(content[:head])
	if path == "" {
		// 落盘失败必须如实说明：旧实现照常输出“完整内容见 <路径>”，
		// 模型随后反复 Read 一个不存在的文件，浪费整个迭代预算。
		sb.WriteString(fmt.Sprintf("\n\n[输出过长：%d 字节已截断；溢出落盘失败，无法提供完整内容]\n\n", len(content)))
	} else {
		sb.WriteString(fmt.Sprintf("\n\n[输出过长：原 %d 字节已截断；完整内容见 %s（可用 Read 工具分段查看）]\n\n", len(content), path))
	}
	sb.WriteString(content[len(content)-tail:])
	return sb.String()
}

// spillTTL 溢出文件保留时长：spill 目录默认在临时目录里，无人清理会无限增长，
// 磁盘最终写满。每次落盘顺带清一次过期文件（开销为一次目录扫描）。
const spillTTL = 7 * 24 * time.Hour

// writeSpill 把内容写入 dir/<sha16>.txt（幂等：存在则不重复写），返回该路径；
// 失败返回 ""（调用方据此改变提示文案）。
func writeSpill(dir, content string) string {
	if strings.TrimSpace(dir) == "" {
		dir = filepath.Join(os.TempDir(), "licode-spills")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ""
	}
	sum := sha256.Sum256([]byte(content))
	name := hex.EncodeToString(sum[:8]) + ".txt"
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err != nil {
		// 原子写：半截的 spill 文件会让模型读到残缺的“完整内容”，比不落盘更糟。
		tmp, terr := os.CreateTemp(dir, ".tmp-*")
		if terr != nil {
			return ""
		}
		tmpName := tmp.Name()
		defer os.Remove(tmpName)
		if _, werr := tmp.WriteString(content); werr != nil {
			tmp.Close()
			return ""
		}
		if cerr := tmp.Close(); cerr != nil {
			return ""
		}
		_ = os.Chmod(tmpName, 0o600)
		if rerr := os.Rename(tmpName, path); rerr != nil {
			return ""
		}
	}
	go cleanupSpills(dir)
	return path
}

// cleanupSpills 删除超过 spillTTL 的溢出文件；尽力而为，静默失败。
func cleanupSpills(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-spillTTL)
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".tmp-") {
			continue
		}
		if info, err := e.Info(); err == nil && info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
