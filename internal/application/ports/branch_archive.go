package ports

import (
	"time"

	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
)

type BranchArchive interface {
	Prepare(directory string, id string) (string, error)
	DiscardPrepared(directory string, id string) error
	Initialize(directory, projectID, mainID, databasePath string, now time.Time) (revisiondomain.BranchCatalog, error)
	Load(directory string) (revisiondomain.BranchCatalog, error)
	HeadPath(directory, branchID string) string
	Checkpoint(directory, branchID, snapshotPath string, now time.Time) (revisiondomain.Branch, error)
	Fork(directory, id, name, description, parentID, snapshotPath string, now time.Time) (revisiondomain.Branch, error)
	Switch(directory, databasePath, targetID, currentSnapshotPath string, now time.Time) (revisiondomain.BranchCatalog, error)
	PlanMerge(directory, sourceID, targetID string) (revisiondomain.BranchMergePlan, error)
	Merge(directory, databasePath string, plan revisiondomain.BranchMergePlan, currentSnapshotPath string, now time.Time) (revisiondomain.BranchCatalog, error)
	Delete(directory, id string) error
	RecoveryRequired(directory, databasePath string) (bool, error)
	RecoverInterruptedOperation(directory, databasePath string) error
}
