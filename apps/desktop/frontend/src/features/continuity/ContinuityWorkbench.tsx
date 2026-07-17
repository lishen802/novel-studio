import {
  Archive,
  BookCheck,
  Clock3,
  Link2,
  Pencil,
  Plus,
  Scale,
  ShieldCheck,
  TriangleAlert,
} from "lucide-react";
import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type FormEvent,
} from "react";
import type {
  ChapterView,
  ContinuityView,
  EventTargetType,
  FactInput,
  FactScopeTargetType,
  FactView,
  StoryEventInput,
  StoryEventView,
  StoryWorldView,
  StudioClient,
  WorldAssetsView,
} from "../../lib/studio-client";

type Props = { client: StudioClient; chapters: ChapterView[] };
type Tab = "timeline" | "facts";
type DialogKind =
  "event" | "event-link" | "fact" | "evidence" | "scope" | "conflict" | null;
type ArchiveTarget = { kind: "event" | "fact"; id: string; name: string };

const emptyContinuity: ContinuityView = {
  events: [],
  eventLinks: [],
  facts: [],
  evidence: [],
  scopes: [],
  conflicts: [],
  consistencyIssues: [],
};
const emptyWorld: StoryWorldView = {
  characters: [],
  factions: [],
  memberships: [],
  relationships: [],
  chapterAppearances: [],
  sceneAppearances: [],
  consistencyIssues: [],
};
const emptyAssets: WorldAssetsView = {
  regions: [],
  locations: [],
  locationLinks: [],
  items: [],
  itemTransfers: [],
  rules: [],
  hooks: [],
  consistencyIssues: [],
};
const emptyEvent: StoryEventInput = {
  title: "",
  summary: "",
  cause: "",
  result: "",
  storyTime: "",
  storyOrder: 0,
  narrativeOrder: 1,
  status: "planned",
};
const emptyFact: FactInput = {
  text: "",
  category: "设定",
  origin: "author",
  status: "author_confirmed",
  confidence: 1,
  validFromEventId: "",
  validUntilEventId: "",
};

const eventStatusNames = {
  planned: "计划中",
  occurred: "已发生",
  uncertain: "待确认",
} as const;
const factStatusNames = {
  author_confirmed: "作者确认",
  text_evidenced: "正文证实",
  ai_inferred: "AI 推断",
  conflicted: "有冲突",
  rejected: "已否决",
} as const;
const originNames = {
  author: "作者",
  text: "正文",
  model: "模型",
  rule: "规则",
} as const;

function messageOf(cause: unknown, fallback: string) {
  return cause instanceof Error ? cause.message : fallback;
}
function Dialog({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="dialog-backdrop">
      <div
        className="project-dialog world-dialog continuity-dialog"
        role="dialog"
        aria-label={label}
      >
        {children}
      </div>
    </div>
  );
}

export function ContinuityWorkbench({ client, chapters }: Props) {
  const [data, setData] = useState(emptyContinuity);
  const [world, setWorld] = useState(emptyWorld);
  const [assets, setAssets] = useState(emptyAssets);
  const [tab, setTab] = useState<Tab>("timeline");
  const [statusFilter, setStatusFilter] = useState("all");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [dialog, setDialog] = useState<DialogKind>(null);
  const [editingId, setEditingId] = useState("");
  const [event, setEvent] = useState(emptyEvent);
  const [fact, setFact] = useState(emptyFact);
  const [selectedFactId, setSelectedFactId] = useState("");
  const [linkType, setLinkType] = useState<EventTargetType>("chapter");
  const [linkTargetId, setLinkTargetId] = useState("");
  const [linkRole, setLinkRole] = useState("");
  const [evidenceChapterId, setEvidenceChapterId] = useState("");
  const [quote, setQuote] = useState("");
  const [scopeType, setScopeType] = useState<FactScopeTargetType>("character");
  const [scopeTargetId, setScopeTargetId] = useState("");
  const [otherFactId, setOtherFactId] = useState("");
  const [conflictReason, setConflictReason] = useState("");
  const [archiveTarget, setArchiveTarget] = useState<ArchiveTarget | null>(
    null,
  );

  const load = useCallback(async () => {
    try {
      const [next, nextWorld, nextAssets] = await Promise.all([
        client.continuity(),
        client.storyWorld(),
        client.worldAssets(),
      ]);
      setData(next);
      setWorld(nextWorld);
      setAssets(nextAssets);
      setError(null);
    } catch (cause) {
      setError(messageOf(cause, "无法读取时间线与事实"));
    } finally {
      setLoading(false);
    }
  }, [client]);
  useEffect(() => {
    void load();
  }, [load]);

  const mutate = async (
    operation: () => Promise<unknown>,
    fallback: string,
  ) => {
    setSaving(true);
    try {
      await operation();
      await load();
      setError(null);
      return true;
    } catch (cause) {
      setError(messageOf(cause, fallback));
      return false;
    } finally {
      setSaving(false);
    }
  };
  const closeDialog = () => {
    setDialog(null);
    setEditingId("");
  };

  const targetOptions = useMemo(
    () => ({
      chapter: chapters.map((item) => ({ id: item.id, name: item.title })),
      character: world.characters.map((item) => ({
        id: item.id,
        name: item.name,
      })),
      faction: world.factions.map((item) => ({ id: item.id, name: item.name })),
      location: assets.locations.map((item) => ({
        id: item.id,
        name: item.name,
      })),
      item: assets.items.map((item) => ({ id: item.id, name: item.name })),
    }),
    [chapters, world, assets],
  );
  const scopeOptions = useMemo(
    () => ({
      chapter: targetOptions.chapter,
      character: targetOptions.character,
      faction: targetOptions.faction,
      location: targetOptions.location,
      item: targetOptions.item,
      region: assets.regions.map((item) => ({ id: item.id, name: item.name })),
      rule: assets.rules.map((item) => ({ id: item.id, name: item.name })),
      event: data.events.map((item) => ({ id: item.id, name: item.title })),
    }),
    [targetOptions, assets, data.events],
  );
  const targetName = (
    type: EventTargetType | FactScopeTargetType,
    id: string,
  ) => scopeOptions[type]?.find((item) => item.id === id)?.name ?? id;

  const openEvent = (value?: StoryEventView) => {
    if (value) {
      const { id, ...input } = value;
      setEditingId(id);
      setEvent(input);
    } else {
      setEditingId("");
      setEvent({
        ...emptyEvent,
        narrativeOrder: data.events.length + 1,
        storyOrder: data.events.length
          ? Math.max(...data.events.map((item) => item.storyOrder)) + 10
          : 0,
      });
    }
    setDialog("event");
  };
  const saveEvent = async (form: FormEvent) => {
    form.preventDefault();
    const ok = await mutate(
      () =>
        editingId
          ? client.updateStoryEvent(editingId, event)
          : client.createStoryEvent(event),
      "无法保存事件",
    );
    if (ok) closeDialog();
  };
  const openFact = (value?: FactView) => {
    if (value) {
      setEditingId(value.id);
      setFact({
        text: value.text,
        category: value.category,
        origin: value.origin,
        status: value.status,
        confidence: value.confidence,
        validFromEventId: value.validFromEventId,
        validUntilEventId: value.validUntilEventId,
      });
    } else {
      setEditingId("");
      setFact(emptyFact);
    }
    setDialog("fact");
  };
  const saveFact = async (form: FormEvent) => {
    form.preventDefault();
    const ok = await mutate(
      () =>
        editingId
          ? client.updateFact(editingId, fact)
          : client.createFact(fact),
      "无法保存事实",
    );
    if (ok) closeDialog();
  };
  const openEventLink = (eventId: string) => {
    setEditingId(eventId);
    setLinkType("chapter");
    setLinkTargetId("");
    setLinkRole("");
    setDialog("event-link");
  };
  const saveEventLink = async (form: FormEvent) => {
    form.preventDefault();
    const ok = await mutate(
      () =>
        client.linkStoryEvent({
          eventId: editingId,
          targetType: linkType,
          targetId: linkTargetId,
          role: linkRole,
        }),
      "无法关联事件",
    );
    if (ok) closeDialog();
  };
  const openEvidence = (factId: string) => {
    setSelectedFactId(factId);
    setEvidenceChapterId("");
    setQuote("");
    setDialog("evidence");
  };
  const saveEvidence = async (form: FormEvent) => {
    form.preventDefault();
    const chapter = chapters.find((item) => item.id === evidenceChapterId);
    const body = Array.from(chapter?.body ?? "");
    const excerpt = Array.from(quote);
    let start = -1;
    outer: for (let i = 0; i <= body.length - excerpt.length; i++) {
      for (let j = 0; j < excerpt.length; j++)
        if (body[i + j] !== excerpt[j]) continue outer;
      start = i;
      break;
    }
    if (!chapter || start < 0 || !excerpt.length) {
      setError("原文摘录必须与所选章节当前正文完全一致。");
      return;
    }
    const ok = await mutate(
      () =>
        client.addFactEvidence({
          factId: selectedFactId,
          chapterId: chapter.id,
          quote,
          startOffset: start,
          endOffset: start + excerpt.length,
          chapterRevision: chapter.revision,
        }),
      "无法保存正文证据",
    );
    if (ok) closeDialog();
  };
  const openScope = (factId: string) => {
    setSelectedFactId(factId);
    setScopeType("character");
    setScopeTargetId("");
    setDialog("scope");
  };
  const saveScope = async (form: FormEvent) => {
    form.preventDefault();
    const ok = await mutate(
      () =>
        client.addFactScope({
          factId: selectedFactId,
          targetType: scopeType,
          targetId: scopeTargetId,
        }),
      "无法保存作用范围",
    );
    if (ok) closeDialog();
  };
  const openConflict = (factId: string) => {
    setSelectedFactId(factId);
    setOtherFactId("");
    setConflictReason("");
    setDialog("conflict");
  };
  const saveConflict = async (form: FormEvent) => {
    form.preventDefault();
    const ok = await mutate(
      () =>
        client.createFactConflict({
          factId: selectedFactId,
          otherFactId,
          reason: conflictReason,
        }),
      "无法保存事实冲突",
    );
    if (ok) closeDialog();
  };

  const events = data.events
    .filter((item) => statusFilter === "all" || item.status === statusFilter)
    .sort(
      (a, b) =>
        a.storyOrder - b.storyOrder || a.narrativeOrder - b.narrativeOrder,
    );
  const facts = data.facts.filter(
    (item) => statusFilter === "all" || item.status === statusFilter,
  );

  return (
    <section className="continuity-workbench">
      <header className="continuity-header">
        <div>
          <p className="eyebrow">CONTINUITY CONTROL</p>
          <h2>时间线与事实</h2>
          <p>同时管理故事发生顺序、叙事揭示顺序与可追溯的设定依据。</p>
        </div>
        <button
          type="button"
          onClick={() => (tab === "timeline" ? openEvent() : openFact())}
        >
          <Plus size={17} />
          {tab === "timeline" ? "新建事件" : "新建事实"}
        </button>
      </header>
      {error && (
        <div className="workspace-alert" role="alert">
          <span>{error}</span>
          <button type="button" onClick={() => setError(null)}>
            关闭
          </button>
        </div>
      )}
      {data.consistencyIssues.length > 0 && (
        <div className="continuity-issues" role="alert">
          <TriangleAlert size={18} />
          <div>
            {data.consistencyIssues.map((issue) => (
              <p key={`${issue.code}-${issue.entityIds.join("-")}`}>
                {issue.message}
              </p>
            ))}
          </div>
        </div>
      )}
      <div className="continuity-toolbar">
        <div className="continuity-tabs" role="tablist">
          <button
            type="button"
            role="tab"
            aria-selected={tab === "timeline"}
            className={tab === "timeline" ? "is-active" : ""}
            onClick={() => {
              setTab("timeline");
              setStatusFilter("all");
            }}
          >
            <Clock3 size={16} />
            时间线
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={tab === "facts"}
            className={tab === "facts" ? "is-active" : ""}
            onClick={() => {
              setTab("facts");
              setStatusFilter("all");
            }}
          >
            <Scale size={16} />
            事实库
          </button>
        </div>
        <label>
          状态筛选
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
          >
            <option value="all">全部</option>
            {tab === "timeline"
              ? Object.entries(eventStatusNames).map(([value, name]) => (
                  <option key={value} value={value}>
                    {name}
                  </option>
                ))
              : Object.entries(factStatusNames).map(([value, name]) => (
                  <option key={value} value={value}>
                    {name}
                  </option>
                ))}
          </select>
        </label>
      </div>
      {loading ? (
        <p className="world-muted">正在读取连续性资料…</p>
      ) : tab === "timeline" ? (
        <div className="timeline-list">
          {events.length === 0 ? (
            <div className="continuity-empty">
              <Clock3 />
              <p>还没有时间线事件。</p>
            </div>
          ) : (
            events.map((item) => (
              <article className="timeline-card" key={item.id}>
                <div className="timeline-marker">
                  <span>{item.storyOrder}</span>
                </div>
                <div className="timeline-card__body">
                  <div className="continuity-card-heading">
                    <div>
                      <span
                        className={`continuity-status status-${item.status}`}
                      >
                        {eventStatusNames[item.status]}
                      </span>
                      <h3>{item.title}</h3>
                      <p>
                        故事顺序 {item.storyOrder} · 叙事顺序{" "}
                        {item.narrativeOrder}
                        {item.storyTime ? ` · ${item.storyTime}` : ""}
                      </p>
                    </div>
                    <div>
                      <button
                        type="button"
                        aria-label={`编辑${item.title}`}
                        onClick={() => openEvent(item)}
                      >
                        <Pencil size={15} />
                      </button>
                      <button
                        type="button"
                        aria-label={`关联${item.title}`}
                        onClick={() => openEventLink(item.id)}
                      >
                        <Link2 size={15} />
                      </button>
                      <button
                        type="button"
                        aria-label={`删除${item.title}`}
                        onClick={() =>
                          setArchiveTarget({
                            kind: "event",
                            id: item.id,
                            name: item.title,
                          })
                        }
                      >
                        <Archive size={15} />
                      </button>
                    </div>
                  </div>
                  {item.summary && <p>{item.summary}</p>}{" "}
                  {(item.cause || item.result) && (
                    <div className="continuity-causality">
                      <span>因：{item.cause || "未设置"}</span>
                      <span>果：{item.result || "未设置"}</span>
                    </div>
                  )}
                  <div className="world-tags">
                    {data.eventLinks
                      .filter((link) => link.eventId === item.id)
                      .map((link) => (
                        <span key={`${link.targetType}-${link.targetId}`}>
                          {link.role || "关联"} ·{" "}
                          {targetName(link.targetType, link.targetId)}
                          <button
                            type="button"
                            aria-label={`解除关联${targetName(link.targetType, link.targetId)}`}
                            onClick={() =>
                              void mutate(
                                () =>
                                  client.unlinkStoryEvent(
                                    link.eventId,
                                    link.targetType,
                                    link.targetId,
                                  ),
                                "无法解除事件关联",
                              )
                            }
                          >
                            ×
                          </button>
                        </span>
                      ))}
                  </div>
                </div>
              </article>
            ))
          )}
        </div>
      ) : (
        <div className="fact-grid">
          {facts.length === 0 ? (
            <div className="continuity-empty">
              <BookCheck />
              <p>还没有事实记录。</p>
            </div>
          ) : (
            facts.map((item) => (
              <article className="fact-card" key={item.id}>
                <div className="continuity-card-heading">
                  <div>
                    <span className={`continuity-status status-${item.status}`}>
                      {factStatusNames[item.status]}
                    </span>
                    <h3>{item.text}</h3>
                    <p>
                      {item.category} · 来源 {originNames[item.origin]} · 置信度{" "}
                      {Math.round(item.confidence * 100)}%
                    </p>
                  </div>
                  <div>
                    <button
                      type="button"
                      aria-label={`编辑${item.text}`}
                      onClick={() => openFact(item)}
                    >
                      <Pencil size={15} />
                    </button>
                    <button
                      type="button"
                      aria-label={`删除${item.text}`}
                      onClick={() =>
                        setArchiveTarget({
                          kind: "fact",
                          id: item.id,
                          name: item.text,
                        })
                      }
                    >
                      <Archive size={15} />
                    </button>
                  </div>
                </div>
                {(item.validFromEventId || item.validUntilEventId) && (
                  <p className="fact-validity">
                    有效区间：
                    {data.events.find(
                      (event) => event.id === item.validFromEventId,
                    )?.title || "起始不限"}{" "}
                    →{" "}
                    {data.events.find(
                      (event) => event.id === item.validUntilEventId,
                    )?.title || "持续有效"}
                  </p>
                )}
                <div className="fact-actions">
                  {item.status !== "author_confirmed" && (
                    <button
                      type="button"
                      aria-label={`作者确认${item.text}`}
                      onClick={() =>
                        void mutate(
                          () => client.confirmFact(item.id),
                          "无法确认事实",
                        )
                      }
                    >
                      <ShieldCheck size={15} />
                      作者确认
                    </button>
                  )}
                  <button
                    type="button"
                    aria-label={`添加证据${item.text}`}
                    onClick={() => openEvidence(item.id)}
                  >
                    添加证据
                  </button>
                  <button
                    type="button"
                    aria-label={`设置范围${item.text}`}
                    onClick={() => openScope(item.id)}
                  >
                    设置范围
                  </button>
                  <button
                    type="button"
                    aria-label={`标记冲突${item.text}`}
                    onClick={() => openConflict(item.id)}
                  >
                    标记冲突
                  </button>
                </div>
                <div className="fact-detail">
                  <h4>正文证据</h4>
                  {data.evidence
                    .filter((value) => value.factId === item.id)
                    .map((value) => (
                      <div className="fact-evidence" key={value.id}>
                        <q>{value.quote}</q>
                        <span>
                          {chapters.find(
                            (chapter) => chapter.id === value.chapterId,
                          )?.title ?? value.chapterId}{" "}
                          · 修订 {value.chapterRevision}
                        </span>
                        <button
                          type="button"
                          aria-label={`删除证据${value.quote}`}
                          onClick={() =>
                            void mutate(
                              () => client.removeFactEvidence(value.id),
                              "无法删除证据",
                            )
                          }
                        >
                          ×
                        </button>
                      </div>
                    ))}
                </div>
                <div className="world-tags">
                  {data.scopes
                    .filter((value) => value.factId === item.id)
                    .map((value) => (
                      <span key={value.id}>
                        适用：{targetName(value.targetType, value.targetId)}
                        <button
                          type="button"
                          aria-label={`删除范围${targetName(value.targetType, value.targetId)}`}
                          onClick={() =>
                            void mutate(
                              () => client.removeFactScope(value.id),
                              "无法删除范围",
                            )
                          }
                        >
                          ×
                        </button>
                      </span>
                    ))}
                </div>
                {data.conflicts
                  .filter(
                    (value) =>
                      value.factId === item.id || value.otherFactId === item.id,
                  )
                  .map((value) => (
                    <div
                      className={`fact-conflict ${value.status === "resolved" ? "is-resolved" : ""}`}
                      key={value.id}
                    >
                      <TriangleAlert size={15} />
                      <span>{value.reason}</span>
                      {value.status === "open" ? (
                        <button
                          type="button"
                          aria-label={`解决冲突${item.text}${value.reason}`}
                          onClick={() =>
                            void mutate(
                              () => client.resolveFactConflict(value.id),
                              "无法解决冲突",
                            )
                          }
                        >
                          标记已解决
                        </button>
                      ) : (
                        <em>已解决</em>
                      )}
                      <button
                        type="button"
                        aria-label={`删除冲突${item.text}${value.reason}`}
                        onClick={() =>
                          void mutate(
                            () => client.deleteFactConflict(value.id),
                            "无法删除冲突",
                          )
                        }
                      >
                        ×
                      </button>
                    </div>
                  ))}
              </article>
            ))
          )}
        </div>
      )}

      {dialog === "event" && (
        <Dialog label={editingId ? "编辑事件" : "新建事件"}>
          <form onSubmit={saveEvent}>
            <p className="eyebrow">STORY EVENT</p>
            <h2>{editingId ? "编辑事件" : "新建事件"}</h2>
            <div className="dialog-field-grid">
              <label>
                事件标题
                <input
                  autoFocus
                  required
                  value={event.title}
                  onChange={(e) =>
                    setEvent({ ...event, title: e.target.value })
                  }
                />
              </label>
              <label>
                故事内时间
                <input
                  value={event.storyTime}
                  onChange={(e) =>
                    setEvent({ ...event, storyTime: e.target.value })
                  }
                />
              </label>
              <label>
                故事顺序
                <input
                  type="number"
                  min="0"
                  required
                  value={event.storyOrder}
                  onChange={(e) =>
                    setEvent({ ...event, storyOrder: Number(e.target.value) })
                  }
                />
              </label>
              <label>
                叙事顺序
                <input
                  type="number"
                  min="1"
                  required
                  value={event.narrativeOrder}
                  onChange={(e) =>
                    setEvent({
                      ...event,
                      narrativeOrder: Number(e.target.value),
                    })
                  }
                />
              </label>
              <label>
                状态
                <select
                  value={event.status}
                  onChange={(e) =>
                    setEvent({
                      ...event,
                      status: e.target.value as StoryEventInput["status"],
                    })
                  }
                >
                  {Object.entries(eventStatusNames).map(([value, name]) => (
                    <option value={value} key={value}>
                      {name}
                    </option>
                  ))}
                </select>
              </label>
            </div>
            <label>
              事件摘要
              <textarea
                value={event.summary}
                onChange={(e) =>
                  setEvent({ ...event, summary: e.target.value })
                }
              />
            </label>
            <div className="dialog-field-grid">
              <label>
                原因
                <textarea
                  value={event.cause}
                  onChange={(e) =>
                    setEvent({ ...event, cause: e.target.value })
                  }
                />
              </label>
              <label>
                结果
                <textarea
                  value={event.result}
                  onChange={(e) =>
                    setEvent({ ...event, result: e.target.value })
                  }
                />
              </label>
            </div>
            <div className="dialog-actions">
              <button
                type="button"
                className="secondary-action"
                onClick={closeDialog}
              >
                取消
              </button>
              <button
                type="submit"
                className="primary-action"
                disabled={saving}
              >
                保存事件
              </button>
            </div>
          </form>
        </Dialog>
      )}
      {dialog === "fact" && (
        <Dialog label={editingId ? "编辑事实" : "新建事实"}>
          <form onSubmit={saveFact}>
            <p className="eyebrow">CANON FACT</p>
            <h2>{editingId ? "编辑事实" : "新建事实"}</h2>
            <label>
              事实内容
              <textarea
                autoFocus
                required
                value={fact.text}
                onChange={(e) => setFact({ ...fact, text: e.target.value })}
              />
            </label>
            <div className="dialog-field-grid">
              <label>
                分类
                <input
                  required
                  value={fact.category}
                  onChange={(e) =>
                    setFact({ ...fact, category: e.target.value })
                  }
                />
              </label>
              <label>
                来源
                <select
                  value={fact.origin}
                  disabled={Boolean(editingId)}
                  onChange={(e) =>
                    setFact({
                      ...fact,
                      origin: e.target.value as FactInput["origin"],
                    })
                  }
                >
                  {Object.entries(originNames).map(([value, name]) => (
                    <option key={value} value={value}>
                      {name}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                状态
                <select
                  value={fact.status}
                  onChange={(e) =>
                    setFact({
                      ...fact,
                      status: e.target.value as FactInput["status"],
                    })
                  }
                >
                  {Object.entries(factStatusNames)
                    .filter(
                      ([value]) =>
                        value !== "author_confirmed" ||
                        !editingId ||
                        fact.status === "author_confirmed",
                    )
                    .map(([value, name]) => (
                      <option key={value} value={value}>
                        {name}
                      </option>
                    ))}
                </select>
              </label>
              <label>
                置信度
                <input
                  type="number"
                  min="0"
                  max="1"
                  step="0.01"
                  value={fact.confidence}
                  onChange={(e) =>
                    setFact({ ...fact, confidence: Number(e.target.value) })
                  }
                />
              </label>
              <label>
                生效事件
                <select
                  value={fact.validFromEventId}
                  onChange={(e) =>
                    setFact({ ...fact, validFromEventId: e.target.value })
                  }
                >
                  <option value="">不限</option>
                  {data.events.map((value) => (
                    <option key={value.id} value={value.id}>
                      {value.title}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                失效事件
                <select
                  value={fact.validUntilEventId}
                  onChange={(e) =>
                    setFact({ ...fact, validUntilEventId: e.target.value })
                  }
                >
                  <option value="">持续有效</option>
                  {data.events.map((value) => (
                    <option key={value.id} value={value.id}>
                      {value.title}
                    </option>
                  ))}
                </select>
              </label>
            </div>
            <div className="dialog-actions">
              <button
                type="button"
                className="secondary-action"
                onClick={closeDialog}
              >
                取消
              </button>
              <button
                type="submit"
                className="primary-action"
                disabled={saving}
              >
                保存事实
              </button>
            </div>
          </form>
        </Dialog>
      )}
      {dialog === "event-link" && (
        <Dialog label="关联事件资料">
          <form onSubmit={saveEventLink}>
            <h2>关联事件资料</h2>
            <div className="dialog-field-grid">
              <label>
                资料类型
                <select
                  value={linkType}
                  onChange={(e) => {
                    setLinkType(e.target.value as EventTargetType);
                    setLinkTargetId("");
                  }}
                >
                  {Object.entries({
                    chapter: "章节",
                    character: "角色",
                    faction: "势力",
                    location: "地点",
                    item: "物品",
                  }).map(([v, n]) => (
                    <option key={v} value={v}>
                      {n}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                关联对象
                <select
                  required
                  value={linkTargetId}
                  onChange={(e) => setLinkTargetId(e.target.value)}
                >
                  <option value="">请选择</option>
                  {targetOptions[linkType].map((value) => (
                    <option key={value.id} value={value.id}>
                      {value.name}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                关联角色
                <input
                  value={linkRole}
                  onChange={(e) => setLinkRole(e.target.value)}
                  placeholder="发生章节、参与者…"
                />
              </label>
            </div>
            <div className="dialog-actions">
              <button
                type="button"
                className="secondary-action"
                onClick={closeDialog}
              >
                取消
              </button>
              <button type="submit" className="primary-action">
                保存关联
              </button>
            </div>
          </form>
        </Dialog>
      )}
      {dialog === "evidence" && (
        <Dialog label="添加正文证据">
          <form onSubmit={saveEvidence}>
            <h2>添加正文证据</h2>
            <p>
              摘录必须与当前章节正文逐字一致，保存时会记录修订号和字符位置。
            </p>
            <label>
              章节
              <select
                required
                value={evidenceChapterId}
                onChange={(e) => setEvidenceChapterId(e.target.value)}
              >
                <option value="">请选择章节</option>
                {chapters.map((value) => (
                  <option key={value.id} value={value.id}>
                    {value.title}
                  </option>
                ))}
              </select>
            </label>
            <label>
              原文摘录
              <textarea
                required
                value={quote}
                onChange={(e) => setQuote(e.target.value)}
              />
            </label>
            <div className="dialog-actions">
              <button
                type="button"
                className="secondary-action"
                onClick={closeDialog}
              >
                取消
              </button>
              <button type="submit" className="primary-action">
                保存证据
              </button>
            </div>
          </form>
        </Dialog>
      )}
      {dialog === "scope" && (
        <Dialog label="设置事实范围">
          <form onSubmit={saveScope}>
            <h2>设置事实范围</h2>
            <div className="dialog-field-grid">
              <label>
                对象类型
                <select
                  value={scopeType}
                  onChange={(e) => {
                    setScopeType(e.target.value as FactScopeTargetType);
                    setScopeTargetId("");
                  }}
                >
                  {Object.entries({
                    character: "角色",
                    faction: "势力",
                    location: "地点",
                    item: "物品",
                    region: "区域",
                    rule: "规则",
                    event: "事件",
                  }).map(([v, n]) => (
                    <option key={v} value={v}>
                      {n}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                适用对象
                <select
                  required
                  value={scopeTargetId}
                  onChange={(e) => setScopeTargetId(e.target.value)}
                >
                  <option value="">请选择</option>
                  {scopeOptions[scopeType].map((value) => (
                    <option key={value.id} value={value.id}>
                      {value.name}
                    </option>
                  ))}
                </select>
              </label>
            </div>
            <div className="dialog-actions">
              <button
                type="button"
                className="secondary-action"
                onClick={closeDialog}
              >
                取消
              </button>
              <button type="submit" className="primary-action">
                保存范围
              </button>
            </div>
          </form>
        </Dialog>
      )}
      {dialog === "conflict" && (
        <Dialog label="标记事实冲突">
          <form onSubmit={saveConflict}>
            <h2>标记事实冲突</h2>
            <label>
              冲突事实
              <select
                required
                value={otherFactId}
                onChange={(e) => setOtherFactId(e.target.value)}
              >
                <option value="">请选择另一事实</option>
                {data.facts
                  .filter((value) => value.id !== selectedFactId)
                  .map((value) => (
                    <option key={value.id} value={value.id}>
                      {value.text}
                    </option>
                  ))}
              </select>
            </label>
            <label>
              冲突原因
              <textarea
                required
                value={conflictReason}
                onChange={(e) => setConflictReason(e.target.value)}
              />
            </label>
            <div className="dialog-actions">
              <button
                type="button"
                className="secondary-action"
                onClick={closeDialog}
              >
                取消
              </button>
              <button type="submit" className="primary-action">
                保存冲突
              </button>
            </div>
          </form>
        </Dialog>
      )}
      {archiveTarget && (
        <Dialog label="确认删除">
          <h2>确认删除</h2>
          <p>将“{archiveTarget.name}”移入归档？仍被其他资料引用时不会删除。</p>
          <div className="dialog-actions">
            <button
              type="button"
              className="secondary-action"
              onClick={() => setArchiveTarget(null)}
            >
              取消
            </button>
            <button
              type="button"
              className="danger-action"
              onClick={() =>
                void mutate(
                  () =>
                    archiveTarget.kind === "event"
                      ? client.archiveStoryEvent(archiveTarget.id)
                      : client.archiveFact(archiveTarget.id),
                  "无法删除资料",
                ).then((ok) => {
                  if (ok) setArchiveTarget(null);
                })
              }
            >
              确认删除
            </button>
          </div>
        </Dialog>
      )}
    </section>
  );
}
