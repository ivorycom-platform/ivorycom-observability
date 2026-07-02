package obs

import (
	"strconv"
	"time"

	"github.com/gofiber/contrib/otelfiber/v2"
	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// FiberMiddleware returns the two handlers every service should register, in
// order: otelfiber (span + W3C context propagation) followed by the RED
// metrics recorder. Register with:
//
//	for _, m := range obs.FiberMiddleware() { app.Use(m) }
func FiberMiddleware() []fiber.Handler {
	return []fiber.Handler{otelfiber.Middleware(), REDMiddleware()}
}

// REDMiddleware records the RED duration histogram
// (http.server.request.duration, seconds) labelled by route/method/status.
// Register it AFTER otelfiber.Middleware() so the span/context already exists.
func REDMiddleware() fiber.Handler {
	meter := otel.GetMeterProvider().Meter("ivorycom/http")
	dur, _ := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithUnit("s"),
		metric.WithDescription("HTTP server request duration"),
	)
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		route := c.Route().Path
		if route == "" {
			route = c.Path()
		}
		dur.Record(c.UserContext(), time.Since(start).Seconds(), metric.WithAttributes(
			attribute.String("http.route", route),
			attribute.String("http.request.method", c.Method()),
			attribute.String("http.response.status_code", strconv.Itoa(c.Response().StatusCode())),
		))
		return err
	}
}
