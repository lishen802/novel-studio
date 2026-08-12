import {
  CheckCircle2,
  Gauge,
  Play,
  RotateCcw,
  StopCircle,
} from "lucide-react";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import type {
  AgentRunView,
  AgentRuntimeSettingsInput,
  AgentRuntimeSettingsView,
  ChapterView,
  StudioClient,
} from "../../lib/studio-client";
import { useJobEventReplay } from "./useJobEventReplay";

type AgentRunPanelProps = {
  client: StudioClient;
  chapters: ChapterView[];
  onOpenChanges(): void;
};

function agentRunErrorText(run: AgentRunView): string {
  switch (run.errorCode) {
    case "token_budget_exceeded":
      return "任务已达到令牌预算，请提高预算或降低角色输出上限。";
    case "context_budget_exceeded":
      return "必要的故事上下文超过预算，请提高上下文字符预算。";
    case "cancelled":
      return "任务已由作者取消。";
    case "provider_error":
      return "模型调用失败，请检查连接配置后重试。";
    default:
      return run.errorMessage;
  }
}

// AgentRunPanel 只承载项目任务运行：预算、任务创建、运行进度与恢复。
// 模型连接和 API Key 不在此处；请在 ApplicationSettings 中管理。
export function AgentRunPanel({
  client,
  chapters,
  onOpenChanges,
}: AgentRunPanelProps) {
  const [runs, setRuns] = useState<AgentRunView[]>([]);
  const [maxRunTokens, setMaxRunTokens] = useState(30000);
  const [maxContextChars, setMaxContextChars] = useState(120000);
  const [chapterId, setChapterId] = useState(chapters[0]?.id ?? "");
  const [prompt, setPrompt] = useState("");
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");

  const reload = async () => {
    const [nextSettings, nextRuns] = await Promise.all([
      client.agentRuntimeSettings(),
      client.agentRuns(),
    ]);
    setMaxRunTokens(nextSettings.maxRunTokens);
    setMaxContextChars(nextSettings.maxContextChars);
    setRuns(nextRuns);
  };

  useEffect(() => {
    void reload().catch((cause: unknown) =>
      setError(cause instanceof Error ? cause.message : "加载运行配置失败"),
    );
  }, []);

  // 拉取权威 AgentRunView 以获取 changeSetId（事件流只承载状态/步骤/令牌）
  const refreshCanonicalRun = useCallback(
    async (runId: string) => {
      try {
        const updated = await client.agentRun(runId);
        setRuns((current) =>
          current.map((item) => (item.id === runId ? updated : item)),
        );
        if (updated.status === "completed" && updated.changeSetId) {
          setNotice("草稿已进入候选变更");
        } else if (
          ["failed", "cancelled", "completed"].includes(updated.status)
        ) {
          setNotice(`任务状态：${updated.status}`);
        }
      } catch {
        // 任务可能已被删除；忽略刷新错误。
      }
    },
    [client],
  );

  const { replayStates, pullJobEvents } = useJobEventReplay({
    client,
    jobKindFilter: new Set(["agent"]),
    onJobTerminal: (jobId) => {
      void refreshCanonicalRun(jobId);
    },
  });

  // 新任务出现时为其播种重放状态，使界面在权威刷新前就能反映最新进度
  useEffect(() => {
    for (const run of runs) {
      if (
        run.status === "queued" ||
        run.status === "running" ||
        run.status === "recoverable"
      ) {
        void pullJobEvents(run.id, "agent");
      }
    }
  }, [runs, pullJobEvents]);

  // 把重放状态合并到渲染视图，使界面在权威刷新前就能反映最新进度
  const mergedRuns = runs.map((run) => {
    const replay = replayStates[run.id];
    if (!replay) return run;
    if (replay.cursor === 0) return run;
    return {
      ...run,
      status: replay.status as AgentRunView["status"],
      inputTokens: replay.inputTokens || run.inputTokens,
      outputTokens: replay.outputTokens || run.outputTokens,
    };
  });

  const saveRuntimeSettings = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      const input: AgentRuntimeSettingsInput = {
        maxRunTokens,
        maxContextChars,
      };
      const saved: AgentRuntimeSettingsView = await client.saveAgentRuntimeSettings(input);
      setMaxRunTokens(saved.maxRunTokens);
      setMaxContextChars(saved.maxContextChars);
      setNotice("运行预算已保存，新任务将使用此配置");
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "保存运行预算失败");
    } finally {
      setBusy(false);
    }
  };

  const startRun = async () => {
    if (!chapterId || !prompt.trim()) return;
    setBusy(true);
    setError("");
    setNotice("正在执行规划、写作与连续性审阅…");
    try {
      const queued = await client.createChapterAgentRun(chapterId, prompt.trim());
      setRuns((current) => [
        queued,
        ...current.filter((item) => item.id !== queued.id),
      ]);
      const running = await client.executeAgentRun(queued.id);
      setRuns((current) => [
        running,
        ...current.filter((item) => item.id !== running.id),
      ]);
      void pullJobEvents(running.id, "agent");
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "智能创作执行失败");
      await reload().catch(() => undefined);
    } finally {
      setBusy(false);
    }
  };

  const cancelRun = async (run: AgentRunView) => {
    setError("");
    try {
      await client.cancelAgentRun(run.id);
      setNotice("已请求取消智能创作任务");
      await reload();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "取消任务失败");
    }
  };

  const retryRun = async (run: AgentRunView) => {
    setBusy(true);
    setError("");
    setNotice("正在重新执行智能创作任务…");
    try {
      const queued = await client.retryAgentRun(run.id);
      setRuns((current) => [
        queued,
        ...current.filter((item) => item.id !== queued.id),
      ]);
      const running = await client.executeAgentRun(queued.id);
      setRuns((current) => [
        running,
        ...current.filter((item) => item.id !== running.id),
      ]);
      void pullJobEvents(running.id, "agent");
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "重试任务失败");
      await reload().catch(() => undefined);
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="agent-run-panel" aria-label="章节智能创作">
      <header className="workbench-heading">
        <div>
          <p className="eyebrow">MULTI-AGENT AUTHORING</p>
          <h2>章节智能创作</h2>
          <p>规划、正文和连续性审阅各自留痕，最终结果只进入候选变更。</p>
        </div>
        <Play size={26} aria-hidden="true" />
      </header>

      {(error || notice) && (
        <div
          className={error ? "workspace-alert" : "agent-notice"}
          role={error ? "alert" : "status"}
        >
          {error || notice}
          {!error && notice === "草稿已进入候选变更" && (
            <button type="button" onClick={onOpenChanges}>
              立即审阅
            </button>
          )}
        </div>
      )}

      <section className="agent-panel agent-budget-panel">
        <h3>
          <Gauge size={18} />
          运行预算
        </h3>
        <form className="agent-budget-form" onSubmit={saveRuntimeSettings}>
          <label>
            单次任务令牌预算
            <input
              type="number"
              min={1}
              value={maxRunTokens}
              onChange={(event) => setMaxRunTokens(Number(event.target.value))}
            />
          </label>
          <label>
            上下文字符预算
            <input
              type="number"
              min={1}
              value={maxContextChars}
              onChange={(event) => setMaxContextChars(Number(event.target.value))}
            />
          </label>
          <button
            className="primary-action"
            disabled={busy || maxRunTokens < 1 || maxContextChars < 1}
          >
            保存运行预算
          </button>
        </form>
        <p className="agent-hint">
          预算会固化到新任务中；执行时优先保留已确认事实、目标章节大纲与场景。
        </p>
      </section>

      <section className="agent-panel">
        <h3>
          <Play size={18} />
          新建章节智能创作
        </h3>
        <div className="agent-run-form">
          <label>
            目标章节
            <select
              value={chapterId}
              onChange={(event) => setChapterId(event.target.value)}
            >
              {chapters.map((chapter) => (
                <option key={chapter.id} value={chapter.id}>
                  {chapter.position}. {chapter.title}
                </option>
              ))}
            </select>
          </label>
          <label className="agent-prompt">
            创作要求
            <textarea
              value={prompt}
              onChange={(event) => setPrompt(event.target.value)}
              placeholder="例如：续写本章，主角发现师父隐瞒的线索，保持克制的第三人称。"
            />
          </label>
          <button
            type="button"
            className="primary-action"
            disabled={busy || !chapterId || !prompt.trim()}
            onClick={() => void startRun()}
          >
            {busy ? "智能体执行中…" : "开始智能创作"}
          </button>
        </div>
      </section>

      <section className="agent-panel">
        <h3>运行进度与恢复</h3>
        <div className="agent-runs">
          {mergedRuns.length === 0 && (
            <p className="agent-hint">尚无任务。选择章节并填写创作要求后开始。</p>
          )}
          {mergedRuns.map((run) => (
            <article key={run.id}>
              <CheckCircle2 size={18} />
              <div>
                <strong>
                  {chapters.find((chapter) => chapter.id === run.chapterId)
                    ?.title ?? "章节任务"}
                </strong>
                <span>
                  {run.status} ·{" "}
                  {run.steps
                    .map((step) => `${step.role}:${step.status}`)
                    .join(" / ")}
                </span>
                <span>
                  令牌 {run.inputTokens + run.outputTokens}/{run.tokenBudget}
                  {" · "}上下文 {run.contextChars}/{run.contextCharBudget}
                  {run.contextTruncated && " · 上下文已裁剪"}
                </span>
                {agentRunErrorText(run) && (
                  <span className="agent-run-error">
                    {agentRunErrorText(run)}
                  </span>
                )}
              </div>
              <div className="agent-run-actions">
                {(run.status === "queued" || run.status === "running") && (
                  <button type="button" onClick={() => void cancelRun(run)}>
                    <StopCircle size={15} />
                    取消任务
                  </button>
                )}
                {(
                  ["failed", "cancelled", "recoverable"] as const
                ).includes(run.status as "failed" | "cancelled" | "recoverable") && (
                  <button
                    type="button"
                    disabled={busy}
                    onClick={() => void retryRun(run)}
                  >
                    <RotateCcw size={15} />
                    重试任务
                  </button>
                )}
                {run.changeSetId && (
                  <button type="button" onClick={onOpenChanges}>
                    审阅变更
                  </button>
                )}
              </div>
            </article>
          ))}
        </div>
      </section>
    </section>
  );
}
