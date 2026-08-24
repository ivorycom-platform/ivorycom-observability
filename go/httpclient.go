package obs

import (
	"net"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// DefaultTimeout bounds a cross-service call made with HTTPClient.
//
// H-3/INV-011: the previous helper returned &http.Client{Transport: ...} with no
// Timeout, so every service wiring in "the shared cross-service call helper"
// would have inherited an unbounded client from the one place meant to
// standardise this. (A fleet-wide audit found zero call sites at the time, so
// nothing had inherited it yet — this closes the trap before it is used.)
const DefaultTimeout = 10 * time.Second

// Transport returns the shared traced transport with per-phase bounds.
//
// The phase bounds matter independently of any total timeout: they are what
// protects a STREAMING caller, which cannot set a total timeout without cutting
// the stream. ResponseHeaderTimeout in particular is what catches a peer that
// completes the handshake and then never begins answering — the accepted-but-
// silent failure behind INV-011.
//
// headerBudget is a PARAMETER, not a constant, because a single shared value
// silently caps every client built on it: a hard-coded 30s header limit would
// mean HTTPClientWithTimeout(120s) could never actually reach 120s against an
// upstream that thinks for 45s before emitting headers.
func TransportWithHeaderBudget(headerBudget time.Duration) http.RoundTripper {
	return otelhttp.NewTransport(&http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: headerBudget,
	})
}

// Transport returns the traced transport at the default header budget.
func Transport() http.RoundTripper {
	return TransportWithHeaderBudget(DefaultTimeout)
}

// HTTPClient returns a bounded *http.Client whose transport injects W3C trace
// context into outbound requests, so gateway -> service -> service calls join
// one trace. Use this for unary (non-streaming) calls.
func HTTPClient() *http.Client {
	return HTTPClientWithTimeout(DefaultTimeout)
}

// HTTPClientWithTimeout is HTTPClient with an explicit total budget, for callers
// whose upstream is legitimately slower than the default. The header budget
// tracks it, so the advertised total is actually reachable.
func HTTPClientWithTimeout(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: TransportWithHeaderBudget(timeout)}
}

// StreamHTTPClient returns a traced client with NO total timeout, for a
// long-lived response body (SSE) that a total timeout would cut mid-flight.
//
// It is not unbounded: dial, TLS handshake and response-header phases are still
// capped by Transport(), so a peer that stalls BEFORE the stream starts fails
// fast. A peer that stalls AFTER headers must be bounded by the caller — pass a
// context with a deadline, or enforce a read-idle/heartbeat deadline. A stream
// is not an approved exception to INV-011.
func StreamHTTPClient() *http.Client {
	return StreamHTTPClientWithHeaderBudget(DefaultStreamHeaderBudget)
}

// DefaultStreamHeaderBudget bounds how long a stream may take to START.
const DefaultStreamHeaderBudget = 30 * time.Second

// StreamHTTPClientWithHeaderBudget is StreamHTTPClient with an explicit bound on
// the pre-stream phase.
func StreamHTTPClientWithHeaderBudget(headerBudget time.Duration) *http.Client {
	return &http.Client{Transport: TransportWithHeaderBudget(headerBudget)}
}
