package cordis

import (
	"fmt"
	"runtime/debug"
	"sync"
)

// FiberState 是插件 Fiber 的生命周期状态。
type FiberState int

const (
	// StatePending 依赖未就绪（或级联失效后等待服务恢复）。
	StatePending FiberState = iota
	// StateActive Apply 已成功执行，副作用生效中。
	StateActive
	// StateDisposed 已永久卸载。
	StateDisposed
)

func (s FiberState) String() string {
	switch s {
	case StatePending:
		return "pending"
	case StateActive:
		return "active"
	case StateDisposed:
		return "disposed"
	}
	return "unknown"
}

// Fiber 是插件的生命周期状态机：每次 Apply 期间注册的所有 Effect 都被压入
// 清理栈，卸载时由框架按 LIFO 顺序自动回滚。
type Fiber struct {
	id       string
	plugin   Plugin
	injects  []string
	isInject bool // 是否为 ctx.Inject 创建的子 Fiber

	state    FiberState
	effects  []Disposer
	applyErr error

	mu      sync.Mutex
	parent  *Fiber
	runtime *Runtime
}

func newFiber(r *Runtime, p Plugin, parent *Fiber) *Fiber {
	return &Fiber{
		id:      p.Name,
		plugin:  p,
		injects: append([]string(nil), p.Inject...),
		state:   StatePending,
		parent:  parent,
		runtime: r,
	}
}

// effect 应用一个可逆副作用并把其 Disposer 压入清理栈。
// 已卸载的 Fiber 上注册的副作用会被立即回滚并返回 ErrFiberDisposed。
func (f *Fiber) effect(fn Effect) error {
	if fn == nil {
		return nil
	}
	d, err := fn()
	if err != nil {
		return err
	}
	f.mu.Lock()
	if f.state == StateDisposed {
		f.mu.Unlock()
		if d != nil {
			safeCall(d)
		}
		return ErrFiberDisposed
	}
	if d != nil {
		f.effects = append(f.effects, d)
	}
	f.mu.Unlock()
	return nil
}

// unwind 逆序执行清理栈（不改变 state，由调用方决定 Pending/Disposed）。
func (f *Fiber) unwind() {
	f.mu.Lock()
	if f.state == StateDisposed {
		f.mu.Unlock()
		return
	}
	effects := f.effects
	f.effects = nil
	f.mu.Unlock()

	for i := len(effects) - 1; i >= 0; i-- {
		safeCall(effects[i])
	}
}

func (f *Fiber) getState() FiberState {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state
}

func (f *Fiber) parentActive() bool {
	if f.parent == nil {
		return true
	}
	return f.parent.getState() == StateActive
}

// apply 调用插件入口，panic 转为错误。
func (f *Fiber) apply(ctx Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("apply panic: %v\n%s", r, debug.Stack())
		}
	}()
	return f.plugin.Apply(ctx)
}

// safeCall 执行 Disposer，panic 只记录不扩散。
func safeCall(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("cordis: disposer panic: %v\n%s\n", r, debug.Stack())
		}
	}()
	fn()
}
