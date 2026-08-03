import {
  CheckCircle2,
  FileSearch2,
  FileText,
  Trash2,
  Upload,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import type {
  ChapterView,
  ImportChapterView,
  ImportJobStatus,
  ImportJobView,
  StudioClient,
} from "../../lib/studio-client";

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

function wordCount(body: string): number {
  return Array.from(body.replace(/\s/g, "")).length;
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
  const [jobs, setJobs] = useState<ImportJobView[]>([]);
  const [selectedJobId, setSelectedJobId] = useState<string | null>(null);
  const [detail, setDetail] = useState<ImportJobView | null>(null);
  const [selectedChapterIds, setSelectedChapterIds] = useState<Set<string>>(
    new Set(),
  );
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [busy, setBusy] = useState<"prepare" | "apply" | "delete" | null>(null);
  const [error, setError] = useState<string | null>(null);

  const selectDetail = (job: ImportJobView) => {
    setDetail(job);
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
        setDetail(null);
        setSelectedChapterIds(new Set());
      }
    } catch (loadError) {
      setError(errorMessage(loadError));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadJobs();
    // The client is stable for the mounted workbench.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client]);

  useEffect(() => {
    if (!selectedJobId) return;
    let current = true;
    setDetailLoading(true);
    setError(null);
    void client
      .importJob(selectedJobId)
      .then((job) => {
        if (current) selectDetail(job);
      })
      .catch((loadError: unknown) => {
        if (current) setError(errorMessage(loadError));
      })
      .finally(() => {
        if (current) setDetailLoading(false);
      });
    return () => {
      current = false;
    };
  }, [client, selectedJobId]);

  const groups = useMemo(
    () => groupChapters(detail?.chapters ?? []),
    [detail],
  );
  const selectable = detail?.status === "prepared";
  const allSelected =
    Boolean(detail?.chapters.length) &&
    detail!.chapters.every((chapter) => selectedChapterIds.has(chapter.id));

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
    setBusy("delete");
    setError(null);
    try {
      await client.deleteImportJob(detail.id);
      const remaining = jobs.filter((job) => job.id !== detail.id);
      setJobs(remaining);
      setSelectedJobId(remaining[0]?.id ?? null);
      if (remaining.length === 0) {
        setDetail(null);
        setSelectedChapterIds(new Set());
      }
    } catch (deleteError) {
      setError(errorMessage(deleteError));
    } finally {
      setBusy(null);
    }
  };

  const apply = async () => {
    if (!detail || !selectable || selectedChapterIds.size === 0) return;
    setBusy("apply");
    setError(null);
    try {
      const orderedSelection = detail.chapters
        .filter((chapter) => selectedChapterIds.has(chapter.id))
        .map((chapter) => chapter.id);
      const result = await client.applyImportJob(detail.id, orderedSelection);
      setJobs((current) =>
        current.map((job) => (job.id === detail.id ? result.job : job)),
      );
      selectDetail(result.job);
      onChaptersImported(result.chapters);
    } catch (applyError) {
      setError(errorMessage(applyError));
    } finally {
      setBusy(null);
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

  return (
    <section className="import-workbench">
      <header className="import-heading">
        <div>
          <p className="eyebrow">本地原稿重建</p>
          <h2>导入分析</h2>
          <p>先预览章节边界，再由你决定哪些正文进入当前项目。</p>
        </div>
        <button
          type="button"
          className="primary-action"
          disabled={busy !== null}
          onClick={() => void prepare()}
        >
          <Upload size={17} aria-hidden="true" />
          {busy === "prepare" ? "正在分析…" : "选择小说文件"}
        </button>
      </header>

      {error && (
        <div className="import-alert" role="alert">
          <span>{error}</span>
          <button type="button" onClick={() => setError(null)}>
            关闭
          </button>
        </div>
      )}

      {loading ? (
        <div className="import-state">
          <FileSearch2 size={28} aria-hidden="true" />
          <p>正在读取导入任务…</p>
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
                type="button"
                key={job.id}
                className={job.id === selectedJobId ? "is-active" : ""}
                onClick={() => setSelectedJobId(job.id)}
              >
                <span>{job.sourceName}</span>
                <small data-status={job.status}>{statusLabels[job.status]}</small>
              </button>
            ))}
          </aside>

          <div className="import-detail">
            {detailLoading && !detail ? (
              <div className="import-state">
                <p>正在读取章节预览…</p>
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
                        <button type="button" onClick={toggleAll}>
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
                                  disabled={!selectable}
                                  onChange={() => toggleChapter(chapter.id)}
                                />
                                <span>
                                  <strong>{chapter.title}</strong>
                                  <small>
                                    {wordCount(chapter.body)} 字 · 原文第{" "}
                                    {chapter.sourceStartLine}–
                                    {chapter.sourceEndLine} 行
                                  </small>
                                </span>
                              </label>
                              <div className="import-body-preview">
                                {chapter.body
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
                      onClick={() => void remove()}
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
                    onClick={() => void apply()}
                  >
                    {busy === "apply" ? "正在导入…" : "导入所选章节"}
                  </button>
                </footer>
              </>
            ) : null}
          </div>
        </div>
      )}
    </section>
  );
}
