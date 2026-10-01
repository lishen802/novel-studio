import { useEffect, useMemo, useRef, useState } from 'react';
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
import { useApp } from '../store/useApp';
import type { ChapterStatus } from '../types';
import { getActiveModel } from '../types';
import { canvasPresets } from '../editor';

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
}

const initialVolumes: Volume[] = [
  {
    id: 'v1',
    title: '第一卷：雾起',
    chapters: [
      { id: 'c1', title: '第一章　未寄出的信', order: 1, wordCount: 3240, status: 'approved' },
      { id: 'c2', title: '第二章　江心岛', order: 2, wordCount: 4100, status: 'review' },
      { id: 'c3', title: '第三章　灯塔不亮', order: 3, wordCount: 2850, status: 'draft' },
    ],
  },
  {
    id: 'v2',
    title: '第二卷：灯芯',
    chapters: [
      { id: 'c4', title: '第四章　十七封信', order: 4, wordCount: 0, status: 'empty' },
      { id: 'c5', title: '第五章　雾中人影', order: 5, wordCount: 1200, status: 'draft' },
      { id: 'c6', title: '第六章　海图红圈', order: 6, wordCount: 0, status: 'empty' },
    ],
  },
];

const initialContent = `沈知秋抵达江心岛时，雾正从水面升起。

码头上只剩一盏马灯，被风推着晃。船夫不肯靠岸，只把行李扔在栈桥尽头，低声说：“灯塔上的灯，昨夜没亮。”

他抬眼望去，灰色的塔身隐没在雾中，像一只沉默的巨兽。三日前收到的那封信还在内袋里，字迹潦草，末尾只有一句：“请来接替我。”

栈桥的木板在脚下发出空洞的回响。沈知秋走得很慢，每一步都在确认这地方是否还属于人间。雾气带着水草与铁锈混合的气息，黏在衣领和袖口。他忽然停住——栈桥尽头，有人跪在那里。

是个老人，背对着他，肩膀一耸一耸，像是在哭，又像是在笑。

“请问……”

老人没有回头。沈知秋走近两步，才发现那人面前摆着三盏熄灭的油灯，灯芯已经被剪去了一半。`;

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

export default function Chapters() {
  const navigate = useNavigate();
  const { state, dispatch } = useApp();
  const settings = state.settings;
  const canvasColor = useMemo(() => canvasPresets.find((p) => p.id === settings.canvasColorId) || canvasPresets[0], [settings.canvasColorId]);
  const [volumes, setVolumes] = useState<Volume[]>(initialVolumes);
  const [selectedId, setSelectedId] = useState<string | null>('c3');
  const [title, setTitle] = useState('第三章　灯塔不亮');
  const [content, setContentState] = useState(initialContent);
  const historyRef = useRef({past:[] as string[],future:[] as string[]});
  const [findOpen,setFindOpen] = useState(false);
  const [findText,setFindText] = useState('');
  const [findMessage,setFindMessage] = useState('');
  const setContent = (body:string) => {
    if(body!==content){historyRef.current.past.push(content);historyRef.current.future=[];}
    setContentState(body);
  };
  const historyAction = (redo=false) => {
    const history=historyRef.current;const from=redo?history.future:history.past;const to=redo?history.past:history.future;
    const body=from.pop();if(body===undefined)return;to.push(content);setContentState(body);
  };
  const findNext = () => {
    if(!findText||!editorRef.current)return;
    const editor=editorRef.current;let index=content.indexOf(findText,editor.selectionEnd);if(index<0)index=content.indexOf(findText);
    if(index<0){setFindMessage('未找到匹配内容');return;}
    editor.focus();editor.setSelectionRange(index,index+findText.length);setFindMessage(`已定位第 ${index+1} 个字符`);
  };
  const [saveState, setSaveState] = useState<'idle' | 'saving' | 'saved'>('saved');
  const [search, setSearch] = useState('');
  const [focusMode, setFocusMode] = useState(false);
  const [aiOpen, setAiOpen] = useState(false);
  const [aiWidth, setAiWidth] = useState(320);
  const [aiInput, setAiInput] = useState('');
  const [candidates, setCandidates] = useState<Candidate[]>([]);
  const [aiLoading, setAiLoading] = useState(false);

  const [settingsOpen, setSettingsOpen] = useState(false);

  const [selection, setSelection] = useState<{ text: string; x: number; y: number } | null>(null);
  const editorRef = useRef<HTMLTextAreaElement>(null);
  const dragRef = useRef<{ startX: number; startWidth: number } | null>(null);

  const chapters = useMemo(() => volumes.flatMap((v) => v.chapters), [volumes]);
  const selected = useMemo(() => chapters.find((c) => c.id === selectedId) || null, [chapters, selectedId]);
  const liveWordCount = countWords(content);

  useEffect(() => {
    if (!selected) return;
    setTitle(selected.title);
    setContentState(selected.id === 'c3' ? initialContent : '');
    historyRef.current={past:[],future:[]};
    setSaveState('saved');
  }, [selected]);

  useEffect(() => {
    if (!selected) return;
    const timer = window.setTimeout(() => {
      setSaveState('saving');
      window.setTimeout(() => setSaveState('saved'), 600);
    }, 800);
    return () => window.clearTimeout(timer);
  }, [content, title, selected]);

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

  const runAiAction = (type: Candidate['type'], label: string, result: string) => {
    if (!selection) return;
    const candidate: Candidate = {
      id: `can-${Date.now()}`,
      type,
      label,
      original: selection.text,
      result,
      applied: false,
    };
    setCandidates((prev) => [candidate, ...prev]);
    setAiOpen(true);
    setSelection(null);
    editorRef.current?.focus();
  };

  const handleFloatingAction = (action: string) => {
    if (!selection) return;
    switch (action) {
      case 'explain':
        runAiAction('explain', '解释这段', '这段通过栈桥、马灯、雾等意象，建立沈知秋登岛时的不安氛围，同时为后续“灯塔不亮”的核心悬念做铺垫。老人与三盏熄灯的细节暗示岛上已有不寻常之事发生。');
        break;
      case 'polish':
        runAiAction('polish', '润色', '沈知秋踏上江心岛时，雾正贴着水面漫上来。码头上只剩一盏马灯，被风吹得晃个不停。船夫不肯靠岸，只把行李往栈桥尽头一扔，低声道：“灯塔上的灯，昨夜没亮。”');
        break;
      case 'rewrite':
        runAiAction('rewrite', '改写', '雾气从江心漫向岸边，像一层湿透的棉絮。沈知秋站在船头，看着码头上那盏孤零零的马灯。船夫说：“灯塔昨夜没亮。”他没有抬头，只把行李推过甲板。');
        break;
      case 'mark':
        runAiAction('mark', '标记问题', '【氛围与节奏】“像一只沉默的巨兽”此处比喻略显常见，建议换用更克制的意象，或改为角色主观感受。');
        break;
      case 'review':
        runAiAction('review', '交给 AI 审阅', '【审阅意见】开篇信息密度适中，但“三日前收到的那封信”可提前点出具体委托内容，增强主角动机。建议保留雾、灯、老人的视觉层次，它们有效构建了悬疑氛围。');
        break;
    }
  };

  const sendAiInstruction = () => {
    if (!aiInput.trim()) return;
    setAiLoading(true);
    window.setTimeout(() => {
      const candidate: Candidate = {
        id: `can-${Date.now()}`,
        type: 'rewrite',
        label: '按指令生成',
        original: '',
        result: `【按指令：${aiInput}】\n\n沈知秋没有立刻回答。他蹲下身，用手指抹了抹灯盏边缘的油渍——是新的。有人在他到来之前刚刚熄灭这些灯，也许就是为了让他看见这一幕。`,
        applied: false,
      };
      setCandidates((prev) => [candidate, ...prev]);
      setAiInput('');
      setAiLoading(false);
    }, 900);
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

  const createChapter = () => {
    const newChapter: Chapter = {
      id: `c-${Date.now()}`,
      title: `第 ${chapters.length + 1} 章　新章节`,
      order: chapters.length + 1,
      wordCount: 0,
      status: 'empty',
    };
    setVolumes((prev) => {
      const copy = [...prev];
      copy[copy.length - 1].chapters.push(newChapter);
      return copy;
    });
    setSelectedId(newChapter.id);
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

  return (
    <div data-editor-focus={focusMode} className="manuscript-workspace h-[calc(100%+3rem)] flex animate-fade-up -m-6">
      {/* Left chapter tree */}
      <aside className={clsx("chapter-navigation w-[260px] shrink-0 flex flex-col border-r border-border bg-surface", focusMode && "hidden")}>
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
          <Button variant="default" size="sm" onClick={createChapter} className="w-full gap-1">
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
                    onClick={() => setSelectedId(chapter.id)}
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
          <div className="flex-1 flex items-center justify-center p-6">
            <EmptyState
              icon={BookOpen}
              title="选择章节开始写作"
              description="从左侧章节树中选择一章，或新建章节进入沉浸式写作。"
            />
          </div>
        ) : (
          <>
            {/* Model warning */}
            {!getActiveModel(state.settings).apiKeyConfigured && (
              <div
                data-testid="model-configuration-notice"
                className="px-5 py-2 bg-model-notice border-b border-model-notice-border text-text flex items-center gap-3"
              >
                <AlertCircle className="w-4 h-4 text-warning shrink-0" />
                <p className="text-sm text-text-sub flex-1">尚未配置 AI 模型，创作助手与浮动操作仅展示示例。</p>
                <Button size="sm" onClick={() => navigate('/settings')}>前往配置</Button>
              </div>
            )}

            {/* Top toolbar */}
            <div className="flex items-center justify-between gap-3 px-5 py-3 border-b border-border bg-surface/80 backdrop-blur-sm sticky top-0 z-10">
              <div className="flex items-center gap-2">
                <Button
                  variant="ghost"
                  size="sm"
                  disabled={!canPrev}
                  onClick={() => setSelectedId(chapters[selectedIndex - 1].id)}
                  className="px-2"
                >
                  <ChevronLeft className="w-4 h-4" />
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  disabled={!canNext}
                  onClick={() => setSelectedId(chapters[selectedIndex + 1].id)}
                  className="px-2"
                >
                  <ChevronRight className="w-4 h-4" />
                </Button>
                <div className="h-4 w-px bg-border mx-1" />
                <Button variant="ghost" size="sm" className="px-2" title="撤销" disabled={!historyRef.current.past.length} onClick={() => historyAction()}>
                  <Undo2 className="w-4 h-4" />
                </Button>
                <Button variant="ghost" size="sm" className="px-2" title="重做" disabled={!historyRef.current.future.length} onClick={() => historyAction(true)}>
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
                <Button variant="ghost" size="sm" onClick={() => navigate('/versions')} className="gap-1 px-2">
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
                <span className="flex items-center gap-1.5">
                  {saveState === 'saving' ? (
                    <Loader2 className="w-3.5 h-3.5 animate-spin" />
                  ) : (
                    <Save className="w-3.5 h-3.5" />
                  )}
                  {saveState === 'saved' ? '自动保存于 刚刚' : '保存中…'}
                </span>
                <span className="flex items-center gap-1.5">
                  <Clock className="w-3.5 h-3.5" />
                  版本 v0.3
                </span>
                <span className="font-medium text-text">{liveWordCount.toLocaleString()} 字</span>
              </div>
            </div>

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
                  onChange={(e) => setTitle(e.target.value)}
                  className="text-2xl font-display font-semibold bg-transparent border-0 shadow-none px-0 mb-6 focus:ring-0 placeholder:text-text-muted"
                  style={{ color: canvasColor.text }}
                  placeholder="章节标题"
                />
                <Textarea
                  ref={editorRef}
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
            <div className="p-3 bg-primary-subtle border border-primary/10 rounded-lg text-xs text-text-sub leading-relaxed">
              基于当前章节、选中文本或你的指令工作。AI 不会直接覆盖正文，所有修改都会以候选片段形式供你确认。
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
                onClick={sendAiInstruction}
                disabled={aiLoading || !aiInput.trim()}
                className="w-full gap-1"
              >
                {aiLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Send className="w-4 h-4" />}
                发送给 AI
              </Button>
            </div>

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
                    {!c.applied && (
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
