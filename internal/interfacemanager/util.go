package interfacemanager

import (
	"crypto/sha256"
	"encoding/json"
	"strings"
)

func equalGUID(first, second string) bool {
	normalize := func(value string) string { return strings.ToLower(strings.Trim(strings.TrimSpace(value), "{}")) }
	return normalize(first) != "" && normalize(first) == normalize(second)
}

func fingerprint(snapshot Snapshot) [sha256.Size]byte {
	value := struct {
		Candidates  []Candidate
		Adapters    any
		Routes      any
		Topology    any
		InterfaceA  ResolvedSelection
		InterfaceB  ResolvedSelection
		TUNPrefix   string
		Diagnostics []Diagnostic
	}{
		Candidates: snapshot.Candidates, Adapters: snapshot.Adapters, Routes: snapshot.Routes,
		Topology: snapshot.Topology, InterfaceA: snapshot.InterfaceA, InterfaceB: snapshot.InterfaceB,
		Diagnostics: snapshot.Diagnostics,
	}
	if snapshot.TUN != nil {
		value.TUNPrefix = snapshot.TUN.Prefix
	}
	data, _ := json.Marshal(value)
	return sha256.Sum256(data)
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return Snapshot{}
	}
	var clone Snapshot
	if err := json.Unmarshal(data, &clone); err != nil {
		return Snapshot{}
	}
	return clone
}
