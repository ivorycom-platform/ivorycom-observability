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
//
// Unexported on purpose. While a raw transport was exported, a caller could pair
// it with &http.Client{Transport: obs.Transport()} — no total timeout — and get
// an unbounded client from sanctioned infrastructure. Removing StreamHTTPClient
// while leaving its building blocks exported would only have moved the hazard.
func transportWithHeaderBudget(headerBudget time.Duration) http.RoundTripper {
	requirePositive("transport header budget", headerBudget)
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
	requirePositive("HTTPClientWithTimeout", timeout)
	return &http.Client{
		Timeout:       timeout,
		Transport:     transportWithHeaderBudget(timeout),
		CheckRedirect: noRedirects,
	}
}

// noRedirects refuses to follow redirects. Go's default follows up to 10 and
// REWRITES POST to GET on 301/302/303, so a state-changing internal call could
// be silently turned into a GET whose 200 looks like success. Go also forwards
// custom trust headers to the redirect target; only Authorization and cookies
// are specially stripped. Internal RPC has no legitimate redirect.
func noRedirects(*http.Request, []*http.Request) error {
	return http.ErrUseLastResponse
}

func requirePositive(what string, d time.Duration) {
	if d <= 0 {
		panic("obs: " + what + " requires a positive duration; a zero or " +
			"negative budget disables the bound entirely")
	}
}

// NOTE ON STREAMING. This module deliberately exposes NO stream client.
//
// A helper that bounds only the pre-stream phases and then instructs future
// callers to enforce their own read-idle deadline is not INV-011-safe
// infrastructure — it is an unbounded client with a comment. There are no
// stream callers of this module today, so the honest move is to not ship the
// hazard: a service that needs one should use a client paired with an enforced
// read-idle reader (see ivorycom-crm-marketing/internal/httpx.IdleTimeoutReader
// for a tested implementation) and that pattern can be promoted here when a
// second consumer needs it.
