package workspace

import (
	"context"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	"github.com/lishen802/novel-studio/internal/application/ports"
)

// noopJobEventNotifier is the default notifier used when no UI bridge is
// attached. It silently drops every notification so tests that don't assert
// event delivery are unaffected.
type noopJobEventNotifier struct{}

func (noopJobEventNotifier) NotifyJobUpdated(string, agentdomain.JobKind, uint64) {}

// SetJobEventNotifier installs the UI-bridge notifier. It is called by the Wails
// layer after creating the Service so the desktop app can emit runtime.EventsEmit
// wake-ups. Tests inject a fake notifier to assert delivery.
func (s *Service) SetJobEventNotifier(notifier ports.JobEventNotifier) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if notifier == nil {
		s.notifier = noopJobEventNotifier{}
	} else {
		s.notifier = notifier
	}
}

// notifyJobUpdated reads the authoritative snapshot for the job and forwards a
// best-effort wake-up. If the snapshot read fails (e.g. the job has no events
// yet, or the store is closing), the notification is still emitted with a zero
// sequence so the frontend can trigger a pull. The notifier itself may lose or
// coalesce the call; correctness is restored by cursor-based replay on the
// receiving side.
func (s *Service) notifyJobUpdated(
	ctx context.Context,
	store ports.JobEventStore,
	jobID string,
	kind agentdomain.JobKind,
) {
	if s.notifier == nil {
		return
	}
	var latest uint64
	if snapshot, err := store.JobSnapshot(ctx, jobID); err == nil {
		latest = snapshot.MaxSequence
	}
	s.notifier.NotifyJobUpdated(jobID, kind, latest)
}
