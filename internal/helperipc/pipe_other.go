//go:build !windows

package helperipc

import (
	"context"
	"errors"
	"net"
	"time"
)

func CurrentUserSID() (string, error) {
	return "", errors.New("named pipe RPC is supported only on Windows")
}
func PipeNameForSID(string) (string, error) {
	return "", errors.New("named pipe RPC is supported only on Windows")
}
func ListenCurrentUserPipe() (net.Listener, string, error) {
	return nil, "", errors.New("named pipe RPC is supported only on Windows")
}
func DialCurrentUserPipe(context.Context) (net.Conn, error) {
	return nil, errors.New("named pipe RPC is supported only on Windows")
}
func IsElevated() bool { return false }
func DefaultDialer(time.Duration) func() (net.Conn, error) {
	return func() (net.Conn, error) { return nil, errors.New("named pipe RPC is supported only on Windows") }
}
