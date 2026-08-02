package autostart

import "testing"

func TestCommandForExecutableQuotesPathAndAddsStartupMarker(t *testing.T) {
	got := commandForExecutable(`C:\Program Files\WinRouter\WinRouter.exe`)
	want := `"C:\Program Files\WinRouter\WinRouter.exe" --autostart`
	if got != want {
		t.Fatalf("command = %q, want %q", got, want)
	}
}
