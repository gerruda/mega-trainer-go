package telegram

import "sync"

// keyedLocker serializes work for the same key while letting different keys run
// concurrently. Locks are reference counted and removed once unused, so the map
// does not grow with the number of users.
type keyedLocker struct {
	mu    sync.Mutex
	locks map[int64]*keyedLock
}

type keyedLock struct {
	mu   sync.Mutex
	refs int
}

func newKeyedLocker() *keyedLocker {
	return &keyedLocker{locks: make(map[int64]*keyedLock)}
}

// Lock blocks until the key is free and returns the matching unlock function.
func (l *keyedLocker) Lock(key int64) func() {
	l.mu.Lock()
	entry, ok := l.locks[key]
	if !ok {
		entry = &keyedLock{}
		l.locks[key] = entry
	}
	entry.refs++
	l.mu.Unlock()

	entry.mu.Lock()

	return func() {
		entry.mu.Unlock()

		l.mu.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(l.locks, key)
		}
		l.mu.Unlock()
	}
}
