package nodes

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func (s *Store) GetSelection() ProxySelection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	mode := s.state.Mode
	if mode == "" || len(s.state.Nodes) == 0 {
		mode = ProxyModeSingle
	}
	chain := make([]string, 0, len(s.state.SelectedChain))
	for _, id := range s.state.SelectedChain {
		if id != "" {
			chain = append(chain, id)
		}
	}
	return ProxySelection{
		Mode:          mode,
		SelectedID:    s.state.SelectedID,
		SelectedChain: chain,
	}
}

func (s *Store) SetMode(mode string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if mode != ProxyModeSingle && mode != ProxyModeChain {
		return fmt.Errorf("unsupported proxy mode %q", mode)
	}
	if mode == ProxyModeChain && len(s.state.Nodes) == 0 {
		return errors.New("cannot enable chain mode without any proxy nodes")
	}
	next := cloneState(s.state)
	next.Mode = mode
	if err := s.save(next); err != nil {
		return err
	}
	s.state = next
	return nil
}

func (s *Store) SelectChain(ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	clean := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if indexByID(s.state.Nodes, trimmed) < 0 {
			return fmt.Errorf("node %q not found", trimmed)
		}
		if _, ok := seen[trimmed]; ok {
			return fmt.Errorf("duplicate node %q in chain", trimmed)
		}
		seen[trimmed] = struct{}{}
		clean = append(clean, trimmed)
	}
	next := cloneState(s.state)
	next.SelectedChain = clean
	if len(clean) > 0 {
		next.Mode = ProxyModeChain
	}
	if err := s.save(next); err != nil {
		return err
	}
	s.state = next
	return nil
}

func (s *Store) SelectedChainCredentials() ([]Node, []Credentials, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.state.SelectedChain) == 0 {
		return nil, nil, errors.New("no proxy chain configured")
	}
	nodesList := make([]Node, 0, len(s.state.SelectedChain))
	credsList := make([]Credentials, 0, len(s.state.SelectedChain))
	for idx, id := range s.state.SelectedChain {
		index := indexByID(s.state.Nodes, id)
		if index < 0 {
			return nil, nil, fmt.Errorf("proxy chain node %q not found", id)
		}
		item := s.state.Nodes[index]
		creds := Credentials{Username: item.Username, Method: item.Method, Flow: item.Flow}
		if item.ProtectedSecret != "" {
			ciphertext, err := base64.StdEncoding.DecodeString(item.ProtectedSecret)
			if err != nil {
				return nil, nil, errors.New("decode protected node secret")
			}
			plaintext, err := s.protector.Unprotect(ciphertext)
			if err != nil {
				return nil, nil, fmt.Errorf("unprotect node secret: %w", err)
			}
			var doc secretDocument
			if err := json.Unmarshal(plaintext, &doc); err == nil && (doc.Password != "" || doc.UUID != "") {
				creds.Password = doc.Password
				creds.UUID = doc.UUID
			} else {
				creds.Password = string(plaintext)
			}
		}
		n := publicNode(item, true)
		n.ChainPosition = idx + 1
		nodesList = append(nodesList, n)
		credsList = append(credsList, creds)
	}
	return nodesList, credsList, nil
}
