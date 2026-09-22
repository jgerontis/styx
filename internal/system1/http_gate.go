package system1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HTTPGate talks to a local System 1 sidecar process (e.g. a small Python
// server preloading a Laya checkpoint) over HTTP. The sidecar's lifecycle is
// managed outside of Styx; HTTPGate only performs the request/response.
type HTTPGate struct {
	Endpoint string
	Client   *http.Client
}

// NewHTTPGate builds an HTTPGate with a short default timeout, since System 1
// answers are only useful if they're fast enough to sit ahead of every tool
// call and phase transition.
func NewHTTPGate(endpoint string) *HTTPGate {
	return &HTTPGate{
		Endpoint: endpoint,
		Client:   &http.Client{Timeout: 2 * time.Second},
	}
}

type predictRequest struct {
	State     map[string]any      `json:"state"`
	Questions map[string]Question `json:"questions"`
}

type predictResponse struct {
	Answers map[string]Answer `json:"answers"`
}

// Answer sends a single Question to the sidecar's /predict endpoint.
func (g *HTTPGate) Answer(ctx context.Context, q Question, state map[string]any) (Answer, error) {
	body, err := json.Marshal(predictRequest{
		State:     state,
		Questions: map[string]Question{q.Name: q},
	})
	if err != nil {
		return Answer{}, fmt.Errorf("marshal system1 request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.Endpoint+"/predict", bytes.NewReader(body))
	if err != nil {
		return Answer{}, fmt.Errorf("build system1 request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.Client.Do(req)
	if err != nil {
		return Answer{}, fmt.Errorf("call system1 sidecar: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Answer{}, fmt.Errorf("system1 sidecar returned status %d", resp.StatusCode)
	}

	var out predictResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Answer{}, fmt.Errorf("decode system1 response: %w", err)
	}

	return out.Answers[q.Name], nil
}
