package ports

import agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"

// JobEventNotifier is the lightweight wake-up channel between the workspace
// Service and the UI bridge. After each JobEvent commit, the Service calls
// NotifyJobUpdated so the frontend can trigger a cursor-based pull. The
// notification is best-effort: it may be lost, coalesced or duplicated. The
// frontend reconciles by pulling events with its current cursor, so a missed
// notification only delays the next pull (e.g. until the next user action or
// remount). The payload intentionally carries only the job identity and the
// latest sequence — bulky content lives in artifacts and is fetched on demand.
type JobEventNotifier interface {
	NotifyJobUpdated(jobID string, jobKind agentdomain.JobKind, latestSequence uint64)
}
