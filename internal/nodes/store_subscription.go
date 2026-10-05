package nodes

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func (s *Store) UpdateSubscriptionDefaultEgress(subscriptionID string, newEgress string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	subscriptionID = strings.TrimSpace(subscriptionID)
	newEgress = strings.ToLower(strings.TrimSpace(newEgress))
	if newEgress != EgressA && newEgress != EgressB {
		return errors.New("subscription egress must be 'a' or 'b'")
	}
	next := cloneState(s.state)
	changed := false
	for i := range next.Nodes {
		if next.Nodes[i].SubscriptionID == subscriptionID {
			if !next.Nodes[i].EgressOverridden {
				if next.Nodes[i].Egress != newEgress {
					next.Nodes[i].Egress = newEgress
					changed = true
				}
			}
		}
	}
	if !changed {
		return nil
	}
	if err := s.save(next); err != nil {
		return err
	}
	s.state = next
	return nil
}

func (s *Store) ReplaceSubscription(subscriptionID string, inputs []Input) ([]Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" {
		return nil, errors.New("subscription id is required")
	}

	favorites := make(map[string]bool)
	egressOverrides := make(map[string]string)
	egressOverriddenMap := make(map[string]bool)
	var selectedIdentity string
	for _, existing := range s.state.Nodes {
		if existing.SubscriptionID == subscriptionID {
			identity := s.storedNodeIdentity(existing)
			if existing.Favorite {
				favorites[identity] = true
			}
			if existing.EgressOverridden {
				egressOverrides[identity] = existing.Egress
				egressOverriddenMap[identity] = true
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
		overridden := egressOverriddenMap[identityKey]
		if overridden {
			input.Egress = egressOverrides[identityKey]
		}
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
		item.EgressOverridden = overridden
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

	validNodeIDs := make(map[string]struct{}, len(next.Nodes))
	for _, n := range next.Nodes {
		validNodeIDs[n.ID] = struct{}{}
	}
	nextChain := make([]string, 0, len(next.SelectedChain))
	for _, cid := range next.SelectedChain {
		if _, ok := validNodeIDs[cid]; ok {
			nextChain = append(nextChain, cid)
		}
	}
	next.SelectedChain = nextChain
	if len(next.Nodes) == 0 {
		next.SelectedID = ""
		next.Mode = ProxyModeSingle
		next.SelectedChain = []string{}
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
	return fmt.Sprintf("%s|%s|%d|%s|%s|%s|%s|%s|%s",
		strings.ToLower(item.Type), strings.ToLower(item.Server), item.Port,
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
	return fmt.Sprintf("%s|%s|%d|%s|%s|%s|%s|%s|%s",
		strings.ToLower(input.Type), strings.ToLower(input.Server), input.Port,
		username, strings.ToLower(method), strings.ToLower(auth.Flow), secret, tlsKey, transportKey)
}
