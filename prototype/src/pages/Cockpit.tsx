import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { clsx } from 'clsx';
import {
  Compass,
  Send,
  RefreshCw,
  Bookmark,
  GitMerge,
  Edit3,
  Check,
  Lock,
  Sparkles,
  ArrowLeft,
  ChevronRight,
  Lightbulb,
  AlertCircle,
  User,
  Map,
  Globe,
  GitBranch,
  Layers,
  Ban,
  Target,
} from 'lucide-react';
import { Button } from '../components/ui/Button';
import { Card } from '../components/ui/Card';
import { Textarea } from '../components/ui/Input';
import { StatusBadge } from '../components/ui/StatusBadge';
import { Modal } from '../components/ui/Modal';
import { SamplePanel } from '../components/creation/SamplePanel';
import { CreationProgress } from '../components/creation/CreationProgress';
import { CandidateFields } from '../components/creation/CandidateFields';
import { EntryPanel } from '../components/creation/EntryPanel';
import { useApp } from '../store/useApp';
import { getActiveModel } from '../types';

interface DirectionCard {
  id: string;
  selected: boolean;
  kept: boolean;
  fields: {
    卖点: string;
    类型: string;
    主角: string;
    核心冲突: string;
    成长代价: string;
    节奏: string;
    结局方向: string;
  };
}

interface DraftSection {
  id: string;
  title: string;
  icon: typeof Target;
  content: string;
  locked: boolean;
  accepted: boolean;
}

interface CreativeContext {
  locked: string[];
  preferences: string[];
  undecided: string[];
}

const initialDirections: DirectionCard[] = [
  {
    id: 'dir-1',
    selected: false,
    kept: false,
    fields: {
      卖点: '一座雾中灯塔，三任守塔人接连失踪，每一盏灯都是一封未寄出的信。',
      类型: '民国悬疑 · 密闭孤岛 · 书信推理',
      主角: '沈知秋：归国画师，冷静观察者，因受托调查回到故乡岛屿。',
      核心冲突: '商会私吞灯塔经费，与渔村家族恩怨交织，真相指向童年旧案。',
      成长代价: '从只想完成委托，到必须亲手点燃灯塔、面对自己逃避的过去。',
      节奏: '前慢后快，第三幕集中回收伏笔，结尾留白。',
      结局方向: '真凶身份揭晓，但灯塔的秘密被继续守护，开放式收尾。',
    },
  },
  {
    id: 'dir-2',
    selected: false,
    kept: false,
    fields: {
      卖点: '江心岛地理奇观：大雾来临水位下降，露出通往旧灯塔的密道。',
      类型: '民国悬疑 · 冒险解谜 · 民俗志怪',
      主角: '沈知秋 + 渔家少女阿沅，两人视角互补，共同揭开灯塔历史。',
      核心冲突: '岛上流传的「灯语」传说与真实历史事件相互缠绕，真假难辨。',
      成长代价: '阿沅为寻父失踪真相，沈知秋则为赎回对故人的亏欠。',
      节奏: '双线并进，每章一个民俗谜题，整体呈阶梯式上升。',
      结局方向: '灯语实为求救信号，父辈的牺牲换来灯塔重建，情感收束。',
    },
  },
  {
    id: 'dir-3',
    selected: false,
    kept: false,
    fields: {
      卖点: '所有失踪的守塔人，都在同一晚看见了同一个「雾中人影」。',
      类型: '心理悬疑 · 超自然暗示 · 不可靠叙事',
      主角: '沈知秋：理性画师，但逐渐怀疑自己是否也在雾中看到了什么。',
      核心冲突: '是人为谋杀，还是集体幻觉？证据不断推翻既有结论。',
      成长代价: '主角必须在「相信理智」与「相信证人」之间做出选择。',
      节奏: '层层反转，每三章一次小颠覆，最终章收束所有视角。',
      结局方向: '雾中人影是前任守塔人假扮，动机出于保护而非伤害。',
    },
  },
];

const initialDraftSections: DraftSection[] = [
  {
    id: 'position',
    title: '故事定位',
    icon: Target,
    content: '民国末期，江心孤岛。以悬疑失踪案为明线，以两代守塔人的信约为暗线。基调克制、潮湿、幽微，不卖腐、不 jump scare。',
    locked: false,
    accepted: false,
  },
  {
    id: 'character',
    title: '人物弧',
    icon: User,
    content: '沈知秋：从旁观者到必须点燃灯塔的人。阿沅：从渔家少女到真相守护者。两人共同完成父辈未竟之事，彼此不构成爱情线。',
    locked: false,
    accepted: false,
  },
  {
    id: 'world',
    title: '世界规则',
    icon: Globe,
    content: '灯塔每夜须有人值守；大雾天水位下降，露出礁石密道；守塔人日志采用特定灯号记录风向与异常。所有「超自然」现象最终须有现实解释。',
    locked: false,
    accepted: false,
  },
  {
    id: 'mainplot',
    title: '主线',
    icon: Map,
    content: '沈知秋登岛调查失踪案 → 发现书信与海图 → 登上灯塔 → 揭开商会阴谋 → 与真凶对峙 → 灯塔重新点亮。',
    locked: false,
    accepted: false,
  },
  {
    id: 'subplot',
    title: '支线',
    icon: GitBranch,
    content: '阿沅寻父线：父亲多年前同样在灯塔附近失踪。周砚白信约线：十七封信的真正收件人与寄件人身份。',
    locked: false,
    accepted: false,
  },
  {
    id: 'ending',
    title: '结局',
    icon: Layers,
    content: '真凶身份揭晓后，沈知秋选择不公开全部真相，而是继续守护灯塔。开放式结尾：下一个雾夜，灯塔准时亮起。',
    locked: false,
    accepted: false,
  },
  {
    id: 'forbidden',
    title: '禁写项',
    icon: Ban,
    content: '不引入真实鬼魂；不将女性角色工具化；不将悬疑简单归结为「精神病」；不出现穿越或超时代科技。',
    locked: false,
    accepted: false,
  },
];

const initialContext: CreativeContext = {
  locked: ['民国末期江心岛', '第三人称限知视角', '主角为男性画师'],
  preferences: ['克制、潮湿的悬疑氛围', '不卖腐、不迷信解释', '每章一个可回收的感官细节'],
  undecided: ['是否加入真实历史事件', '反派是否最终反转', '是否需要爱情线'],
};

export default function Cockpit() {
 const [fieldDemo,setFieldDemo] = useState({candidateId:'demo-1',version:1,fields:[{path:'/directions/@a/hook',label:'卖点',group:'故事方向 · A',value:'一座雾中灯塔，三任守塔人接连失踪。',locked:false},{path:'/characters/@hero/secret',label:'秘密',group:'人物 · 沈知秋',value:'曾见过一封未寄出的信。',locked:false}]});
 const [sampleCard,setSampleCard] = useState('');
 const [samplePreview,setSamplePreview] = useState<{id:string;status:string;pointOfView:string;tone:string;constraints:string[];targetRunes:number}|null>(null);
  const [progressStarted,setProgressStarted] = useState(() => new Date().toISOString());
  const [progressDemo,setProgressDemo] = useState('running');
  const [entryPreview, setEntryPreview] = useState<{ id: string; kind: 'idea' | 'author_plan'; material: string }>({ id: '', kind: 'idea', material: '' });
  const navigate = useNavigate();
  const { state } = useApp();
  const [mode, setMode] = useState<'brainstorm' | 'draft'>('brainstorm');
  const [input, setInput] = useState('');
  const [directions, setDirections] = useState<DirectionCard[]>(initialDirections);
  const [draftSections, setDraftSections] = useState<DraftSection[]>(initialDraftSections);
  const [context, setContext] = useState<CreativeContext>(initialContext);
  const [editingSection, setEditingSection] = useState<string | null>(null);
  const [editingDirection, setEditingDirection] = useState<string | null>(null);
  const [compareOpen, setCompareOpen] = useState(false);
  const [nextHint, setNextHint] = useState('输入你的想法，或让 AI 基于当前上下文给出三个不同方向。');

  const keptCount = directions.filter((d) => d.kept).length;
  const acceptedCount = draftSections.filter((s) => s.accepted).length;

  const handleSend = () => {
    if (!input.trim()) return;
    setNextHint('AI 团队已基于你的输入生成三套故事方向，请比较、编辑或选择其一。');
    setInput('');
  };

  const toggleKeep = (id: string) => {
    setDirections((prev) =>
      prev.map((d) => (d.id === id ? { ...d, kept: !d.kept } : d))
    );
  };

  const redoDirection = (id: string) => {
    setDirections((prev) =>
      prev.map((d) =>
        d.id === id
          ? {
              ...d,
              fields: {
                ...d.fields,
                卖点: d.fields.卖点 + '（已按你的要求重新生成变体…）',
              },
            }
          : d
      )
    );
  };

  const selectDirection = (id: string) => {
    setDirections((prev) =>
      prev.map((d) => ({ ...d, selected: d.id === id }))
    );
    setMode('draft');
  };

  const combineDirections = () => {
    const kept = directions.filter((d) => d.kept);
    if (kept.length < 2) return;
    setNextHint(`已将 ${kept.length} 个保留方向组合为新方案，可继续编辑。`);
  };

  const updateDraft = (id: string, content: string) => {
    setDraftSections((prev) =>
      prev.map((s) => (s.id === id ? { ...s, content } : s))
    );
  };

  const toggleLock = (id: string) => {
    setDraftSections((prev) =>
      prev.map((s) => (s.id === id ? { ...s, locked: !s.locked } : s))
    );
  };

  const acceptSection = (id: string) => {
    setDraftSections((prev) =>
      prev.map((s) => (s.id === id ? { ...s, accepted: true, locked: true } : s))
    );
  };

  const aiModifySection = (id: string) => {
    setDraftSections((prev) =>
      prev.map((s) =>
        s.id === id
          ? { ...s, content: s.content + '\n\n[AI 修改建议：补充一个具体场景示例，并强化与第三章灯芯伏笔的呼应。]' }
          : s
      )
    );
  };

  const addContextItem = (type: keyof CreativeContext, value: string) => {
    if (!value.trim()) return;
    setContext((prev) => ({ ...prev, [type]: [...prev[type], value] }));
  };

  return (
    <div className="h-full flex flex-col animate-fade-up -m-6">
      {/* Header */}
      <header className="px-8 py-5 border-b border-border bg-surface flex items-center justify-between shrink-0">
        <div className="flex items-center gap-4">
          <div className="w-10 h-10 rounded-xl bg-primary-subtle border border-primary/10 flex items-center justify-center">
            <Compass className="w-5 h-5 text-primary" />
          </div>
          <div>
            <h1 className="text-lg font-medium text-text">{state.currentProject?.title || '未命名作品'}</h1>
            <p className="text-sm text-text-sub">{state.currentProject?.author || '未署名作者'} · 创意构建中</p>
          </div>
        </div>
        <div className="flex items-center gap-4">
          <StatusBadge variant="info">创意构建中</StatusBadge>
          <div className="flex items-center gap-2 text-sm text-text-sub bg-surface-hover px-3 py-1.5 rounded-md">
            <Lightbulb className="w-4 h-4 text-warning" />
            {entryPreview.kind === 'author_plan' ? '已有方案先检查缺口，再复用已提供的内容。' : nextHint}
          </div>
        </div>
      </header>

      {!getActiveModel(state.settings).apiKeyConfigured && (
        <div className="px-8 py-3 bg-warning/5 border-b border-warning/15 flex items-center gap-3 shrink-0">
          <AlertCircle className="w-4 h-4 text-warning" />
          <p className="text-sm text-text-sub flex-1">{entryPreview.kind === 'author_plan' ? '已有方案解析尚未接入此浏览器原型。' : '尚未配置 AI 模型，方向卡仅展示示例内容。'}</p>
          <Button size="sm" onClick={() => navigate('/settings')}>前往配置</Button>
        </div>
      )}

      <div className="flex-1 flex overflow-hidden">
        {/* Main creative area */}
        <main className="flex-1 overflow-auto p-8">
          {sampleCard && entryPreview.kind === 'idea' && <SamplePanel preview directionCandidateId="prototype-direction" directionCardId={sampleCard} version={1} runVersion={0} sample={samplePreview} onSave={async (value) => {setSamplePreview({...value,id:String(Date.now()),status:'prepared'});}} />}
          <CandidateFields editor={fieldDemo} preview onSave={async(_,changes)=>setFieldDemo(old=>({...old,candidateId:`demo-${old.version+1}`,version:old.version+1,fields:old.fields.map(f=>({...f,value:changes[f.path]??f.value}))}))} onLock={async(_,path,locked)=>setFieldDemo(old=>({...old,fields:old.fields.map(f=>f.path===path?{...f,locked}:f)}))}/>
          <EntryPanel entry={entryPreview} contentVersion={1} runSlotVersion={0} onSave={async (kind, material) => { setEntryPreview({ id: String(Date.now()), kind, material }); }} />
          {entryPreview.kind === 'author_plan' && <div className="mb-6 rounded-xl border border-border bg-surface p-4 text-sm text-text-sub">已有方案入口预览：方向不适用，保存材料后进入缺口检查与候选复用。此浏览器原型未接模型解析，真实调用未就绪。</div>}
          {entryPreview.kind !== 'author_plan' && <>

          {mode === 'brainstorm' ? (
            <div className="max-w-5xl mx-auto space-y-8">
              {/* Natural language input */}
              <section className="space-y-4">
                <label className="text-sm font-medium text-text">创作输入</label>
                <div className="bg-surface border border-border rounded-xl p-4 shadow-sm focus-within:border-border-strong transition-colors">
                  <Textarea
                    value={input}
                    onChange={(e) => setInput(e.target.value)}
                    placeholder="输入一句想法、题材、氛围、人物灵感、目标篇幅或零散设定…例如：民国江心岛，一座总在大雾天熄灭的灯塔，三任守塔人失踪。"
                    className="min-h-[120px] bg-transparent border-0 shadow-none focus:ring-0 text-base leading-relaxed placeholder:text-text-muted"
                  />
                  <div className="flex items-center justify-between mt-3 pt-3 border-t border-border">
                    <p className="text-xs text-text-muted">
                      AI 不会直接写入作品档案，所有输出均为候选。
                    </p>
                    <Button variant="primary" onClick={handleSend} className="gap-2">
                      <Send className="w-4 h-4" />
                      发送给 AI 团队
                    </Button>
                  </div>
                </div>
              </section>

              {/* Direction cards */}
              <section>
                <div className="flex items-center justify-between mb-4">
                  <h2 className="text-base font-medium text-text">故事方向候选</h2>
                  <div className="flex items-center gap-2">
                    {keptCount >= 2 && (
                      <Button variant="default" size="sm" onClick={combineDirections} className="gap-1">
                        <GitMerge className="w-4 h-4" />
                        组合 {keptCount} 个保留方向
                      </Button>
                    )}
                    <Button variant="ghost" size="sm" onClick={() => setCompareOpen(true)} className="gap-1">
                      比较
                    </Button>
                  </div>
                </div>

                <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
                  {directions.map((dir, idx) => (
                    <Card
                      key={dir.id}
                      padding="none"
                      className={clsx(
                        'relative overflow-hidden transition-all',
                        dir.kept ? 'ring-1 ring-warning' : 'ring-0'
                      )}
                    >
                      <div className="p-5">
                        <div className="flex items-center justify-between mb-4">
                          <span className="text-xs font-medium text-text-sub uppercase tracking-wider">
                            方向 {String.fromCharCode(65 + idx)}
                          </span>
                          <button
                            onClick={() => toggleKeep(dir.id)}
                            className={clsx(
                              'p-1.5 rounded-md transition-colors',
                              dir.kept
                                ? 'bg-warning/10 text-warning'
                                : 'text-text-muted hover:text-text hover:bg-surface-hover'
                            )}
                            title={dir.kept ? '取消保留' : '保留'}
                          >
                            <Bookmark className={clsx('w-4 h-4', dir.kept && 'fill-current')} />
                          </button>
                        </div>

                        <div className="space-y-3">
                          {Object.entries(dir.fields).map(([key, value]) =>
                            editingDirection === `${dir.id}-${key}` ? (
                              <div key={key} className="space-y-1">
                                <p className="text-xs text-text-sub">{key}</p>
                                <Textarea
                                  value={value}
                                  onChange={(e) =>
                                    setDirections((prev) =>
                                      prev.map((d) =>
                                        d.id === dir.id
                                          ? { ...d, fields: { ...d.fields, [key]: e.target.value } }
                                          : d
                                      )
                                    )
                                  }
                                  className="min-h-[60px] text-sm"
                                  onBlur={() => setEditingDirection(null)}
                                  autoFocus
                                />
                              </div>
                            ) : (
                              <div
                                key={key}
                                className="group cursor-pointer"
                                onClick={() => setEditingDirection(`${dir.id}-${key}`)}
                              >
                                <p className="text-xs text-text-sub mb-0.5">{key}</p>
                                <p className="text-sm text-text leading-relaxed group-hover:text-primary transition-colors">
                                  {value}
                                </p>
                              </div>
                            )
                          )}
                        </div>
                      </div>

                      <div className="px-5 py-3 bg-surface-hover border-t border-border flex items-center gap-2">
                        <Button
                          variant="primary"
                          size="sm"
                          onClick={() => selectDirection(dir.id)}
                          className="flex-1"
                        >
                          选择此方向
                        </Button><Button size="sm" onClick={() => {setSampleCard(dir.id);setSamplePreview(null);}}>选定并准备试写</Button>
                        <Button variant="ghost" size="sm" onClick={() => redoDirection(dir.id)}>
                          <RefreshCw className="w-4 h-4" />
                        </Button>
                      </div>
                    </Card>
                  ))}
                </div>
              </section>
            </div>
          ) : (
            <div className="max-w-4xl mx-auto space-y-6">
              <div className="flex items-center justify-between">
                <Button variant="ghost" size="sm" onClick={() => setMode('brainstorm')} className="gap-1">
                  <ArrowLeft className="w-4 h-4" />
                  返回方向选择
                </Button>
                <div className="text-sm text-text-sub">
                  已确认 {acceptedCount} / {draftSections.length} 项
                </div>
              </div>

              <div className="p-4 bg-primary-subtle border border-primary/10 rounded-xl">
                <div className="flex items-start gap-3">
                  <Sparkles className="w-5 h-5 text-primary shrink-0 mt-0.5" />
                  <div>
                    <p className="text-sm font-medium text-text">已选择：方向 {String.fromCharCode(65 + directions.findIndex((d) => d.selected))}</p>
                    <p className="text-sm text-text-sub mt-1">
                      以下是基于该方向生成的《作品方案》结构化草稿。你可以直接编辑每一部分，锁定不变动的内容，或让 AI 修改；只有点击「接受为正式方案」后，才会进入作品档案。
                    </p>
                  </div>
                </div>
              </div>

              <div className="space-y-4">
                {draftSections.map((section) => {
                  const Icon = section.icon;
                  return (
                    <Card key={section.id} padding="none" className={clsx('overflow-hidden', section.accepted && 'ring-1 ring-success')}>
                      <div className="px-5 py-3 border-b border-border bg-surface-hover flex items-center justify-between">
                        <div className="flex items-center gap-2.5">
                          <Icon className={clsx('w-4 h-4', section.accepted ? 'text-success' : 'text-text-sub')} />
                          <h3 className="text-sm font-medium text-text">{section.title}</h3>
                          {section.locked && <Lock className="w-3.5 h-3.5 text-text-muted" />}
                          {section.accepted && (
                            <span className="text-xs text-success bg-success/10 px-1.5 py-0.5 rounded">已接受</span>
                          )}
                        </div>
                        <div className="flex items-center gap-1">
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => setEditingSection(section.id)}
                            className="px-2"
                          >
                            <Edit3 className="w-4 h-4" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => toggleLock(section.id)}
                            className="px-2"
                            title={section.locked ? '解锁' : '锁定'}
                          >
                            <Lock className={clsx('w-4 h-4', section.locked && 'fill-current text-primary')} />
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => aiModifySection(section.id)}
                            className="px-2"
                          >
                            <Sparkles className="w-4 h-4" />
                          </Button>
                          {!section.accepted && (
                            <Button
                              variant="primary"
                              size="sm"
                              onClick={() => acceptSection(section.id)}
                              className="gap-1"
                            >
                              <Check className="w-3.5 h-3.5" />
                              接受
                            </Button>
                          )}
                        </div>
                      </div>
                      <div className="p-5">
                        {editingSection === section.id ? (
                          <Textarea
                            value={section.content}
                            onChange={(e) => updateDraft(section.id, e.target.value)}
                            className="min-h-[120px]"
                            autoFocus
                          />
                        ) : (
                          <p
                            className="text-sm text-text leading-relaxed whitespace-pre-wrap"
                            onClick={() => setEditingSection(section.id)}
                          >
                            {section.content}
                          </p>
                        )}
                        {editingSection === section.id && (
                          <div className="flex justify-end mt-3">
                            <Button size="sm" onClick={() => setEditingSection(null)}>完成编辑</Button>
                          </div>
                        )}
                      </div>
                    </Card>
                  );
                })}
              </div>

              {acceptedCount === draftSections.length && (
                <div className="flex justify-center pb-8">
                  <Button variant="primary" size="lg" onClick={() => navigate('/archive/outline')} className="gap-2">
                    进入作品档案
                    <ChevronRight className="w-4 h-4" />
                  </Button>
                </div>
              )}
            </div>
          )}
        </>}
</main>

        {/* Right creative context */}
        <aside className="w-[280px] shrink-0 border-l border-border bg-surface p-5 overflow-auto">
          <div className="mb-5 space-y-2"><p className="text-xs text-text-muted">进度展示示例 · 未调用模型</p><CreationProgress createdAt={progressStarted} updatedAt={progressDemo === 'running' ? undefined : new Date().toISOString()} status={progressDemo} stage="story_planning" steps={[{stage:'context_preparing',status:'completed'},{stage:'story_planning',status:progressDemo === 'running' ? 'running' : 'cancelled'},{stage:'independent_review',status:'queued'}]} onCancel={async()=>setProgressDemo('cancelled')} /><Button size="sm" variant="ghost" onClick={()=>{setProgressStarted(new Date().toISOString());setProgressDemo('running');}}>重看进度示例</Button></div><h3 className="text-sm font-medium text-text mb-4">创作上下文</h3>

          <div className="space-y-5">
            <ContextList
              title="已锁定"
              icon={Lock}
              items={context.locked}
              color="text-primary"
              onAdd={(v) => addContextItem('locked', v)}
            />
            <ContextList
              title="当前偏好"
              icon={Sparkles}
              items={context.preferences}
              color="text-warning"
              onAdd={(v) => addContextItem('preferences', v)}
            />
            <ContextList
              title="尚未决定"
              icon={AlertCircle}
              items={context.undecided}
              color="text-info"
              onAdd={(v) => addContextItem('undecided', v)}
            />
          </div>

          <div className="mt-6 p-3 bg-surface-hover border border-border rounded-lg">
            <p className="text-xs text-text-sub leading-relaxed">
              这些上下文会随你发送的输入自动累积。AI 在生成方向时会参考它们，但不会覆盖你已锁定的内容。
            </p>
          </div>
        </aside>
      </div>

      {/* Compare modal */}
      <Modal open={compareOpen} onClose={() => setCompareOpen(false)} title="比较故事方向">
        <div className="space-y-4 max-h-[70vh] overflow-auto pr-1">
          {directions.map((dir, idx) => (
            <div key={dir.id} className="p-4 border border-border rounded-lg bg-surface-hover">
              <p className="text-sm font-medium text-text mb-2">方向 {String.fromCharCode(65 + idx)}</p>
              <dl className="space-y-1.5 text-sm">
                {Object.entries(dir.fields).map(([key, value]) => (
                  <div key={key} className="flex gap-2">
                    <dt className="text-text-sub shrink-0 w-16">{key}</dt>
                    <dd className="text-text">{value}</dd>
                  </div>
                ))}
              </dl>
            </div>
          ))}
        </div>
      </Modal>
    </div>
  );
}

function ContextList({
  title,
  icon: Icon,
  items,
  color,
  onAdd,
}: {
  title: string;
  icon: typeof Lock;
  items: string[];
  color: string;
  onAdd: (value: string) => void;
}) {
  const [adding, setAdding] = useState(false);
  const [value, setValue] = useState('');

  const submit = () => {
    onAdd(value);
    setValue('');
    setAdding(false);
  };

  return (
    <div>
      <div className="flex items-center gap-2 mb-2">
        <Icon className={clsx('w-3.5 h-3.5', color)} />
        <h4 className="text-xs font-medium text-text-sub uppercase tracking-wider">{title}</h4>
      </div>
      <ul className="space-y-1.5">
        {items.map((item, idx) => (
          <li
            key={idx}
            className="text-sm text-text bg-surface-elevated border border-border rounded-md px-2.5 py-1.5"
          >
            {item}
          </li>
        ))}
        {adding ? (
          <li className="flex gap-1">
            <input
              value={value}
              onChange={(e) => setValue(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && submit()}
              placeholder="添加…"
              className="flex-1 min-w-0 px-2 py-1 text-sm bg-surface-elevated border border-border rounded-md focus-ring outline-none"
              autoFocus
            />
            <Button size="sm" onClick={submit} className="px-2">
              <Check className="w-3.5 h-3.5" />
            </Button>
          </li>
        ) : (
          <li>
            <button
              onClick={() => setAdding(true)}
              className="text-xs text-text-muted hover:text-text flex items-center gap-1 py-1"
            >
              + 添加
            </button>
          </li>
        )}
      </ul>
    </div>
  );
}
