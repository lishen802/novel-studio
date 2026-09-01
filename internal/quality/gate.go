package quality

import (
	"errors"
	"fmt"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

var ErrInvalidReconciliation = errors.New("invalid quality reconciliation")

type SeverityCounts struct {
	Blocker int `json:"blocker"`
	Major   int `json:"major"`
	Minor   int `json:"minor"`
	Note    int `json:"note"`
}

type GateReport struct {
	Stage            Stage                       `json:"stage"`
	Status           agentdomain.QualityStatus   `json:"status"`
	ReviewComplete   bool                        `json:"review_complete"`
	BudgetSufficient bool                        `json:"budget_sufficient"`
	Unresolved       SeverityCounts              `json:"unresolved"`
	Findings         []agentdomain.ReviewFinding `json:"findings"`
	Verification     *VerificationReport         `json:"verification,omitempty"`
}

// Decide 依据未解决严重度和执行完整性给出不可被模型评分覆盖的质量结论。
func Decide(stage Stage, findings []agentdomain.ReviewFinding, reviewComplete, budgetSufficient bool) GateReport {
	report := GateReport{
		Stage: stage, Status: agentdomain.QualityPassed, ReviewComplete: reviewComplete,
		BudgetSufficient: budgetSufficient, Findings: append([]agentdomain.ReviewFinding(nil), findings...),
	}
	for _, finding := range findings {
		if finding.Status == agentdomain.FindingResolved {
			continue
		}
		switch finding.Severity {
		case agentdomain.SeverityBlocker:
			report.Unresolved.Blocker++
		case agentdomain.SeverityMajor:
			report.Unresolved.Major++
		case agentdomain.SeverityMinor:
			report.Unresolved.Minor++
		case agentdomain.SeverityNote:
			report.Unresolved.Note++
		}
	}
	if report.Unresolved.Blocker > 0 {
		report.Status = agentdomain.QualityBlocked
	} else if report.Unresolved.Major > 0 || !reviewComplete || !budgetSufficient {
		report.Status = agentdomain.QualityNeedsReview
	}
	return report
}

// VerificationReport 记录修订自述与独立验证结论的分歧，用于判断修订节点是在改进质量还是在制造通过假象。
type VerificationReport struct {
	ClaimedResolved   int `json:"claimed_resolved"`
	ConfirmedResolved int `json:"confirmed_resolved"`
	Overturned        int `json:"overturned"`
	Regressions       int `json:"regressions"`
}

// ReconcileVerified 以独立验证结论合并修订结果。
// 修订模型自述的处置状态只是待核实声明，单独不足以让问题退出未解决计数；
// 验证者也不能把修订模型自己都未声称解决的问题升级为已解决。
func ReconcileVerified(
	before []agentdomain.ReviewFinding,
	resolutions []agentdomain.FindingResolution,
	verdicts []agentdomain.ResolutionVerdict,
	regressions []agentdomain.ReviewFinding,
	finalLint []agentdomain.ReviewFinding,
) ([]agentdomain.ReviewFinding, VerificationReport, error) {
	if len(before) != len(resolutions) || len(before) != len(verdicts) {
		return nil, VerificationReport{}, ErrInvalidReconciliation
	}
	resolutionByID := make(map[string]agentdomain.FindingResolution, len(resolutions))
	for _, resolution := range resolutions {
		if _, exists := resolutionByID[resolution.FindingID]; exists {
			return nil, VerificationReport{}, ErrInvalidReconciliation
		}
		resolutionByID[resolution.FindingID] = resolution
	}
	verdictByID := make(map[string]agentdomain.ResolutionVerdict, len(verdicts))
	for _, verdict := range verdicts {
		if _, exists := verdictByID[verdict.FindingID]; exists {
			return nil, VerificationReport{}, ErrInvalidReconciliation
		}
		verdictByID[verdict.FindingID] = verdict
	}
	report := VerificationReport{}
	result := make([]agentdomain.ReviewFinding, 0, len(before)+len(regressions)+len(finalLint))
	seen := make(map[string]struct{}, len(before)+len(regressions)+len(finalLint))
	for _, finding := range before {
		resolution, exists := resolutionByID[finding.ID]
		if !exists || resolution.Status == agentdomain.FindingOpen {
			return nil, VerificationReport{}, fmt.Errorf("%w: missing resolution for %s", ErrInvalidReconciliation, finding.ID)
		}
		verdict, verified := verdictByID[finding.ID]
		if !verified {
			return nil, VerificationReport{}, fmt.Errorf("%w: missing verdict for %s", ErrInvalidReconciliation, finding.ID)
		}
		claimed := resolution.Status == agentdomain.FindingResolved
		if claimed {
			report.ClaimedResolved++
		}
		finding.Status = conservativeStatus(resolution.Status, verdict.VerifiedStatus)
		switch {
		case claimed && finding.Status == agentdomain.FindingResolved:
			report.ConfirmedResolved++
		case claimed:
			report.Overturned++
		}
		result = append(result, finding)
		seen[finding.ID] = struct{}{}
	}
	for _, group := range [][]agentdomain.ReviewFinding{regressions, finalLint} {
		for _, finding := range group {
			if _, exists := seen[finding.ID]; exists {
				return nil, VerificationReport{}, fmt.Errorf("%w: duplicate finding %s", ErrInvalidReconciliation, finding.ID)
			}
			result = append(result, finding)
			seen[finding.ID] = struct{}{}
		}
	}
	report.Regressions = len(regressions)
	return result, report, nil
}

// conservativeStatus 只在修订自述与独立验证同时判定已解决时才认可解决。
func conservativeStatus(claimed, verified agentdomain.FindingStatus) agentdomain.FindingStatus {
	if claimed != agentdomain.FindingResolved {
		return claimed
	}
	if verified == agentdomain.FindingResolved {
		return agentdomain.FindingResolved
	}
	if verified == agentdomain.FindingOpen {
		return agentdomain.FindingOpen
	}
	return verified
}

// Reconcile 将修订处理状态应用到原问题，并追加最终 lint 新发现。
func Reconcile(
	before []agentdomain.ReviewFinding,
	resolutions []agentdomain.FindingResolution,
	finalLint []agentdomain.ReviewFinding,
) ([]agentdomain.ReviewFinding, error) {
	if len(before) != len(resolutions) {
		return nil, ErrInvalidReconciliation
	}
	resolutionByID := make(map[string]agentdomain.FindingResolution, len(resolutions))
	for _, resolution := range resolutions {
		if _, exists := resolutionByID[resolution.FindingID]; exists {
			return nil, ErrInvalidReconciliation
		}
		resolutionByID[resolution.FindingID] = resolution
	}
	result := make([]agentdomain.ReviewFinding, 0, len(before)+len(finalLint))
	seen := make(map[string]struct{}, len(before)+len(finalLint))
	for _, finding := range before {
		resolution, exists := resolutionByID[finding.ID]
		if !exists || resolution.Status == agentdomain.FindingOpen {
			return nil, fmt.Errorf("%w: missing resolution for %s", ErrInvalidReconciliation, finding.ID)
		}
		finding.Status = resolution.Status
		result = append(result, finding)
		seen[finding.ID] = struct{}{}
	}
	for _, finding := range finalLint {
		if _, exists := seen[finding.ID]; exists {
			return nil, fmt.Errorf("%w: duplicate finding %s", ErrInvalidReconciliation, finding.ID)
		}
		result = append(result, finding)
		seen[finding.ID] = struct{}{}
	}
	return result, nil
}
