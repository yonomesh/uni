package uni

import "github.com/prometheus/client_golang/prometheus"

// adminMetrics is a collection of metrics that can be tracked for the admin API.
var adminMetrics = struct {
	requestCount  *prometheus.CounterVec
	requestErrors *prometheus.CounterVec
}{}

// globalMetrics is a collection of metrics that can be tracked for Caddy global state
var globalMetrics = struct {
	configSuccess     prometheus.Gauge
	configSuccessTime prometheus.Gauge
}{}
