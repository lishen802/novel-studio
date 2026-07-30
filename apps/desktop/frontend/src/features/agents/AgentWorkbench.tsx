import {
  Bot,
  CheckCircle2,
  Gauge,
  KeyRound,
  Play,
  RotateCcw,
  ShieldCheck,
  StopCircle,
  Trash2,
} from "lucide-react";
import { useEffect, useState, type FormEvent } from "react";
import type {
  AgentProfileInput,
  AgentProfileView,
  AgentRole,
  AgentRunView,
  ChapterView,
  ProviderConnectionView,
  ProviderKind,
  StudioClient,
} from "../../lib/studio-client";

type AgentWorkbenchProps = {
  client: StudioClient;
  chapters: ChapterView[];
  onOpenChanges(): void;
};

const roles: Array<{ role: AgentRole; label: string; detail: string }> = [
  { role: "planner", label: "章节规划", detail: "把作者目标拆成可执行场景" },
  { role: "writer", label: "正文写作", detail: "依据上下文与规划生成完整正文" },
  {
    role: "continuity_reviewer",
    label: "连续性审阅",
    detail: "用独立模型检查事实、人物与时间线",
  },
];

export function AgentWorkbench({
  client,
  chapters,
  onOpenChanges,
}: AgentWorkbenchProps) {
  const [connections, setConnections] = useState<ProviderConnectionView[]>([]);
  const [profiles, setProfiles] = useState<AgentProfileView[]>([]);
  const [runs, setRuns] = useState<AgentRunView[]>([]);
  const [maxRunTokens, setMaxRunTokens] = useState(30000);
  const [maxContextChars, setMaxContextChars] = useState(120000);
  const [name, setName] = useState("");
  const [kind, setKind] = useState<ProviderKind>("openai");
  const [baseUrl, setBaseUrl] = useState("https://api.openai.com/v1");
  const [apiKey, setAPIKey] = useState("");
  const [chapterId, setChapterId] = useState(chapters[0]?.id ?? "");
  const [prompt, setPrompt] = useState("");
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");

  const reload = async () => {
    const [nextConnections, nextProfiles, nextSettings, nextRuns] = await Promise.all([
      client.providerConnections(),
      client.agentProfiles(),
      client.agentRuntimeSettings(),
      client.agentRuns(),
    ]);
    setConnections(nextConnections);
    setProfiles(nextProfiles);
    setMaxRunTokens(nextSettings.maxRunTokens);
    setMaxContextChars(nextSettings.maxContextChars);
    setRuns(nextRuns);
  };

  useEffect(() => {
    void reload().catch((cause: unknown) =>
      setError(cause instanceof Error ? cause.message : "加载智能体配置失败"),
    );
  }, []);

  const saveConnection = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      await client.saveProviderConnection({ name, kind, baseUrl, apiKey });
      setName("");
      setAPIKey("");
      setNotice("模型连接已安全保存");
      await reload();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "保存模型连接失败");
    } finally {
      setBusy(false);
    }
  };

  const saveProfile = async (
    role: AgentRole,
    connectionId: string,
    model: string,
  ) => {
    const defaults: Record<AgentRole, Pick<AgentProfileInput, "maxOutputTokens" | "temperature">> = {
      planner: { maxOutputTokens: 1800, temperature: 0.3 },
      writer: { maxOutputTokens: 6000, temperature: 0.75 },
      continuity_reviewer: { maxOutputTokens: 1800, temperature: 0.15 },
    };
    setBusy(true);
    setError("");
    try {
      await client.saveAgentProfile({
        role,
        connectionId,
        model,
        enabled: true,
        ...defaults[role],
      });
      setNotice("智能体角色已更新");
      await reload();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "保存智能体角色失败");
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
      const completed = await client.executeAgentRun(queued.id);
      setRuns((current) => [
        completed,
        ...current.filter((item) => item.id !== completed.id),
      ]);
      setNotice(
        completed.changeSetId
          ? "草稿已进入候选变更"
          : `任务状态：${completed.status}`,
      );
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "智能创作执行失败");
      await reload().catch(() => undefined);
    } finally {
      setBusy(false);
    }
  };

  const saveRuntimeSettings = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      const saved = await client.saveAgentRuntimeSettings({
        maxRunTokens,
        maxContextChars,
      });
      setMaxRunTokens(saved.maxRunTokens);
      setMaxContextChars(saved.maxContextChars);
      setNotice("运行预算已保存，新任务将使用此配置");
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "保存运行预算失败");
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
      const completed = await client.executeAgentRun(queued.id);
      setRuns((current) => [
        completed,
        ...current.filter((item) => item.id !== completed.id),
      ]);
      setNotice(
        completed.changeSetId
          ? "草稿已进入候选变更"
          : `任务状态：${completed.status}`,
      );
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "重试任务失败");
      await reload().catch(() => undefined);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="agent-workbench">
      <header className="workbench-heading">
        <div>
          <p className="eyebrow">MULTI-AGENT AUTHORING</p>
          <h2>模型与智能体</h2>
          <p>规划、正文和连续性审阅各自留痕，最终结果只进入候选变更。</p>
        </div>
        <Bot size={30} aria-hidden="true" />
      </header>

      {(error || notice) && (
        <div className={error ? "workspace-alert" : "agent-notice"} role={error ? "alert" : "status"}>
          {error || notice}
          {!error && notice === "草稿已进入候选变更" && (
            <button type="button" onClick={onOpenChanges}>立即审阅</button>
          )}
        </div>
      )}

      <section className="agent-grid">
        <div className="agent-panel">
          <h3><KeyRound size={18} />模型连接</h3>
          <form className="agent-form" onSubmit={saveConnection}>
            <label>连接名称<input required value={name} onChange={(event) => setName(event.target.value)} /></label>
            <label>
              协议
              <select
                value={kind}
                onChange={(event) => {
                  const next = event.target.value as ProviderKind;
                  setKind(next);
                  setBaseUrl(next === "openai" ? "https://api.openai.com/v1" : "https://api.anthropic.com");
                }}
              >
                <option value="openai">OpenAI Responses</option>
                <option value="anthropic">Anthropic Messages</option>
              </select>
            </label>
            <label>接口地址<input required value={baseUrl} onChange={(event) => setBaseUrl(event.target.value)} /></label>
            <label>API Key<input required type="password" autoComplete="off" value={apiKey} onChange={(event) => setAPIKey(event.target.value)} /></label>
            <button className="primary-action" disabled={busy}>安全保存连接</button>
          </form>
          <div className="agent-list">
            {connections.map((connection) => (
              <article key={connection.id}>
                <div><strong>{connection.name}</strong><span>{connection.kind} · {connection.hasCredential ? "凭据就绪" : "缺少凭据"}</span></div>
                <button
                  type="button"
                  className="agent-test-connection"
                  onClick={() => {
                    setNotice("正在测试模型连接…");
                    setError("");
                    void client.testProviderConnection(connection.id).then(
                      () => setNotice("模型连接测试成功"),
                      (cause: unknown) => setError(cause instanceof Error ? cause.message : "连接测试失败"),
                    );
                  }}
                >测试</button>
                <button
                  type="button"
                  aria-label={`删除连接 ${connection.name}`}
                  onClick={() => void client.deleteProviderConnection(connection.id).then(reload).catch((cause: unknown) => setError(cause instanceof Error ? cause.message : "删除失败"))}
                ><Trash2 size={15} /></button>
              </article>
            ))}
          </div>
        </div>

        <div className="agent-panel">
          <h3><ShieldCheck size={18} />角色编排</h3>
          {roles.map(({ role, label, detail }) => (
            <ProfileEditor
              key={role}
              role={role}
              label={label}
              detail={detail}
              connections={connections}
              profile={profiles.find((item) => item.role === role)}
              disabled={busy}
              onSave={saveProfile}
            />
          ))}
          <p className="agent-hint">正文写作与连续性审阅必须使用不同连接或不同模型。</p>
        </div>
      </section>

      <section className="agent-panel agent-budget-panel">
        <h3><Gauge size={18} />运行预算</h3>
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
          <button className="primary-action" disabled={busy || maxRunTokens < 1 || maxContextChars < 1}>
            保存运行预算
          </button>
        </form>
        <p className="agent-hint">预算会固化到新任务中；执行时优先保留已确认事实、目标章节大纲与场景。</p>
      </section>

      <section className="agent-panel agent-run-panel">
        <h3><Play size={18} />章节智能创作</h3>
        <div className="agent-run-form">
          <label>
            目标章节
            <select value={chapterId} onChange={(event) => setChapterId(event.target.value)}>
              {chapters.map((chapter) => <option key={chapter.id} value={chapter.id}>{chapter.position}. {chapter.title}</option>)}
            </select>
          </label>
          <label className="agent-prompt">
            创作要求
            <textarea value={prompt} onChange={(event) => setPrompt(event.target.value)} placeholder="例如：续写本章，主角发现师父隐瞒的线索，保持克制的第三人称。" />
          </label>
          <button type="button" className="primary-action" disabled={busy || !chapterId || !prompt.trim()} onClick={() => void startRun()}>
            {busy ? "智能体执行中…" : "开始智能创作"}
          </button>
        </div>
        <div className="agent-runs">
          {runs.map((run) => (
            <article key={run.id}>
              <CheckCircle2 size={18} />
              <div>
                <strong>{chapters.find((chapter) => chapter.id === run.chapterId)?.title ?? "章节任务"}</strong>
                <span>{run.status} · {run.steps.map((step) => `${step.role}:${step.status}`).join(" / ")}</span>
                <span>
                  令牌 {run.inputTokens + run.outputTokens}/{run.tokenBudget}
                  {" · "}上下文 {run.contextChars}/{run.contextCharBudget}
                  {run.contextTruncated && " · 上下文已裁剪"}
                </span>
              </div>
              <div className="agent-run-actions">
                {(run.status === "queued" || run.status === "running") && (
                  <button type="button" onClick={() => void cancelRun(run)}>
                    <StopCircle size={15} />取消任务
                  </button>
                )}
                {(["failed", "cancelled", "recoverable"] as const).includes(
                  run.status as "failed" | "cancelled" | "recoverable",
                ) && (
                  <button type="button" disabled={busy} onClick={() => void retryRun(run)}>
                    <RotateCcw size={15} />重试任务
                  </button>
                )}
                {run.changeSetId && <button type="button" onClick={onOpenChanges}>审阅变更</button>}
              </div>
            </article>
          ))}
        </div>
      </section>
    </div>
  );
}

function ProfileEditor({
  role,
  label,
  detail,
  connections,
  profile,
  disabled,
  onSave,
}: {
  role: AgentRole;
  label: string;
  detail: string;
  connections: ProviderConnectionView[];
  profile?: AgentProfileView;
  disabled: boolean;
  onSave(role: AgentRole, connectionId: string, model: string): Promise<void>;
}) {
  const [connectionId, setConnectionId] = useState(profile?.connectionId ?? connections[0]?.id ?? "");
  const [model, setModel] = useState(profile?.model ?? "");
  useEffect(() => {
    setConnectionId(profile?.connectionId ?? connections[0]?.id ?? "");
    setModel(profile?.model ?? "");
  }, [profile?.connectionId, profile?.model, connections]);
  return (
    <div className="profile-editor">
      <div><strong>{label}</strong><span>{detail}</span></div>
      <select aria-label={`${label}连接`} value={connectionId} onChange={(event) => setConnectionId(event.target.value)}>
        <option value="">选择连接</option>
        {connections.map((connection) => <option key={connection.id} value={connection.id}>{connection.name}</option>)}
      </select>
      <input aria-label={`${label}模型`} value={model} onChange={(event) => setModel(event.target.value)} placeholder="模型 ID" />
      <button type="button" disabled={disabled || !connectionId || !model.trim()} onClick={() => void onSave(role, connectionId, model.trim())}>保存</button>
    </div>
  );
}
