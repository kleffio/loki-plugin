// Command plugin is the entrypoint for the logs-loki Kleff plugin.
// It receives workload log entries via gRPC and serves log queries over HTTP.
package main

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	pluginsv1 "github.com/kleffio/plugin-sdk-go/v1"
	grpcadapter "github.com/kleffio/loki-plugin/internal/adapters/grpc"
	lokiadapter "github.com/kleffio/loki-plugin/internal/adapters/loki"
	"github.com/kleffio/loki-plugin/internal/application"
	"google.golang.org/grpc"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	lokiURL := env("LOKI_URL", "http://loki:3100")
	lokiClient := lokiadapter.New(lokiURL)
	logger.Info("loki backend", "url", lokiURL)

	svc := application.New(lokiClient)
	srv := grpcadapter.New(svc)

	// gRPC server — receives IngestLog calls from the platform.
	gs := grpc.NewServer()
	pluginsv1.RegisterPluginHealthServer(gs, srv)
	pluginsv1.RegisterMonitoringLogsServer(gs, srv)

	grpcPort := env("PLUGIN_PORT", "50051")
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		logger.Error("listen failed", "error", err)
		os.Exit(1)
	}

	go func() {
		logger.Info("plugin gRPC listening", "port", grpcPort)
		if err := gs.Serve(lis); err != nil {
			logger.Error("gRPC server error", "error", err)
			os.Exit(1)
		}
	}()

	// HTTP server — serves generic log queries from the platform.
	httpPort := env("PLUGIN_HTTP_PORT", "8080")
	mux := http.NewServeMux()
	mux.HandleFunc("/logs", makeLogsHandler(svc, logger))
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	hs := &http.Server{Addr: ":" + httpPort, Handler: mux}
	go func() {
		logger.Info("plugin HTTP listening", "port", httpPort)
		if err := hs.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	<-stop
	logger.Info("shutting down")
	gs.GracefulStop()
	_ = hs.Close()
}

// makeLogsHandler returns a handler for GET /logs?workload_id=<id>&limit=<n>
// that satisfies the platform's standard log query contract.
func makeLogsHandler(svc *application.Service, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workloadID := r.URL.Query().Get("workload_id")
		if workloadID == "" {
			http.Error(w, "workload_id is required", http.StatusBadRequest)
			return
		}
		limit := 200
		if s := r.URL.Query().Get("limit"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				limit = n
			}
		}

		lines, err := svc.QueryLogs(r.Context(), workloadID, limit)
		if err != nil {
			logger.Warn("log query failed", "workload_id", workloadID, "error", err)
			http.Error(w, "query failed", http.StatusInternalServerError)
			return
		}
		if lines == nil {
			lines = []application.LogLine{}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"lines": lines})
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
