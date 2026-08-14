import {
  Camera,
  History,
  RotateCcw,
  Save,
  ShieldCheck,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import type {
  ChapterRevisionView,
  ChapterView,
  ProjectView,
  SnapshotRestoreView,
  SnapshotView,
  StudioClient,
} from "../../lib/studio-client";

type Props = {
  client: StudioClient;
  project: ProjectView;
  chapters: ChapterView[];
  onChapterUpdated(chapter: ChapterView): void;
  onWorkspaceRestored(
    result: Pick<SnapshotRestoreView, "project" | "chapters">,
  ): void;
  onBeforeVersionOperation(): Promise<void>;
};

const sourceNames: Record<ChapterRevisionView["source"], string> = {
  initial: "初始版本",
  author: "作者保存",
  agent: "智能体变更",
  restore: "历史恢复",
};

function messageOf(cause: unknown) {
  return cause instanceof Error ? cause.message : "版本操作失败，请重试";
}

function displayTime(value: string) {
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value));
}

export function VersionHistoryWorkbench({
  client,
  project,
  chapters,
  onChapterUpdated,
  onWorkspaceRestored,
  onBeforeVersionOperation,
}: Props) {
  const [chapterID, setChapterID] = useState(chapters[0]?.id ?? "");
  const [history, setHistory] = useState<ChapterRevisionView[]>([]);
  const [snapshots, setSnapshots] = useState<SnapshotView[]>([]);
  const [selectedRevision, setSelectedRevision] = useState<number | null>(null);
  const [snapshotName, setSnapshotName] = useState("");
  const [chapterRestoreTarget, setChapterRestoreTarget] =
    useState<ChapterRevisionView | null>(null);
  const [snapshotRestoreTarget, setSnapshotRestoreTarget] =
    useState<SnapshotView | null>(null);
  const [loading, setLoading] = useState(true);
  const [working, setWorking] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const chapterRevision =
    chapters.find((chapter) => chapter.id === chapterID)?.revision ?? 0;

  const loadSnapshots = useCallback(async () => {
    const values = await client.snapshots();
    setSnapshots(values);
  }, [client]);

  const loadHistory = useCallback(
    async (targetChapterID: string) => {
      if (!targetChapterID) {
        setHistory([]);
        setSelectedRevision(null);
        return;
      }
      const values = await client.chapterHistory(targetChapterID);
      setHistory(values);
      setSelectedRevision((current) =>
        values.some((item) => item.revision === current)
          ? current
          : (values[0]?.revision ?? null),
      );
    },
    [client],
  );

  useEffect(() => {
    if (
      chapterID &&
      !chapters.some((chapter) => chapter.id === chapterID)
    ) {
      setChapterID(chapters[0]?.id ?? "");
    }
  }, [chapterID, chapters]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    Promise.all([client.snapshots(), chapterID ? client.chapterHistory(chapterID) : []])
      .then(([nextSnapshots, nextHistory]) => {
        if (cancelled) return;
        setSnapshots(nextSnapshots);
        setHistory(nextHistory);
        setSelectedRevision(nextHistory[0]?.revision ?? null);
        setError(null);
      })
      .catch((cause) => {
        if (!cancelled) setError(messageOf(cause));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [chapterID, chapterRevision, client]);

  const chapterByID = useMemo(
    () => new Map(chapters.map((chapter) => [chapter.id, chapter])),
    [chapters],
  );
  const currentChapter = chapterByID.get(chapterID);
  const selected = history.find(
    (item) => item.revision === selectedRevision,
  );

  const createSnapshot = async () => {
    const name = snapshotName.trim();
    if (!name) return;
    setWorking(true);
    try {
      await onBeforeVersionOperation();
      const created = await client.createSnapshot(name);
      setSnapshots((current) => [created, ...current]);
      setSnapshotName("");
      setError(null);
    } catch (cause) {
      setError(messageOf(cause));
    } finally {
      setWorking(false);
    }
  };

  const restoreChapter = async () => {
    if (!chapterRestoreTarget || !currentChapter) return;
    setWorking(true);
    try {
      await onBeforeVersionOperation();
      const latestChapter = await client.openChapter(currentChapter.id);
      if (latestChapter.revision !== currentChapter.revision) {
        onChapterUpdated(latestChapter);
      }
      const restored = await client.restoreChapterRevision(
        latestChapter.id,
        chapterRestoreTarget.revision,
        latestChapter.revision,
      );
      onChapterUpdated(restored);
      await loadHistory(latestChapter.id);
      setChapterRestoreTarget(null);
      setError(null);
    } catch (cause) {
      setError(messageOf(cause));
    } finally {
      setWorking(false);
    }
  };

  const restoreSnapshot = async () => {
    if (!snapshotRestoreTarget) return;
    setWorking(true);
    try {
      await onBeforeVersionOperation();
      const restored = await client.restoreSnapshot(snapshotRestoreTarget.id);
      onWorkspaceRestored(restored);
      await loadSnapshots();
      setSnapshotRestoreTarget(null);
      setError(null);
    } catch (cause) {
      setError(messageOf(cause));
    } finally {
      setWorking(false);
    }
  };

  return (
    <section className="version-workbench">
      <header className="version-header">
        <div>
          <p className="eyebrow">Version &amp; Recovery</p>
          <h2>版本历史</h2>
          <p>查看章节修订、建立整部作品恢复点，并安全回到任意已保存状态。</p>
        </div>
        <span className="version-project">
          <ShieldCheck size={16} />
          {project.title}
        </span>
      </header>

      {error && (
        <div className="story-error" role="alert">
          {error}
        </div>
      )}

      <section className="snapshot-panel">
        <header>
          <div>
            <Camera size={19} />
            <div>
              <h3>项目快照</h3>
              <p>快照同时保存正文、人物、世界、时间线和事实。</p>
            </div>
          </div>
          <div className="snapshot-create">
            <label>
              <span>快照名称</span>
              <input
                aria-label="快照名称"
                value={snapshotName}
                onChange={(event) => setSnapshotName(event.target.value)}
                placeholder="例如：第一幕定稿"
              />
            </label>
            <button
              type="button"
              className="primary-action"
              disabled={working || !snapshotName.trim()}
              onClick={() => void createSnapshot()}
            >
              <Save size={15} />
              创建命名快照
            </button>
          </div>
        </header>

        <div className="snapshot-list">
          {snapshots.length === 0 ? (
            <p className="version-empty">还没有项目快照。</p>
          ) : (
            snapshots.map((snapshot) => (
              <article
                key={snapshot.id}
                aria-label={`快照 ${snapshot.name}`}
                className="snapshot-card"
              >
                <div>
                  <span className={`snapshot-kind snapshot-kind--${snapshot.kind}`}>
                    {snapshot.kind === "recovery" ? "自动恢复点" : "命名快照"}
                  </span>
                  <strong>{snapshot.name}</strong>
                  <small>{displayTime(snapshot.createdAt)}</small>
                </div>
                <button
                  type="button"
                  className="secondary-action"
                  disabled={working}
                  onClick={() => setSnapshotRestoreTarget(snapshot)}
                >
                  <RotateCcw size={14} />
                  恢复整个项目
                </button>
              </article>
            ))
          )}
        </div>
      </section>

      <div className="version-history-layout">
        <aside className="revision-list-panel">
          <header>
            <History size={18} />
            <h3>章节修订</h3>
          </header>
          <label className="version-chapter-select">
            <span>选择章节</span>
            <select
              value={chapterID}
              onChange={(event) => setChapterID(event.target.value)}
            >
              {chapters.map((chapter) => (
                <option key={chapter.id} value={chapter.id}>
                  {chapter.title}
                </option>
              ))}
            </select>
          </label>
          {loading ? (
            <p className="version-empty">正在读取版本历史…</p>
          ) : history.length === 0 ? (
            <p className="version-empty">这个章节还没有可用版本。</p>
          ) : (
            <nav aria-label="章节修订列表" className="revision-list">
              {history.map((item) => (
                <button
                  type="button"
                  key={item.revision}
                  className={
                    item.revision === selectedRevision ? "is-active" : ""
                  }
                  onClick={() => setSelectedRevision(item.revision)}
                >
                  <span>
                    <strong>Revision {item.revision}</strong>
                    <small>{sourceNames[item.source]}</small>
                  </span>
                  <em>{item.isCurrent ? "当前" : displayTime(item.createdAt)}</em>
                </button>
              ))}
            </nav>
          )}
        </aside>

        <article className="revision-compare-panel">
          {selected && currentChapter ? (
            <>
              <header>
                <div>
                  <span className="snapshot-kind">
                    {sourceNames[selected.source]}
                  </span>
                  <h3>Revision {selected.revision}</h3>
                  <p>{selected.summary || "未填写版本说明"}</p>
                </div>
                {!selected.isCurrent && (
                  <button
                    type="button"
                    className="secondary-action"
                    disabled={working}
                    onClick={() => setChapterRestoreTarget(selected)}
                  >
                    <RotateCcw size={15} />
                    恢复此版本
                  </button>
                )}
              </header>
              <section
                className="revision-body-compare"
                aria-label="章节版本对比"
              >
                <div>
                  <h4>所选 Revision {selected.revision}</h4>
                  <pre>{selected.body || "（空正文）"}</pre>
                </div>
                <div>
                  <h4>当前 Revision {currentChapter.revision}</h4>
                  <pre>{currentChapter.body || "（空正文）"}</pre>
                </div>
              </section>
            </>
          ) : (
            <p className="version-empty">选择一个章节版本查看内容。</p>
          )}
        </article>
      </div>

      {chapterRestoreTarget && currentChapter && (
        <div className="dialog-backdrop">
          <section
            className="confirm-dialog"
            role="dialog"
            aria-label="确认恢复章节版本"
          >
            <p className="eyebrow">Chapter Recovery</p>
            <h2>恢复到 Revision {chapterRestoreTarget.revision}？</h2>
            <p>
              当前正文会先进入历史，再以新的 Revision{" "}
              {currentChapter.revision + 1} 保存所选内容，不会删除后续版本。
            </p>
            <div className="dialog-actions">
              <button
                type="button"
                className="secondary-action"
                disabled={working}
                onClick={() => setChapterRestoreTarget(null)}
              >
                取消
              </button>
              <button
                type="button"
                className="primary-action"
                disabled={working}
                onClick={() => void restoreChapter()}
              >
                确认恢复
              </button>
            </div>
          </section>
        </div>
      )}

      {snapshotRestoreTarget && (
        <div className="dialog-backdrop">
          <section
            className="confirm-dialog"
            role="dialog"
            aria-label="确认恢复项目快照"
          >
            <p className="eyebrow">Project Recovery</p>
            <h2>恢复“{snapshotRestoreTarget.name}”？</h2>
            <p>
              这会恢复整部作品的正文和全部故事设定。系统会先自动创建恢复前快照，
              之后仍可回到当前状态。
            </p>
            <div className="dialog-actions">
              <button
                type="button"
                className="secondary-action"
                disabled={working}
                onClick={() => setSnapshotRestoreTarget(null)}
              >
                取消
              </button>
              <button
                type="button"
                className="primary-action"
                disabled={working}
                onClick={() => void restoreSnapshot()}
              >
                确认恢复整个项目
              </button>
            </div>
          </section>
        </div>
      )}
    </section>
  );
}
