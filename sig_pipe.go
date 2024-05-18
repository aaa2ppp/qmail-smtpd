package main

import (
	"os/signal"
	"syscall"
)

// void sig_pipeignore() { sig_catch(SIGPIPE,SIG_IGN); }
// void sig_pipedefault() { sig_catch(SIGPIPE,SIG_DFL); }

func sig_pipeignore()  { signal.Ignore(syscall.SIGPIPE) }
func sig_pipedefault() { signal.Reset(syscall.SIGPIPE) }
