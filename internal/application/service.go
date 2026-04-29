// Package application holds the core business logic for the logs-loki plugin.
package application

import (
	"context"
	"time"

	pluginsv1 "github.com/kleffio/plugin-sdk-go/v1"
)

// LogLine is the platform's standard log line shape.
type LogLine struct {
	WorkloadID string    `json:"workload_id"`
	Ts         time.Time `json:"ts"`
	Stream     string    `json:"stream"`
	Line       string    `json:"line"`
}

// LogStore is implemented by the Loki adapter.
type LogStore interface {
	Ingest(ctx context.Context, entry *pluginsv1.LogEntry) error
	Query(ctx context.Context, workloadID string, limit int) ([]LogLine, error)
}

// Service orchestrates log ingestion.
type Service struct {
	store LogStore
}

// New creates a Service backed by the given LogStore.
func New(store LogStore) *Service {
	return &Service{store: store}
}

// IngestLog forwards a log entry to the backing store.
func (s *Service) IngestLog(ctx context.Context, entry *pluginsv1.LogEntry) error {
	if entry == nil {
		return nil
	}
	return s.store.Ingest(ctx, entry)
}

// QueryLogs returns log lines for a workload in chronological order.
func (s *Service) QueryLogs(ctx context.Context, workloadID string, limit int) ([]LogLine, error) {
	return s.store.Query(ctx, workloadID, limit)
}
