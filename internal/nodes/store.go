package nodes

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Protector interface {
	Protect([]byte) ([]byte, error)
	Unprotect([]byte) ([]byte, error)
}

type Store struct {
	mu        sync.RWMutex
	path      string
	protector Protector
	state     state
}

func New(path string, protector Protector) (*Store, error) {
	if strings.TrimSpace(path) == "" || protector == nil {
		return nil, errors.New("node store path and protector are required")
	}
	s := &Store{path: path, protector: protector, state: state{SchemaVersion: SchemaVersion, Nodes: []storedNode{}}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read node state: %w", err)
	}
	if err := json.Unmarshal(data, &s.state); err != nil {
		return nil, fmt.Errorf("decode node state: %w", err)
	}
	if s.state.SchemaVersion < SchemaVersion {
		for i := range s.state.Nodes {
			if s.state.Nodes[i].Egress == "" {
				s.state.Nodes[i].Egress = EgressB
			}
			if s.state.Nodes[i].Type == TypeShadowsocks && s.state.Nodes[i].Method == "" && s.state.Nodes[i].Username != "" {
				s.state.Nodes[i].Method = s.state.Nodes[i].Username
				s.state.Nodes[i].Username = ""
			}
			if s.state.Nodes[i].ProtectedSecret == "" && s.state.Nodes[i].LegacyProtected != "" {
				s.state.Nodes[i].ProtectedSecret = s.state.Nodes[i].LegacyProtected
				s.state.Nodes[i].LegacyProtected = ""
			}
		}
		s.state.SchemaVersion = SchemaVersion
		if err := validateState(s.state); err != nil {
			return nil, err
		}
		if err := s.save(s.state); err != nil {
			return nil, fmt.Errorf("migrate node state: %w", err)
		}
	}
	if err := validateState(s.state); err != nil {
		return nil, err
	}
	if len(s.state.Nodes) == 0 {
		s.state.Mode = ProxyModeSingle
		s.state.SelectedID = ""
		s.state.SelectedChain = []string{}
	} else {
		if s.state.Mode == "" {
			s.state.Mode = ProxyModeSingle
		}
		if len(s.state.SelectedChain) == 0 && s.state.SelectedID != "" {
			s.state.SelectedChain = []string{s.state.SelectedID}
		}
	}
	if s.state.SelectedChain == nil {
		s.state.SelectedChain = []string{}
	}
	return s, nil
}

func (s *Store) List() []Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Node, 0, len(s.state.Nodes))
	chainPos := make(map[string]int, len(s.state.SelectedChain))
	for idx, id := range s.state.SelectedChain {
		chainPos[id] = idx + 1
	}
	for _, item := range s.state.Nodes {
		pos := chainPos[item.ID]
		isSelected := (s.state.Mode == ProxyModeChain && pos > 0) || (s.state.Mode != ProxyModeChain && item.ID == s.state.SelectedID)
		n := publicNode(item, isSelected)
		n.ChainPosition = pos
		result = append(result, n)
	}
	return result
}

func (s *Store) Get(id string) (Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	index := indexByID(s.state.Nodes, strings.TrimSpace(id))
	if index < 0 {
		return Node{}, fmt.Errorf("node %q not found", id)
	}
	return publicNode(s.state.Nodes[index], s.state.Nodes[index].ID == s.state.SelectedID), nil
}

func (s *Store) save(next state) error {
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return fmt.Errorf("encode node state: %w", err)
	}
	directory := filepath.Dir(s.path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create node state directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".nodes-*.tmp")
	if err != nil {
		return fmt.Errorf("create node state temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err == nil {
		_, err = temporary.Write(append(data, '\n'))
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write node state: %w", err)
	}
	if err := replaceFile(temporaryPath, s.path); err != nil {
		return fmt.Errorf("replace node state: %w", err)
	}
	return nil
}

func validateState(value state) error {
	if value.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported node state schema %d", value.SchemaVersion)
	}
	seen := make(map[string]struct{}, len(value.Nodes))
	for _, item := range value.Nodes {
		if item.ID == "" {
			return errors.New("stored node id is empty")
		}
		if _, exists := seen[item.ID]; exists {
			return fmt.Errorf("duplicate stored node id %q", item.ID)
		}
		seen[item.ID] = struct{}{}
		if item.ResolvedIP != "" {
			address, err := netip.ParseAddr(item.ResolvedIP)
			if err != nil || (!usableIPv4(address) && !usableIPv6(address)) {
				return fmt.Errorf("stored node %q has invalid resolved IP", item.ID)
			}
		}
		if item.Egress != "" && item.Egress != EgressA && item.Egress != EgressB {
			return fmt.Errorf("stored node %q has invalid egress %q", item.ID, item.Egress)
		}
	}
	if value.SelectedID != "" {
		if _, exists := seen[value.SelectedID]; !exists {
			return errors.New("selected node does not exist")
		}
	}
	if value.Mode != "" && value.Mode != ProxyModeSingle && value.Mode != ProxyModeChain {
		return fmt.Errorf("unsupported proxy mode %q", value.Mode)
	}
	for _, chainID := range value.SelectedChain {
		if _, exists := seen[chainID]; !exists {
			return fmt.Errorf("proxy chain node %q does not exist", chainID)
		}
	}
	return nil
}

func publicNode(item storedNode, selected bool) Node {
	egress := item.Egress
	if egress == "" {
		egress = EgressB
	}
	username := item.Username
	if username == "" && item.Type == TypeShadowsocks {
		username = item.Method
	}
	return Node{
		ID:         item.ID,
		Name:       item.Name,
		Type:       item.Type,
		Server:     item.Server,
		ResolvedIP: item.ResolvedIP,
		Port:       item.Port,
		Egress:     egress,
		Authentication: AuthenticationNode{
			Username: item.Username,
			Method:   item.Method,
			Flow:     item.Flow,
		},
		TLS:              item.TLS,
		Transport:        item.Transport,
		HasSecret:        item.ProtectedSecret != "",
		HasPassword:      item.ProtectedSecret != "",
		Username:         username,
		Selected:         selected,
		SubscriptionID:   item.SubscriptionID,
		Favorite:         item.Favorite,
		EgressOverridden: item.EgressOverridden,
	}
}

func indexByID(items []storedNode, id string) int {
	for index := range items {
		if items[index].ID == id {
			return index
		}
	}
	return -1
}

func cloneState(value state) state {
	value.Nodes = append([]storedNode(nil), value.Nodes...)
	value.SelectedChain = append([]string(nil), value.SelectedChain...)
	return value
}

func newID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate node id: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}
