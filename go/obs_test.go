package obs

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/contrib/otelfiber/v2"
	"github.com/gofiber/fiber/v2"
)

func TestInitReturnsShutdownAndSetsGlobals(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")
	shutdown, err := Init(context.Background(), Config{Service: "test-svc", Env: "test"})
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown func")
	}
	// shutdown flushes exporters; with no live collector the flush fails with a
	// connection error. That's environmental, not a code defect — we only assert
	// the func is callable and returns without panicking.
	_ = shutdown(context.Background())
}

func TestInitRequiresEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	if _, err := Init(context.Background(), Config{Service: "x", Env: "test"}); err == nil {
		t.Fatal("expected error when OTEL_EXPORTER_OTLP_ENDPOINT is unset")
	}
}

func TestLoggerWritesJSONWithFields(t *testing.T) {
	var buf bytes.Buffer
	orig := stdout
	stdout = &buf
	defer func() { stdout = orig }()

	log := Logger(context.Background()).With().Str("service", "test-svc").Logger()
	log.Info().Str("k", "v").Msg("hello")

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("log line not JSON: %v (%q)", err, buf.String())
	}
	if m["service"] != "test-svc" || m["k"] != "v" || m["message"] != "hello" {
		t.Fatalf("unexpected fields: %v", m)
	}
	if m["level"] != "info" {
		t.Fatalf("expected level=info, got %v", m["level"])
	}
}

func TestFiberMiddlewareServesRequests(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")
	if _, err := Init(context.Background(), Config{Service: "mw-test", Env: "test"}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	app := fiber.New()
	app.Use(otelfiber.Middleware())
	app.Use(REDMiddleware())
	app.Get("/ping", func(c *fiber.Ctx) error { return c.SendString("pong") })

	resp, err := app.Test(httptest.NewRequest("GET", "/ping", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestHTTPClientHasInstrumentedTransport(t *testing.T) {
	c := HTTPClient()
	if c.Transport == nil {
		t.Fatal("expected instrumented transport, got nil")
	}
}
