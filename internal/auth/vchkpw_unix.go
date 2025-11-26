//go:build unix

package auth

import (
	"os"
	"os/exec"
)

func vchkpwSetupPipe(cmd *exec.Cmd, pr *os.File) {
	cmd.ExtraFiles = []*os.File{pr} // ExtraFiles[0] becomes fd 3 in the child process
}
