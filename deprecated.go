package uni

import (
	"net/http"
	"net/url"
)

// Deprecated: it is for caddy
// ProxyFuncProducer is implemented by modules which produce a
// function that returns a URL to use as network proxy. Modules
// in the namespace `caddy.network_proxy` must implement this
// interface.
type ProxyFuncProducer interface {
	ProxyFunc() func(*http.Request) (*url.URL, error)
}

// Log represents the log data format.
type LogEntry struct {
	Time     string   `json:"ts"`       // Timestamp of the log entry
	Level    string   `json:"level"`    // Log level (e.g., Trace, Debug, Info, Warning, Error, Fataland Panic)
	Category string   `json:"category"` // Category or type of the log (e.g., user-action)
	Tags     []string `json:"tags"`     // Tags related to the log
	Msg      Msger    `json:"msg"`      // Msg content, implemented via the interface for customization
	Extra    Extra    `json:"extra"`    // Extra content, implemented via the interface for customization
}

type Msger interface {
	MsgToString() (string, error)
}

type Extra interface {
	ExtraToString() (string, error)
}
