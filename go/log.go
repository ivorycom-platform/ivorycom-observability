package obs

import (
	"bytes"
	"context"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/trace"
)

// stdout is a test seam; production writes JSON logs to os.Stdout. Railway
// captures stdout too, so logs survive even if the OTLP path is degraded.
var stdout io.Writer = os.Stdout

// otelLineWriter is a zerolog.LevelWriter that mirrors every fully-rendered
// JSON log line to the OTLP LoggerProvider. Unlike a zerolog Hook (which only
// sees the message), a LevelWriter receives the complete line — all fields
// survive into Loki. ctx, when set, carries the active span so the record is
// trace-correlated.
type otelLineWriter struct{ ctx context.Context }

func (w otelLineWriter) Write(p []byte) (int, error) {
	return w.WriteLevel(zerolog.NoLevel, p)
}

func (w otelLineWriter) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	if loggerProvider != nil {
		ctx := w.ctx
		if ctx == nil {
			ctx = context.Background()
		}
		var rec otellog.Record
		rec.SetTimestamp(time.Now())
		rec.SetBody(otellog.StringValue(string(bytes.TrimRight(p, "\n"))))
		rec.SetSeverityText(level.String())
		rec.SetSeverity(otelSeverity(level))
		loggerProvider.Logger("ivorycom").Emit(ctx, rec)
	}
	return len(p), nil
}

// ZerologWriter tees every rendered zerolog line to next (normally os.Stdout)
// and to the OTLP log pipeline. Use it when constructing a service-owned
// logger: zerolog.New(obs.ZerologWriter(os.Stdout)).
func ZerologWriter(next io.Writer) io.Writer {
	return zerolog.MultiLevelWriter(next, otelLineWriter{})
}

// HookGlobalZerolog reroutes the global zerolog logger (rs/zerolog/log.Logger)
// through ZerologWriter so a service's existing log.Info()… calls export full
// JSON lines to Loki. Call once after Init; without Init it degrades to
// stdout-only, so local/CI runs are unaffected.
func HookGlobalZerolog() {
	zlog.Logger = zerolog.New(ZerologWriter(stdout)).With().Timestamp().Logger()
}

// Logger returns a request-scoped zerolog logger. When ctx carries a span, the
// trace_id/span_id are attached as fields and the OTLP record is emitted with
// ctx so Loki correlates it to the trace.
func Logger(ctx context.Context) zerolog.Logger {
	w := zerolog.MultiLevelWriter(stdout, otelLineWriter{ctx: ctx})
	l := zerolog.New(w).With().Timestamp().Logger()
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		l = l.With().
			Str("trace_id", sc.TraceID().String()).
			Str("span_id", sc.SpanID().String()).
			Logger()
	}
	return l
}

func otelSeverity(l zerolog.Level) otellog.Severity {
	switch l {
	case zerolog.TraceLevel:
		return otellog.SeverityTrace
	case zerolog.DebugLevel:
		return otellog.SeverityDebug
	case zerolog.InfoLevel:
		return otellog.SeverityInfo
	case zerolog.WarnLevel:
		return otellog.SeverityWarn
	case zerolog.ErrorLevel:
		return otellog.SeverityError
	case zerolog.FatalLevel:
		return otellog.SeverityFatal
	case zerolog.PanicLevel:
		return otellog.SeverityFatal
	default:
		return otellog.SeverityInfo
	}
}
