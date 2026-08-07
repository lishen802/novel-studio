import {
  AlertTriangle,
  ArrowRightLeft,
  CheckCircle2,
  Loader2,
  Play,
  RotateCcw,
  Sparkles,
  X,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type {
  ChangeOperationView,
  ChangeSetView,
  EvidenceView,
  ExtractionRunInput,
  ExtractionRunView,
  ExtractionStepView,
  ExtractionTaskView,
  ImportJobView,
  ProviderConnectionView,
  StudioClient,
} from "../../lib/studio-client";
import { useJobEventReplay } from "../agents/useJobEventReplay";

type ExtractionWorkbenchProps = {
  client: StudioClient;
  onExtractionApplied(): void;
};

const taskStatusLabels: Record<ExtractionTaskView["status"], string> = {
  queued: "排队中",
  running: "运行中",
  completed: "已完成",
  failed: "已失败",
  cancelled: "已取消",
  recoverable: "待恢复",
};

const stepStatusLabels: Record<ExtractionStepView["status"], string> = {
  queued: "排队中",
  running: "运行中",
  completed: "已完成",
  failed: "已失败",
  cancelled: "已取消",
};

const kindLabels: Record<ChangeOperationView["kind"], string> = {
  replace_chapter_body: "章节正文",
  update_outline_node: "大纲节点",
  update_character: "角色档案",
  update_fact: "故事事实",
  create_outline_node: "新建大纲节点",
  create_character: "新建角色",
  create_relationship: "新建人物关系",
  create_world_rule: "新建世界规则",
  create_story_event: "新建故事事件",
  create_fact: "新建事实",
  create_foreshadowing_hook: "新建伏笔",
};

function messageOf(cause: unknown): string {
  if (cause instanceof Error && /[\u3400-\u9fff]/u.test(cause.message)) {
    return cause.message;
  }
  return "抽取操作失败，请重试。";
}

function structuredValue(raw: string): Record<string, unknown> {
  try {
    return JSON.parse(raw) as Record<string, unknown>;
  } catch {
    return {};
  }
}

function displayValue(value: unknown): string {
  if (Array.isArray(value)) {
    return value.length === 0 ? "（无）" : value.map(String).join("、");
  }
  if (value === "" || value == null) return "（空）";
  if (typeof value === "number") return String(value);
  return String(value);
}

function isTerminal(status: ExtractionTaskView["status"]): boolean {
  return (
    status === "completed" ||
    status === "failed" ||
    status === "cancelled" ||
    status === "recoverable"
  );
}

function AppliedImportJobs({
  jobs,
  selectedJobId,
  onSelect,
}: {
  jobs: ImportJobView[];
  selectedJobId: string | null;
  onSelect(id: string): void;
}) {
  if (jobs.length === 0) {
    return (
      <p className="extraction-empty-hint">
        当前没有可用的已导入稿件，请先在「导入分析」中应用一份小说。
      </p>
    );
  }
  return (
    <select
      className="extraction-source-select"
      value={selectedJobId ?? ""}
      onChange={(event) => onSelect(event.target.value)}
      aria-label="选择已导入稿件"
    >
      {jobs.map((job) => (
        <option key={job.id} value={job.id}>
          {job.sourceName}（{job.chapterCount} 章）
        </option>
      ))}
    </select>
  );
}

function ConnectionModelPicker({
  connections,
  connectionId,
  model,
  onConnectionChange,
  onModelChange,
}: {
  connections: ProviderConnectionView[];
  connectionId: string;
  model: string;
  onConnectionChange(id: string): void;
  onModelChange(model: string): void;
}) {
  return (
    <div className="extraction-model-row">
      <label>
        <span>模型连接</span>
        <select
          value={connectionId}
          onChange={(event) => onConnectionChange(event.target.value)}
          aria-label="选择模型连接"
        >
          {connections.length === 0 ? (
            <option value="">未配置连接</option>
          ) : (
            connections.map((connection) => (
              <option key={connection.id} value={connection.id}>
                {connection.name}
              </option>
            ))
          )}
        </select>
      </label>
      <label>
        <span>模型名称</span>
        <input
          type="text"
          value={model}
          placeholder="例如 gpt-4o、claude-sonnet-4-5"
          onChange={(event) => onModelChange(event.target.value)}
          aria-label="模型名称"
        />
      </label>
    </div>
  );
}

function StepList({ steps }: { steps: ExtractionStepView[] }) {
  if (steps.length === 0) {
    return (
      <p className="extraction-step-empty">该任务尚未生成章节抽取步骤。</p>
    );
  }
  const sorted = [...steps].sort((a, b) => a.position - b.position);
  return (
    <ol className="extraction-step-list" aria-label="章节抽取进度">
      {sorted.map((step) => (
        <li
          key={step.id}
          className={`extraction-step extraction-step--${step.status}`}
        >
          <div className="extraction-step__heading">
            <span className="extraction-step__position">
              {step.position.toString().padStart(2, "0")}
            </span>
            <span className="extraction-step__title">{step.chapterTitle}</span>
            <small data-status={step.status}>
              {stepStatusLabels[step.status]}
            </small>
          </div>
          {step.errorMessage && (
            <p className="extraction-step__error">
              <AlertTriangle size={14} aria-hidden="true" />
              {step.errorMessage}
            </p>
          )}
          {step.output && (
            <details className="extraction-step__output">
              <summary>查看模型输出</summary>
              <pre>{step.output}</pre>
            </details>
          )}
        </li>
      ))}
    </ol>
  );
}

function EvidenceList({ evidence }: { evidence: EvidenceView[] }) {
  if (evidence.length === 0) return null;
  return (
    <ul className="extraction-evidence-list" aria-label="原稿证据">
      {evidence.map((item, index) => (
        <li key={`${item.chapterId}-${item.startLine}-${index}`}>
          <header>
            <span>
              {item.chapterTitle}（第 {item.startLine}–{item.endLine} 行）
            </span>
          </header>
          <pre>{item.excerpt}</pre>
        </li>
      ))}
    </ul>
  );
}

function OperationCard({
  operation,
}: {
  operation: ChangeOperationView;
}) {
  const after = structuredValue(operation.afterValue);
  const entries = Object.entries(after);
  return (
    <article className="extraction-operation">
      <header>
        <span className="extraction-operation__kind">
          {kindLabels[operation.kind]}
        </span>
        {operation.reason && (
          <p className="extraction-operation__reason">{operation.reason}</p>
        )}
      </header>
      {entries.length > 0 && (
        <dl className="extraction-operation__fields">
          {entries.map(([field, value]) => (
            <div key={field}>
              <dt>{field}</dt>
              <dd>{displayValue(value)}</dd>
            </div>
          ))}
        </dl>
      )}
      {operation.evidence && operation.evidence.length > 0 && (
        <details className="extraction-operation__evidence">
          <summary>原稿证据（{operation.evidence.length} 条）</summary>
          <EvidenceList evidence={operation.evidence} />
        </details>
      )}
    </article>
  );
}

function ChangeSetPreview({
  changeSet,
  onApply,
  busy,
}: {
  changeSet: ChangeSetView;
  onApply(): void;
  busy: boolean;
}) {
  const operations = changeSet.operations;
  const pending = operations.filter((op) => op.status === "pending");
  const applied = operations.filter((op) => op.status === "applied");
  return (
    <section className="extraction-changeset" aria-label="候选变更预览">
      <header>
        <h3>{changeSet.title}</h3>
        <p>
          共 {operations.length} 项候选，已应用 {applied.length} 项，待审阅{" "}
          {pending.length} 项。
        </p>
        <button
          type="button"
          className="primary-action"
          disabled={busy || pending.length === 0}
          onClick={onApply}
        >
          <CheckCircle2 size={16} aria-hidden="true" />
          应用到设定库
        </button>
      </header>
      {operations.length === 0 ? (
        <p className="extraction-empty-hint">本次抽取未生成候选变更。</p>
      ) : (
        <div className="extraction-operation-grid">
          {operations.map((operation) => (
            <OperationCard
              key={operation.id}
              operation={operation}
            />
          ))}
        </div>
      )}
    </section>
  );
}

export function ExtractionWorkbench({
  client,
  onExtractionApplied,
}: ExtractionWorkbenchProps) {
  const [tasks, setTasks] = useState<ExtractionTaskView[]>([]);
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null);
  const [run, setRun] = useState<ExtractionRunView | null>(null);
  const [jobs, setJobs] = useState<ImportJobView[]>([]);
  const [connections, setConnections] = useState<ProviderConnectionView[]>([]);
  const [importJobId, setImportJobId] = useState<string | null>(null);
  const [connectionId, setConnectionId] = useState("");
  const [model, setModel] = useState("");
  const [instruction, setInstruction] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<
    "start" | "execute" | "cancel" | "retry" | "apply" | null
  >(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const mountedRef = useRef(true);

  const appliedJobs = useMemo(
    () => jobs.filter((job) => job.status === "applied"),
    [jobs],
  );

  const reloadAll = async () => {
    setLoading(true);
    setError(null);
    try {
      const [nextTasks, nextJobs, nextConnections] = await Promise.all([
        client.extractionTasks(),
        client.importJobs(),
        client.providerConnections(),
      ]);
      if (!mountedRef.current) return;
      setTasks(nextTasks);
      setJobs(nextJobs);
      setConnections(nextConnections);
      setSelectedTaskId((current) =>
        nextTasks.some((task) => task.id === current)
          ? current
          : (nextTasks[0]?.id ?? null),
      );
      const nextAppliedJobs = nextJobs.filter((job) => job.status === "applied");
      setImportJobId((current) =>
        nextAppliedJobs.some((job) => job.id === current)
          ? current
          : (nextAppliedJobs[0]?.id ?? null),
      );
      setConnectionId((current) =>
        current || (nextConnections[0]?.id ?? ""),
      );
    } catch (cause) {
      if (!mountedRef.current) return;
      setError(messageOf(cause));
    } finally {
      if (mountedRef.current) setLoading(false);
    }
  };

  useEffect(() => {
    mountedRef.current = true;
    void reloadAll();
    return () => {
      mountedRef.current = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client]);

  // refreshCanonicalTask pulls the authoritative ExtractionRunView for a
  // task that has just transitioned to terminal in the replay state. The
  // reducer tracks status via events; the canonical view supplies the
  // changeSet and step artifacts.
  const refreshCanonicalTask = useCallback(
    async (taskId: string) => {
      try {
        const detail = await client.extractionTask(taskId);
        if (!mountedRef.current) return;
        setTasks((current) =>
          current.map((task) =>
            task.id === detail.task.id ? detail.task : task,
          ),
        );
        setSelectedTaskId((current) => {
          if (current === taskId) setRun(detail);
          return current;
        });
        if (detail.task.status === "completed") {
          setNotice("抽取完成，可在下方审阅候选变更。");
        } else if (detail.task.status === "failed") {
          setNotice("抽取失败，可修正后重试。");
        }
      } catch {
        // Task may have been removed; ignore refresh errors.
      }
    },
    [client],
  );

  // Subscribe to job:updated via the cursor-based replay hook. The hook
  // pulls events via getJobEvents and applies the reducer; when a task
  // transitions to terminal we refresh the canonical task view to surface
  // the changeSet and step artifacts.
  const { replayStates, pullJobEvents } = useJobEventReplay({
    client,
    jobKindFilter: new Set(["extraction"]),
    onJobTerminal: (jobId) => {
      void refreshCanonicalTask(jobId);
    },
  });

  // When tasks are loaded or a new one is created, seed its replay state.
  useEffect(() => {
    for (const task of tasks) {
      if (
        task.status === "queued" ||
        task.status === "running" ||
        task.status === "recoverable"
      ) {
        void pullJobEvents(task.id, "extraction");
      }
    }
  }, [tasks, pullJobEvents]);

  // Merge replay state into the rendered task list so the UI reflects the
  // latest cursor-driven status even before the canonical view is refreshed.
  const mergedTasks = tasks.map((task) => {
    const replay = replayStates[task.id];
    if (!replay) return task;
    if (replay.cursor === 0) return task;
    return {
      ...task,
      status: replay.status as ExtractionTaskView["status"],
      inputTokens: replay.inputTokens || task.inputTokens,
      outputTokens: replay.outputTokens || task.outputTokens,
    };
  });

  const selectedTask = useMemo(
    () => mergedTasks.find((task) => task.id === selectedTaskId) ?? null,
    [mergedTasks, selectedTaskId],
  );

  useEffect(() => {
    if (!selectedTaskId) {
      setRun(null);
      return;
    }
    let cancelled = false;
    void (async () => {
      try {
        const detail = await client.extractionTask(selectedTaskId);
        if (!cancelled) setRun(detail);
      } catch (cause) {
        if (!cancelled) {
          setRun(null);
          setError(messageOf(cause));
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [client, selectedTaskId]);

  const startRun = async () => {
    if (!importJobId) {
      setError("请先选择一份已导入的稿件。");
      return;
    }
    if (!connectionId || !model.trim()) {
      setError("请配置模型连接与模型名称。");
      return;
    }
    setBusy("start");
    setError(null);
    setNotice(null);
    try {
      const input: ExtractionRunInput = {
        importJobId,
        connectionId,
        model: model.trim(),
        instruction: instruction.trim(),
      };
      const started = await client.startExtractionRun(input);
      if (!mountedRef.current) return;
      setTasks((current) => [
        started.task,
        ...current.filter((task) => task.id !== started.task.id),
      ]);
      setSelectedTaskId(started.task.id);
      setRun(started);
      setNotice("抽取任务已创建，可点击「执行抽取」开始运行。");
    } catch (cause) {
      setError(messageOf(cause));
    } finally {
      if (mountedRef.current) setBusy(null);
    }
  };

  const executeRun = async () => {
    if (!selectedTaskId) return;
    setBusy("execute");
    setError(null);
    setNotice(null);
    try {
      // ExecuteExtractionRun returns immediately with a running view. The
      // backend goroutine emits job:updated as it progresses; the
      // useJobEventReplay subscription pulls events via getJobEvents and
      // applies the reducer, then refreshCanonicalTask surfaces changeSet.
      const result = await client.executeExtractionRun(selectedTaskId);
      if (!mountedRef.current) return;
      setTasks((current) =>
        current.map((task) =>
          task.id === result.task.id ? result.task : task,
        ),
      );
      setRun(result);
      setNotice("抽取任务已启动，完成后将自动更新。");
      void pullJobEvents(result.task.id, "extraction");
    } catch (cause) {
      setError(messageOf(cause));
    } finally {
      if (mountedRef.current) setBusy(null);
    }
  };

  const cancelRun = async () => {
    if (!selectedTaskId) return;
    setBusy("cancel");
    setError(null);
    setNotice(null);
    try {
      await client.cancelExtractionRun(selectedTaskId);
      if (!mountedRef.current) return;
      setTasks((current) =>
        current.filter((task) => task.id !== selectedTaskId),
      );
      if (selectedTask?.id === selectedTaskId) {
        setSelectedTaskId(null);
        setRun(null);
      }
      setNotice("已取消抽取任务。");
    } catch (cause) {
      setError(messageOf(cause));
    } finally {
      if (mountedRef.current) setBusy(null);
    }
  };

  const retryRun = async () => {
    if (!selectedTaskId) return;
    setBusy("retry");
    setError(null);
    setNotice(null);
    try {
      const result = await client.retryExtractionRun(selectedTaskId);
      if (!mountedRef.current) return;
      setTasks((current) =>
        current.map((task) =>
          task.id === result.task.id ? result.task : task,
        ),
      );
      setRun(result);
      setNotice("已重置抽取任务，可再次点击「执行抽取」。");
    } catch (cause) {
      setError(messageOf(cause));
    } finally {
      if (mountedRef.current) setBusy(null);
    }
  };

  const applyChangeSet = async () => {
    if (!run?.changeSet) return;
    setBusy("apply");
    setError(null);
    setNotice(null);
    try {
      await client.applyChangeSet(run.changeSet.id);
      if (!mountedRef.current) return;
      setNotice("候选变更已应用到设定库。");
      onExtractionApplied();
    } catch (cause) {
      setError(messageOf(cause));
    } finally {
      if (mountedRef.current) setBusy(null);
    }
  };

  const canExecute =
    selectedTask?.status === "queued" || selectedTask?.status === "recoverable";
  const canCancel = selectedTask?.status === "running";
  const canRetry = selectedTask?.status === "failed";

  return (
    <section className="extraction-workbench">
      <header className="extraction-heading">
        <div>
          <p className="eyebrow">原稿设定抽取</p>
          <h2>设定抽取</h2>
          <p>
            按章节逐段调用模型抽取人物、关系、世界规则等设定，并跨章节合并去重，
            最终生成一份待审阅的候选变更。
          </p>
        </div>
        <button
          type="button"
          className="secondary-action"
          disabled={busy !== null || loading}
          onClick={() => void reloadAll()}
        >
          <ArrowRightLeft size={16} aria-hidden="true" />
          刷新
        </button>
      </header>

      {error && (
        <div className="extraction-alert" role="alert">
          <AlertTriangle size={16} aria-hidden="true" />
          <span>{error}</span>
          <button type="button" onClick={() => setError(null)}>
            关闭
          </button>
        </div>
      )}

      {notice && (
        <div className="extraction-notice" role="status" aria-live="polite">
          <CheckCircle2 size={16} aria-hidden="true" />
          <span>{notice}</span>
          <button type="button" onClick={() => setNotice(null)}>
            关闭
          </button>
        </div>
      )}

      <section className="extraction-new-run">
        <h3>启动新抽取</h3>
        <AppliedImportJobs
          jobs={appliedJobs}
          selectedJobId={importJobId}
          onSelect={setImportJobId}
        />
        <ConnectionModelPicker
          connections={connections}
          connectionId={connectionId}
          model={model}
          onConnectionChange={setConnectionId}
          onModelChange={setModel}
        />
        <label className="extraction-instruction">
          <span>抽取说明（可选）</span>
          <textarea
            value={instruction}
            placeholder="补充风格、术语、关注重点等指引…"
            onChange={(event) => setInstruction(event.target.value)}
            aria-label="抽取说明"
          />
        </label>
        <button
          type="button"
          className="primary-action"
          disabled={busy !== null || loading || appliedJobs.length === 0}
          onClick={() => void startRun()}
        >
          {busy === "start" ? (
            <Loader2 size={16} aria-hidden="true" className="spin" />
          ) : (
            <Sparkles size={16} aria-hidden="true" />
          )}
          {busy === "start" ? "创建中…" : "创建抽取任务"}
        </button>
      </section>

      {loading ? (
        <div className="extraction-state" role="status" aria-live="polite">
          <Loader2 size={28} aria-hidden="true" className="spin" />
          <p>正在读取抽取任务…</p>
        </div>
      ) : mergedTasks.length === 0 ? (
        <div className="extraction-state">
          <Sparkles size={28} aria-hidden="true" />
          <h3>暂无抽取任务</h3>
          <p>选择一份已导入稿件并创建抽取任务，模型会逐章抽取设定候选。</p>
        </div>
      ) : (
        <div className="extraction-layout">
          <aside className="extraction-task-list" aria-label="抽取任务">
            {mergedTasks.map((task) => (
              <button
                type="button"
                key={task.id}
                className={task.id === selectedTaskId ? "is-active" : ""}
                aria-current={task.id === selectedTaskId ? "true" : undefined}
                disabled={busy !== null}
                onClick={() => setSelectedTaskId(task.id)}
              >
                <span>
                  {jobs.find((job) => job.id === task.importJobId)?.sourceName ??
                    task.importJobId}
                </span>
                <small data-status={task.status}>
                  {taskStatusLabels[task.status]}
                </small>
              </button>
            ))}
          </aside>

          <div className="extraction-detail">
            {!selectedTask || !run ? (
              <div className="extraction-state">
                <p>选择一个抽取任务查看详情。</p>
              </div>
            ) : (
              <>
                <section className="extraction-summary">
                  <div className="extraction-summary__heading">
                    <p className="eyebrow">
                      {jobs.find((job) => job.id === selectedTask.importJobId)
                        ?.sourceName ?? "未知稿件"}
                    </p>
                    <h3>抽取任务 {selectedTask.id.slice(0, 8)}</h3>
                    <span
                      className={`extraction-status extraction-status--${selectedTask.status}`}
                    >
                      {taskStatusLabels[selectedTask.status]}
                    </span>
                  </div>
                  <dl className="extraction-metadata">
                    <div>
                      <dt>模型</dt>
                      <dd>{selectedTask.model || "（未设置）"}</dd>
                    </div>
                    <div>
                      <dt>步骤</dt>
                      <dd>
                        {selectedTask.steps.length === 0
                          ? "—"
                          : `${selectedTask.steps.filter((s) => s.status === "completed").length} / ${selectedTask.steps.length}`}
                      </dd>
                    </div>
                    <div>
                      <dt>Token 用量</dt>
                      <dd>
                        入 {selectedTask.inputTokens} / 出{" "}
                        {selectedTask.outputTokens}
                      </dd>
                    </div>
                  </dl>
                  {selectedTask.errorMessage && (
                    <p className="extraction-task-error">
                      <AlertTriangle size={14} aria-hidden="true" />
                      {selectedTask.errorMessage}
                    </p>
                  )}
                  <div className="extraction-actions">
                    {canExecute && (
                      <button
                        type="button"
                        className="primary-action"
                        disabled={busy !== null}
                        onClick={() => void executeRun()}
                      >
                        {busy === "execute" ? (
                          <Loader2
                            size={16}
                            aria-hidden="true"
                            className="spin"
                          />
                        ) : (
                          <Play size={16} aria-hidden="true" />
                        )}
                        {busy === "execute" ? "执行中…" : "执行抽取"}
                      </button>
                    )}
                    {canRetry && (
                      <button
                        type="button"
                        className="secondary-action"
                        disabled={busy !== null}
                        onClick={() => void retryRun()}
                      >
                        <RotateCcw size={16} aria-hidden="true" />
                        重置重试
                      </button>
                    )}
                    {canCancel && (
                      <button
                        type="button"
                        className="secondary-action"
                        disabled={busy !== null}
                        onClick={() => void cancelRun()}
                      >
                        <X size={16} aria-hidden="true" />
                        取消任务
                      </button>
                    )}
                  </div>
                </section>

                <section className="extraction-steps">
                  <h4>章节抽取进度</h4>
                  <StepList steps={selectedTask.steps} />
                </section>

                {run.changeSet && isTerminal(selectedTask.status) && (
                  <ChangeSetPreview
                    changeSet={run.changeSet}
                    busy={busy !== null}
                    onApply={() => void applyChangeSet()}
                  />
                )}
              </>
            )}
          </div>
        </div>
      )}
    </section>
  );
}
