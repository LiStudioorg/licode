package agent

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"licode/internal/procutil"
)

var (
	allowedCommandPattern = regexp.MustCompile(`^[a-zA-Z0-9_./-]+$`)
	workspaceRoot         atomic.Value // string：AI 工具的工作目录
)

func init() {
	if wd, err := os.Getwd(); err == nil {
		workspaceRoot.Store(filepath.Clean(wd))
	}
}

// SetWorkspaceRoot 设置 AI 工具的工作目录：文件边界、相对路径基准与 Shell 默认 cwd。
func SetWorkspaceRoot(dir string) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return
	}
	if abs, err := filepath.Abs(dir); err == nil {
		workspaceRoot.Store(filepath.Clean(abs))
	}
}

// WorkspaceRoot 返回 AI 工具当前工作目录（默认进程启动目录）。
func WorkspaceRoot() string {
	if v, ok := workspaceRoot.Load().(string); ok {
		return v
	}
	return ""
}

// pathApprover 是工作目录之外路径的人工确认钩子：返回 true 表示用户已批准
// 本次访问（一次一个路径），false / 错误表示拒绝。由 Agent 在执行工具前注入。
type pathApprover func(ctx context.Context, path, tool string) bool

// approveCtxKey 用于向工具执行传递确认钩子。
type approveCtxKey struct{}

// withPathApprover 把路径确认钩子放入工具执行上下文。
func withPathApprover(ctx context.Context, fn pathApprover) context.Context {
	return context.WithValue(ctx, approveCtxKey{}, fn)
}

func getPathApprover(ctx context.Context) pathApprover {
	if fn, ok := ctx.Value(approveCtxKey{}).(pathApprover); ok {
		return fn
	}
	return nil
}

// ensurePathAllowed 校验路径：工作目录内直接放行；工作目录外先请求用户
// 确认（弹窗会展示工具名与完整路径），批准后才放行。无确认通道时拒绝。
// 路径中包含 ".." 一律拒绝（相对路径穿越没有合法场景）。
func ensurePathAllowed(ctx context.Context, path, tool string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path required")
	}
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("path contains ..")
	}
	root := WorkspaceRoot()
	if root == "" {
		return "", fmt.Errorf("path outside allowed directories")
	}
	// 相对路径以工作目录为基准解析（与用户看到的「工作目录」一致）。
	var clean string
	if filepath.IsAbs(path) {
		clean = filepath.Clean(path)
	} else {
		clean = filepath.Clean(filepath.Join(root, path))
	}
	inWorkdir := func(p string) bool {
		rel, err := filepath.Rel(root, p)
		return err == nil && !strings.HasPrefix(rel, "..") && rel != ".."
	}
	if !inWorkdir(clean) {
		approve := getPathApprover(ctx)
		if approve == nil || !approve(ctx, clean, tool) {
			return "", fmt.Errorf("路径 %s 在工作目录之外，且未被用户批准（需要访问请让用户确认）", clean)
		}
		return clean, nil
	}
	// 工作目录内的符号链接解析：目标不存在（Write 新建文件）时校验最近
	// 存在的父目录，防止经符号链接绕过边界检查，也让新建文件场景可用。
	target := clean
	if _, lerr := os.Lstat(clean); lerr != nil {
		if !os.IsNotExist(lerr) {
			return "", lerr
		}
		parent, perr := filepath.EvalSymlinks(filepath.Dir(clean))
		if perr != nil {
			return "", perr
		}
		target = filepath.Join(parent, filepath.Base(clean))
	} else {
		resolved, rerr := filepath.EvalSymlinks(clean)
		if rerr != nil {
			return "", rerr
		}
		target = resolved
	}
	cleanResolved := filepath.Clean(target)
	if !inWorkdir(cleanResolved) {
		// 符号链接指向工作目录之外：同样需要用户确认，放行真实目标。
		approve := getPathApprover(ctx)
		if approve == nil || !approve(ctx, cleanResolved, tool) {
			return "", fmt.Errorf("路径 %s 经符号链接指向工作目录之外，且未被用户批准", cleanResolved)
		}
		return cleanResolved, nil
	}
	return cleanResolved, nil
}

func strArg(args map[string]any, key string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func intArg(args map[string]any, key string, def int) int {
	if v, ok := args[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return def
}

// ShellConfig 配置 Shell 工具执行方式。
type ShellConfig struct {
	Path string // shell 可执行文件（默认 /bin/sh）
}

func (c *ShellConfig) resolve() {
	c.Path = strings.TrimSpace(c.Path)
	if c.Path == "" {
		c.Path = "/bin/sh"
	}
}

// RegisterDefaultTools installs the built-in coding tools on a registry.
func RegisterDefaultTools(r *Registry, sh ShellConfig) {
	sh.resolve()
	// Read - 读取文件
	r.Register(Tool{
		Name:        "Read",
		Description: "Read a text file. Returns the requested lines with line numbers; use offset/limit for large files.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":   map[string]any{"type": "string", "description": "Path to the file"},
				"offset": map[string]any{"type": "integer", "description": "1-based starting line"},
				"limit":  map[string]any{"type": "integer", "description": "Max lines to read (default 200)"},
			},
			"required": []string{"path"},
		},
		Run: func(ctx context.Context, args map[string]any) (string, error) {
			path := strArg(args, "path")
			path, err := ensurePathAllowed(ctx, path, "Read")
			if err != nil {
				return "", err
			}
			f, err := os.Open(path)
			if err != nil {
				return "", err
			}
			defer f.Close()
			offset := intArg(args, "offset", 1)
			limit := intArg(args, "limit", 200)
			if offset < 1 {
				offset = 1
			}
			var sb strings.Builder
			sc := bufio.NewScanner(f)
			sc.Buffer(make([]byte, 1024*1024), 1024*1024)
			line := 0
			skipped := 0
			for sc.Scan() {
				line++
				if line < offset {
					skipped++
					continue
				}
				if line >= offset+limit {
					break
				}
				fmt.Fprintf(&sb, "%d: %s\n", line, sc.Text())
			}
			if err := sc.Err(); err != nil {
				return "", err
			}
			if skipped > 0 {
				return fmt.Sprintf("(skipped %d lines before offset %d)\n%s", skipped, offset, sb.String()), nil
			}
			if sb.Len() == 0 {
				return "(empty or offset beyond end of file)", nil
			}
			return sb.String(), nil
		},
	})

	// Write - 写入文件
	r.Register(Tool{
		Name:        "Write",
		Description: "Write or replace a file with the given content. Creates parent directories automatically.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":    map[string]any{"type": "string", "description": "Path to write"},
				"content": map[string]any{"type": "string", "description": "Full file content"},
			},
			"required": []string{"path", "content"},
		},
		Run: func(ctx context.Context, args map[string]any) (string, error) {
			path := strArg(args, "path")
			path, err := ensurePathAllowed(ctx, path, "Write")
			if err != nil {
				return "", err
			}
			content := strArg(args, "content")
			if path == "" {
				return "", fmt.Errorf("path required")
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return "", err
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return "", err
			}
			return fmt.Sprintf("wrote %d bytes to %s", len(content), path), nil
		},
	})

	// Edit - 编辑文件（查找替换）
	r.Register(Tool{
		Name:        "Edit",
		Description: "Edit a file by finding and replacing specific text. Use for targeted changes without rewriting the whole file.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "Path to the file"},
				"old":  map[string]any{"type": "string", "description": "Exact text to find (must be unique in file)"},
				"new":  map[string]any{"type": "string", "description": "Replacement text"},
				"all":  map[string]any{"type": "boolean", "description": "Replace all occurrences (default: false, only first)"},
			},
			"required": []string{"path", "old", "new"},
		},
		Run: func(ctx context.Context, args map[string]any) (string, error) {
			path := strArg(args, "path")
			path, err := ensurePathAllowed(ctx, path, "Edit")
			if err != nil {
				return "", err
			}
			old := strArg(args, "old")
			new := strArg(args, "new")
			if old == "" {
				return "", fmt.Errorf("old text required")
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return "", err
			}
			content := string(data)
			if !strings.Contains(content, old) {
				return "", fmt.Errorf("old text not found in file")
			}
			count := strings.Count(content, old)
			replaceAll := false
			if v, ok := args["all"]; ok {
				if b, ok := v.(bool); ok {
					replaceAll = b
				}
			}
			if !replaceAll && count > 1 {
				return "", fmt.Errorf("old text appears %d times; use all=true or provide more context", count)
			}
			var result string
			if replaceAll {
				result = strings.ReplaceAll(content, old, new)
			} else {
				result = strings.Replace(content, old, new, 1)
			}
			if err := os.WriteFile(path, []byte(result), 0o644); err != nil {
				return "", err
			}
			replaced := 1
			if replaceAll {
				replaced = count
			}
			return fmt.Sprintf("replaced %d occurrence(s) in %s", replaced, path), nil
		},
	})

	// ListDirectory - 列出目录
	r.Register(Tool{
		Name:        "ListDirectory",
		Description: "List entries in a directory with file sizes.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "Directory to list"},
			},
			"required": []string{"path"},
		},
		Run: func(ctx context.Context, args map[string]any) (string, error) {
			path := strArg(args, "path")
			path, err := ensurePathAllowed(ctx, path, "ListDirectory")
			if err != nil {
				return "", err
			}
			if path == "" {
				path = "."
			}
			entries, err := os.ReadDir(path)
			if err != nil {
				return "", err
			}
			var sb strings.Builder
			for _, e := range entries {
				suffix := ""
				if e.IsDir() {
					suffix = "/"
				}
				info, ierr := e.Info()
				size := ""
				if ierr == nil {
					size = fmt.Sprintf(" %d", info.Size())
				}
				fmt.Fprintf(&sb, "%s%s%s\n", e.Name(), suffix, size)
			}
			return sb.String(), nil
		},
	})

	// Grep - 搜索文件内容
	r.Register(Tool{
		Name:        "Grep",
		Description: "Search file contents with a regex pattern. Returns matching file:line results.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{"type": "string", "description": "Regular expression pattern"},
				"include": map[string]any{"type": "string", "description": "File glob filter, e.g. *.go or *.ts"},
				"path":    map[string]any{"type": "string", "description": "Root directory to search (default: current dir)"},
			},
			"required": []string{"pattern"},
		},
		Run: func(ctx context.Context, args map[string]any) (string, error) {
			pattern := strArg(args, "pattern")
			include := strArg(args, "include")
			root := strArg(args, "path")
			if root == "" {
				root = "."
			}
			// 与 Read/Write 等工具保持一致的路径边界（默认 "." 即工作目录）。
			resolvedRoot, err := ensurePathAllowed(ctx, root, "Grep")
			if err != nil {
				return "", fmt.Errorf("invalid path: %w", err)
			}
			root = resolvedRoot
			if len(pattern) > 1000 {
				return "", fmt.Errorf("pattern too long")
			}
			if strings.Count(pattern, "*") > 10 {
				return "", fmt.Errorf("pattern too complex")
			}
			if _, err := exec.LookPath("rg"); err != nil {
				// rg 不存在时回退到内置遍历搜索，避免工具整体不可用。
				return grepFallback(ctx, pattern, include, root)
			}
			cmd := exec.CommandContext(ctx, "rg", "--line-number", "--no-heading", "-S")
			if include != "" {
				cmd.Args = append(cmd.Args, "-g", include)
			}
			cmd.Args = append(cmd.Args, "-g", "!.git", "-g", "!node_modules", "-g", "!vendor")
			cmd.Args = append(cmd.Args, pattern, root)
			out, err := cmd.CombinedOutput()
			if err != nil {
				if len(out) == 0 {
					return "(no matches)", nil
				}
			}
			s := string(out)
			if len(s) > 30000 {
				s = s[:30000] + "\n...(truncated)"
			}
			return s, nil
		},
	})

	// Glob - 按通配符查找文件
	r.Register(Tool{
		Name:        "Glob",
		Description: "Find files by glob pattern, e.g. *.go, src/**/*.ts (supports ** for recursion)",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{"type": "string", "description": "Glob pattern"},
			},
			"required": []string{"pattern"},
		},
		Run: func(ctx context.Context, args map[string]any) (string, error) {
			pattern := strArg(args, "pattern")
			if pattern == "" {
				return "", fmt.Errorf("pattern required")
			}
			absPattern := pattern
			if !filepath.IsAbs(absPattern) {
				absPattern = filepath.Join(WorkspaceRoot(), absPattern)
			}
			root := WorkspaceRoot()
			if root != "" {
				cleanPattern := filepath.Clean(absPattern)
				rel, _ := filepath.Rel(root, cleanPattern)
				if !strings.HasPrefix(rel, "..") && rel != ".." {
					matches, err := globMatches(cleanPattern)
					if err != nil {
						return "", err
					}
					if len(matches) == 0 {
						return "(no matches)", nil
					}
					return strings.Join(matches, "\n"), nil
				}
			}
			return "", fmt.Errorf("pattern must be within workspace")
		},
	})

	// Shell - 执行 shell 命令
	r.Register(Tool{
		Name:        "Shell",
		Description: "Execute a shell command and return its output. Use for building, testing, git, and system operations.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{"type": "string", "description": "Shell command to execute"},
				"timeout": map[string]any{"type": "integer", "description": "Timeout in seconds (default 30, max 300)"},
				"cwd":     map[string]any{"type": "string", "description": "Working directory for the command"},
			},
			"required": []string{"command"},
		},
		Run: func(ctx context.Context, args map[string]any) (string, error) {
			command := strArg(args, "command")
			if command == "" {
				return "", fmt.Errorf("command required")
			}
			if !safeCommand(command) {
				return "", fmt.Errorf("command contains disallowed characters or patterns")
			}
			cwd := strArg(args, "cwd")
			if cwd != "" {
				validatedCwd, err := ensurePathAllowed(ctx, cwd, "Shell")
				if err != nil {
					return "", fmt.Errorf("invalid cwd: %w", err)
				}
				cwd = validatedCwd
			}
			timeout := time.Duration(intArg(args, "timeout", 30)) * time.Second
			if timeout <= 0 {
				timeout = 30 * time.Second
			}
			if timeout > 300*time.Second {
				timeout = 300 * time.Second
			}
			cmdCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			bin, berr := procutil.ResolveEntry(sh.Path)
			if berr != nil {
				return "", berr
			}
			cmd := exec.CommandContext(cmdCtx, bin, "-c", command)
			if cwd == "" {
				cwd = WorkspaceRoot()
			}
			if cwd != "" {
				cmd.Dir = cwd
			}
			out, err := cmd.CombinedOutput()
			s := string(out)
			if len(s) > 30000 {
				s = s[:30000] + "\n...(truncated)"
			}
			if err != nil {
				return fmt.Sprintf("exit error: %v\n%s", err, s), nil
			}
			return s, nil
		},
	})

	// Delete - 删除文件
	r.Register(Tool{
		Name:        "Delete",
		Description: "Delete a file or empty directory.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "Path to delete"},
			},
			"required": []string{"path"},
		},
		Run: func(ctx context.Context, args map[string]any) (string, error) {
			path := strArg(args, "path")
			path, err := ensurePathAllowed(ctx, path, "Delete")
			if err != nil {
				return "", err
			}
			info, err := os.Stat(path)
			if err != nil {
				return "", err
			}
			if info.IsDir() {
				entries, _ := os.ReadDir(path)
				if len(entries) > 0 {
					return "", fmt.Errorf("directory not empty (%d entries); only empty directories can be deleted", len(entries))
				}
			}
			if err := os.Remove(path); err != nil {
				return "", err
			}
			return fmt.Sprintf("deleted %s", path), nil
		},
	})

	// Move - 移动/重命名文件
	r.Register(Tool{
		Name:        "Move",
		Description: "Move or rename a file/directory.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"source": map[string]any{"type": "string", "description": "Source path"},
				"dest":   map[string]any{"type": "string", "description": "Destination path"},
			},
			"required": []string{"source", "dest"},
		},
		Run: func(ctx context.Context, args map[string]any) (string, error) {
			source := strArg(args, "source")
			source, err := ensurePathAllowed(ctx, source, "Move")
			if err != nil {
				return "", err
			}
			dest := strArg(args, "dest")
			dest, err = ensurePathAllowed(ctx, dest, "Move")
			if err != nil {
				return "", err
			}
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return "", err
			}
			if err := os.Rename(source, dest); err != nil {
				return "", err
			}
			return fmt.Sprintf("moved %s -> %s", source, dest), nil
		},
	})
}

var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`rm\s+-rf\s+/`),
	regexp.MustCompile(`rm\s+-rf\s+~\s*`),
	regexp.MustCompile(`:\(\)\{\s*:\|:\&\}\s*;`),
	regexp.MustCompile(`>\s*/dev/sd`),
	regexp.MustCompile(`mkfs\s+`),
	regexp.MustCompile(`dd\s+.*of=`),
	// 常见“下载即执行”与越权提权写法，作为黑名单纵深防御（沙箱仍为主防线）。
	regexp.MustCompile(`\bcurl\b[^|]*\|[^|]*(ba)?sh\b`),
	regexp.MustCompile(`\bwget\b[^|]*\|[^|]*(ba)?sh\b`),
	regexp.MustCompile(`\beval\s+\$\(`),
	regexp.MustCompile(`\bchmod\s+(-R\s+)?777\b`),
	regexp.MustCompile(`\bsudo\s+i?ptables\b`),
	regexp.MustCompile(`\bkill\b.*\b-9\s+1`),
}

func safeCommand(command string) bool {
	lower := strings.ToLower(command)
	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(lower) {
			return false
		}
	}
	return true
}

// grepFallback 在系统缺少 rg 时内置遍历搜索（结果上限与截断与 rg 路径一致）。
func grepFallback(ctx context.Context, pattern, include, root string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid pattern: %w", err)
	}
	skipDirs := map[string]bool{".git": true, "node_modules": true, "vendor": true}
	var sb strings.Builder
	matches := 0
	root = filepath.Clean(root)
	stop := ctx.Err()
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if stop != nil || ctx.Err() != nil {
			return fs.SkipAll
		}
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != root && skipDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if include != "" {
			if ok, gerr := filepath.Match(include, d.Name()); gerr != nil || !ok {
				return nil
			}
		}
		data, rerr := os.ReadFile(p)
		if rerr != nil || len(data) > 8<<20 {
			return nil
		}
		for i, line := range strings.Split(string(data), "\n") {
			if re.MatchString(line) {
				fmt.Fprintf(&sb, "%s:%d:%s\n", p, i+1, line)
				matches++
				if matches >= 1000 || sb.Len() > 30000 {
					sb.WriteString("...(truncated)")
					return fs.SkipAll
				}
			}
		}
		return nil
	})
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if matches == 0 {
		return "(no matches)", nil
	}
	return sb.String(), nil
}

// globMatches 支持普通 glob 与 "**" 递归通配（filepath.Glob 不识别 **）。
func globMatches(pattern string) ([]string, error) {
	if !strings.Contains(pattern, "**") {
		return filepath.Glob(pattern)
	}
	re, err := globToRegexp(pattern)
	if err != nil {
		return nil, err
	}
	base := globBase(pattern)
	var out []string
	maxMatches := 2000
	_ = filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "vendor") && p != base {
			return fs.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		if re.MatchString(p) {
			out = append(out, p)
			if len(out) >= maxMatches {
				return fs.SkipAll
			}
		}
		return nil
	})
	return out, nil
}

// globBase 返回 glob 模式中第一个通配符之前最深的静态目录，作为遍历起点。
func globBase(pattern string) string {
	idx := strings.IndexAny(pattern, "*?[")
	if idx < 0 {
		return filepath.Dir(pattern)
	}
	prefix := pattern[:idx]
	if i := strings.LastIndexAny(prefix, "/\\"); i > 0 {
		return prefix[:i]
	}
	if filepath.IsAbs(pattern) {
		return string(filepath.Separator)
	}
	return "."
}

// globToRegexp 把支持 ** 的 glob 转成正则：** 跨目录，* 不跨目录，? 单字符。
func globToRegexp(pattern string) (*regexp.Regexp, error) {
	var sb strings.Builder
	sb.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch {
		case c == '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				sb.WriteString(".*")
				i++
			} else {
				sb.WriteString("[^/]*")
			}
		case c == '?':
			sb.WriteString("[^/]")
		default:
			sb.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	sb.WriteString("$")
	return regexp.Compile(sb.String())
}
