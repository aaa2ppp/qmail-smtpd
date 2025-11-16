package server

type semaphore chan struct{}

func makeSemaphore(n int) semaphore {
	if n <= 0 {
		panic("newSemaphore: n must be > 0")
	}
	s := make(semaphore, n)
	for range n {
		s <- struct{}{}
	}
	return s
}

func (s semaphore) acquire() bool {
	if s == nil {
		return true
	}
	select {
	case <-s:
		return true
	default:
		return false
	}
}

func (s semaphore) release() {
	if s == nil {
		return
	}
	select {
	case s <- struct{}{}:
		// ok
	default:
		panic("semaphore: release without acquire - YOU HAVE A BUG!")
	}
}
