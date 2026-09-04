package creation

import (
	"strings"
	"time"
)

type ImpactSeverity string

const (
	ImpactNone  ImpactSeverity = ""
	ImpactMaybe ImpactSeverity = "maybe_affected"
	ImpactMust  ImpactSeverity = "must_fix"
)

type ImpactChoice string

const (
	ChoiceRegenerateLocal  ImpactChoice = "regenerate_local"
	ChoiceKeepException    ImpactChoice = "keep_exception"
	ChoiceContinueTalk     ImpactChoice = "continue_discussion"
	ChoiceAbandonUpstream  ImpactChoice = "abandon_upstream"
)

func (choice ImpactChoice) Valid() bool {
	switch choice {
	case ChoiceRegenerateLocal, ChoiceKeepException, ChoiceContinueTalk, ChoiceAbandonUpstream:
		return true
	default:
		return false
	}
}

func (candidate Candidate) WithUpstream(digest string) Candidate {
	candidate.UpstreamDigest = strings.TrimSpace(digest)
	return candidate
}

// MarkStale 把下游候选标为过期，不改 payload，也不改写正式资产。
func (candidate Candidate) MarkStale(reason string, refs []string, severity ImpactSeverity, now time.Time) (Candidate, error) {
	reason = strings.TrimSpace(reason)
	if candidate.ID == "" || reason == "" {
		return Candidate{}, ErrBlankValue
	}
	if candidate.Status == StatusSuperseded || candidate.Status == StatusDiscarded {
		return Candidate{}, ErrInvalidTransition
	}
	if severity != ImpactMaybe && severity != ImpactMust {
		return Candidate{}, ErrBlankValue
	}
	candidate.Status = StatusStale
	candidate.StaleReason = reason
	candidate.ImpactRefs = cloneImpactRefs(refs)
	candidate.ImpactSeverity = severity
	candidate.ExceptedUpstreamDigest = ""
	candidate.UpdatedAt = now.UTC()
	return candidate, nil
}

// KeepException 把当前上游摘要记为例外，候选保持 stale 但允许作者前进。
func (candidate Candidate) KeepException(upstreamDigest string, now time.Time) (Candidate, error) {
	upstreamDigest = strings.TrimSpace(upstreamDigest)
	if candidate.ID == "" || upstreamDigest == "" {
		return Candidate{}, ErrBlankValue
	}
	if candidate.Status != StatusStale {
		return Candidate{}, ErrInvalidTransition
	}
	candidate.ExceptedUpstreamDigest = upstreamDigest
	candidate.UpdatedAt = now.UTC()
	return candidate, nil
}

// ClearStale 在上游哈希恢复后解除过期标记，不恢复为已确认。
func (candidate Candidate) ClearStale(now time.Time) (Candidate, error) {
	if candidate.ID == "" {
		return Candidate{}, ErrBlankValue
	}
	if candidate.Status != StatusStale {
		return Candidate{}, ErrInvalidTransition
	}
	candidate.Status = StatusDraft
	candidate.StaleReason = ""
	candidate.ImpactRefs = []string{}
	candidate.ImpactSeverity = ImpactNone
	candidate.ExceptedUpstreamDigest = ""
	candidate.UpdatedAt = now.UTC()
	return candidate, nil
}

// Discard 废弃一次上游修改，供作者放弃后恢复被替代版本。
func (candidate Candidate) Discard(now time.Time) (Candidate, error) {
	if candidate.ID == "" {
		return Candidate{}, ErrBlankValue
	}
	if candidate.Status == StatusDiscarded {
		return Candidate{}, ErrInvalidTransition
	}
	candidate.Status = StatusDiscarded
	candidate.UpdatedAt = now.UTC()
	return candidate, nil
}

// Restore 把被替代或废弃的候选恢复为可审阅草稿，不改 payload。
func (candidate Candidate) Restore(now time.Time) (Candidate, error) {
	if candidate.ID == "" {
		return Candidate{}, ErrBlankValue
	}
	if candidate.Status != StatusSuperseded && candidate.Status != StatusDiscarded {
		return Candidate{}, ErrInvalidTransition
	}
	candidate.Status = StatusDraft
	candidate.UpdatedAt = now.UTC()
	return candidate, nil
}

func (candidate Candidate) ExceptionMatches(upstreamDigest string) bool {
	digest := strings.TrimSpace(upstreamDigest)
	return candidate.Status == StatusStale && digest != "" && candidate.ExceptedUpstreamDigest == digest
}

func (candidate Candidate) HasLocks() bool {
	for _, locked := range candidate.LockMap {
		if locked {
			return true
		}
	}
	return false
}

func StaleSeverity(candidate Candidate) ImpactSeverity {
	if candidate.HasLocks() || candidate.Status == StatusConfirmed {
		return ImpactMust
	}
	return ImpactMaybe
}

func cloneImpactRefs(values []string) []string {
	cloned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		cloned = append(cloned, value)
	}
	return cloned
}
