package qmail

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
)

var binqqargs = []string{"bin/qmail-queue"}

type Qmail struct {
	cmd       *exec.Cmd
	fdm       *os.File
	fde       *os.File
	ss        *bufio.Writer
	stickyErr error
}

func Open() (qq *Qmail, err error) {
	var (
		cmd *exec.Cmd
		fdm *os.File
		fde *os.File
	)

	defer func() {
		if err != nil {
			cmd = nil
			if fdm != nil {
				fdm.Close()
			}
			if fde != nil {
				fde.Close()
			}
		}
	}()

	cmd = exec.Command(binqqargs[0], binqqargs[1:]...)

	{
		pr, pw, err := os.Pipe()
		if err != nil {
			return nil, err
		}
		defer pr.Close() // TODO: close pw on children side
		cmd.Stdin = pr
		fdm = pw
	}

	{
		pr, pw, err := os.Pipe()
		if err != nil {
			return nil, err
		}
		defer pr.Close() // TODO: close pw on children side
		cmd.Stdout = pr  // yes, qmail-queue reads from fd=1 (stdout)
		fde = pw
	}

	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return &Qmail{
		cmd: cmd,
		fdm: fdm,
		fde: fde,
		ss:  bufio.NewWriter(fdm),
	}, err
}

func (qq *Qmail) Pid() int {
	return qq.cmd.Process.Pid
}

func (qq *Qmail) Err() error {
	return qq.stickyErr
}

func (qq *Qmail) Fail() {
	qq.stickyErr = errors.New("calling code error")
	// XXX: В результате мы не зафлешим тело и/или конверт (последнее сигдал для qmail-queue?).
	// А на выходе получим ошибку "Zqq read error (#4.3.0)". Но это пофиг, т.к вызывающий код сам
	// знает что случилось. Как-то корявинько и неявненько
}

func (qq *Qmail) Puts(s string) {
	if qq.stickyErr == nil {
		if _, err := qq.ss.WriteString(s); err != nil {
			qq.stickyErr = err
		}
	}
}

func (qq *Qmail) Putc(ch byte) {
	if qq.stickyErr == nil {
		if err := qq.ss.WriteByte(ch); err != nil {
			qq.stickyErr = err
		}
	}
}

func (qq *Qmail) From(s string) {
	if err := qq.ss.Flush(); err != nil {
		qq.stickyErr = err
	}
	qq.fdm.Close()
	qq.ss = bufio.NewWriter(qq.fde)

	qq.Putc('F')
	qq.Puts(s)
	qq.Putc(0)
}

func (qq *Qmail) To(s string) {
	qq.Putc('T')
	qq.Puts(s)
	qq.Putc(0)
}

func (qq *Qmail) Close() string {
	qq.Putc(0)
	if qq.stickyErr == nil {
		if err := qq.ss.Flush(); err != nil {
			qq.stickyErr = err
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
		if qq.stickyErr == nil {
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
