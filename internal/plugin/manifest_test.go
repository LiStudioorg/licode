package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func writeManifest(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadManifestValid(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `{
	  "id": "hello",
	  "name": "Hello",
	  "version": "0.1.0",
	  "apiVersion": 1,
	  "entry": "python3",
	  "args": ["main.py"],
	  "capabilities": ["tools", "hooks"],
	  "permissions": {"fs": ["~/docs"], "shell": true},
	  "contributes": {
	    "tools": [{"name": "hello", "description": "打招呼"}],
	    "commands": [{"name": "hello"}]
	  }
	}`)
	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.ID != "hello" || m.ToolName("hello") != "plugin__hello__hello" {
		t.Fatalf("unexpected manifest: %+v", m)
	}
	if !m.HasCapability("hooks") || m.HasCapability("panels") {
		t.Fatalf("capability 判断错误")
	}
	if m.Contributes.Tools[0].Schema == nil {
		t.Fatalf("缺省 schema 应补为 object")
	}
	summary := m.Permissions.Summary()
	if len(summary) != 2 {
		t.Fatalf("权限摘要 = %v", summary)
	}
}

func TestLoadManifestInvalid(t *testing.T) {
	cases := []struct{ name, content string }{
		{"非法 id", `{"id":"Bad ID","name":"x","version":"1","apiVersion":1,"entry":"sh"}`},
		{"API 版本不符", `{"id":"ok","name":"x","version":"1","apiVersion":99,"entry":"sh"}`},
		{"缺少 entry", `{"id":"ok","name":"x","version":"1","apiVersion":1}`},
		{"工具重名", `{"id":"ok","name":"x","version":"1","apiVersion":1,"entry":"sh","contributes":{"tools":[{"name":"a"},{"name":"a"}]}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			writeManifest(t, dir, c.content)
			if _, err := LoadManifest(dir); err == nil {
				t.Fatalf("应当报错")
			}
		})
	}
	if _, err := LoadManifest(t.TempDir()); err == nil {
		t.Fatalf("缺少 plugin.json 应当报错")
	}
}

func TestResultContent(t *testing.T) {
	if got, _ := resultContent([]byte(`"纯文本"`)); got != "纯文本" {
		t.Fatalf("字符串结果 = %q", got)
	}
	if got, _ := resultContent([]byte(`{"content":"对象"}`)); got != "对象" {
		t.Fatalf("对象结果 = %q", got)
	}
	if _, err := resultContent([]byte(`{"error":"失败"}`)); err == nil {
		t.Fatalf("错误对象应当报错")
	}
	if got, _ := resultContent(nil); got != "" {
		t.Fatalf("空结果 = %q", got)
	}
}
