package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathOutsideWorkdirRequiresApproval(t *testing.T) {
	ag := NewAgent(&mockClient{}, "x")
	// 无确认通道：外部路径被拒绝
	_, err := ag.Tools.Execute(context.Background(), "Read", []byte(`{"path":"/etc/hostname"}`))
	if err == nil || !strings.Contains(err.Error(), "未被用户批准") {
		t.Fatalf("expected denial without approver, got %v", err)
	}
	// 有确认通道且用户拒绝
	ctx := withPathApprover(context.Background(), func(ctx context.Context, path, tool string) bool { return false })
	_, err = ag.Tools.Execute(ctx, "Read", []byte(`{"path":"/etc/hostname"}`))
	if err == nil || !strings.Contains(err.Error(), "未被用户批准") {
		t.Fatalf("expected denial on reject, got %v", err)
	}
	// 有确认通道且用户批准：放行（选一个测试环境必然存在的绝对路径）
	probe := "/etc/passwd"
	if _, err := os.Stat(probe); err != nil {
		probe = os.Args[0] // 测试二进制自身，任何环境都存在
	}
	ctx = withPathApprover(context.Background(), func(ctx context.Context, path, tool string) bool {
		if tool != "Read" || path != filepath.Clean(probe) {
			t.Errorf("unexpected approval request: tool=%s path=%s", tool, path)
		}
		return true
	})
	if _, err := ag.Tools.Execute(ctx, "Read", []byte(`{"path":"`+probe+`"}`)); err != nil {
		t.Fatalf("expected success after approval, got %v", err)
	}
	// ".." 穿越一律拒绝（即使批准）
	ctx = withPathApprover(context.Background(), func(ctx context.Context, path, tool string) bool { return true })
	_, err = ag.Tools.Execute(ctx, "Read", []byte(`{"path":"../x"}`))
	if err == nil || !strings.Contains(err.Error(), "..") {
		t.Fatalf("expected traversal rejection, got %v", err)
	}
	// 工作目录内路径：无需确认
	wd, _ := filepath.Abs(".")
	ctx = withPathApprover(context.Background(), func(ctx context.Context, path, tool string) bool {
		t.Errorf("workdir path should not trigger approval: %s", path)
		return false
	})
	_ = wd
	if _, err := ag.Tools.Execute(ctx, "Write", []byte(`{"path":"licode_tptest_tmp.txt","content":"hi"}`)); err != nil {
		t.Fatalf("workdir write should pass: %v", err)
	}
	if _, err := ag.Tools.Execute(context.Background(), "Delete", []byte(`{"path":"licode_tptest_tmp.txt"}`)); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
}
