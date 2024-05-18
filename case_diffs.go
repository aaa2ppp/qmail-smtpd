package main

import "strings"

func case_diffs(s, t string) bool {
	return !strings.EqualFold(s, t)
}
