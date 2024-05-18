package main

import (
	"bufio"
	"io"
	"os"
	"os/exec"
)

var binqqargs = []string{"bin/qmail-queue"}

type tQmail struct {
	cmd     *exec.Cmd
	flagerr bool
	//pid     uint
	fdm *os.File
	fde *os.File
	ss  *bufio.Writer
}

func qmail_open(qq *tQmail) int {
	var pi io.WriteCloser
	var po io.ReadCloser
	var err error
	qq.cmd = exec.Command(binqqargs[0], binqqargs[1:]...)
	qq.cmd.Dir = auto_qmail

	// fd(0)
	if pi, err = qq.cmd.StdinPipe(); err != nil {
		return -1
	}
	// fd(1)
	if po, err = qq.cmd.StdoutPipe(); err != nil {
		return -1
	}

	// XXX: qmail-queue reads from fd(0) and (!) from fd(1)
	// to possible write to both fd we convert pi and po interfaces to os.File
	qq.fdm = pi.(*os.File)
	qq.fde = po.(*os.File)

	qq.ss = bufio.NewWriter(qq.fdm)

	if err = qq.cmd.Start(); err != nil {
		return -1
	}
	return 0
}

func qmail_qp(qq *tQmail) int {
	return qq.cmd.Process.Pid
}

func qmail_fail(qq *tQmail) bool {
	return qq.flagerr
}

func qmail_puts(qq *tQmail, s string) {
	if !qq.flagerr {
		if _, err := qq.ss.WriteString(s); err != nil {
			qq.flagerr = true
		}
	}
}

func qmail_putc(qq *tQmail, ch byte) {
	if !qq.flagerr {
		if err := qq.ss.WriteByte(ch); err != nil {
			qq.flagerr = true
		}
	}
}

func qmail_from(qq *tQmail, s string) {
	if err := qq.ss.Flush(); err != nil {
		qq.flagerr = true
	}
	qq.fdm.Close()

	qq.ss = bufio.NewWriter(qq.fde)
	qmail_putc(qq, 'F')
	qmail_puts(qq, s)
	qmail_putc(qq, 0)

	qq.flagerr = true
}

func qmail_to(qq *tQmail, s string) {
	qmail_putc(qq, 'T')
	qmail_puts(qq, s)
	qmail_putc(qq, 0)
}

func qmail_close(qq *tQmail) string {
	qmail_puts(qq, "\x00")
	if !qq.flagerr {
		if err := qq.ss.Flush(); err != nil {
			qq.flagerr = true
		}
	}
	qq.fde.Close()

	// if (wait_pid(&wstat,qq->pid) != qq->pid)
	// 	return "Zqq waitpid surprise (#4.3.0)"; // WTF?
	// if (wait_crashed(wstat))
	// 	return "Zqq crashed (#4.3.0)";
	if err := qq.cmd.Wait(); err != nil {
		return "Zqq crashed (#4.3.0)"
	}

	exitcode := qq.cmd.ProcessState.ExitCode()
	switch exitcode {
	case 115: /* compatibility */
		fallthrough
	case 11:
		return "Denvelope address too long for qq (#5.1.3)"
	case 31:
		return "Dmail server permanently rejected message (#5.3.0)"
	case 51:
		return "Zqq out of memory (#4.3.0)"
	case 52:
		return "Zqq timeout (#4.3.0)"
	case 53:
		return "Zqq write error or disk full (#4.3.0)"
	case 0:
		if !qq.flagerr {
			return ""
		}
		fallthrough
	case 54:
		return "Zqq read error (#4.3.0)"
	case 55:
		return "Zqq unable to read configuration (#4.3.0)"
	case 56:
		return "Zqq trouble making network connection (#4.3.0)"
	case 61:
		return "Zqq trouble in home directory (#4.3.0)"
	case 63:
		fallthrough
	case 64:
		fallthrough
	case 65:
		fallthrough
	case 66:
		fallthrough
	case 62:
		return "Zqq trouble creating files in queue (#4.3.0)"
	case 71:
		return "Zmail server temporarily rejected message (#4.3.0)"
	case 72:
		return "Zconnection to mail server timed out (#4.4.1)"
	case 73:
		return "Zconnection to mail server rejected (#4.4.1)"
	case 74:
		return "Zcommunication with mail server failed (#4.4.2)"
	case 91:
		fallthrough
	case 81:
		return "Zqq internal bug (#4.3.0)"
	case 120:
		return "Zunable to exec qq (#4.3.0)"
	}
	if (exitcode >= 11) && (exitcode <= 40) {
		return "Dqq permanent problem (#5.3.0)"
	}
	return "Zqq temporary problem (#4.3.0)"
}
