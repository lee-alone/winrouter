package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGenerateMVPMapsPolicyOrderToFinalJSON(t *testing.T) {
	input := fixtureInput(t)
	generated, err := GenerateMVP(input)
	if err != nil {
		t.Fatal(err)
	}
	var decoded MinimalTUN
	if err := json.Unmarshal(generated.JSON, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded.Route.Rules, generated.Model.Route.Rules) {
		t.Fatal("serialized route rules differ from policy mapping")
	}
	if decoded.Route.Final != "foreign-direct" {
		t.Fatalf("route.final = %q", decoded.Route.Final)
	}
	if len(decoded.Outbounds) != 4 || decoded.Outbounds[3].Tag != "bypass-001" {
		t.Fatalf("outbounds = %#v", decoded.Outbounds)
	}
	if !decoded.DNS.IndependentCache || decoded.DNS.Servers[0].Detour != "domestic-direct" || decoded.DNS.Servers[1].Detour != "foreign-direct" {
		t.Fatalf("DNS = %#v", decoded.DNS)
	}
	if decoded.DNS.Rules[0].Action != "reject" || decoded.DNS.Rules[0].QueryType[0] != "AAAA" {
		t.Fatalf("DNS IPv6 policy = %#v", decoded.DNS.Rules)
	}
	foundPrivateDirect, foundPrivateReject, foundIPv6Reject := false, false, false
	for _, rule := range decoded.Route.Rules {
		if len(rule.IPCIDR) == 1 && rule.IPCIDR[0] == "192.168.10.0/24" && rule.Outbound == "domestic-direct" {
			foundPrivateDirect = true
		}
		for _, prefix := range rule.IPCIDR {
			if prefix == "192.168.0.0/16" && rule.Action == "reject" {
				foundPrivateReject = true
			}
		}
		if rule.IPVersion == 6 && rule.Action == "reject" {
			foundIPv6Reject = true
		}
	}
	if !foundPrivateDirect || !foundPrivateReject || !foundIPv6Reject {
		t.Fatalf("missing direct/reserved/IPv6 rules: %#v", decoded.Route.Rules)
	}
}

func TestGenerateMVPUsesConfiguredFallbackOutbound(t *testing.T) {
	input := fixtureInput(t)
	input.DefaultOutbound = "a"
	generated, err := GenerateMVP(input)
	if err != nil {
		t.Fatal(err)
	}
	if generated.Model.Route.Final != "domestic-direct" {
		t.Fatalf("route.final = %q", generated.Model.Route.Final)
	}
}

func TestGenerateMVPIsDeterministic(t *testing.T) {
	input := fixtureInput(t)
	first, err := GenerateMVP(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := GenerateMVP(input)
	if err != nil {
		t.Fatal(err)
	}
	if string(first.JSON) != string(second.JSON) {
		t.Fatal("same input produced different JSON")
	}
}

func TestGenerateMVPConnectionObservationOmitsDeprecatedStoreMode(t *testing.T) {
	input := fixtureInput(t)
	input.ConnectionObservation = true
	input.ConnectionAPISecret = "test-secret"
	generated, err := GenerateMVP(input)
	if err != nil {
		t.Fatal(err)
	}
	if generated.Model.Experimental == nil || generated.Model.Experimental.ClashAPI == nil {
		t.Fatalf("clash api configuration = %#v", generated.Model.Experimental)
	}
	var decoded map[string]any
	if err := json.Unmarshal(generated.JSON, &decoded); err != nil {
		t.Fatal(err)
	}
	clashAPI := decoded["experimental"].(map[string]any)["clash_api"].(map[string]any)
	if _, exists := clashAPI["store_mode"]; exists {
		t.Fatal("deprecated store_mode must not be generated")
	}
}

func TestGenerateMVPSplitIPv6OmitsBlockRules(t *testing.T) {
	input := fixtureInput(t)
	input.IPv6 = IPv6Split
	input.DirectPrefixes = append(input.DirectPrefixes, MVPDirectPrefix{Prefix: "2001:db8:1::/64", BindInterface: input.InterfaceA.BindInterface})
	generated, err := GenerateMVP(input)
	if err != nil {
		t.Fatal(err)
	}
	if generated.Model.DNS.Strategy != "prefer_ipv4" {
		t.Fatalf("DNS strategy = %q", generated.Model.DNS.Strategy)
	}
	for _, rule := range generated.Model.DNS.Rules {
		if len(rule.QueryType) == 1 && rule.QueryType[0] == "AAAA" && rule.Action == "reject" {
			t.Fatalf("unexpected AAAA reject rule: %#v", rule)
		}
	}
	for _, rule := range generated.Model.Route.Rules {
		if rule.IPVersion == 6 && rule.Action == "reject" {
			t.Fatalf("unexpected IPv6 reject rule: %#v", rule)
		}
	}
}

func TestGenerateMVPCustomRulesMapActionsAndPreserveOrder(t *testing.T) {
	input := fixtureInput(t)
	input.CustomRules = []MVPCustomRule{
		{Name: "block", Type: "domain", Value: "ads.example", Action: "reject"},
		{Name: "via A", Type: "ip", Value: "8.8.8.8/32", Action: "a"},
		{Name: "mode final", Type: "domain", Value: "final.example", Action: "final"},
	}
	generated, err := GenerateMVP(input)
	if err != nil {
		t.Fatal(err)
	}
	indexes := []int{}
	for index, category := range generated.RuleCategories {
		if category == "user" {
			indexes = append(indexes, index)
		}
	}
	if len(indexes) != 3 {
		t.Fatalf("user categories = %#v", generated.RuleCategories)
	}
	rules := generated.Model.Route.Rules
	if rules[indexes[0]].Action != "reject" || rules[indexes[1]].Outbound != "domestic-direct" || rules[indexes[2]].Outbound != "foreign-direct" {
		t.Fatalf("custom rules = %#v", rules[indexes[0]:indexes[2]+1])
	}
}

func TestGenerateMVPPreservesEveryValueInAGroupedRule(t *testing.T) {
	input := fixtureInput(t)
	input.CustomRules = []MVPCustomRule{
		{ID: "domains", Name: "Domains", Type: "domain-suffix", Value: "one.example", Action: "b"},
		{ID: "domains", Name: "Domains", Type: "domain-suffix", Value: "two.example", Action: "b"},
	}
	input.RuleOrder = []string{"domains"}
	generated, err := GenerateMVP(input)
	if err != nil {
		t.Fatal(err)
	}
	values := make([]string, 0, 2)
	for index, category := range generated.RuleCategories {
		if category == "user" {
			values = append(values, generated.Model.Route.Rules[index].DomainSuffix...)
		}
	}
	if len(values) != 2 || values[0] != "one.example" || values[1] != "two.example" {
		t.Fatalf("grouped values = %#v", values)
	}
}

func TestPreviewMVPRulesIncludesGeneratedOrderAndFinal(t *testing.T) {
	input := fixtureInput(t)
	input.CustomRules = []MVPCustomRule{{Name: "block", Type: "domain", Value: "ads.example", Action: "reject"}}
	preview, err := PreviewMVPRules(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview) < 2 || preview[len(preview)-1].Category != "final" || preview[len(preview)-1].Action != "route -> foreign-direct" {
		t.Fatalf("preview final = %#v", preview)
	}
	found := false
	for _, item := range preview {
		if item.Category == "user" && item.Match[0] == "ads.example" && item.Action == "reject" {
			found = true
		}
	}
	if !found {
		t.Fatalf("custom rule missing from preview: %#v", preview)
	}
}

func TestGenerateMVPPreservesUserRuleOrder(t *testing.T) {
	input := fixtureInput(t)
	input.CustomRules = []MVPCustomRule{
		{Name: "site", Type: "domain-suffix", Value: "example.com", Action: "reject"},
		{Name: "browser", Type: "process-name", Value: "browser.exe", Action: "b"},
		{Name: "specific browser", Type: "process-path", Value: `C:\Tools\browser.exe`, Action: "a"},
	}
	generated, err := GenerateMVP(input)
	if err != nil {
		t.Fatal(err)
	}
	users := []RouteRule{}
	for index, category := range generated.RuleCategories {
		if category == "user" {
			users = append(users, generated.Model.Route.Rules[index])
		}
	}
	if len(users) != 3 || users[0].DomainSuffix[0] != "example.com" || users[1].ProcessName[0] != "browser.exe" || users[2].ProcessPath[0] != `C:\Tools\browser.exe` {
		t.Fatalf("generated process order = %#v", users)
	}
}

func TestGenerateMVPAllowsUnifiedInlineAndRuleSetOrder(t *testing.T) {
	input := fixtureInput(t)
	input.CustomRules = []MVPCustomRule{
		{ID: "first", Name: "First", Type: "domain", Value: "first.example", Action: "a"},
		{ID: "last", Name: "Last", Type: "domain", Value: "last.example", Action: "b"},
	}
	input.RuleSets = []MVPRuleSet{{Tag: "winrouter-geosite", Kind: "domain", Action: "reject", Path: `C:\rules\geosite.srs`}}
	input.RuleOrder = []string{"first", "srs:geosite", "last"}
	generated, err := GenerateMVP(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateMVPSemantics(generated); err != nil {
		t.Fatalf("unified order rejected: %v; categories = %#v", err, generated.RuleCategories)
	}
	want := []string{"user", "rule-set", "user"}
	got := make([]string, 0, 3)
	for _, category := range generated.RuleCategories {
		if category == "user" || category == "rule-set" {
			got = append(got, category)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unified categories = %#v, want %#v", got, want)
	}
}

func TestGenerateMVPPlacesRuleSetBeforeBuiltInPolicyWithoutInlineRules(t *testing.T) {
	input := fixtureInput(t)
	input.RuleSets = []MVPRuleSet{{Tag: "winrouter-geoip", Kind: "ip", Action: "a", Path: `C:\rules\geoip.srs`}}
	generated, err := GenerateMVP(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateMVPSemantics(generated); err != nil {
		t.Fatalf("rule-set-only order rejected: %v; categories = %#v", err, generated.RuleCategories)
	}
	ruleSetIndex, directIndex := -1, -1
	for index, category := range generated.RuleCategories {
		if category == "rule-set" {
			ruleSetIndex = index
		}
		if directIndex < 0 && category == "direct-prefix" {
			directIndex = index
		}
	}
	if ruleSetIndex < 0 || directIndex < 0 || ruleSetIndex >= directIndex {
		t.Fatalf("rule-set is outside override region: %#v", generated.RuleCategories)
	}
}

func TestDecodeMVPConfigEnforcesSchema(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "tests", "fixtures", "config", "direct-split.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeMVPConfig(data); err != nil {
		t.Fatalf("fixture rejected: %v", err)
	}
	unknown := append([]byte(nil), data[:len(data)-2]...)
	unknown = append(unknown, []byte(",\n  \"unknown\": true\n}\n")...)
	if _, err := DecodeMVPConfig(unknown); err == nil {
		t.Fatal("unknown field accepted")
	}
	if _, err := DecodeMVPConfig(append(data, []byte(` {}`)...)); err == nil {
		t.Fatal("trailing JSON accepted")
	}
}

func TestValidateMVPModelRejectsErrorsAndUnopenedModes(t *testing.T) {
	base := fixtureInput(t)
	tests := []func(*MVPConfig){
		func(value *MVPConfig) { value.SchemaVersion = 2 },
		func(value *MVPConfig) { value.Mode = "hybrid" },
		func(value *MVPConfig) { value.InterfaceB.GUID = value.InterfaceA.GUID },
		func(value *MVPConfig) { value.TUN.Prefix = "172.19.0.1/30" },
		func(value *MVPConfig) { value.TUN.Stack = "bad" },
		func(value *MVPConfig) { value.IPv6 = "allow" },
		func(value *MVPConfig) { value.DNS.Global.Server = "dns.example" },
		func(value *MVPConfig) { value.Domestic.CIDRs = []string{"192.168.0.0/16"} },
		func(value *MVPConfig) { value.DirectPrefixes[0].BindInterface = "" },
		func(value *MVPConfig) { value.DirectPrefixes[0].Prefix = "fe80::/64" },
	}
	for index, mutate := range tests {
		value := base
		mutate(&value)
		if err := ValidateMVPModel(value); err == nil {
			t.Fatalf("case %d succeeded", index)
		}
	}
}

func TestGenerateProxySplitUsesStrictProxyFinalBoundToInterfaceB(t *testing.T) {
	input := fixtureInput(t)
	input.Mode = ModeProxySplit
	input.Proxy = &MVPProxy{Type: "http", Server: "203.0.113.10", Port: 8080}
	generated, err := GenerateMVP(input)
	if err != nil {
		t.Fatal(err)
	}
	if generated.Model.Route.Final != "proxy" {
		t.Fatalf("route.final = %q", generated.Model.Route.Final)
	}
	foundEndpointRule := false
	for _, rule := range generated.Model.Route.Rules {
		if rule.Outbound == "foreign-direct" && reflect.DeepEqual(rule.IPCIDR, []string{"203.0.113.10/32"}) {
			foundEndpointRule = true
		}
	}
	if !foundEndpointRule {
		t.Fatal("proxy endpoint loop-prevention rule missing")
	}
	for _, outbound := range generated.Model.Outbounds {
		if outbound.Tag == "proxy" {
			if outbound.Type != "http" || outbound.Server != input.Proxy.Server || outbound.ServerPort != input.Proxy.Port || outbound.BindInterface != input.InterfaceB.BindInterface {
				t.Fatalf("proxy outbound = %#v", outbound)
			}
			return
		}
	}
	t.Fatal("proxy outbound missing")
}

func TestGenerateProxySplitIncludesHTTPAuthentication(t *testing.T) {
	input := fixtureInput(t)
	input.Mode = ModeProxySplit
	input.Proxy = &MVPProxy{Type: "http", Server: "203.0.113.10", Port: 8080, Username: "alice", Password: "secret"}
	generated, err := GenerateMVP(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, outbound := range generated.Model.Outbounds {
		if outbound.Tag == "proxy" {
			if outbound.Username != "alice" || outbound.Password != "secret" {
				t.Fatalf("proxy authentication = %#v", outbound)
			}
			return
		}
	}
	t.Fatal("proxy outbound missing")
}

func TestGenerateProxySplitIncludesShadowsocksMethod(t *testing.T) {
	input := fixtureInput(t)
	input.Mode = ModeProxySplit
	input.Proxy = &MVPProxy{Type: "shadowsocks", Server: "37.19.198.244", Port: 443, Method: "aes-128-gcm", Password: "shadowsocks"}
	generated, err := GenerateMVP(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, outbound := range generated.Model.Outbounds {
		if outbound.Tag == "proxy" {
			if outbound.Type != "shadowsocks" || outbound.Method != "aes-128-gcm" || outbound.Password != "shadowsocks" || outbound.BindInterface != input.InterfaceB.BindInterface {
				t.Fatalf("Shadowsocks outbound = %#v", outbound)
			}
			return
		}
	}
	t.Fatal("Shadowsocks outbound missing")
}

func TestValidateProxySplitRejectsMissingInvalidProxyAndHybrid(t *testing.T) {
	base := fixtureInput(t)
	base.Mode = ModeProxySplit
	tests := []func(*MVPConfig){
		func(value *MVPConfig) {},
		func(value *MVPConfig) { value.Proxy = &MVPProxy{Type: "socks", Server: "203.0.113.10", Port: 1080} },
		func(value *MVPConfig) { value.Proxy = &MVPProxy{Type: "http", Server: "proxy.example", Port: 8080} },
		func(value *MVPConfig) { value.Proxy = &MVPProxy{Type: "http", Server: "203.0.113.10"} },
		func(value *MVPConfig) {
			value.Proxy = &MVPProxy{Type: "http", Server: "203.0.113.10", Port: 8080, Password: "secret"}
		},
	}
	for index, mutate := range tests {
		value := base
		mutate(&value)
		if err := ValidateMVPModel(value); err == nil {
			t.Fatalf("invalid proxy case %d accepted", index)
		}
	}
	base.Mode = "hybrid"
	if err := ValidateMVPModel(base); err == nil {
		t.Fatal("hybrid accepted")
	}
}

func TestValidationLayersRejectTamperedOutput(t *testing.T) {
	generated, err := GenerateMVP(fixtureInput(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateGeneratedSchema([]byte(`{"route":`)); err == nil {
		t.Fatal("malformed JSON accepted")
	}
	generated.Model.Route.Final = "missing"
	err = ValidateMVPSemantics(generated)
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) || validationErr.Stage != StageSemantic {
		t.Fatalf("semantic error = %#v", err)
	}
	generated, err = GenerateMVP(fixtureInput(t))
	if err != nil {
		t.Fatal(err)
	}
	generated.RuleCategories[1], generated.RuleCategories[len(generated.RuleCategories)-1] = generated.RuleCategories[len(generated.RuleCategories)-1], generated.RuleCategories[1]
	if err := ValidateMVPSemantics(generated); err == nil {
		t.Fatal("reordered policy categories accepted")
	}
}

func TestProxySemanticsRejectMissingLoopPreventionRule(t *testing.T) {
	input := fixtureInput(t)
	input.Mode = ModeProxySplit
	input.Proxy = &MVPProxy{Type: "http", Server: "203.0.113.10", Port: 8080}
	generated, err := GenerateMVP(input)
	if err != nil {
		t.Fatal(err)
	}
	filtered := generated.Model.Route.Rules[:0]
	filteredCategories := generated.RuleCategories[:0]
	for index, rule := range generated.Model.Route.Rules {
		if rule.Outbound == "foreign-direct" && reflect.DeepEqual(rule.IPCIDR, []string{"203.0.113.10/32"}) {
			continue
		}
		filtered = append(filtered, rule)
		filteredCategories = append(filteredCategories, generated.RuleCategories[index])
	}
	generated.Model.Route.Rules = filtered
	generated.RuleCategories = filteredCategories
	if err := ValidateMVPSemantics(generated); err == nil {
		t.Fatal("missing proxy loop-prevention rule accepted")
	}
}

func fixtureInput(t *testing.T) MVPConfig {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "tests", "fixtures", "config", "direct-split.json"))
	if err != nil {
		t.Fatal(err)
	}
	input, err := DecodeMVPConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	return input
}
