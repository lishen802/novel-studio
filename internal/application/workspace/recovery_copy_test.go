package workspace

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	"github.com/lishen802/novel-studio/internal/application/ports"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
)

func TestCreateRecoveryCopySnapshotsSourceThroughReadOnlyStoreThenOpensPublishedCopy(t *testing.T) {
	project, err := projectdomain.NewProject("project-copy", "副本", "奇幻", "", time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	sourceStore := newFakeStore(project)
	copyStore := newFakeStore(project)
	source := fakeProjectPackage("/source.zjproject")
	published := fakeProjectPackage("/copies/source-recovery.zjproject")
	packages := &fakePackageManager{openPackage: source, copyPackage: published}
	factory := &fakeStoreFactory{readOnlyStore: sourceStore, openStore: copyStore}
	service := NewService(packages, factory, license.NoopService{}, fixedClock{now: time.Unix(101, 0)}, &sequenceIDs{}, &fakeSnapshotArchive{})
	leaseManager := &fakeProjectLeaseManager{}
	service.ConfigureProjectLease(leaseManager, ports.ProjectLeaseMetadata{})

	result, err := service.CreateProjectRecoveryCopy(context.Background(), source.Root, filepath.Dir(published.Root))
	if err != nil {
		t.Fatalf("CreateProjectRecoveryCopy: %v", err)
	}
	if result.Path != published.Root || result.Project == nil || result.Project.Path != published.Root {
		t.Fatalf("result = %#v", result)
	}
	if sourceStore.backupPath == "" || filepath.Dir(sourceStore.backupPath) == source.Root {
		t.Fatalf("source backup path = %q, want published temporary package", sourceStore.backupPath)
	}
	if sourceStore.closeCalls != 1 {
		t.Fatalf("read-only source close calls = %d, want 1", sourceStore.closeCalls)
	}
	if factory.openCalls != 1 {
		t.Fatalf("editable open calls = %d, want 1", factory.openCalls)
	}
	if len(leaseManager.acquired) != 1 || leaseManager.acquired[0] != published.Root {
		t.Fatalf("copy lease acquisitions = %#v", leaseManager.acquired)
	}
}

func TestCreateRecoveryCopyKeepsValidatedCopyWhenEditableOpenFails(t *testing.T) {
	project, err := projectdomain.NewProject("project-copy", "副本", "奇幻", "", time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	source := fakeProjectPackage("/source.zjproject")
	published := fakeProjectPackage("/copies/source-recovery.zjproject")
	packages := &fakePackageManager{openPackage: source, copyPackage: published}
	factory := &fakeStoreFactory{readOnlyStore: newFakeStore(project), openErr: errors.New("target opening failed")}
	service := NewService(packages, factory, license.NoopService{}, fixedClock{now: time.Unix(101, 0)}, &sequenceIDs{}, &fakeSnapshotArchive{})

	result, err := service.CreateProjectRecoveryCopy(context.Background(), source.Root, filepath.Dir(published.Root))
	if err == nil || !errors.Is(err, factory.openErr) {
		t.Fatalf("CreateProjectRecoveryCopy error = %v, want target opening failure", err)
	}
	if result.Path != published.Root || result.Project != nil {
		t.Fatalf("partial recovery copy result = %#v", result)
	}
	if packages.copyCalls != 1 {
		t.Fatalf("copy calls = %d, want 1", packages.copyCalls)
	}
}

func TestCreateRecoveryCopyRetainsPublishedPathWhenReadOnlySourceCloseFails(t *testing.T) {
	project, err := projectdomain.NewProject("project-copy", "副本", "奇幻", "", time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	source := fakeProjectPackage("/source.zjproject")
	published := fakeProjectPackage("/copies/source-recovery.zjproject")
	closeErr := errors.New("read-only close failed")
	sourceStore := newFakeStore(project)
	sourceStore.closeErr = closeErr
	packages := &fakePackageManager{openPackage: source, copyPackage: published}
	factory := &fakeStoreFactory{readOnlyStore: sourceStore, openStore: newFakeStore(project)}
	service := NewService(packages, factory, license.NoopService{}, fixedClock{now: time.Unix(101, 0)}, &sequenceIDs{}, &fakeSnapshotArchive{})

	result, err := service.CreateProjectRecoveryCopy(context.Background(), source.Root, filepath.Dir(published.Root))
	if !errors.Is(err, closeErr) {
		t.Fatalf("CreateProjectRecoveryCopy error = %v, want source close error", err)
	}
	if result.Path != published.Root || result.Project != nil {
		t.Fatalf("published copy result lost after close error: %#v", result)
	}
	if factory.openCalls != 0 {
		t.Fatalf("editable open calls = %d, want no open after close failure", factory.openCalls)
	}
}

func TestCreateRecoveryCopyRejectsRecoveryRequiredSourceBeforeReadOnlyOpen(t *testing.T) {
	project, err := projectdomain.NewProject("project-copy", "副本", "奇幻", "", time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	source := fakeProjectPackage("/source.zjproject")
	packages := &fakePackageManager{openPackage: source, copyPackage: fakeProjectPackage("/copies/source-recovery.zjproject")}
	factory := &fakeStoreFactory{readOnlyStore: newFakeStore(project)}
	snapshots := &fakeSnapshotArchive{recoveryRequired: true}
	service := NewService(packages, factory, license.NoopService{}, fixedClock{now: time.Unix(101, 0)}, &sequenceIDs{}, snapshots)

	_, err = service.CreateProjectRecoveryCopy(context.Background(), source.Root, "/copies")
	if !errors.Is(err, ErrProjectRecoveryRequired) {
		t.Fatalf("CreateProjectRecoveryCopy error = %v, want recovery required", err)
	}
	if factory.readOnlyCalls != 0 || packages.copyCalls != 0 {
		t.Fatalf("unstable source was opened/copied: readOnly=%d copy=%d", factory.readOnlyCalls, packages.copyCalls)
	}
}

func fakeProjectPackage(root string) ports.ProjectPackage {
	return ports.ProjectPackage{
		Root:      root,
		Database:  filepath.Join(root, "project.db"),
		Imports:   filepath.Join(root, "imports"),
		Recovery:  filepath.Join(root, "recovery"),
		Snapshots: filepath.Join(root, "snapshots"),
		Branches:  filepath.Join(root, "branches"),
		Lease:     filepath.Join(root, "project.lease"),
	}
}
