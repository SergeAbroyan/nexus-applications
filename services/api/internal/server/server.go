package server

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/SergeAbroyan/nexus-applications/services/api/internal/items"
	"github.com/SergeAbroyan/nexus-applications/services/api/internal/telemetry"
)

//go:embed static
var embedded embed.FS

//go:embed openapi.yaml
var OpenAPI []byte

type ItemService interface {
	List(ctx context.Context) ([]items.Item, items.Source, error)
	Create(ctx context.Context, name string) (items.Item, error)
}

type NotificationService interface {
	Request(ctx context.Context, message string) error
}

type Check struct {
	Name string
	Ping func(context.Context) error
}

type Dependencies struct {
	Logger        *slog.Logger
	Metrics       *telemetry.Metrics
	Items         ItemService
	Notifications NotificationService
	Readiness     []Check
}

func New(dependencies Dependencies) http.Handler {
	handlers := &handlers{
		logger:        dependencies.Logger,
		items:         dependencies.Items,
		notifications: dependencies.Notifications,
		readiness:     dependencies.Readiness,
	}

	application := http.NewServeMux()
	application.Handle("GET /", http.FileServerFS(staticFiles()))
	application.HandleFunc("GET /api/items", handlers.listItems)
	application.HandleFunc("POST /api/items", handlers.createItem)
	application.HandleFunc("POST /api/notify", handlers.requestNotification)
	application.HandleFunc("GET /api/openapi.yaml", serveOpenAPI)

	instrumented := otelhttp.NewHandler(
		dependencies.Metrics.Instrument(securityHeaders(application)),
		"http.server",
		otelhttp.WithSpanNameFormatter(func(_ string, request *http.Request) string {
			return request.Method
		}),
	)

	routes := http.NewServeMux()
	routes.HandleFunc("GET /healthz", handlers.health)
	routes.HandleFunc("GET /readyz", handlers.ready)
	routes.Handle("GET /metrics", dependencies.Metrics.Handler())
	routes.Handle("/", instrumented)
	return routes
}

func serveOpenAPI(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "application/yaml")
	_, _ = response.Write(OpenAPI)
}

func staticFiles() fs.FS {
	files, err := fs.Sub(embedded, "static")
	if err != nil {
		panic(err)
	}
	return files
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		headers := response.Header()
		headers.Set("Content-Security-Policy", "default-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		headers.Set("Cross-Origin-Embedder-Policy", "require-corp")
		headers.Set("Cross-Origin-Opener-Policy", "same-origin")
		headers.Set("Cross-Origin-Resource-Policy", "same-origin")
		headers.Set("Permissions-Policy", "camera=(), geolocation=(), microphone=(), payment=(), usb=()")
		headers.Set("Referrer-Policy", "no-referrer")
		headers.Set("X-Content-Type-Options", "nosniff")
		headers.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(response, request)
	})
}
