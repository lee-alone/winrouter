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
	item, err := s.makeStored(input, s.state.Nodes[index].ProtectedSecret)
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
	if err != nil || (!usableIPv4(parsed) && !usableIPv6(parsed)) {
		return Node{}, errors.New("resolved proxy address must be a usable IP address")
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
	var selectedIdentity string
	for _, existing := range s.state.Nodes {
		if existing.SubscriptionID == subscriptionID {
			identity := s.storedNodeIdentity(existing)
			if existing.Favorite {
				favorites[identity] = true
			}
			if existing.ID == s.state.SelectedID {
				selectedIdentity = identity
			}
		}
	}

	var newSelectedID string
	replacements := make([]storedNode, 0, len(inputs))
	for _, input := range inputs {
		input.ID = ""
		identityKey := inputIdentity(input)
		item, err := s.makeStored(input, "")
		if err != nil {
			return nil, err
		}
		item.ID, err = newID()
		if err != nil {
			return nil, err
		}
		item.SubscriptionID = subscriptionID
		item.Favorite = favorites[identityKey]
		if selectedIdentity != "" && selectedIdentity == identityKey && newSelectedID == "" {
			newSelectedID = item.ID
		}
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
	if selectedRemoved {
		if newSelectedID != "" {
			next.SelectedID = newSelectedID
		} else if len(replacements) > 0 {
			next.SelectedID = replacements[0].ID
		} else if len(next.Nodes) > 0 {
			next.SelectedID = next.Nodes[0].ID
		} else {
			next.SelectedID = ""
		}
	} else if next.SelectedID == "" {
		if len(next.Nodes) > 0 {
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
	credentials := Credentials{Username: item.Username, Method: item.Method, Flow: item.Flow}
	if item.ProtectedSecret != "" {
		ciphertext, err := base64.StdEncoding.DecodeString(item.ProtectedSecret)
		if err != nil {
			return Node{}, Credentials{}, errors.New("decode protected node secret")
		}
		plaintext, err := s.protector.Unprotect(ciphertext)
		if err != nil {
			return Node{}, Credentials{}, fmt.Errorf("unprotect node secret: %w", err)
		}
		var doc secretDocument
		if err := json.Unmarshal(plaintext, &doc); err == nil && (doc.Password != "" || doc.UUID != "") {
			credentials.Password = doc.Password
			credentials.UUID = doc.UUID
		} else {
			credentials.Password = string(plaintext)
		}
	}
	return publicNode(item, true), credentials, nil
}

func validUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, c := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func (s *Store) makeStored(input Input, existingProtected string) (storedNode, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Type = strings.ToLower(strings.TrimSpace(input.Type))
	input.Server = strings.TrimSpace(input.Server)
	input.Egress = strings.ToLower(strings.TrimSpace(input.Egress))
	if input.Egress == "" {
		input.Egress = EgressB
	}
	if input.Egress != EgressA && input.Egress != EgressB {
		return storedNode{}, errors.New("node egress must be 'a' or 'b'")
	}

	auth := input.Authentication
	if auth.Username == "" && input.Username != "" {
		auth.Username = input.Username
	}
	if auth.Password == "" && input.Password != "" {
		auth.Password = input.Password
	}
	if input.Type == TypeShadowsocks && auth.Method == "" && input.Username != "" {
		auth.Method = input.Username
	}
	clearSecret := input.ClearSecret || input.ClearPassword

	if input.Name == "" || input.Port == 0 {
		return storedNode{}, errors.New("node name and port are required")
	}

	switch input.Type {
	case TypeHTTP:
	case TypeShadowsocks:
		if !supportedShadowsocksMethod(auth.Method) {
			return storedNode{}, fmt.Errorf("unsupported Shadowsocks method %q", auth.Method)
		}
		if clearSecret {
			return storedNode{}, errors.New("Shadowsocks nodes must have a password; cannot clear secret")
		}
		if auth.Password == "" && existingProtected == "" {
			return storedNode{}, errors.New("Shadowsocks password is required")
		}
	case TypeVMess:
		if auth.UUID == "" && auth.Password != "" {
			auth.UUID = auth.Password
		}
		if clearSecret {
			return storedNode{}, errors.New("VMess nodes must have a UUID; cannot clear secret")
		}
		if auth.UUID == "" && existingProtected == "" {
			return storedNode{}, errors.New("VMess UUID is required")
		}
		if auth.UUID != "" && !validUUID(auth.UUID) {
			return storedNode{}, errors.New("VMess UUID must be a valid 36-character UUID")
		}
	case TypeVLESS:
		if auth.UUID == "" && auth.Password != "" {
			auth.UUID = auth.Password
		}
		if clearSecret {
			return storedNode{}, errors.New("VLESS nodes must have a UUID; cannot clear secret")
		}
		if auth.UUID == "" && existingProtected == "" {
			return storedNode{}, errors.New("VLESS UUID is required")
		}
		if auth.UUID != "" && !validUUID(auth.UUID) {
			return storedNode{}, errors.New("VLESS UUID must be a valid 36-character UUID")
		}
		if auth.Flow != "" && auth.Flow != "xtls-rprx-vision" {
			return storedNode{}, fmt.Errorf("unsupported VLESS flow %q", auth.Flow)
		}
	case TypeTrojan:
		if clearSecret {
			return storedNode{}, errors.New("Trojan nodes must have a password; cannot clear secret")
		}
		if auth.Password == "" && existingProtected == "" {
			return storedNode{}, errors.New("Trojan password is required")
		}
	default:
		return storedNode{}, fmt.Errorf("unsupported proxy type %q", input.Type)
	}

	if input.TLS != nil && input.TLS.Enabled {
		if input.TLS.ServerName != "" {
			sName := strings.ToLower(strings.TrimSpace(input.TLS.ServerName))
			if strings.Contains(sName, " ") || len(sName) > 253 {
				return storedNode{}, errors.New("invalid TLS server_name")
			}
		}
		for _, alpn := range input.TLS.ALPN {
			if strings.TrimSpace(alpn) == "" || len(alpn) > 32 {
				return storedNode{}, errors.New("invalid TLS ALPN entry")
			}
		}
	}

	if input.Transport != nil && input.Transport.Type != "" {
		tType := strings.ToLower(strings.TrimSpace(input.Transport.Type))
		if tType != "ws" && tType != "tcp" {
			return storedNode{}, fmt.Errorf("unsupported transport type %q", input.Transport.Type)
		}
		if input.Transport.Path != "" {
			if !strings.HasPrefix(input.Transport.Path, "/") || len(input.Transport.Path) > 2048 {
				return storedNode{}, errors.New("transport path must start with '/' and be at most 2048 characters")
			}
		}
		if input.Transport.Host != "" {
			if strings.Contains(input.Transport.Host, " ") || len(input.Transport.Host) > 253 {
				return storedNode{}, errors.New("invalid transport host header")
			}
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

	if clearSecret && (auth.Password != "" || auth.UUID != "" || input.Password != "") {
		return storedNode{}, errors.New("clear_secret and secret cannot be used together")
	}

	protected := existingProtected
	if clearSecret {
		protected = ""
	}
	if auth.Password != "" || auth.UUID != "" {
		doc := secretDocument{Password: auth.Password, UUID: auth.UUID}
		docBytes, err := json.Marshal(doc)
		if err != nil {
			return storedNode{}, fmt.Errorf("marshal node secret: %w", err)
		}
		ciphertext, err := s.protector.Protect(docBytes)
		if err != nil {
			return storedNode{}, fmt.Errorf("protect node secret: %w", err)
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

	var tlsNode *TLSNode
	if input.TLS != nil {
		tlsNode = &TLSNode{
			Enabled:    input.TLS.Enabled,
			ServerName: strings.TrimSpace(input.TLS.ServerName),
			Insecure:   input.TLS.Insecure,
			ALPN:       append([]string(nil), input.TLS.ALPN...),
		}
	}

	var transportNode *TransportNode
	if input.Transport != nil {
		transportNode = &TransportNode{
			Type: strings.ToLower(strings.TrimSpace(input.Transport.Type)),
			Path: strings.TrimSpace(input.Transport.Path),
			Host: strings.TrimSpace(input.Transport.Host),
		}
	}

	return storedNode{
		Name:            input.Name,
		Type:            input.Type,
		Server:          server,
		ResolvedIP:      resolved,
		Port:            input.Port,
		Egress:          input.Egress,
		Username:        strings.TrimSpace(auth.Username),
		Method:          strings.TrimSpace(auth.Method),
		Flow:            strings.TrimSpace(auth.Flow),
		TLS:             tlsNode,
		Transport:       transportNode,
		ProtectedSecret: protected,
	}, nil
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
		TLS:            item.TLS,
		Transport:      item.Transport,
		HasSecret:      item.ProtectedSecret != "",
		HasPassword:    item.ProtectedSecret != "",
		Username:       username,
		Selected:       selected,
		SubscriptionID: item.SubscriptionID,
		Favorite:       item.Favorite,
	}
}

func (s *Store) storedNodeIdentity(item storedNode) string {
	var secret string
	if item.ProtectedSecret != "" {
		if ciphertext, err := base64.StdEncoding.DecodeString(item.ProtectedSecret); err == nil {
			if plaintext, err := s.protector.Unprotect(ciphertext); err == nil {
				var doc secretDocument
				if err := json.Unmarshal(plaintext, &doc); err == nil && (doc.Password != "" || doc.UUID != "") {
					secret = doc.Password
					if secret == "" {
						secret = doc.UUID
					}
				} else {
					secret = string(plaintext)
				}
			}
		}
	}
	tlsKey := ""
	if item.TLS != nil && item.TLS.Enabled {
		tlsKey = fmt.Sprintf("%t|%s|%t|%s", item.TLS.Enabled, strings.ToLower(item.TLS.ServerName), item.TLS.Insecure, strings.Join(item.TLS.ALPN, ","))
	}
	transportKey := ""
	if item.Transport != nil && item.Transport.Type != "" {
		transportKey = fmt.Sprintf("%s|%s|%s", strings.ToLower(item.Transport.Type), item.Transport.Path, strings.ToLower(item.Transport.Host))
	}
	egress := strings.ToLower(item.Egress)
	if egress == "" {
		egress = EgressB
	}
	return fmt.Sprintf("%s|%s|%d|%s|%s|%s|%s|%s|%s|%s",
		strings.ToLower(item.Type), strings.ToLower(item.Server), item.Port, egress,
		item.Username, strings.ToLower(item.Method), strings.ToLower(item.Flow), secret, tlsKey, transportKey)
}

func inputIdentity(input Input) string {
	tlsKey := ""
	if input.TLS != nil && input.TLS.Enabled {
		tlsKey = fmt.Sprintf("%t|%s|%t|%s", input.TLS.Enabled, strings.ToLower(input.TLS.ServerName), input.TLS.Insecure, strings.Join(input.TLS.ALPN, ","))
	}
	transportKey := ""
	if input.Transport != nil && input.Transport.Type != "" {
		transportKey = fmt.Sprintf("%s|%s|%s", strings.ToLower(input.Transport.Type), input.Transport.Path, strings.ToLower(input.Transport.Host))
	}
	auth := input.Authentication
	secret := auth.Password
	if secret == "" {
		secret = auth.UUID
	}
	if secret == "" {
		secret = input.Password
	}
	method := auth.Method
	if method == "" && input.Type == TypeShadowsocks {
		method = input.Username
	}
	username := auth.Username
	if username == "" && input.Type != TypeShadowsocks {
		username = input.Username
	}
	egress := strings.ToLower(input.Egress)
	if egress == "" {
		egress = EgressB
	}
	return fmt.Sprintf("%s|%s|%d|%s|%s|%s|%s|%s|%s|%s",
		strings.ToLower(input.Type), strings.ToLower(input.Server), input.Port, egress,
		username, strings.ToLower(method), strings.ToLower(auth.Flow), secret, tlsKey, transportKey)
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
