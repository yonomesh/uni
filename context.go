package uni

import "context"

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
	// TODO cfg             *Config
	ancestry     []Module
	cleanupFuncs []func()                // invoked at every config unload
	exitFuncs    []func(context.Context) // invoked at config unload ONLY IF the process is exiting (EXPERIMENTAL)
	// TODO metricsRegistry *prometheus.Registry
}
