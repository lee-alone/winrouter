package app

import (
	"errors"

	"winrouter/internal/nodes"
)

func (a *App) GetSecurityStatus() nodes.SecurityStatus {
	a.mu.RLock()
	p := a.protector
	a.mu.RUnlock()
	if p == nil {
		return nodes.SecurityStatus{PINEnabled: false, Unlocked: true}
	}
	return p.Status()
}

func (a *App) UnlockSecurityVault(pin string) error {
	a.mu.RLock()
	p := a.protector
	a.mu.RUnlock()
	if p == nil {
		return errors.New("security vault not initialized")
	}
	return p.UnlockWithPIN(pin)
}

func (a *App) EnableSecurityPIN(pin string) error {
	a.mu.RLock()
	p := a.protector
	a.mu.RUnlock()
	if p == nil {
		return errors.New("security vault not initialized")
	}
	return p.EnablePIN(pin)
}

func (a *App) ChangeSecurityPIN(oldPIN, newPIN string) error {
	a.mu.RLock()
	p := a.protector
	a.mu.RUnlock()
	if p == nil {
		return errors.New("security vault not initialized")
	}
	return p.ChangePIN(oldPIN, newPIN)
}

func (a *App) DisableSecurityPIN(pin string) error {
	a.mu.RLock()
	p := a.protector
	a.mu.RUnlock()
	if p == nil {
		return errors.New("security vault not initialized")
	}
	return p.DisablePIN(pin)
}
