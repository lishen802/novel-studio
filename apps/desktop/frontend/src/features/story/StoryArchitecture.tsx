import { Archive, ArrowDown, ArrowUp, GitBranch, ListTree, Plus, Route, ScrollText } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import type {
  ChapterView,
  CreateOutlineNodeInput,
  OutlineLevel,
  OutlineNodeView,
  StoryStructureView,
  StudioClient,
} from "../../lib/studio-client";

type Props = { client: StudioClient; chapters: ChapterView[]; onOpenChapter(id: string): void };
const emptyStructure: StoryStructureView = { outlineNodes: [], storyLines: [], storyLineLinks: [], sceneCards: [] };
const levelNames: Record<OutlineLevel, string> = { book: "全书", volume: "分卷", arc: "故事弧", chapter: "章节", scene: "场景" };
const nextLevel: Partial<Record<OutlineLevel, OutlineLevel>> = { book: "volume", volume: "arc", arc: "chapter", chapter: "scene" };

export function StoryArchitecture({ client, chapters, onOpenChapter }: Props) {
  const [structure, setStructure] = useState(emptyStructure);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [dialog, setDialog] = useState<"outline" | "line" | "card" | null>(null);
  const [parentId, setParentId] = useState("");
  const [title, setTitle] = useState("");
  const [summary, setSummary] = useState("");
  const [chapterId, setChapterId] = useState("");
  const [pov, setPOV] = useState("");
  const [location, setLocation] = useState("");
  const [goal, setGoal] = useState("");
  const [conflict, setConflict] = useState("");
  const [outcome, setOutcome] = useState("");
  const [linkTargets, setLinkTargets] = useState<Record<string, string>>({});

  const load = useCallback(async () => {
    try { setStructure(await client.storyStructure()); setError(null); }
    catch (cause) { setError(cause instanceof Error ? cause.message : "无法读取故事架构"); }
    finally { setLoading(false); }
  }, [client]);
  useEffect(() => { void load(); }, [load]);

  const parent = structure.outlineNodes.find((node) => node.id === parentId);
  const outlineLevel: OutlineLevel = parent ? (nextLevel[parent.level] ?? "scene") : "book";
  const sceneNodes = structure.outlineNodes.filter((node) => node.level === "scene");
  const availableSceneNodes = sceneNodes.filter((node) =>
    structure.outlineNodes.some((parentNode) =>
      parentNode.id === node.parentId && parentNode.level === "chapter" && parentNode.chapterId === chapterId,
    ),
  );
  const grouped = useMemo(() => {
    const map = new Map<string, OutlineNodeView[]>();
    for (const node of structure.outlineNodes) map.set(node.parentId, [...(map.get(node.parentId) ?? []), node]);
    for (const nodes of map.values()) nodes.sort((a, b) => a.position - b.position);
    return map;
  }, [structure.outlineNodes]);

  const resetForm = () => {
    setDialog(null); setParentId(""); setTitle(""); setSummary(""); setChapterId("");
    setPOV(""); setLocation(""); setGoal(""); setConflict(""); setOutcome("");
  };

  const createOutline = async (event: FormEvent) => {
    event.preventDefault();
    const input: CreateOutlineNodeInput = { parentId, level: outlineLevel, title, summary, goal, conflict, outcome, chapterId: outlineLevel === "chapter" ? chapterId : "" };
    try { await client.createOutlineNode(input); resetForm(); await load(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : "无法创建大纲节点"); }
  };

  const createLine = async (event: FormEvent) => {
    event.preventDefault();
    try { await client.createStoryLine({ name: title, summary }); resetForm(); await load(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : "无法创建故事线"); }
  };

  const createCard = async (event: FormEvent) => {
    event.preventDefault();
    try {
      await client.createSceneCard({ chapterId, outlineNodeId: parentId, title, pov, location, goal, conflict, outcome });
      resetForm(); await load();
    } catch (cause) { setError(cause instanceof Error ? cause.message : "无法创建场景卡"); }
  };

  const linkStoryLine = async (storyLineId: string) => {
    const outlineNodeId = linkTargets[storyLineId];
    if (!outlineNodeId) return;
    try {
      await client.linkStoryLine({ storyLineId, outlineNodeId });
      setLinkTargets((current) => ({ ...current, [storyLineId]: "" }));
      await load();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "无法关联故事线");
    }
  };

  const moveOutline = async (id: string, direction: "up" | "down") => {
    try { await client.moveOutlineNode(id, direction); await load(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : "无法调整节点顺序"); }
  };

  const archiveOutline = async (node: OutlineNodeView) => {
    if (!window.confirm(`归档“${node.title}”？仍被引用的节点不会被删除。`)) return;
    try { await client.archiveOutlineNode(node.id); await load(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : "无法归档节点"); }
  };

  const renderBranch = (parentKey: string, depth = 0): React.ReactNode =>
    (grouped.get(parentKey) ?? []).map((node) => (
      <div key={node.id} className="outline-branch">
        <div className="outline-node" style={{ "--outline-depth": depth } as React.CSSProperties}>
          <span className={`outline-level outline-level--${node.level}`}>{levelNames[node.level]}</span>
          <div><strong>{node.title}</strong>{node.summary && <p>{node.summary}</p>}</div>
          <div className="outline-node-actions">
            {node.chapterId && <button type="button" onClick={() => onOpenChapter(node.chapterId)} aria-label={`打开章节：${node.title}`}>打开正文</button>}
            <button type="button" onClick={() => void moveOutline(node.id, "up")} aria-label={`上移：${node.title}`}><ArrowUp size={14} /></button>
            <button type="button" onClick={() => void moveOutline(node.id, "down")} aria-label={`下移：${node.title}`}><ArrowDown size={14} /></button>
            <button type="button" onClick={() => void archiveOutline(node)} aria-label={`归档：${node.title}`}><Archive size={14} /></button>
          </div>
        </div>
        {renderBranch(node.id, depth + 1)}
      </div>
    ));

  return (
    <section className="story-architecture">
      <header className="story-header">
        <div><p className="eyebrow">Story Engineering</p><h2>故事架构</h2><p>让章节、场景和故事线共享同一套可追踪结构。</p></div>
        <div className="story-actions">
          <button type="button" onClick={() => setDialog("outline")}><Plus size={16} />新建大纲节点</button>
          <button type="button" onClick={() => setDialog("line")}><Route size={16} />新建故事线</button>
          <button type="button" disabled={!chapters.length || !sceneNodes.length} onClick={() => setDialog("card")}><ScrollText size={16} />新建场景卡</button>
        </div>
      </header>
      {error && <div className="story-error" role="alert">{error}</div>}
      {loading ? <p className="story-loading">正在读取故事结构…</p> : (
        <div className="story-grid">
          <article className="story-panel story-panel--outline"><h3><ListTree size={18} />五级大纲</h3>{structure.outlineNodes.length ? renderBranch("") : <p className="story-empty">从全书大纲开始搭建故事。</p>}</article>
          <article className="story-panel"><h3><GitBranch size={18} />故事线</h3>{structure.storyLines.length ? structure.storyLines.map((line) => {
            const linked = new Set(structure.storyLineLinks.filter((link) => link.storyLineId === line.id).map((link) => link.outlineNodeId));
            const candidates = structure.outlineNodes.filter((node) => !linked.has(node.id));
            return <div className="story-line-card" key={line.id}><strong>{line.name}</strong><span>{line.status === "planned" ? "规划中" : line.status}</span><p>{line.summary || "还没有故事线说明"}</p><div className="story-line-nodes">{structure.storyLineLinks.filter((link) => link.storyLineId === line.id).map((link) => <span key={link.outlineNodeId}>{structure.outlineNodes.find((node) => node.id === link.outlineNodeId)?.title}</span>)}</div>{candidates.length > 0 && <div className="story-line-linker"><label><span className="sr-only">{line.name}关联节点</span><select aria-label={`${line.name}关联节点`} value={linkTargets[line.id] ?? ""} onChange={(event) => setLinkTargets((current) => ({ ...current, [line.id]: event.target.value }))}><option value="">选择轨迹节点</option>{candidates.map((node) => <option key={node.id} value={node.id}>{node.title}</option>)}</select></label><button type="button" aria-label={`关联${line.name}`} disabled={!linkTargets[line.id]} onClick={() => void linkStoryLine(line.id)}>关联</button></div>}</div>;
          }) : <p className="story-empty">故事线用于串联跨章节的发展轨迹。</p>}</article>
          <article className="story-panel"><h3><ScrollText size={18} />场景卡</h3>{structure.sceneCards.length ? structure.sceneCards.map((card) => <div className="scene-card" key={card.id}><strong>{card.title}</strong><span>{chapters.find((chapter) => chapter.id === card.chapterId)?.title}</span><p>{card.goal || "尚未设置场景目标"}</p></div>) : <p className="story-empty">为场景节点补充目标、冲突与结果。</p>}</article>
        </div>
      )}

      {dialog && <div className="dialog-backdrop"><form className="project-dialog story-dialog" role="dialog" aria-label={dialog === "outline" ? "新建大纲节点" : dialog === "line" ? "新建故事线" : "新建场景卡"} onSubmit={dialog === "outline" ? createOutline : dialog === "line" ? createLine : createCard}>
        <p className="eyebrow">结构化故事资产</p><h2>{dialog === "outline" ? `新建${levelNames[outlineLevel]}节点` : dialog === "line" ? "新建故事线" : "新建场景卡"}</h2>
        {dialog === "outline" && structure.outlineNodes.length > 0 && <label>父节点<select aria-label="父节点" required value={parentId} onChange={(event) => { setParentId(event.target.value); setChapterId(""); }}><option value="">请选择</option>{structure.outlineNodes.filter((node) => node.level !== "scene").map((node) => <option key={node.id} value={node.id}>{levelNames[node.level]} · {node.title}</option>)}</select></label>}
        {dialog === "card" && <><label>所属章节<select aria-label="所属章节" required value={chapterId} onChange={(event) => { setChapterId(event.target.value); setParentId(""); }}><option value="">请选择</option>{chapters.map((chapter) => <option key={chapter.id} value={chapter.id}>{chapter.title}</option>)}</select></label><label>场景节点<select aria-label="场景节点" required value={parentId} onChange={(event) => setParentId(event.target.value)}><option value="">请选择</option>{availableSceneNodes.map((node) => <option key={node.id} value={node.id}>{node.title}</option>)}</select></label></>}
        <label>{dialog === "line" ? "故事线名称" : dialog === "card" ? "场景标题" : "节点标题"}<input required aria-label={dialog === "line" ? "故事线名称" : dialog === "card" ? "场景标题" : "节点标题"} value={title} onChange={(event) => setTitle(event.target.value)} /></label>
        {dialog !== "card" && <label>说明<textarea value={summary} onChange={(event) => setSummary(event.target.value)} /></label>}
        {dialog === "outline" && outlineLevel === "chapter" && <label>关联章节<select aria-label="关联章节" value={chapterId} onChange={(event) => setChapterId(event.target.value)}><option value="">暂不关联</option>{chapters.map((chapter) => <option key={chapter.id} value={chapter.id}>{chapter.title}</option>)}</select></label>}
        {(dialog === "outline" || dialog === "card") && <><label>目标<input value={goal} onChange={(event) => setGoal(event.target.value)} /></label><label>冲突<input value={conflict} onChange={(event) => setConflict(event.target.value)} /></label><label>结果<input value={outcome} onChange={(event) => setOutcome(event.target.value)} /></label></>}
        {dialog === "card" && <><label>POV<input value={pov} onChange={(event) => setPOV(event.target.value)} /></label><label>地点<input value={location} onChange={(event) => setLocation(event.target.value)} /></label></>}
        <div className="dialog-actions"><button type="button" className="secondary-action" onClick={resetForm}>取消</button><button type="submit" className="primary-action">{dialog === "outline" ? "保存节点" : dialog === "line" ? "保存故事线" : "保存场景卡"}</button></div>
      </form></div>}
    </section>
  );
}
