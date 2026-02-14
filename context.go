package uni

import (
	"context"
	"log"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Context is a type which defines the lifetime of modules that
// are loaded and provides access to the parent configuration
// that spawned the modules which are loaded. It should be used
// with care and wrapped with derivation functions from the
// standard context package only if you don't need the Caddy
// specific features. These contexts are canceled when the
// lifetime of the modules loaded from it is over.
//
// Use NewContext() to get a valid value (but most modules will
// not actually need to do this).
type Context struct {
	context.Context

	moduleInstances map[string][]Module
	cfg             *Config
	ancestry        []Module
	cleanupFuncs    []func()                // invoked at every config unload
	exitFuncs       []func(context.Context) // invoked at config unload ONLY IF the process is exiting (EXPERIMENTAL)
	metricsRegistry *prometheus.Registry
}

// NewContext provides a new context derived from the given
// Context ctx. Normally, you will not need to call this
// function unless you are loading modules which have a
// different lifespan than the ones for the context the
// module was provisioned with. Be sure to call the cancel
// func when the context is to be cleaned up so that
// modules which are loaded will be properly unloaded.
// See standard library context package's documentation.
func NewContext(ctx Context) (Context, context.CancelFunc) {
	newCtx := Context{
		moduleInstances: make(map[string][]Module),
		cfg:             ctx.cfg,
		metricsRegistry: prometheus.NewPedanticRegistry(),
	}

	c, cancel := context.WithCancel(ctx.Context)

	wrappedCancel := func() {
		cancel()

		for _, f := range ctx.cleanupFuncs {
			f()
		}

		for modName, modInstances := range newCtx.moduleInstances {
			for _, inst := range modInstances {
				if cu, ok := inst.(CleanerUpper); ok {
					err := cu.Cleanup()
					if err != nil {
						log.Printf("[ERROR] %s (%p): cleanup: %v", modName, inst, err)
					}
				}
			}
		}
	}

	newCtx.Context = c
	newCtx.initMetrics()
	return newCtx, wrappedCancel
}

func (ctx *Context) initMetrics() {
	ctx.metricsRegistry.MustRegister(
		collectors.NewBuildInfoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewGoCollector(),
		adminMetrics.requestCount,
		adminMetrics.requestErrors,
		globalMetrics.configSuccess,
		globalMetrics.configSuccessTime,
	)
}

// WithValue returns a new context with the given key-value pair.
func (ctx *Context) WithValue(key, value any) Context {
	return Context{
		Context:         context.WithValue(ctx.Context, key, value),
		moduleInstances: ctx.moduleInstances,
		cfg:             ctx.cfg,
		ancestry:        ctx.ancestry,
		cleanupFuncs:    ctx.cleanupFuncs,
		exitFuncs:       ctx.exitFuncs,
	}
}

// OnCancel executes f when ctx is canceled.
//
// # TODO
//
// 目前的问题是 Semantic Drift
//
// 使用 context.AfterFunc 解决 Caddy 这种“手动维护清理列表”或“开大量协程监听取消信号”的痛点
func (ctx *Context) OnCancel(f func()) {
	ctx.cleanupFuncs = append(ctx.cleanupFuncs, f)
}

// OnExit executes f when the process exits gracefully.
// The function is only executed if the process is gracefully
// shut down while this context is active.
//
// EXPERIMENTAL API: subject to change or removal.
//
// # TODO
//
// 生命周期的设计应该更加现代化
func (ctx *Context) OnExit(f func(context.Context)) {
	ctx.exitFuncs = append(ctx.exitFuncs, f)
}

// Returns the active metrics registry for the context
// EXPERIMENTAL: This API is subject to change.
func (ctx *Context) GetMetricsRegistry() *prometheus.Registry {
	return ctx.metricsRegistry
}

// Module returns the current module, or the most recent one
// provisioned by the context.
func (ctx Context) Module() Module {
	if len(ctx.ancestry) == 0 {
		return nil
	}
	return ctx.ancestry[len(ctx.ancestry)-1]
}

// Modules returns the lineage of modules that this context provisioned,
// with the most recent/current module being last in the list.
func (ctx Context) Modules() []Module {
	mods := make([]Module, len(ctx.ancestry))
	copy(mods, ctx.ancestry)
	return mods
}
