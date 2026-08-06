package workspace

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestLeaseAcquireReturnsLeaseWithGenerationAndReleaseDrainsImmediately(t *testing.T) {
	registry := newLeaseRegistry()

	lease, err := registry.acquire("job-1", 7)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if lease.Generation() != 7 {
		t.Fatalf("generation = %d, want 7", lease.Generation())
	}
	if lease.IsStale() {
		t.Fatal("fresh lease must not be stale")
	}

	registry.release("job-1")

	if err := registry.drainAndBlock(context.Background(), time.Second); err != nil {
		t.Fatalf("drainAndBlock after release: %v", err)
	}
}

func TestLeaseAcquireRejectsWhileDraining(t *testing.T) {
	registry := newLeaseRegistry()
	registry.drainAndBlock(context.Background(), time.Second) // no leases → returns immediately, sets draining

	if _, err := registry.acquire("job-1", 1); !errors.Is(err, ErrWorkspaceDraining) {
		t.Fatalf("acquire during drain err = %v, want ErrWorkspaceDraining", err)
	}

	registry.endDraining()

	if _, err := registry.acquire("job-1", 1); err != nil {
		t.Fatalf("acquire after endDraining: %v", err)
	}
}

func TestLeaseDrainTimesOutAndMarksRemainingStale(t *testing.T) {
	registry := newLeaseRegistry()
	lease, err := registry.acquire("job-1", 1)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}

	err = registry.drainAndBlock(context.Background(), 20*time.Millisecond)
	if !errors.Is(err, ErrDrainTimeout) {
		t.Fatalf("drainAndBlock err = %v, want ErrDrainTimeout", err)
	}
	if !registry.stale("job-1") {
		t.Fatal("remaining lease must be marked stale after drain timeout")
	}
	if !lease.IsStale() {
		t.Fatal("lease instance must report stale after drain timeout")
	}
}

func TestLeaseStaleMarkingPersistsAfterRelease(t *testing.T) {
	registry := newLeaseRegistry()
	if _, err := registry.acquire("job-1", 1); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if err := registry.drainAndBlock(context.Background(), 20*time.Millisecond); err == nil {
		t.Fatal("drainAndBlock expected timeout error")
	}

	registry.release("job-1")

	if !registry.stale("job-1") {
		t.Fatal("stale marking must persist after release")
	}
}

func TestLeaseConcurrentAcquireReleaseNoRace(t *testing.T) {
	registry := newLeaseRegistry()
	const goroutines = 16
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(prefix string) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				jobID := prefix + "-" + itoa(i)
				lease, err := registry.acquire(jobID, 1)
				if err != nil {
					continue
				}
				_ = lease.IsStale()
				registry.release(jobID)
			}
		}("g" + itoa(g))
	}
	wg.Wait()
}

// itoa is a tiny local int-to-string helper to keep the test file dependency-free.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
