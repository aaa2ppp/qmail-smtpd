package filters

import (
	"context"
	"strings"
)

type BadMailFrom struct {
	cmap ConstMap
}

func NewBadMailFrom(cmap ConstMap) *BadMailFrom {
	return &BadMailFrom{cmap: cmap}
}

func (bmf *BadMailFrom) Match(_ context.Context, addr string) bool {
	if bmf.cmap == nil {
		return false
	}

	addr = strings.ToLower(addr)
	if bmf.cmap.Contains(addr) {
		return true
	}

	if j := strings.IndexByte(addr, '@'); j != -1 {
		if bmf.cmap.Contains(addr[j+1:]) {
			return true
		}
	}

	return false
}
