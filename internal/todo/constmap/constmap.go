package constmap

// XXX: for now, set is enough for us

type Constmap map[string]struct{}

func New(ss []string) Constmap {
	cm := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		cm[s] = struct{}{}
	}
	return cm
}

func (cm Constmap) Contains(s string) bool {
	_, ok := cm[s]
	return ok
}
