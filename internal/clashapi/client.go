package clashapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL, Secret string
	HTTPClient      *http.Client
}

type Connection struct {
	ID       string `json:"id"`
	Metadata struct {
		Network         string `json:"network"`
		Type            string `json:"type"`
		SourceIP        string `json:"sourceIP"`
		SourcePort      string `json:"sourcePort"`
		DestinationIP   string `json:"destinationIP"`
		DestinationPort string `json:"destinationPort"`
		Host            string `json:"host"`
		DNSMode         string `json:"dnsMode"`
		ProcessPath     string `json:"processPath"`
	} `json:"metadata"`
	Rule        string   `json:"rule"`
	RulePayload string   `json:"rulePayload"`
	Chains      []string `json:"chains"`
	Upload      int64    `json:"upload"`
	Download    int64    `json:"download"`
	Start       string   `json:"start"`
}

type Connections struct {
	Connections   []Connection `json:"connections"`
	DownloadTotal int64        `json:"downloadTotal"`
	UploadTotal   int64        `json:"uploadTotal"`
}

func (c Client) ListConnections(ctx context.Context) (Connections, error) {
	base := strings.TrimRight(c.BaseURL, "/")
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/connections", nil)
	if err != nil {
		return Connections{}, err
	}
	if c.Secret != "" {
		request.Header.Set("Authorization", "Bearer "+c.Secret)
	}
	response, err := (func() (*http.Response, error) {
		if c.HTTPClient != nil {
			return c.HTTPClient.Do(request)
		}
		return (&http.Client{Timeout: 3 * time.Second}).Do(request)
	})()
	if err != nil {
		return Connections{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return Connections{}, fmt.Errorf("clash api returned HTTP %d", response.StatusCode)
	}
	var result Connections
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return Connections{}, fmt.Errorf("decode connections: %w", err)
	}
	return result, nil
}
