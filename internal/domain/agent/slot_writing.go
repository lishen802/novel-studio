package agent

import (
	"errors"
	"sort"
	"strings"
	"unicode/utf8"
)

var ErrInvalidSlotWriting = errors.New("invalid slot writing output")

type SlotType string

const (
	SlotDialogue    SlotType = "dialogue"
	SlotAction      SlotType = "action"
	SlotInteriority SlotType = "interiority"
	SlotDescription SlotType = "description"
	SlotTransition  SlotType = "transition"
)

type SkeletonSlot struct {
	ID          string   `json:"slot_id"`
	Type        SlotType `json:"slot_type"`
	Intent      string   `json:"intent"`
	Constraints []string `json:"constraints"`
	TargetRunes int      `json:"target_runes"`
	SourceRefs  []string `json:"source_refs"`
}

type SkeletonBeat struct {
	ID      string         `json:"beat_id"`
	Summary string         `json:"summary"`
	POV     string         `json:"pov"`
	Slots   []SkeletonSlot `json:"slots"`
}

type ChapterSkeletonOutput struct {
	ChapterGoal string         `json:"chapter_goal"`
	Beats       []SkeletonBeat `json:"beats"`
}

func (output ChapterSkeletonOutput) Validate() error {
	if blank(output.ChapterGoal) || len(output.Beats) == 0 || len(output.Beats) > MaxChapterScenes {
		return ErrInvalidSlotWriting
	}
	beats, slots := map[string]bool{}, map[string]bool{}
	for _, beat := range output.Beats {
		if blank(beat.ID) || blank(beat.Summary) || blank(beat.POV) || beats[beat.ID] || len(beat.Slots) == 0 {
			return ErrInvalidSlotWriting
		}
		beats[beat.ID] = true
		for _, slot := range beat.Slots {
			if blank(slot.ID) || !ValidSlotType(slot.Type) || blank(slot.Intent) || slot.TargetRunes < 1 || slots[slot.ID] ||
				hasBlankOrDuplicate(slot.Constraints) || hasBlankOrDuplicate(slot.SourceRefs) {
				return ErrInvalidSlotWriting
			}
			slots[slot.ID] = true
		}
	}
	return nil
}

type FilledSlot struct {
	ID         string   `json:"slot_id"`
	Content    string   `json:"content"`
	AnchorHits []string `json:"anchor_hits"`
}

type FillSlotOutput struct {
	FilledSlots []FilledSlot `json:"filled_slots"`
}

func (output FillSlotOutput) Validate() error {
	if len(output.FilledSlots) == 0 || len(output.FilledSlots) > MaxChapterScenes*5 {
		return ErrInvalidSlotWriting
	}
	seen := map[string]bool{}
	for _, slot := range output.FilledSlots {
		if blank(slot.ID) || blank(slot.Content) || seen[slot.ID] || hasBlankOrDuplicate(slot.AnchorHits) {
			return ErrInvalidSlotWriting
		}
		seen[slot.ID] = true
	}
	return nil
}

type JoinedSlotSpan struct {
	SlotID string `json:"slot_id"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
}

type SlotJoin struct {
	Body  string           `json:"body"`
	Spans []JoinedSlotSpan `json:"spans"`
}

// MechanicalJoin 按骨架声明顺序拼接槽位，并用规范化 LF 计算 Unicode rune 偏移。
func MechanicalJoin(skeleton ChapterSkeletonOutput, groups ...FillSlotOutput) (SlotJoin, error) {
	if skeleton.Validate() != nil {
		return SlotJoin{}, ErrInvalidSlotWriting
	}
	filled := map[string]string{}
	for _, group := range groups {
		if group.Validate() != nil {
			return SlotJoin{}, ErrInvalidSlotWriting
		}
		for _, slot := range group.FilledSlots {
			if _, exists := filled[slot.ID]; exists {
				return SlotJoin{}, ErrInvalidSlotWriting
			}
			filled[slot.ID] = normalizeSlotText(slot.Content)
		}
	}
	var parts []string
	var spans []JoinedSlotSpan
	offset := 0
	for _, beat := range skeleton.Beats {
		for _, slot := range beat.Slots {
			content, exists := filled[slot.ID]
			if !exists {
				return SlotJoin{}, ErrInvalidSlotWriting
			}
			delete(filled, slot.ID)
			if len(parts) > 0 {
				offset += 2
			}
			start := offset
			offset += utf8.RuneCountInString(content)
			parts = append(parts, content)
			spans = append(spans, JoinedSlotSpan{SlotID: slot.ID, Start: start, End: offset})
		}
	}
	if len(filled) != 0 {
		return SlotJoin{}, ErrInvalidSlotWriting
	}
	return SlotJoin{Body: strings.Join(parts, "\n\n"), Spans: spans}, nil
}

func normalizeSlotText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.TrimSpace(value)
}

func ValidSlotType(value SlotType) bool {
	return value == SlotDialogue || value == SlotAction || value == SlotInteriority || value == SlotDescription || value == SlotTransition
}

func (output ChapterSkeletonOutput) SlotIDs(kind SlotType) []string {
	var result []string
	for _, beat := range output.Beats {
		for _, slot := range beat.Slots {
			if slot.Type == kind {
				result = append(result, slot.ID)
			}
		}
	}
	sort.Strings(result)
	return result
}
