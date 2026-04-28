// Package grpc is the inbound gRPC adapter for the logs-loki plugin.
package grpc

import (
	"context"

	pluginsv1 "github.com/kleffio/plugin-sdk-go/v1"
	"github.com/kleffio/loki-plugin/internal/application"
)

// Server implements PluginHealth and MonitoringLogs gRPC services.
type Server struct {
	pluginsv1.UnimplementedPluginHealthServer
	pluginsv1.UnimplementedMonitoringLogsServer
	svc *application.Service
}

// New creates a Server wired to the given application service.
func New(svc *application.Service) *Server {
	return &Server{svc: svc}
}

// Health reports the plugin as healthy.
func (s *Server) Health(_ context.Context, _ *pluginsv1.HealthRequest) (*pluginsv1.HealthResponse, error) {
	return &pluginsv1.HealthResponse{Status: pluginsv1.HealthStatusHealthy}, nil
}

// GetCapabilities declares the monitoring.logs capability.
func (s *Server) GetCapabilities(_ context.Context, _ *pluginsv1.GetCapabilitiesRequest) (*pluginsv1.GetCapabilitiesResponse, error) {
	return &pluginsv1.GetCapabilitiesResponse{
		Capabilities: []string{pluginsv1.CapabilityMonitoringLogs},
	}, nil
}

// IngestLog receives a log entry from the platform and writes it to Loki.
func (s *Server) IngestLog(ctx context.Context, req *pluginsv1.IngestLogRequest) (*pluginsv1.IngestLogResponse, error) {
	if err := s.svc.IngestLog(ctx, req.Entry); err != nil {
		return &pluginsv1.IngestLogResponse{
			Error: &pluginsv1.PluginError{
				Code:    pluginsv1.ErrorCodeInternal,
				Message: err.Error(),
			},
		}, nil
	}
	return &pluginsv1.IngestLogResponse{}, nil
}
