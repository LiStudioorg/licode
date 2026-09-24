package cordis

import (
	"fmt"
	"runtime/debug"
	"sort"
	"sync"
)

// handlerEntry 是事件总线中的一个监听器，归属于注册它的 Fiber。
type handlerEntry struct {
	id     uint64
	fiber  *Fiber
	fn     Handler
	name   string // 归属插件名（诊断用）
	priority int
}

// EventBus 是类型化事件总线，支持普通 Emit 与 waterfall 决策链。
//
// 监听器按 priority 升序、注册顺序（FIFO）排序执行。waterfall 链内串行调用，
// 每个处理器收到一个 next 延续，可选择放行、改写输入或短路。
type EventBus struct {
	mu       sync.RWMutex
	handlers map[string][]*handlerEntry
	seq      uint64
	log      loggerProvider
}

func newEventBus(log loggerProvider) *EventBus {
	return &EventBus{handlers: map[string][]*handlerEntry{}, log: log}
}

// logf 记录总线内部错误。
func (b *EventBus) logf(format string, args ...any) {
	if b.log != nil {
		b.log.Logger().Printf(format, args...)
	}
}

// on 注册监听器，返回注销函数（Fiber.Effect 的 Disposer）。
func (b *EventBus) on(event string, priority int, fiber *Fiber, name string, fn Handler) func() {
	if fn == nil {
		return func() {}
	}
	b.mu.Lock()
	b.seq++
	e := &handlerEntry{id: b.seq, fiber: fiber, fn: fn, name: name, priority: priority}
	b.handlers[event] = append(b.handlers[event], e)
	sort.SliceStable(b.handlers[event], func(i, j int) bool {
		return b.handlers[event][i].priority < b.handlers[event][j].priority
	})
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		hs := b.handlers[event]
		for i, h := range hs {
			if h.id == e.id {
				b.handlers[event] = append(hs[:i], hs[i+1:]...)
				break
			}
		}
		b.mu.Unlock()
	}
}

// snapshot 按当前顺序返回事件处理器的副本，锁外安全遍历。
func (b *EventBus) snapshot(event string) []*handlerEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(b.handlers[event]) == 0 {
		return nil
	}
	out := make([]*handlerEntry, len(b.handlers[event]))
	copy(out, b.handlers[event])
	return out
}

// removeFiber 卸载时兜底清理某 Fiber 的全部监听器。
func (b *EventBus) removeFiber(f *Fiber) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ev, hs := range b.handlers {
		kept := hs[:0]
		for _, h := range hs {
			if h.fiber != f {
				kept = append(kept, h)
			}
		}
		b.handlers[ev] = kept
	}
}

// Emit 广播事件，按序同步调用所有处理器（fire-and-forget）。
// 处理器返回的 error 仅记录日志，不影响后续处理器。
func (b *EventBus) Emit(ctx Context, event string, args ...any) {
	hs := b.snapshot(event)
	var input any
	if len(args) == 1 {
		input = args[0]
	} else if len(args) > 1 {
		input = args
	}
	for _, h := range hs {
		b.call(ctx, h, input, passthrough)
	}
}

// Waterfall 执行事件链：input 依次流过每个处理器，最后一个处理器之后调用核心
// 动作 next。处理器可以改写输入、短路（直接返回结果）或返回错误中止。
// 没有任何处理器时直接返回 next(input)。
func (b *EventBus) Waterfall(ctx Context, event string, input any, next func(any) (any, error)) (any, error) {
	if next == nil {
		next = passthrough
	}
	hs := b.snapshot(event)
	if len(hs) == 0 {
		return next(input)
	}
	var chain func(i int, in any) (any, error)
	chain = func(i int, in any) (any, error) {
		if i >= len(hs) {
			return next(in)
		}
		return b.call(ctx, hs[i], in, func(out any) (any, error) {
			return chain(i+1, out)
		})
	}
	return chain(0, input)
}

// call 在 panic 保护下调用单个处理器。
func (b *EventBus) call(ctx Context, h *handlerEntry, input any, next func(any) (any, error)) (out any, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("cordis: handler %q (from %s) panicked: %v\n%s", h.name, h.name, r, debug.Stack())
			b.logf("%v", err)
		}
	}()
	return h.fn(ctx, input, next)
}

func passthrough(in any) (any, error) { return in, nil }
