//go:build !windows

package nodes

import (
	"os/exec"
)

func setNoWindowSysProcAttr(cmd *exec.Cmd) {}
