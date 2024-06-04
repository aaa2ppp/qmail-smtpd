package auth

import (
	"bytes"
	"os"
	"os/exec"
)

func Authenticate(childargs []string, user, pass, resp string) (ok bool, err error) {
	cmd := exec.Command(childargs[0], childargs[1:]...)

	pr, pw, err := os.Pipe()
	if err != nil {
		return false, err
	}
	cmd.ExtraFiles = []*os.File{pr}

	if err := cmd.Start(); err != nil {
		return false, err
	}
	defer func() {
		// TODO: wait timeout
		if e := cmd.Wait(); e == nil {
			if err == nil {
				err = e
			}
		}
		ok = err == nil
	}()

	var buf bytes.Buffer
	buf.WriteString(user)
	buf.WriteByte(0)
	buf.WriteString(pass)
	buf.WriteByte(0)
	buf.WriteString(resp)
	buf.WriteByte(0)
	if _, err := buf.WriteTo(pw); err != nil {
		return false, err
	}
	return 
}
