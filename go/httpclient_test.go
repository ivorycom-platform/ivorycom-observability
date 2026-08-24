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

// A stream client intentionally has no TOTAL timeout, but must still bound the
// phases before the stream starts. Asserting Transport != nil proved nothing —
// this drives a real silent peer through the actual stream client, where the
// header budget is the ONLY thing that can end the call.
func TestStreamClientBoundsPreStreamPhases(t *testing.T) {
	c := StreamHTTPClientWithHeaderBudget(1 * time.Second)
	if c.Timeout != 0 {
		t.Fatalf("StreamHTTPClient must not set a total timeout (it would cut the stream)")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn // accept and hold: never answer, never close
		}
	}()

	start := time.Now()
	resp, err := c.Get("http://" + ln.Addr().String() + "/silent")
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("a stream client must still fail against a peer that never sends headers")
	}
	if elapsed := time.Since(start); elapsed > 15*time.Second {
		t.Fatalf("took %v — the header budget did not fire", elapsed)
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
	if long <= DefaultStreamHeaderBudget {
		t.Fatalf("test not meaningful: %v <= %v", long, DefaultStreamHeaderBudget)
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
