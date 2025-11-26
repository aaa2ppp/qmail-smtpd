package config

// void setup()
// {
//   char *x;
//   unsigned long u;

//   if (control_init() == -1) die_control();
//   if (control_rldef(&greeting,"control/smtpgreeting",1,(char *) 0) != 1)
//     die_control();
//   liphostok = control_rldef(&liphost,"control/localiphost",1,(char *) 0);
//   if (liphostok == -1) die_control();
//   if (control_readint(&timeout,"control/timeoutsmtpd") == -1) die_control();
//   if (timeout <= 0) timeout = 1;

//   if (rcpthosts_init() == -1) die_control();

//   bmfok = control_readfile(&bmf,"control/badmailfrom",0);
//   if (bmfok == -1) die_control();
//   if (bmfok)
//     if (!constmap_init(&mapbmf,bmf.s,bmf.len,0)) die_nomem();

//   if (control_readint(&databytes,"control/databytes") == -1) die_control();
//   x = env_get("DATABYTES");
//   if (x) { scan_ulong(x,&u); databytes = u; }
//   if (!(databytes + 1)) --databytes;

//   remoteip = env_get("TCPREMOTEIP");
//   if (!remoteip) remoteip = "unknown";
//   local = env_get("TCPLOCALHOST");
//   if (!local) local = env_get("TCPLOCALIP");
//   if (!local) local = "unknown";
//   remotehost = env_get("TCPREMOTEHOST");
//   if (!remotehost) remotehost = "unknown";
//   remoteinfo = env_get("TCPREMOTEINFO");
//   relayclient = env_get("RELAYCLIENT");
//   dohelo(remotehost);
// }

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"qmail-smtpd/internal/control"
	"qmail-smtpd/internal/smtpd"
	"qmail-smtpd/internal/smtpd/filters"
)

func Load(engine control.Engine) (*smtpd.Config, error) {
	ctrl, err := control.New(engine)
	if err != nil {
		return nil, err
	}

	cfg := &smtpd.Config{}

	if s, err := ctrl.ReadLineDef("control/greeting", true, ""); err != nil {
		return nil, fmt.Errorf("control/greeting: %w", err)
	} else {
		cfg.Greeting = s
	}

	if s, err := ctrl.ReadLineDef("control/localiphost", true, ""); err != nil {
		return nil, fmt.Errorf("control/localiphost: %w", err)
	} else {
		cfg.LocalIPHost = s
	}

	if i, err := ctrl.ReadInt("control/timeoutsmtpd"); err != nil && i > 0 {
		return nil, fmt.Errorf("control/timeoutsmtpd: %w", err)
	} else if i < 0 {
		return nil, errors.New("control/timeoutsmtpd: must be => 0")
	} else {
		cfg.Timeout = time.Duration(i) * time.Second
	}

	if i, err := ctrl.ReadInt("control/databytes"); err != nil {
		return nil, fmt.Errorf("control/databytes: %w", err)
	} else if i < 0 {
		return nil, errors.New("control/databytes: must be => 0")
	} else {
		cfg.Databytes = i
	}

	if ss, err := ctrl.ReadText("control/badmailfrom", false); err != nil {
		return nil, fmt.Errorf("control/badmailfrom: %w", err)
	} else {
		cfg.BadMailFrom = filters.NewConstMap(ss)
	}

	if ss, err := ctrl.ReadText("control/rcpthosts", false); err != nil {
		return nil, fmt.Errorf("control/rcpthosts: %w", err)
	} else {
		cfg.RcptHosts = filters.NewConstMap(ss)
	}

	if ss, err := ctrl.ReadText("control/mbxhosts", false); err != nil {
		return nil, fmt.Errorf("control/mbxhosts: %w", err)
	} else {
		cfg.MbxHosts = filters.NewConstMap(ss)
	}

	if s, err := ctrl.ReadLineDef("control/mbxzone", false, "at"); err != nil {
		return nil, fmt.Errorf("control/mbxzone: %w", err)
	} else {
		cfg.MbxZone = s
	}

	if i, err := ctrl.ReadInt("control/openrelay"); err != nil {
		return nil, fmt.Errorf("control/openrelay: %w", err)
	} else {
		cfg.OpenRealy = (i != 0)
	}

	if i, err := ctrl.ReadInt("control/smtplog"); err != nil {
		return nil, fmt.Errorf("control/smtplog: %w", err)
	} else {
		cfg.SMTPLog = (i != 0)
	}

	return cfg, nil
}

type Manager struct {
	engine control.Engine
	cfg    atomic.Value // *Config
}

func NewManager(engine control.Engine) (*Manager, error) {
	m := &Manager{engine: engine}
	if err := m.Reload(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manager) Get() *smtpd.Config {
	return m.cfg.Load().(*smtpd.Config)
}

func (m *Manager) Reload() error {
	cfg, err := Load(m.engine)
	if err != nil {
		return err
	}
	m.cfg.Store(cfg)
	return nil
}
