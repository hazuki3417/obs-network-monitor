package probe

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type httpProbe struct {
	client *http.Client
}

func newHTTPProbe() Runner {
	return &httpProbe{client: &http.Client{}}
}

func newHTTPProbeWithClient(client *http.Client) Runner {
	return &httpProbe{client: client}
}

func (probe *httpProbe) Probe(ctx context.Context, target string, timeout time.Duration) (time.Duration, error) {
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(probeCtx, http.MethodGet, target, nil)
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}

	start := time.Now()
	response, err := probe.client.Do(request)
	rtt := time.Since(start)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4*1024))

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusBadRequest {
		return 0, fmt.Errorf("unexpected HTTP status %d", response.StatusCode)
	}
	return rtt, nil
}
