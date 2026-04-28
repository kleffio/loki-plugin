// Package application holds the core business logic for the logs-loki plugin.
package application

import (
	"context"

	pluginsv1 "github.com/kleffio/plugin-sdk-go/v1"
)

// LogStore is implemented by the Loki adapter.
type LogStore interface {
	Ingest(ctx context.Context, entry *pluginsv1.LogEntry) error
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
