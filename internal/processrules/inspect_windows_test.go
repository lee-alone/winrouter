//go:build windows

package processrules

import "testing"

func TestInspectDetectsSameNameAmbiguityAndExactPath(t *testing.T) {
	processes := []process{{name: "app.exe", path: `C:\One\app.exe`}, {name: "app.exe", path: `D:\Two\app.exe`}}
	status := inspect(Identity{Type: "process-name", Value: "APP.EXE"}, processes)
	if status.State != "ambiguous" || status.Matches != 2 {
		t.Fatalf("name status = %#v", status)
	}
	status = inspect(Identity{Type: "process-path", Value: `D:\Two\app.exe`}, processes)
	if status.State != "matched" || status.Matches != 1 {
		t.Fatalf("path status = %#v", status)
	}
}
