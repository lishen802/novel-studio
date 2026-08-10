package projectfs

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lishen802/novel-studio/internal/application/ports"
)

const (
	projectLeaseHelperEnv   = "NOVEL_STUDIO_PROJECT_LEASE_HELPER"
	projectLeaseRootEnv     = "NOVEL_STUDIO_PROJECT_LEASE_ROOT"
	projectLeaseMetadataEnv = "NOVEL_STUDIO_PROJECT_LEASE_METADATA"
	projectLeaseCorruptEnv  = "NOVEL_STUDIO_PROJECT_LEASE_CORRUPT"
	projectLeaseHelperReady = "project lease acquired"
)

func TestProjectLeaseRejectsAnotherProcessAndAcquiresAfterRelease(t *testing.T) {
	packageInfo := leaseTestPackage(t)
	holderMetadata := leaseTestMetadata("holder-host")
	holder := startProjectLeaseHolder(t, packageInfo.Root, holderMetadata)

	before, err := os.ReadFile(packageInfo.Lease)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := (LeaseManager{}).Acquire(packageInfo, leaseTestMetadata("contender-host"))
	if lease != nil {
		t.Fatal("contending process unexpectedly acquired a lease")
	}
	var conflict *ports.ProjectLeaseConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("Acquire error = %T %[1]v, want typed ProjectLeaseConflict", err)
	}
	if !errors.Is(err, ports.ErrProjectLeaseHeld) {
		t.Fatalf("Acquire error = %v, want ErrProjectLeaseHeld", err)
	}
	if conflict.PackagePath != packageInfo.Root || !conflict.Retryable {
		t.Fatalf("conflict = %#v", conflict)
	}
	if conflict.Holder == nil || *conflict.Holder != holderMetadata {
		t.Fatalf("holder = %#v, want %#v", conflict.Holder, holderMetadata)
	}
	after, err := os.ReadFile(packageInfo.Lease)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("a process without the OS lock rewrote lease metadata")
	}

	holder.release(t)

	lease, err = (LeaseManager{}).Acquire(packageInfo, leaseTestMetadata("next-holder"))
	if err != nil {
		t.Fatalf("Acquire after holder release: %v", err)
	}
	if err := lease.Release(); err != nil {
		t.Fatalf("Release after holder release: %v", err)
	}
}

func TestProjectLeaseConflictIgnoresMalformedHolderMetadata(t *testing.T) {
	packageInfo := leaseTestPackage(t)
	holder := startProjectLeaseHolderWithMalformedMetadata(t, packageInfo.Root, leaseTestMetadata("holder-host"))

	lease, err := (LeaseManager{}).Acquire(packageInfo, leaseTestMetadata("contender-host"))
	if lease != nil {
		t.Fatal("contending process unexpectedly acquired a lease")
	}
	var conflict *ports.ProjectLeaseConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("Acquire error = %T %[1]v, want typed ProjectLeaseConflict", err)
	}
	if conflict.Holder != nil {
		t.Fatalf("malformed holder metadata = %#v, want unknown holder", conflict.Holder)
	}

	holder.release(t)
}

func TestProjectLeaseHelperProcess(t *testing.T) {
	if os.Getenv(projectLeaseHelperEnv) != "1" {
		return
	}

	var metadata ports.ProjectLeaseMetadata
	if err := json.Unmarshal([]byte(os.Getenv(projectLeaseMetadataEnv)), &metadata); err != nil {
		t.Fatalf("decode helper metadata: %v", err)
	}
	lease, err := (LeaseManager{}).Acquire(
		ports.ProjectPackage{Root: os.Getenv(projectLeaseRootEnv)},
		metadata,
	)
	if err != nil {
		t.Fatalf("helper Acquire: %v", err)
	}
	defer func() {
		if err := lease.Release(); err != nil {
			t.Errorf("helper Release: %v", err)
		}
	}()
	if os.Getenv(projectLeaseCorruptEnv) == "1" {
		if err := os.WriteFile(filepath.Join(os.Getenv(projectLeaseRootEnv), "project.lease"), []byte("malformed metadata"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := os.Stdout.WriteString(projectLeaseHelperReady + "\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil && !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
}

type projectLeaseHolder struct {
	stdin io.WriteCloser
	cmd   *exec.Cmd
}

func startProjectLeaseHolder(t *testing.T, root string, metadata ports.ProjectLeaseMetadata) projectLeaseHolder {
	t.Helper()
	return startProjectLeaseHolderWithEnvironment(t, root, metadata, nil)
}

func startProjectLeaseHolderWithMalformedMetadata(t *testing.T, root string, metadata ports.ProjectLeaseMetadata) projectLeaseHolder {
	t.Helper()
	return startProjectLeaseHolderWithEnvironment(t, root, metadata, []string{projectLeaseCorruptEnv + "=1"})
}

func startProjectLeaseHolderWithEnvironment(t *testing.T, root string, metadata ports.ProjectLeaseMetadata, extraEnvironment []string) projectLeaseHolder {
	t.Helper()
	encodedMetadata, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestProjectLeaseHelperProcess$")
	cmd.Env = append(os.Environ(),
		projectLeaseHelperEnv+"=1",
		projectLeaseRootEnv+"="+root,
		projectLeaseMetadataEnv+"="+string(encodedMetadata),
	)
	cmd.Env = append(cmd.Env, extraEnvironment...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(stdout)
	ready, err := reader.ReadString('\n')
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("wait for holder: %v", err)
	}
	if ready != projectLeaseHelperReady+"\n" {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("holder output = %q", ready)
	}
	return projectLeaseHolder{stdin: stdin, cmd: cmd}
}

func (holder projectLeaseHolder) release(t *testing.T) {
	t.Helper()
	if _, err := io.WriteString(holder.stdin, "release\n"); err != nil {
		t.Fatal(err)
	}
	if err := holder.stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := holder.cmd.Wait(); err != nil {
		t.Fatal(err)
	}
}
