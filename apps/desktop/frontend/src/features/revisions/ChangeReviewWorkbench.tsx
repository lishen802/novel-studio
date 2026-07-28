import { Check, GitCompareArrows, ShieldAlert, X } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  RevisionConflictError,
  type ChangeSetStatus,
  type ChangeSetView,
  type ChapterView,
  type StudioClient,
} from "../../lib/studio-client";

type Props = {
  client: StudioClient;
  chapters: ChapterView[];
  onChapterUpdated(chapter: ChapterView): void;
};

const statusNames: Record<ChangeSetStatus, string> = {
  pending: "待审阅",
  applied: "已应用",
  rejected: "已拒绝",
  conflicted: "存在冲突",
};

function messageOf(cause: unknown) {
  if (cause instanceof RevisionConflictError) {
    return "正文已在候选生成后发生变化。候选已保留并标记为冲突，请拒绝后重新生成。";
  }
  return cause instanceof Error ? cause.message : "无法处理候选变更";
}

export function ChangeReviewWorkbench({
  client,
  chapters,
  onChapterUpdated,
}: Props) {
  const [changeSets, setChangeSets] = useState<ChangeSetView[]>([]);
  const [selectedID, setSelectedID] = useState("");
  const [loading, setLoading] = useState(true);
  const [working, setWorking] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const next = await client.changeSets();
      setChangeSets(next);
      setSelectedID((current) =>
        next.some((item) => item.id === current)
          ? current
          : (next.find((item) => item.status === "pending")?.id ??
            next[0]?.id ??
            ""),
      );
    } catch (cause) {
      setError(messageOf(cause));
    } finally {
      setLoading(false);
    }
  }, [client]);

  useEffect(() => {
    void load();
  }, [load]);

  const chapterByID = useMemo(
    () => new Map(chapters.map((chapter) => [chapter.id, chapter])),
    [chapters],
  );
  const selected = changeSets.find((item) => item.id === selectedID);
  const operation = selected?.operations[0];
  const targetChapter = operation
    ? chapterByID.get(operation.targetId)
    : undefined;

  const apply = async () => {
    if (!selected) return;
    setWorking(true);
    try {
      const chapter = await client.applyChangeSet(selected.id);
      onChapterUpdated(chapter);
      setError(null);
    } catch (cause) {
      setError(messageOf(cause));
    } finally {
      await load();
      setWorking(false);
    }
  };

  const reject = async () => {
    if (!selected) return;
    setWorking(true);
    try {
      await client.rejectChangeSet(selected.id);
      setError(null);
    } catch (cause) {
      setError(messageOf(cause));
    } finally {
      await load();
      setWorking(false);
    }
  };

  return (
    <section className="change-review-workbench">
      <header className="change-review-header">
        <div>
          <p className="eyebrow">Reviewable Changes</p>
          <h2>候选变更</h2>
          <p>所有智能体修改先在这里审阅，接受前不会改动正式作品。</p>
        </div>
        <span className="change-review-count">
          {changeSets.filter((item) => item.status === "pending").length} 项待审阅
        </span>
      </header>

      {error && (
        <div className="story-error" role="alert">
          <ShieldAlert size={17} />
          {error}
        </div>
      )}

      {loading ? (
        <p className="story-loading">正在读取候选变更…</p>
      ) : changeSets.length === 0 ? (
        <div className="change-review-empty">
          <GitCompareArrows size={32} aria-hidden="true" />
          <h3>暂无候选变更</h3>
          <p>智能体生成的正文或故事设定修改会先进入此处。</p>
        </div>
      ) : (
        <div className="change-review-layout">
          <nav className="change-set-list" aria-label="候选变更列表">
            {changeSets.map((item) => (
              <button
                type="button"
                key={item.id}
                className={item.id === selectedID ? "is-active" : ""}
                onClick={() => setSelectedID(item.id)}
              >
                <span>
                  <strong>{item.title}</strong>
                  <small>{item.summary || "未填写变更摘要"}</small>
                </span>
                <em className={`change-status change-status--${item.status}`}>
                  {statusNames[item.status]}
                </em>
              </button>
            ))}
          </nav>

          {selected && operation && (
            <article className="change-review-detail">
              <header>
                <div>
                  <span className={`change-status change-status--${selected.status}`}>
                    {statusNames[selected.status]}
                  </span>
                  <h3>{selected.title}</h3>
                  <p>{selected.summary || "未填写变更摘要"}</p>
                </div>
                <dl>
                  <div>
                    <dt>目标章节</dt>
                    <dd>{targetChapter?.title ?? "未知章节"}</dd>
                  </div>
                  <div>
                    <dt>基础版本</dt>
                    <dd>Revision {operation.baseRevision}</dd>
                  </div>
                  <div>
                    <dt>来源</dt>
                    <dd>{selected.createdBy}</dd>
                  </div>
                </dl>
              </header>

              {operation.reason && (
                <section className="change-reason">
                  <h4>修改理由</h4>
                  <p>{operation.reason}</p>
                </section>
              )}

              <section className="change-body-compare" aria-label="正文变更对比">
                <div>
                  <h4>修改前</h4>
                  <pre>{operation.beforeValue || "（空正文）"}</pre>
                </div>
                <div>
                  <h4>修改后</h4>
                  <pre>{operation.afterValue || "（空正文）"}</pre>
                </div>
              </section>

              {(selected.status === "pending" ||
                selected.status === "conflicted") && (
                <footer>
                  <button
                    type="button"
                    className="secondary-action"
                    disabled={working}
                    onClick={() => void reject()}
                  >
                    <X size={16} />
                    拒绝此变更
                  </button>
                  {selected.status === "pending" && (
                    <button
                      type="button"
                      className="primary-action"
                      disabled={working}
                      onClick={() => void apply()}
                    >
                      <Check size={16} />
                      {working ? "正在应用…" : "接受并应用"}
                    </button>
                  )}
                </footer>
              )}
            </article>
          )}
        </div>
      )}
    </section>
  );
}
