package agentkernel

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

// revisionChangeView 是提供给验证者的改动明细。
// 只保留“针对哪条问题改了什么”，不含修订模型的处置结论与说明，
// 避免验证退化成认可修订模型的说辞。
type revisionChangeView struct {
	FindingID    string `json:"finding_id"`
	OriginalText string `json:"original_text"`
	RevisedText  string `json:"revised_text"`
}

// verificationInputs 汇集修订复核所需的原始问题、修订产物与改动明细。
func verificationInputs(
	node FrozenNode,
	latest map[string]agentdomain.RunStep,
	artifacts []agentdomain.Artifact,
) ([]agentdomain.ReviewFinding, []agentdomain.FindingResolution, []agentdomain.RevisionChange, string, error) {
	revised, reviewJSON, err := revisionInputs(node, latest, artifacts)
	if err != nil {
		return nil, nil, nil, "", err
	}
	decodedReview, err := agentdomain.DecodeTaskOutput(agentdomain.TaskReviewBundle, []byte(reviewJSON))
	if err != nil {
		return nil, nil, nil, "", err
	}
	resolutions, changes, err := revisionTrace(node, latest, artifacts)
	if err != nil {
		return nil, nil, nil, "", err
	}
	return decodedReview.(*agentdomain.ReviewBundleOutput).Findings, resolutions, changes, revised, nil
}

// revisionTrace 从修订节点的结构化输出中取回逐项处置与变更证据。
func revisionTrace(
	node FrozenNode,
	latest map[string]agentdomain.RunStep,
	artifacts []agentdomain.Artifact,
) ([]agentdomain.FindingResolution, []agentdomain.RevisionChange, error) {
	for _, selector := range node.InputSelectors {
		if !strings.HasPrefix(selector, "node:") {
			continue
		}
		step, exists := latest[strings.TrimPrefix(selector, "node:")]
		if !exists || step.Capability != agentdomain.CapabilityBoundedRevision {
			continue
		}
		for _, artifact := range artifacts {
			if artifact.StepID != step.ID || artifact.Kind != agentdomain.ArtifactStructuredOutput {
				continue
			}
			for _, kind := range []agentdomain.TaskKind{agentdomain.TaskReviseDraft, agentdomain.TaskReviseBlueprint} {
				decoded, err := agentdomain.DecodeTaskOutput(kind, []byte(artifact.Content))
				if err != nil {
					continue
				}
				switch value := decoded.(type) {
				case *agentdomain.ReviseDraftOutput:
					return value.Resolutions, value.Changes, nil
				case *agentdomain.ReviseBlueprintOutput:
					return value.Resolutions, value.Changes, nil
				}
			}
		}
	}
	return nil, nil, errors.New("missing revision trace")
}

// changeEvidence 把变更证据序列化为验证者可读的输入，剥离处置状态与说明。
func changeEvidence(changes []agentdomain.RevisionChange) (string, error) {
	views := make([]revisionChangeView, 0, len(changes))
	for _, change := range changes {
		views = append(views, revisionChangeView{
			FindingID: change.FindingID, OriginalText: change.OriginalText, RevisedText: change.RevisedText,
		})
	}
	encoded, err := json.Marshal(views)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// decodeRevisionVerdictOutput 在领域校验前把验证结论对齐到冻结问题清单与修订稿证据。
func decodeRevisionVerdictOutput(
	node FrozenNode,
	latest map[string]agentdomain.RunStep,
	artifacts []agentdomain.Artifact,
	content string,
) (agentdomain.TaskOutput, error) {
	var verdict agentdomain.RevisionVerdictOutput
	if err := agentdomain.UnmarshalModelJSON([]byte(content), &verdict); err != nil {
		return nil, err
	}
	findings, _, _, revised, err := verificationInputs(node, latest, artifacts)
	if err != nil {
		return nil, err
	}
	if err := normalizeRevisionVerdict(&verdict, findings, revised); err != nil {
		return nil, err
	}
	if err := verdict.Validate(); err != nil {
		return nil, err
	}
	return &verdict, nil
}

// normalizeRevisionVerdict 把验证结论收口为可信状态：
// 裁定“已解决”必须能在修订稿中定位到证据，无法定位一律退回未解决。
func normalizeRevisionVerdict(
	verdict *agentdomain.RevisionVerdictOutput,
	findings []agentdomain.ReviewFinding,
	revised string,
) error {
	if verdict == nil || len(findings) == 0 || strings.TrimSpace(revised) == "" {
		return agentdomain.ErrInvalidTaskOutput
	}
	assigned := assignVerdicts(findings, verdict.Verdicts)
	revisedRunes := []rune(revised)
	for index := range assigned {
		item := &assigned[index]
		if strings.TrimSpace(item.Rationale) == "" {
			item.Rationale = "验证者未提供裁定理由。"
		}
		if !validVerdictStatus(item.VerifiedStatus) {
			item.VerifiedStatus = agentdomain.FindingOpen
		}
		alignVerdictEvidence(item, revisedRunes)
		if item.VerifiedStatus == agentdomain.FindingResolved && item.ScopeStart == nil {
			// 无法在修订稿中定位证据的“已解决”不予采信。
			item.VerifiedStatus = agentdomain.FindingOpen
			item.Rationale = "验证者未能在修订稿中提供可定位的解决证据。"
		}
	}
	verdict.Verdicts = assigned
	if strings.TrimSpace(verdict.Summary) == "" {
		verdict.Summary = fmt.Sprintf("修订复核裁定 %d 个问题。", len(assigned))
	}
	return normalizeRegressions(verdict, findings, revised)
}

// assignVerdicts 优先保留正确 ID，再按顺序补位；缺失裁定按未解决处理。
func assignVerdicts(
	findings []agentdomain.ReviewFinding,
	model []agentdomain.ResolutionVerdict,
) []agentdomain.ResolutionVerdict {
	assigned := make([]agentdomain.ResolutionVerdict, len(findings))
	matched := make([]bool, len(findings))
	used := make([]bool, len(model))
	for findingIndex, finding := range findings {
		for modelIndex, candidate := range model {
			if !used[modelIndex] && candidate.FindingID == finding.ID {
				candidate.FindingID = finding.ID
				assigned[findingIndex] = candidate
				matched[findingIndex] = true
				used[modelIndex] = true
				break
			}
		}
	}
	for findingIndex, finding := range findings {
		if matched[findingIndex] {
			continue
		}
		modelIndex := nextUnusedVerdict(used)
		if modelIndex < 0 {
			assigned[findingIndex] = agentdomain.ResolutionVerdict{
				FindingID: finding.ID, VerifiedStatus: agentdomain.FindingOpen,
				Rationale: "验证者未返回该问题的裁定。",
			}
			continue
		}
		candidate := model[modelIndex]
		candidate.FindingID = finding.ID
		assigned[findingIndex] = candidate
		used[modelIndex] = true
	}
	return assigned
}

func nextUnusedVerdict(used []bool) int {
	for index := range used {
		if !used[index] {
			return index
		}
	}
	return -1
}

// alignVerdictEvidence 用确定性匹配重算引文范围；模型不负责可靠计算 Unicode 偏移。
func alignVerdictEvidence(verdict *agentdomain.ResolutionVerdict, revised []rune) {
	quote := strings.TrimSpace(verdict.Quote)
	if quote == "" {
		verdict.Quote, verdict.ScopeStart, verdict.ScopeEnd = "", nil, nil
		return
	}
	starts := quoteRuneStarts(revised, []rune(quote))
	start, ok := chooseQuoteStart(starts, verdict.ScopeStart)
	if !ok {
		verdict.Quote, verdict.ScopeStart, verdict.ScopeEnd = "", nil, nil
		return
	}
	end := start + len([]rune(quote))
	verdict.Quote, verdict.ScopeStart, verdict.ScopeEnd = quote, &start, &end
}

// normalizeRegressions 为修订引入的新问题生成不与原问题冲突的确定性 ID 与可追溯证据。
func normalizeRegressions(
	verdict *agentdomain.RevisionVerdictOutput,
	findings []agentdomain.ReviewFinding,
	revised string,
) error {
	if len(verdict.Regressions) == 0 {
		verdict.Regressions = nil
		return nil
	}
	reserved := make(map[string]struct{}, len(findings))
	for _, finding := range findings {
		reserved[finding.ID] = struct{}{}
	}
	normalized := make([]agentdomain.ReviewFinding, 0, len(verdict.Regressions))
	for index := range verdict.Regressions {
		regression := verdict.Regressions[index]
		regression.ID = regressionFindingID(index, reserved)
		regression.Source = agentdomain.FindingModelReview
		regression.Status = agentdomain.FindingOpen
		if err := normalizeFindingEvidence(&regression, revised, "revision:document"); err != nil {
			return err
		}
		if regression.Validate() != nil {
			continue
		}
		normalized = append(normalized, regression)
	}
	if len(normalized) == 0 {
		verdict.Regressions = nil
		return nil
	}
	verdict.Regressions = normalized
	return nil
}

func regressionFindingID(index int, reserved map[string]struct{}) string {
	for suffix := index + 1; ; suffix++ {
		id := fmt.Sprintf("revision-regression-%d", suffix)
		if _, exists := reserved[id]; !exists {
			reserved[id] = struct{}{}
			return id
		}
	}
}

func validVerdictStatus(status agentdomain.FindingStatus) bool {
	return status == agentdomain.FindingOpen || status == agentdomain.FindingResolved ||
		status == agentdomain.FindingRetained || status == agentdomain.FindingCannotResolve
}
