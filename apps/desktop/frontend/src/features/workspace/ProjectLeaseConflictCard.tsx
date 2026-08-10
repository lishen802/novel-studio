import type { ProjectLeaseConflictView } from "../../lib/studio-client";

type ProjectLeaseConflictCardProps = {
  conflict: ProjectLeaseConflictView;
  busy?: boolean;
  onRetry(): void;
  onOpenReadOnly(): void;
  onCreateRecoveryCopy(): void;
  onCancel(): void;
};

export function ProjectLeaseConflictCard({
  conflict,
  busy = false,
  onRetry,
  onOpenReadOnly,
  onCreateRecoveryCopy,
  onCancel,
}: ProjectLeaseConflictCardProps) {
  const holder = conflict.holder;

  return (
    <section
      className="project-lease-conflict"
      role="dialog"
      aria-label="项目正在编辑"
    >
      <p className="eyebrow">本地项目保护</p>
      <h2>项目正在由另一个实例编辑</h2>
      <p>
        为避免两个编辑器同时写入同一个项目，本次没有取得编辑权限。已定稿内容不会被覆盖。
      </p>
      <p className="dialog-path">{conflict.path}</p>
      {holder && (
        <dl className="project-lease-conflict__holder">
          <div>
            <dt>设备</dt>
            <dd>{holder.hostname}</dd>
          </div>
          <div>
            <dt>进程</dt>
            <dd>{holder.pid}</dd>
          </div>
          <div>
            <dt>启动时间</dt>
            <dd>{holder.startedAt}</dd>
          </div>
        </dl>
      )}
      {!holder && (
        <p className="project-lease-conflict__unknown-holder">
          占用实例的详细信息不可用；仍可选择只读打开或创建恢复副本。
        </p>
      )}
      <div className="dialog-actions project-lease-conflict__actions">
        <button type="button" className="secondary-action" disabled={busy} onClick={onCancel}>
          取消
        </button>
        {conflict.retryable && (
          <button type="button" className="secondary-action" disabled={busy} onClick={onRetry}>
            重试
          </button>
        )}
        <button type="button" className="secondary-action" disabled={busy} onClick={onOpenReadOnly}>
          只读打开
        </button>
        <button type="button" className="primary-action" disabled={busy} onClick={onCreateRecoveryCopy}>
          创建恢复副本
        </button>
      </div>
    </section>
  );
}
