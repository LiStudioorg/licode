package cordis

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
)

func newTestRuntime() *Runtime {
	return NewRuntime(WithOutput(discard{}))
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

// 测试 Effect LIFO 回滚：Provide/On/RegisterTool 自动登记，Unload 时逆序清理。
func TestEffectLIFORollback(t *testing.T) {
	r := newTestRuntime()
	var order []string

	p := Plugin{
		Name: "a",
		Apply: func(ctx Context) error {
			ctx.Effect(func() (func(), error) {
				return func() { order = append(order, "1") }, nil
			})
			ctx.Provide("svc", "v")
			ctx.Effect(func() (func(), error) {
				return func() { order = append(order, "3") }, nil
			})
			ctx.Effect(func() (func(), error) {
				return func() { order = append(order, "2") }, nil
			})
			return nil
		},
	}
	if err := r.Load(p); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Get("svc"); !ok {
		t.Fatal("svc missing after load")
	}
	if err := r.Unload("a"); err != nil {
		t.Fatal(err)
	}
	want := []string{"2", "3", "1"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("LIFO order = %v, want %v", order, want)
	}
	if _, ok := r.Get("svc"); ok {
		t.Fatal("svc still present after unload")
	}
}

// 测试响应式级联：服务消失 → 依赖者自动卸载并回滚；服务恢复 → 自动重载。
func TestReactiveCascadeReload(t *testing.T) {
	r := newTestRuntime()

	var applyCount int
	var toolActive bool
	var handlerFired int
	var mu sync.Mutex

	consumer := Plugin{
		Name:   "consumer",
		Inject: []string{"llm"},
		Apply: func(ctx Context) error {
			mu.Lock()
			applyCount++
			mu.Unlock()
			ctx.RegisterTool(Tool{Name: "c-tool", Execute: func(context.Context, map[string]any) (string, error) { return "ok", nil }})
			ctx.On("ping", func(ctx Context, in any, next func(any) (any, error)) (any, error) {
				mu.Lock()
				handlerFired++
				mu.Unlock()
				return in, nil
			})
			return nil
		},
	}
	// 依赖未就绪：应停留在 PENDING，不执行 Apply。
	if err := r.Load(consumer); err != nil {
		t.Fatal(err)
	}
	if st, _ := r.FiberState("consumer"); st != StatePending {
		t.Fatalf("state = %v, want pending", st)
	}
	if applyCount != 0 {
		t.Fatal("apply ran before deps ready")
	}

	r.Provide("llm", "openai")

	mu.Lock()
	if applyCount != 1 {
		t.Fatalf("applyCount = %d, want 1", applyCount)
	}
	mu.Unlock()
	if st, _ := r.FiberState("consumer"); st != StateActive {
		t.Fatalf("state = %v, want active", st)
	}
	r.Emit("ping")
	mu.Lock()
	if handlerFired != 1 {
		t.Fatalf("handlerFired = %d, want 1", handlerFired)
	}
	mu.Unlock()

	// 服务消失：依赖者级联卸载，工具/监听器自动回滚。
	r.Remove("llm")
	if st, _ := r.FiberState("consumer"); st != StatePending {
		t.Fatalf("after remove: state = %v, want pending", st)
	}
	registry := toolRegistry(t, r)
	if _, ok := registry.Get("c-tool"); ok {
		t.Fatal("tool should be unregistered after cascade dispose")
	}
	r.Emit("ping")
	mu.Lock()
	if handlerFired != 1 {
		t.Fatalf("handler survived cascade dispose: fired=%d", handlerFired)
	}
	mu.Unlock()
	_ = toolActive

	// 服务恢复：自动重新加载（重新 Apply）。
	r.Provide("llm", "claude")
	mu.Lock()
	if applyCount != 2 {
		t.Fatalf("applyCount after reload = %d, want 2", applyCount)
	}
	mu.Unlock()
	if _, ok := registry.Get("c-tool"); !ok {
		t.Fatal("tool should be re-registered after reload")
	}
}

// 测试 Provider 热切换：卸载旧 Provider → 依赖者卸载 → 加载新 Provider → 依赖者重载。
func TestProviderHotSwap(t *testing.T) {
	r := newTestRuntime()

	provider := func(name, val string) Plugin {
		return Plugin{Name: name, Apply: func(ctx Context) error {
			return ctx.Provide("llm", val)
		}}
	}
	if err := r.Load(provider("openai", "openai-client")); err != nil {
		t.Fatal(err)
	}

	var seen []string
	consumer := Plugin{
		Name:   "consumer",
		Inject: []string{"llm"},
		Apply: func(ctx Context) error {
			v, _ := ctx.Get("llm")
			seen = append(seen, v.(string))
			return nil
		},
	}
	if err := r.Load(consumer); err != nil {
		t.Fatal(err)
	}

	// 切换：卸载 openai → consumer 回到 PENDING；加载 claude → consumer 自动重载。
	if err := r.Unload("openai"); err != nil {
		t.Fatal(err)
	}
	if st, _ := r.FiberState("consumer"); st != StatePending {
		t.Fatalf("consumer state after unload = %v, want pending", st)
	}
	if err := r.Load(provider("claude", "claude-client")); err != nil {
		t.Fatal(err)
	}
	if st, _ := r.FiberState("consumer"); st != StateActive {
		t.Fatalf("consumer state after swap = %v, want active", st)
	}
	if !reflect.DeepEqual(seen, []string{"openai-client", "claude-client"}) {
		t.Fatalf("seen = %v", seen)
	}
	v, _ := r.Get("llm")
	if v != "claude-client" {
		t.Fatalf("llm = %v, want claude-client", v)
	}
}

// 测试 waterfall：改写、短路、出错中止。
func TestWaterfall(t *testing.T) {
	r := newTestRuntime()
	if err := r.Load(Plugin{Name: "w", Apply: func(ctx Context) error {
		ctx.On("ev", func(ctx Context, in any, next func(any) (any, error)) (any, error) {
			return next("[" + in.(string) + "]")
		})
		return nil
	}}); err != nil {
		t.Fatal(err)
	}
	if err := r.Load(Plugin{Name: "w2", Apply: func(ctx Context) error {
		ctx.On("ev", func(ctx Context, in any, next func(any) (any, error)) (any, error) {
			return next(in.(string) + "!")
		})
		return nil
	}}); err != nil {
		t.Fatal(err)
	}
	out, err := r.Waterfall("ev", "x", func(v any) (any, error) { return "core:" + v.(string), nil })
	if err != nil {
		t.Fatal(err)
	}
	if out != "core:[x]!" {
		t.Fatalf("waterfall out = %v", out)
	}

	// 短路：不调用 next。
	if err := r.Load(Plugin{Name: "short", Apply: func(ctx Context) error {
		return ctx.OnPriority("ev2", -1, func(ctx Context, in any, next func(any) (any, error)) (any, error) {
			return "short-circuited", nil
		})
	}}); err != nil {
		t.Fatal(err)
	}
	coreCalled := false
	out, _ = r.Waterfall("ev2", "in", func(v any) (any, error) { coreCalled = true; return v, nil })
	if out != "short-circuited" || coreCalled {
		t.Fatalf("short circuit failed: out=%v core=%v", out, coreCalled)
	}

	// 错误中止。
	boom := errors.New("boom")
	if err := r.Load(Plugin{Name: "bad", Apply: func(ctx Context) error {
		return ctx.OnPriority("ev3", -1, func(ctx Context, in any, next func(any) (any, error)) (any, error) {
			return nil, boom
		})
	}}); err != nil {
		t.Fatal(err)
	}
	_, err = r.Waterfall("ev3", "in", func(v any) (any, error) { return v, nil })
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
}

// 测试工具管道：pre 短路（deny）、pre 改写参数、post 改写输出。
func TestToolPipeline(t *testing.T) {
	r := newTestRuntime()

	// 注册工具 + 管道处理器。
	if err := r.Load(Plugin{Name: "tools-fs", Apply: func(ctx Context) error {
		return ctx.RegisterTool(Tool{Name: "Echo", Execute: func(context.Context, map[string]any) (string, error) {
			return "executed", nil
		}})
	}}); err != nil {
		t.Fatal(err)
	}
	if err := r.Load(Plugin{Name: "guard", Apply: func(ctx Context) error {
		// deny 工具：短路返回拒绝文本。
		ctx.OnPriority(EventToolPreExecute, -10, func(ctx Context, in any, next func(any) (any, error)) (any, error) {
			tc := in.(ToolCall)
			if tc.Name == "Echo" && tc.Args["deny"] == true {
				reject := "denied by guard"
				tc.Output = &reject
				return tc, nil
			}
			if tc.Name == "Echo" {
				nargs := map[string]any{"rewritten": true}
				tc.Args = nargs
			}
			return next(tc)
		})
		ctx.On(EventToolPostExecute, func(ctx Context, in any, next func(any) (any, error)) (any, error) {
			tr := in.(ToolResult)
			tr.Output = "post:" + tr.Output
			return next(tr)
		})
		return nil
	}}); err != nil {
		t.Fatal(err)
	}

	registry := toolRegistry(t, r)
	// 放行 + 改写 + post。
	out, err := registry.Execute(context.Background(), "Echo", map[string]any{"x": 1})
	if err != nil {
		t.Fatal(err)
	}
	if out != "post:executed" {
		t.Fatalf("out = %v", out)
	}
	// 短路拒绝。
	out, _ = registry.Execute(context.Background(), "Echo", map[string]any{"deny": true})
	if out != "denied by guard" {
		t.Fatalf("deny out = %v", out)
	}
	// 未知工具。
	// 未知工具作为 ToolResult.Err 返回，Execute 透传该 error。
	_, err = registry.ExecuteResult(context.Background(), "Nope", nil)
	_, err = registry.Execute(context.Background(), "Nope", nil)
	if !errors.Is(err, ErrToolUnknown) {
		t.Fatalf("unknown tool err = %v", err)
	}
}

// 测试 ctx.Inject：依赖出现时执行 fn，消失时回滚 fn 的副作用，恢复后重跑。
func TestContextInject(t *testing.T) {
	r := newTestRuntime()

	var fnRuns int
	inner := Plugin{Name: "outer", Apply: func(ctx Context) error {
		return ctx.Inject([]string{"llm"}, func(ctx Context) {
			fnRuns++
			ctx.RegisterTool(Tool{Name: "i-tool", Execute: func(context.Context, map[string]any) (string, error) { return "ok", nil }})
		})
	}}
	if err := r.Load(inner); err != nil {
		t.Fatal(err)
	}
	if fnRuns != 0 {
		t.Fatal("inject fn ran before dep")
	}
	r.Provide("llm", "a")
	if fnRuns != 1 {
		t.Fatalf("fnRuns = %d, want 1", fnRuns)
	}
	registry := toolRegistry(t, r)
	if _, ok := registry.Get("i-tool"); !ok {
		t.Fatal("inject tool missing")
	}
	r.Remove("llm")
	if _, ok := registry.Get("i-tool"); ok {
		t.Fatal("inject tool survived dep removal")
	}
	r.Provide("llm", "b")
	if fnRuns != 2 {
		t.Fatalf("fnRuns after restore = %d, want 2", fnRuns)
	}
}

// 测试 Apply 失败时自动回滚已注册的副作用。
func TestApplyFailureRollback(t *testing.T) {
	r := newTestRuntime()
	boom := errors.New("boom")
	err := r.Load(Plugin{Name: "bad", Apply: func(ctx Context) error {
		ctx.RegisterTool(Tool{Name: "t", Execute: func(context.Context, map[string]any) (string, error) { return "ok", nil }})
		ctx.Provide("svc", "x")
		return boom
	}})
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	registry := toolRegistry(t, r)
	if _, ok := registry.Get("t"); ok {
		t.Fatal("tool survived apply failure")
	}
	if _, ok := r.Get("svc"); ok {
		t.Fatal("service survived apply failure")
	}
}

// 测试 Apply panic 被捕获为错误。
func TestApplyPRecovered(t *testing.T) {
	r := newTestRuntime()
	err := r.Load(Plugin{Name: "panicky", Apply: func(ctx Context) error {
		var p *int
		*p = 1
		return nil
	}})
	if err == nil {
		t.Fatal("expected error from panic")
	}
}

// 测试 detached context 拒绝注册类 API。
func TestDetachedContextRejectsRegistration(t *testing.T) {
	r := newTestRuntime()
	if err := r.Load(Plugin{Name: "h", Apply: func(ctx Context) error {
		return ctx.On("ev", func(ctx Context, in any, next func(any) (any, error)) (any, error) {
			if err := ctx.Provide("leak", 1); err != ErrNoFiber {
				t.Errorf("Provide in detached ctx = %v, want ErrNoFiber", err)
			}
			if err := ctx.RegisterTool(Tool{Name: "leak", Execute: func(context.Context, map[string]any) (string, error) { return "", nil }}); err != ErrNoFiber {
				t.Errorf("RegisterTool in detached ctx = %v, want ErrNoFiber", err)
			}
			return in, nil
		})
	}}); err != nil {
		t.Fatal(err)
	}
	r.Emit("ev")
}

// 测试 Shutdown 逆序卸载。
func TestShutdown(t *testing.T) {
	r := newTestRuntime()
	var order []string
	mk := func(name string) Plugin {
		return Plugin{Name: name, Apply: func(ctx Context) error {
			ctx.Effect(func() (func(), error) {
				return func() { order = append(order, name) }, nil
			})
			return nil
		}}
	}
	r.Load(mk("a"))
	r.Load(mk("b"))
	r.Load(mk("c"))
	r.Shutdown()
	if !reflect.DeepEqual(order, []string{"c", "b", "a"}) {
		t.Fatalf("shutdown order = %v", order)
	}
	if st, ok := r.FiberState("a"); ok || st != StateDisposed {
		t.Fatalf("fiber a state = %v %v", st, ok)
	}
}

// 测试向已卸载 Fiber 注册 Effect 会被立即回滚。
func TestEffectAfterDispose(t *testing.T) {
	r := newTestRuntime()
	var held *fiberContext
	r.Load(Plugin{Name: "x", Apply: func(ctx Context) error {
		held = ctx.(*fiberContext)
		return nil
	}})
	r.Unload("x")
	rolled := false
	err := held.Effect(func() (func(), error) { return func() { rolled = true }, nil })
	if err != ErrFiberDisposed {
		t.Fatalf("err = %v, want ErrFiberDisposed", err)
	}
	if !rolled {
		t.Fatal("late effect disposer was not rolled back")
	}
}

func toolRegistry(t *testing.T, r *Runtime) *ToolRegistry {
	t.Helper()
	v, ok := r.Get(ServiceTools)
	if !ok {
		t.Fatal("tools service missing")
	}
	return v.(*ToolRegistry)
}
