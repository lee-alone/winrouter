package config

import (
	"encoding/json"
	"testing"
)

func TestGenerateMinimalTUN(t *testing.T) {
	data, err := GenerateMinimalTUN("172.19.0.0/30", TUNStackSystem)
	if err != nil {
		t.Fatalf("GenerateMinimalTUN() error: %v", err)
	}
	var model MinimalTUN
	if err := json.Unmarshal(data, &model); err != nil {
		t.Fatalf("generated JSON is invalid: %v", err)
	}
	if len(model.Inbounds) != 1 || model.Inbounds[0].Address[0] != "172.19.0.1/30" || model.Inbounds[0].Stack != TUNStackSystem {
		t.Fatalf("inbound = %#v", model.Inbounds)
	}
	if len(model.Outbounds) != 1 || model.Route.Final != model.Outbounds[0].Tag {
		t.Fatalf("route/outbounds mismatch: %#v %#v", model.Route, model.Outbounds)
	}
	if len(model.DNS.Servers) != 1 || model.DNS.Final != model.DNS.Servers[0].Tag || len(model.Route.Rules) != 2 || model.Route.Rules[0].Action != "sniff" || model.Route.Rules[1].Action != "hijack-dns" {
		t.Fatalf("DNS loop prevention is incomplete: DNS=%#v route=%#v", model.DNS, model.Route)
	}
}

func TestGenerateMinimalTUNRejectsInvalidInput(t *testing.T) {
	for _, test := range []struct{ prefix, stack string }{
		{"bad", TUNStackSystem}, {"fd00::/126", TUNStackSystem}, {"172.19.0.0/31", TUNStackSystem}, {"172.19.0.0/30", "unknown"},
	} {
		if _, err := GenerateMinimalTUN(test.prefix, test.stack); err == nil {
			t.Fatalf("GenerateMinimalTUN(%q, %q) succeeded", test.prefix, test.stack)
		}
	}
}
