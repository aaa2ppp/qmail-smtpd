//go:build !unix

package auth

import (
	"os"
	"os/exec"
)

func vchkpwSetupPipe(cmd *exec.Cmd, pr *os.File) {
	cmd.Stdin = pr
}
