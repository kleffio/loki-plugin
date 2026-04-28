// Command plugin is the entrypoint for the logs-loki Kleff plugin.
// It receives workload log entries via gRPC and writes them to Loki.
package main

import (
	"log/slog"
	"net"
	"os"
	"os/signal"
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

	gs := grpc.NewServer()
	pluginsv1.RegisterPluginHealthServer(gs, srv)
	pluginsv1.RegisterMonitoringLogsServer(gs, srv)

	port := env("PLUGIN_PORT", "50051")
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logger.Error("listen failed", "error", err)
		os.Exit(1)
	}

	go func() {
		logger.Info("plugin gRPC listening", "port", port)
		if err := gs.Serve(lis); err != nil {
			logger.Error("gRPC server error", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	<-stop
	logger.Info("shutting down")
	gs.GracefulStop()
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
