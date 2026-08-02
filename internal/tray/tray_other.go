//go:build !windows

package tray

type Actions struct {
	Show  func()
	Start func()
	Stop  func()
	Quit  func()
}

type Controller struct{}

func (*Controller) Start([]byte, Actions)  {}
func (*Controller) Stop()                  {}
func (*Controller) SetStatus(string, bool) {}
