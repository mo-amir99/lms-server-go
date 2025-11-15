package bunny

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// StatisticsClient provides access to Bunny's account-level statistics API (api.bunny.net).
type StatisticsClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewStatisticsClient creates a new client for the Bunny statistics API.
func NewStatisticsClient(baseURL, apiKey string) *StatisticsClient {
	trimmedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmedBaseURL == "" {
		trimmedBaseURL = "https://api.bunny.net"
	}

	return &StatisticsClient{
		baseURL: trimmedBaseURL,
		apiKey:  strings.TrimSpace(apiKey),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// BandwidthSummary represents aggregated bandwidth usage for a time range.
type BandwidthSummary struct {
	TotalBandwidthBytes int64
	RangeStart          time.Time
	RangeEnd            time.Time
}

// BandwidthUsage fetches aggregated bandwidth usage between two timestamps.
func (c *StatisticsClient) BandwidthUsage(ctx context.Context, from, to time.Time) (BandwidthSummary, error) {
	summary := BandwidthSummary{RangeStart: from, RangeEnd: to}

	if c == nil {
		return summary, fmt.Errorf("statistics client is not configured")
	}
	if strings.TrimSpace(c.apiKey) == "" {
		return summary, fmt.Errorf("bunny statistics API key is missing")
	}

	if from.After(to) {
		from, to = to, from
		summary.RangeStart, summary.RangeEnd = from, to
	}

	params := url.Values{}
	params.Set("dateFrom", from.UTC().Format(time.RFC3339))
	params.Set("dateTo", to.UTC().Format(time.RFC3339))

	endpoint := fmt.Sprintf("%s/statistics/bandwidth?%s", c.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return summary, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("AccessKey", c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "LMS-Server-Go/1.0.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return summary, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return summary, fmt.Errorf("bunny statistics error: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
	}

	var payload struct {
		TotalBandwidthUsed float64 `json:"totalBandwidthUsed"`
		Summary            struct {
			TotalBandwidthUsed float64 `json:"totalBandwidthUsed"`
		} `json:"summary"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return summary, fmt.Errorf("failed to decode bandwidth response: %w", err)
	}

	totalBytes := payload.TotalBandwidthUsed
	if totalBytes == 0 && payload.Summary.TotalBandwidthUsed > 0 {
		totalBytes = payload.Summary.TotalBandwidthUsed
	}

	summary.TotalBandwidthBytes = int64(totalBytes)
	return summary, nil
}
