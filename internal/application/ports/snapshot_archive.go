package ports

import revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"

type SnapshotArchive interface {
	Prepare(directory string, id string) (string, error)
	Record(directory string, snapshot revisiondomain.Snapshot) error
	List(directory string) ([]revisiondomain.Snapshot, error)
	Find(directory string, id string) (revisiondomain.Snapshot, string, error)
	Discard(directory string, id string) error
	BeginRestore(databasePath string, snapshotPath string) error
	CommitRestore(databasePath string) error
	RollbackRestore(databasePath string) error
	RecoveryRequired(databasePath string) (bool, error)
	RecoverInterruptedRestore(databasePath string) error
}
