package clashapi

import "testing"

func TestAggregate(t *testing.T) {
	var c Connection
	c.Metadata.Network = "tcp"
	c.Metadata.DestinationIP = "1.2.3.4"
	c.Metadata.DestinationPort = "443"
	c.Metadata.Host = "cdn.example.cn"
	c.Rule = "geoip-cn"
	c.Chains = []string{"domestic-direct"}
	c.Upload = 2
	c.Download = 5
	result := Aggregate([]Connection{c, c})
	if len(result) != 1 || result[0].Attempts != 2 || result[0].BytesDown != 10 || result[0].AddressType != "A" {
		t.Fatalf("unexpected %#v", result)
	}
}
