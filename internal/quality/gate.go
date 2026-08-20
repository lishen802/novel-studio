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
