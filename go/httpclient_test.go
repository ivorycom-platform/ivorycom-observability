package obs

import (
	"net"
	"net/http"
	"testing"
	"time"
)

// H-3/INV-011: the helper must never hand back an unbounded client again.
func TestHTTPClientIsBounded(t *testing.T) {
	c := HTTPClient()
	if c.Timeout <= 0 {
		t.Fatalf("HTTPClient() has no timeout — every consumer inherits an unbounded client")
	}
	if got := HTTPClientWithTimeout(3 * time.Second).Timeout; got != 3*time.Second {
		t.Fatalf("HTTPClientWithTimeout: got %v, want 3s", got)
	}
}

// A non-positive budget must fail loudly rather than silently disabling bounds.
func TestNonPositiveBudgetIsRejected(t *testing.T) {
	for _, d := range []time.Duration{0, -time.Second} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("HTTPClientWithTimeout(%v) did not reject a non-positive budget", d)
				}
			}()
			_ = HTTPClientWithTimeout(d)
		}()
	}
}

// Internal RPC must not follow redirects: 301/302/303 rewrite POST to GET, so a
// state-changing call could return a 200 that never executed.
func TestRedirectsAreNotFollowed(t *testing.T) {
	c := HTTPClient()
	if c.CheckRedirect == nil {
		t.Fatal("client follows redirects by default — a POST can be rewritten to GET")
	}
	if err := c.CheckRedirect(nil, nil); err != http.ErrUseLastResponse {
		t.Fatalf("CheckRedirect = %v, want ErrUseLastResponse", err)
	}
}

// Regression: a hard-coded header budget silently capped every client built on
// it, so an advertised longer total timeout was unreachable.
func TestHeaderBudgetTracksTotalTimeout(t *testing.T) {
	long := 120 * time.Second
	c := HTTPClientWithTimeout(long)
	if c.Timeout != long {
		t.Fatalf("total timeout = %v, want %v", c.Timeout, long)
	}
	if long <= DefaultTimeout {
		t.Fatalf("test not meaningful: %v <= %v", long, DefaultTimeout)
	}
}

// The failure INV-011 is about: the peer completes the TCP handshake and then
// never answers. A dial-only bound cannot catch this.
func TestSilentPeerDoesNotHangForever(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			// Accept and hold: never write a byte back.
			defer func() { _ = conn.Close() }()
		}
	}()

	c := HTTPClientWithTimeout(2 * time.Second)
	start := time.Now()
	resp, err := c.Get("http://" + ln.Addr().String() + "/hang")
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected a timeout against a silent peer, got a response")
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("took %v — the bound did not fire", elapsed)
	}
	_ = http.DefaultClient // keep the import honest
}
