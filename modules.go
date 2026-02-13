package uni

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Module is a type that is used as a Uni module. In
// addition to this interface, most modules will implement
// some interface expected by their host module in order
// to be useful. To learn which interface(s) to implement,
// see the documentation for the host module. At a bare
// minimum, this interface, when implemented, only provides
// the module's ID and constructor function.
//
// Modules will often implement additional interfaces
// including Provisioner, Validator, and CleanerUpper.
// If a module implements these interfaces, their
// methods are called during the module's lifespan.
//
// When a module is loaded by a host module, the following
// happens:
//
// 1) ModuleInfo.New() is called to get a new instance of the module.
//
// 2) The module's configuration is unmarshaled into that instance.
//
// 3) If the module is a Provisioner, the Provision() method is called.
//
// 4) If the module is a Validator, the Validate() method is called.
//
// 5) The module will probably be type-asserted from
// 'any' to some other, more useful interface expected
// by the host module.
//
// For example, HTTP handler modules are type-asserted as unihttp.MiddlewareHandler values.
//
// 6) When a module's containing Context is canceled, if it is
// a CleanerUpper, its Cleanup() method is called.
type Module interface {
	// his method indicates that the type is a Uni module.
	// The returned ModuleInfo must have both a name and a constructor function.
	// This method must not have any side-effects.
	UniModule() ModuleInfo
}

// ModuleInfo represents a registered Uni module.
type ModuleInfo struct {
	// ID is the "full name" of the module. It
	// must be unique and properly namespaced.
	ID ModuleID

	// New returns a pointer to a new, empty
	// instance of the module's type.
	//
	// This method must not have any side-effects,
	// and no other initialization should occur within it.
	//
	// Any initialization of the returned value should be done
	// in a Provision() method (see the
	// Provisioner interface).
	New func() Module
}

func (mi ModuleInfo) String() string {
	return string(mi.ID)
}

// ModuleID is a string that uniquely identifies a Uni module. A
// module ID is lightly structured. It consists of dot-separated
// labels which form a simple hierarchy from left to right. The last
// label is the module name, and the labels before that constitute
// the namespace (or scope).
//
// Thus, a module ID has the form: <namespace>.<name>
//
// An ID with no dot has the empty namespace, which is appropriate
// for app modules (these are "top-level" modules that Uni core
// loads and runs).
//
// Module IDs should be lowercase and use underscores (_) instead of
// spaces.
//
// Examples of valid IDs:
// - endpoint
// - endpoint.socks
// - logging.encoders.json
// - router.dpi
type ModuleID string

// Namespace returns the namespace (or scope) portion of a module ID,
// which is all but the last label of the ID. If the ID has only one
// label, then the namespace is empty.
func (id ModuleID) Namespace() string {
	lastDot := strings.LastIndex(string(id), ".")
	if lastDot < 0 {
		return ""
	}
	return string(id)[:lastDot]
}

// Name returns the Name (last element) of a module ID.
func (id ModuleID) Name() string {
	s := string(id)
	pos := strings.LastIndex(s, ".")
	if pos == -1 {
		return s
	}
	return s[pos+1:]
}

// ModuleMap is a map that can contain multiple modules,
// where the map key is the module's name. (The namespace
// is usually read from an associated field's struct tag.)
// Because the module's name is given as the key in a
// module map, the name does not have to be given in the
// json.RawMessage.
//
// Note json.RawMessage 是一种 “保留 JSON 原文以便稍后再解析” 的机制。 适合在结构不固定、或需要动态决定解析方式的场景。
type ModuleMap map[string]json.RawMessage

var (
	modules   = make(map[string]ModuleInfo)
	modulesMu sync.RWMutex
)

// RegisterModule registers a module by receiving a
// plain/empty value of the module. For registration to
// be properly recorded, this should be called in the
// init phase of runtime. Typically, the module package
// will do this as a side-effect of being imported.
// This function panics if the module's info is
// incomplete or invalid, or if the module is already
// registered.
func RegisterModule(instance Module) {
	mi := instance.UniModule()

	if mi.ID == "" {
		panic("module ID missing")
	}
	if mi.ID == "uni" || mi.ID == "admin" {
		panic(fmt.Sprintf("module ID '%s' is reserved", mi.ID))
	}
	if mi.New == nil {
		panic("missing ModuleInfo.New")
	}
	if val := mi.New(); val == nil {
		panic("ModuleInfo.New must return a non-nil module instance")
	}
	modulesMu.Lock()
	defer modulesMu.Unlock()
	if _, ok := modules[string(mi.ID)]; ok {
		panic(fmt.Sprintf("module already registered: %s", mi.ID))
	}
	modules[string(mi.ID)] = mi
}

// GetModule returns module information from its ID (full name).
func GetModule(name string) (ModuleInfo, error) {
	modulesMu.RLock()
	defer modulesMu.RUnlock()
	m, ok := modules[name]
	if !ok {
		return ModuleInfo{}, fmt.Errorf("module not registered: %s", name)
	}
	return m, nil
}

// GetModules returns all modules in the given scope/namespace.
// For example, a scope of "foo" returns modules named "foo.bar",
// "foo.loo", but not "bar", "foo.bar.loo", etc. An empty scope
// returns top-level modules, for example "foo" or "bar". Partial
// scopes are not matched (i.e. scope "foo.ba" does not match
// name "foo.bar").
//
// Because modules are registered to a map under the hood, the
// returned slice will be sorted to keep it deterministic.
func GetModules(scope string) []ModuleInfo {
	modulesMu.RLock()
	defer modulesMu.RUnlock()

	scopeParts := strings.Split(scope, ".")

	// handle the special case of an empty scope, which
	// should match only the top-level modules
	if scope == "" {
		scopeParts = []string{}
	}

	var mods []ModuleInfo
iterateModules:
	for id, m := range modules {
		modParts := strings.Split(id, ".")

		// match only the next level of nesting
		if len(modParts) != len(scopeParts)+1 {
			continue
		}

		// specified parts must be exact matches
		for i := range scopeParts {
			if modParts[i] != scopeParts[i] {
				continue iterateModules
			}
		}

		mods = append(mods, m)
	}

	// make return value deterministic
	sort.Slice(mods, func(i, j int) bool {
		return mods[i].ID < mods[j].ID
	})

	return mods
}

// GetModuleName returns a module's name (the last label of its ID)
// from an instance of its value. If the value is not a module, an
// empty string will be returned.
func GetModuleName(instance any) string {
	var name string
	if mod, ok := instance.(Module); ok {
		name = mod.UniModule().ID.Name()
	}
	return name
}
