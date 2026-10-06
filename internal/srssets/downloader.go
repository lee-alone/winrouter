package srssets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const maxSRSSize = 32 << 20

func fetchSRS(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/octet-stream")
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxSRSSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxSRSSize {
		return nil, errors.New("invalid response size")
	}
	if len(data) < 3 || string(data[:3]) != "SRS" {
		return nil, errors.New("invalid SRS header")
	}
	return data, nil
}
