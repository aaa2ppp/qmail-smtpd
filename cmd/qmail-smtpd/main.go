package main

import (
	"cmp"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"qmail-smtpd/internal/auth"
	"qmail-smtpd/internal/config"
	"qmail-smtpd/internal/control"
	"qmail-smtpd/internal/control/badmailfrom"
	"qmail-smtpd/internal/control/mbxhosts"
	"qmail-smtpd/internal/control/rcpthosts"
	"qmail-smtpd/internal/ipme"
	log1 "qmail-smtpd/internal/log"
	"qmail-smtpd/internal/pipeconn"
	"qmail-smtpd/internal/qmail"
	"qmail-smtpd/internal/scan"
	"qmail-smtpd/internal/smtpd"
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
	log1.Writer
}

func (a *logAdapter) WithPrefix(prefix string) smtpd.LogWriter {
	return &logAdapter{log1.Writer{
		Out:    a.Out,
		Prefix: a.Prefix + prefix,
	}}
}

func main() {
	// void sig_pipeignore() { sig_catch(SIGPIPE,SIG_IGN); }
	signal.Ignore(syscall.SIGPIPE)

	if err := os.Chdir(config.AutoQmail); err != nil {
		log.Fatal(err)
	}

	cfg := prepareConfig()
	if len(os.Args) > 1 {
		cfg.Hostname = os.Args[1]
		path := os.Args[1]
		args := os.Args[2:]
		cfg.Auth = auth.NewVchkpwCommand(path, args...)
	}

	srv := smtpd.NewServer(cfg)

	relayClient, relayClientOk := os.LookupEnv("RELAYCLIENT")
	env := qmail.Env{
		LocalIP:       cmp.Or(os.Getenv("TCPLOCALIP"), "unknown"),
		LocalHost:     cmp.Or(os.Getenv("TCPLOCALHOST"), "unknown"),
		RemoteIP:      cmp.Or(os.Getenv("TCPREMOTEIP"), "unknown"),
		RemoteHost:    cmp.Or(os.Getenv("TCPREMOTEHOST"), "unknown"),
		RemoteInfo:    "", // ignoring
		RelayClient:   relayClient,
		RelayClientOk: relayClientOk,
	}

	conn := &pipeconn.Conn{
		Reader:   os.Stdin,
		Writer:   os.Stdout,
		LocalIP:  pipeconn.Addr(env.LocalIP),
		RemoteIP: pipeconn.Addr(env.RemoteIP),
	}

	if err := srv.Run(conn, env); err != nil {
		log.Fatalf("run failed: %v", err)
	}
}

func die_control() {
	os.Stdout.WriteString("421 unable to read controls (#4.3.0)\r\n")
	os.Exit(1)
}

func die_ipme() {
	os.Stdout.WriteString("421 unable to figure out my IP addresses (#4.3.0)\r\n")
	os.Exit(1)
}

func prepareConfig() *smtpd.Config {
	var cfg smtpd.Config

	if control.Init() == -1 {
		die_control()
	}

	if s, r := control.Rldef("control/smtpgreeting", true, ""); r != 1 {
		die_control()
	} else {
		cfg.Greeting = s
	}

	if s, r := control.Rldef("control/localiphost", true, ""); r == -1 {
		die_control()
	} else if r == 1 {
		cfg.LocalIPHost = s
	}

	cfg.Timeout = smtpd.DefaultTimeout
	if i, r := control.ReadInt("control/timeoutsmtpd"); r == -1 {
		die_control()
	} else if r == 1 {
		if i <= 0 {
			i = 1
		}
		cfg.Timeout = time.Duration(i) * time.Second
	}

	if r := rcpthosts.Init(); r == -1 {
		die_control()
	} else if r == 1 {
		cfg.RcptHosts = rcpthostsAdapter{}
	}

	if r := badmailfrom.Init(); r == -1 {
		die_control()
	} else if r == 1 {
		cfg.BadMailFrom = badmailfromAdapter{}
	}

	if r := mbxhosts.Init(); r == -1 {
		die_control()
	} else if r == 1 {
		cfg.MbxHosts = mbxhostsAdapter{}
	}

	if i, r := control.ReadInt("control/databytes"); r == -1 {
		die_control()
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
		die_ipme()
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
		die_control()
	} else {
		logEnable = i != 0
	}
	if x := os.Getenv("SMTPLOG"); x != "" {
		_, u := scan.ScanUlong(x)
		logEnable = u != 0
	}
	if logEnable {
		pid := os.Getpid()
		cfg.Logger = &logAdapter{log1.Writer{
			Out:    os.Stderr,
			Prefix: fmt.Sprintf("smtp-log[%d]: ", pid),
		}}
	}

	return &cfg
}
