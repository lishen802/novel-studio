import {
  Check,
  GitCompareArrows,
  ListChecks,
  Pencil,
  ShieldAlert,
  X,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  RevisionConflictError,
  type ChangeOperationEditInput,
  type ChangeOperationKind,
  type ChangeOperationView,
  type ChangeSetStatus,
  type ChangeSetView,
  type ChapterView,
  type CharacterInput,
  type FactInput,
  type OutlineContentInput,
  type StudioClient,
} from "../../lib/studio-client";

type Props = {
  client: StudioClient;
  chapters: ChapterView[];
  onChapterUpdated(chapter: ChapterView): void;
};

const statusNames: Record<ChangeSetStatus, string> = {
  pending: "待审阅",
  partially_applied: "部分已应用",
  applied: "已应用",
  rejected: "已拒绝",
  conflicted: "存在冲突",
};

const kindNames: Record<ChangeOperationKind, string> = {
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

const fieldNames: Record<string, string> = {
  title: "标题",
  summary: "摘要",
  goal: "目标",
  conflict: "冲突",
  outcome: "结果",
  name: "姓名",
  aliases: "别名",
  role: "角色类型",
  motivation: "动机",
  fear: "恐惧",
  secret: "秘密",
  arc: "人物弧",
  status: "状态",
  text: "事实内容",
  category: "分类",
  confidence: "置信度",
  validFromEventId: "生效事件",
  validUntilEventId: "失效事件",
};

function messageOf(cause: unknown) {
  if (cause instanceof RevisionConflictError) {
    return "目标内容已在候选生成后发生变化。为保护作者的新修改，本次没有应用候选；可拒绝冲突项后重新生成。";
  }
  return cause instanceof Error ? cause.message : "无法处理候选变更";
}

function parseValue(operation: ChangeOperationView): Record<string, unknown> {
  if (operation.kind === "replace_chapter_body") {
    return { body: operation.afterValue };
  }
  try {
    return JSON.parse(operation.afterValue) as Record<string, unknown>;
  } catch {
    return {};
  }
}

function displayValue(value: unknown) {
  if (Array.isArray(value)) return value.join("、") || "（无）";
  if (value === "" || value == null) return "（空）";
  if (typeof value === "number") return String(value);
  return String(value);
}

function structuredValue(raw: string): Record<string, unknown> {
  try {
    return JSON.parse(raw) as Record<string, unknown>;
  } catch {
    return {};
  }
}

function OperationComparison({ operation }: { operation: ChangeOperationView }) {
  if (operation.kind === "replace_chapter_body") {
    return (
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
    );
  }
  const before = structuredValue(operation.beforeValue);
  const after = structuredValue(operation.afterValue);
  const fields = Array.from(new Set([...Object.keys(before), ...Object.keys(after)]));
  return (
    <section className="structured-change-compare" aria-label="结构化字段对比">
      <div className="structured-change-heading">
        <span>字段</span>
        <span>修改前</span>
        <span>修改后</span>
      </div>
      {fields.map((field) => (
        <div className="structured-change-row" key={field}>
          <strong>{fieldNames[field] ?? field}</strong>
          <span>{displayValue(before[field])}</span>
          <span className={before[field] !== after[field] ? "is-changed" : ""}>
            {displayValue(after[field])}
          </span>
        </div>
      ))}
    </section>
  );
}

function OperationEditor({
  operation,
  working,
  onCancel,
  onSave,
}: {
  operation: ChangeOperationView;
  working: boolean;
  onCancel(): void;
  onSave(value: Record<string, unknown>): void;
}) {
  const [value, setValue] = useState<Record<string, unknown>>(() =>
    parseValue(operation),
  );
  const update = (field: string, next: unknown) =>
    setValue((current) => ({ ...current, [field]: next }));
  const textField = (field: string, multiline = false) => (
    <label key={field}>
      <span>{fieldNames[field] ?? field}</span>
      {multiline ? (
        <textarea
          value={String(value[field] ?? "")}
          onChange={(event) => update(field, event.target.value)}
        />
      ) : (
        <input
          value={String(value[field] ?? "")}
          onChange={(event) => update(field, event.target.value)}
        />
      )}
    </label>
  );

  return (
    <section className="change-operation-editor" aria-label="修改候选内容">
      <h4>修改候选内容</h4>
      <div className="change-operation-fields">
        {operation.kind === "replace_chapter_body" && textField("body", true)}
        {operation.kind === "update_outline_node" && (
          <>
            {textField("title")}
            {textField("summary", true)}
            {textField("goal", true)}
            {textField("conflict", true)}
            {textField("outcome", true)}
          </>
        )}
        {operation.kind === "update_character" && (
          <>
            {textField("name")}
            <label>
              <span>别名（用逗号分隔）</span>
              <input
                value={
                  Array.isArray(value.aliases)
                    ? value.aliases.join(", ")
                    : String(value.aliases ?? "")
                }
                onChange={(event) =>
                  update(
                    "aliases",
                    event.target.value
                      .split(/[,，]/)
                      .map((item) => item.trim())
                      .filter(Boolean),
                  )
                }
              />
            </label>
            <label>
              <span>角色类型</span>
              <select
                value={String(value.role ?? "supporting")}
                onChange={(event) => update("role", event.target.value)}
              >
                <option value="protagonist">主角</option>
                <option value="supporting">重要配角</option>
                <option value="antagonist">对立角色</option>
                <option value="minor">次要角色</option>
              </select>
            </label>
            {textField("summary", true)}
            {textField("motivation", true)}
            {textField("fear", true)}
            {textField("secret", true)}
            {textField("arc", true)}
            <label>
              <span>状态</span>
              <select
                value={String(value.status ?? "active")}
                onChange={(event) => update("status", event.target.value)}
              >
                <option value="active">活跃</option>
                <option value="inactive">暂离</option>
                <option value="deceased">已故</option>
              </select>
            </label>
          </>
        )}
        {operation.kind === "update_fact" && (
          <>
            {textField("text", true)}
            {textField("category")}
            <label>
              <span>状态</span>
              <select
                value={String(value.status ?? "ai_inferred")}
                onChange={(event) => update("status", event.target.value)}
              >
                <option value="author_confirmed">作者确认</option>
                <option value="text_evidenced">正文有据</option>
                <option value="ai_inferred">AI 推断</option>
                <option value="conflicted">有冲突</option>
                <option value="rejected">已否定</option>
              </select>
            </label>
            <label>
              <span>置信度</span>
              <input
                type="number"
                min="0"
                max="1"
                step="0.05"
                value={Number(value.confidence ?? 0)}
                onChange={(event) =>
                  update("confidence", Number(event.target.value))
                }
              />
            </label>
            {textField("validFromEventId")}
            {textField("validUntilEventId")}
          </>
        )}
      </div>
      <div className="change-editor-actions">
        <button type="button" className="secondary-action" onClick={onCancel}>
          取消
        </button>
        <button
          type="button"
          className="primary-action"
          disabled={working}
          onClick={() => onSave(value)}
        >
          保存候选
        </button>
      </div>
    </section>
  );
}

export function ChangeReviewWorkbench({
  client,
  chapters,
  onChapterUpdated,
}: Props) {
  const [changeSets, setChangeSets] = useState<ChangeSetView[]>([]);
  const [selectedID, setSelectedID] = useState("");
  const [selectedOperationID, setSelectedOperationID] = useState("");
  const [editing, setEditing] = useState(false);
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
          : (next.find((item) =>
                item.operations.some((operation) => operation.status === "pending"),
              )?.id ??
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

  useEffect(() => {
    if (!selected) return;
    setSelectedOperationID((current) =>
      selected.operations.some((item) => item.id === current)
        ? current
        : (selected.operations.find((item) => item.status === "pending")?.id ??
          selected.operations[0]?.id ??
          ""),
    );
    setEditing(false);
  }, [selected]);

  const operation = selected?.operations.find(
    (item) => item.id === selectedOperationID,
  );
  const targetName =
    operation?.kind === "replace_chapter_body"
      ? (chapterByID.get(operation.targetId)?.title ?? operation.targetId)
      : operation?.targetId;

  const refreshAfterApply = async (result: Awaited<ReturnType<StudioClient["applyChangeSet"]>>) => {
    for (const chapter of result.chapters) onChapterUpdated(chapter);
    setError(null);
    await load();
  };

  const applyGroup = async () => {
    if (!selected) return;
    setWorking(true);
    try {
      await refreshAfterApply(await client.applyChangeSet(selected.id));
    } catch (cause) {
      setError(messageOf(cause));
      await load();
    } finally {
      setWorking(false);
    }
  };

  const rejectGroup = async () => {
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

  const applyOperation = async () => {
    if (!selected || !operation) return;
    setWorking(true);
    try {
      const result = await client.applyChangeOperation(
        selected.id,
        operation.id,
      );
      for (const chapter of result.chapters) onChapterUpdated(chapter);
      setError(null);
    } catch (cause) {
      setError(messageOf(cause));
    } finally {
      await load();
      setWorking(false);
    }
  };

  const rejectOperation = async () => {
    if (!selected || !operation) return;
    setWorking(true);
    try {
      await client.rejectChangeOperation(selected.id, operation.id);
      setError(null);
    } catch (cause) {
      setError(messageOf(cause));
    } finally {
      await load();
      setWorking(false);
    }
  };

  const saveEdit = async (value: Record<string, unknown>) => {
    if (!selected || !operation) return;
    const input: ChangeOperationEditInput = {
      changeSetId: selected.id,
      operationId: operation.id,
      chapterBody: "",
    };
    if (operation.kind === "replace_chapter_body") {
      input.chapterBody = String(value.body ?? "");
    } else if (operation.kind === "update_outline_node") {
      input.outline = value as OutlineContentInput;
    } else if (operation.kind === "update_character") {
      input.character = value as CharacterInput;
    } else {
      input.fact = {
        ...(value as FactInput),
        origin: "model",
      };
    }
    setWorking(true);
    try {
      await client.editChangeOperation(input);
      setEditing(false);
      setError(null);
    } catch (cause) {
      setError(messageOf(cause));
    } finally {
      await load();
      setWorking(false);
    }
  };

  const pendingCount = changeSets.reduce(
    (total, item) =>
      total + item.operations.filter((candidate) => candidate.status === "pending").length,
    0,
  );

  return (
    <section className="change-review-workbench">
      <header className="change-review-header">
        <div>
          <p className="eyebrow">Reviewable Changes</p>
          <h2>候选变更</h2>
          <p>正文、人物、大纲和事实修改均需作者确认后才会进入正式作品。</p>
        </div>
        <span className="change-review-count">{pendingCount} 项待审阅</span>
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
                  <small>
                    {item.operations.length} 项 · {item.summary || "未填写变更摘要"}
                  </small>
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
                  <span
                    className={`change-status change-status--${selected.status}`}
                  >
                    {statusNames[selected.status]}
                  </span>
                  <h3>{selected.title}</h3>
                  <p>{selected.summary || "未填写变更摘要"}</p>
                </div>
                <dl>
                  <div>
                    <dt>当前目标</dt>
                    <dd>{targetName}</dd>
                  </div>
                  <div>
                    <dt>变更类型</dt>
                    <dd>{kindNames[operation.kind]}</dd>
                  </div>
                  <div>
                    <dt>来源</dt>
                    <dd>{selected.createdBy}</dd>
                  </div>
                </dl>
              </header>

              <nav className="change-operation-list" aria-label="变更项列表">
                {selected.operations.map((item, index) => (
                  <button
                    type="button"
                    key={item.id}
                    className={item.id === operation.id ? "is-active" : ""}
                    onClick={() => {
                      setSelectedOperationID(item.id);
                      setEditing(false);
                    }}
                  >
                    <ListChecks size={15} />
                    <span>
                      {index + 1}. {kindNames[item.kind]}
                    </span>
                    <em className={`change-status change-status--${item.status}`}>
                      {statusNames[item.status]}
                    </em>
                  </button>
                ))}
              </nav>

              {operation.reason && (
                <section className="change-reason">
                  <h4>修改理由</h4>
                  <p>{operation.reason}</p>
                </section>
              )}

              {editing ? (
                <OperationEditor
                  key={operation.id}
                  operation={operation}
                  working={working}
                  onCancel={() => setEditing(false)}
                  onSave={(value) => void saveEdit(value)}
                />
              ) : (
                <OperationComparison operation={operation} />
              )}

              <footer className="change-item-actions">
                {(operation.status === "pending" ||
                  operation.status === "conflicted") && (
                  <button
                    type="button"
                    className="secondary-action"
                    disabled={working}
                    onClick={() => void rejectOperation()}
                  >
                    <X size={16} />
                    拒绝此项
                  </button>
                )}
                {operation.status === "pending" && !editing && (
                  <>
                    <button
                      type="button"
                      className="secondary-action"
                      disabled={working}
                      onClick={() => setEditing(true)}
                    >
                      <Pencil size={15} />
                      修改候选
                    </button>
                    <button
                      type="button"
                      className="primary-action"
                      disabled={working}
                      onClick={() => void applyOperation()}
                    >
                      <Check size={16} />
                      接受此项
                    </button>
                  </>
                )}
              </footer>

              {(selected.operations.some((item) => item.status === "pending") ||
                selected.status === "conflicted") && (
                <footer className="change-group-actions">
                  <span>整组操作会在同一事务中完成，并预先创建恢复点。</span>
                  <button
                    type="button"
                    className="secondary-action"
                    disabled={working}
                    onClick={() => void rejectGroup()}
                  >
                    拒绝整组
                  </button>
                  {selected.operations.some(
                    (item) => item.status === "pending",
                  ) && (
                    <button
                      type="button"
                      className="primary-action"
                      disabled={working}
                      onClick={() => void applyGroup()}
                    >
                      {working ? "正在应用…" : "接受剩余并应用"}
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
