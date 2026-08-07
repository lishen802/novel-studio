package main

import (
	"context"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// jobUpdatedEvent is the Wails event name emitted whenever a job's event feed
// advances. The frontend subscribes via EventsOn("job:updated", ...) and uses
// the payload as a wake-up to pull events via GetJobEvents — never as a source
// of state itself.
const jobUpdatedEvent = "job:updated"

// jobUpdatedPayload is the lightweight Wails event payload for job:updated. It
// carries only the job identity and the latest committed sequence so the
// frontend can decide whether it needs to pull. Notifications may be lost or
// coalesced; correctness is restored by cursor-based replay on the receiving
// side.
type jobUpdatedPayload struct {
	JobID          string `json:"jobId"`
	JobKind        string `json:"jobKind"`
	LatestSequence uint64 `json:"latestSequence"`
}

// wailsJobEventNotifier implements ports.JobEventNotifier by emitting
// "job:updated" Wails runtime events. The emit function is injectable so tests
// can capture events without a running Wails app. Before OnStartup sets the
// Wails context, emit uses context.Background(); in practice jobs only run
// after a project is opened, which requires the app to have started.
type wailsJobEventNotifier struct {
	ctx  context.Context
	emit func(ctx context.Context, eventName string, data ...any)
}

func newWailsJobEventNotifier() *wailsJobEventNotifier {
	return &wailsJobEventNotifier{
		ctx:  context.Background(),
		emit: runtime.EventsEmit,
	}
}

// startup replaces the background context with the Wails app context so
// EventsEmit can reach the frontend. Called from App.startup.
func (n *wailsJobEventNotifier) startup(ctx context.Context) {
	n.ctx = ctx
}

// NotifyJobUpdated emits a job:updated event with the job identity and latest
// sequence. It is best-effort: if the emit fails or the event is lost, the
// frontend will catch up on the next wake-up (mount, HMR, or a subsequent
// notification).
func (n *wailsJobEventNotifier) NotifyJobUpdated(
	jobID string,
	jobKind agentdomain.JobKind,
	latestSequence uint64,
) {
	if n.emit == nil {
		return
	}
	n.emit(n.ctx, jobUpdatedEvent, jobUpdatedPayload{
		JobID:          jobID,
		JobKind:        string(jobKind),
		LatestSequence: latestSequence,
	})
}
