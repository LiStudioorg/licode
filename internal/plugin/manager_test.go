package plugin

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func buildZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const validManifest = `{"id":"zipdemo","name":"ZipDemo","version":"1.0.0","apiVersion":1,"entry":"python3"}`

func TestInstallFromZip(t *testing.T) {
	root := t.TempDir()
	m := NewManager(filepath.Join(root, "state.json"), filepath.Join(root, "plugins"))
	data := buildZip(t, map[string]string{
		"plugin.json": validManifest,
		"main.py":     "print('hi')",
	})
	id, err := m.Install(data)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if id != "zipdemo" {
		t.Fatalf("id = %q", id)
	}
	if _, err := os.Stat(filepath.Join(root, "plugins", "zipdemo", "main.py")); err != nil {
		t.Fatalf("插件文件未落盘: %v", err)
	}
	if len(m.List()) != 1 || m.List()[0].Enabled {
		t.Fatalf("新插件应默认未启用: %+v", m.List())
	}
}

func TestInstallZipWithSubdir(t *testing.T) {
	root := t.TempDir()
	m := NewManager(filepath.Join(root, "state.json"), filepath.Join(root, "plugins"))
	data := buildZip(t, map[string]string{
		"zipdemo/plugin.json": validManifest,
		"zipdemo/main.py":     "print('hi')",
	})
	if _, err := m.Install(data); err != nil {
		t.Fatalf("单层子目录应可安装: %v", err)
	}
}

func TestInstallRejectsBadZip(t *testing.T) {
	root := t.TempDir()
	m := NewManager(filepath.Join(root, "state.json"), filepath.Join(root, "plugins"))
	if _, err := m.Install(buildZip(t, map[string]string{"readme.txt": "no manifest"})); err == nil {
		t.Fatalf("缺少 plugin.json 应当报错")
	}
	if _, err := m.Install([]byte("not a zip")); err == nil {
		t.Fatalf("非法 zip 应当报错")
	}
}

func TestEnableRequiresAck(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "plugins", "demo")
	writeManifest(t, dir, `{"id":"demo","name":"Demo","version":"1","apiVersion":1,"entry":"/bin/true"}`)
	m := NewManager(filepath.Join(root, "state.json"), filepath.Join(root, "plugins"))
	m.scanAll(context.Background(), false)

	if err := m.Enable(context.Background(), "demo", false); !errors.Is(err, ErrNeedAck) {
		t.Fatalf("未确认权限应返回 ErrNeedAck，得到 %v", err)
	}
	if err := m.Disable("nope"); err == nil {
		t.Fatalf("停用不存在的插件应当报错")
	}
	if err := m.SetSettings("demo", []byte("{bad json")); err == nil {
		t.Fatalf("非法设置应当报错")
	}
	if err := m.SetSettings("demo", []byte(`{"k":"v"}`)); err != nil {
		t.Fatalf("合法设置失败: %v", err)
	}
	// 状态持久化后再开一个管理器应能读到设置
	m2 := NewManager(filepath.Join(root, "state.json"), filepath.Join(root, "plugins"))
	m2.scanAll(context.Background(), false)
	var got map[string]string
	if err := json.Unmarshal(m2.List()[0].Settings, &got); err != nil || got["k"] != "v" {
		t.Fatalf("设置未持久化: %s (%v)", m2.List()[0].Settings, err)
	}
}
