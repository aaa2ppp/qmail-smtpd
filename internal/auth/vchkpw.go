package auth

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"slices"
)

// Implementation of protocol for external authentication command (vckpw):
//
// Credentials are passed via file descriptor 3 as three null-terminated
// strings in sequence: username, password, challenge (optional).
//
//   - PLAIN format: "authcid\x00passwd\x00\x00"
//   - CRAM-MD5 format: "username\x00hexdigest\x00challenge\x00"
//
// Max total length of credentials data must be less or equal than [VchkpwMaxAuthSize] bytes.
//
// Security: credentials are passed via pipe, not through command-line
// arguments or environment variables.
type VchkpwCommand struct {
	path string
	args []string
}

// vchkpw command exit codes:
//   - 0: authentication successful
//   - 1: forbidden (VLOG_ERROR_ACCESS)
//   - 3, ...: authentication failed (VLOG_ERROR_LOGON or VLOG_ERROR_PASSWD)
//   - Other: internal error (will be logged)
var VchkpwAuthFailed = []int{1, 3, 12, 13, 20, 21, 22, 23}

// Max total length of credentials data for vchkpw authentication.
// Credentials are passed as three null-terminated strings:
// "username\x00password\x00challenge\x00"
const VchkpwMaxAuthSize = 156

func NewVchkpwCommand(path string, args ...string) *VchkpwCommand {
	return &VchkpwCommand{
		path: path,
		args: args,
	}
}

// Authenticate verifies credentials using external vchkpw command.
//
// Returns:
//   - (Result, nil): Authentication process completed. Check [Result].Success
//     to determine if credentials were valid.
//   - (Result, error): Authentication failed due to backend error (command execution failed,
//     system error, etc.). The error indicates our internal problems, not invalid credentials.
func (a *VchkpwCommand) Authenticate(cred Credentials) (Result, error) {
	const op = "VchkpwCommand.Authenticate"

	var (
		username  string
		password  string
		challenge string
		zero      Result
	)

	switch cred := cred.(type) {
	case *PlainCredentials:
		username, password = cred.AuthCID, cred.Passwd
	case *CRAMCredentials:
		username, password, challenge = cred.Username, cred.Hexdigest, cred.Challenge
	default:
		log.Printf("%s: unknown credentials type %T", op, cred)
		return zero, ErrBackendUnavailable
	}

	ok, err := execVchkpw(a.path, a.args, username, password, challenge)
	if err != nil {
		log.Printf("%s: exec command failed: %v", op, err)
		return zero, ErrBackendUnavailable
	}
	if !ok {
		return zero, nil
	}

	return Result{Username: username, Success: true}, nil
}

// execVchkpw executes vchkpw command with given credentials.
//
// Returns:
//   - (true, nil): authentication successful
//   - (false, nil): authentication failed due to invalid credentials or access restrictions
//   - (false, error): command execution failed
func execVchkpw(path string, args []string, username, password, challenge string) (ok bool, err error) {
	const op = "execVchkpw"

	totalLength := len(username) + len(password) + len(challenge) + 3
	if totalLength > VchkpwMaxAuthSize {
		log.Printf("%s: credentials too long for vchkpw: %d > %d bytes",
			op, totalLength, VchkpwMaxAuthSize)
		return false, nil // this is client error, simulate authentication failed
	}

	cmd := exec.Command(path, args...)

	pr, pw, err := os.Pipe()
	if err != nil {
		return false, err
	}
	defer pr.Close()

	vchkpwSetupPipe(cmd, pr)

	if err := cmd.Start(); err != nil {
		pw.Close()
		return false, err
	}

	// Create and write data
	buf := bytes.NewBuffer(make([]byte, 0, VchkpwMaxAuthSize))
	buf.WriteString(username)
	buf.WriteByte(0)
	buf.WriteString(password)
	buf.WriteByte(0)
	buf.WriteString(challenge)
	buf.WriteByte(0)

	data := buf.Bytes()
	_, err = pw.Write(data)

	// Securely clear the buffer
	for i := range data {
		data[i] = 0
	}

	if err != nil {
		pw.Close()
		cmd.Process.Kill()
		cmd.Wait()
		return false, err
	}
	pw.Close()

	if err := cmd.Wait(); err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			return false, fmt.Errorf("cmd.Wait returned an unexpected error: %w", err)
		}
		code := exitErr.ProcessState.ExitCode()
		if code == -1 {
			return false, fmt.Errorf("%s terminated by signal", path)
		}
		if slices.Contains(VchkpwAuthFailed, code) { // forbidden or authentication failed
			return false, nil
		}
		return false, fmt.Errorf("%s exited with code %d", path, code)
	}

	return true, nil
}
