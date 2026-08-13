import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { clsx } from 'clsx';
import {
  Upload,
  BookOpen,
  AlertCircle,
  RefreshCw,
  FileText,
  ChevronDown,
  ChevronRight,
  Sparkles,
  Pause,
  Play,
  X,
  Edit3,
  GitMerge,
  Trash2,
  Check,
  Save,
  Search,
  FolderOpen,
  Plus,
} from 'lucide-react';
import { Button } from '../components/ui/Button';
import { Card } from '../components/ui/Card';
import { Input } from '../components/ui/Input';
import { Textarea } from '../components/ui/Input';
import { StatusBadge } from '../components/ui/StatusBadge';
import { useApp } from '../store/useApp';
import { canConfirmWholeBookArchive, createOriginalProject } from '../store/appFlows';

type View = 'wizard' | 'editor' | 'task' | 'review';

interface ImportedChapter {
  id: string;
  title: string;
  order: number;
  wordCount: number;
  status: 'imported' | 'draft';
}

interface ReviewItem {
  id: string;
  category: string;
  title: string;
  content: string;
  evidence: { chapter: string; quote: string }[];
  conflicts?: string[];
  aliases?: string[];
  accepted: boolean;
  rejected: boolean;
}

const importedChapters: ImportedChapter[] = [
  { id: 'i1', title: '序章　雾中灯塔', order: 0, wordCount: 1200, status: 'imported' },
  { id: 'i2', title: '第一章　未寄出的信', order: 1, wordCount: 3240, status: 'imported' },
  { id: 'i3', title: '第二章　江心岛', order: 2, wordCount: 4100, status: 'imported' },
  { id: 'i4', title: '第三章　灯塔不亮', order: 3, wordCount: 2850, status: 'imported' },
  { id: 'i5', title: '第四章　十七封信', order: 4, wordCount: 0, status: 'draft' },
];

const initialReviewItems: ReviewItem[] = [
  {
    id: 'r1',
    category: '全书大纲',
    title: '主线：沈知秋受托调查守塔人失踪案，逐渐揭开灯塔与家族往事',
    content: '故事以沈知秋登岛为起点，通过未寄出的信、海图红圈、三盏熄灯等线索，逐步指向商会私吞灯塔经费与渔村家族恩怨。',
    evidence: [
      { chapter: '第二章', quote: '船夫不肯靠岸，只把行李扔在栈桥尽头，低声说：“灯塔上的灯，昨夜没亮。”' },
      { chapter: '第三章', quote: '那人面前摆着三盏熄灭的油灯，灯芯已经被剪去了一半。' },
    ],
    accepted: false,
    rejected: false,
  },
  {
    id: 'r2',
    category: '人物与势力',
    title: '沈知秋：归国画师，冷静观察者，与故乡江心岛有未解过往',
    content: '主角擅长观察细节，对情感保持距离。童年曾在岛上生活，父亲可能与旧灯塔有关。',
    evidence: [
      { chapter: '第一章', quote: '三日前收到的那封信还在内袋里，字迹潦草，末尾只有一句：“请来接替我。”' },
    ],
    accepted: false,
    rejected: false,
  },
  {
    id: 'r3',
    category: '人物与势力',
    title: '别名合并：沈知秋 / 沈画师',
    content: '多处原文以“沈画师”称呼主角，应与“沈知秋”合并为同一人物。',
    aliases: ['沈知秋', '沈画师'],
    evidence: [
      { chapter: '第二章', quote: '码头上有人喊：“沈画师，这边。”' },
    ],
    accepted: false,
    rejected: false,
  },
  {
    id: 'r4',
    category: '世界设定',
    title: '江心岛地理：大雾天水位下降，露出通往旧灯塔的礁石密道',
    content: '雾不仅是氛围元素，也是进入灯塔地下层的自然条件。',
    evidence: [
      { chapter: '第三章', quote: '雾气带着水草与铁锈混合的气息，黏在衣领和袖口。' },
    ],
    accepted: false,
    rejected: false,
  },
  {
    id: 'r5',
    category: '时间线与事实',
    title: '时间冲突：第三章说“三日前收到信”，序章却说“五日前”',
    content: '需要确认主角登岛前的时间线，建议统一为“三日前”。',
    conflicts: ['第三章：三日前', '序章：五日前'],
    evidence: [
      { chapter: '序章', quote: '五日前的那个下午，他第一次读到那封信。' },
      { chapter: '第三章', quote: '三日前收到的那封信还在内袋里。' },
    ],
    accepted: false,
    rejected: false,
  },
  {
    id: 'r6',
    category: '伏笔与关系',
    title: '伏笔：三盏熄灭的油灯与“接替我”的召唤',
    content: '油灯数量暗示三任守塔人，信中的“接替我”暗示主角将成为新的守塔人。',
    evidence: [
      { chapter: '第三章', quote: '那人面前摆着三盏熄灭的油灯。' },
      { chapter: '第一章', quote: '末尾只有一句：“请来接替我。”' },
    ],
    accepted: false,
    rejected: false,
  },
];

const categories = ['全书大纲', '人物与势力', '世界设定', '时间线与事实', '伏笔与关系'];

export default function Import() {
  const navigate = useNavigate();
  const { dispatch } = useApp();
  const [view, setView] = useState<View>('wizard');

  // Wizard
  const [fileName, setFileName] = useState('');
  const [bookTitle, setBookTitle] = useState('');
  const [genre, setGenre] = useState('');
  const [hasRights, setHasRights] = useState(false);
  const [dragOver, setDragOver] = useState(false);

  // Editor
  const [selectedChapterId, setSelectedChapterId] = useState('i3');
  const [editorContent, setEditorContent] = useState(`沈知秋抵达江心岛时，雾正从水面升起。

码头上只剩一盏马灯，被风推着晃。船夫不肯靠岸，只把行李扔在栈桥尽头，低声说：“灯塔上的灯，昨夜没亮。”

他抬眼望去，灰色的塔身隐没在雾中，像一只沉默的巨兽。三日前收到的那封信还在内袋里，字迹潦草，末尾只有一句：“请来接替我。”

栈桥的木板在脚下发出空洞的回响。沈知秋走得很慢，每一步都在确认这地方是否还属于人间。雾气带着水草与铁锈混合的气息，黏在衣领和袖口。他忽然停住——栈桥尽头，有人跪在那里。`);

  // Task
  const [taskProgress, setTaskProgress] = useState(0);
  const [taskStatus, setTaskStatus] = useState<'idle' | 'running' | 'paused' | 'completed' | 'cancelled'>('idle');
  const [processedRange, setProcessedRange] = useState('等待开始');
  const [archiveCandidateReady, setArchiveCandidateReady] = useState(false);
  const [sourceChanged, setSourceChanged] = useState(false);

  // Review
  const [reviewItems, setReviewItems] = useState<ReviewItem[]>(initialReviewItems);
  const [expandedItems, setExpandedItems] = useState<Set<string>>(new Set());
  const [editingItem, setEditingItem] = useState<string | null>(null);

  const selectedChapter = useMemo(
    () => importedChapters.find((c) => c.id === selectedChapterId) || importedChapters[0],
    [selectedChapterId]
  );

  const totalWords = useMemo(
    () => importedChapters.reduce((sum, c) => sum + c.wordCount, 0),
    []
  );

  const acceptedCount = useMemo(() => reviewItems.filter((i) => i.accepted).length, [reviewItems]);
  const canConfirmArchive = canConfirmWholeBookArchive(reviewItems) && !sourceChanged;

  useEffect(() => {
    if (taskStatus !== 'running') return;
    const timer = window.setInterval(() => {
      setTaskProgress((p) => {
        if (p >= 100) {
          window.clearInterval(timer);
          setTaskStatus('completed');
          setArchiveCandidateReady(true);
          return 100;
        }
        const next = Math.min(100, p + 3);
        if (next >= 50) setProcessedRange('第 2–3 章 / 共 4 章');
        if (next >= 80) setProcessedRange('第 3–4 章 / 共 4 章');
        return next;
      });
    }, 300);
    return () => window.clearInterval(timer);
  }, [taskStatus]);

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(false);
    setFileName('雾中灯塔-手稿.txt');
    setBookTitle('雾中灯塔');
    setGenre('民国悬疑');
  };

  const startImport = () => {
    if (!bookTitle || !hasRights) return;
    const project = createOriginalProject({ title: bookTitle, genre, author: '未署名作者' });
    dispatch({ type: 'SET_PROJECT', payload: { ...project, status: 'writing', phase: '导入后续写' } });
    setView('editor');
  };

  /** 启动整书任务时冻结本次来源；后续正文修改会让候选包失效。 */
  const startWholeBookArchive = () => {
    setTaskProgress(0);
    setProcessedRange('第 1–2 章 / 共 4 章');
    setSourceChanged(false);
    setArchiveCandidateReady(false);
    setTaskStatus('running');
    setView('task');
  };

  const toggleExpand = (id: string) => {
    setExpandedItems((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const updateItem = (id: string, patch: Partial<ReviewItem>) => {
    setReviewItems((prev) => prev.map((i) => (i.id === id ? { ...i, ...patch } : i)));
  };

  const mergeAlias = (id: string) => {
    updateItem(id, { aliases: [...(reviewItems.find((i) => i.id === id)?.aliases || []), '已合并'] });
  };

  const resolveConflict = (id: string) => {
    updateItem(id, { conflicts: [] });
  };

  const renderWizard = () => (
    <div className="max-w-2xl mx-auto space-y-6 animate-fade-up">
      <Card padding="lg">
        <div
          onDragOver={(e) => {
            e.preventDefault();
            setDragOver(true);
          }}
          onDragLeave={() => setDragOver(false)}
          onDrop={handleDrop}
          className={clsx(
            'border-2 border-dashed rounded-xl p-10 transition-colors text-center cursor-pointer',
            dragOver ? 'border-primary bg-primary-subtle' : 'border-border bg-surface-hover'
          )}
        >
          <div className="w-12 h-12 rounded-full bg-surface border border-border flex items-center justify-center mx-auto mb-4">
            <FolderOpen className={clsx('w-6 h-6', dragOver ? 'text-primary' : 'text-text-muted')} />
          </div>
          <p className="text-sm font-medium text-text mb-1">选择 TXT 或 Markdown 小说文件</p>
          <p className="text-xs text-text-sub">也可将文件拖入此区域</p>
          {fileName && (
            <div className="mt-4 inline-flex items-center gap-2 px-3 py-1.5 bg-surface border border-border rounded-md text-xs text-text">
              <FileText className="w-3.5 h-3.5" />
              {fileName}
            </div>
          )}
        </div>

        <div className="grid grid-cols-2 gap-4 mt-6">
          <div>
            <label className="text-xs font-medium text-text-sub uppercase tracking-wider mb-1.5 block">书名</label>
            <Input value={bookTitle} onChange={(e) => setBookTitle(e.target.value)} placeholder="例如：雾中灯塔" />
          </div>
          <div>
            <label className="text-xs font-medium text-text-sub uppercase tracking-wider mb-1.5 block">题材</label>
            <Input value={genre} onChange={(e) => setGenre(e.target.value)} placeholder="例如：民国悬疑" />
          </div>
        </div>

        <label className="flex items-start gap-3 mt-5 cursor-pointer">
          <input
            type="checkbox"
            checked={hasRights}
            onChange={(e) => setHasRights(e.target.checked)}
            className="mt-0.5 w-4 h-4 rounded border-border bg-surface text-primary focus:ring-primary"
          />
          <span className="text-sm text-text-sub leading-relaxed">
            我确认拥有处理该材料的权利，或已获得必要授权。导入仅用于个人创作与本地分析。
          </span>
        </label>

        <div className="mt-5 flex items-start gap-3 p-3 bg-primary-subtle border border-primary/10 rounded-lg">
          <AlertCircle className="w-4 h-4 text-primary shrink-0 mt-0.5" />
          <p className="text-xs text-text-sub leading-relaxed">
            导入不会自动调用 AI。在你明确发起「生成整书档案」之前，文本仅保存在本地，不会上传分析。
          </p>
        </div>

        <div className="flex justify-end mt-6">
          <Button variant="primary" size="lg" onClick={startImport} disabled={!bookTitle || !hasRights} className="gap-2">
            <Upload className="w-4 h-4" />
            开始导入
          </Button>
        </div>
      </Card>
    </div>
  );

  const renderEditor = () => (
    <div className="h-full flex animate-fade-up -m-6">
      {/* Left panel */}
      <aside className="w-[260px] shrink-0 flex flex-col border-r border-border bg-surface">
        <div className="p-4 border-b border-border space-y-3">
          <div className="flex items-center justify-between">
            <h2 className="text-sm font-medium text-text">导入结果</h2>
            <StatusBadge variant="success">已导入</StatusBadge>
          </div>
          <p className="text-xs text-text-sub">{importedChapters.length - 1} 章 · 约 {totalWords.toLocaleString()} 字</p>
          <Button variant="default" size="sm" className="w-full gap-1">
            <Plus className="w-4 h-4" />
            新建章节
          </Button>
        </div>
        <div className="flex-1 overflow-auto p-3 pb-0 space-y-4">
          <div>
            <h3 className="text-xs font-medium text-text-sub uppercase tracking-wider mb-2 px-1">正文</h3>
            <div className="space-y-1">
              {importedChapters.map((chapter) => (
                <button
                  key={chapter.id}
                  onClick={() => setSelectedChapterId(chapter.id)}
                  className={clsx(
                    'w-full text-left px-3 py-2 rounded-lg border transition-colors',
                    selectedChapterId === chapter.id
                      ? 'bg-primary-subtle border-primary/15'
                      : 'bg-surface-elevated border-border hover:border-border-strong'
                  )}
                >
                  <div className="flex items-center justify-between gap-2 mb-1">
                    <span className={clsx('text-sm truncate', selectedChapterId === chapter.id ? 'text-primary' : 'text-text')}>
                      {chapter.title}
                    </span>
                    <StatusBadge variant={chapter.status === 'imported' ? 'success' : 'default'} className="text-[10px]">
                      {chapter.status === 'imported' ? '已导入' : '草稿'}
                    </StatusBadge>
                  </div>
                  <div className="flex items-center justify-between text-xs text-text-muted">
                    <span>{chapter.wordCount.toLocaleString()} 字</span>
                    <span>第 {chapter.order} 章</span>
                  </div>
                </button>
              ))}
            </div>
          </div>
        </div>
      </aside>

      {/* Editor */}
      <main className="flex-1 flex flex-col min-w-0 bg-surface">
        <div className="flex items-center justify-between gap-3 px-5 py-3 border-b border-border bg-surface/80 backdrop-blur-sm sticky top-0 z-10">
          <div className="flex items-center gap-2">
            <Button variant="ghost" size="sm" className="px-2">
              <ChevronRight className="w-4 h-4 rotate-180" />
            </Button>
            <Button variant="ghost" size="sm" className="px-2">
              <ChevronRight className="w-4 h-4" />
            </Button>
            <div className="h-4 w-px bg-border mx-1" />
            <Button variant="ghost" size="sm" className="px-2" title="撤销">
              <RefreshCw className="w-4 h-4" />
            </Button>
            <Button variant="ghost" size="sm" className="gap-1 px-2">
              <Search className="w-4 h-4" />
              查找
            </Button>
            <Button variant="ghost" size="sm" className="gap-1 px-2">
              <Save className="w-4 h-4" />
              保存
            </Button>
          </div>
          <div className="text-xs text-text-sub flex items-center gap-3">
            <span>自动保存于 刚刚</span>
            <span className="font-medium text-text">{editorContent.replace(/\s/g, '').length.toLocaleString()} 字</span>
          </div>
        </div>

        <div className="flex-1 overflow-auto">
          <div className="max-w-3xl mx-auto py-8 px-12">
            <Input
              value={selectedChapter.title}
              readOnly
              className="text-2xl font-display font-semibold bg-transparent border-0 shadow-none px-0 mb-6 text-text"
            />
            <Textarea
              value={editorContent}
              onChange={(e) => {
                setEditorContent(e.target.value);
                if (taskStatus === 'running' || taskStatus === 'paused' || taskStatus === 'completed') {
                  setSourceChanged(true);
                }
              }}
              className="w-full min-h-[60vh] prose-literary bg-transparent border-0 shadow-none focus:ring-0 resize-none leading-loose text-text"
            />
          </div>
        </div>

        {/* Whole-book archive action */}
        <div className="px-5 py-4 border-t border-border bg-surface flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-8 h-8 rounded-md bg-primary-subtle flex items-center justify-center">
              <Sparkles className="w-4 h-4 text-primary" />
            </div>
            <div>
              <p className="text-sm font-medium text-text">AI 生成整书档案</p>
              <p className="text-xs text-text-sub">
                分析整本小说，生成分类档案；长篇会自动分卷分章处理，最终只呈现整书层面的统一结论。
              </p>
            </div>
          </div>
          {archiveCandidateReady ? (
            <Button variant="primary" onClick={() => setView('review')} className="gap-2">
              <Check className="w-4 h-4" />
              查看整书档案候选
            </Button>
          ) : (
            <Button variant="primary" onClick={startWholeBookArchive} className="gap-2">
              <Sparkles className="w-4 h-4" />
              开始生成
            </Button>
          )}
        </div>
      </main>
    </div>
  );

  const renderTask = () => (
    <div className="h-full flex flex-col animate-fade-up -m-6">
      <main className="flex-1 overflow-auto p-8">
        <div className="max-w-3xl mx-auto space-y-6">
          <Card padding="lg">
            <div className="flex items-start gap-4">
              <div className="w-12 h-12 rounded-xl bg-primary-subtle flex items-center justify-center shrink-0">
                {taskStatus === 'running' ? (
                  <RefreshCw className="w-6 h-6 text-primary animate-spin" />
                ) : (
                  <Pause className="w-6 h-6 text-primary" />
                )}
              </div>
              <div className="flex-1">
                <h2 className="text-lg font-medium text-text mb-1">
                  {taskStatus === 'completed' ? '整书档案候选已生成' : '正在生成整书档案'}
                </h2>
                <p className="text-sm text-text-sub mb-4">
                  任务在后台运行，你可以继续编辑正文。完成后会保留一份可审阅的整书候选包，不会直接写入作品档案。
                </p>
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-5">
                  <div className="p-3 bg-surface-hover rounded-lg">
                    <p className="text-xs text-text-sub mb-1">全书范围</p>
                    <p className="text-sm font-medium text-text">{bookTitle || '雾中灯塔'}</p>
                  </div>
                  <div className="p-3 bg-surface-hover rounded-lg">
                    <p className="text-xs text-text-sub mb-1">章节数量</p>
                    <p className="text-sm font-medium text-text">{importedChapters.length - 1} 章</p>
                  </div>
                  <div className="p-3 bg-surface-hover rounded-lg">
                    <p className="text-xs text-text-sub mb-1">已处理范围</p>
                    <p className="text-sm font-medium text-text">{processedRange}</p>
                  </div>
                  <div className="p-3 bg-surface-hover rounded-lg">
                    <p className="text-xs text-text-sub mb-1">预计消耗</p>
                    <p className="text-sm font-medium text-text">约 12,000 tokens</p>
                  </div>
                </div>
                <div className="h-2 bg-surface-hover rounded-full overflow-hidden mb-2">
                  <div
                    className="h-full bg-primary rounded-full transition-all duration-200"
                    style={{ width: `${taskProgress}%` }}
                  />
                </div>
                <div className="flex items-center justify-between text-xs text-text-muted">
                  <span>后台进度 {taskProgress}%</span>
                  <span>预计剩余 2 分钟</span>
                </div>
              </div>
            </div>
          </Card>

          <Card padding="md" className="border-dashed">
            <div className="flex items-start gap-3">
              <BookOpen className="w-5 h-5 text-text-sub shrink-0 mt-0.5" />
              <div>
                <p className="text-sm font-medium text-text mb-1">不阻塞写作</p>
                <p className="text-xs text-text-sub leading-relaxed">
                  章节结果仅作为后台中间过程与证据，不会逐章呈现给你。最终审阅界面只展示整书层面的统一档案。
                </p>
              </div>
            </div>
          </Card>

          <div className="flex items-center justify-end gap-3">
            <Button variant="ghost" onClick={() => setView('editor')}>
              返回正文
            </Button>
            {taskStatus === 'completed' ? (
              <Button variant="primary" onClick={() => setView('review')} className="gap-1">
                <Check className="w-4 h-4" />
                审阅整书候选
              </Button>
            ) : (
              <>
                <Button
                  variant="default"
                  onClick={() => setTaskStatus((status) => (status === 'running' ? 'paused' : 'running'))}
                  className="gap-1"
                >
                  {taskStatus === 'running' ? <Pause className="w-4 h-4" /> : <Play className="w-4 h-4" />}
                  {taskStatus === 'running' ? '暂停任务' : '恢复任务'}
                </Button>
                <Button
                  variant="ghost"
                  className="text-danger gap-1"
                  onClick={() => {
                    setTaskStatus('cancelled');
                    setView('editor');
                  }}
                >
                  <X className="w-4 h-4" />
                  取消
                </Button>
              </>
            )}
          </div>
        </div>
      </main>
    </div>
  );

  const renderReview = () => {
    const grouped = categories.map((cat) => ({
      category: cat,
      items: reviewItems.filter((i) => i.category === cat),
    }));

    return (
      <div className="h-full flex flex-col animate-fade-up -m-6">
        <header className="px-8 py-5 border-b border-border bg-surface flex items-center justify-between shrink-0">
          <div>
            <h1 className="text-lg font-medium text-text">整书档案候选审阅</h1>
            <p className="text-sm text-text-sub mt-1">
              已确认 {acceptedCount} / {reviewItems.length} 条结论
            </p>
            {sourceChanged && (
              <p className="text-xs text-warning mt-2">正文在任务冻结后发生修改；请增量重算后再确认整书档案。</p>
            )}
          </div>
          <div className="flex items-center gap-3">
            <Button variant="ghost" onClick={() => setView('editor')}>
              返回正文
            </Button>
            <Button variant="primary" disabled={!canConfirmArchive} onClick={() => navigate('/archive/outline')}>
              <Check className="w-4 h-4" />
              确认整书档案入库
            </Button>
          </div>
        </header>

        <main className="flex-1 overflow-auto p-8">
          <div className="max-w-4xl mx-auto space-y-8">
            {grouped.map((group) =>
              group.items.length === 0 ? null : (
                <section key={group.category}>
                  <h2 className="text-sm font-medium text-text-sub uppercase tracking-wider mb-4">{group.category}</h2>
                  <div className="space-y-3">
                    {group.items.map((item) => (
                      <Card key={item.id} padding="none" className={clsx('overflow-hidden', item.accepted && 'ring-1 ring-success')}>
                        <div
                          className="p-4 flex items-start gap-3 cursor-pointer hover:bg-surface-hover transition-colors"
                          onClick={() => toggleExpand(item.id)}
                        >
                          <button className="mt-0.5 text-text-muted">
                            {expandedItems.has(item.id) ? (
                              <ChevronDown className="w-4 h-4" />
                            ) : (
                              <ChevronRight className="w-4 h-4" />
                            )}
                          </button>
                          <div className="flex-1 min-w-0">
                            <div className="flex items-start justify-between gap-3">
                              <p className="text-sm font-medium text-text">{item.title}</p>
                              <div className="flex items-center gap-1 shrink-0">
                                {item.conflicts && item.conflicts.length > 0 && (
                                  <span className="text-[10px] px-1.5 py-0.5 rounded bg-danger/10 text-danger">冲突</span>
                                )}
                                {item.aliases && item.aliases.length > 0 && (
                                  <span className="text-[10px] px-1.5 py-0.5 rounded bg-info/10 text-info">别名</span>
                                )}
                                {item.accepted && (
                                  <span className="text-[10px] px-1.5 py-0.5 rounded bg-success/10 text-success">已接受</span>
                                )}
                              </div>
                            </div>
                            <p className="text-xs text-text-sub mt-1 line-clamp-2">{item.content}</p>
                          </div>
                        </div>

                        {expandedItems.has(item.id) && (
                          <div className="px-4 pb-4 pl-11">
                            {editingItem === item.id ? (
                              <div className="space-y-3 mb-4">
                                <Textarea
                                  value={item.content}
                                  onChange={(e) => updateItem(item.id, { content: e.target.value })}
                                  className="min-h-[100px]"
                                />
                                <div className="flex justify-end gap-2">
                                  <Button size="sm" variant="ghost" onClick={() => setEditingItem(null)}>
                                    取消
                                  </Button>
                                  <Button size="sm" onClick={() => setEditingItem(null)}>
                                    保存
                                  </Button>
                                </div>
                              </div>
                            ) : (
                              <p className="text-sm text-text leading-relaxed mb-4">{item.content}</p>
                            )}

                            {item.aliases && item.aliases.length > 0 && (
                              <div className="mb-3 p-3 bg-info/5 border border-info/10 rounded-lg">
                                <p className="text-xs text-info font-medium mb-1">检测到的别名</p>
                                <div className="flex flex-wrap gap-2">
                                  {item.aliases.map((alias, idx) => (
                                    <span key={idx} className="text-xs px-2 py-1 bg-surface border border-border rounded">
                                      {alias}
                                    </span>
                                  ))}
                                </div>
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => mergeAlias(item.id)}
                                  className="mt-2 gap-1 text-xs"
                                >
                                  <GitMerge className="w-3.5 h-3.5" />
                                  合并别名
                                </Button>
                              </div>
                            )}

                            {item.conflicts && item.conflicts.length > 0 && (
                              <div className="mb-3 p-3 bg-danger/5 border border-danger/10 rounded-lg">
                                <p className="text-xs text-danger font-medium mb-1">时间线冲突</p>
                                <ul className="space-y-1 mb-2">
                                  {item.conflicts.map((conflict, idx) => (
                                    <li key={idx} className="text-xs text-text-sub">
                                      {conflict}
                                    </li>
                                  ))}
                                </ul>
                                <Button variant="ghost" size="sm" onClick={() => resolveConflict(item.id)} className="text-xs gap-1">
                                  <Check className="w-3.5 h-3.5" />
                                  标记为已解决
                                </Button>
                              </div>
                            )}

                            <div className="mb-3">
                              <p className="text-xs font-medium text-text-sub mb-2">原文证据</p>
                              <div className="space-y-2">
                                {item.evidence.map((ev, idx) => (
                                  <div key={idx} className="p-3 bg-surface-hover border border-border rounded-lg">
                                    <p className="text-[11px] text-primary mb-1">{ev.chapter}</p>
                                    <p className="text-xs text-text-sub leading-relaxed">{ev.quote}</p>
                                  </div>
                                ))}
                              </div>
                            </div>

                            <div className="flex items-center gap-2">
                              {!item.accepted && (
                                <Button
                                  variant="primary"
                                  size="sm"
                                  onClick={() => updateItem(item.id, { accepted: true, rejected: false })}
                                  className="gap-1"
                                >
                                  <Check className="w-3.5 h-3.5" />
                                  接受
                                </Button>
                              )}
                              <Button
                                variant="ghost"
                                size="sm"
                                onClick={() => setEditingItem(item.id)}
                                className="gap-1"
                              >
                                <Edit3 className="w-3.5 h-3.5" />
                                修改
                              </Button>
                              <Button
                                variant="ghost"
                                size="sm"
                                onClick={() => updateItem(item.id, { rejected: true, accepted: false })}
                                className="gap-1 text-danger"
                              >
                                <Trash2 className="w-3.5 h-3.5" />
                                拒绝
                              </Button>
                            </div>
                          </div>
                        )}
                      </Card>
                    ))}
                  </div>
                </section>
              )
            )}
          </div>
        </main>
      </div>
    );
  };

  return (
    <div className="h-full flex flex-col">
      {view === 'wizard' && (
        <header className="mb-6">
          <h1 className="text-xl font-medium text-text flex items-center gap-2">
            <Upload className="w-5 h-5 text-primary" />
            导入与续写
          </h1>
          <p className="text-sm text-text-sub mt-1">将外部手稿导入项目，确认后即可继续阅读和写作。</p>
        </header>
      )}
      <div className="flex-1 overflow-hidden">
        {view === 'wizard' && renderWizard()}
        {view === 'editor' && renderEditor()}
        {view === 'task' && renderTask()}
        {view === 'review' && renderReview()}
      </div>
    </div>
  );
}
