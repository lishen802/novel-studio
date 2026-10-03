import { ChapterHistory } from '../components/ChapterHistory';
import { EditorSession } from '../editorSession';
import { CreationProgress } from '../components/creation/CreationProgress';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { clsx } from 'clsx';
import {
  BookOpen,
  Search,
  Plus,
  ChevronLeft,
  ChevronRight,
  Undo2,
  Redo2,
  Focus,
  History,
  Sparkles,
  Bot,
  Send,
  Check,
  Loader2,
  MessageSquareQuote,
  Pencil,
  RefreshCw,
  AlertCircle,
  Clock,
  Save,
  PanelRightClose,
  GripVertical,
  Type,
  Palette,
  AlignLeft,
} from 'lucide-react';
import { Button } from '../components/ui/Button';
import { Card } from '../components/ui/Card';
import { Input } from '../components/ui/Input';
import { Textarea } from '../components/ui/Input';
import { StatusBadge } from '../components/ui/StatusBadge';
import { EmptyState } from '../components/ui/EmptyState';
import { Modal } from '../components/ui/Modal';
import { useApp } from '../store/useApp';
import type { ChapterStatus } from '../types';
import { canvasPresets } from '../editor';
import { studio, type AgentRunView, type ChapterView, type CreativeProfileView, type JobSnapshotView } from '../api/studio';
import { formatBudget, reasoningLabel } from '../budget';

interface Chapter {
  id: string;
  title: string;
  order: number;
  wordCount: number;
  status: ChapterStatus;
}

interface Volume {
  id: string;
  title: string;
  chapters: Chapter[];
}

interface Candidate {
  id: string;
  type: 'explain' | 'polish' | 'rewrite' | 'review' | 'mark';
  label: string;
  original: string;
  result: string;
  applied: boolean;
  actionable: boolean;
}

const statusVariant: Record<ChapterStatus, 'default' | 'success' | 'warning' | 'danger' | 'info'> = {
  approved: 'success',
  review: 'warning',
  draft: 'default',
  empty: 'info',
};

const statusLabel: Record<ChapterStatus, string> = {
  approved: '已定稿',
  review: '待审阅',
  draft: '草稿',
  empty: '未开始',
};

function countWords(text: string) {
  return text.replace(/\s/g, '').length;
}

function stepStatusLabel(status: string) {
  switch (status) {
    case 'queued': return '等待中';
    case 'running': return '进行中';
    case 'completed': return '完成';
    case 'failed': return '失败';
    case 'cancelled': return '已取消';
    default: return '待确认';
  }
}

function roleLabel(role: string) {
  switch (role) {
    case 'planner': return '规划';
    case 'writer': return '写作';
    case 'continuity_reviewer': return '审阅';
    default: return '其他协作';
  }
}

export default function Chapters() {
  const navigate = useNavigate();
  const { state, dispatch, registerBeforeClose } = useApp();
  const settings = state.settings;
  // AI 功能仅在项目锁定创作方案后可用；方案锁定来自后端项目投影，不再依赖
  // 已废弃的"当前激活模型"概念。
  const profileConfigured = Boolean(state.backendProject?.profileSelection?.profileId);
  const canvasColor = useMemo(() => canvasPresets.find((p) => p.id === settings.canvasColorId) || canvasPresets[0], [settings.canvasColorId]);
  const [volumes, setVolumes] = useState<Volume[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [title, setTitle] = useState('');
  const [content, setContentState] = useState('');
  const historyRef = useRef({past: [] as string[], future: [] as string[]});
  const [findOpen, setFindOpen] = useState(false);
  const [findText, setFindText] = useState('');
  const [findMessage, setFindMessage] = useState('');
  const sessionRef = useRef<EditorSession | null>(null);
  const [saveError, setSaveError] = useState('');
  const [loadError,setLoadError] = useState(false);
  const [loadAttempt,setLoadAttempt] = useState(0);
  const [actionNotice,setActionNotice] = useState('');
  const [switching, setSwitching] = useState(false);
  const [committing, setCommitting] = useState(false);
  const readOnly = state.backendProject?.accessMode === 'read_only';
  const setContent = (body: string) => {
    if (sessionRef.current) {
      const history = historyRef.current;
      if (sessionRef.current.body !== body) { history.past.push(sessionRef.current.body); if (history.past.length > 100) history.past.shift(); history.future = []; }
      sessionRef.current.body = body;
    }
    setContentState(body);
    setSaveState('idle');
  };
  const historyAction = (redo = false) => {
    const session = sessionRef.current;
    if (!session || readOnly || hydrating) return;
    const history = historyRef.current;
    const from = redo ? history.future : history.past;
    const to = redo ? history.past : history.future;
    const body = from.pop();
    if (body === undefined) return;
    to.push(session.body); session.body = body; setContentState(body); setSaveState('idle');
  };
  const findNext = () => {
    if (!findText || !editorRef.current) return;
    const editor = editorRef.current;
    let index = content.indexOf(findText, editor.selectionEnd);
    if (index < 0) index = content.indexOf(findText);
    if (index < 0) { setFindMessage('未找到匹配内容'); return; }
    editor.focus(); editor.setSelectionRange(index,index + findText.length); setFindMessage(`已定位第 ${index + 1} 个字符`);
  };
  const flushDraft = useCallback(async () => {
    const session = sessionRef.current;
    if (!session) return;
    setSaveState(session.dirty ? 'saving' : 'saved');
    try {
      await session.flush();
      if (sessionRef.current === session) { setSaveState('saved'); setSaveError(''); }
    } catch (error) {
      if (sessionRef.current === session) {
        setSaveState('idle');
        setSaveError(error instanceof Error ? error.message : '自动保存失败，输入仍保留，请重试。');
      }
      throw error;
    }
  }, []);
  const selectChapter = async (id: string) => {
    if (switching) return;
    setSwitching(true);
    try { await flushDraft(); setSelectedId(id); if (window.matchMedia?.('(max-width: 1100px)').matches) setChapterTreeOpen(false); }
    catch { /* 保存失败时保留原章节和输入。 */ }
    finally { setSwitching(false); }
  };
  const [revision, setRevision] = useState(0);
  const [hydrating, setHydrating] = useState(true);
  const [saveState, setSaveState] = useState<'idle' | 'saving' | 'saved'>('saved');
  const [search, setSearch] = useState('');
  const [focusMode, setFocusMode] = useState(false);
  const [chapterTreeOpen, setChapterTreeOpen] = useState(() => !window.matchMedia?.('(max-width: 1100px)').matches);
  // 小窗口默认让出正文空间；跨越断点时重新调整目录，手动展开仍可选章。
  useEffect(() => {
    const query = window.matchMedia?.('(max-width: 1100px)');
    if (!query) return;
    const resize = () => setChapterTreeOpen(!query.matches);
    query.addEventListener('change', resize);
    return () => query.removeEventListener('change', resize);
  }, []);

  const [historyOpen,setHistoryOpen] = useState(false);
  const [aiOpen, setAiOpen] = useState(false);
  const [aiWidth, setAiWidth] = useState(320);
  const [aiInput, setAiInput] = useState('');
  const [candidates, setCandidates] = useState<Candidate[]>([]);
  const [aiLoading, setAiLoading] = useState(false);
  const [activeRun, setActiveRun] = useState<AgentRunView | null>(null);
  const [cancelBusy, setCancelBusy] = useState(false);
  const [runPollError, setRunPollError] = useState<string | null>(null);
  // 运行前确认：用户发起创作任务后先暂存 prompt，确认弹窗展示方案来源与预算后再真正创建 AgentRun。
  const [pendingRun, setPendingRun] = useState<{ prompt: string; label: string; retryId?: string } | null>(null);
  const pollRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // JobEvent 游标：按持久化序列号增量补拉，事件漏推或重复推送后仍能重建一致状态。
  const jobCursorRef = useRef(0);
  // 保存当前轮询的 tick 闭包，供 job:updated 事件立即触发增量刷新。
  const tickRef = useRef<(() => void) | null>(null);
  const [jobSnapshot, setJobSnapshot] = useState<JobSnapshotView | null>(null);

  const [settingsOpen, setSettingsOpen] = useState(false);

  const [selection, setSelection] = useState<{ text: string; x: number; y: number } | null>(null);
  const editorRef = useRef<HTMLTextAreaElement>(null);
  const dragRef = useRef<{ startX: number; startWidth: number } | null>(null);

  const chapters = useMemo(() => volumes.flatMap((v) => v.chapters), [volumes]);
  const selected = useMemo(() => chapters.find((c) => c.id === selectedId) || null, [chapters, selectedId]);
  const liveWordCount = countWords(content);

  // 运行前确认弹窗按项目锁定的 profileId + version 异步拉取准确版本快照，
  // 不再依赖 bootstrap 只返回的最新版本——否则项目锁定 v1、最新是 v2 时
  // 弹窗会显示 v2 的角色却标 v1、实际运行用 v1。
  const [runPreview, setRunPreview] = useState<{
    profileName: string;
    version: number;
    roleRows: Array<{ role: string; providerName: string; model: string; reasoningEffort: string; enabled: boolean }>;
    maxRunTokens: number;
    maxContextChars: number;
    runBudgetUnlimited: boolean;
    contextBudgetUnlimited: boolean;
  } | null>(null);
  const [runPreviewLoading, setRunPreviewLoading] = useState(false);
  const [runPreviewError, setRunPreviewError] = useState(false);
  const [runPreviewReloadKey, setRunPreviewReloadKey] = useState(0);

  useEffect(() => {
    if (!pendingRun) {
      setRunPreview(null);
      setRunPreviewError(false);
      setRunPreviewLoading(false);
      return;
    }
    const selection = state.backendProject?.profileSelection;
    if (!selection?.profileId || selection.version == null) {
      setRunPreview(null);
      setRunPreviewError(true);
      setRunPreviewLoading(false);
      return;
    }
    let active = true;
    setRunPreview(null);
    setRunPreviewError(false);
    setRunPreviewLoading(true);
    void studio.getProfileVersion(selection.profileId, selection.version).then((profile: CreativeProfileView) => {
      if (!active) return;
      const providers = state.bootstrap?.providers || [];
      const roleRows = profile.roles.map((role) => {
        const provider = providers.find((p) => p.id === role.connectionId);
        return {
          role: roleLabel(role.role),
          providerName: provider?.name || '未知连接',
          model: role.model,
          reasoningEffort: role.reasoningEffort || 'auto',
          enabled: role.enabled,
        };
      });
      setRunPreview({
        profileName: profile.name,
        version: profile.version,
        roleRows,
        maxRunTokens: profile.maxRunTokens,
        maxContextChars: profile.maxContextChars,
        runBudgetUnlimited: Boolean(profile.runBudgetUnlimited),
        contextBudgetUnlimited: Boolean(profile.contextBudgetUnlimited),
      });
    }).catch(() => {
      if (active) {
        setRunPreview(null);
        setRunPreviewError(true);
      }
    }).finally(() => {
      if (active) setRunPreviewLoading(false);
    });
    return () => { active = false; };
    // 依赖 pendingRun 和项目锁定方案：弹窗打开时拉取，关闭时清空。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pendingRun, runPreviewReloadKey, state.backendProject?.profileSelection]);

  useEffect(() => {
    let active = true;
    void studio.listChapters().then((items) => {
      if (!active) return;
      const chapters = items.map((chapter) => ({
        id: chapter.id,
        title: chapter.title,
        order: chapter.position,
        wordCount: countWords(chapter.body),
        status: chapter.body ? 'draft' as const : 'empty' as const,
      }));
      setVolumes([{ id: 'manuscript', title: '正文', chapters }]);
      setSelectedId((current) => current || chapters[0]?.id || null);
    }).catch((error) => setSaveError(error instanceof Error ? error.message : '无法读取章节'));
    return () => { active = false; };
  }, []);

  useEffect(() => {
    if (!selectedId) return;
    let active = true;
    setHydrating(true);setLoadError(false);
    void studio.openChapter(selectedId).then((chapter: ChapterView) => {
      if (!active) return;
      setTitle(chapter.title);
      const body = chapter.hasRecoveryDraft ? chapter.recoveryBody ?? '' : chapter.body;
      sessionRef.current = new EditorSession(chapter.id, body, chapter.revision, studio.saveDraft);
      setContentState(body);
      historyRef.current = {past:[],future:[]};
      setSaveError('');
      setRevision(chapter.revision);
      setSaveState('saved');
      setHydrating(false);
    }).catch((error) => {
      if (active) { setLoadError(true);setSaveError(error instanceof Error ? error.message : '正文读取失败'); setHydrating(true); }
    });
    return () => { active = false; };
  }, [selectedId,loadAttempt]);

  useEffect(() => {
    if (hydrating || readOnly || !sessionRef.current?.dirty) return;
    const timer = window.setTimeout(() => { void flushDraft().catch(() => undefined); }, 800);
    return () => window.clearTimeout(timer);
  }, [content, revision, hydrating, readOnly, flushDraft]);

  useEffect(() => {
    if (readOnly) return;
    return registerBeforeClose(flushDraft);
  }, [readOnly, registerBeforeClose, flushDraft]);

  // 组件卸载时停止运行状态轮询。
  useEffect(() => () => {
    if (pollRef.current) clearTimeout(pollRef.current);
  }, []);

  // 切换章节时清空上一个章节的运行视图，避免残留误显示。
  useEffect(() => {
    if (pollRef.current) clearTimeout(pollRef.current);
    pollRef.current = null;
    tickRef.current = null;
    jobCursorRef.current = 0;
    setActiveRun(null);
    setRunPollError(null);
    setJobSnapshot(null);
  }, [selected?.id]);

  // 重新进入页面时恢复当前章节未完成的创作任务，继续轮询直到终态。
  // 后端 recoverInterruptedJobs 会把中断的 queued/running 转为 recoverable，
  // 因此必须同时查找 recoverable，否则作者看不到需要恢复的任务。
  useEffect(() => {
    if (!selected) return;
    let cancelled = false;
    void studio.agentRuns().then((runs) => {
      if (cancelled) return;
      const active = runs.find((run) => (
        run.chapterId === selected.id &&
        (run.status === 'queued' || run.status === 'running' || run.status === 'recoverable')
      ));
      if (active) {
        setActiveRun(active);
        // recoverable 是终态，无需轮询；queued/running 才需要继续观察。
        if (active.status === 'queued' || active.status === 'running') {
          observeRun(active.id);
        }
      }
    }).catch(() => {
      // 恢复失败不阻塞编辑，作者可重新发起任务。
    });
    return () => { cancelled = true; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selected?.id]);

  // 监听后端 job:updated 事件：收到对应运行的信号时立即增量刷新，
  // 减少轮询延迟。事件可能丢失或合并，正确性由 cursor 补拉保证。
  useEffect(() => {
    if (!activeRun) return;
    const runtime = (window as Window & {
      runtime?: { EventsOn: (name: string, cb: (data: { jobId?: string }) => void) => () => void };
    }).runtime;
    if (!runtime) return;
    const off = runtime.EventsOn('job:updated', (payload) => {
      if (payload.jobId !== activeRun.id) return;
      // 立即触发增量刷新，不重置游标；tick 会自动恢复轮询。
      if (pollRef.current) clearTimeout(pollRef.current);
      pollRef.current = null;
      if (tickRef.current) void tickRef.current();
    });
    return off;
    // 只在 run id 变化时重新订阅，状态更新不触发重订阅。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeRun?.id]);

  const filteredVolumes = useMemo(() => {
    if (!search.trim()) return volumes;
    return volumes
      .map((v) => ({ ...v, chapters: v.chapters.filter((c) => c.title.includes(search)) }))
      .filter((v) => v.chapters.length > 0);
  }, [volumes, search]);

  const handleMouseUp = () => {
    const textarea = editorRef.current;
    if (!textarea) return;
    const start = textarea.selectionStart;
    const end = textarea.selectionEnd;
    if (start === end) {
      setSelection(null);
      return;
    }
    const text = content.slice(start, end);
    const rect = textarea.getBoundingClientRect();
    setSelection({
      text,
      x: rect.left + rect.width / 2,
      y: rect.top + 24,
    });
  };

  /** 增量拉取 JobEvent：按持久化 cursor 补拉，事件漏推或重复推送后仍能重建一致状态。 */
  const pullJobEvents = async (runId: string) => {
    try {
      const snapshot = await studio.getJobSnapshot(runId);
      setJobSnapshot(snapshot);
      // 排空事件流：循环拉取直到没有更多事件，只推进游标，事件 payload 不直接消费。
      let cursor = jobCursorRef.current;
      let hasMore = true;
      let guard = 0;
      while (hasMore && guard < 10) {
        const page = await studio.getJobEvents(runId, cursor, 100);
        guard++;
        // 游标早于最早可重放序列时，后端要求重新对齐，防止漏拉历史。
        // 必须在此 continue 重新请求，不能用 page.nextCursor——snapshotRequired
        // 时它是 0，否则游标被重置后下次仍从旧位置请求，永久循环要求快照。
        if (page.snapshotRequired) {
          cursor = page.earliestReplayableSequence || snapshot.earliestReplayableSequence;
          continue;
        }
        cursor = page.nextCursor;
        hasMore = page.hasMore;
      }
      jobCursorRef.current = cursor;
    } catch {
      // 快照/事件拉取失败不阻断状态轮询，下次 tick 会重试。
    }
  };

  /** 轮询运行状态直到终态，实时更新步骤与用量；同时补拉 JobEvent 增量。 */
  const observeRun = (runId: string) => {
    // 新运行从头开始补拉事件，重置游标与快照。
    jobCursorRef.current = 0;
    setJobSnapshot(null);
    const tick = async () => {
      try {
        const run = await studio.agentRun(runId);
        setActiveRun(run);
        setRunPollError(null);
        await pullJobEvents(runId);
        // recoverable 也是终态：运行被中断但可恢复，需要作者主动重试，不应继续轮询。
        if (
          run.status === 'completed' ||
          run.status === 'failed' ||
          run.status === 'cancelled' ||
          run.status === 'recoverable'
        ) return;
      } catch (error) {
        // 轮询异常不静默吞掉，展示给作者以便发现后端不可达或鉴权失败等问题。
        setRunPollError(error instanceof Error ? error.message : '无法获取运行状态');
        return;
      }
      pollRef.current = setTimeout(tick, 1200);
    };
    tickRef.current = tick;
    void tick();
  };

  /** 取消正在运行的创作任务；结果仍保留为候选/变更集，不覆盖正文。 */
  const cancelRun = async () => {
    if (!activeRun) return;
    setCancelBusy(true);
    try {
      await studio.cancelAgentRun(activeRun.id);
      setActiveRun(await studio.agentRun(activeRun.id));
    } finally {
      setCancelBusy(false);
    }
  };

  // 失败任务按当前方案重新生成，重新展示预算并征得本次调用确认。
  const retryRun = () => {
    if (!activeRun || !selected) return;
    setPendingRun({prompt: activeRun.prompt, label: '重新生成', retryId: activeRun.id});
  };

  const runAiAction = (label: string) => {
    setActionNotice(`${label}尚未接入独立的选区处理流程，当前不会发起整章生成。`);
    setAiOpen(true); setSelection(null);
  };

  const handleFloatingAction = (action: string) => {
    if (!selection) return;
    switch (action) {
      case 'explain':
        runAiAction('解释这段');
        break;
      case 'polish':
        runAiAction('润色');
        break;
      case 'rewrite':
        runAiAction('改写');
        break;
      case 'mark':
        runAiAction('标记问题');
        break;
      case 'review':
        runAiAction('交给 AI 审阅');
        break;
    }
  };

  const sendAiInstruction = async () => {
    if (!aiInput.trim() || !selected) return;
    if (!profileConfigured) {
      navigate('/settings');
      return;
    }
    // 真实调用前先经确认弹窗，避免误触产生费用。
    setPendingRun({ prompt: aiInput.trim(), label: 'AI 指令' });
  };

  /** 运行前确认通过后，真正创建并执行 AgentRun；结果仍进入候选区，不覆盖正文。 */
  const confirmRun = async () => {
    // 精确方案预览是付费调用前置条件；函数守卫和按钮禁用共同防止绕过确认。
    if (!pendingRun || !selected || !runPreview || runPreviewLoading) return;
    const { prompt, label, retryId } = pendingRun;
    setPendingRun(null);
    setAiLoading(true);
    try {
      await flushDraft();
      const run = retryId ? await studio.retryAgentRun(retryId) : await studio.createChapterAgentRun(selected.id, prompt);
      const executed = await studio.executeAgentRun(String(run.id));
      setActiveRun(executed);
      observeRun(String(executed.id));
      const candidate: Candidate = {
        id: `can-${Date.now()}`,
        type: 'review',
        label: label === 'AI 指令' ? 'AI 任务已启动' : label,
        original: '',
        result: '任务已在后台运行。完成后会作为可审阅变更集出现，不会直接覆盖正文。',
        applied: false,
        actionable: false,
      };
      setCandidates((prev) => [candidate, ...prev]);
      if (label === 'AI 指令') setAiInput('');
    } catch (reason) {
      setCandidates((prev) => [{
        id: `can-error-${Date.now()}`, type: 'review', label: '任务提交失败', original: '',
        result: reason instanceof Error ? reason.message : '无法提交创作任务', applied: false, actionable: false,
      }, ...prev]);
    } finally {
      setAiLoading(false);
    }
  };

  const applyCandidate = (id: string, partial?: string) => {
    const candidate = candidates.find((c) => c.id === id);
    if (!candidate) return;
    const insert = partial ?? candidate.result;
    if (candidate.original && content.includes(candidate.original)) {
      setContent(content.replace(candidate.original, insert));
    } else {
      setContent(content ? `${content}\n\n${insert}` : insert);
    }
    setCandidates((prev) => prev.map((c) => (c.id === id ? { ...c, applied: true } : c)));
  };

  const discardCandidate = (id: string) => {
    setCandidates((prev) => prev.filter((c) => c.id !== id));
  };

  /** 显式提交正文 revision；恢复草稿仍由自动保存维护。 */
  const commitCurrentChapter = async () => {
    if (!selected) return;
    const session = sessionRef.current;
    if (!session || committing || hydrating || readOnly) return;
    setSaveState('saving');
    try {
      setCommitting(true);
      const committed = await session.commit(studio.commitChapter);
      setRevision(committed.revision);
      setSaveError('');
      setSaveState('saved');
      setVolumes((previous) => previous.map((volume) => ({
        ...volume,
        chapters: volume.chapters.map((chapter) => chapter.id === selected.id
          ? { ...chapter, wordCount: countWords(committed.body), status: committed.body ? 'draft' : 'empty' }
          : chapter),
      })));
    } catch (error) {
      setSaveState('idle');
      setSaveError(error instanceof Error ? error.message : '保存版本失败');
    } finally { setCommitting(false); }
  };

  const createChapter = async () => {
    await flushDraft();
    const created = await studio.createChapter(`第 ${chapters.length + 1} 章　新章节`);
    const newChapter: Chapter = {
      id: created.id,
      title: created.title,
      order: created.position,
      wordCount: 0,
      status: 'empty',
    };
    setVolumes((previous) => previous.length
      ? previous.map((volume, index) => index === previous.length - 1
        ? { ...volume, chapters: [...volume.chapters, newChapter] }
        : volume)
      : [{ id: 'manuscript', title: '正文', chapters: [newChapter] }]);
    setSelectedId(created.id);
  };

  const startResize = (e: React.MouseEvent) => {
    dragRef.current = { startX: e.clientX, startWidth: aiWidth };
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
    const move = (ev: MouseEvent) => {
      if (!dragRef.current) return;
      const delta = dragRef.current.startX - ev.clientX;
      setAiWidth(Math.max(260, Math.min(520, dragRef.current.startWidth + delta)));
    };
    const up = () => {
      dragRef.current = null;
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
      window.removeEventListener('mousemove', move);
      window.removeEventListener('mouseup', up);
    };
    window.addEventListener('mousemove', move);
    window.addEventListener('mouseup', up);
  };

  const selectedIndex = selected ? chapters.findIndex((c) => c.id === selected.id) : -1;
  const canPrev = selectedIndex > 0;
  const canNext = selectedIndex >= 0 && selectedIndex < chapters.length - 1;

  const directoryButton = <Button variant="ghost" size="sm" aria-controls="chapter-directory" aria-expanded={chapterTreeOpen && !focusMode} onClick={() => {
    setChapterTreeOpen(focusMode || !chapterTreeOpen); setFocusMode(false);
  }} className="gap-1 shrink-0"><BookOpen className="w-4 h-4" />章节目录</Button>;

  return (
    <div data-editor-focus={focusMode} className="manuscript-workspace relative h-[calc(100%+3rem)] flex animate-fade-up -m-6">
      {/* Left chapter tree */}
      {chapterTreeOpen && !focusMode && <button aria-label="关闭章节目录浮层" className="chapter-dismiss" onClick={() => setChapterTreeOpen(false)} />}
      <aside id="chapter-directory" hidden={focusMode || !chapterTreeOpen} aria-label="章节目录" className={clsx("chapter-navigation w-[260px] shrink-0 flex-col border-r border-border bg-surface", !focusMode && chapterTreeOpen ? "flex" : "hidden")}>
        <div className="p-4 border-b border-border space-y-3">
          <div className="flex items-center justify-between">
            <h2 className="text-sm font-medium text-text">卷与章节</h2>
            <span className="text-xs text-text-muted">{chapters.length} 章</span>
          </div>
          <div className="relative">
            <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-text-muted" />
            <Input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="搜索章节…"
              className="pl-9 h-8 text-sm"
            />
          </div>
          <Button variant="default" size="sm" disabled={readOnly || hydrating || committing || switching} onClick={() => void createChapter().catch(error => setSaveError(error instanceof Error ? error.message : '创建章节失败'))} className="w-full gap-1">
            <Plus className="w-4 h-4" />
            新建章节
          </Button>
        </div>
        <div className="flex-1 overflow-auto p-3 pb-0 space-y-4">
          {filteredVolumes.map((volume) => (
            <div key={volume.id}>
              <h3 className="text-xs font-medium text-text-sub uppercase tracking-wider mb-2 px-1">{volume.title}</h3>
              <div className="space-y-1">
                {volume.chapters.map((chapter) => (
                  <button
                    key={chapter.id}
                    disabled={switching || committing}
                    onClick={() => void selectChapter(chapter.id)}
                    className={clsx(
                      'w-full text-left px-3 py-2 rounded-lg border transition-colors group',
                      selectedId === chapter.id
                        ? 'bg-primary-subtle border-primary/15'
                        : 'bg-surface-elevated border-border hover:border-border-strong'
                    )}
                  >
                    <div className="flex items-center justify-between gap-2 mb-1">
                      <span className={clsx('text-sm truncate', selectedId === chapter.id ? 'text-primary' : 'text-text')}>
                        {chapter.title}
                      </span>
                      <StatusBadge variant={statusVariant[chapter.status]} className="shrink-0 text-[10px]">
                        {statusLabel[chapter.status]}
                      </StatusBadge>
                    </div>
                    <div className="flex items-center justify-between text-xs text-text-muted">
                      <span>{chapter.wordCount.toLocaleString()} 字</span>
                      <span className="opacity-0 group-hover:opacity-100 transition-opacity">第 {chapter.order} 章</span>
                    </div>
                  </button>
                ))}
              </div>
            </div>
          ))}
        </div>
      </aside>

      {/* Middle editor */}
      <main
        className={clsx('flex-1 flex flex-col min-w-0 relative')}
        style={{ backgroundColor: canvasColor.bg, color: canvasColor.text }}
      >
        {!selected ? (
          <div className="flex-1 flex flex-col gap-4 items-center justify-center p-6">
            {directoryButton}
            <EmptyState
              icon={BookOpen}
              title="选择章节开始写作"
              description="从左侧章节树中选择一章，或新建章节进入沉浸式写作。"
            />
          </div>
        ) : (
          <>
            {/* Model warning */}
            {!profileConfigured && (
              <div
                data-testid="model-configuration-notice"
                className="px-5 py-2 bg-model-notice border-b border-model-notice-border text-text flex items-center gap-3"
              >
                <AlertCircle className="w-4 h-4 text-warning shrink-0" />
                <p className="text-sm text-text-sub flex-1">本项目尚未配置创作方案，请先在应用设置中发布并设为默认方案。</p>
                <Button size="sm" onClick={() => navigate('/settings')}>前往配置</Button>
              </div>
            )}

            {/* 正文工具栏 */}
            <div className="editor-toolbar flex flex-wrap items-center justify-between gap-3 px-5 py-3 border-b border-border bg-surface/80 backdrop-blur-sm sticky top-0 z-10">
              <div className="flex flex-wrap items-center gap-2">
                {directoryButton}
                <Button
                  variant="ghost"
                  size="sm"
                  disabled={!canPrev || switching || committing}
                  onClick={() => void selectChapter(chapters[selectedIndex - 1].id)}
                  className="px-2"
                >
                  <ChevronLeft className="w-4 h-4" />
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  disabled={!canNext || switching || committing}
                  onClick={() => void selectChapter(chapters[selectedIndex + 1].id)}
                  className="px-2"
                >
                  <ChevronRight className="w-4 h-4" />
                </Button>
                <div className="h-4 w-px bg-border mx-1" />
                <Button variant="ghost" size="sm" className="px-2" title="撤销" disabled={readOnly || hydrating || !historyRef.current.past.length} onClick={() => historyAction()}>
                  <Undo2 className="w-4 h-4" />
                </Button>
                <Button variant="ghost" size="sm" className="px-2" title="重做" disabled={readOnly || hydrating || !historyRef.current.future.length} onClick={() => historyAction(true)}>
                  <Redo2 className="w-4 h-4" />
                </Button>
                <Button variant="ghost" size="sm" className="px-2" title="查找" onClick={() => setFindOpen(open => !open)}>
                  <Search className="w-4 h-4" />
                </Button>
                <Button
                  variant={focusMode ? 'primary' : 'ghost'}
                  size="sm"
                  onClick={() => setFocusMode((v) => !v)}
                  className="gap-1 px-2"
                >
                  <Focus className="w-4 h-4" />
                  专注
                </Button>
                <Button variant="ghost" size="sm" onClick={() => setHistoryOpen(true)} className="gap-1 px-2">
                  <History className="w-4 h-4" />
                  版本
                </Button>
                <div className="relative">
                  <Button
                    variant={settingsOpen ? 'primary' : 'ghost'}
                    size="sm"
                    onClick={() => setSettingsOpen((v) => !v)}
                    className="gap-1 px-2"
                  >
                    <Type className="w-4 h-4" />
                    排版
                  </Button>

                  {settingsOpen && (
                    <div className="absolute right-0 top-full mt-2 w-72 bg-surface border border-border rounded-xl shadow-lg p-4 z-50 animate-fade-up">
                      {/* Background color */}
                      <div className="mb-5">
                        <div className="flex items-center gap-2 mb-3">
                          <Palette className="w-3.5 h-3.5 text-text-sub" />
                          <span className="text-xs font-medium text-text-sub uppercase tracking-wider">画布背景</span>
                        </div>
                        <div className="grid grid-cols-4 gap-2">
                          {canvasPresets.map((preset) => (
                            <button
                              key={preset.id}
                              onClick={() => dispatch({ type: 'SET_EDITOR_SETTINGS', payload: { canvasColorId: preset.id } })}
                              className={clsx(
                                'h-10 rounded-lg border-2 transition-all flex items-center justify-center',
                                canvasColor.id === preset.id
                                  ? 'border-primary scale-105'
                                  : 'border-transparent hover:border-border-strong'
                              )}
                              style={{ backgroundColor: preset.bg }}
                              title={preset.label}
                            />
                          ))}
                        </div>
                        <div className="flex justify-between text-xs text-text-muted mt-2 px-1">
                          {canvasPresets.map((p) => (
                            <span key={p.id}>{p.label}</span>
                          ))}
                        </div>
                      </div>

                      {/* Font size */}
                      <div className="mb-5">
                        <div className="flex items-center justify-between mb-2">
                          <span className="text-xs font-medium text-text-sub uppercase tracking-wider">字号</span>
                          <span className="text-xs text-text">{settings.fontSize}px</span>
                        </div>
                        <input
                          type="range"
                          min={14}
                          max={24}
                          step={1}
                          value={settings.fontSize}
                          onChange={(e) => dispatch({ type: 'SET_EDITOR_SETTINGS', payload: { fontSize: Number(e.target.value) } })}
                          className="w-full accent-primary"
                        />
                        <div className="flex justify-between text-xs text-text-muted mt-1">
                          <span>A</span>
                          <span className="text-base">A</span>
                        </div>
                      </div>

                      {/* Line height */}
                      <div className="mb-5">
                        <div className="flex items-center justify-between mb-2">
                          <span className="text-xs font-medium text-text-sub uppercase tracking-wider">行距</span>
                          <span className="text-xs text-text">{settings.lineHeight.toFixed(1)}</span>
                        </div>
                        <input
                          type="range"
                          min={1.4}
                          max={2.6}
                          step={0.1}
                          value={settings.lineHeight}
                          onChange={(e) => dispatch({ type: 'SET_EDITOR_SETTINGS', payload: { lineHeight: Number(e.target.value) } })}
                          className="w-full accent-primary"
                        />
                        <div className="flex justify-between text-xs text-text-muted mt-1">
                          <span>紧</span>
                          <span>松</span>
                        </div>
                      </div>

                      {/* Margin */}
                      <div className="mb-5">
                        <div className="flex items-center gap-2 mb-3">
                          <AlignLeft className="w-3.5 h-3.5 text-text-sub" />
                          <span className="text-xs font-medium text-text-sub uppercase tracking-wider">边距</span>
                        </div>
                        <div className="flex rounded-lg border border-border overflow-hidden">
                          {(['narrow', 'medium', 'wide'] as const).map((m) => (
                            <button
                              key={m}
                              onClick={() => dispatch({ type: 'SET_EDITOR_SETTINGS', payload: { marginSize: m } })}
                              className={clsx(
                                'flex-1 py-1.5 text-xs transition-colors',
                                settings.marginSize === m ? 'bg-primary text-white' : 'text-text-sub hover:bg-surface-hover'
                              )}
                            >
                              {m === 'narrow' ? '窄' : m === 'medium' ? '中' : '宽'}
                            </button>
                          ))}
                        </div>
                      </div>

                      {/* Indent */}
                      <div className="flex items-center justify-between">
                        <span className="text-xs font-medium text-text-sub uppercase tracking-wider">首行缩进</span>
                        <button
                          onClick={() => dispatch({ type: 'SET_EDITOR_SETTINGS', payload: { indent: !settings.indent } })}
                          className={clsx(
                            'w-9 h-5 rounded-full transition-colors relative',
                            settings.indent ? 'bg-primary' : 'bg-surface-hover'
                          )}
                        >
                          <span
                            className={clsx(
                              'absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow-sm transition-transform',
                              settings.indent && 'translate-x-4'
                            )}
                          />
                        </button>
                      </div>
                    </div>
                  )}
                </div>
              </div>

              <div className="flex items-center gap-3 text-xs text-text-sub">
                <Button variant="default" size="sm" disabled={hydrating || readOnly || committing} onClick={() => void commitCurrentChapter()}>
                  <Save className="w-3.5 h-3.5" />
                  保存版本
                </Button>
                <span className="flex items-center gap-1.5">
                  {saveState === 'saving' ? (
                    <Loader2 className="w-3.5 h-3.5 animate-spin" />
                  ) : (
                    <Save className="w-3.5 h-3.5" />
                  )}
                  {loadError ? '读取失败' : saveError ? '保存失败' : saveState === 'saved' ? '工作稿已自动保存' : saveState === 'saving' ? '保存中…' : '有未保存修改'}
                </span>
                <span className="flex items-center gap-1.5">
                  <Clock className="w-3.5 h-3.5" />
                  正文修订 {revision}
                </span>
                <span className="font-medium text-text">{liveWordCount.toLocaleString()} 字</span>
              </div>
            </div>

            {saveError && <div role="alert" className="p-3 text-sm text-danger">{saveError} <Button size="sm" onClick={() => loadError ? setLoadAttempt(value => value+1) : void flushDraft().catch(() => undefined)}>{loadError ? '重新读取' : '重试保存'}</Button></div>}
            {/* Editor */}
            <div className="flex-1 overflow-auto">
              <div
                className={clsx(
                  'w-full h-full transition-all duration-200',
                  focusMode ? 'py-12' : 'py-8',
                  settings.marginSize === 'narrow' && 'px-6',
                  settings.marginSize === 'medium' && 'px-12',
                  settings.marginSize === 'wide' && 'px-24',
                )}
              >
                <Input
                  value={title}
                  readOnly
                  title="当前后端尚未提供章节重命名命令"
                  className="text-2xl font-display font-semibold bg-transparent border-0 shadow-none px-0 mb-6 focus:ring-0 placeholder:text-text-muted"
                  style={{ color: canvasColor.text }}
                  placeholder="章节标题"
                />
                <Textarea
                  ref={editorRef}
                  onKeyDown={event => {
                    if (!(event.metaKey || event.ctrlKey)) return;
                    if (event.key.toLowerCase() === 'z') { event.preventDefault(); historyAction(event.shiftKey); }
                    if (event.key.toLowerCase() === 'y') { event.preventDefault(); historyAction(true); }
                    if (event.key.toLowerCase() === 'f') { event.preventDefault(); setFindOpen(true); }
                  }}
                  disabled={hydrating || readOnly || switching || committing}
                  value={content}
                  onChange={(e) => setContent(e.target.value)}
                  onMouseUp={handleMouseUp}
                  onKeyUp={handleMouseUp}
                  placeholder="从这里开始书写……"
                  className={clsx(
                    'w-full min-h-[70vh] prose-literary bg-transparent border-0 shadow-none focus:ring-0 resize-none placeholder:text-text-muted',
                  )}
                  style={{
                    color: canvasColor.text,
                    fontSize: `${settings.fontSize}px`,
                    lineHeight: settings.lineHeight,
                    textIndent: settings.indent ? '2em' : '0',
                  }}
                />
              </div>
            </div>
          </>
        )}
      </main>

      {findOpen && <div className="absolute top-16 right-8 z-30 bg-surface border border-border rounded-lg p-3 shadow-lg flex flex-wrap gap-2 items-center" role="search">
        <Input autoFocus aria-label="查找正文" value={findText} onChange={e => {setFindText(e.target.value);setFindMessage('');}} onKeyDown={e => {if(e.key === 'Enter')findNext();if(e.key === 'Escape')setFindOpen(false);}} />
        <Button onClick={findNext} disabled={!findText}>下一处</Button><Button onClick={() => setFindOpen(false)}>关闭查找</Button><span role="status" className="text-xs">{findMessage}</span>
      </div>}
      {/* AI entry / panel */}
      {!aiOpen && selected && (
        <button
          onClick={() => {setFocusMode(false);setAiOpen(true);}}
          className="absolute right-6 top-24 z-20 flex items-center gap-2 px-3 py-2 bg-surface border border-border rounded-lg shadow-sm text-sm text-text-sub hover:text-text hover:border-border-strong transition-colors"
        >
          <Sparkles className="w-4 h-4" />
          唤起创作助手
          <kbd className="px-1.5 py-0.5 bg-surface-hover rounded text-xs">⌘J</kbd>
        </button>
      )}

      {aiOpen && !focusMode && <button aria-label="关闭创作助手浮层" className="assistant-dismiss" onClick={() => setAiOpen(false)} />}
      {aiOpen && !focusMode && (
        <aside
          className="manuscript-assistant shrink-0 border-l border-border bg-surface flex flex-col relative"
          style={{ width: aiWidth }}
        >
          <div
            className="absolute left-0 top-0 bottom-0 w-1 cursor-col-resize hover:bg-primary/20 z-10"
            onMouseDown={startResize}
            title="拖动调整宽度"
          >
            <div className="absolute left-0 top-1/2 -translate-y-1/2 -translate-x-1/2 text-text-muted">
              <GripVertical className="w-3 h-3" />
            </div>
          </div>

          <div className="h-12 border-b border-border flex items-center justify-between px-4 pl-5 shrink-0">
            <div className="flex items-center gap-2">
              <Bot className="w-4 h-4 text-primary" />
              <span className="text-sm font-medium text-text">创作助手</span>
            </div>
            <Button variant="ghost" size="sm" onClick={() => setAiOpen(false)} aria-label="关闭创作助手" className="px-2">
              <PanelRightClose className="w-4 h-4" />
            </Button>
          </div>

          <div className="flex-1 overflow-auto p-4 space-y-4">
            {actionNotice && <p role="status" className="text-sm text-warning">{actionNotice}</p>}
            <div className="p-3 bg-primary-subtle border border-primary/10 rounded-lg text-xs text-text-sub leading-relaxed">
              根据指令和本章保存版本生成整章候选，需审阅后采用。选区解释、润色、改写暂未就绪。
            </div>

            <div className="space-y-3">
              <label className="text-xs font-medium text-text-sub">指令</label>
              <Textarea
                value={aiInput}
                onChange={(e) => setAiInput(e.target.value)}
                placeholder="例如：让沈知秋在栈桥上发现一个隐藏记号…"
                className="min-h-[80px] text-sm"
              />
              <Button
                variant="primary"
                size="sm"
                  onClick={() => void sendAiInstruction()}
                disabled={aiLoading || !aiInput.trim()}
                className="w-full gap-1"
              >
                {aiLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Send className="w-4 h-4" />}
                发送给 AI
              </Button>
            </div>

            {activeRun && (
              <div className="space-y-3">
                <CreationProgress status={activeRun.status} steps={activeRun.steps.map(step => ({stage:step.role,status:step.status}))} createdAt={activeRun.createdAt} updatedAt={activeRun.updatedAt} onCancel={cancelRun} cancelDisabled={cancelBusy} cancelLabel="取消" />
                {(activeRun.status === 'recoverable' || activeRun.status === 'failed') && <Button variant="ghost" size="sm" onClick={() => void retryRun()} disabled={aiLoading || Boolean(pendingRun)}>重新生成</Button>}
                <div className="space-y-2">
                  <details className="rounded-lg border border-border p-3"><summary className="cursor-pointer text-xs text-text-sub">模型与用量详情</summary>
                  <div className="text-xs text-text-sub">
                    预算 {activeRun.tokenBudget} · 已用 {activeRun.inputTokens + activeRun.outputTokens} tokens
                  </div>
                  <div className="space-y-1">
                    {activeRun.steps.map((step) => (
                      <div key={step.id} className="flex flex-wrap items-center gap-x-2 gap-y-0.5 text-xs">
                        <span className="text-text-muted">{roleLabel(step.role)}</span>
                        <span className="text-text-sub">{stepStatusLabel(step.status)}</span>
                        {step.model && (
                          <span className="text-text-muted">· {step.providerName || step.providerKind} / {step.model}</span>
                        )}
                        {(step.inputTokens + step.outputTokens) > 0 && (
                          <span className="text-text-muted">· {step.inputTokens + step.outputTokens} tokens</span>
                        )}
                      </div>
                    ))}
                  </div>
                  </details>
                  {jobSnapshot?.pendingAction && (
                    <p className="text-xs text-text-muted">正在处理你的操作，请稍候。</p>
                  )}
                  {runPollError && <p className="text-xs text-danger">暂时无法更新进度：{runPollError}</p>}
                  {activeRun.errorMessage && <p className="text-xs text-danger">{activeRun.errorMessage}</p>}
                  {activeRun.status === 'recoverable' && (
                    <p className="text-xs text-warning">运行被中断，可重新发起以继续。</p>
                  )}
                  {activeRun.status === 'completed' && activeRun.changeSetId && (
                    <p className="text-xs text-success">已生成可审阅变更集，请在“审阅与候选”中查看。</p>
                  )}
                </div>
              </div>
            )}

            {candidates.length > 0 && (
              <div className="space-y-3">
                <h4 className="text-xs font-medium text-text-sub uppercase tracking-wider">候选片段</h4>
                {candidates.map((c) => (
                  <Card key={c.id} padding="md" className={clsx('border', c.applied && 'border-success/50 bg-success/5')}>
                    <div className="flex items-center gap-2 mb-2">
                      <span className="text-xs font-medium text-primary bg-primary-subtle px-1.5 py-0.5 rounded">{c.label}</span>
                      {c.applied && <span className="text-xs text-success">已应用</span>}
                    </div>
                    {c.original && (
                      <div className="mb-2">
                        <p className="text-[11px] text-text-muted mb-1">原文</p>
                        <p className="text-xs text-text-sub line-through bg-surface-hover p-2 rounded">{c.original}</p>
                      </div>
                    )}
                    <div className="mb-3">
                      <p className="text-[11px] text-text-muted mb-1">候选</p>
                      <p className="text-sm text-text leading-relaxed whitespace-pre-wrap bg-surface-hover p-2 rounded">{c.result}</p>
                    </div>
                    {!c.applied && c.actionable && (
                      <div className="flex items-center gap-2">
                        <Button variant="primary" size="sm" onClick={() => applyCandidate(c.id)} className="gap-1">
                          <Check className="w-3.5 h-3.5" />
                          应用
                        </Button>
                        <Button variant="ghost" size="sm" onClick={() => applyCandidate(c.id, `[部分采纳]\n${c.result}`)}>
                          部分采纳
                        </Button>
                        <Button variant="ghost" size="sm" onClick={() => discardCandidate(c.id)}>
                          放弃
                        </Button>
                      </div>
                    )}
                  </Card>
                ))}
              </div>
            )}
          </div>
        </aside>
      )}

      {/* Floating toolbar on selection */}
      {selection && (
        <div
          className="fixed z-50 flex items-center gap-1 bg-surface border border-border rounded-lg shadow-lg p-1 animate-fade-up"
          style={{ left: selection.x, top: selection.y, transform: 'translate(-50%, -100%)' }}
        >
          <ToolbarButton icon={MessageSquareQuote} label="解释" onClick={() => handleFloatingAction('explain')} />
          <ToolbarButton icon={Pencil} label="润色" onClick={() => handleFloatingAction('polish')} />
          <ToolbarButton icon={RefreshCw} label="改写" onClick={() => handleFloatingAction('rewrite')} />
          <ToolbarButton icon={AlertCircle} label="标记" onClick={() => handleFloatingAction('mark')} />
          <ToolbarButton icon={Bot} label="审阅" onClick={() => handleFloatingAction('review')} />
        </div>
      )}

      {selected && <ChapterHistory chapterId={selected.id} open={historyOpen} readOnly={readOnly} onClose={() => setHistoryOpen(false)} onRestore={async target => {
        const session = sessionRef.current;
        if (!session || committing) return;
        setCommitting(true);
        try {
          await session.commit(studio.commitChapter);
          setRevision(session.revision);
          const restored = await studio.restoreChapterRevision(session.id,target,session.revision);
          sessionRef.current = new EditorSession(restored.id,restored.body,restored.revision,studio.saveDraft);
          setContentState(restored.body);setRevision(restored.revision);setSaveError('');setSaveState('saved');historyRef.current={past:[],future:[]};
        } finally {setCommitting(false);}
      }} />}
      {/* 运行前确认弹窗：展示方案角色来源、模型与预算，作者确认后才真实调用模型。 */}
      <Modal
        open={!!pendingRun}
        onClose={() => setPendingRun(null)}
        title="确认运行创作任务"
        className="max-w-xl"
      >
        <div className="space-y-4">
          <div className="flex items-start gap-2 p-3 bg-warning/10 border border-warning/20 rounded-md text-sm">
            <AlertCircle className="w-4 h-4 text-warning shrink-0 mt-0.5" />
            <span className="text-text-sub">将基于本章已保存版本生成整章候选，请先点击「保存版本」保留最新工作稿。会调用真实模型并产生费用，结果需另行审阅采用。</span>
          </div>
          {pendingRun?.retryId && <p className="text-sm text-warning">重新生成会使用当前方案重新执行规划、写作和审阅步骤，并重新计费。</p>}
          {runPreviewLoading ? (
            <p className="text-sm text-text-sub">正在加载方案预览…</p>
          ) : runPreview ? (
            <>
              <div className="text-sm text-text-sub">
                方案：<span className="text-text">{runPreview.profileName}</span> · v{runPreview.version}
              </div>
              <div className="border border-border rounded-lg overflow-hidden">
                <table className="w-full text-sm">
                  <thead className="bg-surface-hover text-text-sub">
                    <tr>
                      <th className="text-left px-3 py-2 font-medium">角色</th>
                      <th className="text-left px-3 py-2 font-medium">Provider</th>
                      <th className="text-left px-3 py-2 font-medium">模型</th>
                      <th className="text-left px-3 py-2 font-medium">推理强度</th>
                    </tr>
                  </thead>
                  <tbody>
                    {runPreview.roleRows.map((row, i) => (
                      <tr key={i} className="border-t border-border">
                        <td className="px-3 py-2 text-text">{row.role}</td>
                        <td className="px-3 py-2 text-text-sub">{row.providerName}</td>
                        <td className="px-3 py-2 text-text-sub">{row.model || '—'}</td>
                        <td className="px-3 py-2 text-text-sub">{reasoningLabel(row.reasoningEffort)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <div className="text-xs text-text-muted space-y-1">
                <div>运行 Token 预算：{formatBudget(runPreview.maxRunTokens, runPreview.runBudgetUnlimited)}</div>
                <div>上下文字符预算：{formatBudget(runPreview.maxContextChars, runPreview.contextBudgetUnlimited)}</div>
                {runPreview.runBudgetUnlimited && <div>不限制预算时，应用无法封顶本次真实调用费用。</div>}
              </div>
            </>
          ) : runPreviewError ? (
            <div className="flex items-center justify-between gap-3 p-3 border border-danger/20 bg-danger/5 rounded-md">
              <p className="text-sm text-text-sub">无法加载方案预览，已阻止真实模型调用。请重试或检查模型设置。</p>
              <Button variant="default" size="sm" onClick={() => setRunPreviewReloadKey((key) => key + 1)}>
                <RefreshCw className="w-4 h-4" />
                重试加载
              </Button>
            </div>
          ) : (
            <p className="text-sm text-text-sub">尚未取得方案预览，暂不能运行。</p>
          )}
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="ghost" size="sm" onClick={() => setPendingRun(null)}>取消</Button>
            <Button
              variant="primary"
              size="sm"
              onClick={() => void confirmRun()}
              disabled={aiLoading || runPreviewLoading || !runPreview}
            >
              {aiLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : null}
              确认运行
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}

function ToolbarButton({
  icon: Icon,
  label,
  onClick,
}: {
  icon: typeof MessageSquareQuote;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-md text-xs text-text-sub hover:text-text hover:bg-surface-hover transition-colors"
    >
      <Icon className="w-3.5 h-3.5" />
      {label}
    </button>
  );
}
