import { useMemo, useRef, useState, type ReactNode } from 'react';
import { clsx } from 'clsx';
import {
  GitPullRequest,
  Check,
  RotateCcw,
  MessageSquarePlus,
  AlertCircle,
  Info,
  Zap,
  BookOpen,
  Clock,
  ChevronRight,
  FileDiff,
  Edit3,
  Trash2,
  Sparkles,
  Target,
  GitMerge,
  Layers,
  Type,
  Palette,
  AlignLeft,
} from 'lucide-react';
import { Button } from '../components/ui/Button';
import { Card } from '../components/ui/Card';
import { Textarea } from '../components/ui/Input';
import { StatusBadge } from '../components/ui/StatusBadge';
import { useApp } from '../store/useApp';
import { canvasPresets } from '../editor';

type IssueType = 'character' | 'continuity' | 'causality' | 'pacing' | 'world' | 'foreshadowing' | 'knowledge';
type Severity = 'high' | 'medium' | 'low';
type ChapterStatus = 'pending' | 'confirmed' | 'rewrite' | 'discarded';
type MiddleView = 'text' | 'diff' | 'history';

interface Issue {
  id: string;
  type: IssueType;
  severity: Severity;
  start: number;
  end: number;
  title: string;
  scope: string;
  evidence: string;
  suggestion: string;
}

interface BatchChapter {
  id: string;
  title: string;
  status: ChapterStatus;
  wordCount: number;
  riskCount: number;
}

interface ImpactPlan {
  affected: string[];
  unaffected: string[];
  conflicts: string[];
  estimatedScope: string;
}

const issueMeta: Record<IssueType, { label: string; color: string; icon: typeof AlertCircle }> = {
  character: { label: '人物动机', color: 'bg-amber-100 text-amber-700 border-amber-200', icon: Target },
  continuity: { label: '连续性', color: 'bg-rose-100 text-rose-700 border-rose-200', icon: Layers },
  causality: { label: '因果', color: 'bg-orange-100 text-orange-700 border-orange-200', icon: GitMerge },
  pacing: { label: '节奏', color: 'bg-blue-100 text-blue-700 border-blue-200', icon: Zap },
  world: { label: '世界规则', color: 'bg-emerald-100 text-emerald-700 border-emerald-200', icon: BookOpen },
  foreshadowing: { label: '伏笔', color: 'bg-purple-100 text-purple-700 border-purple-200', icon: Sparkles },
  knowledge: { label: '知识边界', color: 'bg-slate-100 text-slate-700 border-slate-200', icon: Info },
};

const severityMeta: Record<Severity, { label: string; badge: 'danger' | 'warning' | 'default' }> = {
  high: { label: '严重', badge: 'danger' },
  medium: { label: '中等', badge: 'warning' },
  low: { label: '提示', badge: 'default' },
};

const candidateText = `灯芯是新的。灯油也只用了不到一成。这说明有人最近上过塔，而且不是匆匆一瞥——他花时间添了油、换了芯，甚至还像真正的守塔人一样，把灯罩擦了一遍。

沈知秋的手指抚过灯罩边缘，那里有一道新鲜的棉布擦痕。谁会做这些？如果周砚白已经失踪，那么替他来守灯的人，又是谁？

他走下螺旋楼梯，脚步放得很轻。雾气正从门缝渗进来，像一匹湿透的白布。楼下没有声音，但他确信，就在刚才，有人站在这里。`;

const issues: Issue[] = [
  {
    id: 'iss-1',
    type: 'continuity',
    severity: 'high',
    start: 11,
    end: 22,
    title: '灯油消耗与“灯未亮”存在连续性冲突',
    scope: '涉及第一章、第三章',
    evidence: '第一章船夫说“灯塔上的灯，昨夜没亮”；若灯未亮，灯油不应“只用了不到一成”。',
    suggestion: '改为“灯油是满的”或说明昨夜有人私下点灯，以保留悬疑。',
  },
  {
    id: 'iss-2',
    type: 'knowledge',
    severity: 'medium',
    start: 62,
    end: 73,
    title: '主角过早推断“替周砚白守灯”',
    scope: '第三章结尾',
    evidence: '沈知秋刚到岛上，尚未确认周砚白失踪，直接得出“替他来守灯”结论过于跳跃。',
    suggestion: '将结论改为疑问，例如“如果周砚白不在，又是谁在做这些事？”',
  },
  {
    id: 'iss-3',
    type: 'foreshadowing',
    severity: 'low',
    start: 91,
    end: 104,
    title: '“棉布擦痕”可作为身份伏笔',
    scope: '第三章 → 第六章',
    evidence: '棉布擦痕暗示擦灯者习惯用旧布，可与渔村少女阿沅的设定呼应。',
    suggestion: '保留此细节，并在第六章通过阿沅的袖口补丁回收。',
  },
];

const batchChapters: BatchChapter[] = [
  { id: 'b1', title: '第三章 · 灯芯', status: 'pending', wordCount: 4200, riskCount: 2 },
  { id: 'b2', title: '第四章 · 雾中人影', status: 'pending', wordCount: 3850, riskCount: 1 },
  { id: 'b3', title: '第五章 · 潮间带', status: 'confirmed', wordCount: 4100, riskCount: 0 },
  { id: 'b4', title: '第六章 · 礁石密道', status: 'rewrite', wordCount: 1200, riskCount: 0 },
  { id: 'b5', title: '第七章 · 未寄出的第十七封信', status: 'discarded', wordCount: 0, riskCount: 0 },
];

export default function Review() {
  const { state, dispatch } = useApp();
  const settings = state.settings;
  const canvasColor = useMemo(() => canvasPresets.find((p) => p.id === settings.canvasColorId) || canvasPresets[0], [settings.canvasColorId]);

  const [selectedChapterId, setSelectedChapterId] = useState('b1');
  const [selectedIssueId, setSelectedIssueId] = useState<string | null>(null);
  const [middleView, setMiddleView] = useState<MiddleView>('text');
  const [hoveredIssueId, setHoveredIssueId] = useState<string | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);

  const [selection, setSelection] = useState<{ start: number; end: number; text: string; left: number; top: number } | null>(null);
  const [feedbackDraft, setFeedbackDraft] = useState('');
  const [impactPlan, setImpactPlan] = useState<ImpactPlan | null>(null);
  const [impactLoading, setImpactLoading] = useState(false);
  const [regenerating, setRegenerating] = useState(false);
  const [chapterStatuses, setChapterStatuses] = useState<Record<string, ChapterStatus>>(
    Object.fromEntries(batchChapters.map((c) => [c.id, c.status]))
  );

  const contentRef = useRef<HTMLDivElement>(null);

  const selectedChapter = useMemo(
    () => batchChapters.find((c) => c.id === selectedChapterId) || batchChapters[0],
    [selectedChapterId]
  );

  const stats = useMemo(() => {
    const total = batchChapters.length;
    const pending = batchChapters.filter((c) => chapterStatuses[c.id] === 'pending').length;
    const risks = batchChapters.reduce((sum, c) => sum + c.riskCount, 0);
    return { total, pending, risks };
  }, [chapterStatuses]);

  const handleMouseUp = () => {
    const sel = window.getSelection();
    if (!sel || sel.isCollapsed || !contentRef.current) return;
    const range = sel.getRangeAt(0);
    if (!contentRef.current.contains(range.commonAncestorContainer)) return;
    const text = range.toString().trim();
    if (!text) return;
    const rect = range.getBoundingClientRect();
    setSelection({
      start: 0,
      end: 0,
      text,
      left: rect.left + rect.width / 2,
      top: rect.top - 8,
    });
    setFeedbackDraft('');
    setImpactPlan(null);
  };

  const analyzeImpact = () => {
    if (!feedbackDraft.trim()) return;
    setImpactLoading(true);
    window.setTimeout(() => {
      setImpactPlan({
        affected: ['第三章 · 灯芯', '第四章 · 雾中人影'],
        unaffected: ['第五章 · 潮间带'],
        conflicts: ['既定事实：第一章灯未亮'],
        estimatedScope: '约 680 字，涉及 2 处候选改写',
      });
      setImpactLoading(false);
    }, 900);
  };

  const regenerate = () => {
    setRegenerating(true);
    window.setTimeout(() => {
      setRegenerating(false);
      setImpactPlan(null);
      setSelection(null);
      setFeedbackDraft('');
    }, 1200);
  };

  const setChapterStatus = (id: string, status: ChapterStatus) => {
    setChapterStatuses((prev) => ({ ...prev, [id]: status }));
  };

  const renderAnnotatedText = (text: string, issues: Issue[]) => {
    const sorted = [...issues].sort((a, b) => a.start - b.start);
    const paragraphs = text.split(/\n\n+/);
    let cursor = 0;

    return paragraphs.map((para, pIdx) => {
      const paraStart = cursor;
      const paraEnd = cursor + para.length;
      cursor = paraEnd + 2;
      const spans: ReactNode[] = [];
      let pCursor = paraStart;

      for (const issue of sorted) {
        if (issue.end <= pCursor || issue.start >= paraEnd) continue;
        const s = Math.max(pCursor, issue.start);
        const e = Math.min(paraEnd, issue.end);
        if (s > pCursor) {
          spans.push(<span key={`${pIdx}-n-${s}`}>{text.slice(pCursor, s)}</span>);
        }
        const meta = issueMeta[issue.type];
        const isActive = selectedIssueId === issue.id || hoveredIssueId === issue.id;
        spans.push(
          <mark
            key={`${pIdx}-a-${issue.id}`}
            onClick={(e) => {
              e.stopPropagation();
              setSelectedIssueId(issue.id);
            }}
            onMouseEnter={() => setHoveredIssueId(issue.id)}
            onMouseLeave={() => setHoveredIssueId(null)}
            className={clsx(
              'rounded-sm px-0.5 cursor-pointer transition-all border-b-2',
              meta.color,
              isActive ? 'ring-2 ring-offset-1 ring-primary' : ''
            )}
            title={issue.title}
          >
            {text.slice(s, e)}
          </mark>
        );
        pCursor = e;
        if (pCursor >= paraEnd) break;
      }
      if (pCursor < paraEnd) {
        spans.push(<span key={`${pIdx}-n-${pCursor}`}>{text.slice(pCursor, paraEnd)}</span>);
      }
      return (
        <p
          key={pIdx}
          className="font-display"
          style={{
            color: canvasColor.text,
            textIndent: settings.indent ? '2em' : '0',
            marginBottom: pIdx === paragraphs.length - 1 ? 0 : `${settings.lineHeight}em`,
          }}
        >
          {spans}
        </p>
      );
    });
  };

  const renderDiff = () => (
    <div className="grid grid-cols-2 gap-4 h-full">
      <div className="bg-surface border border-border rounded-lg p-4 overflow-auto">
        <p className="text-xs text-text-muted mb-3">原文</p>
        <p className="text-sm font-display leading-relaxed text-text-sub line-through decoration-rose-400">
          灯芯是新的。这说明有人最近上过塔。
        </p>
      </div>
      <div className="bg-surface border border-border rounded-lg p-4 overflow-auto">
        <p className="text-xs text-text-muted mb-3">候选</p>
        <p className="text-sm font-display leading-relaxed text-text">{candidateText}</p>
      </div>
    </div>
  );

  const renderHistory = () => (
    <div className="space-y-3">
      <Card padding="md" className="border-l-4 border-l-primary">
        <div className="flex items-center gap-2 mb-1">
          <Clock className="w-3.5 h-3.5 text-text-muted" />
          <span className="text-xs text-text-muted">2026-08-13 10:24</span>
        </div>
        <p className="text-sm text-text">AI 提交候选：扩展第三章灯芯段落，增加灯油与灯罩细节。</p>
      </Card>
      <Card padding="md" className="border-l-4 border-l-warning">
        <div className="flex items-center gap-2 mb-1">
          <Clock className="w-3.5 h-3.5 text-text-muted" />
          <span className="text-xs text-text-muted">2026-08-13 10:45</span>
        </div>
        <p className="text-sm text-text">作者批注：灯油消耗与第一章“灯未亮”存在冲突。</p>
      </Card>
    </div>
  );

  return (
    <div className="h-full flex flex-col animate-fade-up -m-6">
      {/* Header */}
      <header className="px-6 py-4 border-b border-border bg-surface flex items-center justify-between shrink-0">
        <div>
          <h1 className="text-lg font-medium text-text flex items-center gap-2">
            <GitPullRequest className="w-5 h-5 text-primary" />
            审阅与候选
          </h1>
          <p className="text-sm text-text-sub mt-1">以文本、证据和差异为核心，逐批确认 AI 候选。</p>
        </div>
        <div className="flex items-center gap-2 text-sm">
          <span className="px-3 py-1.5 rounded-md bg-surface-elevated border border-border text-text-sub">
            本批次 <strong className="text-text">{stats.total}</strong> 章
          </span>
          <span className="px-3 py-1.5 rounded-md bg-surface-elevated border border-border text-text-sub">
            待确认 <strong className="text-warning">{stats.pending}</strong>
          </span>
          <span className="px-3 py-1.5 rounded-md bg-surface-elevated border border-border text-text-sub">
            连续性风险 <strong className="text-danger">{stats.risks}</strong>
          </span>
        </div>
      </header>

      {/* Main */}
      <div className="flex-1 flex min-h-0">
        {/* Left chapter list */}
        <aside className="w-[260px] shrink-0 flex flex-col border-r border-border bg-surface">
          <div className="p-4 border-b border-border">
            <h2 className="text-sm font-medium text-text">本批次章节</h2>
          </div>
          <div className="flex-1 overflow-auto p-3 pb-0 space-y-2">
            {batchChapters.map((chapter) => {
              const status = chapterStatuses[chapter.id];
              return (
                <button
                  key={chapter.id}
                  onClick={() => setSelectedChapterId(chapter.id)}
                  className={clsx(
                    'w-full text-left rounded-lg border p-3 transition-colors',
                    selectedChapterId === chapter.id
                      ? 'bg-primary-subtle border-primary/30'
                      : 'bg-surface-elevated border-border hover:border-border-strong'
                  )}
                >
                  <div className="flex items-start justify-between gap-2 mb-1">
                    <span className={clsx('text-sm truncate', selectedChapterId === chapter.id ? 'text-primary' : 'text-text')}>
                      {chapter.title}
                    </span>
                    {chapter.riskCount > 0 && (
                      <span className="shrink-0 w-5 h-5 rounded-full bg-danger/10 text-danger text-[10px] flex items-center justify-center font-medium">
                        {chapter.riskCount}
                      </span>
                    )}
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-xs text-text-muted">{chapter.wordCount.toLocaleString()} 字</span>
                    <StatusBadge
                      variant={
                        status === 'confirmed' ? 'success' : status === 'rewrite' ? 'warning' : status === 'discarded' ? 'danger' : 'default'
                      }
                      className="text-[10px]"
                    >
                      {status === 'confirmed' ? '已确认' : status === 'rewrite' ? '需重写' : status === 'discarded' ? '已废弃' : '待确认'}
                    </StatusBadge>
                  </div>
                </button>
              );
            })}
          </div>
        </aside>

        {/* Middle text */}
        <main
          className="flex-1 flex flex-col min-w-0"
          style={{ backgroundColor: canvasColor.bg, color: canvasColor.text }}
        >
          <div className="flex items-center justify-between px-5 py-3 border-b border-border bg-surface/80 backdrop-blur-sm">
            <div className="flex items-center gap-2">
              {(['text', 'diff', 'history'] as const).map((v) => (
                <Button
                  key={v}
                  variant={middleView === v ? 'primary' : 'ghost'}
                  size="sm"
                  onClick={() => setMiddleView(v)}
                  className="gap-1"
                >
                  {v === 'text' && <BookOpen className="w-4 h-4" />}
                  {v === 'diff' && <FileDiff className="w-4 h-4" />}
                  {v === 'history' && <Clock className="w-4 h-4" />}
                  {v === 'text' ? '候选正文' : v === 'diff' ? '版本差异' : '审阅历史'}
                </Button>
              ))}
            </div>
            <div className="flex items-center gap-2">
              <div className="text-xs text-text-sub mr-2">{issues.length} 条标注 · {candidateText.replace(/\s/g, '').length} 字</div>
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
                              canvasColor.id === preset.id ? 'border-primary scale-105' : 'border-transparent hover:border-border-strong'
                            )}
                            style={{ backgroundColor: preset.bg }}
                            title={preset.label}
                          />
                        ))}
                      </div>
                    </div>
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
                    </div>
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
                    </div>
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
          </div>

          <div className="flex-1 overflow-auto">
            {middleView === 'text' && (
              <div
                className={clsx(
                  'w-full h-full transition-all duration-200 py-8',
                  settings.marginSize === 'narrow' && 'px-6',
                  settings.marginSize === 'medium' && 'px-12',
                  settings.marginSize === 'wide' && 'px-24',
                )}
              >
                <h2
                  className="text-2xl font-display font-semibold mb-6"
                  style={{ color: canvasColor.text }}
                >
                  {selectedChapter.title}
                </h2>
                <div
                  ref={contentRef}
                  onMouseUp={handleMouseUp}
                  className="prose-literary min-h-[70vh]"
                  style={{
                    color: canvasColor.text,
                    fontSize: `${settings.fontSize}px`,
                    lineHeight: settings.lineHeight,
                    textIndent: settings.indent ? '2em' : '0',
                  }}
                >
                  {renderAnnotatedText(candidateText, issues)}
                </div>
              </div>
            )}
            {middleView === 'diff' && renderDiff()}
            {middleView === 'history' && (
              <div className="max-w-2xl mx-auto">{renderHistory()}</div>
            )}
          </div>
        </main>

        {/* Right review panel */}
        <aside className="w-[360px] shrink-0 border-l border-border bg-surface flex flex-col">
          <div className="p-4 border-b border-border">
            <h2 className="text-sm font-medium text-text">审阅结论</h2>
          </div>
          <div className="flex-1 overflow-auto p-4 space-y-4">
            {issues.map((issue) => {
              const meta = issueMeta[issue.type];
              const sev = severityMeta[issue.severity];
              const Icon = meta.icon;
              const isSelected = selectedIssueId === issue.id;
              return (
                <Card
                  key={issue.id}
                  padding="md"
                  className={clsx('cursor-pointer transition-all', isSelected && 'ring-1 ring-primary')}
                  onClick={() => setSelectedIssueId(issue.id)}
                  onMouseEnter={() => setHoveredIssueId(issue.id)}
                  onMouseLeave={() => setHoveredIssueId(null)}
                >
                  <div className="flex items-start gap-2.5 mb-2">
                    <span className={clsx('px-1.5 py-0.5 rounded text-[10px] border flex items-center gap-1', meta.color)}>
                      <Icon className="w-3 h-3" />
                      {meta.label}
                    </span>
                    <StatusBadge variant={sev.badge} className="text-[10px]">{sev.label}</StatusBadge>
                  </div>
                  <p className="text-sm font-medium text-text mb-1">{issue.title}</p>
                  <p className="text-xs text-text-sub line-clamp-2 mb-3">{issue.suggestion}</p>

                  {isSelected && (
                    <div className="space-y-3 pt-3 border-t border-border">
                      <div>
                        <p className="text-[11px] text-text-muted mb-1">涉及范围</p>
                        <p className="text-xs text-text-sub">{issue.scope}</p>
                      </div>
                      <div>
                        <p className="text-[11px] text-text-muted mb-1">证据</p>
                        <p className="text-xs text-text-sub leading-relaxed bg-surface-hover p-2 rounded">{issue.evidence}</p>
                      </div>
                      <div>
                        <p className="text-[11px] text-text-muted mb-1">建议</p>
                        <p className="text-xs text-text-sub leading-relaxed">{issue.suggestion}</p>
                      </div>
                    </div>
                  )}
                </Card>
              );
            })}

            {/* Author feedback impact plan */}
            {impactPlan && (
              <Card padding="md" className="bg-primary-subtle border-primary/15">
                <div className="flex items-center gap-2 mb-3">
                  <AlertCircle className="w-4 h-4 text-primary" />
                  <h3 className="text-sm font-medium text-text">影响方案</h3>
                </div>
                <div className="space-y-3 text-xs">
                  <div>
                    <p className="text-text-sub mb-1">受影响候选</p>
                    <ul className="space-y-1">
                      {impactPlan.affected.map((a, i) => (
                        <li key={i} className="flex items-center gap-1.5 text-text">
                          <ChevronRight className="w-3 h-3 text-warning" />
                          {a}
                        </li>
                      ))}
                    </ul>
                  </div>
                  <div>
                    <p className="text-text-sub mb-1">无需改动</p>
                    <ul className="space-y-1">
                      {impactPlan.unaffected.map((a, i) => (
                        <li key={i} className="flex items-center gap-1.5 text-text-sub">
                          <Check className="w-3 h-3 text-success" />
                          {a}
                        </li>
                      ))}
                    </ul>
                  </div>
                  {impactPlan.conflicts.length > 0 && (
                    <div className="p-2 bg-danger/5 border border-danger/10 rounded">
                      <p className="text-danger font-medium mb-1">与既定事实冲突</p>
                      {impactPlan.conflicts.map((c, i) => (
                        <p key={i} className="text-text-sub">{c}</p>
                      ))}
                    </div>
                  )}
                  <div className="flex items-center justify-between pt-2 border-t border-primary/10">
                    <span className="text-text-sub">预计改写范围</span>
                    <span className="font-medium text-text">{impactPlan.estimatedScope}</span>
                  </div>
                </div>
                <Button
                  variant="primary"
                  size="sm"
                  onClick={regenerate}
                  disabled={regenerating}
                  className="w-full mt-4 gap-1"
                >
                  {regenerating ? <RotateCcw className="w-4 h-4 animate-spin" /> : <Sparkles className="w-4 h-4" />}
                  仅改写相关候选
                </Button>
              </Card>
            )}

            {/* Chapter actions */}
            <div className="pt-4 border-t border-border space-y-2">
              <p className="text-xs font-medium text-text-sub uppercase tracking-wider mb-2">本章操作</p>
              <Button
                variant="primary"
                size="sm"
                className="w-full gap-1"
                onClick={() => setChapterStatus(selectedChapter.id, 'confirmed')}
                disabled={chapterStatuses[selectedChapter.id] === 'confirmed'}
              >
                <Check className="w-4 h-4" />
                接受定稿
              </Button>
              <div className="grid grid-cols-2 gap-2">
                <Button
                  variant="default"
                  size="sm"
                  className="gap-1"
                  onClick={() => setChapterStatus(selectedChapter.id, 'rewrite')}
                >
                  <RotateCcw className="w-4 h-4" />
                  要求重写
                </Button>
                <Button
                  variant="default"
                  size="sm"
                  className="gap-1"
                  onClick={() => setChapterStatus(selectedChapter.id, 'discarded')}
                >
                  <Trash2 className="w-4 h-4" />
                  废弃候选
                </Button>
              </div>
              <div className="grid grid-cols-2 gap-2">
                <Button variant="ghost" size="sm" className="gap-1">
                  <Edit3 className="w-4 h-4" />
                  直接编辑
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  className="gap-1"
                  onClick={() => setMiddleView('diff')}
                >
                  <FileDiff className="w-4 h-4" />
                  查看差异
                </Button>
              </div>
            </div>
          </div>
        </aside>
      </div>

      {/* Floating feedback composer */}
      {selection && (
        <div
          className="fixed z-50 bg-surface border border-border rounded-xl shadow-xl p-4 w-80"
          style={{ left: selection.left, top: selection.top, transform: 'translate(-50%, -100%)' }}
        >
          <p className="text-xs text-text-muted mb-2">选中：「{selection.text.slice(0, 20)}{selection.text.length > 20 ? '…' : ''}」</p>
          <Textarea
            value={feedbackDraft}
            onChange={(e) => setFeedbackDraft(e.target.value)}
            placeholder="例如：这里主角不应该知道真相，重写这一段"
            className="min-h-[80px] text-sm mb-3"
          />
          {!impactPlan ? (
            <Button
              variant="primary"
              size="sm"
              onClick={analyzeImpact}
              disabled={impactLoading || !feedbackDraft.trim()}
              className="w-full gap-1"
            >
              {impactLoading ? <RotateCcw className="w-4 h-4 animate-spin" /> : <MessageSquarePlus className="w-4 h-4" />}
              分析影响范围
            </Button>
          ) : (
            <div className="flex gap-2">
              <Button variant="ghost" size="sm" className="flex-1" onClick={() => setSelection(null)}>
                取消
              </Button>
              <Button variant="primary" size="sm" className="flex-1 gap-1" onClick={regenerate} disabled={regenerating}>
                {regenerating ? <RotateCcw className="w-4 h-4 animate-spin" /> : <Sparkles className="w-4 h-4" />}
                确认改写
              </Button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
