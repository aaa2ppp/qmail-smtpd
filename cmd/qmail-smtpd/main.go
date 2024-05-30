package main

import (
	"crypto/tls"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"qmail-smtpd/internal/config"
	"qmail-smtpd/internal/conn"
	"qmail-smtpd/internal/control"
	"qmail-smtpd/internal/control/badmailfrom"
	"qmail-smtpd/internal/control/rcpthosts"
	"qmail-smtpd/internal/ipme"
	"qmail-smtpd/internal/qmail"
	"qmail-smtpd/internal/scan"
	"qmail-smtpd/internal/smtpd"
)

type qmailAdapter struct{}

func (qa qmailAdapter) Open() (smtpd.QmailQueue, error) {
	return qmail.Open()
}

type rcpthostsAdapter struct{}

func (a rcpthostsAdapter) Match(addr string) bool {
	return rcpthosts.Match(addr)
}

type badmailfromAdapter struct{}

func (a badmailfromAdapter) Match(addr string) bool {
	return badmailfrom.Match(addr)
}

type ipmeAdapter struct{}

func (a ipmeAdapter) Is(ip scan.IPAddress) bool {
	return ipme.Is(ip)
}

func main() {
	// void sig_pipeignore() { sig_catch(SIGPIPE,SIG_IGN); }
	signal.Ignore(syscall.SIGPIPE)

	if err := os.Chdir(config.AutoQmail); err != nil {
		log.Fatal(err)
	}

	d := mustSetupSmtpd()
	c := &conn.Conn{
		Reader:   os.Stdin,
		Writer:   os.Stdout,
		LocalIP:  conn.Addr(d.LocalIP),
		RemoteIP: conn.Addr(d.RemoteIP),
	}

	if err := d.Run(c); err != nil {
		log.Fatal(err)
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

func mustSetupSmtpd() *smtpd.Smtpd {
	var d smtpd.Smtpd

	if control.Init() == -1 {
		die_control()
	}

	if s, r := control.Rldef("control/smtpgreeting", true, ""); r != 1 {
		die_control()
	} else {
		d.Greeting = s
	}

	if s, r := control.Rldef("control/localiphost", true, ""); r == -1 {
		die_control()
	} else if r == 1 {
		d.LocalIPHost = s
	}

	d.Timeout = smtpd.DefaultTimeout
	if i, r := control.ReadInt("control/timeoutsmtpd"); r == -1 {
		die_control()
	} else if r == 1 {
		if i <= 0 {
			i = 1
		}
		d.Timeout = time.Duration(i) * time.Second
	}

	if r := rcpthosts.Init(); r == -1 {
		die_control()
	} else if r == 1 {
		d.RcptHosts = rcpthostsAdapter{}
	}

	if r := badmailfrom.Init(); r == -1 {
		die_control()
	} else if r == 1 {
		d.BadMailFrom = badmailfromAdapter{}
	}

	if i, r := control.ReadInt("control/databytes"); r == -1 {
		die_control()
	} else if r == 1 {
		d.Databytes = i
	}

	// x = env_get("DATABYTES");
	// if (x) { scan_ulong(x,&u); databytes = u; }
	// if (!(databytes + 1)) --databytes;  // WTF: if databytes == -1 then databytes = -2 ?
	if x := os.Getenv("DATABYTES"); x != "" {
		_, u := scan.ScanUlong(x)
		if u != 0 {
			d.Databytes = int(u)
		}
	}
	if d.Databytes+1 == 0 { // WTF?
		d.Databytes--
	}

	d.LocalIP = os.Getenv("TCPLOCALIP")
	if d.LocalIP == "" {
		d.LocalIP = "unknown"
	}

	d.LocalHost = os.Getenv("TCPLOCALHOST")
	if d.LocalHost == "" {
		d.LocalHost = d.LocalIP
	}

	d.RemoteIP = os.Getenv("TCPREMOTEIP")
	if d.RemoteIP == "" {
		d.RemoteIP = "unknown"
	}

	d.RemoteHost = os.Getenv("TCPREMOTEHOST")
	if d.RemoteHost == "" {
		d.RemoteHost = "unknown"
	}

	d.RemoteInfo = os.Getenv("TCPREMOTEINFO")
	d.RelayClient, d.RelayClientOk = os.LookupEnv("RELAYCLIENT")

	if !ipme.Init() {
		die_ipme()
	}
	d.IPMe = ipmeAdapter{}

	d.Qmail = qmailAdapter{}

	cert, err := tls.LoadX509KeyPair("control/servercert.pem", "control/servercert.pem")
	if err != nil {
		log.Fatal(err)
	}

	d.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}

	return &d
}
