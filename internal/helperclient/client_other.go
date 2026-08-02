//go:build !windows

package helperclient

import (
	"context"
	"errors"
)

type Session struct{}

func Launch(context.Context) (*Session, error) {
	return nil, errors.New("helper is supported only on Windows")
}
func (s *Session) Close() {}
