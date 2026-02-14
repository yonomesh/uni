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
