package clashapi

import (
	"net"
	"net/url"
	"sort"
	"strings"
	"time"
)

type Summary struct {
	Domain      string    `json:"domain,omitempty"`
	AddressType string    `json:"address_type,omitempty"`
	IP          string    `json:"ip,omitempty"`
	Protocol    string    `json:"protocol,omitempty"`
	Rule        string    `json:"rule,omitempty"`
	Outbound    string    `json:"outbound,omitempty"`
	Attempts    int       `json:"attempts"`
	Established int       `json:"established"`
	BytesUp     int64     `json:"bytes_up"`
	BytesDown   int64     `json:"bytes_down"`
	LastSeen    time.Time `json:"last_seen"`
}

func Aggregate(values []Connection) []Summary {
	items := map[string]Summary{}
	for _, value := range values {
		ip := strings.TrimSpace(value.Metadata.DestinationIP)
		name := strings.TrimSpace(value.Metadata.Host)
		if name == "" {
			name = ip
		}
		if parsed, err := url.Parse("//" + name); err == nil && parsed.Hostname() != "" {
			name = parsed.Hostname()
		}
		addressType := ""
		if parsedIP := net.ParseIP(ip); parsedIP != nil && parsedIP.To4() != nil {
			addressType = "A"
		} else if parsedIP != nil {
			addressType = "AAAA"
		}
		outbound := ""
		if len(value.Chains) > 0 {
			outbound = value.Chains[len(value.Chains)-1]
		}
		key := strings.Join([]string{name, addressType, ip, value.Metadata.Network, value.Rule, outbound}, "|")
		item := items[key]
		item.Domain = name
		item.AddressType = addressType
		item.IP = ip
		item.Protocol = value.Metadata.Network
		item.Rule = value.Rule
		item.Outbound = outbound
		item.Attempts++
		item.Established++
		item.BytesUp += value.Upload
		item.BytesDown += value.Download
		if parsed, err := time.Parse(time.RFC3339Nano, value.Start); err == nil {
			item.LastSeen = parsed
		} else {
			item.LastSeen = time.Now()
		}
		items[key] = item
	}
	result := make([]Summary, 0, len(items))
	for _, item := range items {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].LastSeen.After(result[j].LastSeen) })
	return result
}
