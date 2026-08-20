package quality

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

type Stage string

const (
	StageBaseline Stage = "baseline"
	StageInitial  Stage = "initial"
	StageFinal    Stage = "final"
)

type Input struct {
	Namespace string
	Stage     Stage
	Text      string
	Spec      Spec
}

type RuleBinding struct {
	ID       string                      `json:"id"`
	Version  int                         `json:"version"`
	Category agentdomain.FindingCategory `json:"category"`
	Severity agentdomain.FindingSeverity `json:"severity"`
}

type Report struct {
	PolicyVersion int                         `json:"policy_version"`
	PolicyHash    string                      `json:"policy_hash"`
	Stage         Stage                       `json:"stage"`
	TextHash      string                      `json:"text_hash"`
	RuneCount     int                         `json:"rune_count"`
	Truncated     bool                        `json:"truncated"`
	Rules         []RuleBinding               `json:"rules"`
	Findings      []agentdomain.ReviewFinding `json:"findings"`
}

type Engine struct{ registry *Registry }

// NewEngine 创建只执行本地确定性规则的质量引擎。
func NewEngine(registry *Registry) *Engine { return &Engine{registry: registry} }

// Policy 返回引擎可执行的精确策略版本。
func (engine *Engine) Policy(version int) (Policy, error) {
	if engine == nil {
		return Policy{}, ErrPolicyNotFound
	}
	return engine.registry.Policy(version)
}

type match struct {
	message       string
	suggestion    string
	quote         string
	start         *int
	end           *int
	constraintRef string
}

// Evaluate 使用指定不可变策略执行规则，结果只取决于冻结输入与策略版本。
func (engine *Engine) Evaluate(policyVersion int, input Input) (Report, error) {
	if engine == nil || strings.TrimSpace(input.Namespace) == "" || !validStage(input.Stage) || input.Spec.Validate() != nil {
		return Report{}, ErrInvalidInput
	}
	policy, err := engine.registry.Policy(policyVersion)
	if err != nil {
		return Report{}, err
	}
	report := Report{
		PolicyVersion: policy.Version, PolicyHash: policy.Hash, Stage: input.Stage,
		TextHash: agentdomain.SHA256Hex([]byte(input.Text)), RuneCount: utf8.RuneCountInString(input.Text),
		Rules: make([]RuleBinding, 0, len(policy.Rules)),
	}
	for _, rule := range policy.Rules {
		report.Rules = append(report.Rules, RuleBinding{
			ID: rule.ID, Version: rule.Version, Category: rule.Category, Severity: rule.Severity,
		})
		matches := evaluateRule(policy, rule.ID, input)
		if len(matches) > maxFindingsPerRule {
			matches = matches[:maxFindingsPerRule]
			report.Truncated = true
		}
		for index, current := range matches {
			if len(report.Findings) == agentdomain.MaxReviewFindings {
				report.Truncated = true
				break
			}
			constraintRef := current.constraintRef
			if constraintRef == "" && current.start == nil {
				constraintRef = "rule:" + rule.ID
			}
			finding := agentdomain.ReviewFinding{
				ID:       fmt.Sprintf("%s:%s:%03d", input.Namespace, rule.ID, index+1),
				Category: rule.Category, Severity: rule.Severity, Status: agentdomain.FindingOpen,
				Source: agentdomain.FindingDeterministicLint, Message: current.message,
				Suggestion: current.suggestion, Quote: current.quote, ScopeStart: current.start,
				ScopeEnd: current.end, ConstraintRef: constraintRef,
			}
			if finding.Validate() != nil {
				return Report{}, ErrInvalidPolicy
			}
			report.Findings = append(report.Findings, finding)
		}
	}
	return report, nil
}

const maxFindingsPerRule = 10

func evaluateRule(policy Policy, ruleID string, input Input) []match {
	switch ruleID {
	case "structure.non_empty":
		if strings.TrimSpace(input.Text) == "" {
			return []match{{message: "正文为空。", suggestion: "生成非空正文。"}}
		}
	case "length.target_deviation":
		return lengthMatches(policy, input)
	case "format.code_fence":
		return phraseMatches(input.Text, []string{"```"}, "正文包含 Markdown 代码围栏。", "删除代码围栏，只保留正文。", "")
	case "format.meta_narration":
		return phraseMatches(input.Text, policy.MetaPhrases, "正文包含模型自述或提示词元话语。", "删除元话语并直接进入叙事。", "")
	case "constraint.required_anchor":
		return missingConstraintMatches(input.Text, input.Spec.RequiredAnchors)
	case "constraint.forbidden_item":
		return presentConstraintMatches(input.Text, input.Spec.ForbiddenItems, "正文出现禁止项。", "forbidden:")
	case "originality.reserved_name":
		return presentConstraintMatches(input.Text, input.Spec.ReservedNames, "正文出现测试集保留专名。", "reserved_name:")
	case "repetition.consecutive_sentence":
		return consecutiveSentenceMatches(input.Text)
	case "repetition.duplicate_paragraph":
		return duplicateParagraphMatches(input.Text)
	case "repetition.high_frequency_phrase":
		return highFrequencyPhraseMatches(policy, input.Text)
	case "style.short_paragraph_ratio":
		return shortParagraphMatches(policy, input.Text)
	case "style.repeated_opening":
		return repeatedOpeningMatches(policy, input.Text)
	case "style.mechanical_parallel":
		return mechanicalParallelMatches(policy, input.Text)
	case "style.summary_tone":
		return phraseMatches(input.Text, policy.SummaryPhrases, "正文包含总结腔或模板化表达。", "改为具体动作、感受或场景证据。", "")
	}
	return nil
}

func lengthMatches(policy Policy, input Input) []match {
	if input.Spec.TargetRunes == 0 {
		return nil
	}
	count := utf8.RuneCountInString(strings.TrimSpace(input.Text))
	minimum := int(float64(input.Spec.TargetRunes) * policy.MinLengthRatio)
	maximum := int(float64(input.Spec.TargetRunes) * policy.MaxLengthRatio)
	if count >= minimum && count <= maximum {
		return nil
	}
	return []match{{
		message:    fmt.Sprintf("正文长度为 %d 字符，目标 %d，允许范围 %d–%d。", count, input.Spec.TargetRunes, minimum, maximum),
		suggestion: "在不改变硬约束的前提下调整正文长度。", constraintRef: "target_length",
	}}
}

func missingConstraintMatches(text string, constraints []Constraint) []match {
	result := make([]match, 0)
	for _, constraint := range constraints {
		if !strings.Contains(text, constraint.Text) {
			result = append(result, match{
				message:    "正文未满足必须出现的硬锚点：" + constraint.Text,
				suggestion: "在正文中明确落实该硬锚点。", constraintRef: "anchor:" + constraint.ID,
			})
		}
	}
	return result
}

func presentConstraintMatches(text string, constraints []Constraint, message, constraintPrefix string) []match {
	result := make([]match, 0)
	for _, constraint := range constraints {
		for _, span := range runeOccurrences(text, constraint.Text) {
			start, end := span[0], span[1]
			result = append(result, match{
				message: message + constraint.Text, suggestion: "删除或改写该明确命中。",
				quote: constraint.Text, start: &start, end: &end, constraintRef: constraintPrefix + constraint.ID,
			})
		}
	}
	return result
}

func phraseMatches(text string, phrases []string, message, suggestion, constraintRef string) []match {
	result := make([]match, 0)
	for _, phrase := range phrases {
		for _, span := range runeOccurrences(text, phrase) {
			start, end := span[0], span[1]
			result = append(result, match{message: message, suggestion: suggestion, quote: phrase, start: &start, end: &end, constraintRef: constraintRef})
		}
	}
	sort.SliceStable(result, func(left, right int) bool { return *result[left].start < *result[right].start })
	return result
}

type segment struct {
	text       string
	start, end int
}

func sentences(text string) []segment {
	runes := []rune(text)
	result := make([]segment, 0)
	start := 0
	for index, current := range runes {
		if current != '。' && current != '！' && current != '？' && current != '!' && current != '?' {
			continue
		}
		appendSegment(&result, runes, start, index+1)
		start = index + 1
	}
	appendSegment(&result, runes, start, len(runes))
	return result
}

func appendSegment(target *[]segment, runes []rune, start, end int) {
	for start < end && unicode.IsSpace(runes[start]) {
		start++
	}
	for end > start && unicode.IsSpace(runes[end-1]) {
		end--
	}
	if start < end {
		*target = append(*target, segment{text: string(runes[start:end]), start: start, end: end})
	}
}

func consecutiveSentenceMatches(text string) []match {
	items := sentences(text)
	result := make([]match, 0)
	for index := 1; index < len(items); index++ {
		if items[index-1].text != items[index].text {
			continue
		}
		start, end := items[index].start, items[index].end
		result = append(result, match{message: "出现连续重复句。", suggestion: "删除重复句或补充新的动作与信息。", quote: items[index].text, start: &start, end: &end})
	}
	return result
}

func paragraphs(text string) []segment {
	runes := []rune(text)
	result := make([]segment, 0)
	start := 0
	for index := 0; index < len(runes); index++ {
		if runes[index] != '\n' {
			continue
		}
		next := index + 1
		for next < len(runes) && (runes[next] == '\r' || runes[next] == ' ' || runes[next] == '\t') {
			next++
		}
		if next < len(runes) && runes[next] == '\n' {
			appendSegment(&result, runes, start, index)
			index = next
			for index+1 < len(runes) && (runes[index+1] == '\r' || runes[index+1] == '\n' || runes[index+1] == ' ' || runes[index+1] == '\t') {
				index++
			}
			start = index + 1
		}
	}
	appendSegment(&result, runes, start, len(runes))
	return result
}

func duplicateParagraphMatches(text string) []match {
	items := paragraphs(text)
	seen := map[string]struct{}{}
	result := make([]match, 0)
	for _, item := range items {
		if _, exists := seen[item.text]; !exists {
			seen[item.text] = struct{}{}
			continue
		}
		start, end := item.start, item.end
		result = append(result, match{message: "出现重复段落。", suggestion: "删除重复段或补充新的叙事信息。", quote: item.text, start: &start, end: &end})
	}
	return result
}

func shortParagraphMatches(policy Policy, text string) []match {
	items := paragraphs(text)
	if len(items) < policy.ShortParagraphMinCount {
		return nil
	}
	short := 0
	for _, item := range items {
		if utf8.RuneCountInString(item.text) <= policy.ShortParagraphMaxRunes {
			short++
		}
	}
	if float64(short)/float64(len(items)) < policy.ShortParagraphRatio {
		return nil
	}
	return []match{{
		message:    fmt.Sprintf("%d/%d 个段落短于或等于 %d 字符，短段落比例异常。", short, len(items), policy.ShortParagraphMaxRunes),
		suggestion: "合并缺少独立叙事作用的短段落。", constraintRef: "paragraph_ratio",
	}}
}

func repeatedOpeningMatches(policy Policy, text string) []match {
	items := sentences(text)
	if len(items) < policy.RepeatedOpeningCount {
		return nil
	}
	for start := 0; start+policy.RepeatedOpeningCount <= len(items); start++ {
		prefix := opening(items[start].text, policy.RepeatedOpeningRunes)
		if prefix == "" {
			continue
		}
		matched := true
		for offset := 1; offset < policy.RepeatedOpeningCount; offset++ {
			if opening(items[start+offset].text, policy.RepeatedOpeningRunes) != prefix {
				matched = false
				break
			}
		}
		if matched {
			from, to := items[start].start, items[start+policy.RepeatedOpeningCount-1].end
			return []match{{message: "连续句子使用相同开头：" + prefix, suggestion: "调整句式起点与叙述焦点。", quote: string([]rune(text)[from:to]), start: &from, end: &to}}
		}
	}
	return nil
}

func opening(text string, count int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) < count {
		return ""
	}
	return string(runes[:count])
}

func mechanicalParallelMatches(policy Policy, text string) []match {
	items := sentences(text)
	matches := make([]segment, 0)
	for _, item := range items {
		if strings.Contains(item.text, "不是") && strings.Contains(item.text, "而是") {
			matches = append(matches, item)
		}
	}
	if len(matches) < policy.MechanicalParallelCount {
		return nil
	}
	from, to := matches[0].start, matches[len(matches)-1].end
	return []match{{message: "连续使用“不是……而是……”机械排比。", suggestion: "保留必要对比，改用动作与场景推进其余信息。", quote: string([]rune(text)[from:to]), start: &from, end: &to}}
}

func highFrequencyPhraseMatches(policy Policy, text string) []match {
	items := sentences(text)
	type phraseStat struct {
		count, start int
	}
	stats := map[string]phraseStat{}
	for _, item := range items {
		runes := []rune(item.text)
		seen := map[string]struct{}{}
		for index := 0; index+policy.HighFrequencyPhraseRunes <= len(runes); index++ {
			window := runes[index : index+policy.HighFrequencyPhraseRunes]
			if containsPunctuation(window) {
				continue
			}
			phrase := string(window)
			if _, exists := seen[phrase]; exists {
				continue
			}
			seen[phrase] = struct{}{}
			stat := stats[phrase]
			if stat.count == 0 {
				stat.start = item.start + index
			}
			stat.count++
			stats[phrase] = stat
		}
	}
	bestPhrase := ""
	best := phraseStat{}
	for phrase, stat := range stats {
		if stat.count < policy.HighFrequencyPhraseCount {
			continue
		}
		if bestPhrase == "" || stat.count > best.count || (stat.count == best.count && stat.start < best.start) ||
			(stat.count == best.count && stat.start == best.start && phrase < bestPhrase) {
			bestPhrase, best = phrase, stat
		}
	}
	if bestPhrase == "" {
		return nil
	}
	start, end := best.start, best.start+policy.HighFrequencyPhraseRunes
	return []match{{message: fmt.Sprintf("短语“%s”在多个句子中重复 %d 次。", bestPhrase, best.count), suggestion: "删除无信息重复或改用具体变化。", quote: bestPhrase, start: &start, end: &end}}
}

func containsPunctuation(runes []rune) bool {
	for _, current := range runes {
		if unicode.IsPunct(current) || unicode.IsSpace(current) {
			return true
		}
	}
	return false
}

func runeOccurrences(text, needle string) [][2]int {
	haystack, target := []rune(text), []rune(needle)
	if len(target) == 0 || len(target) > len(haystack) {
		return nil
	}
	result := make([][2]int, 0)
	for index := 0; index+len(target) <= len(haystack); index++ {
		if string(haystack[index:index+len(target)]) == string(target) {
			result = append(result, [2]int{index, index + len(target)})
			index += len(target) - 1
		}
	}
	return result
}

func validStage(stage Stage) bool {
	return stage == StageBaseline || stage == StageInitial || stage == StageFinal
}
