//go:build windows

package helperipc

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

func CurrentUserSID() (string, error) {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return "", err
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return "", err
	}
	return user.User.Sid.String(), nil
}

func PipeNameForSID(sid string) (string, error) {
	if !strings.HasPrefix(sid, "S-1-") || strings.ContainsAny(sid, `\/:*?"<>| `) {
		return "", fmt.Errorf("invalid Windows SID %q", sid)
	}
	return `\\.\pipe\WinRouter-` + sid, nil
}

func ListenCurrentUserPipe() (net.Listener, string, error) {
	sid, err := CurrentUserSID()
	if err != nil {
		return nil, "", err
	}
	name, err := PipeNameForSID(sid)
	if err != nil {
		return nil, "", err
	}
	sddl := "D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GRGW;;;" + sid + ")"
	listener, err := winio.ListenPipe(name, &winio.PipeConfig{SecurityDescriptor: sddl, MessageMode: false, InputBufferSize: 64 * 1024, OutputBufferSize: 64 * 1024})
	if err != nil {
		return nil, "", fmt.Errorf("listen helper pipe: %w", err)
	}
	return listener, name, nil
}

func DialCurrentUserPipe(ctx context.Context) (net.Conn, error) {
	sid, err := CurrentUserSID()
	if err != nil {
		return nil, err
	}
	name, err := PipeNameForSID(sid)
	if err != nil {
		return nil, err
	}
	return winio.DialPipeContext(ctx, name)
}

func IsElevated() bool { return windows.GetCurrentProcessToken().IsElevated() }

func DefaultDialer(timeout time.Duration) func() (net.Conn, error) {
	return func() (net.Conn, error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		return DialCurrentUserPipe(ctx)
	}
}
