package uni

import (
	"time"

	"github.com/yonomesh/uuid"
)

// Config is the top (or beginning) of the Uni configuration structure.
//
// Many parts of this config are extensible through the use of Kaze modules.
// Fields which have a json.RawMessage type and which appear as dots (•••) in
// the online docs can be fulfilled by modules in a certain module
// namespace. The docs show which modules can be used in a given place.
//
// Whenever a module is used, its name must be given either inline as part of
// the module, or as the key to the module's value. The docs will make it clear
// which to use.
//
// Generally, all config settings are optional, as it is Kaze convention to
// have good, documented default values. If a parameter is required, the docs
// should say so.
//
// Go programs which are directly building a Config struct value should take
// care to populate the JSON-encodable fields of the struct (i.e. the fields
// with `json` struct tags) if employing the module lifecycle (e.g. Provision
// method calls).
type Config struct {
	apps map[string]App

	// failedApps is a map of apps that failed to provision with their underlying error.
	failedApps   map[string]error
	eventEmitter eventEmitter
}

// App is a thing that Caddy runs.
type App interface {
	Start() error
	Stop() error
}

// Event represents something that has happened or is happening.
// An Event value is not synchronized, so it should be copied if
// being used in goroutines.
//
// EXPERIMENTAL: Events are subject to change.
type Event struct {
	// If non-nil, the event has been aborted, meaning
	// propagation has stopped to other handlers and
	// the code should stop what it was doing. Emitters
	// may choose to use this as a signal to adjust their
	// code path appropriately.
	Aborted error

	// The data associated with the event. Usually the
	// original emitter will be the only one to set or
	// change these values, but the field is exported
	// so handlers can have full access if needed.
	// However, this map is not synchronized, so
	// handlers must not use this map directly in new
	// goroutines; instead, copy the map to use it in a
	// goroutine. Data may be nil.
	Data map[string]any

	id     uuid.UUID
	ts     time.Time
	name   string
	origin Module
}
