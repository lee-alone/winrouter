package nodes

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

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
	nextChain := make([]string, 0, len(next.SelectedChain))
	for _, cid := range next.SelectedChain {
		if cid != id {
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
	next.Mode = ProxyModeSingle
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

func (s *Store) SetNodeEgress(id string, egress string, overridden bool) (Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id = strings.TrimSpace(id)
	egress = strings.ToLower(strings.TrimSpace(egress))
	if egress != EgressA && egress != EgressB {
		return Node{}, errors.New("node egress must be 'a' or 'b'")
	}
	index := indexByID(s.state.Nodes, id)
	if index < 0 {
		return Node{}, fmt.Errorf("node %q not found", id)
	}
	next := cloneState(s.state)
	next.Nodes[index].Egress = egress
	if next.Nodes[index].SubscriptionID != "" {
		next.Nodes[index].EgressOverridden = overridden
	}
	if err := s.save(next); err != nil {
		return Node{}, err
	}
	s.state = next
	return publicNode(next.Nodes[index], next.Nodes[index].ID == next.SelectedID), nil
}
