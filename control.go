package main

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"
)

var me string
var meok bool

func control_init() int {
	var r int
	me, r = control_readline("control/me")
	if r == 1 {
		meok = true
	}
	return r
}

func control_readline(fn string) (string, int) {
	fd, err := os.Open(fn)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", -1
		}
		return "", 0
	}
	defer fd.Close()

	br := bufio.NewReader(fd)

	sa, err := br.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", -1
	}

	sa = strings.TrimSpace(sa)
	return sa, 1
}

func control_rldef(fn string, flagme bool, def string) (string, int) {
	sa, r := control_readline(fn)
	if r != 0 {
		return sa, r
	}
	if flagme && meok {
		return me, 1
	}
	if def != "" {
		return def, 1
	}
	return "", 0
}

func control_readint(fn string) (int, int) {
	line, r := control_readline(fn)
	switch r {
	case 0:
		return 0, 0
	case -1:
		return 0, -1
	}
	u, err := strconv.ParseUint(line, 0, 32) // xxx
	if err != nil {
		return 0, 0
	}
	return int(u), 1
}

func control_readfile(fn string, flagme bool) ([]string, int) {
	fd, err := os.Open(fn)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, -1
		}
		if flagme && meok {
			return []string{me}, 1
		}
		return nil, 0
	}
	defer fd.Close()
	br := bufio.NewReader(fd)

	var sa []string
	for {
		line, err := br.ReadString('\n')
		if err != nil && err != io.EOF {
			return sa, -1
		}
		if err == io.EOF && line == "" {
			return sa, 1
		}
		line = strings.TrimSpace(line)
		if len(line) > 0 && line[0] != '#' {
			sa = append(sa, line)
		}
		if err == io.EOF {
			return sa, 1
		}
	}

	//return nil, -1
}
