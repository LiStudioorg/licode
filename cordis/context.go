package cordis

import (
	"log"
)

// Context 是所有插件共享的服务总线视图，每个方法调用都会绑定到当前 Fiber
// 的 Effect 栈（Provide/On/RegisterTool/Effect/Inject）。
type Context interface {
	// —— 服务 ——
	// Provide 注册服务，Fiber 卸载时自动注销（并级联卸载依赖者）。
	Provide(name string, svc any) error
	// Get 读取服务。
	Get(name string) (any, bool)
	// Inject 声明依赖；就绪后立即执行 fn，否则等待服务出现。
	// fn 内注册的副作用绑定到一个子 Fiber：依赖服务消失时自动回滚，
	// 服务恢复后自动重新执行 fn。
	Inject(deps []string, fn func(ctx Context)) error

	// —— 可逆 Effect ——
	// Effect 注册一个可逆副作用；disposer 在 Fiber 卸载时逆序执行。
	Effect(fn func() (disposer func(), err error)) error

	// —— 事件 ——
	// On 注册 waterfall 监听器，Fiber 卸载时自动摘除。
	On(event string, handler Handler) error
	// OnPriority 同 On，但显式指定优先级（数值小者先执行）。
	OnPriority(event string, priority int, handler Handler) error
	// Emit 广播事件（fire-and-forget）。
	Emit(event string, args ...any)
	// Waterfall 执行事件决策链。
	Waterfall(event string, input any, next func(any) (any, error)) (any, error)

	// —— 工具 ——
	// RegisterTool 注册工具，Fiber 卸载时自动移除。
	RegisterTool(tool Tool) error

	// —— 内部 ——
	Logger() *log.Logger
}

// fiberContext 是绑定到具体 Fiber 的 Context 实现。
type fiberContext struct {
	r *Runtime
	f *Fiber
}

func (fc *fiberContext) Provide(name string, svc any) error {
	f := fc.f
	if name == "" || svc == nil {
		return ErrNoFiber
	}
	return f.effect(func() (Disposer, error) {
		fc.r.inj.provide(name, svc, f)
		return func() { fc.r.inj.remove(name, f) }, nil
	})
}

func (fc *fiberContext) Get(name string) (any, bool) {
	return fc.r.inj.get(name)
}

func (fc *fiberContext) Inject(deps []string, fn func(ctx Context)) error {
	if fn == nil {
		return nil
	}
	f := fc.f
	return f.effect(func() (Disposer, error) {
		child := newFiber(fc.r, Plugin{Name: f.id + "#inject", Inject: deps, Apply: func(c Context) error {
			fn(c)
			return nil
		}}, f)
		child.isInject = true
		fc.r.activate(child)
		return func() { fc.r.unloadFiber(child) }, nil
	})
}

func (fc *fiberContext) Effect(fn func() (func(), error)) error {
	if fn == nil {
		return nil
	}
	return fc.f.effect(func() (Disposer, error) {
		d, err := fn()
		return Disposer(d), err
	})
}

func (fc *fiberContext) On(event string, handler Handler) error {
	return fc.OnPriority(event, 0, handler)
}

func (fc *fiberContext) OnPriority(event string, priority int, handler Handler) error {
	f := fc.f
	return f.effect(func() (Disposer, error) {
		d := fc.r.bus.on(event, priority, f, f.id, handler)
		return d, nil
	})
}

func (fc *fiberContext) Emit(event string, args ...any) {
	fc.r.bus.Emit(fc.r.detached, event, args...)
}

func (fc *fiberContext) Waterfall(event string, input any, next func(any) (any, error)) (any, error) {
	return fc.r.bus.Waterfall(fc.r.detached, event, input, next)
}

func (fc *fiberContext) RegisterTool(t Tool) error {
	f := fc.f
	if t.Name == "" || t.Execute == nil {
		return ErrNoFiber
	}
	return f.effect(func() (Disposer, error) {
		fc.r.addTool(t, f)
		return func() { fc.r.removeTool(t.Name, f) }, nil
	})
}

func (fc *fiberContext) Logger() *log.Logger {
	return fc.r.Logger()
}

// detachedContext 提供给事件处理器/工具执行器：只读，注册类 API 一律拒绝，
// 防止在无 Fiber 归属的位置制造无法回滚的副作用。
type detachedContext struct {
	r *Runtime
}

func (dc *detachedContext) Provide(string, any) error { return ErrNoFiber }
func (dc *detachedContext) Get(name string) (any, bool) {
	return dc.r.inj.get(name)
}
func (dc *detachedContext) Inject([]string, func(Context)) error { return ErrNoFiber }
func (dc *detachedContext) Effect(func() (func(), error)) error  { return ErrNoFiber }
func (dc *detachedContext) On(string, Handler) error             { return ErrNoFiber }
func (dc *detachedContext) OnPriority(string, int, Handler) error { return ErrNoFiber }
func (dc *detachedContext) Emit(event string, args ...any) {
	dc.r.bus.Emit(dc, event, args...)
}
func (dc *detachedContext) Waterfall(event string, input any, next func(any) (any, error)) (any, error) {
	return dc.r.bus.Waterfall(dc, event, input, next)
}
func (dc *detachedContext) RegisterTool(Tool) error { return ErrNoFiber }
func (dc *detachedContext) Logger() *log.Logger     { return dc.r.Logger() }

// _ 编译期断言。
var (
	_ Context = (*fiberContext)(nil)
	_ Context = (*detachedContext)(nil)
)
