package obs

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

// captureProcessor records every emitted OTLP log record for assertions.
type captureProcessor struct{ records []sdklog.Record }

func (p *captureProcessor) Enabled(context.Context, sdklog.EnabledParameters) bool { return true }

func (p *captureProcessor) OnEmit(_ context.Context, r *sdklog.Record) error {
	p.records = append(p.records, *r)
	return nil
}
func (p *captureProcessor) Shutdown(context.Context) error   { return nil }
func (p *captureProcessor) ForceFlush(context.Context) error { return nil }

// withCaptureProvider swaps the package loggerProvider for one backed by a
// capturing processor, returning the capture and a restore func.
func withCaptureProvider(t *testing.T) *captureProcessor {
	t.Helper()
	cap := &captureProcessor{}
	orig := loggerProvider
	loggerProvider = sdklog.NewLoggerProvider(sdklog.WithProcessor(cap))
	t.Cleanup(func() { loggerProvider = orig })
	return cap
}

func TestGlobalZerologExportsFullJSONLine(t *testing.T) {
	cap := withCaptureProvider(t)

	var buf bytes.Buffer
	origStdout := stdout
	stdout = &buf
	t.Cleanup(func() { stdout = origStdout })

	origLogger := zlog.Logger
	t.Cleanup(func() { zlog.Logger = origLogger })

	HookGlobalZerolog()
	zlog.Warn().Str("tenant_id", "t-123").Int("status", 429).Msg("rate limited")

	// stdout still gets the line (Railway log survival guarantee)
	var onStdout map[string]any
	if err := json.Unmarshal(buf.Bytes(), &onStdout); err != nil {
		t.Fatalf("stdout line not JSON: %v (%q)", err, buf.String())
	}

	if len(cap.records) != 1 {
		t.Fatalf("expected 1 OTLP record, got %d", len(cap.records))
	}
	rec := cap.records[0]
	if rec.SeverityText() != "warn" {
		t.Fatalf("expected severity text warn, got %q", rec.SeverityText())
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(rec.Body().AsString()), &body); err != nil {
		t.Fatalf("OTLP body not the JSON line: %v (%q)", err, rec.Body().AsString())
	}
	// The whole point of the LevelWriter bridge: fields survive, not just msg.
	if body["tenant_id"] != "t-123" || body["status"] != float64(429) || body["message"] != "rate limited" {
		t.Fatalf("fields missing from exported line: %v", body)
	}
}

func TestLoggerExportsFieldsAndLevels(t *testing.T) {
	cap := withCaptureProvider(t)

	var buf bytes.Buffer
	origStdout := stdout
	stdout = &buf
	t.Cleanup(func() { stdout = origStdout })

	log := Logger(context.Background())
	log.Error().Str("k", "v").Msg("boom")

	if len(cap.records) != 1 {
		t.Fatalf("expected 1 OTLP record, got %d", len(cap.records))
	}
	rec := cap.records[0]
	if rec.SeverityText() != "error" {
		t.Fatalf("expected severity error, got %q", rec.SeverityText())
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(rec.Body().AsString()), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body["k"] != "v" || body["message"] != "boom" {
		t.Fatalf("fields missing: %v", body)
	}
}

func TestZerologWriterWithoutInitIsStdoutOnly(t *testing.T) {
	orig := loggerProvider
	loggerProvider = nil
	t.Cleanup(func() { loggerProvider = orig })

	var buf bytes.Buffer
	l := zerolog.New(ZerologWriter(&buf))
	l.Info().Msg("local run")
	if buf.Len() == 0 {
		t.Fatal("expected line on the tee'd writer even without Init")
	}
}
