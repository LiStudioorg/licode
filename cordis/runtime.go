package cordis

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

// Runtime 是 Cordis 微内核：生命周期状态机 + 服务总线 + 事件总线 + 工具注册表。
// 所有生命周期入口（Load/Unload/Reload/Shutdown）由 actMu 串行化，保证
// Apply/回滚在单线程语义下按序执行（无并发 Apply、无并发 Dispose）。
type Runtime struct {
	actMu sync.Mutex // 生命周期串行锁

	inj     *Injector
	bus     *EventBus
	tools   *toolStore
	log     *log.Logger
	setOnce sync.Once

	detached Context

	mapMu sync.RWMutex
	fibers map[string]*Fiber
	order  []string
}

// Option 配置 Runtime。
type Option func(*Runtime)

// WithLogger 自定义日志器。
func WithLogger(l *log.Logger) Option {
	return func(r *Runtime) { r.setOnce.Do(func() { r.log = l }) }
}

// WithOutput 指定日志输出（默认 os.Stderr）。
func WithOutput(w io.Writer) Option {
	return WithLogger(log.New(w, "[cordis] ", log.LstdFlags))
}

// NewRuntime 创建微内核，并提供引导服务：
//   - "cordis": Runtime 自身
//   - "tools":  ToolRegistry（工具执行管道入口）
func NewRuntime(opts ...Option) *Runtime {
	r := &Runtime{
		fibers: map[string]*Fiber{},
		log:    log.New(os.Stderr, "[cordis] ", log.LstdFlags),
	}
	r.inj = newInjector(r)
	r.bus = newEventBus(r)
	r.tools = newToolStore()
	r.detached = &detachedContext{r: r}
	for _, o := range opts {
		o(r)
	}
	r.inj.provide(ServiceCordis, r, nil)
	r.inj.provide(ServiceTools, &ToolRegistry{r: r}, nil)
	return r
}

// Logger 返回运行时日志器。
func (r *Runtime) Logger() *log.Logger { return r.log }

func (r *Runtime) logf(format string, args ...any) {
	r.log.Printf(format, args...)
}

// Load 加载插件：创建 Fiber，依赖就绪则立即 Apply，否则进入 PENDING 等待。
// Apply 失败时回滚其副作用并返回错误。
func (r *Runtime) Load(p Plugin) error {
	if p.Name == "" || p.Apply == nil {
		return errors.New("cordis: plugin name and Apply are required")
	}
	r.actMu.Lock()
	defer r.actMu.Unlock()

	r.mapMu.Lock()
	if _, dup := r.fibers[p.Name]; dup {
		r.mapMu.Unlock()
		return fmt.Errorf("%w: %s", ErrPluginExists, p.Name)
	}
	f := newFiber(r, p, nil)
	r.fibers[p.Name] = f
	r.order = append(r.order, p.Name)
	r.mapMu.Unlock()

	r.activate(f)

	f.mu.Lock()
	st, aerr := f.state, f.applyErr
	f.mu.Unlock()
	if st != StateActive && aerr != nil {
		f.mu.Lock()
		f.applyErr = nil
		f.mu.Unlock()
		r.unloadFiber(f)
		return fmt.Errorf("cordis: plugin %s: %w", p.Name, aerr)
	}
	return nil
}

// Unload 永久卸载插件（副作用 LIFO 回滚，服务注销并级联失效依赖者）。
func (r *Runtime) Unload(name string) error {
	r.actMu.Lock()
	defer r.actMu.Unlock()
	r.mapMu.RLock()
	f := r.fibers[name]
	r.mapMu.RUnlock()
	if f == nil {
		return fmt.Errorf("%w: %s", ErrPluginNotFound, name)
	}
	r.unloadFiber(f)
	return nil
}

// Reload 重新加载插件：先回滚全部副作用回到 PENDING，再重新激活。
func (r *Runtime) Reload(name string) error {
	r.actMu.Lock()
	defer r.actMu.Unlock()
	r.mapMu.RLock()
	f := r.fibers[name]
	r.mapMu.RUnlock()
	if f == nil {
		return fmt.Errorf("%w: %s", ErrPluginNotFound, name)
	}
	if f.getState() == StateActive {
		r.disposeDependent(f, "")
	}
	f.mu.Lock()
	f.applyErr = nil
	f.mu.Unlock()
	r.activate(f)
	f.mu.Lock()
	aerr := f.applyErr
	f.mu.Unlock()
	if aerr != nil {
		return fmt.Errorf("cordis: plugin %s: %w", name, aerr)
	}
	return nil
}

// FiberState 返回插件当前生命周期状态。
func (r *Runtime) FiberState(name string) (FiberState, bool) {
	r.mapMu.RLock()
	f := r.fibers[name]
	r.mapMu.RUnlock()
	if f == nil {
		return StateDisposed, false
	}
	return f.getState(), true
}

// Status 按加载顺序返回所有插件的状态快照。
func (r *Runtime) Status() []FiberStatus {
	r.mapMu.RLock()
	names := append([]string(nil), r.order...)
	fs := make([]*Fiber, 0, len(names))
	for _, n := range names {
		fs = append(fs, r.fibers[n])
	}
	r.mapMu.RUnlock()
	out := make([]FiberStatus, 0, len(fs))
	for _, f := range fs {
		if f == nil {
			continue
		}
		// 不得在持有 f.mu 时调用 getState()：getState 自己也拿 f.mu，
		// sync.Mutex 不可重入，Status() 一旦被调用即自死锁。
		// injects 仅在 fiber 构造时写入、之后只读，无需持锁。
		out = append(out, FiberStatus{Name: f.id, State: f.getState(), Inject: f.injects})
	}
	return out
}

// FiberStatus 是插件状态快照。
type FiberStatus struct {
	Name  string
	State FiberState
	Inject []string
}

// WaitIdle 等待当前生命周期操作完成（测试与热路径同步用）。
// 通过 acquire/release 一个零容量 channel 表达“排空到当前操作结束”，
// 与 actMu 的串行语义一致，且空临界区不会被误读成遗漏逻辑。
func (r *Runtime) WaitIdle() {
	r.actMu.Lock()
	defer r.actMu.Unlock()
}

// Shutdown 按加载逆序卸载全部插件并清空服务总线。
func (r *Runtime) Shutdown() {
	r.actMu.Lock()
	defer r.actMu.Unlock()
	r.mapMu.Lock()
	names := append([]string(nil), r.order...)
	r.mapMu.Unlock()
	for i := len(names) - 1; i >= 0; i-- {
		r.mapMu.RLock()
		f := r.fibers[names[i]]
		r.mapMu.RUnlock()
		if f != nil {
			r.unloadFiber(f)
		}
	}
}

// —— 内部生命周期（仅 actMu 持有者或同 goroutine 内的级联调用）——

// activate 尝试激活 Fiber：依赖全齐则置 Active 并执行 Apply，否则挂等待队列。
func (r *Runtime) activate(f *Fiber) {
	if f.getState() != StatePending {
		return
	}
	if !r.inj.waitFor(f.injects, f) {
		return
	}
	if f.isInject && !f.parentActive() {
		return
	}
	r.inj.removeDependents(f)
	for _, d := range f.injects {
		r.inj.addDependent(d, f)
	}
	f.mu.Lock()
	f.state = StateActive
	f.applyErr = nil
	f.mu.Unlock()

	err := f.apply(&fiberContext{r: r, f: f})
	if err != nil {
		f.mu.Lock()
		f.applyErr = err
		f.state = StatePending
		f.mu.Unlock()
		f.unwind()
		r.inj.removeDependents(f)
		r.inj.unwait(f)
		r.inj.waitFor(f.injects, f)
		r.logf("plugin %s apply failed: %v", f.id, err)
	}
}

// disposeDependent 级联失效一个已激活的依赖者：回滚副作用后回到 PENDING，
// 服务恢复时由 Injector 自动唤醒。
func (r *Runtime) disposeDependent(f *Fiber, name string) {
	if f.getState() != StateActive {
		return
	}
	f.unwind()
	f.mu.Lock()
	f.state = StatePending
	f.mu.Unlock()
	r.inj.removeDependents(f)
	r.inj.unwait(f)
	r.inj.waitFor(f.injects, f)
}

// unloadFiber 永久卸载 Fiber。
func (r *Runtime) unloadFiber(f *Fiber) {
	if f.getState() == StateDisposed {
		return
	}
	f.unwind()
	f.mu.Lock()
	f.state = StateDisposed
	f.mu.Unlock()
	r.inj.removeDependents(f)
	r.inj.unwait(f)
	r.bus.removeFiber(f)
	r.mapMu.Lock()
	if r.fibers[f.id] == f {
		delete(r.fibers, f.id)
		for i, n := range r.order {
			if n == f.id {
				r.order = append(r.order[:i], r.order[i+1:]...)
				break
			}
		}
	}
	r.mapMu.Unlock()
}

// Provide 在无插件归属处注册静态服务（核心引导/测试用）。
// 唤醒等待者与级联失效在本调用内同步完成。
func (r *Runtime) Provide(name string, svc any) {
	r.actMu.Lock()
	defer r.actMu.Unlock()
	r.inj.provide(name, svc, nil)
}

// Remove 移除静态服务（owner 为空匹配任意归属），并同步级联失效依赖者。
func (r *Runtime) Remove(name string) {
	r.actMu.Lock()
	defer r.actMu.Unlock()
	r.inj.remove(name, nil)
}

// Get 读取服务。
func (r *Runtime) Get(name string) (any, bool) {
	return r.inj.get(name)
}

// Emit 在事件总线上广播事件。
func (r *Runtime) Emit(event string, args ...any) {
	r.bus.Emit(r.detached, event, args...)
}

// Waterfall 执行事件决策链。
func (r *Runtime) Waterfall(event string, input any, next func(any) (any, error)) (any, error) {
	return r.bus.Waterfall(r.detached, event, input, next)
}
