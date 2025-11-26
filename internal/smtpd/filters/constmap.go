package filters

import "strings"

type ConstMap map[string]struct{}

func NewConstMap(ss []string) ConstMap {
	cm := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		cm[strings.ToLower(s)] = struct{}{}
	}
	return cm
}

func (cm ConstMap) Contains(s string) bool {
	_, ok := cm[strings.ToLower(s)]
	return ok
}
