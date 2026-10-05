package srssets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const maxSRSSize = 32 << 20

func fetchSRS(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	urls := []string{rawURL}
	if strings.HasPrefix(rawURL, "https://raw.githubusercontent.com/") {
		urls = append(urls, "https://ghfast.top/"+rawURL)
	}
	var lastErr error
	for _, targetURL := range urls {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Accept", "application/octet-stream")
		response, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", response.StatusCode)
			continue
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, maxSRSSize+1))
		response.Body.Close()
		if err != nil || len(data) > maxSRSSize {
			lastErr = errors.New("invalid response size")
			continue
		}
		if len(data) < 3 || string(data[:3]) != "SRS" {
			lastErr = errors.New("invalid SRS header")
			continue
		}
		return data, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("download failed")
}
