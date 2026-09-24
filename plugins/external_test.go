package plugins

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"licode/cordis"
)

// TestHelperMCP 在 LICODE_FAKE_MCP=1 时充当假 MCP stdio 插件进程；
// 常规测试运行中跳过。经典 go helper-subprocess 模式，测试自包含无外部依赖。
func TestHelperMCP(t *testing.T) {
	if os.Getenv("LICODE_FAKE_MCP") == "" {
		t.Skip("helper subprocess for external plugin tests")
	}
	serveFakeMCP()
	os.Exit(0)
}

func serveFakeMCP() {
	in := bufio.NewReader(os.Stdin)
	for {
		cl := 0
		for {
			line, err := in.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				break
			}
			if strings.HasPrefix(strings.ToLower(line), "content-length:") {
				cl, _ = strconv.Atoi(strings.TrimSpace(line[len("content-length:"):]))
			}
		}
		if cl <= 0 {
			continue
		}
		buf := make([]byte, cl)
		if _, err := io.ReadFull(in, buf); err != nil {
			return
		}
		var req struct {
			ID     *int            `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(buf, &req) != nil || req.ID == nil {
			continue // 通知不需要响应
		}
		var result string
		switch req.Method {
		case "initialize":
			result = `{"protocolVersion":"2024-11-05","capabilities":{"tools":{}},"serverInfo":{"name":"echoplugin","version":"1.0.0"}}`
		case "tools/list":
			result = `{"tools":[{"name":"ping","description":"返回 pong","inputSchema":{"type":"object"}},{"name":"boom","description":"执行即崩溃","inputSchema":{"type":"object"}}]}`
		case "tools/call":
			var p struct {
				Name string `json:"name"`
			}
			_ = json.Unmarshal(req.Params, &p)
			if p.Name == "boom" {
				os.Exit(3) // 模拟插件进程崩溃
			}
			result = `{"content":[{"type":"text","text":"pong"}]}`
		default:
			result = `{}`
		}
		payload := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":%s}`, *req.ID, result)
		if _, err := fmt.Printf("Content-Length: %d\r\n\r\n%s", len(payload), payload); err != nil {
			return
		}
	}
}

func writeFakePlugin(t *testing.T, dir, name string, manifest map[string]any) {
	t.Helper()
	pdir := filepath.Join(dir, name)
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := map[string]any{"name": name, "version": "1.0.0", "entry": "./run.sh"}
	for k, v := range manifest {
		m[k] = v
	}
	data, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(pdir, "plugin.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	sh := "/system/bin/sh"
	if _, err := os.Stat(sh); err != nil {
		sh = "/bin/sh"
	}
	script := "#!" + sh + "\nLICODE_FAKE_MCP=1 exec '" + os.Args[0] + "' -test.run '^TestHelperMCP$' -test.timeout 120s\n"
	if err := os.WriteFile(filepath.Join(pdir, "run.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestExternalPluginLifecycle(t *testing.T) {
	dir := t.TempDir()
	writeFakePlugin(t, dir, "echoplugin", nil)

	manifests, warns := Discover(dir)
	if len(warns) != 0 || len(manifests) != 1 || manifests[0].Name != "echoplugin" {
		t.Fatalf("discover: %+v warns=%v", manifests, warns)
	}

	r := cordis.NewRuntime(cordis.WithOutput(io.Discard))
	loaded, warns := RegisterExternalAll(r, dir)
	if len(loaded) != 1 || loaded[0] != "echoplugin" {
		t.Fatalf("loaded=%v warns=%v", loaded, warns)
	}
	reg, _ := r.Get(cordis.ServiceTools)
	tr := reg.(*cordis.ToolRegistry)
	if _, ok := tr.Get("mcp__echoplugin__ping"); !ok {
		t.Fatalf("tools = %v", tr.Names())
	}
	if st, _ := r.FiberState("ext-echoplugin"); st != cordis.StateActive {
		t.Fatalf("fiber state = %v", st)
	}

	out, err := tr.Execute(context.Background(), "mcp__echoplugin__ping", map[string]any{})
	if err != nil || out != "pong" {
		t.Fatalf("ping out=%q err=%v", out, err)
	}

	// 插件进程崩溃：工具调用快速报错，核心与其他插件不受影响。
	start := time.Now()
	if _, err := tr.Execute(context.Background(), "mcp__echoplugin__boom", map[string]any{}); err == nil {
		t.Fatal("boom should error after subprocess died")
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("crash detection too slow: %v", time.Since(start))
	}

	// 卸载：连接关闭、进程被回收（close 幂等，验证不 panic）。
	if err := r.Unload("ext-echoplugin"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tr.Get("mcp__echoplugin__ping"); ok {
		t.Fatal("ext tool survived unload")
	}
	r.Shutdown()
}

func TestExternalPluginManifestRules(t *testing.T) {
	dir := t.TempDir()
	writeFakePlugin(t, dir, "disabled", map[string]any{"auto_start": false})
	writeFakePlugin(t, dir, "broken", map[string]any{"entry": "./missing"})
	os.WriteFile(filepath.Join(dir, "notaplugin.txt"), []byte("x"), 0o644)
	if err := os.MkdirAll(filepath.Join(dir, "no-manifest"), 0o755); err != nil {
		t.Fatal(err)
	}

	manifests, warns := Discover(dir)
	if len(manifests) != 2 || len(warns) != 0 {
		t.Fatalf("manifests=%+v warns=%v", manifests, warns)
	}
	r := cordis.NewRuntime(cordis.WithOutput(io.Discard))
	loaded, warns := RegisterExternalAll(r, dir)
	if len(loaded) != 0 {
		t.Fatalf("nothing should load: %v", loaded)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "broken") {
		t.Fatalf("warns=%v, want broken entry failure", warns)
	}
	r.Shutdown()
}
