package uni

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"

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

// ErrNotConfigured indicates a module is not configured.
var ErrNotConfigured = fmt.Errorf("module not configured")

// LoadModule loads the Caddy module(s) from the specified field of the parent struct
// pointer and returns the loaded module(s). The struct pointer and its field name as
// a string are necessary so that reflection can be used to read the struct tag on the
// field to get the module namespace and inline module name key (if specified).
//
// The field can be any one of the supported raw module types: json.RawMessage,
// []json.RawMessage, map[string]json.RawMessage, or []map[string]json.RawMessage.
// ModuleMap may be used in place of map[string]json.RawMessage. The return value's
// underlying type mirrors the input field's type:
//
//	json.RawMessage              => any
//	[]json.RawMessage            => []any
//	[][]json.RawMessage          => [][]any
//	map[string]json.RawMessage   => map[string]any
//	[]map[string]json.RawMessage => []map[string]any
//
// The field must have a "caddy" struct tag in this format:
//
//	caddy:"key1=val1 key2=val2"
//
// To load modules, a "namespace" key is required. For example, to load modules
// in the "http.handlers" namespace, you'd put: `namespace=http.handlers` in the
// Caddy struct tag.
//
// The module name must also be available. If the field type is a map or slice of maps,
// then key is assumed to be the module name if an "inline_key" is NOT specified in the
// caddy struct tag. In this case, the module name does NOT need to be specified in-line
// with the module itself.
//
// If not a map, or if inline_key is non-empty, then the module name must be embedded
// into the values, which must be objects; then there must be a key in those objects
// where its associated value is the module name. This is called the "inline key",
// meaning the key containing the module's name that is defined inline with the module
// itself. You must specify the inline key in a struct tag, along with the namespace:
//
//	caddy:"namespace=http.handlers inline_key=handler"
//
// This will look for a key/value pair like `"handler": "..."` in the json.RawMessage
// in order to know the module name.
//
// To make use of the loaded module(s) (the return value), you will probably want
// to type-assert each 'any' value(s) to the types that are useful to you
// and store them on the same struct. Storing them on the same struct makes for
// easy garbage collection when your host module is no longer needed.
//
// Loaded modules have already been provisioned and validated. Upon returning
// successfully, this method clears the json.RawMessage(s) in the field since
// the raw JSON is no longer needed, and this allows the GC to free up memory.
func (ctx Context) LoadModule(structPointer any, fieldName string) (any, error) {
	val := reflect.ValueOf(structPointer).Elem().FieldByName(fieldName)
	typ := val.Type()

	field, ok := reflect.TypeOf(structPointer).Elem().FieldByName(fieldName)
	if !ok {
		panic(fmt.Sprintf("field %s does not exist in %#v", fieldName, structPointer))
	}

	opts, err := ParseStructTag(field.Tag.Get("caddy"))
	if err != nil {
		panic(fmt.Sprintf("malformed tag on field %s: %v", fieldName, err))
	}

	moduleNamespace, ok := opts["namespace"]
	if !ok {
		panic(fmt.Sprintf("missing 'namespace' key in struct tag on field %s", fieldName))
	}
	inlineModuleKey := opts["inline_key"]

	var result any

	switch val.Kind() {
	case reflect.Slice:
		if isJSONRawMessage(typ) { // val is `json.RawMessage` ([]uint8 under the hood)
			if inlineModuleKey == "" {
				panic("unable to determine module name without inline_key when type is not a ModuleMap")
			}
			val, err := ctx.loadModuleInline(inlineModuleKey, moduleNamespace, val.Interface().(json.RawMessage))
			if err != nil {
				return nil, err
			}
			result = val
		} else if isJSONRawMessage(typ.Elem()) { // val is `[]json.RawMessage`
			if inlineModuleKey == "" {
				panic("unable to determine module name without inline_key because type is not a ModuleMap")
			}
			var all []any
			for i := 0; i < val.Len(); i++ {
				val, err := ctx.loadModuleInline(inlineModuleKey, moduleNamespace, val.Index(i).Interface().(json.RawMessage))
				if err != nil {
					return nil, fmt.Errorf("position %d: %v", i, err)
				}
				all = append(all, val)
			}
			result = all
		} else if typ.Elem().Kind() == reflect.Slice && isJSONRawMessage(typ.Elem().Elem()) {
			// val is `[][]json.RawMessage`

			if inlineModuleKey == "" {
				panic("unable to determine module name without inline_key because type is not a ModuleMap")
			}
			var all [][]any
			for i := 0; i < val.Len(); i++ {
				innerVal := val.Index(i)
				var allInner []any
				for j := 0; j < innerVal.Len(); j++ {
					innerInnerVal, err := ctx.loadModuleInline(inlineModuleKey, moduleNamespace, innerVal.Index(j).Interface().(json.RawMessage))
					if err != nil {
						return nil, fmt.Errorf("position %d: %v", j, err)
					}
					allInner = append(allInner, innerInnerVal)
				}
				all = append(all, allInner)
			}
			result = all
		} else if isModuleMapType(typ.Elem()) {
			// val is `[]map[string]json.RawMessage`

			var all []map[string]any
			for i := 0; i < val.Len(); i++ {
				thisSet, err := ctx.loadModulesFromSomeMap(moduleNamespace, inlineModuleKey, val.Index(i))
				if err != nil {
					return nil, err
				}
				all = append(all, thisSet)
			}
			result = all
		}

	case reflect.Map:
		// val is a ModuleMap or some other kind of map
		result, err = ctx.loadModulesFromSomeMap(moduleNamespace, inlineModuleKey, val)
		if err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unrecognized type for module: %s", typ)
	}

	// we're done with the raw bytes; allow GC to deallocate
	val.Set(reflect.Zero(typ))

	return result, nil
}

// LoadModuleByID decodes rawMsg into a new instance of mod and
// returns the value. If mod.New is nil, an error is returned.
// If the module implements Validator or Provisioner interfaces,
// those methods are invoked to ensure the module is fully
// configured and valid before being used.
//
// This is a lower-level method and will usually not be called
// directly by most modules. However, this method is useful when
// dynamically loading/unloading modules in their own context,
// like from embedded scripts, etc.
func (ctx Context) LoadModuleByID(id string, rawMsg json.RawMessage) (any, error) {
	modulesMu.RLock()
	modInfo, ok := modules[id]
	modulesMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown module: %s", id)
	}

	if modInfo.New == nil {
		return nil, fmt.Errorf("module '%s' has no constructor", modInfo.ID)
	}

	val := modInfo.New()

	// Ensure that val is a pointer, because json.Unmarshal requires a pointer
	// to decode into concrete types (structs, slices, maps, etc.).
	//
	// Ideally, ModuleInfo.New() SHOULD return a pointer, e.g. `&MyModule{}` or `new(MyModule)`.
	// If it doesn't, we have a problem: json.Unmarshal would fail or panic when
	// trying to fill the module's fields.
	//
	// To handle this safely, we check the kind of val using reflection.
	if rv := reflect.ValueOf(val); rv.Kind() != reflect.Pointer {
		log.Printf("[WARNING] ModuleInfo.New() for module '%s' did not return a pointer,"+
			" so we are creating a new pointer to an empty instance instead; please fix this by"+
			" using new(Type) or &Type notation in your module's New() function.", id)

		// Steps explained:
		// 1) reflect.New(rv.Type())      -> create a new reflect.Value of type T (non-pointer)
		//                                 This returns *T (a pointer to the new instance).
		// 2) .Elem()                     -> dereference the pointer to get the actual value (T).
		// 3) .Addr()                     -> take the address again to get a pointer (*T).
		// 4) .Interface().(Module)       -> convert the reflect.Value back to Module interface.
		//
		// The result is a *fresh, EMPTY, pointer instance* of the module type,
		// guaranteed to be safe for json.Unmarshal.
		val = reflect.New(rv.Type()).Elem().Addr().Interface().(Module)
	}

	// fill in its config only if there is a config to fill in
	if len(rawMsg) > 0 {
		err := StrictUnmarshalJSON(rawMsg, &val)
		if err != nil {
			return nil, fmt.Errorf("decoding module config: %s: %v", modInfo, err)
		}
	}

	if val == nil {
		// returned module values are almost always type-asserted
		// before being used, so a nil value would panic; and there
		// is no good reason to explicitly declare null modules in
		// a config; it might be because the user is trying to achieve
		// a result the developer isn't expecting, which is a smell
		return nil, fmt.Errorf("module value cannot be null")
	}

	var err error

	// if this is an app module, keep a reference to it,
	// since submodules may need to reference it during
	// provisioning (even though the parent app module
	// may not be fully provisioned yet; this is the case
	// with the tls app's automation policies, which may
	// refer to the tls app to check if a global DNS
	// module has been configured for DNS challenges)
	if appModule, ok := val.(App); ok {
		ctx.cfg.apps[id] = appModule
		defer func() {
			if err != nil {
				ctx.cfg.failedApps[id] = err
			}
		}()
	}

	ctx.ancestry = append(ctx.ancestry, val)

	if prov, ok := val.(Provisioner); ok {
		err = prov.Provision(ctx)
		if err != nil {
			// incomplete provisioning could have left state
			// dangling, so make sure it gets cleaned up
			if cleanerUpper, ok := val.(CleanerUpper); ok {
				err2 := cleanerUpper.Cleanup()
				if err2 != nil {
					err = fmt.Errorf("%v; additionally, cleanup: %v", err, err2)
				}
			}
			return nil, fmt.Errorf("provision %s: %v", modInfo, err)
		}
	}

	if validator, ok := val.(Validator); ok {
		err = validator.Validate()
		if err != nil {
			// since the module was already provisioned, make sure we clean up
			if cleanerUpper, ok := val.(CleanerUpper); ok {
				err2 := cleanerUpper.Cleanup()
				if err2 != nil {
					err = fmt.Errorf("%v; additionally, cleanup: %v", err, err2)
				}
			}
			return nil, fmt.Errorf("%s: invalid configuration: %v", modInfo, err)
		}
	}

	ctx.moduleInstances[id] = append(ctx.moduleInstances[id], val)

	// if the loaded module happens to be an app that can emit events, store it so the
	// core can have access to emit events without an import cycle
	if ee, ok := val.(eventEmitter); ok {
		if _, ok := ee.(App); ok {
			ctx.cfg.eventEmitter = ee
		}
	}

	return val, nil
}

// loadModuleInline loads a module from a JSON raw message which decodes to
// a map[string]any, where one of the object keys is moduleNameKey
// and the corresponding value is the module name (as a string) which can
// be found in the given namespace. In other words, the module name is declared
// in-line with the module itself.
//
// This allows modules to be decoded into their concrete types and used when
// their names cannot be the unique key in a map, such as when there are
// multiple instances in the map or it appears in an array (where there are
// no custom keys). In other words, the key containing the module name is
// treated special/separate from all the other keys in the object.
func (ctx Context) loadModuleInline(moduleInlineKey, moduleNamespace string, raw json.RawMessage) (any, error) {
	moduleName, raw, err := getModuleNameInline(moduleInlineKey, raw)
	if err != nil {
		return nil, err
	}

	val, err := ctx.LoadModuleByID(moduleNamespace+"."+moduleName, raw)
	if err != nil {
		return nil, fmt.Errorf("loading module '%s': %v", moduleName, err)
	}

	return val, nil
}

// loadModulesFromSomeMap loads modules from val, which must be a type of map[string]any.
// Depending on inlineModuleKey, it will be interpreted as either a ModuleMap (key is the module
// name) or as a regular map (key is not the module name, and module name is defined inline).
func (ctx Context) loadModulesFromSomeMap(namespace, moduleInlineKey string, val reflect.Value) (map[string]any, error) {
	// if no inline_key is specified, then val must be a ModuleMap,
	// where the key is the module name
	if moduleInlineKey == "" {
		if !isModuleMapType(val.Type()) {
			panic(fmt.Sprintf("expected ModuleMap because inline_key is empty; but we do not recognize this type: %s", val.Type()))
		}
		return ctx.loadModuleMap(namespace, val)
	}

	// otherwise, val is a map with modules, but the module name is
	// inline with each value (the key means something else)
	return ctx.loadModulesFromRegularMap(namespace, moduleInlineKey, val)
}

// loadModulesFromRegularMap loads modules from val, where val is a map[string]json.RawMessage.
// Map keys are NOT interpreted as module names, so module names are still expected to appear
// inline with the objects.
func (ctx Context) loadModulesFromRegularMap(namespace, inlineModuleKey string, val reflect.Value) (map[string]any, error) {
	mods := make(map[string]any)
	iter := val.MapRange()
	for iter.Next() {
		k := iter.Key()
		v := iter.Value()
		mod, err := ctx.loadModuleInline(inlineModuleKey, namespace, v.Interface().(json.RawMessage))
		if err != nil {
			return nil, fmt.Errorf("key %s: %v", k, err)
		}
		mods[k.String()] = mod
	}
	return mods, nil
}

// loadModuleMap loads modules from a ModuleMap, i.e. map[string]any, where the key is the
// module name. With a module map, module names do not need to be defined inline with their values.
func (ctx Context) loadModuleMap(namespace string, val reflect.Value) (map[string]any, error) {
	all := make(map[string]any)
	iter := val.MapRange()
	for iter.Next() {
		k := iter.Key().Interface().(string)
		v := iter.Value().Interface().(json.RawMessage)
		moduleName := namespace + "." + k
		if namespace == "" {
			moduleName = k
		}
		val, err := ctx.LoadModuleByID(moduleName, v)
		if err != nil {
			return nil, fmt.Errorf("module name '%s': %v", k, err)
		}
		all[k] = val
	}
	return all, nil
}

// eventEmitter is a small interface that inverts dependencies for
// the caddyevents package, so the core can emit events without an
// import cycle (i.e. the caddy package doesn't have to import
// the caddyevents package, which imports the caddy package).
type eventEmitter interface {
	Emit(ctx Context, eventName string, data map[string]any) Event
}
