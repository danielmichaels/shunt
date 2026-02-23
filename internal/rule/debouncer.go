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
// On true, it atomically updates the last-fired timestamp.
func (d *Debouncer) ShouldFire(key string, duration time.Duration) bool {
	now := time.Now()

	val, loaded := d.lastFired.Load(key)
	if !loaded {
		d.lastFired.Store(key, now)
		return true
	}

	lastTime := val.(time.Time)
	if now.Sub(lastTime) >= duration {
		d.lastFired.Store(key, now)
		return true
	}

	return false
}

// DebounceKey builds the debounce map key from trigger and action subjects.
func DebounceKey(triggerSubject, actionSubject string) string {
	return triggerSubject + "::" + actionSubject
}