package workspace

import (
	"context"

	"github.com/lishen802/novel-studio/internal/application/agentkernel"
	creationapp "github.com/lishen802/novel-studio/internal/application/creation"
	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

// creationKernelStore 在每次内核持久化前核对捕获代次与租约。
// 短锁覆盖数据库访问，避免通过检查后 store 立即被关闭；不覆盖模型 IO。
type creationKernelStore struct {
	agentkernel.Store
	workspace  *Service
	generation uint64
}

func (store creationKernelStore) access(ctx context.Context, write bool, action func() error) error {
	store.workspace.mu.RLock()
	defer store.workspace.mu.RUnlock()
	active := store.workspace.active
	if active == nil || active.generation != store.generation {
		return creationapp.ErrStaleStore
	}
	if lease, ok := ctx.Value(creationLeaseKey{}).(*jobLease); ok && lease.IsStale() {
		return creationapp.ErrStaleStore
	}
	if write {
		// 排水中的已有租约仍允许落取消终态，只读项目不能写入。
		if active.mode != ports.ProjectAccessEditable {
			return ErrWorkspaceReadOnly
		}
		if _, ok := ctx.Value(creationLeaseKey{}).(*jobLease); !ok {
			store.workspace.leases.mu.Lock()
			draining := store.workspace.leases.draining
			store.workspace.leases.mu.Unlock()
			if draining {
				return ErrWorkspaceDraining
			}
		}
	}
	return action()
}

func (store creationKernelStore) CreateAgentRunBundle(ctx context.Context, bundle agentdomain.RunBundle) error {
	return store.access(ctx, true, func() error { return store.Store.CreateAgentRunBundle(ctx, bundle) })
}
func (store creationKernelStore) UpdateAgentRun(ctx context.Context, run agentdomain.Run, events ...agentdomain.JobEventDraft) error {
	return store.access(ctx, true, func() error { return store.Store.UpdateAgentRun(ctx, run, events...) })
}
func (store creationKernelStore) UpdateAgentRunStep(ctx context.Context, step agentdomain.RunStep, events ...agentdomain.JobEventDraft) error {
	return store.access(ctx, true, func() error { return store.Store.UpdateAgentRunStep(ctx, step, events...) })
}
func (store creationKernelStore) AppendAgentRunStep(ctx context.Context, step agentdomain.RunStep, events ...agentdomain.JobEventDraft) error {
	return store.access(ctx, true, func() error { return store.Store.AppendAgentRunStep(ctx, step, events...) })
}
func (store creationKernelStore) SaveAgentStepBundle(ctx context.Context, bundle agentdomain.StepBundle, events ...agentdomain.JobEventDraft) error {
	return store.access(ctx, true, func() error { return store.Store.SaveAgentStepBundle(ctx, bundle, events...) })
}
func (store creationKernelStore) AgentRun(ctx context.Context, id string) (run agentdomain.Run, err error) {
	err = store.access(ctx, false, func() error { run, err = store.Store.AgentRun(ctx, id); return err })
	return
}
func (store creationKernelStore) ListAgentArtifacts(ctx context.Context, id string) (artifacts []agentdomain.Artifact, err error) {
	err = store.access(ctx, false, func() error { artifacts, err = store.Store.ListAgentArtifacts(ctx, id); return err })
	return
}
