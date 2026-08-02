//go:build !windows

package singleinstance

import "errors"

var ErrAlreadyRunning = errors.New("WinRouter is already running")

type Lock struct{}

func Acquire() (*Lock, error)   { return &Lock{}, nil }
func (lock *Lock) Close() error { return nil }
