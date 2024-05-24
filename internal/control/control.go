package control

import (
	"bufio"
	"io"
	"os"
	"strings"

	"qmail-smtpd/internal/scan"
)

var me string
var meok bool

func Init() int {
	var r int
	me, r = ReadLine("control/me")
	if r == 1 {
		meok = true
	}
	return r
}

func ReadLine(fn string) (string, int) {
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

// TODO: rename ReadLineDef
func Rldef(fn string, flagme bool, def string) (string, int) {
	sa, r := ReadLine(fn)
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

func ReadInt(fn string) (int, int) {
	line, r := ReadLine(fn)
	switch r {
	case 0:
		return 0, 0
	case -1:
		return 0, -1
	}
	_, u := scan.ScanUlong(line)
	if u == 0 { // WTF?
		return 0, 0
	}
	return int(u), 1
}

func ReadFile(fn string, flagme bool) ([]string, int) {
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
		// if err == io.EOF && line == "" {
		// 	return sa, 1
		// }
		line = strings.TrimSpace(line)
		if len(line) > 0 && line[0] != '#' {
			sa = append(sa, line)
		}
		if err == io.EOF {
			return sa, 1
		}
	}
}
