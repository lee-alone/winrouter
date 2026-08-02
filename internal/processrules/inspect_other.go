//go:build !windows

package processrules

import "errors"

func Inspect([]Identity) ([]Status, error) {
	return nil, errors.New("process rule inspection is only supported on Windows")
}
