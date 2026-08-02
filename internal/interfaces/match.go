package interfaces

import (
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"strings"
)

var (
	ErrAdapterNotFound = errors.New("saved adapter was not found")
	ErrAmbiguousMatch  = errors.New("saved adapter matches multiple candidates")
	ErrSelectionNeeded = errors.New("adapter selection requires confirmation")
)

const minimumAutomaticMatchScore = 4

type Identity struct {
	GUID         string   `json:"guid"`
	MAC          string   `json:"mac,omitempty"`
	IPv4Prefixes []string `json:"ipv4_prefixes,omitempty"`
	FriendlyName string   `json:"friendly_name,omitempty"`
}

type MatchMethod string

const (
	MatchByGUID      MatchMethod = "guid"
	MatchByAuxiliary MatchMethod = "auxiliary"
)

type Match struct {
	Adapter Adapter     `json:"adapter"`
	Method  MatchMethod `json:"method"`
	Score   int         `json:"score"`
	Reasons []string    `json:"reasons"`
}

type MatchError struct {
	Kind       error   `json:"-"`
	Candidates []Match `json:"candidates"`
}

func (e *MatchError) Error() string {
	return fmt.Sprintf("%v (%d candidate(s))", e.Kind, len(e.Candidates))
}

func (e *MatchError) Unwrap() error { return e.Kind }

func IdentityFromAdapter(adapter Adapter) Identity {
	return Identity{
		GUID:         normalizeGUID(adapter.GUID),
		MAC:          normalizeMAC(adapter.MAC),
		IPv4Prefixes: ipv4Prefixes(adapter),
		FriendlyName: adapter.FriendlyName,
	}
}

// Resolve finds a physical adapter using its stable GUID first. Auxiliary
// evidence is considered only when the GUID is gone and must identify one
// strictly best candidate.
func Resolve(identity Identity, adapters []Adapter) (Match, error) {
	candidates := physicalCandidates(adapters)
	wantedGUID := normalizeGUID(identity.GUID)
	if wantedGUID != "" {
		guidMatches := make([]Match, 0, 1)
		for _, adapter := range candidates {
			if strings.EqualFold(normalizeGUID(adapter.GUID), wantedGUID) {
				guidMatches = append(guidMatches, Match{Adapter: adapter, Method: MatchByGUID, Reasons: []string{"guid"}})
			}
		}
		if len(guidMatches) == 1 {
			return guidMatches[0], nil
		}
		if len(guidMatches) > 1 {
			return Match{}, &MatchError{Kind: ErrAmbiguousMatch, Candidates: guidMatches}
		}
	}

	ranked := rankAuxiliary(identity, candidates)
	if len(ranked) == 0 {
		return Match{}, &MatchError{Kind: ErrAdapterNotFound}
	}
	if ranked[0].Score < minimumAutomaticMatchScore {
		return Match{}, &MatchError{Kind: ErrSelectionNeeded, Candidates: ranked}
	}
	if len(ranked) > 1 && ranked[0].Score == ranked[1].Score {
		topScore := ranked[0].Score
		end := 1
		for end < len(ranked) && ranked[end].Score == topScore {
			end++
		}
		return Match{}, &MatchError{Kind: ErrAmbiguousMatch, Candidates: ranked[:end]}
	}
	return ranked[0], nil
}

func physicalCandidates(adapters []Adapter) []Adapter {
	result := make([]Adapter, 0, len(adapters))
	for _, adapter := range adapters {
		if adapter.Candidate && isCandidate(adapter.Kind) {
			result = append(result, adapter)
		}
	}
	return result
}

func rankAuxiliary(identity Identity, adapters []Adapter) []Match {
	wantedMAC := normalizeMAC(identity.MAC)
	wantedPrefixes := make(map[string]struct{}, len(identity.IPv4Prefixes))
	for _, prefix := range identity.IPv4Prefixes {
		if parsed, err := netip.ParsePrefix(prefix); err == nil && parsed.Addr().Is4() {
			wantedPrefixes[parsed.Masked().String()] = struct{}{}
		}
	}

	result := make([]Match, 0, len(adapters))
	for _, adapter := range adapters {
		match := Match{Adapter: adapter, Method: MatchByAuxiliary, Reasons: make([]string, 0, 3)}
		if wantedMAC != "" && normalizeMAC(adapter.MAC) == wantedMAC {
			match.Score += 4
			match.Reasons = append(match.Reasons, "mac")
		}
		for _, prefix := range ipv4Prefixes(adapter) {
			if _, ok := wantedPrefixes[prefix]; ok {
				match.Score += 2
				match.Reasons = append(match.Reasons, "ipv4-prefix:"+prefix)
			}
		}
		if identity.FriendlyName != "" && strings.EqualFold(strings.TrimSpace(identity.FriendlyName), strings.TrimSpace(adapter.FriendlyName)) {
			match.Score++
			match.Reasons = append(match.Reasons, "friendly-name")
		}
		if match.Score > 0 {
			result = append(result, match)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Score != result[j].Score {
			return result[i].Score > result[j].Score
		}
		return strings.ToLower(result[i].Adapter.FriendlyName) < strings.ToLower(result[j].Adapter.FriendlyName)
	})
	return result
}

func ipv4Prefixes(adapter Adapter) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, address := range adapter.Addresses {
		ip, err := netip.ParseAddr(address.IP)
		if err != nil || !ip.Is4() || address.PrefixLength > 32 {
			continue
		}
		prefix := netip.PrefixFrom(ip, int(address.PrefixLength)).Masked().String()
		if _, ok := seen[prefix]; !ok {
			seen[prefix] = struct{}{}
			result = append(result, prefix)
		}
	}
	sort.Strings(result)
	return result
}

func normalizeMAC(value string) string {
	return strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(value), "-", ":"), ".", ""))
}
