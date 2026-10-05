package nodes

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

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
		if !usableIPv4(address) && !usableIPv6(address) {
			return storedNode{}, errors.New("node server must be a usable fixed IP address or DNS name")
		}
		server = address.String()
	} else if !validDNSName(server) {
		return storedNode{}, errors.New("node server must be a usable fixed IP address or DNS name")
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
