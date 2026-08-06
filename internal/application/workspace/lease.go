package workspace

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// jobLease represents an in-flight agent or extraction job's claim on the
// active project store. The lease is acquired while the workspace lock is held
// so the captured generation matches the active store; it is released after
// the job has persisted its final state (including cancellation), never while
// a Model IO call is in flight on the owning goroutine.
//
// If the workspace is switched or closed while a lease is still held, drain
// first cancels the job's context and waits for it to release. Should the job
// fail to release within the drain timeout, the lease is marked stale so the
// job's subsequent persistence attempts are skipped instead of writing to a
// closed store.
type jobLease struct {
	generation uint64
	stale      atomic.Bool
	released   chan struct{}
	once       sync.Once
}

// Generation returns the store generation captured when the lease was acquired.
// The job compares this against the workspace's current generation before each
// persistence step.
func (l *jobLease) Generation() uint64 { return l.generation }

// IsStale reports whether the lease was force-marked stale by a drain timeout.
// A stale lease must not persist to its captured store; the owning job should
// transition to a recoverable terminal state instead.
func (l *jobLease) IsStale() bool { return l.stale.Load() }

// leaseRegistry tracks in-flight job leases and coordinates draining when the
// workspace switches, closes, or recovers a project. It is safe for concurrent
// use. draining blocks new acquisitions while existing leases drain; stale
// tracking persists across release so jobs that wind down after a drain timeout
// still skip stale writes.
type leaseRegistry struct {
	mu          sync.Mutex
	draining    bool
	leases      map[string]*jobLease
	staleJobIDs map[string]struct{}
	wg          sync.WaitGroup
}

func newLeaseRegistry() *leaseRegistry {
	return &leaseRegistry{
		leases:      make(map[string]*jobLease),
		staleJobIDs: make(map[string]struct{}),
	}
}

// acquire registers a lease for jobID bound to generation. It returns
// ErrWorkspaceDraining when the registry is draining (new jobs must be rejected
// until endDraining reopens the workspace), or ErrAgentRunActive when jobID
// already holds a lease.
func (r *leaseRegistry) acquire(jobID string, generation uint64) (*jobLease, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.draining {
		return nil, ErrWorkspaceDraining
	}
	if _, exists := r.leases[jobID]; exists {
		return nil, ErrAgentRunActive
	}
	lease := &jobLease{generation: generation, released: make(chan struct{})}
	r.leases[jobID] = lease
	r.wg.Add(1)
	return lease, nil
}

// release removes the lease for jobID and signals waiters. Releasing a lease
// that was force-marked stale keeps the stale marking (a stale job must not
// resume writes), but still counts down the drain WaitGroup so switch/close can
// proceed. Releasing an unknown jobID is a no-op, allowing jobs to call release
// defensively from deferred cleanup.
func (r *leaseRegistry) release(jobID string) {
	r.mu.Lock()
	lease, exists := r.leases[jobID]
	if exists {
		delete(r.leases, jobID)
	}
	r.mu.Unlock()
	if lease == nil {
		return
	}
	lease.once.Do(func() {
		close(lease.released)
		r.wg.Done()
	})
}

// drainAndBlock marks the registry draining (new acquisitions fail with
// ErrWorkspaceDraining) and waits for all in-flight leases to release, the
// timeout to elapse, or ctx to be cancelled. If leases remain when the wait
// ends, they are marked stale and ErrDrainTimeout (or ctx.Err()) is returned.
// A zero or negative timeout marks remaining leases stale immediately.
//
// drainAndBlock may be called more than once; the draining flag stays set
// until endDraining clears it, and each call waits on whatever leases are still
// registered.
func (r *leaseRegistry) drainAndBlock(ctx context.Context, timeout time.Duration) error {
	r.mu.Lock()
	r.draining = true
	remaining := len(r.leases)
	r.mu.Unlock()
	if remaining == 0 {
		return nil
	}
	if timeout <= 0 {
		r.markRemainingStale()
		return ErrDrainTimeout
	}

	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return nil
	case <-timer.C:
		r.markRemainingStale()
		return ErrDrainTimeout
	case <-ctx.Done():
		r.markRemainingStale()
		return ctx.Err()
	}
}

// markRemainingStale marks every still-registered lease stale and records its
// jobID so stale(jobID) keeps reporting true even after the job eventually
// releases the lease.
func (r *leaseRegistry) markRemainingStale() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for jobID, lease := range r.leases {
		lease.stale.Store(true)
		r.staleJobIDs[jobID] = struct{}{}
	}
}

// endDraining reopens the registry to new leases after switch/close has
// finished closing the previous store and installing the next active
// workspace. It does not clear stale markings: jobs that were marked stale
// during the drain must still skip stale writes even though new jobs (with new
// jobIDs) may now acquire leases on the new generation.
func (r *leaseRegistry) endDraining() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.draining = false
}

// stale reports whether jobID was force-marked stale by a drain timeout. It is
// safe to call after release; the marking persists so jobs that wind down late
// still skip stale writes.
func (r *leaseRegistry) stale(jobID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.staleJobIDs[jobID]
	return ok
}

// drainTimeout is the default upper bound for waiting on in-flight leases
// during project switch/close. Tests inject shorter values via the service.
var drainTimeoutDefault = 5 * time.Second

var (
	ErrWorkspaceDraining = errors.New("workspace is draining; new jobs rejected")
	ErrDrainTimeout      = errors.New("timed out waiting for in-flight job leases to drain")
)
