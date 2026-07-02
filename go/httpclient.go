package obs

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// HTTPClient returns an *http.Client whose transport injects W3C trace context
// into outbound requests, so gateway -> service -> service calls join one trace.
// Wire this into the shared cross-service call helper.
func HTTPClient() *http.Client {
	return &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}
}
