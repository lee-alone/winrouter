package nodes

import (
	"crypto/rand"
	"encoding/base64"
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
	if s.state.SchemaVersion == 1 {
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
	return s, nil
}

func (s *Store) List() []Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Node, 0, len(s.state.Nodes))
	for _, item := range s.state.Nodes {
		result = append(result, publicNode(item, item.ID == s.state.SelectedID))
	}
	return result
}

func (s *Store) Add(input Input) (Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(input.ID) != "" {
		return Node{}, errors.New("new node must not specify an id")
	}
	item, err := s.makeStored(input, "")
	if err != nil {
		return Node{}, err
	}
	item.ID, err = newID()
	if err != nil {
		return Node{}, err
	}
	next := cloneState(s.state)
	next.Nodes = append(next.Nodes, item)
	if next.SelectedID == "" {
		next.SelectedID = item.ID
	}
	if err := s.save(next); err != nil {
		return Node{}, err
	}
	s.state = next
	return publicNode(item, item.ID == next.SelectedID), nil
}

func (s *Store) Update(input Input) (Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := strings.TrimSpace(input.ID)
	index := indexByID(s.state.Nodes, id)
	if index < 0 {
		return Node{}, fmt.Errorf("node %q not found", id)
	}
	if s.state.Nodes[index].SubscriptionID != "" {
		return Node{}, errors.New("subscription nodes must be updated through their subscription")
	}
	item, err := s.makeStored(input, s.state.Nodes[index].Protected)
	if err != nil {
		return Node{}, err
	}
	item.ID = id
	item.Favorite = s.state.Nodes[index].Favorite
	next := cloneState(s.state)
	next.Nodes[index] = item
	if err := s.save(next); err != nil {
		return Node{}, err
	}
	s.state = next
	return publicNode(item, id == next.SelectedID), nil
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id = strings.TrimSpace(id)
	index := indexByID(s.state.Nodes, id)
	if index < 0 {
		return fmt.Errorf("node %q not found", id)
	}
	if s.state.Nodes[index].SubscriptionID != "" {
		return errors.New("subscription nodes must be deleted through their subscription")
	}
	next := cloneState(s.state)
	next.Nodes = append(next.Nodes[:index], next.Nodes[index+1:]...)
	if next.SelectedID == id {
		next.SelectedID = ""
		if len(next.Nodes) > 0 {
			next.SelectedID = next.Nodes[0].ID
		}
	}
	if err := s.save(next); err != nil {
		return err
	}
	s.state = next
	return nil
}

func (s *Store) Select(id string) (Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id = strings.TrimSpace(id)
	index := indexByID(s.state.Nodes, id)
	if index < 0 {
		return Node{}, fmt.Errorf("node %q not found", id)
	}
	next := cloneState(s.state)
	next.SelectedID = id
	if err := s.save(next); err != nil {
		return Node{}, err
	}
	s.state = next
	return publicNode(next.Nodes[index], true), nil
}

func (s *Store) SetFavorite(id string, favorite bool) (Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := indexByID(s.state.Nodes, strings.TrimSpace(id))
	if index < 0 {
		return Node{}, fmt.Errorf("node %q not found", id)
	}
	next := cloneState(s.state)
	next.Nodes[index].Favorite = favorite
	if err := s.save(next); err != nil {
		return Node{}, err
	}
	s.state = next
	return publicNode(next.Nodes[index], next.Nodes[index].ID == next.SelectedID), nil
}

func (s *Store) CommitResolvedIP(id, address string) (Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := indexByID(s.state.Nodes, strings.TrimSpace(id))
	parsed, err := netip.ParseAddr(strings.TrimSpace(address))
	if index < 0 {
		return Node{}, fmt.Errorf("node %q not found", id)
	}
	if err != nil || !usableIPv4(parsed) {
		return Node{}, errors.New("resolved proxy address must be a usable IPv4 address")
	}
	next := cloneState(s.state)
	next.Nodes[index].ResolvedIP = parsed.String()
	if err := s.save(next); err != nil {
		return Node{}, err
	}
	s.state = next
	return publicNode(next.Nodes[index], next.Nodes[index].ID == next.SelectedID), nil
}

func (s *Store) ReplaceSubscription(subscriptionID string, inputs []Input) ([]Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" {
		return nil, errors.New("subscription id is required")
	}
	favorites := make(map[string]bool)
	for _, existing := range s.state.Nodes {
		if existing.SubscriptionID == subscriptionID && existing.Favorite {
			favorites[nodeIdentity(existing)] = true
		}
	}
	replacements := make([]storedNode, 0, len(inputs))
	for _, input := range inputs {
		input.ID = ""
		item, err := s.makeStored(input, "")
		if err != nil {
			return nil, err
		}
		item.ID, err = newID()
		if err != nil {
			return nil, err
		}
		item.SubscriptionID = subscriptionID
		item.Favorite = favorites[nodeIdentity(item)]
		replacements = append(replacements, item)
	}
	next := cloneState(s.state)
	kept := next.Nodes[:0]
	selectedRemoved := false
	for _, item := range next.Nodes {
		if item.SubscriptionID == subscriptionID {
			selectedRemoved = selectedRemoved || item.ID == next.SelectedID
			continue
		}
		kept = append(kept, item)
	}
	next.Nodes = append(kept, replacements...)
	if selectedRemoved || next.SelectedID == "" {
		next.SelectedID = ""
		if len(replacements) > 0 {
			next.SelectedID = replacements[0].ID
		} else if len(next.Nodes) > 0 {
			next.SelectedID = next.Nodes[0].ID
		}
	}
	if err := s.save(next); err != nil {
		return nil, err
	}
	s.state = next
	result := make([]Node, 0, len(replacements))
	for _, item := range replacements {
		result = append(result, publicNode(item, item.ID == next.SelectedID))
	}
	return result, nil
}

func (s *Store) SelectedCredentials() (Node, Credentials, error) {
	s.mu.RLock()
	id := s.state.SelectedID
	s.mu.RUnlock()
	if id == "" {
		return Node{}, Credentials{}, errors.New("no proxy node selected")
	}
	return s.Credentials(id)
}

func (s *Store) Credentials(id string) (Node, Credentials, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	index := indexByID(s.state.Nodes, strings.TrimSpace(id))
	if index < 0 {
		return Node{}, Credentials{}, fmt.Errorf("node %q not found", id)
	}
	item := s.state.Nodes[index]
	credentials := Credentials{Username: item.Username}
	if item.Protected != "" {
		ciphertext, err := base64.StdEncoding.DecodeString(item.Protected)
		if err != nil {
			return Node{}, Credentials{}, errors.New("decode protected node password")
		}
		plaintext, err := s.protector.Unprotect(ciphertext)
		if err != nil {
			return Node{}, Credentials{}, fmt.Errorf("unprotect node password: %w", err)
		}
		credentials.Password = string(plaintext)
	}
	return publicNode(item, true), credentials, nil
}

func (s *Store) makeStored(input Input, existingProtected string) (storedNode, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Type = strings.ToLower(strings.TrimSpace(input.Type))
	input.Server = strings.TrimSpace(input.Server)
	input.Username = strings.TrimSpace(input.Username)
	if input.Name == "" || (input.Type != TypeHTTP && input.Type != TypeShadowsocks) || input.Port == 0 {
		return storedNode{}, errors.New("node name, supported type, and port are required")
	}
	if input.Type == TypeShadowsocks {
		if !supportedShadowsocksMethod(input.Username) {
			return storedNode{}, fmt.Errorf("unsupported Shadowsocks method %q", input.Username)
		}
		if input.Password == "" && existingProtected == "" {
			return storedNode{}, errors.New("Shadowsocks password is required")
		}
	}
	server := strings.ToLower(strings.TrimSuffix(input.Server, "."))
	address, addressErr := netip.ParseAddr(server)
	if addressErr == nil {
		if !usableIPv4(address) {
			return storedNode{}, errors.New("node server must be a usable IPv4 address or DNS name")
		}
		server = address.String()
	} else if !validDNSName(server) {
		return storedNode{}, errors.New("node server must be a usable IPv4 address or DNS name")
	}
	protected := existingProtected
	if input.ClearPassword && input.Password != "" {
		return storedNode{}, errors.New("clear_password and password cannot be used together")
	}
	if input.ClearPassword {
		protected = ""
	}
	if input.Password != "" {
		ciphertext, err := s.protector.Protect([]byte(input.Password))
		if err != nil {
			return storedNode{}, fmt.Errorf("protect node password: %w", err)
		}
		protected = base64.StdEncoding.EncodeToString(ciphertext)
	}
	resolved := ""
	if existing := indexByID(s.state.Nodes, strings.TrimSpace(input.ID)); existing >= 0 && strings.EqualFold(s.state.Nodes[existing].Server, server) {
		resolved = s.state.Nodes[existing].ResolvedIP
	}
	if addressErr == nil {
		resolved = server
	}
	return storedNode{Name: input.Name, Type: input.Type, Server: server, ResolvedIP: resolved, Port: input.Port, Username: input.Username, Protected: protected}, nil
}

func supportedShadowsocksMethod(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "aes-128-gcm", "aes-192-gcm", "aes-256-gcm", "chacha20-ietf-poly1305", "xchacha20-ietf-poly1305":
		return true
	default:
		return false
	}
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
			if err != nil || !usableIPv4(address) {
				return fmt.Errorf("stored node %q has invalid resolved IP", item.ID)
			}
		}
	}
	if value.SelectedID != "" {
		if _, exists := seen[value.SelectedID]; !exists {
			return errors.New("selected node does not exist")
		}
	}
	return nil
}

func publicNode(item storedNode, selected bool) Node {
	return Node{ID: item.ID, Name: item.Name, Type: item.Type, Server: item.Server, ResolvedIP: item.ResolvedIP, Port: item.Port, Username: item.Username, HasPassword: item.Protected != "", Selected: selected, SubscriptionID: item.SubscriptionID, Favorite: item.Favorite}
}

func nodeIdentity(item storedNode) string {
	return strings.ToLower(fmt.Sprintf("%s|%s|%d", item.Type, item.Server, item.Port))
}

func usableIPv4(address netip.Addr) bool {
	return address.Is4() && !address.IsUnspecified() && !address.IsLoopback() && !address.IsMulticast()
}

func validDNSName(value string) bool {
	if len(value) == 0 || len(value) > 253 || !strings.Contains(value, ".") {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
				return false
			}
		}
	}
	return true
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
	return value
}

func newID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate node id: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}
