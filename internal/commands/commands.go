package commands

import (
	"strings"
)

type Command struct {
	Name  string
	Fun   func(arg string)
	Flush func()
}

type StringReader interface {
	ReadString(byte) (string, error)
}

func Loop(r StringReader, c []Command) error {
	if len(c) == 0 || c[len(c)-1].Name != "" {
		panic("name of last command must be empty")
	}

	for {
		cmd, err := r.ReadString('\n')
		if err != nil {
			return err
		}

		cmd = cmd[:len(cmd)-1]

		if len(cmd) > 0 && cmd[len(cmd)-1] == '\r' {
			cmd = cmd[:len(cmd)-1]
		}

		i := strings.IndexByte(cmd, ' ')
		if i == -1 {
			i = len(cmd)
		}

		arg := cmd[i:]
		for len(arg) > 0 && arg[0] == ' ' {
			arg = arg[1:]
		}
		cmd = cmd[:i]

		{
			i := 0 // xxx
			for ; c[i].Name != ""; i++ {
				if strings.EqualFold(c[i].Name, cmd) {
					break
				}
			}
			c[i].Fun(arg)
			if c[i].Flush != nil {
				c[i].Flush()
			}
		}
	}
}
