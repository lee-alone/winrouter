package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"strings"

	"winrouter/internal/core"
)

type ValidationStage string

const (
	StageModel    ValidationStage = "model"
	StageSchema   ValidationStage = "schema"
	StageSemantic ValidationStage = "semantic"
	StageCore     ValidationStage = "sing-box"
)

type ValidationError struct {
	Stage ValidationStage
	Err   error
}

func (e *ValidationError) Error() string { return fmt.Sprintf("%s validation: %v", e.Stage, e.Err) }
func (e *ValidationError) Unwrap() error { return e.Err }

type MVPValidation struct {
	ModelValid    bool            `json:"model_valid"`
	SchemaValid   bool            `json:"schema_valid"`
	SemanticValid bool            `json:"semantic_valid"`
	Core          core.Validation `json:"core"`
}

func ValidateGeneratedSchema(data []byte) error {
	if !json.Valid(data) {
		return &ValidationError{Stage: StageSchema, Err: errors.New("generated output is not valid JSON")}
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var model MinimalTUN
	if err := decoder.Decode(&model); err != nil {
		return &ValidationError{Stage: StageSchema, Err: err}
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return &ValidationError{Stage: StageSchema, Err: errors.New("generated output has trailing JSON data")}
	}
	if len(model.Inbounds) != 1 || len(model.Outbounds) < 3 || len(model.DNS.Servers) < 2 || len(model.Route.Rules) == 0 || model.Route.Final == "" {
		return &ValidationError{Stage: StageSchema, Err: errors.New("generated output is missing required MVP fields")}
	}
	return nil
}

func ValidateMVPSemantics(generated Generated) error {
	model := generated.Model
	if model.Route.Final != "domestic-direct" && model.Route.Final != "foreign-direct" && model.Route.Final != "proxy" {
		return semanticError("route.final must be domestic-direct, foreign-direct, or proxy")
	}
	tags := make(map[string]struct{}, len(model.Outbounds))
	for _, outbound := range model.Outbounds {
		if outbound.Tag == "" {
			return semanticError("outbound tag is empty")
		}
		if _, exists := tags[outbound.Tag]; exists {
			return semanticError("duplicate outbound tag %q", outbound.Tag)
		}
		tags[outbound.Tag] = struct{}{}
	}
	if _, ok := tags[model.Route.Final]; !ok {
		return semanticError("route.final references missing outbound %q", model.Route.Final)
	}
	var proxy *Outbound
	for index := range model.Outbounds {
		if model.Outbounds[index].Tag == "proxy" {
			proxy = &model.Outbounds[index]
		}
	}
	if proxy != nil {
		if proxy.BindInterface != generated.ProxyBindInterface || proxy.Server == "" || proxy.ServerPort == 0 {
			return semanticError("proxy outbound must be complete and correctly bound")
		}
		switch proxy.Type {
		case "http":
		case "shadowsocks":
			if proxy.Method == "" || proxy.Password == "" {
				return semanticError("Shadowsocks proxy outbound requires method and password")
			}
		case "vmess":
			if proxy.UUID == "" {
				return semanticError("VMess proxy outbound requires uuid")
			}
		case "vless":
			if proxy.UUID == "" {
				return semanticError("VLESS proxy outbound requires uuid")
			}
		case "trojan":
			if proxy.Password == "" {
				return semanticError("Trojan proxy outbound requires password")
			}
			if proxy.TLS == nil || !proxy.TLS.Enabled {
				return semanticError("Trojan proxy outbound requires TLS")
			}
		default:
			return semanticError("unsupported proxy outbound type %q", proxy.Type)
		}

		expectedDirectOutbound := "foreign-direct"
		for _, direct := range model.Outbounds {
			if direct.Tag == "domestic-direct" && strings.EqualFold(direct.BindInterface, proxy.BindInterface) {
				expectedDirectOutbound = "domestic-direct"
			}
		}

		proxyAddr, err := netip.ParseAddr(proxy.Server)
		if err != nil {
			return semanticError("invalid proxy server address %q", proxy.Server)
		}
		endpoint := netip.PrefixFrom(proxyAddr, proxyAddr.BitLen()).String()
		protected := false
		for _, rule := range model.Route.Rules {
			if rule.Outbound != expectedDirectOutbound {
				continue
			}
			for _, prefix := range rule.IPCIDR {
				if prefix == endpoint {
					protected = true
				}
			}
		}
		if !protected {
			return semanticError("proxy endpoint must have a %s loop-prevention rule", expectedDirectOutbound)
		}
	}
	expectedDNSFinal := "dns-global"
	if generated.Model.Route.Final == "domestic-direct" {
		expectedDNSFinal = "dns-domestic"
	} else if generated.Model.Route.Final == "proxy" {
		expectedDNSFinal = "dns-proxy"
	}
	if !model.DNS.IndependentCache || (model.DNS.Strategy != "ipv4_only" && model.DNS.Strategy != "prefer_ipv4") || model.DNS.Final != expectedDNSFinal {
		return semanticError("DNS cache isolation, strategy, or final server is invalid")
	}
	dnsTags := make(map[string]struct{}, len(model.DNS.Servers))
	for _, server := range model.DNS.Servers {
		dnsTags[server.Tag] = struct{}{}
		if _, ok := tags[server.Detour]; !ok {
			return semanticError("DNS server %q references missing detour %q", server.Tag, server.Detour)
		}
	}
	if _, ok := dnsTags[model.DNS.Final]; !ok {
		return semanticError("DNS final references missing server")
	}
	if model.Route.DefaultDNSResolver != model.DNS.Final {
		return semanticError("default_domain_resolver must equal DNS final")
	}
	if len(generated.RuleCategories) != len(model.Route.Rules) {
		return semanticError("policy and generated route rule counts differ")
	}
	// Inline user rules and SRS-backed rules share one ordered override region.
	ranks := map[string]int{"metadata": 0, "local": 1, "infrastructure": 2, "user": 3, "rule-set": 3, "direct-prefix": 4, "reserved": 5, "domestic": 6}
	previous := -1
	ipv6Rejected := false
	for index, rule := range model.Route.Rules {
		rank, ok := ranks[generated.RuleCategories[index]]
		if !ok || rank < previous {
			return semanticError("route rule %d is out of policy order", index)
		}
		previous = rank
		if rule.Outbound != "" {
			if _, ok := tags[rule.Outbound]; !ok {
				return semanticError("route rule %d references missing outbound %q", index, rule.Outbound)
			}
		}
		if rule.IPVersion == 6 && rule.Action == "reject" {
			ipv6Rejected = true
		}
	}
	if model.DNS.Strategy != "ipv4_only" && model.DNS.Strategy != "prefer_ipv4" {
		return semanticError("unsupported DNS strategy %q", model.DNS.Strategy)
	}
	if model.DNS.Strategy == "ipv4_only" {
		if !ipv6Rejected {
			return semanticError("IPv6 reject rule is missing")
		}
		if len(model.DNS.Rules) == 0 || len(model.DNS.Rules[0].QueryType) != 1 || model.DNS.Rules[0].QueryType[0] != "AAAA" || model.DNS.Rules[0].Action != "reject" {
			return semanticError("AAAA reject rule must be first")
		}
	} else if ipv6Rejected {
		return semanticError("IPv6 split must not include an IPv6 reject rule")
	}
	ruleSetTags := make(map[string]struct{}, len(model.Route.RuleSets))
	for _, ruleSet := range model.Route.RuleSets {
		if ruleSet.Type != "local" || ruleSet.Format != "binary" || ruleSet.Tag == "" || ruleSet.Path == "" {
			return semanticError("invalid local binary rule-set %q", ruleSet.Tag)
		}
		ruleSetTags[ruleSet.Tag] = struct{}{}
	}
	for index, rule := range model.Route.Rules {
		for _, tag := range rule.RuleSet {
			if _, ok := ruleSetTags[tag]; !ok {
				return semanticError("route rule %d references missing rule-set %q", index, tag)
			}
		}
	}
	return nil
}

func ValidateMVPWithCore(ctx context.Context, input MVPConfig, executable string) (Generated, MVPValidation, error) {
	if err := ValidateMVPModel(input); err != nil {
		return Generated{}, MVPValidation{}, &ValidationError{Stage: StageModel, Err: err}
	}
	report := MVPValidation{ModelValid: true}
	generated, err := GenerateMVP(input)
	if err != nil {
		var validationErr *ValidationError
		if errors.As(err, &validationErr) {
			return Generated{}, report, err
		}
		return Generated{}, report, &ValidationError{Stage: StageSemantic, Err: err}
	}
	report.SchemaValid = true
	report.SemanticValid = true
	coreResult, err := core.Validate(ctx, executable, core.LockedVersion, core.LockedSHA256, generated.JSON)
	if err != nil {
		return Generated{}, report, &ValidationError{Stage: StageCore, Err: err}
	}
	report.Core = coreResult
	return generated, report, nil
}

func semanticError(format string, args ...any) error {
	return &ValidationError{Stage: StageSemantic, Err: fmt.Errorf(format, args...)}
}
