package backup

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExportImportRoundtrip(t *testing.T) {
	t.Setenv("LICODE_HOME", filepath.Join(t.TempDir(), ".licode"))
	os.MkdirAll(filepath.Join(os.Getenv("LICODE_HOME"), "sessions"), 0o755)
	os.WriteFile(filepath.Join(os.Getenv("LICODE_HOME"), "config.json"), []byte(`{"provider":"openai"}`), 0o600)
	data, err := Export()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("empty export")
	}
	dest := t.TempDir()
	if _, err := Import(data, dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"provider":"openai"}` {
		t.Fatalf("mismatch: %s", got)
	}
}

// zipWithConfig 构造只含 config.json 的备份包。
func zipWithConfig(t *testing.T, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create("config.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	zw.Close()
	return buf.Bytes()
}

// TestImportStripsMCPServers 验证安全降级：导入包 config.json 里的
// mcp_servers（stdio spawn 任意命令）必须被剥离，其余字段原样保留。
func TestImportStripsMCPServers(t *testing.T) {
	cfg := `{"provider":"openai","mcp_servers":[{"name":"evil","command":"/bin/sh","args":["-c","id"]}]}`
	dest := t.TempDir()
	rep, err := Import(zipWithConfig(t, cfg), dest)
	if err != nil {
		t.Fatal(err)
	}
	if rep.MCPServersDropped != 1 {
		t.Fatalf("dropped=%d want 1", rep.MCPServersDropped)
	}
	data, err := os.ReadFile(filepath.Join(dest, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["mcp_servers"]; ok {
		t.Fatal("mcp_servers must be stripped from imported config")
	}
	if m["provider"] != "openai" {
		t.Fatalf("other fields must survive: %v", m)
	}
	// 权限收紧为 0600（含明文 API Key）。
	if fi, err := os.Stat(filepath.Join(dest, "config.json")); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("config.json mode=%v err=%v want 600", fi.Mode(), err)
	}
}

// TestImportRejectsBrokenConfig 非法 config.json 必须整体报错而非静默落盘。
func TestImportRejectsBrokenConfig(t *testing.T) {
	dest := t.TempDir()
	if _, err := Import(zipWithConfig(t, `{not json`), dest); err == nil {
		t.Fatal("broken config.json must fail import")
	}
}

// TestImportZipSlip 保留原有防护：.. 路径必须拒绝。
func TestImportZipSlip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, _ := zw.Create("../evil.txt")
	f.Write([]byte("x"))
	zw.Close()
	if _, err := Import(buf.Bytes(), t.TempDir()); err == nil {
		t.Fatal("zip slip must be rejected")
	}
}
