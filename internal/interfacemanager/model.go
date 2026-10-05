package interfacemanager

import (
	"errors"

	"winrouter/internal/interfaces"
	"winrouter/internal/routes"
	"winrouter/internal/tunprefix"
)

const (
	StateSchemaVersion        = 3
	legacyStateSchemaVersion2 = 2
	legacyStateSchemaVersion  = 1
	IPv6PolicyBlock           = "block"
	IPv6PolicySplit           = "split"

	ModeSingle = "single"
	ModeDual   = "dual"
)

var (
	ErrSameAdapter      = errors.New("interface A and B must be different")
	ErrSelectionInvalid = errors.New("saved interface selection is invalid")
)

type State struct {
	SchemaVersion int                 `json:"schema_version"`
	Mode          string              `json:"mode,omitempty"`
	InterfaceA    interfaces.Identity `json:"interface_a"`
	InterfaceB    interfaces.Identity `json:"interface_b"`
	TUNPrefix     string              `json:"tun_prefix,omitempty"`
	IPv6TUNPrefix string              `json:"ipv6_tun_prefix,omitempty"`
	IPv6Policy    string              `json:"ipv6_policy"`
}

type Candidate struct {
	Adapter             interfaces.Adapter `json:"adapter"`
	Eligible            bool               `json:"eligible"`
	RecommendationScore int                `json:"recommendation_score"`
	Reasons             []string           `json:"reasons"`
}

type ResolvedSelection struct {
	Role   string              `json:"role"`
	Saved  interfaces.Identity `json:"saved"`
	Match  *interfaces.Match   `json:"match,omitempty"`
	Status string              `json:"status"`
	Error  string              `json:"error,omitempty"`
}

type Snapshot struct {
	Sequence      uint64                `json:"sequence"`
	Mode          string                `json:"mode"`
	Candidates    []Candidate           `json:"candidates"`
	Adapters      []interfaces.Adapter  `json:"adapters"`
	Routes        []routes.Route        `json:"routes"`
	Topology      interfaces.Topology   `json:"topology"`
	InterfaceA    ResolvedSelection     `json:"interface_a"`
	InterfaceB    ResolvedSelection     `json:"interface_b"`
	TUN           *tunprefix.Allocation `json:"tun,omitempty"`
	IPv6TUNPrefix string                `json:"ipv6_tun_prefix,omitempty"`
	IPv6Policy    string                `json:"ipv6_policy,omitempty"`
	Diagnostics   []Diagnostic          `json:"diagnostics"`
}

type Diagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type Event struct {
	Reason   string   `json:"reason"`
	Snapshot Snapshot `json:"snapshot"`
}
