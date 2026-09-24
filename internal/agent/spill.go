package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	sb.WriteString(fmt.Sprintf("\n\n[输出过长：原 %d 字节已截断；完整内容见 %s（可用 Read 工具分段查看）]\n\n",
		len(content), path))
	sb.WriteString(content[len(content)-tail:])
	return sb.String()
}

// writeSpill 把内容写入 dir/<sha16>.txt（幂等：已存在则不重复写），返回该路径。
func writeSpill(dir, content string) string {
	if strings.TrimSpace(dir) == "" {
		dir = filepath.Join(os.TempDir(), "licode-spills")
	}
	_ = os.MkdirAll(dir, 0o755)
	sum := sha256.Sum256([]byte(content))
	name := hex.EncodeToString(sum[:8]) + ".txt"
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err != nil {
		_ = os.WriteFile(path, []byte(content), 0o600)
	}
	return path
}
