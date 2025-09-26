package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// JSONRPCNotifier delivers events to a custom HTTP endpoint.
type JSONRPCNotifier struct {
	url        string
	httpClient *http.Client
}

// NewJSONRPCNotifier builds a notifier targeting the supplied endpoint.
func NewJSONRPCNotifier(url string) *JSONRPCNotifier {
	return &JSONRPCNotifier{
		url:        url,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Notify posts a minimal JSON body with a single message field required by the downstream endpoint.
func (j *JSONRPCNotifier) Notify(ctx context.Context, event SupplyChangeEvent) error {
	oldValue := "n/a"
	if event.OldTotalSupply != nil {
		oldValue = event.OldTotalSupply.String()
	}

	body := map[string]string{
		"message": fmt.Sprintf("asset %s total supply changed: %s -> %s", event.AssetName, oldValue, event.NewTotalSupply.String()),
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal json payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, j.url, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("build post request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := j.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send post request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("json endpoint returned status %s", resp.Status)
	}

	return nil
}
