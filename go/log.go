package obs

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/trace"
)

// stdout is a test seam; production writes JSON logs to os.Stdout. Railway
// captures stdout too, so logs survive even if the OTLP path is degraded.
var stdout io.Writer = os.Stdout

// Logger returns a request-scoped zerolog logger. When ctx carries a span, the
// trace_id/span_id are attached; every event is also mirrored to the OTel
// LoggerProvider so logs land in Loki correlated to traces.
func Logger(ctx context.Context) zerolog.Logger {
	l := zerolog.New(stdout).With().Timestamp().Logger()
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		l = l.With().
			Str("trace_id", sc.TraceID().String()).
			Str("span_id", sc.SpanID().String()).
			Logger()
	}
	if loggerProvider != nil {
		l = l.Hook(otelHook{ctx: ctx})
	}
	return l
}

// otelHook mirrors each zerolog event to the OTLP LoggerProvider.
type otelHook struct{ ctx context.Context }

func (h otelHook) Run(_ *zerolog.Event, level zerolog.Level, msg string) {
	if loggerProvider == nil {
		return
	}
	var rec otellog.Record
	rec.SetTimestamp(time.Now())
	rec.SetBody(otellog.StringValue(msg))
	rec.SetSeverityText(level.String())
	rec.SetSeverity(otelSeverity(level))
	loggerProvider.Logger("ivorycom").Emit(h.ctx, rec)
}

func otelSeverity(l zerolog.Level) otellog.Severity {
	switch l {
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
