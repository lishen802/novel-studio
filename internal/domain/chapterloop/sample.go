package chapterloop

import (
	"errors"
	"strings"
)

var (
	ErrInvalidOpeningSample = errors.New("invalid opening sample")
	ErrSampleNotPromotable  = errors.New("opening sample cannot be promoted")
)

type SampleStatus string

const (
	SampleCandidate SampleStatus = "candidate"
	SampleFeedback  SampleStatus = "feedback"
	SampleStale     SampleStatus = "stale"
	SampleDiscarded SampleStatus = "discarded"
	SamplePromoted  SampleStatus = "promoted"
)

type SampleFeedbackKind string

const (
	FeedbackContinuePlan SampleFeedbackKind = "continue_plan"
	FeedbackAdjust       SampleFeedbackKind = "adjust"
	FeedbackDiscard      SampleFeedbackKind = "discard"
)

// OpeningSample 是方向后的非正式试写，不进入正式记忆，也不计入定稿字数。
type OpeningSample struct {
	ID                     string             `json:"id"`
	HandoffID              string             `json:"handoffId"`
	DirectionCandidateID   string             `json:"directionCandidateId"`
	DirectionContentSHA256 string             `json:"directionContentSha256"`
	Body                   string             `json:"body"`
	Status                 SampleStatus       `json:"status"`
	Feedback               SampleFeedbackKind `json:"feedback,omitempty"`
	ArchiveConfirmed       bool               `json:"archiveConfirmed"`
	RecheckPassed          bool               `json:"recheckPassed"`
	ExplicitAccept         bool               `json:"explicitAccept"`
}

// NewOpeningSample 校验来源后创建可审阅候选。
func NewOpeningSample(id, handoffID, directionID, digest, body string) (OpeningSample, error) {
	sample := OpeningSample{
		ID: strings.TrimSpace(id), HandoffID: strings.TrimSpace(handoffID),
		DirectionCandidateID: strings.TrimSpace(directionID), DirectionContentSHA256: strings.TrimSpace(digest),
		Body: strings.TrimSpace(body), Status: SampleCandidate,
	}
	if sample.ID == "" || sample.HandoffID == "" || sample.DirectionCandidateID == "" || sample.DirectionContentSHA256 == "" || sample.Body == "" {
		return OpeningSample{}, ErrInvalidOpeningSample
	}
	return sample, nil
}

// CountsAsMemory 试写在任何状态下都不是正式记忆。
func (sample OpeningSample) CountsAsMemory() bool { return false }

// CountsAsFinalizedWords 试写在任何状态下都不计入定稿字数。
func (sample OpeningSample) CountsAsFinalizedWords() bool { return false }

// RecordFeedback 把试写意见退回策划，不创建章节。
func (sample OpeningSample) RecordFeedback(kind SampleFeedbackKind) (OpeningSample, error) {
	switch kind {
	case FeedbackContinuePlan, FeedbackAdjust, FeedbackDiscard:
	default:
		return OpeningSample{}, ErrInvalidOpeningSample
	}
	if sample.Status == SampleStale || sample.Status == SamplePromoted {
		return OpeningSample{}, ErrSampleNotPromotable
	}
	sample.Feedback = kind
	sample.Status = SampleFeedback
	if kind == FeedbackDiscard {
		sample.Status = SampleDiscarded
	}
	return sample, nil
}

// MarkStaleIfDigestChanged 在方向摘要变化时使旧试写失效。
func (sample OpeningSample) MarkStaleIfDigestChanged(current string) OpeningSample {
	if strings.TrimSpace(current) != sample.DirectionContentSHA256 {
		sample.Status = SampleStale
		sample.ArchiveConfirmed = false
		sample.RecheckPassed = false
		sample.ExplicitAccept = false
	}
	return sample
}

// Promote 只在档案确认、复检和显式接受同时成立时标记转正。
func (sample OpeningSample) Promote(archiveConfirmed, recheckPassed, explicitAccept bool) (OpeningSample, error) {
	if sample.Status == SampleStale || sample.Status == SampleDiscarded || !archiveConfirmed || !recheckPassed || !explicitAccept {
		return sample, ErrSampleNotPromotable
	}
	sample.Status = SamplePromoted
	sample.ArchiveConfirmed = true
	sample.RecheckPassed = true
	sample.ExplicitAccept = true
	return sample, nil
}
