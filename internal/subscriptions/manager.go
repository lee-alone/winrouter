package subscriptions

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"winrouter/internal/nodes"
)

const maxDocumentSize = 1 << 20

type Manager struct {
	mu        sync.Mutex
	path      string
	protector nodes.Protector
	nodes     *nodes.Store
	client    *http.Client
	state     state
}

func New(path string, protector nodes.Protector, nodeStore *nodes.Store) (*Manager, error) {
	if path == "" || protector == nil || nodeStore == nil {
		return nil, errors.New("subscription path, protector, and node store are required")
	}
	manager := &Manager{
		path: path, protector: protector, nodes: nodeStore,
		state: state{SchemaVersion: SchemaVersion, Subscriptions: []storedSubscription{}},
		client: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("subscription redirects are disabled")
		}},
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return manager, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read subscriptions: %w", err)
	}
	if err := json.Unmarshal(data, &manager.state); err != nil {
		return nil, fmt.Errorf("decode subscriptions: %w", err)
	}
	if manager.state.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("unsupported subscription schema %d", manager.state.SchemaVersion)
	}
	return manager, nil
}

func (m *Manager) List() []Subscription {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]Subscription, 0, len(m.state.Subscriptions))
	for _, item := range m.state.Subscriptions {
		result = append(result, public(item))
	}
	return result
}

func (m *Manager) Add(input Input) (Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if input.ID != "" {
		return Subscription{}, errors.New("new subscription must not specify an id")
	}
	item, err := m.makeStored(input, "")
	if err != nil {
		return Subscription{}, err
	}
	item.ID, err = randomID()
	if err != nil {
		return Subscription{}, err
	}
	next := clone(m.state)
	next.Subscriptions = append(next.Subscriptions, item)
	if err := m.save(next); err != nil {
		return Subscription{}, err
	}
	m.state = next
	return public(item), nil
}

func (m *Manager) UpdateDefinition(input Input) (Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	index := find(m.state.Subscriptions, input.ID)
	if index < 0 {
		return Subscription{}, fmt.Errorf("subscription %q not found", input.ID)
	}
	item, err := m.makeStored(input, m.state.Subscriptions[index].ProtectedURL)
	if err != nil {
		return Subscription{}, err
	}
	item.ID = input.ID
	item.NodeCount = m.state.Subscriptions[index].NodeCount
	item.UpdatedAt = m.state.Subscriptions[index].UpdatedAt
	next := clone(m.state)
	next.Subscriptions[index] = item
	if err := m.save(next); err != nil {
		return Subscription{}, err
	}
	m.state = next
	return public(item), nil
}

func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	index := find(m.state.Subscriptions, id)
	if index < 0 {
		return fmt.Errorf("subscription %q not found", id)
	}
	if _, err := m.nodes.ReplaceSubscription(id, nil); err != nil {
		return err
	}
	next := clone(m.state)
	next.Subscriptions = append(next.Subscriptions[:index], next.Subscriptions[index+1:]...)
	if err := m.save(next); err != nil {
		return err
	}
	m.state = next
	return nil
}

func (m *Manager) Refresh(ctx context.Context, id string) (Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	index := find(m.state.Subscriptions, id)
	if index < 0 {
		return Subscription{}, fmt.Errorf("subscription %q not found", id)
	}
	address, err := m.unprotectURL(m.state.Subscriptions[index].ProtectedURL)
	if err != nil {
		return Subscription{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return Subscription{}, err
	}
	request.Header.Set("Accept", "application/json")
	response, err := m.client.Do(request)
	if err != nil {
		return Subscription{}, m.recordFailure(index, "download failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Subscription{}, m.recordFailure(index, fmt.Sprintf("HTTP %d", response.StatusCode))
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxDocumentSize+1))
	if err != nil || len(data) > maxDocumentSize {
		return Subscription{}, m.recordFailure(index, "invalid response size")
	}
	inputs, err := Parse(data)
	if err != nil {
		return Subscription{}, m.recordFailure(index, "subscription document validation failed: "+err.Error())
	}
	updated, err := m.nodes.ReplaceSubscription(id, inputs)
	if err != nil {
		return Subscription{}, m.recordFailure(index, "node validation failed")
	}
	next := clone(m.state)
	next.Subscriptions[index].NodeCount = len(updated)
	next.Subscriptions[index].UpdatedAt = time.Now().UTC()
	next.Subscriptions[index].LastError = ""
	if err := m.save(next); err != nil {
		return Subscription{}, err
	}
	m.state = next
	return public(next.Subscriptions[index]), nil
}

func (m *Manager) makeStored(input Input, existing string) (storedSubscription, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return storedSubscription{}, errors.New("subscription name is required")
	}
	protected, host := existing, ""
	if strings.TrimSpace(input.URL) != "" {
		parsed, err := validateSubscriptionURL(input.URL)
		if err != nil {
			return storedSubscription{}, err
		}
		ciphertext, err := m.protector.Protect([]byte(parsed.String()))
		if err != nil {
			return storedSubscription{}, err
		}
		protected = base64.StdEncoding.EncodeToString(ciphertext)
		host = parsed.Hostname()
	} else if existing != "" {
		address, err := m.unprotectURL(existing)
		if err != nil {
			return storedSubscription{}, err
		}
		parsed, _ := url.Parse(address)
		host = parsed.Hostname()
	} else {
		return storedSubscription{}, errors.New("subscription URL is required")
	}
	return storedSubscription{Name: name, ProtectedURL: protected, Host: host}, nil
}

func validateSubscriptionURL(value string) (*url.URL, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil {
		return nil, errors.New("subscription URL must have a valid host and must not use URL userinfo")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "https":
		return parsed, nil
	case "http":
		ip := net.ParseIP(parsed.Hostname())
		if ip != nil && (ip.IsPrivate() || ip.IsLoopback()) {
			return parsed, nil
		}
		return nil, errors.New("HTTP subscription URL is allowed only for a literal private or loopback IP address")
	default:
		return nil, errors.New("subscription URL must use HTTPS, or HTTP with a literal private or loopback IP address")
	}
}

func (m *Manager) unprotectURL(value string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	plaintext, err := m.protector.Unprotect(ciphertext)
	return string(plaintext), err
}

func (m *Manager) recordFailure(index int, message string) error {
	next := clone(m.state)
	next.Subscriptions[index].LastError = message
	if err := m.save(next); err == nil {
		m.state = next
	}
	return errors.New(message)
}

func (m *Manager) save(next state) error {
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	directory := filepath.Dir(m.path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".subscriptions-*.tmp")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	_ = temporary.Chmod(0o600)
	if _, err = temporary.Write(append(data, '\n')); err == nil {
		err = temporary.Close()
	} else {
		_ = temporary.Close()
	}
	if err != nil {
		return err
	}
	return replaceFile(name, m.path)
}

func public(item storedSubscription) Subscription {
	return Subscription{ID: item.ID, Name: item.Name, Host: item.Host, NodeCount: item.NodeCount, UpdatedAt: item.UpdatedAt, LastError: item.LastError}
}

func clone(value state) state {
	value.Subscriptions = append([]storedSubscription(nil), value.Subscriptions...)
	return value
}

func find(items []storedSubscription, id string) int {
	for index := range items {
		if items[index].ID == id {
			return index
		}
	}
	return -1
}

func randomID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}
