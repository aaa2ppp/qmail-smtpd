package todo

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"qmail-smtpd/internal/qmail"
	"qmail-smtpd/internal/smtpd"
	"qmail-smtpd/internal/todo/control"
	"qmail-smtpd/internal/todo/control/badmailfrom"
	"qmail-smtpd/internal/todo/control/mbxhosts"
	"qmail-smtpd/internal/todo/control/rcpthosts"
	"qmail-smtpd/internal/todo/ipme"
	"qmail-smtpd/internal/todo/scan"
	"qmail-smtpd/internal/todo/smtplog"
)

type qmailAdapter struct{}

func (qa qmailAdapter) Begin(fromMail string, rcptTo []string, env qmail.Env) (smtpd.Queue, error) {
	return qmail.Begin(fromMail, rcptTo, env)
}

type rcpthostsAdapter struct{}

func (a rcpthostsAdapter) Match(addr string) bool {
	return rcpthosts.Match(addr)
}

type badmailfromAdapter struct{}

func (a badmailfromAdapter) Match(addr string) bool {
	return badmailfrom.Match(addr)
}

type mbxhostsAdapter struct{}

func (a mbxhostsAdapter) Match(addr string) bool {
	return mbxhosts.Match(addr)
}

type ipmeAdapter struct{}

func (a ipmeAdapter) Is(ip scan.IPAddress) bool {
	return ipme.Is(ip)
}

type logAdapter struct {
	smtplog.Writer
}

func (a *logAdapter) WithPrefix(prefix string) smtpd.LogWriter {
	return &logAdapter{smtplog.Writer{
		Out:    a.Out,
		Prefix: a.Prefix + prefix,
	}}
}

var (
	ErrControl = errors.New("unable to read controls")
	ErrIPMe    = errors.New("unable to figure out my IP addresses")
)

func LoadQmailConfig() (*smtpd.Config, error) {
	var cfg smtpd.Config

	if control.Init() == -1 {
		return nil, ErrControl
	}

	cfg.Me = control.Me()
	cfg.AuthFQDN = control.Me()

	if s, r := control.Rldef("control/smtpgreeting", true, ""); r != 1 {
		return nil, ErrControl
	} else {
		cfg.Greeting = s
	}

	if s, r := control.Rldef("control/localiphost", true, ""); r == -1 {
		return nil, ErrControl
	} else if r == 1 {
		cfg.LocalIPHost = s
	}

	cfg.Timeout = smtpd.DefaultTimeout
	if i, r := control.ReadInt("control/timeoutsmtpd"); r == -1 {
		return nil, ErrControl
	} else if r == 1 {
		if i <= 0 {
			i = 1
		}
		cfg.Timeout = time.Duration(i) * time.Second
	}

	if r := rcpthosts.Init(); r == -1 {
		return nil, ErrControl
	} else if r == 1 {
		cfg.RcptHosts = rcpthostsAdapter{}
	}

	if r := badmailfrom.Init(); r == -1 {
		return nil, ErrControl
	} else if r == 1 {
		cfg.BadMailFrom = badmailfromAdapter{}
	}

	if r := mbxhosts.Init(); r == -1 {
		return nil, ErrControl
	} else if r == 1 {
		cfg.MbxHosts = mbxhostsAdapter{}
	}

	if i, r := control.ReadInt("control/databytes"); r == -1 {
		return nil, ErrControl
	} else if r == 1 {
		cfg.Databytes = i
	}

	// x = env_get("DATABYTES");
	// if (x) { scan_ulong(x,&u); databytes = u; }
	// if (!(databytes + 1)) --databytes;  // WTF: if databytes == -1 then databytes = -2 ?
	if x := os.Getenv("DATABYTES"); x != "" {
		_, u := scan.ScanUlong(x)
		if u != 0 {
			cfg.Databytes = int(u)
		}
	}
	if cfg.Databytes+1 == 0 { // WTF?
		cfg.Databytes--
	}

	if !ipme.Init() {
		return nil, ErrIPMe
	}
	cfg.IPMe = ipmeAdapter{}

	cfg.Qmail = qmailAdapter{}

	cert, err := tls.LoadX509KeyPair("control/servercert.pem", "control/servercert.pem")
	if err != nil {
		log.Fatal(err)
	}

	cfg.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}

	var logEnable bool
	if i, r := control.ReadInt("control/smtplog"); r == -1 {
		return nil, ErrControl
	} else {
		logEnable = i != 0
	}
	if x := os.Getenv("SMTPLOG"); x != "" {
		_, u := scan.ScanUlong(x)
		logEnable = u != 0
	}

	if logEnable {
		pid := os.Getpid()
		cfg.Logger = &logAdapter{smtplog.Writer{
			Out:    os.Stderr,
			Prefix: fmt.Sprintf("smtp-log[%d]: ", pid),
		}}
	}

	return &cfg, nil
}
