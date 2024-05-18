package main

import (
	"bufio"
	"strings"
)

type tCommands struct {
	text  string
	fun   func(string)
	flush func()
}

func commands(ss *bufio.Reader, c []tCommands) int {
	for {
		cmd, err := ss.ReadString('\n')
		if err != nil {
			return -1
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
			for ; c[i].text != ""; i++ {
				if strings.EqualFold(c[i].text, cmd) {
					break
				}
			}
			c[i].fun(arg)
			if c[i].flush != nil {
				c[i].flush()
			}
		}
	}
}
