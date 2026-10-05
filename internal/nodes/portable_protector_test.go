package nodes

import (
	"bytes"
	"testing"
)

type dummyLegacyProtector struct{}

func (dummyLegacyProtector) Protect(data []byte) ([]byte, error) {
	return append([]byte("dpapi:"), data...), nil
}

func (dummyLegacyProtector) Unprotect(data []byte) ([]byte, error) {
	if bytes.HasPrefix(data, []byte("dpapi:")) {
		return bytes.TrimPrefix(data, []byte("dpapi:")), nil
	}
	return nil, ErrKeyCorrupted
}

func TestPortableProtectorBasic(t *testing.T) {
	dir := t.TempDir()
	p, err := NewPortableProtector(dir, dummyLegacyProtector{})
	if err != nil {
		t.Fatalf("NewPortableProtector failed: %v", err)
	}

	status := p.Status()
	if status.PINEnabled || !status.Unlocked {
		t.Fatalf("expected PINEnabled=false, Unlocked=true, got %+v", status)
	}

	plaintext := []byte("super-secret-password-123")
	ciphertext, err := p.Protect(plaintext)
	if err != nil {
		t.Fatalf("Protect failed: %v", err)
	}

	decrypted, err := p.Unprotect(ciphertext)
	if err != nil {
		t.Fatalf("Unprotect failed: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted %s != original %s", decrypted, plaintext)
	}

	// Test reload from existing master.key
	reloaded, err := NewPortableProtector(dir, dummyLegacyProtector{})
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	decrypted2, err := reloaded.Unprotect(ciphertext)
	if err != nil {
		t.Fatalf("Unprotect on reloaded failed: %v", err)
	}
	if !bytes.Equal(decrypted2, plaintext) {
		t.Fatalf("decrypted on reloaded %s != original %s", decrypted2, plaintext)
	}
}

func TestPortableProtectorPINFlow(t *testing.T) {
	dir := t.TempDir()
	p, err := NewPortableProtector(dir, dummyLegacyProtector{})
	if err != nil {
		t.Fatal(err)
	}

	plaintext := []byte("node-uuid-550e8400-e29b-41d4-a716-446655440000")
	ciphertext, err := p.Protect(plaintext)
	if err != nil {
		t.Fatal(err)
	}

	// Enable PIN
	pin := "MyCustomPIN#8899"
	if err := p.EnablePIN(pin); err != nil {
		t.Fatalf("EnablePIN failed: %v", err)
	}

	status := p.Status()
	if !status.PINEnabled || !status.Unlocked {
		t.Fatalf("expected PINEnabled=true, Unlocked=true, got %+v", status)
	}

	// Reload (simulate app restarting with PIN enabled)
	locked, err := NewPortableProtector(dir, dummyLegacyProtector{})
	if err != nil {
		t.Fatal(err)
	}
	lockedStatus := locked.Status()
	if !lockedStatus.PINEnabled || lockedStatus.Unlocked {
		t.Fatalf("expected PINEnabled=true, Unlocked=false on reload, got %+v", lockedStatus)
	}

	// Decrypt while locked should fail
	if _, err := locked.Unprotect(ciphertext); err != ErrLocked {
		t.Fatalf("expected ErrLocked, got %v", err)
	}

	// Unlock with wrong PIN
	if err := locked.UnlockWithPIN("WrongPIN"); err != ErrInvalidPIN {
		t.Fatalf("expected ErrInvalidPIN, got %v", err)
	}

	// Unlock with correct PIN
	if err := locked.UnlockWithPIN(pin); err != nil {
		t.Fatalf("UnlockWithPIN failed: %v", err)
	}
	if !locked.Status().Unlocked {
		t.Fatalf("expected Unlocked=true")
	}

	// Decrypt should succeed now
	decrypted, err := locked.Unprotect(ciphertext)
	if err != nil {
		t.Fatalf("Unprotect failed: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted %s != original %s", decrypted, plaintext)
	}

	// Change PIN
	newPIN := "NewPIN2026!#"
	if err := locked.ChangePIN(pin, newPIN); err != nil {
		t.Fatalf("ChangePIN failed: %v", err)
	}

	// Test reload with new PIN
	locked2, err := NewPortableProtector(dir, dummyLegacyProtector{})
	if err != nil {
		t.Fatal(err)
	}
	if err := locked2.UnlockWithPIN(newPIN); err != nil {
		t.Fatalf("UnlockWithPIN with newPIN failed: %v", err)
	}

	// Disable PIN
	if err := locked2.DisablePIN(newPIN); err != nil {
		t.Fatalf("DisablePIN failed: %v", err)
	}
	if locked2.Status().PINEnabled {
		t.Fatalf("expected PINEnabled=false after disable")
	}

	// Reload without PIN
	unlocked, err := NewPortableProtector(dir, dummyLegacyProtector{})
	if err != nil {
		t.Fatal(err)
	}
	if unlocked.Status().PINEnabled || !unlocked.Status().Unlocked {
		t.Fatalf("expected clean unsealed state")
	}
}

func TestPortableProtectorLegacyFallback(t *testing.T) {
	dir := t.TempDir()
	p, err := NewPortableProtector(dir, dummyLegacyProtector{})
	if err != nil {
		t.Fatal(err)
	}

	legacyCiphertext := []byte("dpapi:legacy-secret-content")
	decrypted, err := p.Unprotect(legacyCiphertext)
	if err != nil {
		t.Fatalf("Unprotect legacy failed: %v", err)
	}
	if string(decrypted) != "legacy-secret-content" {
		t.Fatalf("unexpected legacy decrypted content: %s", string(decrypted))
	}
}
