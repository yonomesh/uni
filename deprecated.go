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
