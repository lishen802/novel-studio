package quality

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

// EvaluateSkeleton 检查骨架是否越权写成正文，以及长度分配是否偏离目标。
func (engine *Engine) EvaluateSkeleton(version int, namespace string, skeleton agentdomain.ChapterSkeletonOutput, targetRunes int) (Report, error) {
	policy, err := engine.Policy(version)
	if err != nil || strings.TrimSpace(namespace) == "" || skeleton.Validate() != nil || policy.SkeletonIntentMaxRunes < 1 {
		return Report{}, ErrInvalidInput
	}
	encoded, _ := agentJSON(skeleton)
	report := slotReport(policy, namespace, encoded)
	total := 0
	for _, beat := range skeleton.Beats {
		for _, slot := range beat.Slots {
			total += slot.TargetRunes
			if utf8.RuneCountInString(slot.Intent) > policy.SkeletonIntentMaxRunes {
				report.Findings = append(report.Findings, slotFinding(namespace, "slot.skeleton_overreach", agentdomain.FindingFormat, agentdomain.SeverityMajor,
					fmt.Sprintf("槽位 %s 的意图超过 %d 字符，骨架疑似越权产出正文。", slot.ID, policy.SkeletonIntentMaxRunes)))
			}
		}
	}
	if targetRunes > 0 && float64(abs(total-targetRunes))/float64(targetRunes) > policy.SlotTargetDeviationRatio {
		report.Findings = append(report.Findings, slotFinding(namespace, "slot.length_allocation", agentdomain.FindingInformationDensity, agentdomain.SeverityMajor,
			fmt.Sprintf("槽位目标长度合计 %d，与章节目标 %d 偏离超过 %.0f%%。", total, targetRunes, policy.SlotTargetDeviationRatio*100)))
	}
	return report, nil
}

// EvaluateSlotIntegration 检查覆盖、跨槽重复和整合稿相对确定性拼接的漂移。
func (engine *Engine) EvaluateSlotIntegration(version int, namespace string, skeleton agentdomain.ChapterSkeletonOutput, groups []agentdomain.FillSlotOutput, mechanical, integrated string) (Report, error) {
	policy, err := engine.Policy(version)
	if err != nil || strings.TrimSpace(namespace) == "" || skeleton.Validate() != nil || policy.CrossSlotPhraseRunes < 1 {
		return Report{}, ErrInvalidInput
	}
	report := slotReport(policy, namespace, integrated)
	join, joinErr := agentdomain.MechanicalJoin(skeleton, groups...)
	if joinErr != nil || join.Body != mechanical {
		report.Findings = append(report.Findings, slotFinding(namespace, "slot.coverage", agentdomain.FindingFormat, agentdomain.SeverityBlocker,
			"槽位缺失、重复填充或出现骨架未声明的 slot_id。"))
		return report, nil
	}
	ratio := editRatio([]rune(mechanical), []rune(integrated))
	if ratio > policy.IntegrationMajorRatio {
		severity := agentdomain.SeverityMajor
		if ratio > policy.IntegrationBlockerRatio {
			severity = agentdomain.SeverityBlocker
		}
		report.Findings = append(report.Findings, slotFinding(namespace, "slot.integration_drift", agentdomain.FindingContinuity, severity,
			fmt.Sprintf("整合稿相对确定性拼接的 rune 编辑距离为 %.1f%%。", ratio*100)))
	}
	if phrase, left, right := sharedSlotPhrase(groups, policy.CrossSlotPhraseRunes); phrase != "" {
		report.Findings = append(report.Findings, slotFinding(namespace, "slot.cross_repetition", agentdomain.FindingRepetition, agentdomain.SeverityMajor,
			fmt.Sprintf("槽位 %s 与 %s 重复使用 %d 字符短语“%s”。", left, right, policy.CrossSlotPhraseRunes, phrase)))
	}
	return report, nil
}

func slotReport(policy Policy, namespace, text string) Report {
	rules := make([]RuleBinding, 0, 5)
	for _, rule := range policy.Rules {
		if strings.HasPrefix(rule.ID, "slot.") {
			rules = append(rules, RuleBinding{ID: rule.ID, Version: rule.Version, Category: rule.Category, Severity: rule.Severity})
		}
	}
	return Report{PolicyVersion: policy.Version, PolicyHash: policy.Hash, Stage: StageInitial,
		TextHash: agentdomain.SHA256Hex([]byte(text)), RuneCount: utf8.RuneCountInString(text), Rules: rules}
}

func slotFinding(namespace, rule string, category agentdomain.FindingCategory, severity agentdomain.FindingSeverity, message string) agentdomain.ReviewFinding {
	return agentdomain.ReviewFinding{ID: namespace + ":" + rule + ":001", Category: category, Severity: severity,
		Status: agentdomain.FindingOpen, Source: agentdomain.FindingDeterministicLint, Message: message, ConstraintRef: "rule:" + rule}
}

func editRatio(left, right []rune) float64 {
	denominator := max(len(left), 1)
	previous := make([]int, len(right)+1)
	for index := range previous {
		previous[index] = index
	}
	for i := 1; i <= len(left); i++ {
		current := make([]int, len(right)+1)
		current[0] = i
		for j := 1; j <= len(right); j++ {
			cost := 1
			if left[i-1] == right[j-1] {
				cost = 0
			}
			current[j] = min(current[j-1]+1, previous[j]+1, previous[j-1]+cost)
		}
		previous = current
	}
	return float64(previous[len(right)]) / float64(denominator)
}

// RuneEditRatio 返回候选相对基线的 Unicode rune 级编辑距离比率。
func RuneEditRatio(baseline, candidate string) float64 {
	return editRatio([]rune(baseline), []rune(candidate))
}

func sharedSlotPhrase(groups []agentdomain.FillSlotOutput, width int) (string, string, string) {
	type owner struct{ id string }
	seen := map[string]owner{}
	type hit struct{ phrase, left, right string }
	var hits []hit
	for _, group := range groups {
		for _, slot := range group.FilledSlots {
			runes := []rune(slot.Content)
			local := map[string]bool{}
			for index := 0; index+width <= len(runes); index++ {
				window := runes[index : index+width]
				if hasSpaceOrPunctuation(window) {
					continue
				}
				phrase := string(window)
				if local[phrase] {
					continue
				}
				local[phrase] = true
				if prior, exists := seen[phrase]; exists && prior.id != slot.ID {
					hits = append(hits, hit{phrase: phrase, left: prior.id, right: slot.ID})
				} else {
					seen[phrase] = owner{id: slot.ID}
				}
			}
		}
	}
	if len(hits) == 0 {
		return "", "", ""
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].phrase < hits[j].phrase })
	return hits[0].phrase, hits[0].left, hits[0].right
}

func hasSpaceOrPunctuation(value []rune) bool {
	for _, current := range value {
		if unicode.IsSpace(current) || unicode.IsPunct(current) {
			return true
		}
	}
	return false
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func agentJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	return string(encoded), err
}
