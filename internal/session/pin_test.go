package session

import "testing"

// TestPinnedAndReorder 覆盖置顶与重排两条新协议路径。
//
// 回归重点：SetPinned 曾经在持有 Manager.mu 的情况下触发 onChange 回调，
// 而该回调（Manager.SaveSession）会重新获取同一把锁，导致整个消息循环死锁。
// 该 case 若超时即为回归。
func TestPinnedAndReorder(t *testing.T) {
	m := NewManager(t.TempDir(), false)
	a := m.Current().ID()
	m.New()
	b := m.Current().ID()
	m.New()
	c := m.Current().ID()

	m.SetPinned(a, true)
	list := m.List()
	if len(list) != 3 {
		t.Fatalf("会话数应为 3, got %d", len(list))
	}
	if !list[0].Pinned || list[0].ID != a {
		t.Fatalf("置顶后 a 应在首位且 pinned=true, got %+v", list)
	}

	// 普通项之间重排：c 排到 b 之前，置顶项不受影响
	m.Reorder([]string{a, c, b})
	list = m.List()
	if list[0].ID != a {
		t.Fatalf("置顶项应仍在首位, got %s", list[0].ID)
	}
	if list[1].ID != c || list[2].ID != b {
		t.Fatalf("普通项顺序应为 c,b, got %s,%s", list[1].ID, list[2].ID)
	}

	// 漏传的 id 必须保留在末尾，不能让会话「消失」
	m.Reorder([]string{b})
	list = m.List()
	if len(list) != 3 {
		t.Fatalf("漏传 id 后会话数应仍为 3, got %d", len(list))
	}
	if list[0].ID != a {
		t.Fatalf("置顶项应仍在首位, got %s", list[0].ID)
	}
}

// TestPinnedPersisted 验证置顶状态能落盘并在重新加载后保留。
func TestPinnedPersisted(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, false)
	a := m.Current().ID()
	m.SetPinned(a, true)

	reloaded := NewManager(dir, true)
	found := false
	for _, info := range reloaded.List() {
		if info.ID == a {
			found = true
			if !info.Pinned {
				t.Fatal("重新加载后置顶状态丢失")
			}
		}
	}
	if !found {
		t.Fatal("重新加载后找不到原会话")
	}
}
