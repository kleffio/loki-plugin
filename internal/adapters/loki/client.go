// Package loki implements LogStore backed by Grafana Loki.
// Log entries are written using the Loki push API:
// POST /loki/api/v1/push
package loki

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	pluginsv1 "github.com/kleffio/plugin-sdk-go/v1"
)

// Client writes log entries to a Loki instance.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a Client that sends logs to the Loki instance at baseURL.
func New(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

type pushRequest struct {
	Streams []stream `json:"streams"`
}

type stream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

// Ingest encodes the log entry and POSTs it to Loki /loki/api/v1/push.
func (c *Client) Ingest(ctx context.Context, entry *pluginsv1.LogEntry) error {
	labels := map[string]string{
		"workload_id":   entry.WorkloadID,
		"workload_name": entry.WorkloadName,
		"org_id":        entry.OrgID,
		"project_id":    entry.ProjectID,
	}
	if entry.Level != "" {
		labels["level"] = entry.Level
	}
	for k, v := range entry.Labels {
		labels[k] = v
	}

	ts := strconv.FormatInt(entry.Timestamp, 10)

	payload := pushRequest{
		Streams: []stream{
			{
				Stream: labels,
				Values: [][]string{{ts, entry.Message}},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("loki: marshal payload: %w", err)
	}

	url := c.baseURL + "/loki/api/v1/push"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("loki: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("loki: post log: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("loki: unexpected status %d", resp.StatusCode)
	}
	return nil
}
