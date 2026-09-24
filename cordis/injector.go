package cordis

import "sync"

// serviceEntry 记录一个已注册服务及其归属 Fiber。
type serviceEntry struct {
	svc   any
	owner *Fiber
}

// Injector 是响应式服务总线：
//
//   - Provide 注册服务并唤醒等待者；
//   - Remove 注销服务并**级联卸载**所有 inject 了它的 Fiber（级联失效），
//     这些 Fiber 回到 PENDING 重新等待；服务恢复后自动重新加载；
//   - WaitFor 让依赖未齐的 Fiber 进入等待队列。
//
// 所有对内部状态的修改都在 inj.mu 下完成；Fiber 的 Apply/Dispose 通过
// runtime 的激活队列串行执行，不在持有 inj.mu 时调用插件代码。
type Injector struct {
	inj        sync.RWMutex
	r          *Runtime
	providers  map[string]serviceEntry
	waiters    map[string]map[string]*Fiber // 服务名 -> fiberID -> Fiber
	dependents map[string]map[string]*Fiber // 服务名 -> 已激活的依赖者集合
}

func newInjector(r *Runtime) *Injector {
	return &Injector{
		r:          r,
		providers:  map[string]serviceEntry{},
		waiters:    map[string]map[string]*Fiber{},
		dependents: map[string]map[string]*Fiber{},
	}
}

// provide 注册服务。同名服务被替换：先以旧归属身份移除旧服务（触发旧依赖者
// 级联卸载），再注册新服务并唤醒等待者。
func (inj *Injector) provide(name string, svc any, owner *Fiber) {
	inj.inj.Lock()
	if old, ok := inj.providers[name]; ok && (owner == nil || old.owner != owner) {
		// 替换：递归移除旧归属（会级联其依赖者），随后新值生效。
		delete(inj.providers, name)
		deps := inj.dependents[name]
		delete(inj.dependents, name)
		inj.inj.Unlock()
		for _, f := range deps {
			inj.r.disposeDependent(f, name)
		}
		inj.inj.Lock()
	}
	inj.providers[name] = serviceEntry{svc: svc, owner: owner}
	waits := inj.waiters[name]
	delete(inj.waiters, name)
	inj.inj.Unlock()

	for _, f := range waits {
		inj.r.activate(f)
	}
}

// remove 注销服务；owner 非空时仅当归属匹配才移除（防止 Fiber 重建后
// 旧 Disposer 误删新注册）。返回是否有依赖者被级联。
func (inj *Injector) remove(name string, owner *Fiber) bool {
	inj.inj.Lock()
	e, ok := inj.providers[name]
	if !ok || (owner != nil && e.owner != owner) {
		inj.inj.Unlock()
		return false
	}
	delete(inj.providers, name)
	deps := inj.dependents[name]
	delete(inj.dependents, name)
	inj.inj.Unlock()

	for _, f := range deps {
		inj.r.disposeDependent(f, name)
	}
	return true
}

// get 读取服务。
func (inj *Injector) get(name string) (any, bool) {
	inj.inj.RLock()
	defer inj.inj.RUnlock()
	e, ok := inj.providers[name]
	return e.svc, ok
}

// has 判断服务是否就绪。
func (inj *Injector) has(name string) bool {
	inj.inj.RLock()
	defer inj.inj.RUnlock()
	_, ok := inj.providers[name]
	return ok
}

// waitFor 检查 deps 是否全部就绪；未就绪则把 fiber 挂到缺失服务的等待队列。
func (inj *Injector) waitFor(deps []string, fiber *Fiber) bool {
	inj.inj.Lock()
	defer inj.inj.Unlock()
	ready := true
	for _, dep := range deps {
		if _, ok := inj.providers[dep]; !ok {
			if inj.waiters[dep] == nil {
				inj.waiters[dep] = map[string]*Fiber{}
			}
			inj.waiters[dep][fiber.id] = fiber
			ready = false
		}
	}
	return ready
}

// unwait 从所有等待队列中摘除 fiber（服务恢复重新激活时调用）。
func (inj *Injector) unwait(fiber *Fiber) {
	inj.inj.Lock()
	defer inj.inj.Unlock()
	for _, m := range inj.waiters {
		delete(m, fiber.id)
	}
}

// addDependent 记录 fiber 依赖服务 name（fiber 激活后调用）。
func (inj *Injector) addDependent(name string, fiber *Fiber) {
	inj.inj.Lock()
	defer inj.inj.Unlock()
	if inj.dependents[name] == nil {
		inj.dependents[name] = map[string]*Fiber{}
	}
	inj.dependents[name][fiber.id] = fiber
}

// removeDependents 清除 fiber 的全部依赖记录（fiber 卸载时调用）。
func (inj *Injector) removeDependents(fiber *Fiber) {
	inj.inj.Lock()
	defer inj.inj.Unlock()
	for _, m := range inj.dependents {
		delete(m, fiber.id)
	}
}
