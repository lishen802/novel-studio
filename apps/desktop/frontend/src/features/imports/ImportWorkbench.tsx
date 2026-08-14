import {
  CheckCircle2,
  FileSearch2,
  FileText,
  Sparkles,
  Trash2,
  Upload,
} from "lucide-react";
import {
  type KeyboardEvent,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import type {
  ChapterView,
  ImportChapterView,
  ImportJobStatus,
  ImportJobView,
  StudioClient,
} from "../../lib/studio-client";
import { ExtractionWorkbench } from "../extraction/ExtractionWorkbench";

type ImportWorkbenchProps = {
  client: StudioClient;
  onChaptersImported(chapters: ChapterView[]): void;
};

const statusLabels: Record<ImportJobStatus, string> = {
  prepared: "待导入",
  applied: "已导入",
  failed: "分析失败",
};

function errorMessage(error: unknown): string {
  if (
    error instanceof Error &&
    /[\u3400-\u9fff]/u.test(error.message) &&
    !/[/\\]|select|insert|update|delete\s+from/i.test(error.message)
  ) {
    return error.message;
  }
  return "导入操作失败，请重试。";
}

function failureMessage(job: ImportJobView): string {
  switch (job.errorCode) {
    case "IMPORT_PARSE_EMPTY":
      return "没有识别到可导入章节。";
    case "IMPORT_PARSE_LINE_TOO_LONG":
      return "小说中有一行过长，暂时无法安全分析。";
    case "IMPORT_PARSE_FAILED":
      return "小说章节分析失败，请检查文件格式。";
    default:
      return job.errorMessage
        ? errorMessage(new Error(job.errorMessage))
        : "小说分析失败，请重新选择文件。";
  }
}

function formatName(format: ImportJobView["sourceFormat"]): string {
  return format === "markdown" ? "Markdown" : "TXT";
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const kib = bytes / 1024;
  if (kib < 1024) {
    return `${Number.isInteger(kib) ? kib : kib.toFixed(1)} KiB`;
  }
  const mib = kib / 1024;
  return `${Number.isInteger(mib) ? mib : mib.toFixed(1)} MiB`;
}

function groupChapters(chapters: ImportChapterView[]) {
  const groups = new Map<string, ImportChapterView[]>();
  for (const chapter of chapters) {
    const volume = chapter.volumeTitle.trim() || "未分卷";
    groups.set(volume, [...(groups.get(volume) ?? []), chapter]);
  }
  return [...groups.entries()];
}

export function ImportWorkbench({
  client,
  onChaptersImported,
}: ImportWorkbenchProps) {
  const [mode, setMode] = useState<"imports" | "extraction">("imports");
  const [jobs, setJobs] = useState<ImportJobView[]>([]);
  const [selectedJobId, setSelectedJobId] = useState<string | null>(null);
  const [loadedDetail, setLoadedDetail] = useState<ImportJobView | null>(null);
  const [selectedChapterIds, setSelectedChapterIds] = useState<Set<string>>(
    new Set(),
  );
  const [loading, setLoading] = useState(true);
  const [jobsLoadFailed, setJobsLoadFailed] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailLoadFailed, setDetailLoadFailed] = useState(false);
  const [detailLoadNonce, setDetailLoadNonce] = useState(0);
  const [busy, setBusy] = useState<"prepare" | "apply" | "delete" | null>(null);
  const [confirmation, setConfirmation] = useState<
    "apply" | "delete" | null
  >(null);
  const [error, setError] = useState<string | null>(null);
  const selectedJobIDRef = useRef(selectedJobId);
  const mountedRef = useRef(true);
  const dialogRef = useRef<HTMLElement | null>(null);
  const dialogTriggerRef = useRef<HTMLButtonElement | null>(null);
  const restoreDialogFocusRef = useRef(false);
  const prepareButtonRef = useRef<HTMLButtonElement | null>(null);
  const taskButtonRefs = useRef(new Map<string, HTMLButtonElement>());
  const mutationFocusRef = useRef<
    { kind: "task"; id: string } | { kind: "prepare" } | null
  >(null);
  selectedJobIDRef.current = selectedJobId;
  const detail = loadedDetail?.id === selectedJobId ? loadedDetail : null;

  const selectDetail = (job: ImportJobView) => {
    setLoadedDetail(job);
    setSelectedChapterIds(
      new Set(
        job.chapters
          .filter((chapter) => chapter.selected)
          .map((chapter) => chapter.id),
      ),
    );
  };

  const loadJobs = async () => {
    setLoading(true);
    setJobsLoadFailed(false);
    setError(null);
    try {
      const loaded = await client.importJobs();
      setJobs(loaded);
      setSelectedJobId((current) =>
        loaded.some((job) => job.id === current)
          ? current
          : (loaded[0]?.id ?? null),
      );
      if (loaded.length === 0) {
        setLoadedDetail(null);
        setSelectedChapterIds(new Set());
      }
      return true;
    } catch (loadError) {
      setJobsLoadFailed(true);
      setError(errorMessage(loadError));
      return false;
    } finally {
      setLoading(false);
    }
  };

  const refreshImports = async () => {
    if (await loadJobs()) {
      setDetailLoadNonce((current) => current + 1);
    }
  };

  useEffect(() => {
    void loadJobs();
    // The client is stable for the mounted workbench.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client]);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      selectedJobIDRef.current = null;
    };
  }, []);

  useEffect(() => {
    if (!confirmation) return;
    const dialog = dialogRef.current;
    const firstButton = dialog?.querySelector<HTMLButtonElement>(
      "button:not([disabled])",
    );
    (firstButton ?? dialog)?.focus();
  }, [busy, confirmation]);

  useEffect(() => {
    if (confirmation !== null || !restoreDialogFocusRef.current) return;
    restoreDialogFocusRef.current = false;
    const trigger = dialogTriggerRef.current;
    if (trigger?.isConnected && !trigger.disabled) trigger.focus();
  }, [confirmation]);

  useEffect(() => {
    if (confirmation !== null || busy !== null) return;
    const target = mutationFocusRef.current;
    if (!target) return;
    const element =
      target.kind === "task"
        ? taskButtonRefs.current.get(target.id)
        : prepareButtonRef.current;
    if (!element?.isConnected || element.disabled) return;
    mutationFocusRef.current = null;
    element.focus();
  }, [busy, confirmation, jobs, selectedJobId]);

  useEffect(() => {
    if (!selectedJobId) return;
    let current = true;
    setDetailLoading(true);
    setDetailLoadFailed(false);
    setError(null);
    void client
      .importJob(selectedJobId)
      .then((job) => {
        if (current) selectDetail(job);
      })
      .catch((loadError: unknown) => {
        if (current) {
          setDetailLoadFailed(true);
          setError(errorMessage(loadError));
        }
      })
      .finally(() => {
        if (current) setDetailLoading(false);
      });
    return () => {
      current = false;
    };
  }, [client, detailLoadNonce, selectedJobId]);

  const groups = useMemo(
    () => groupChapters(detail?.chapters ?? []),
    [detail],
  );
  const selectable = detail?.status === "prepared";
  const allSelected =
    Boolean(detail?.chapters.length) &&
    detail!.chapters.every((chapter) => selectedChapterIds.has(chapter.id));

  const openConfirmation = (
    value: "apply" | "delete",
    trigger: HTMLButtonElement,
  ) => {
    dialogTriggerRef.current = trigger;
    setConfirmation(value);
  };

  const closeConfirmation = () => {
    restoreDialogFocusRef.current = true;
    setConfirmation(null);
  };

  const prepare = async () => {
    setError(null);
    try {
      const sourcePath = await client.chooseManuscriptToImport();
      if (!sourcePath) return;
      setBusy("prepare");
      const result = await client.prepareManuscriptImport(sourcePath);
      setJobs((current) => [
        result.job,
        ...current.filter((job) => job.id !== result.job.id),
      ]);
      setSelectedJobId(result.job.id);
      selectDetail(result.job);
    } catch (prepareError) {
      setError(errorMessage(prepareError));
    } finally {
      setBusy(null);
    }
  };

  const remove = async () => {
    if (!detail || detail.status === "applied") return;
    const targetID = detail.id;
    setBusy("delete");
    setError(null);
    try {
      await client.deleteImportJob(targetID);
      const remaining = jobs.filter((job) => job.id !== targetID);
      mutationFocusRef.current = remaining[0]
        ? { kind: "task", id: remaining[0].id }
        : { kind: "prepare" };
      setJobs(remaining);
      setSelectedJobId(remaining[0]?.id ?? null);
      if (remaining.length === 0) {
        setLoadedDetail(null);
        setSelectedChapterIds(new Set());
      }
    } catch (deleteError) {
      setError(errorMessage(deleteError));
    } finally {
      closeConfirmation();
      setBusy(null);
    }
  };

  const apply = async () => {
    if (!detail || !selectable || selectedChapterIds.size === 0) return;
    const targetID = detail.id;
    setBusy("apply");
    setError(null);
    try {
      const orderedSelection = detail.chapters
        .filter((chapter) => selectedChapterIds.has(chapter.id))
        .map((chapter) => chapter.id);
      const result = await client.applyImportJob(targetID, orderedSelection);
      onChaptersImported(result.chapters);
      if (selectedJobIDRef.current !== targetID) return;
      mutationFocusRef.current = { kind: "task", id: targetID };
      setJobs((current) =>
        current.map((job) => (job.id === targetID ? result.job : job)),
      );
      selectDetail(result.job);
    } catch (applyError) {
      if (mountedRef.current) setError(errorMessage(applyError));
    } finally {
      if (mountedRef.current) {
        closeConfirmation();
        setBusy(null);
      }
    }
  };

  const handleDialogKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key === "Escape") {
      event.preventDefault();
      event.stopPropagation();
      if (busy === null) closeConfirmation();
      return;
    }
    if (event.key !== "Tab") return;
    const dialog = dialogRef.current;
    if (!dialog) return;
    const focusable = [
      ...dialog.querySelectorAll<HTMLElement>(
        "button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex='-1'])",
      ),
    ];
    if (focusable.length === 0) {
      event.preventDefault();
      dialog.focus();
      return;
    }
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (
      (event.shiftKey && document.activeElement === first) ||
      (!event.shiftKey && document.activeElement === last) ||
      !dialog.contains(document.activeElement)
    ) {
      event.preventDefault();
      (event.shiftKey ? last : first).focus();
    }
  };

  const toggleChapter = (id: string) => {
    if (!selectable) return;
    setSelectedChapterIds((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const toggleAll = () => {
    if (!detail || !selectable) return;
    setSelectedChapterIds(
      allSelected
        ? new Set()
        : new Set(detail.chapters.map((chapter) => chapter.id)),
    );
  };

  if (mode === "extraction") {
    return (
      <section className="import-workbench">
        <header className="import-heading">
          <div>
            <p className="eyebrow">原稿设定抽取</p>
            <h2>设定抽取</h2>
            <p>从已导入稿件中逐章抽取设定候选，合并后由你审阅应用。</p>
          </div>
          <button
            type="button"
            className="secondary-action"
            onClick={() => setMode("imports")}
          >
            返回导入任务
          </button>
        </header>
        <ExtractionWorkbench
          client={client}
          onExtractionApplied={() => void refreshImports()}
        />
      </section>
    );
  }

  return (
    <section className="import-workbench">
      <header className="import-heading">
        <div>
          <p className="eyebrow">本地原稿重建</p>
          <h2>导入分析</h2>
          <p>先预览章节边界，再由你决定哪些正文进入当前项目。</p>
        </div>
        <div className="import-heading__actions">
          <button
            type="button"
            className="secondary-action"
            onClick={() => setMode("extraction")}
          >
            <Sparkles size={16} aria-hidden="true" />
            设定抽取
          </button>
          <button
            ref={prepareButtonRef}
            type="button"
            className="primary-action"
            disabled={busy !== null || confirmation !== null}
            onClick={() => void prepare()}
          >
            <Upload size={17} aria-hidden="true" />
            {busy === "prepare" ? "正在分析…" : "选择小说文件"}
          </button>
        </div>
      </header>

      {error && (
        <div className="import-alert notice notice--danger" role="alert">
          <span>{error}</span>
          <button
            type="button"
            disabled={busy !== null || loading}
            onClick={() => void refreshImports()}
          >
            刷新任务与详情
          </button>
          <button type="button" onClick={() => setError(null)}>
            关闭
          </button>
        </div>
      )}

      {loading ? (
        <div className="import-state" role="status" aria-live="polite">
          <FileSearch2 size={28} aria-hidden="true" />
          <p>正在读取导入任务…</p>
        </div>
      ) : jobsLoadFailed ? (
        <div className="import-state" role="status" aria-live="polite">
          <FileSearch2 size={28} aria-hidden="true" />
          <p>导入任务暂时无法读取。</p>
          <button
            type="button"
            className="secondary-action"
            disabled={busy !== null || loading}
            onClick={() => void refreshImports()}
          >
            刷新任务与详情
          </button>
        </div>
      ) : jobs.length === 0 ? (
        <div className="import-state">
          <FileText size={28} aria-hidden="true" />
          <h3>暂无导入任务</h3>
          <p>选择 TXT 或 Markdown 小说文件，应用不会修改你的原文件。</p>
        </div>
      ) : (
        <div className="import-layout">
          <aside className="import-task-list" aria-label="导入任务">
            {jobs.map((job) => (
              <button
                ref={(element) => {
                  if (element) taskButtonRefs.current.set(job.id, element);
                  else taskButtonRefs.current.delete(job.id);
                }}
                type="button"
                key={job.id}
                className={job.id === selectedJobId ? "is-active" : ""}
                aria-current={
                  job.id === selectedJobId ? "true" : undefined
                }
                disabled={busy !== null || confirmation !== null}
                onClick={() => setSelectedJobId(job.id)}
              >
                <span>{job.sourceName}</span>
                <small data-status={job.status}>{statusLabels[job.status]}</small>
              </button>
            ))}
          </aside>

          <div className="import-detail" aria-busy={detailLoading}>
            {detailLoading && !detail ? (
              <div
                className="import-state"
                role="status"
                aria-live="polite"
              >
                <p>正在读取章节预览…</p>
              </div>
            ) : detailLoadFailed && !detail ? (
              <div className="import-state" role="status" aria-live="polite">
                <FileSearch2 size={28} aria-hidden="true" />
                <p>任务详情暂时无法读取。</p>
                <button
                  type="button"
                  className="secondary-action"
                  disabled={busy !== null || loading}
                  onClick={() => void refreshImports()}
                >
                  刷新任务与详情
                </button>
              </div>
            ) : detail ? (
              <>
                <section className="import-source-card">
                  <div className="import-source-title">
                    <div>
                      <p className="eyebrow">来源文件</p>
                      <h3>{detail.sourceName}</h3>
                    </div>
                    <span
                      className={`import-status import-status--${detail.status}`}
                    >
                      {detail.status === "applied" && (
                        <CheckCircle2 size={15} aria-hidden="true" />
                      )}
                      {statusLabels[detail.status]}
                    </span>
                  </div>
                  <dl className="import-metadata">
                    <div>
                      <dt>格式</dt>
                      <dd>{formatName(detail.sourceFormat)}</dd>
                    </div>
                    <div>
                      <dt>编码</dt>
                      <dd>{detail.sourceEncoding}</dd>
                    </div>
                    <div>
                      <dt>大小</dt>
                      <dd>{formatBytes(detail.sourceBytes)}</dd>
                    </div>
                    <div>
                      <dt>摘要</dt>
                      <dd>
                        <code>{detail.sourceHashDigest}</code>
                      </dd>
                    </div>
                    <div>
                      <dt>章节</dt>
                      <dd>{detail.chapterCount}</dd>
                    </div>
                  </dl>
                  {detail.status === "failed" && (
                    <p className="import-failure" role="status">
                      {failureMessage(detail)}
                    </p>
                  )}
                  {detail.status === "applied" && (
                    <p className="import-success">
                      已创建 {detail.selectedChapterCount} 个章节，任务已锁定为只读。
                    </p>
                  )}
                </section>

                {detail.status !== "failed" && detail.chapters.length > 0 && (
                  <>
                    <div className="import-selection-bar">
                      <p>
                        已选择 {selectedChapterIds.size} /{" "}
                        {detail.chapters.length} 章
                      </p>
                      {selectable && (
                        <button
                          type="button"
                          disabled={confirmation !== null}
                          onClick={toggleAll}
                        >
                          {allSelected ? "取消全选" : "全选章节"}
                        </button>
                      )}
                    </div>
                    <div className="import-volumes">
                      {groups.map(([volume, chapters]) => (
                        <section className="import-volume" key={volume}>
                          <h3>{volume}</h3>
                          {chapters.map((chapter) => (
                            <section
                              className="import-chapter-card"
                              role="group"
                              aria-label={chapter.title}
                              key={chapter.id}
                            >
                              <label className="import-chapter-title">
                                <input
                                  type="checkbox"
                                  aria-label={`选择 ${chapter.title}`}
                                  checked={selectedChapterIds.has(chapter.id)}
                                  disabled={
                                    !selectable || confirmation !== null
                                  }
                                  onChange={() => toggleChapter(chapter.id)}
                                />
                                <span>
                                  <strong>{chapter.title}</strong>
                                  <small>
                                    {chapter.wordCount} 字 · 原文第{" "}
                                    {chapter.sourceStartLine}–
                                    {chapter.sourceEndLine} 行
                                  </small>
                                </span>
                              </label>
                              <div className="import-body-preview">
                                {chapter.bodyPreview
                                  .split(/\n+/)
                                  .filter(Boolean)
                                  .slice(0, 3)
                                  .map((paragraph, index) => (
                                    <p key={`${chapter.id}-${index}`}>
                                      {paragraph}
                                    </p>
                                  ))}
                              </div>
                            </section>
                          ))}
                        </section>
                      ))}
                    </div>
                  </>
                )}

                <footer className="import-actions">
                  {detail.status !== "applied" && (
                    <button
                      type="button"
                      className="secondary-action import-delete"
                      disabled={busy !== null}
                      onClick={(event) =>
                        openConfirmation("delete", event.currentTarget)
                      }
                    >
                      <Trash2 size={16} aria-hidden="true" />
                      {busy === "delete" ? "正在删除…" : "删除任务"}
                    </button>
                  )}
                  <button
                    type="button"
                    className="primary-action"
                    disabled={
                      !selectable ||
                      selectedChapterIds.size === 0 ||
                      busy !== null
                    }
                    onClick={(event) =>
                      openConfirmation("apply", event.currentTarget)
                    }
                  >
                    {busy === "apply" ? "正在导入…" : "导入所选章节"}
                  </button>
                </footer>
              </>
            ) : null}
          </div>
        </div>
      )}

      {confirmation === "apply" && detail && (
        <div className="dialog-backdrop">
          <section
            ref={dialogRef}
            className="confirm-dialog"
            role="dialog"
            aria-modal="true"
            aria-label="确认导入章节"
            tabIndex={-1}
            onKeyDown={handleDialogKeyDown}
          >
            <p className="eyebrow">正式写入确认</p>
            <h2>导入“{detail.sourceName}”？</h2>
            <p>
              已选择 {selectedChapterIds.size} 个章节；确认后将创建正式章节，
              导入任务将锁定为只读，不能再次应用。
            </p>
            <div className="dialog-actions">
              <button
                type="button"
                className="secondary-action"
                disabled={busy !== null}
                onClick={closeConfirmation}
              >
                取消
              </button>
              <button
                type="button"
                className="primary-action"
                disabled={busy !== null}
                onClick={() => void apply()}
              >
                {busy === "apply" ? "正在导入…" : "确认导入"}
              </button>
            </div>
          </section>
        </div>
      )}

      {confirmation === "delete" && detail && detail.status !== "applied" && (
        <div className="dialog-backdrop">
          <section
            ref={dialogRef}
            className="confirm-dialog"
            role="dialog"
            aria-modal="true"
            aria-label="确认删除导入任务"
            tabIndex={-1}
            onKeyDown={handleDialogKeyDown}
          >
            <p className="eyebrow">删除任务确认</p>
            <h2>删除“{detail.sourceName}”的导入任务？</h2>
            <p>任务记录和章节预览会被删除，此操作无法撤销。</p>
            <div className="dialog-actions">
              <button
                type="button"
                className="secondary-action"
                disabled={busy !== null}
                onClick={closeConfirmation}
              >
                取消
              </button>
              <button
                type="button"
                className="danger-action"
                disabled={busy !== null}
                onClick={() => void remove()}
              >
                {busy === "delete" ? "正在删除…" : "确认删除"}
              </button>
            </div>
          </section>
        </div>
      )}
    </section>
  );
}
