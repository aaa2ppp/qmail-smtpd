package control

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
)

func readLine(r io.Reader) (string, error) {
	br := bufio.NewReader(r)

	line, err := br.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}

	return strings.TrimRight(line, " \t\r\n"), nil
}

func readText(r io.Reader) ([]string, error) {
	sc := bufio.NewScanner(r)

	var lines []string
	for sc.Scan() {
		s := strings.TrimRight(sc.Text(), " \t\r\n")
		if s != "" && s[0] != '#' {
			lines = append(lines, s)
		}
	}

	if err := sc.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}

type Engine interface {
	ReadLine(name string) (string, bool, error)
	ReadText(name string) ([]string, bool, error)
}

type FileEngine struct{}

func (FileEngine) ReadLine(name string) (string, bool, error) {
	fd, err := os.Open(name)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	defer fd.Close()

	s, err := readLine(fd)
	return s, true, err
}

func (FileEngine) ReadText(name string) ([]string, bool, error) {
	fd, err := os.Open(name)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	defer fd.Close()

	ss, err := readText(fd)
	return ss, true, err
}

type Control struct {
	engine Engine
	me     string
}

func New(engine Engine) (*Control, error) {
	me, _, err := engine.ReadLine("control/me")
	if err != nil {
		return nil, err
	}
	if me == "" {
		return nil, errors.New("control/me: must be non-empty")
	}
	return &Control{engine: engine, me: me}, nil
}

func (c *Control) Me() string {
	return c.me
}

func (c *Control) ReadLineDef(name string, flagme bool, def string) (string, error) {
	s, exists, err := c.engine.ReadLine(name)
	if err != nil {
		return "", err
	}
	if exists {
		return s, err
	}
	if flagme {
		return c.me, nil
	}
	return def, nil
}

func (c *Control) ReadInt(name string) (int, error) {
	s, exists, err := c.engine.ReadLine(name)
	if err != nil {
		return 0, err
	}
	if exists {
		i, err := strconv.Atoi(s)
		if err != nil {
			return 0, err
		}
		return i, nil
	}
	return 0, nil
}

func (c *Control) ReadText(name string, flagme bool) ([]string, error) {
	text, exists, err := c.engine.ReadText(name)
	if err != nil {
		return nil, err
	}
	if !exists && flagme {
		return []string{c.me}, nil
	}
	return text, nil
}
