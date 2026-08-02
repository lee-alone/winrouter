package interfaces

import (
	"errors"
	"testing"
)

func TestResolvePrefersGUID(t *testing.T) {
	wanted := testAdapter("{AAAAAAAA-0000-0000-0000-000000000000}", "00:11:22:33:44:55", "Ethernet A", "192.168.1.2", 24)
	other := testAdapter("{BBBBBBBB-0000-0000-0000-000000000000}", "00:11:22:33:44:55", "Ethernet A", "192.168.1.3", 24)

	match, err := Resolve(Identity{GUID: "aaaaaaaa-0000-0000-0000-000000000000", MAC: other.MAC}, []Adapter{other, wanted})
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if match.Method != MatchByGUID || match.Adapter.GUID != wanted.GUID {
		t.Fatalf("Resolve() = %#v, want GUID match", match)
	}
}

func TestResolveUsesUniqueAuxiliaryEvidence(t *testing.T) {
	wanted := testAdapter("{NEW-GUID}", "00:11:22:33:44:55", "Renamed", "192.168.1.9", 24)
	other := testAdapter("{OTHER-GUID}", "00:11:22:33:44:66", "Other", "10.0.0.2", 24)
	identity := Identity{GUID: "{OLD-GUID}", MAC: "00-11-22-33-44-55", IPv4Prefixes: []string{"192.168.1.42/24"}, FriendlyName: "Old name"}

	match, err := Resolve(identity, []Adapter{other, wanted})
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if match.Method != MatchByAuxiliary || match.Adapter.GUID != wanted.GUID || match.Score != 6 {
		t.Fatalf("Resolve() = %#v, want unique auxiliary match with score 6", match)
	}
}

func TestResolveRejectsAmbiguousAuxiliaryMatch(t *testing.T) {
	first := testAdapter("{FIRST}", "00:11:22:33:44:55", "Ethernet", "192.168.1.2", 24)
	second := testAdapter("{SECOND}", "00:11:22:33:44:55", "Ethernet", "192.168.1.3", 24)

	_, err := Resolve(Identity{GUID: "{GONE}", MAC: first.MAC}, []Adapter{first, second})
	if !errors.Is(err, ErrAmbiguousMatch) {
		t.Fatalf("Resolve() error = %v, want ErrAmbiguousMatch", err)
	}
	var matchErr *MatchError
	if !errors.As(err, &matchErr) || len(matchErr.Candidates) != 2 {
		t.Fatalf("Resolve() error = %#v, want two candidates", err)
	}
}

func TestResolveRejectsMissingAndVirtualAdapters(t *testing.T) {
	virtual := testAdapter("{VIRTUAL}", "00:11:22:33:44:55", "vEthernet", "192.168.1.2", 24)
	virtual.Kind = KindHyperV
	virtual.Candidate = false

	_, err := Resolve(Identity{GUID: virtual.GUID, MAC: virtual.MAC}, []Adapter{virtual})
	if !errors.Is(err, ErrAdapterNotFound) {
		t.Fatalf("Resolve() error = %v, want ErrAdapterNotFound", err)
	}
}

func TestResolveRequiresConfirmationForNameOnlyMatch(t *testing.T) {
	adapter := testAdapter("{NEW}", "00:11:22:33:44:55", "Ethernet", "192.168.1.2", 24)

	_, err := Resolve(Identity{GUID: "{GONE}", FriendlyName: "ethernet"}, []Adapter{adapter})
	if !errors.Is(err, ErrSelectionNeeded) {
		t.Fatalf("Resolve() error = %v, want ErrSelectionNeeded", err)
	}
	var matchErr *MatchError
	if !errors.As(err, &matchErr) || len(matchErr.Candidates) != 1 {
		t.Fatalf("Resolve() error = %#v, want suggested candidate", err)
	}
}

func TestIdentityFromAdapterMasksIPv4Prefixes(t *testing.T) {
	adapter := testAdapter("{GUID}", "AA-BB-CC-DD-EE-FF", "Ethernet", "192.168.10.23", 24)
	adapter.Addresses = append(adapter.Addresses, Address{IP: "fe80::1", PrefixLength: 64}, Address{IP: "bad", PrefixLength: 24})

	identity := IdentityFromAdapter(adapter)
	if identity.MAC != "aa:bb:cc:dd:ee:ff" || len(identity.IPv4Prefixes) != 1 || identity.IPv4Prefixes[0] != "192.168.10.0/24" {
		t.Fatalf("IdentityFromAdapter() = %#v", identity)
	}
}

func testAdapter(guid, mac, name, ip string, bits uint8) Adapter {
	return Adapter{
		GUID: guid, MAC: mac, FriendlyName: name, Kind: KindEthernet, Candidate: true,
		Addresses: []Address{{IP: ip, PrefixLength: bits}},
	}
}
