package rule

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestDebouncer_FirstCallAlwaysFires(t *testing.T) {
	d := NewDebouncer()
	if !d.ShouldFire("key1", 30*time.Second) {
		t.Error("first call should always fire")
	}
}

func TestDebouncer_SuppressesWithinWindow(t *testing.T) {
	d := NewDebouncer()
	d.ShouldFire("key1", 30*time.Second)

	if d.ShouldFire("key1", 30*time.Second) {
		t.Error("second call within window should be suppressed")
	}
}

func TestDebouncer_FiresAfterWindowExpires(t *testing.T) {
	d := NewDebouncer()
	d.ShouldFire("key1", 1*time.Millisecond)

	time.Sleep(5 * time.Millisecond)

	if !d.ShouldFire("key1", 1*time.Millisecond) {
		t.Error("should fire after debounce window expires")
	}
}

func TestDebouncer_IndependentKeys(t *testing.T) {
	d := NewDebouncer()
	d.ShouldFire("key1", 30*time.Second)

	if !d.ShouldFire("key2", 30*time.Second) {
		t.Error("different keys should debounce independently")
	}

	if d.ShouldFire("key1", 30*time.Second) {
		t.Error("key1 should still be debounced")
	}
}

func TestDebouncer_ConcurrentAccess(t *testing.T) {
	d := NewDebouncer()
	var wg sync.WaitGroup
	fired := make(chan bool, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", id%5)
			result := d.ShouldFire(key, 30*time.Second)
			fired <- result
		}(i)
	}

	wg.Wait()
	close(fired)

	trueCount := 0
	for f := range fired {
		if f {
			trueCount++
		}
	}

	// With 5 unique keys and 20 goroutines per key, at least 5 should fire (first per key).
	// Due to races, slightly more may fire but never 100.
	if trueCount < 5 {
		t.Errorf("expected at least 5 fires (one per unique key), got %d", trueCount)
	}
	if trueCount > 20 {
		t.Errorf("expected far fewer than 100 fires due to debouncing, got %d", trueCount)
	}
}

func TestDebounceKey(t *testing.T) {
	key := DebounceKey("zigbee2mqtt.DoorSensor", "lab.notifications.garage-back-door")
	expected := "zigbee2mqtt.DoorSensor::lab.notifications.garage-back-door"
	if key != expected {
		t.Errorf("got %q, want %q", key, expected)
	}
}