package rule

import (
	"sync"
	"time"
)

// Debouncer tracks per-rule last-fired timestamps to suppress rapid re-fires.
// Thread-safe via sync.Map for concurrent worker access.
// State is in-memory only — resets on restart (first message always fires).
type Debouncer struct {
	lastFired sync.Map // key: string (triggerSubject::actionSubject) → value: time.Time
}

func NewDebouncer() *Debouncer {
	return &Debouncer{}
}

// ShouldFire returns true if the debounce window has expired (or no prior fire exists).
// Uses LoadOrStore for the initial case and CompareAndSwap at the window boundary so
// exactly one concurrent caller fires when the window expires.
func (d *Debouncer) ShouldFire(key string, duration time.Duration) bool {
	now := time.Now()

	actual, loaded := d.lastFired.LoadOrStore(key, now)
	if !loaded {
		return true // first call for this key
	}

	lastTime := actual.(time.Time)
	if now.Sub(lastTime) < duration {
		return false
	}

	// Window expired: only one concurrent caller wins the CAS and fires.
	return d.lastFired.CompareAndSwap(key, lastTime, now)
}

// DebounceKey builds the debounce map key from trigger and action subjects.
func DebounceKey(triggerSubject, actionSubject string) string {
	return triggerSubject + "::" + actionSubject
}