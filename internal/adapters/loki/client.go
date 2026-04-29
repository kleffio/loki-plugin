// Package loki implements LogStore backed by Grafana Loki.
// Log entries are written using the Loki push API and read via query_range.
package loki

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	pluginsv1 "github.com/kleffio/plugin-sdk-go/v1"
	"github.com/kleffio/loki-plugin/internal/application"
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

// Query fetches log lines for a workload from Loki and returns them in
// chronological order using the platform's standard LogLine shape.
func (c *Client) Query(ctx context.Context, workloadID string, limit int) ([]application.LogLine, error) {
	if limit <= 0 {
		limit = 200
	}

	params := url.Values{
		"query":     {fmt.Sprintf(`{workload_id=%q}`, workloadID)},
		"limit":     {strconv.Itoa(limit)},
		"direction": {"backward"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/loki/api/v1/query_range?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("loki: build query request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("loki: query: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("loki: query returned status %d", resp.StatusCode)
	}

	var result lokiQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("loki: decode query response: %w", err)
	}

	var lines []application.LogLine
	for _, s := range result.Data.Result {
		streamLabel := s.Stream["stream"]
		if streamLabel == "" {
			streamLabel = "stdout"
		}
		for _, v := range s.Values {
			nsec, err := strconv.ParseInt(v[0], 10, 64)
			if err != nil {
				continue
			}
			lines = append(lines, application.LogLine{
				WorkloadID: workloadID,
				Ts:         time.Unix(0, nsec),
				Stream:     streamLabel,
				Line:       v[1],
			})
		}
	}

	// Loki returns newest-first (direction=backward); reverse to chronological.
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}
	return lines, nil
}

type lokiQueryResponse struct {
	Data struct {
		Result []struct {
			Stream map[string]string `json:"stream"`
			Values [][2]string       `json:"values"`
		} `json:"result"`
	} `json:"data"`
}
