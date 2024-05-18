package main

// XXX: for now, set is enough for us

type tConstmap map[string]struct{}

func constmap_init(cm tConstmap, ss []string) {
	for _, s := range ss {
		cm[s] = struct{}{}
	}
}

func constmap(cm tConstmap, s string) bool {
	_, ok := cm[s]
	return ok
}
