package revision

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrBlankBranchID       = errors.New("branch id must not be blank")
	ErrBlankBranchName     = errors.New("branch name must not be blank")
	ErrBlankBranchDigest   = errors.New("branch digest must not be blank")
	ErrInvalidBranchTarget = errors.New("branch can only merge into its direct parent")
	ErrBranchDiverged      = errors.New("target branch changed after source branch was forked")
	ErrBranchUnchanged     = errors.New("source branch has no changes to merge")
	ErrStaleBranchMerge    = errors.New("branch merge preview is stale")
)

type Branch struct {
	ID             string    `json:"id"`
	ProjectID      string    `json:"projectId"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	ParentID       string    `json:"parentId"`
	UpstreamDigest string    `json:"upstreamDigest"`
	HeadDigest     string    `json:"headDigest"`
	Generation     int64     `json:"generation"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type BranchMergePlan struct {
	SourceID         string
	TargetID         string
	SourceDigest     string
	TargetDigest     string
	SourceGeneration int64
	TargetGeneration int64
}

type BranchCatalog struct {
	Revision       int64
	ProjectID      string
	ActiveBranchID string
	Branches       []Branch
}

func (c BranchCatalog) Find(id string) (Branch, bool) {
	for _, branch := range c.Branches {
		if branch.ID == id {
			return branch, true
		}
	}
	return Branch{}, false
}

func NewMainBranch(id, projectID, headDigest string, now time.Time) (Branch, error) {
	return newBranch(id, projectID, "主分支", "", "", "", headDigest, now)
}

func ForkBranch(id, name, description string, parent Branch, now time.Time) (Branch, error) {
	return newBranch(
		id,
		parent.ProjectID,
		name,
		description,
		parent.ID,
		parent.HeadDigest,
		parent.HeadDigest,
		now,
	)
}

func newBranch(
	id, projectID, name, description, parentID, upstreamDigest, headDigest string,
	now time.Time,
) (Branch, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Branch{}, ErrBlankBranchID
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return Branch{}, errors.New("branch project id must not be blank")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Branch{}, ErrBlankBranchName
	}
	headDigest = strings.TrimSpace(headDigest)
	if headDigest == "" {
		return Branch{}, ErrBlankBranchDigest
	}
	return Branch{
		ID:             id,
		ProjectID:      projectID,
		Name:           name,
		Description:    strings.TrimSpace(description),
		ParentID:       strings.TrimSpace(parentID),
		UpstreamDigest: strings.TrimSpace(upstreamDigest),
		HeadDigest:     headDigest,
		Generation:     1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

func (b Branch) Checkpoint(headDigest string, now time.Time) Branch {
	headDigest = strings.TrimSpace(headDigest)
	if headDigest == "" || headDigest == b.HeadDigest {
		return b
	}
	b.HeadDigest = headDigest
	b.Generation++
	b.UpdatedAt = now
	return b
}

func PlanFastForwardMerge(source, target Branch) (BranchMergePlan, error) {
	if source.ParentID == "" || source.ParentID != target.ID ||
		source.ProjectID != target.ProjectID {
		return BranchMergePlan{}, ErrInvalidBranchTarget
	}
	if source.HeadDigest == source.UpstreamDigest {
		return BranchMergePlan{}, ErrBranchUnchanged
	}
	if target.HeadDigest != source.UpstreamDigest {
		return BranchMergePlan{}, ErrBranchDiverged
	}
	return BranchMergePlan{
		SourceID:         source.ID,
		TargetID:         target.ID,
		SourceDigest:     source.HeadDigest,
		TargetDigest:     target.HeadDigest,
		SourceGeneration: source.Generation,
		TargetGeneration: target.Generation,
	}, nil
}

func CompleteFastForwardMerge(
	source, target Branch,
	plan BranchMergePlan,
	now time.Time,
) (Branch, Branch, error) {
	current, err := PlanFastForwardMerge(source, target)
	if err != nil {
		return Branch{}, Branch{}, err
	}
	if current != plan {
		return Branch{}, Branch{}, ErrStaleBranchMerge
	}
	target = target.Checkpoint(source.HeadDigest, now)
	source.UpstreamDigest = target.HeadDigest
	source.UpdatedAt = now
	return source, target, nil
}
