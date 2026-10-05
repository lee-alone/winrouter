package nodes

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/pbkdf2"
)

var (
	ErrLocked        = errors.New("security vault is locked; PIN required")
	ErrInvalidPIN    = errors.New("incorrect security PIN")
	ErrPINAlreadySet = errors.New("security PIN is already enabled")
	ErrPINNotSet     = errors.New("security PIN is not enabled")
	ErrKeyCorrupted  = errors.New("master key is corrupted")
	magicPrefix      = []byte("WR_GCM_V1:")
)

const (
	masterKeyLength = 32
	pbkdf2SaltLen   = 16
	pbkdf2Iter      = 100_000
)

// EncryptedKeyFile represents the serializable structure of master.key.enc
type EncryptedKeyFile struct {
	Version    int    `json:"version"`
	Salt       []byte `json:"salt"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
}

// SecurityStatus represents whether PIN protection is enabled and if the vault is unlocked.
type SecurityStatus struct {
	PINEnabled bool `json:"pin_enabled"`
	Unlocked   bool `json:"unlocked"`
}

type PortableProtector struct {
	mu          sync.RWMutex
	dir         string
	rawKeyPath  string
	encKeyPath  string
	masterKey   []byte
	pinEnabled  bool
	unlocked    bool
	legacyDPAPI Protector // for backward compatibility fallback
}

// NewPortableProtector initializes a portable protector in the specified directory.
func NewPortableProtector(dir string, legacy Protector) (*PortableProtector, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create protector directory: %w", err)
	}

	p := &PortableProtector{
		dir:         dir,
		rawKeyPath:  filepath.Join(dir, "master.key"),
		encKeyPath:  filepath.Join(dir, "master.key.enc"),
		legacyDPAPI: legacy,
	}

	// Case 1: master.key.enc exists -> PIN is enabled and vault starts locked
	if _, err := os.Stat(p.encKeyPath); err == nil {
		p.pinEnabled = true
		p.unlocked = false
		return p, nil
	}

	// Case 2: raw master.key exists -> Load it
	if raw, err := os.ReadFile(p.rawKeyPath); err == nil {
		if len(raw) != masterKeyLength {
			return nil, ErrKeyCorrupted
		}
		p.masterKey = append([]byte(nil), raw...)
		p.pinEnabled = false
		p.unlocked = true
		return p, nil
	}

	// Case 3: Neither exists -> Generate a fresh 32-byte master key
	key := make([]byte, masterKeyLength)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate master key: %w", err)
	}
	if err := os.WriteFile(p.rawKeyPath, key, 0o600); err != nil {
		return nil, fmt.Errorf("save master key: %w", err)
	}

	p.masterKey = key
	p.pinEnabled = false
	p.unlocked = true
	return p, nil
}

func (p *PortableProtector) Status() SecurityStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return SecurityStatus{
		PINEnabled: p.pinEnabled,
		Unlocked:   p.unlocked,
	}
}

// UnlockWithPIN unlocks the master key using the provided user PIN.
func (p *PortableProtector) UnlockWithPIN(pin string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.pinEnabled {
		return nil
	}

	data, err := os.ReadFile(p.encKeyPath)
	if err != nil {
		return fmt.Errorf("read encrypted key file: %w", err)
	}

	var encFile EncryptedKeyFile
	if err := json.Unmarshal(data, &encFile); err != nil {
		return fmt.Errorf("parse encrypted key file: %w", err)
	}

	key := pbkdf2.Key([]byte(pin), encFile.Salt, pbkdf2Iter, masterKeyLength, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	decrypted, err := gcm.Open(nil, encFile.Nonce, encFile.Ciphertext, nil)
	if err != nil {
		return ErrInvalidPIN
	}
	if len(decrypted) != masterKeyLength {
		return ErrKeyCorrupted
	}

	p.masterKey = decrypted
	p.unlocked = true
	return nil
}

// EnablePIN seals the master key with a newly set PIN.
func (p *PortableProtector) EnablePIN(pin string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(pin) < 4 {
		return errors.New("PIN must be at least 4 characters")
	}
	if !p.unlocked || len(p.masterKey) != masterKeyLength {
		return ErrLocked
	}

	salt := make([]byte, pbkdf2SaltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return err
	}

	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}

	derived := pbkdf2.Key([]byte(pin), salt, pbkdf2Iter, masterKeyLength, sha256.New)
	block, err := aes.NewCipher(derived)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	ciphertext := gcm.Seal(nil, nonce, p.masterKey, nil)
	encFile := EncryptedKeyFile{
		Version:    1,
		Salt:       salt,
		Nonce:      nonce,
		Ciphertext: ciphertext,
	}

	data, err := json.MarshalIndent(encFile, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(p.encKeyPath, data, 0o600); err != nil {
		return fmt.Errorf("write encrypted key file: %w", err)
	}

	_ = os.Remove(p.rawKeyPath)
	p.pinEnabled = true
	p.unlocked = true
	return nil
}

// ChangePIN changes the PIN used to seal the master key.
func (p *PortableProtector) ChangePIN(oldPIN, newPIN string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.pinEnabled {
		return ErrPINNotSet
	}
	if len(newPIN) < 4 {
		return errors.New("new PIN must be at least 4 characters")
	}

	data, err := os.ReadFile(p.encKeyPath)
	if err != nil {
		return err
	}
	var encFile EncryptedKeyFile
	if err := json.Unmarshal(data, &encFile); err != nil {
		return err
	}

	oldKey := pbkdf2.Key([]byte(oldPIN), encFile.Salt, pbkdf2Iter, masterKeyLength, sha256.New)
	block, err := aes.NewCipher(oldKey)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	masterKey, err := gcm.Open(nil, encFile.Nonce, encFile.Ciphertext, nil)
	if err != nil {
		return ErrInvalidPIN
	}

	// Re-encrypt with new PIN
	newSalt := make([]byte, pbkdf2SaltLen)
	if _, err := io.ReadFull(rand.Reader, newSalt); err != nil {
		return err
	}
	newNonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, newNonce); err != nil {
		return err
	}

	newDerived := pbkdf2.Key([]byte(newPIN), newSalt, pbkdf2Iter, masterKeyLength, sha256.New)
	newBlock, err := aes.NewCipher(newDerived)
	if err != nil {
		return err
	}
	newGCM, err := cipher.NewGCM(newBlock)
	if err != nil {
		return err
	}

	newCiphertext := newGCM.Seal(nil, newNonce, masterKey, nil)
	newEncFile := EncryptedKeyFile{
		Version:    1,
		Salt:       newSalt,
		Nonce:      newNonce,
		Ciphertext: newCiphertext,
	}

	newData, err := json.MarshalIndent(newEncFile, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(p.encKeyPath, newData, 0o600); err != nil {
		return err
	}

	p.masterKey = masterKey
	p.unlocked = true
	return nil
}

// DisablePIN removes the PIN lock and stores master key in raw format.
func (p *PortableProtector) DisablePIN(pin string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.pinEnabled {
		return nil
	}

	data, err := os.ReadFile(p.encKeyPath)
	if err != nil {
		return err
	}
	var encFile EncryptedKeyFile
	if err := json.Unmarshal(data, &encFile); err != nil {
		return err
	}

	key := pbkdf2.Key([]byte(pin), encFile.Salt, pbkdf2Iter, masterKeyLength, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	masterKey, err := gcm.Open(nil, encFile.Nonce, encFile.Ciphertext, nil)
	if err != nil {
		return ErrInvalidPIN
	}

	if err := os.WriteFile(p.rawKeyPath, masterKey, 0o600); err != nil {
		return fmt.Errorf("write raw master key: %w", err)
	}

	_ = os.Remove(p.encKeyPath)
	p.masterKey = masterKey
	p.pinEnabled = false
	p.unlocked = true
	return nil
}

// Protect encrypts plaintext using AES-256-GCM with the master key.
func (p *PortableProtector) Protect(plaintext []byte) ([]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.unlocked || len(p.masterKey) != masterKeyLength {
		return nil, ErrLocked
	}

	block, err := aes.NewCipher(p.masterKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	result := make([]byte, 0, len(magicPrefix)+len(nonce)+len(ciphertext))
	result = append(result, magicPrefix...)
	result = append(result, nonce...)
	result = append(result, ciphertext...)
	return result, nil
}

// Unprotect decrypts ciphertext, supporting both current GCM format and legacy DPAPI fallback.
func (p *PortableProtector) Unprotect(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}

	if bytes.HasPrefix(data, magicPrefix) {
		p.mu.RLock()
		defer p.mu.RUnlock()

		if !p.unlocked || len(p.masterKey) != masterKeyLength {
			return nil, ErrLocked
		}

		payload := data[len(magicPrefix):]
		block, err := aes.NewCipher(p.masterKey)
		if err != nil {
			return nil, err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}

		nonceSize := gcm.NonceSize()
		if len(payload) < nonceSize {
			return nil, errors.New("ciphertext payload too short")
		}

		nonce := payload[:nonceSize]
		ciphertext := payload[nonceSize:]
		return gcm.Open(nil, nonce, ciphertext, nil)
	}

	// Legacy ciphertext fallback (e.g. DPAPI)
	if p.legacyDPAPI != nil {
		if plain, err := p.legacyDPAPI.Unprotect(data); err == nil {
			return plain, nil
		}
	}

	return nil, errors.New("unrecognized ciphertext format or decryption failed")
}
