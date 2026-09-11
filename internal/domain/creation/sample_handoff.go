package creation

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrInvalidSampleHandoff = errors.New("invalid opening sample handoff")

// SampleHandoff 只冻结开篇试写的来源与局部约束，不创建章节、费用授权或正式记忆。
type SampleHandoff struct {
	ID                     string    `json:"id"`
	SessionID              string    `json:"sessionId"`
	DirectionCandidateID   string    `json:"directionCandidateId"`
	DirectionContentSHA256 string    `json:"directionContentSha256"`
	DirectionCardID        string    `json:"directionCardId"`
	PointOfView            string    `json:"pointOfView"`
	Tone                   string    `json:"tone"`
	Constraints            []string  `json:"constraints"`
	TargetRunes            int       `json:"targetRunes"`
	CreatedAt              time.Time `json:"createdAt"`
}

// Validate 校验交接来源与范围；模型调用和正式转正仍由 R5 的独立预检负责。
func (handoff SampleHandoff) Validate(direction Candidate) error {
	if strings.TrimSpace(handoff.ID) == "" || handoff.SessionID != direction.SessionID ||
		handoff.DirectionCandidateID != direction.ID || handoff.DirectionContentSHA256 != direction.ContentSHA256 || direction.ContentSHA256 != materialHash(direction.PayloadJSON) ||
		direction.Stage != StageDirections || (direction.Kind != KindDirectionSet && direction.Kind != KindDirection) ||
		direction.Status == StatusStale || direction.Status == StatusSuperseded || direction.Status == StatusDiscarded || direction.Blocked() ||
		strings.TrimSpace(handoff.DirectionCardID) == "" || strings.TrimSpace(handoff.PointOfView) == "" || strings.TrimSpace(handoff.Tone) == "" ||
		handoff.TargetRunes < 1 || handoff.TargetRunes > 10000 || handoff.CreatedAt.IsZero() || len(handoff.Constraints) > 100 {
		return ErrInvalidSampleHandoff
	}
	// 卡的结构由版本化任务校验；此处只验证稳定卡 ID 确实存在于绑定载荷。
	if !hasDirectionCard(direction.PayloadJSON, handoff.DirectionCardID) {
		return ErrInvalidSampleHandoff
	}
	for _, constraint := range handoff.Constraints {
		if strings.TrimSpace(constraint) == "" {
			return ErrInvalidSampleHandoff
		}
	}
	return nil
}

func hasDirectionCard(payload, id string) bool {
	var value struct {
		Directions []struct {
			ID string `json:"id"`
		} `json:"directions"`
	}
	if json.Unmarshal([]byte(payload), &value) != nil {
		return false
	}
	count := 0
	for _, card := range value.Directions {
		if card.ID == id {
			count++
		}
	}
	return count == 1
}
