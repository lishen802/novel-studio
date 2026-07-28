import {
  GitBranch,
  GitCompareArrows,
  GitMerge,
  Plus,
  RefreshCw,
  Trash2,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import type {
  BranchComparisonView,
  BranchMergePreviewView,
  BranchSwitchView,
  BranchView,
  StudioClient,
} from "../../lib/studio-client";

type Props = {
  client: StudioClient;
  onBeforeVersionOperation(): Promise<void>;
  onWorkspaceChanged(result: BranchSwitchView): void;
};

function messageOf(cause: unknown): string {
  return cause instanceof Error ? cause.message : "分支操作失败，请重试。";
}

function displayTime(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.valueOf())
    ? value
    : new Intl.DateTimeFormat("zh-CN", {
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      }).format(date);
}

export function BranchWorkbench({
  client,
  onBeforeVersionOperation,
  onWorkspaceChanged,
}: Props) {
  const [branches, setBranches] = useState<BranchView[]>([]);
  const [loading, setLoading] = useState(true);
  const [working, setWorking] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [switchTarget, setSwitchTarget] = useState<BranchView | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<BranchView | null>(null);
  const [comparison, setComparison] = useState<BranchComparisonView | null>(
    null,
  );
  const [mergePreview, setMergePreview] =
    useState<BranchMergePreviewView | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setBranches(await client.branches());
      setError(null);
    } catch (cause) {
      setError(messageOf(cause));
    } finally {
      setLoading(false);
    }
  }, [client]);

  useEffect(() => {
    void load();
  }, [load]);

  const active = useMemo(
    () => branches.find((branch) => branch.active),
    [branches],
  );
  const names = useMemo(
    () => new Map(branches.map((branch) => [branch.id, branch.name])),
    [branches],
  );

  const createBranch = async (event: FormEvent) => {
    event.preventDefault();
    const trimmedName = name.trim();
    if (!trimmedName) return;
    setWorking(true);
    try {
      await onBeforeVersionOperation();
      await client.createBranch({
        name: trimmedName,
        description: description.trim(),
      });
      setName("");
      setDescription("");
      setShowCreate(false);
      await load();
    } catch (cause) {
      setError(messageOf(cause));
    } finally {
      setWorking(false);
    }
  };

  const switchBranch = async () => {
    if (!switchTarget) return;
    setWorking(true);
    try {
      await onBeforeVersionOperation();
      const result = await client.switchBranch(switchTarget.id);
      onWorkspaceChanged(result);
      setSwitchTarget(null);
      setComparison(null);
      await load();
    } catch (cause) {
      setError(messageOf(cause));
    } finally {
      setWorking(false);
    }
  };

  const compareBranch = async (branch: BranchView) => {
    if (!active || branch.id === active.id) return;
    setWorking(true);
    try {
      await onBeforeVersionOperation();
      setComparison(await client.compareBranches(branch.id, active.id));
      setError(null);
    } catch (cause) {
      setError(messageOf(cause));
    } finally {
      setWorking(false);
    }
  };

  const previewMerge = async (branch: BranchView) => {
    setWorking(true);
    try {
      await onBeforeVersionOperation();
      setMergePreview(await client.previewBranchMerge(branch.id));
      setError(null);
    } catch (cause) {
      setError(messageOf(cause));
    } finally {
      setWorking(false);
    }
  };

  const confirmMerge = async () => {
    if (!mergePreview) return;
    setWorking(true);
    try {
      const result = await client.confirmBranchMerge(mergePreview.token);
      onWorkspaceChanged(result);
      setMergePreview(null);
      setComparison(null);
      await load();
    } catch (cause) {
      setError(messageOf(cause));
    } finally {
      setWorking(false);
    }
  };

  const deleteBranch = async () => {
    if (!deleteTarget) return;
    setWorking(true);
    try {
      await client.deleteBranch(deleteTarget.id);
      setDeleteTarget(null);
      setComparison(null);
      await load();
    } catch (cause) {
      setError(messageOf(cause));
    } finally {
      setWorking(false);
    }
  };

  return (
    <section className="branch-panel" aria-labelledby="branch-panel-title">
      <header className="branch-panel__header">
        <div>
          <GitBranch size={20} />
          <div>
            <h3 id="branch-panel-title">剧情分支</h3>
            <p>每个分支保存整部作品，可安全试写互斥剧情方案。</p>
          </div>
        </div>
        <div className="branch-panel__actions">
          <button
            type="button"
            className="secondary-action icon-action"
            aria-label="刷新剧情分支"
            disabled={working || loading}
            onClick={() => void load()}
          >
            <RefreshCw size={15} />
          </button>
          <button
            type="button"
            className="primary-action"
            aria-label="新建剧情分支"
            disabled={working || !active}
            onClick={() => setShowCreate(true)}
          >
            <Plus size={15} />
            新建分支
          </button>
        </div>
      </header>

      {error && (
        <div className="story-error" role="alert">
          {error}
        </div>
      )}

      {loading ? (
        <p className="version-empty">正在读取剧情分支…</p>
      ) : (
        <div className="branch-list">
          {branches.map((branch) => {
            const canMerge =
              !branch.active && branch.parentId === active?.id;
            return (
              <article
                className={`branch-card ${branch.active ? "is-active" : ""}`}
                key={branch.id}
              >
                <div className="branch-card__identity">
                  <span className="branch-card__icon">
                    <GitBranch size={17} />
                  </span>
                  <div>
                    <div className="branch-card__title">
                      <strong>{branch.name}</strong>
                      {branch.active && <em>当前</em>}
                      {branch.main && <em className="is-neutral">主分支</em>}
                    </div>
                    <p>{branch.description || "未填写分支说明"}</p>
                    <small>
                      {branch.parentId
                        ? `来源：${names.get(branch.parentId) ?? "未知分支"}`
                        : "作品基线"}
                      {" · "}
                      第 {branch.generation} 代 · {displayTime(branch.updatedAt)}
                    </small>
                  </div>
                </div>
                <div className="branch-card__actions">
                  {!branch.active && (
                    <>
                      <button
                        type="button"
                        className="secondary-action"
                        aria-label={`比较${branch.name}`}
                        disabled={working}
                        onClick={() => void compareBranch(branch)}
                      >
                        <GitCompareArrows size={14} />
                        比较
                      </button>
                      <button
                        type="button"
                        className="secondary-action"
                        aria-label={`切换到${branch.name}`}
                        disabled={working}
                        onClick={() => setSwitchTarget(branch)}
                      >
                        切换
                      </button>
                    </>
                  )}
                  {canMerge && (
                    <button
                      type="button"
                      className="primary-action"
                      aria-label={`预览合并${branch.name}`}
                      disabled={working}
                      onClick={() => void previewMerge(branch)}
                    >
                      <GitMerge size={14} />
                      合并
                    </button>
                  )}
                  {!branch.main && !branch.active && (
                    <button
                      type="button"
                      className="danger-icon-action"
                      aria-label={`删除${branch.name}`}
                      disabled={working}
                      onClick={() => setDeleteTarget(branch)}
                    >
                      <Trash2 size={15} />
                    </button>
                  )}
                </div>
              </article>
            );
          })}
        </div>
      )}

      {comparison && (
        <BranchComparison
          comparison={comparison}
          title={`${comparison.source.name} → ${comparison.target.name}`}
          onClose={() => setComparison(null)}
        />
      )}

      {showCreate && (
        <div className="dialog-backdrop">
          <form
            className="project-dialog branch-dialog"
            role="dialog"
            aria-label="新建剧情分支"
            onSubmit={createBranch}
          >
            <p className="eyebrow">Story Branch</p>
            <h2>从“{active?.name}”创建分支</h2>
            <p>当前正文和全部故事设定会成为新分支的独立起点。</p>
            <label>
              分支名称
              <input
                autoFocus
                required
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder="例如：主角黑化线"
              />
            </label>
            <label>
              分支说明
              <textarea
                value={description}
                onChange={(event) => setDescription(event.target.value)}
                placeholder="记录这个方案要验证的核心变化"
              />
            </label>
            <div className="dialog-actions">
              <button
                type="button"
                className="secondary-action"
                disabled={working}
                onClick={() => setShowCreate(false)}
              >
                取消
              </button>
              <button
                type="submit"
                className="primary-action"
                disabled={working || !name.trim()}
              >
                创建分支
              </button>
            </div>
          </form>
        </div>
      )}

      {switchTarget && (
        <div className="dialog-backdrop">
          <section
            className="confirm-dialog"
            role="dialog"
            aria-label="确认切换剧情分支"
          >
            <p className="eyebrow">Switch Branch</p>
            <h2>切换到“{switchTarget.name}”？</h2>
            <p>系统会先保存当前分支，再载入目标分支的完整作品状态。</p>
            <div className="dialog-actions">
              <button
                type="button"
                className="secondary-action"
                disabled={working}
                onClick={() => setSwitchTarget(null)}
              >
                取消
              </button>
              <button
                type="button"
                className="primary-action"
                disabled={working}
                onClick={() => void switchBranch()}
              >
                确认切换
              </button>
            </div>
          </section>
        </div>
      )}

      {mergePreview && (
        <div className="dialog-backdrop">
          <section
            className="confirm-dialog branch-merge-dialog"
            role="dialog"
            aria-label="确认合并剧情分支"
          >
            <p className="eyebrow">Fast-forward Merge</p>
            <h2>
              合并“{mergePreview.comparison.source.name}”到“
              {mergePreview.comparison.target.name}”？
            </h2>
            <p>
              <strong>完整项目内容</strong>
              会替换目标分支当前状态，系统会先建立恢复点。来源分支不会被删除。
            </p>
            <BranchComparison
              comparison={mergePreview.comparison}
              compact
            />
            <div className="dialog-actions">
              <button
                type="button"
                className="secondary-action"
                disabled={working}
                onClick={() => setMergePreview(null)}
              >
                取消
              </button>
              <button
                type="button"
                className="primary-action"
                disabled={working}
                onClick={() => void confirmMerge()}
              >
                确认合并
              </button>
            </div>
          </section>
        </div>
      )}

      {deleteTarget && (
        <div className="dialog-backdrop">
          <section
            className="confirm-dialog"
            role="dialog"
            aria-label="确认删除剧情分支"
          >
            <p className="eyebrow">Delete Branch</p>
            <h2>删除“{deleteTarget.name}”？</h2>
            <p>该分支的独立作品状态会被删除。已有项目快照不会受影响。</p>
            <div className="dialog-actions">
              <button
                type="button"
                className="secondary-action"
                disabled={working}
                onClick={() => setDeleteTarget(null)}
              >
                取消
              </button>
              <button
                type="button"
                className="danger-action"
                disabled={working}
                onClick={() => void deleteBranch()}
              >
                确认删除
              </button>
            </div>
          </section>
        </div>
      )}
    </section>
  );
}

function BranchComparison({
  comparison,
  title,
  compact = false,
  onClose,
}: {
  comparison: BranchComparisonView;
  title?: string;
  compact?: boolean;
  onClose?(): void;
}) {
  const content = (
    <>
      <div className="branch-compare__stats">
        <span>
          <strong>{comparison.addedChapters}</strong> 新增章节
        </span>
        <span>
          <strong>{comparison.modifiedChapters}</strong> 修改章节
        </span>
        <span>
          <strong>{comparison.removedChapters}</strong> 删除章节
        </span>
      </div>
      <div className="branch-domain-list">
        {comparison.domains.map((domain) => (
          <div className={domain.changed ? "is-changed" : ""} key={domain.key}>
            <span>{domain.label}</span>
            <small>
              {domain.targetCount} → {domain.sourceCount}
            </small>
            <em>{domain.changed ? "有变化" : "无变化"}</em>
          </div>
        ))}
      </div>
      {comparison.chapterChanges.length > 0 && (
        <ul className="branch-chapter-changes">
          {comparison.chapterChanges.map((change) => (
            <li key={`${change.chapterId}-${change.kind}`}>
              <span>{change.title}</span>
              <em>
                {change.kind === "added"
                  ? "新增"
                  : change.kind === "removed"
                    ? "删除"
                    : "修改"}
              </em>
            </li>
          ))}
        </ul>
      )}
    </>
  );
  if (compact) {
    return <div className="branch-compare branch-compare--compact">{content}</div>;
  }
  return (
    <section className="branch-compare">
      <header>
        <div>
          <GitCompareArrows size={18} />
          <h4>{title ?? "分支比较"}</h4>
        </div>
        {onClose && (
          <button type="button" className="secondary-action" onClick={onClose}>
            关闭比较
          </button>
        )}
      </header>
      {content}
    </section>
  );
}
