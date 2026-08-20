package workspace

import (
	"encoding/json"
	"time"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	extractiondomain "github.com/lishen802/novel-studio/internal/domain/extraction"
)

// eventJSON marshals a small set of key/value pairs to a compact JSON string for
// a JobEvent payload. Bulky content (generated text, manifests) lives in
// artifacts; payloads only carry lightweight summaries so the event feed stays
// cheap to replay. A marshal failure (impossible for the simple maps used here)
// yields an empty string rather than blocking the transition.
func eventJSON(fields map[string]any) string {
	if len(fields) == 0 {
		return ""
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return ""
	}
	return string(encoded)
}

// newAgentJobDraft constructs a JobEventDraft for an agent run transition. The
// run is already persisted and validated, so draft construction only fails on a
// programming error (blank ID/project); callers surface that error alongside
// the store update so the transition is not committed without its event.
func newAgentJobDraft(
	run agentdomain.Run,
	eventType agentdomain.JobEventType,
	stepID, payload string,
	now time.Time,
) (agentdomain.JobEventDraft, error) {
	return agentdomain.NewJobEventDraft(run.ID, agentdomain.JobKindAgent, run.ProjectID, eventType, stepID, payload, now)
}

// newExtractionJobDraft mirrors newAgentJobDraft for extraction tasks.
func newExtractionJobDraft(
	task extractiondomain.Task,
	eventType agentdomain.JobEventType,
	stepID, payload string,
	now time.Time,
) (agentdomain.JobEventDraft, error) {
	return agentdomain.NewJobEventDraft(task.ID, agentdomain.JobKindExtraction, task.ProjectID, eventType, stepID, payload, now)
}
