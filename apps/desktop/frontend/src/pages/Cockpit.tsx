import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { clsx } from 'clsx';
import {
  AlertCircle,
  Ban,
  Bookmark,
  ChevronRight,
  Compass,
  GitBranch,
  Globe,
  Layers,
  Lightbulb,
  Lock,
  Map,
  RefreshCw,
  Send,
  Sparkles,
  Target,
  User,
} from 'lucide-react';
import { studio, StudioClientError, type CreationCostPreview, type CreationWorkbenchView, type RoadmapNodeView } from '../api/studio';
import { roadmapNavItems, RoadmapNavigation } from '../components/creation/RoadmapNavigation';
import { Button } from '../components/ui/Button';
import { Card } from '../components/ui/Card';
import { Textarea } from '../components/ui/Input';
import { Modal } from '../components/ui/Modal';
import { StatusBadge } from '../components/ui/StatusBadge';
import { SamplePanel } from '../components/creation/SamplePanel';
import { CreationProgress } from '../components/creation/CreationProgress';
import { RevisionPanel } from '../components/creation/RevisionPanel';
import { CandidateFields } from '../components/creation/CandidateFields';
import { CoCreationPanel } from '../components/creation/CoCreationPanel';
import { EntryPanel } from '../components/creation/EntryPanel';
import { useApp } from '../store/useApp';

const emptyWorkbench = (): CreationWorkbenchView => ({
  sessionId: '',
  projectId: '',
  branchId: '',
  stage: 'intent',
  advanceMode: 'step_review',
  blockingReason: '',
  recoverable: false,
  archiveConfirmed: false,
  intentText: '',
  stageRail: [],
  directionSet: { candidateId: '', version: 0, selectedId: '', status: '', cards: [] },
  proposal: { candidateId: '', version: 0, status: '', title: '', sections: [] },
  peopleWorld: { candidateId: '', version: 0, status: '', entities: [] },
  roadmap: {
    candidateId: '', version: 0, status: '',
    book: { id: '', title: '', summary: '', position: 0 },
    volumes: [], arcs: [], chapters: [],
  },
  currentRun: { runId: '', status: '', cancellable: false, publicStage: '' },
  assistant: { pendingActionCount: 0 },
  impact: {
    items: [], touchesLocked: false, touchesConfirmed: false, canAdvance: true,
    regenerateStages: [], estimatedCalls: 0, evidence: '',
  },
  capabilities: [],
});

const sectionIcons = {
  position: Target,
  character: User,
  world: Globe,
  mainplot: Map,
  subplot: GitBranch,
  ending: Layers,
  forbidden: Ban,
};

function generateStage(stage: string): string {
  if (stage === 'intent' || stage === 'directions') return 'directions';
  return stage;
}

function previousStage(stage: string): string | null {
  const rail = ['intent', 'directions', 'proposal', 'people_world', 'roadmap', 'archive_confirm'];
  const index = rail.indexOf(stage);
  return index > 0 ? rail[index - 1] : null;
}


function RoadmapGoalBlock({ node }: { node: RoadmapNodeView }) {
  const goals: Array<[string, string]> = [
    ['读者期待', node.readerExpectation ?? ''],
    ['阶段回报', node.stagePayoff ?? ''],
    ['不可逆变化', node.irreversibleChange ?? ''],
    ['终局贡献', node.endingContribution ?? ''],
  ];
  const visible = goals.filter(([, value]) => value.trim());
  if (!visible.length && !node.commitments?.length && !node.retainedQuestions?.length) return null;
  return (
    <div className="mt-3 space-y-3">
      {visible.map(([label, value]) => (
        <div key={label}>
          <p className="text-xs text-text-sub">{label}</p>
          <p className="text-sm text-text leading-relaxed">{value}</p>
        </div>
      ))}
      {(node.commitments ?? []).map((item) => (
        <div key={`cmt-${item.id}`}>
          <p className="text-xs text-text-sub">待兑现承诺</p>
          <p className="text-sm text-text leading-relaxed">{item.statement}</p>
          {item.detail && <p className="text-xs text-text-muted mt-1">兑现范围：{item.detail}</p>}
        </div>
      ))}
      {(node.retainedQuestions ?? []).map((item) => (
        <div key={`q-${item.id}`}>
          <p className="text-xs text-text-sub">保留悬念</p>
          <p className="text-sm text-text leading-relaxed">{item.statement}</p>
          {item.detail && <p className="text-xs text-text-muted mt-1">{item.detail}</p>}
        </div>
      ))}
    </div>
  );
}

export default function Cockpit() {
  const navigate = useNavigate();
  const { state, dispatch } = useApp();
  const [workbench, setWorkbench] = useState<CreationWorkbenchView>(emptyWorkbench);
  const [intent, setIntent] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [materialDirty, setMaterialDirty] = useState(false);
  const [fieldDirty,setFieldDirty] = useState(false);
  const [generating, setGenerating] = useState(false);
  const [preview, setPreview] = useState<CreationCostPreview | null>(null);
  const [compareOpen, setCompareOpen] = useState(false);
  const awaitingRun = generating && !['queued','pending','running','cancelling'].includes(workbench.currentRun.status);
  const keptIds = workbench.directionSet.keptIds ?? [];
  const [entityId, setEntityId] = useState<string | null>(null);
  const intentRef = useRef(intent);
  intentRef.current = intent;
  const savedIntent = useRef('');
  const projectId = state.backendProject?.id ?? '';
  const currentProjectId = useRef(projectId);
  currentProjectId.current = projectId;

  const modelReady = (state.bootstrap?.providers ?? []).some((provider) => provider.hasCredential);
  const projectTitle = state.backendProject?.title || state.currentProject?.title || '未命名作品';

  const load = useCallback(async (recover = false) => {
    const view = recover ? await studio.recoverCreation() : await studio.creationWorkbench();
    if (currentProjectId.current !== projectId) return view;
    setWorkbench(view);
    if (recover || intentRef.current === savedIntent.current) setIntent(view.intentText);
    savedIntent.current = view.intentText;
    setGenerating((['queued', 'pending', 'running', 'cancelling'].includes(view.currentRun.status) && !view.recoverable) || !!view.revisions?.some(action => action.state === 'executing') || !!(view.assistant.messages ?? []).some(message => message.kind === 'discussion' && (message.status === 'pending' || message.status === 'running')));
    return view;
  }, [projectId]);

  useEffect(() => {
    let active = true;
    void (async () => {
      try {
        await load(true);
        if (!active) return;
        setError(null);
      } catch (err) {
        if (!active) return;
        setError(err instanceof Error ? err.message : '无法加载创作工作台');
      }
    })();
    return () => { active = false; };
  }, [load]);

  useEffect(() => {
    if (!generating) return;
    let stopped = false;
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => {
      try { await load(false); }
      catch (err) { if (!stopped) setError(err instanceof Error ? err.message : '运行状态读取失败'); }
      if (!stopped) timer = setTimeout(() => void poll(), 750);
    };
    timer = setTimeout(() => void poll(), 750);
    return () => { stopped = true; clearTimeout(timer); };
  }, [generating, load]);

  const run = async (action: () => Promise<CreationWorkbenchView>) => {
    setBusy(true);
    setError(null);
    try {
      const view = await action();
      setWorkbench(view);
      setIntent(view.intentText);
      savedIntent.current = view.intentText;
      return view;
    } catch (err) {
      const message = err instanceof StudioClientError ? err.message : err instanceof Error ? err.message : '操作失败';
      setError(message);
      throw err;
    } finally {
      setBusy(false);
    }
  };

  const startGenerate = async (requestedStage?: string) => {
    if (fieldDirty) { setError('请先保存文本修改，再继续生成或进入下一阶段。'); return; }
    if (materialDirty) { setError("请先保存已修改的材料与起点，再开始解析或生成。"); return; }
    if (!modelReady) {
      setError('尚未配置 AI 模型，无法生成方向候选。请先完成模型连接。');
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const next = await studio.previewCreationGenerate(requestedStage ?? (workbench.entry?.kind === 'author_plan' ? workbench.stage : generateStage(workbench.stage)));
      setPreview(next);
    } catch (err) {
      setError(err instanceof Error ? err.message : '费用预览失败');
    } finally {
      setBusy(false);
    }
  };

  const confirmGenerate = async () => {
    if (!preview) return;
    setPreview(null);
    setGenerating(true);
    setError(null);
    try {
      await studio.confirmCreationGenerate({
        stage: preview.stage,
        confirmationDigest: preview.confirmationDigest,
        idempotencyKey: `creation-${Date.now()}`,
      });
      await load(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : '生成失败');
      setGenerating(false);
    }
  };

  const cancelRun = async () => {
    await studio.cancelCreationRun();
    await load(false);
  };

  const confirmNext = async (cardId?: string) => {
    if (fieldDirty) { setError('请先保存文本修改，再继续生成或进入下一阶段。'); return; }
    if (materialDirty) { setError("请先保存材料，再审阅下一阶段。"); return; }
    if (workbench.stage === 'archive_confirm') {
      const archive = await studio.previewCreationArchive();
      await studio.confirmCreationArchive({
        confirmationDigest: archive.confirmationDigest,
        idempotencyKey: `archive-${Date.now()}`,
      });
      const workbenchState = await studio.projectWorkbench();
      if (state.backendProject) dispatch({ type: 'SET_BACKEND_PROJECT', payload: { ...state.backendProject, workbench: workbenchState } });
      navigate('/archive/outline');
      return;
    }
    if (workbench.stage === 'directions' && workbench.directionSet.candidateId) {
      const selectedId = cardId ?? workbench.directionSet.selectedId;
      if (!selectedId) { setError('请先选择一张故事方向卡。'); return; }
      await run(() => studio.selectCreationDirection(workbench.directionSet.candidateId, selectedId));
    }
    const candidateId = workbench.stage === 'proposal'
      ? workbench.proposal.candidateId
      : workbench.stage === 'people_world'
        ? workbench.peopleWorld.candidateId
        : workbench.stage === 'roadmap'
          ? workbench.roadmap.candidateId
          : '';
    if (candidateId) await run(() => studio.acceptCreationCandidate(candidateId));
    await run(() => studio.advanceCreationStage());
  };

  const hint = useMemo(() => {
    if (generating) return '正在生成候选，完成后不会自动跳转正文。';
    if (workbench.recoverable) return '检测到未完成的付费运行，已暂停，不会自动续跑。';
    if (workbench.impact.items.length > 0) return workbench.impact.evidence || '上游修改影响了下游候选。';
    if (workbench.stage === 'intent') return '输入你的想法，确认费用后生成三套故事方向。';
    if (workbench.stage === 'directions') return '比较、保留或选择一套方向，再进入作品方案。';
    return '直接编辑、锁定后确认进入下一步；确认档案前不会写入正式资产。';
  }, [generating, workbench]);

  const cards = workbench.directionSet.cards;
  const selectedEntity = workbench.peopleWorld.entities.find((item) => item.id === entityId) ?? workbench.peopleWorld.entities[0];
  const roadmapItems = roadmapNavItems(workbench.roadmap);
  const selectedRoadmap = roadmapItems.find((item) => item.id === entityId) ?? roadmapItems.find((item) => item.kind === 'arc') ?? roadmapItems[0];

  return (
    <div className="h-full flex flex-col animate-fade-up -m-6">
      <header className="px-8 py-5 border-b border-border bg-surface flex items-center justify-between shrink-0">
        <div className="flex items-center gap-4">
          <div className="w-10 h-10 rounded-xl bg-primary-subtle border border-primary/10 flex items-center justify-center">
            <Compass className="w-5 h-5 text-primary" />
          </div>
          <div>
            <h1 className="text-lg font-medium text-text">{projectTitle}</h1>
            <p className="text-sm text-text-sub">作品构建工作台</p>
          </div>
        </div>
        <div className="flex items-center gap-3">
          <StatusBadge variant={workbench.blockingReason ? 'warning' : 'info'}>
            {workbench.blockingReason ? `已暂停：${workbench.blockingReason}` : '创意构建中'}
          </StatusBadge>
          <div className="flex items-center gap-2 text-sm text-text-sub bg-surface-hover px-3 py-1.5 rounded-md max-w-xl">
            <Lightbulb className="w-4 h-4 text-warning shrink-0" />
            {hint}
          </div>
        </div>
      </header>

      <nav className="px-8 py-3 border-b border-border bg-surface flex gap-2 overflow-x-auto" aria-label="创作阶段轨">
        {workbench.stageRail.map((stop) => (
          <div
            key={stop.id}
            data-testid={`rail-${stop.id}`}
            className={clsx(
              'px-3 py-1.5 rounded-full text-xs border whitespace-nowrap',
              stop.status === 'current' && 'bg-primary text-white border-primary',
              stop.status === 'confirmed' && 'bg-success/10 text-success border-success/20',
              stop.status === 'blocked' && 'bg-warning/10 text-warning border-warning/20',
              ['upcoming', 'not_applicable'].includes(stop.status) && 'text-text-muted border-border',
            )}
          >
            {stop.label}{stop.status === 'not_applicable' ? ' · 不适用' : ''}
          </div>
        ))}
      </nav>

      {!modelReady && (
        <div data-testid="model-missing-notice" className="px-8 py-3 bg-warning/5 border-b border-warning/15 flex items-center gap-3 shrink-0">
          <AlertCircle className="w-4 h-4 text-warning" />
          <p className="text-sm text-text-sub flex-1">尚未配置 AI 模型，不会展示示例方向。请先完成模型连接后再生成。</p>
          <Button size="sm" onClick={() => navigate('/settings')}>前往配置</Button>
        </div>
      )}

      {workbench.recoverable && (
        <div data-testid="recovery-notice" className="px-8 py-3 bg-info/5 border-b border-info/15 text-sm text-text-sub">
          恢复后不会自动继续未确认的付费调用。需要时请重新预览费用并确认生成。
        </div>
      )}

      {error && (
        <div role="alert" className="px-8 py-3 bg-danger/5 border-b border-danger/15 text-sm text-danger">{error}</div>
      )}

      {workbench.impact.items.filter((item) => !item.excepted).length > 0 && (
        <div className="px-8 py-3 border-b border-border flex items-center gap-2 flex-wrap">
          <p className="text-sm text-text-sub flex-1">{workbench.impact.evidence || '下游候选已过期。'}</p>
          <Button size="sm" onClick={() => void run(() => studio.resolveCreationImpact('keep_exception').then(() => studio.creationWorkbench()))}>保留为例外</Button>
          <Button size="sm" onClick={() => void run(() => studio.resolveCreationImpact('abandon_upstream').then(() => studio.creationWorkbench()))}>放弃上游修改</Button>
          <Button size="sm" variant="ghost" disabled>局部重算（尚未自动开跑）</Button>
        </div>
      )}

      <div className="flex-1 flex overflow-hidden">
        {(workbench.stage === 'people_world' || workbench.stage === 'roadmap') && (
          <aside className="w-[220px] shrink-0 border-r border-border bg-surface p-4 overflow-auto">
            <h3 className="text-xs font-medium text-text-sub uppercase tracking-wider mb-3">
              {workbench.stage === 'people_world' ? '人物与世界' : '路线图'}
            </h3>
            {workbench.stage === 'people_world' && workbench.peopleWorld.entities.map((entity) => (
              <button
                key={entity.id}
                className={clsx('w-full text-left px-2 py-1.5 rounded-md text-sm mb-1', selectedEntity?.id === entity.id ? 'bg-primary-subtle text-primary' : 'text-text-sub hover:bg-surface-hover')}
                onClick={() => setEntityId(entity.id)}
              >
                {entity.title}
              </button>
            ))}
            {workbench.stage === 'roadmap' && <RoadmapNavigation items={roadmapItems} selectedId={selectedRoadmap?.id} onSelect={setEntityId} />}
          </aside>
        )}

        <main className="flex-1 overflow-auto p-8">
          {(workbench.fieldEditor?.fields.length || workbench.revisions?.length) ? <RevisionPanel key={`revision:${workbench.sessionId}`} editor={workbench.fieldEditor} actions={workbench.revisions ?? []} disabled={busy||generating||workbench.readOnly||fieldDirty||materialDirty} onPreview={async(candidateId,path,instruction,key)=>{await studio.previewCreationRevision({candidateId,paths:[path],instruction,idempotencyKey:key});await load();}} onConfirm={async(action)=>{await studio.confirmCreationRevision({actionId:action.actionId,argumentsHash:action.argumentsHash,idempotencyKey:action.idempotencyKey});await load();}} onReject={async(id)=>{await run(()=>studio.rejectCreationRevision(id));}} onApply={async(action)=>{await run(()=>studio.applyCreationRevision({actionId:action.actionId,argumentsHash:action.argumentsHash,idempotencyKey:action.idempotencyKey}));}}/> : null}
          {workbench.fieldEditor?.fields.length ? <CandidateFields onDirtyChange={setFieldDirty} key={`fields:${workbench.sessionId}`} editor={workbench.fieldEditor} disabled={busy||generating||workbench.readOnly||materialDirty} onSave={async(candidateId,changes)=>{await run(()=>studio.saveCreationFields({candidateId,changes}));}} onLock={async(candidateId,path,locked)=>{await run(()=>(locked?studio.lockCreationField:studio.unlockCreationField)(candidateId,path));}}/> : null}
          {(workbench.canPrepareSample || workbench.sampleHandoff) && <SamplePanel key={`sample:${workbench.sessionId}`} sample={workbench.sampleHandoff} version={workbench.contentVersion ?? 0} runVersion={workbench.runSlotVersion ?? 0} directionCandidateId={workbench.directionSet.candidateId} directionCardId={workbench.directionSet.selectedId} disabled={!workbench.canPrepareSample || busy || generating || workbench.readOnly || materialDirty} onSave={async (value) => { await run(() => studio.saveCreationSample({ ...value, projectId: workbench.projectId, sessionId: workbench.sessionId })); }} />}
          {workbench.sessionId && <EntryPanel key={`entry:${workbench.sessionId}`} onDirtyChange={setMaterialDirty} entry={workbench.entry} contentVersion={workbench.contentVersion ?? 0} runSlotVersion={workbench.runSlotVersion ?? 0} disabled={busy || generating || workbench.readOnly || fieldDirty} onSave={async (kind, material, expectedVersion, expectedRunSlotVersion) => {
            await run(() => studio.saveCreationEntry({ projectId: workbench.projectId, sessionId: workbench.sessionId, kind, material, expectedVersion, expectedRunSlotVersion }));
          }} />}
          {workbench.entry?.kind === 'author_plan' && <section className="max-w-5xl mx-auto mb-6 space-y-4" aria-label="已有方案分析">
            <h2 className="text-base font-medium">已有方案 · 缺口与复用</h2>
            <p className="text-sm text-text-sub">方向步骤不适用。解析不会修改正式档案；完整内容可直接复用为待审候选。</p>
            {workbench.stage === 'intent' && <Button disabled={materialDirty || busy || generating || workbench.readOnly} onClick={() => void startGenerate('intent')}>解析已有方案与检查缺口</Button>}
            {workbench.material?.status === 'needs_review' && <p role="status" className="text-sm text-warning">分析结果已过期、被阻断或来源无法核实，请重新解析。</p>}
            {workbench.material?.status === 'ready' && <>
              <p className="text-sm text-text-sub">已完成结构与来源检查，仍需作者审阅。</p>
              {workbench.material.gaps.map((gap) => <div key={gap.id} className="rounded-lg border border-border p-3 text-sm">
                <p className="font-medium">{gap.kind === 'conflict' ? '冲突' : gap.kind === 'required' ? '必要缺口' : '可选建议'} · {({ proposal: '作品方案', people_world: '人物世界', roadmap: '路线图' } as Record<string, string>)[gap.stage]}</p>
                <p>{gap.reason}</p><p className="text-text-sub">{gap.suggested_action}</p>
                {gap.quote && <blockquote className="mt-2 border-l-2 border-border pl-2 text-text-sub">{gap.quote}</blockquote>}
              </div>)}
              {workbench.material.availableStages.includes(workbench.stage) && !({ proposal: workbench.proposal.candidateId, people_world: workbench.peopleWorld.candidateId, roadmap: workbench.roadmap.candidateId } as Record<string, string>)[workbench.stage] &&
                <Button variant="primary" disabled={materialDirty || busy || generating || workbench.readOnly} onClick={() => void run(() => studio.useCreationMaterial({ projectId: workbench.projectId, sessionId: workbench.sessionId, expectedVersion: workbench.contentVersion ?? 0, analysisId: workbench.material!.analysisId, stage: workbench.stage })).catch(() => undefined)}>复用已提供的本阶段内容</Button>}
            </>}
          </section>}

          {workbench.entry?.kind !== 'author_plan' && (workbench.stage === 'intent' || workbench.stage === 'directions') && (
            <div className="max-w-5xl mx-auto space-y-8">
              <section className="space-y-4">
                <label className="text-sm font-medium text-text" htmlFor="creation-intent">创作输入</label>
                <div className="bg-surface border border-border rounded-xl p-4 shadow-sm">
                  <Textarea
                    id="creation-intent"
                    value={intent}
                    onChange={(event) => setIntent(event.target.value)}
                    placeholder="输入一句想法、题材、氛围或人物灵感…"
                    className="min-h-[120px] bg-transparent border-0 shadow-none focus:ring-0 text-base leading-relaxed"
                    disabled={generating}
                  />
                  <div className="flex items-center justify-between mt-3 pt-3 border-t border-border">
                    <p className="text-xs text-text-muted">AI 不会直接写入作品档案，所有输出均为候选。</p>
                    <Button
                      variant="primary"
                      onClick={() => void (async () => {
                        if (intent.trim()) await run(() => studio.saveCreationIntent(intent.trim()));
                        await startGenerate();
                      })()}
                      disabled={generating || busy}
                      className="gap-2"
                    >
                      <Send className="w-4 h-4" />
                      发送给 AI 团队
                    </Button>
                  </div>
                </div>
              </section>

              <section>
                <div className="flex items-center justify-between mb-4">
                  <h2 className="text-base font-medium text-text">故事方向候选</h2>
                  <div className="flex items-center gap-2">
                    {keptIds.length >= 2 && (
                      <Button size="sm" disabled title="组合方向尚未就绪">组合 {keptIds.length} 个保留方向</Button>
                    )}
                    <Button variant="ghost" size="sm" onClick={() => setCompareOpen(true)} disabled={cards.length === 0}>比较</Button>
                  </div>
                </div>
                {cards.length === 0 ? (
                  <p className="text-sm text-text-sub">还没有方向候选。确认费用后才会向已配置的模型请求生成。</p>
                ) : (
                  <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
                    {cards.map((card, index) => (
                      <Card key={card.id} padding="none" className={clsx(keptIds.includes(card.id) && 'ring-1 ring-warning')}>
                        <div className="p-5 space-y-3">
                          <div className="flex items-center justify-between">
                            <span className="text-xs font-medium text-text-sub">方向 {String.fromCharCode(65 + index)}</span>
                            <button
                              className="p-1.5 rounded-md text-text-muted hover:bg-surface-hover"
                              aria-label={`${keptIds.includes(card.id) ? '取消保留' : '保留'}方向 ${String.fromCharCode(65 + index)}`}
                              disabled={busy || generating || workbench.readOnly}
                              onClick={() => void run(() => (keptIds.includes(card.id) ? studio.unkeepCreationDirection : studio.keepCreationDirection)(card.id)).catch(() => undefined)}
                            >
                              <Bookmark className={clsx('w-4 h-4', keptIds.includes(card.id) && 'fill-current text-warning')} />
                            </button>
                          </div>
                          {[
                            ['卖点', card.hook], ['类型', card.genre], ['主角', card.protagonist],
                            ['核心冲突', card.coreConflict], ['成长代价', card.growthCost], ['节奏', card.pacing], ['结局方向', card.ending],
                          ].map(([label, value]) => (
                            <div key={label}>
                              <p className="text-xs text-text-sub">{label}</p>
                              <p className="text-sm text-text leading-relaxed">{value}</p>
                            </div>
                          ))}
                        </div>
                        <div className="px-5 py-3 bg-surface-hover border-t border-border">
                          <Button variant="primary" size="sm" className="w-full" disabled={generating || busy || workbench.readOnly} onClick={() => void confirmNext(card.id).catch((err) => setError(err.message))}>
                            选择此方向
                          </Button>
                          <Button size="sm" className="w-full mt-2" disabled={generating || busy || workbench.readOnly || materialDirty} onClick={() => void run(() => studio.selectCreationDirection(workbench.directionSet.candidateId, card.id)).catch(() => undefined)}>选定并准备试写</Button>
                        </div>
                      </Card>
                    ))}
                  </div>
                )}
              </section>
            </div>
          )}

          {workbench.stage === 'proposal' && (
            <div className="max-w-4xl mx-auto space-y-4">
              <div className="p-4 bg-primary-subtle border border-primary/10 rounded-xl">
                <p className="text-sm font-medium text-text">作品方案{workbench.proposal.title ? `：${workbench.proposal.title}` : ''}</p>
                <p className="text-sm text-text-sub mt-1">确认档案前这些内容不会进入正式资产。</p>
              </div>
              {workbench.proposal.sections.map((section) => {
                const Icon = sectionIcons[section.id as keyof typeof sectionIcons] ?? Sparkles;
                return (
                  <Card key={section.id} padding="none">
                    <div className="px-5 py-3 border-b border-border bg-surface-hover flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <Icon className="w-4 h-4 text-text-sub" />
                        <h3 className="text-sm font-medium text-text">{section.title}</h3>
                        {section.locked && <Lock className="w-3.5 h-3.5 text-text-muted" />}
                      </div>
                      {section.lockPath && (
                        <Button size="sm" variant="ghost" onClick={() => void run(() => (section.locked ? studio.unlockCreationField : studio.lockCreationField)(workbench.proposal.candidateId, section.lockPath))}>
                          {section.locked ? '解锁' : '锁定'}
                        </Button>
                      )}
                    </div>
                    <p className="p-5 text-sm text-text leading-relaxed whitespace-pre-wrap">{section.content}</p>
                  </Card>
                );
              })}
            </div>
          )}

          {workbench.stage === 'people_world' && selectedEntity && (
            <Card className="max-w-3xl">
              <h2 className="text-base font-medium text-text mb-2">{selectedEntity.title}</h2>
              <p className="text-sm text-text-sub">{selectedEntity.summary}</p>
            </Card>
          )}

          {workbench.stage === 'roadmap' && selectedRoadmap && (
            <Card className="max-w-3xl">
              <p className="text-xs text-text-sub mb-1">{selectedRoadmap.kind === 'book' ? '全书' : selectedRoadmap.kind === 'volume' ? '卷' : selectedRoadmap.kind === 'arc' ? '故事弧' : '近期章节'}</p>
              <h2 className="text-base font-medium text-text">{selectedRoadmap.node.title || selectedRoadmap.title}</h2>
              <p className="text-sm text-text-sub mt-1">{selectedRoadmap.node.summary}</p>
              {selectedRoadmap.kind === 'arc' && <RoadmapGoalBlock node={selectedRoadmap.node} />}
            </Card>
          )}

          {workbench.stage === 'archive_confirm' && (
            <Card className="max-w-xl mx-auto text-center space-y-3">
              <h2 className="text-lg font-medium text-text">确认作品档案</h2>
              <p className="text-sm text-text-sub">确认后才会写入人物、世界规则和大纲等正式资产，并进入可写状态。</p>
              <Button variant="primary" onClick={() => void confirmNext()}>确认作品档案</Button>
            </Card>
          )}
        </main>

        <aside className="w-[340px] shrink-0 border-l border-border bg-surface flex flex-col overflow-hidden">
          {(generating || workbench.currentRun.runId) && <div className="p-4 border-b border-border"><CreationProgress status={awaitingRun ? 'queued' : workbench.currentRun.status} stage={awaitingRun ? undefined : workbench.currentRun.publicStage} steps={awaitingRun ? [] : workbench.currentRun.steps} createdAt={awaitingRun ? undefined : workbench.currentRun.createdAt} updatedAt={workbench.currentRun.updatedAt} onCancel={generating || workbench.currentRun.cancellable ? cancelRun : undefined} /></div>}
          <CoCreationPanel
            key={`cocreation:${workbench.projectId}:${workbench.sessionId}`}
            editor={workbench.fieldEditor}
            entityRef={selectedEntity?.id ?? selectedRoadmap?.id ?? ''}
            messages={(workbench.assistant.messages ?? []).map(message => ({
              ...message,
              actionState: workbench.revisions?.find(action => action.actionId === message.actionId)?.state ?? message.actionState,
            }))}
            authorization={workbench.assistant.authorization}
			authorizations={workbench.assistant.authorizations}
			pendingDiscussion={workbench.assistant.pendingDiscussion}
            disabled={busy || generating || workbench.readOnly || fieldDirty || materialDirty}
            onSend={async request => { await studio.sendCreationMessage(request); await load(); }}
            onPreviewDiscussion={async request => studio.previewCreationDiscussionAuthorization(request)}
            onConfirmDiscussion={async command => { await studio.confirmCreationDiscussionAuthorization(command); setGenerating(true); await load(); }}
            onSendDiscussion={async request => { await studio.sendCreationDiscussionMessage(request); setGenerating(true); await load(); }}
            onRevokeDiscussion={async id => { await studio.revokeCreationDiscussionAuthorization(id); await load(); }}
            onPreviewSuggestion={async (messageId, suggestionId) => { await studio.previewCreationDiscussionSuggestion({messageId, suggestionId}); await load(); }}
          />
        </aside>
      </div>

      <footer className="px-8 py-3 border-t border-border bg-surface flex items-center justify-between shrink-0">
        <Button
          variant="ghost"
          disabled={!previousStage(workbench.stage) || generating || fieldDirty}
          onClick={() => {
            const target = workbench.entry?.kind === 'author_plan' && workbench.stage === 'proposal' ? 'intent' : previousStage(workbench.stage);
            if (target) void run(() => studio.lookBackCreationStage(target));
          }}
        >
          上一步
        </Button>
        <div className="flex gap-2">
          <Button disabled={generating || busy} onClick={() => void startGenerate()} className="gap-1">
            <RefreshCw className="w-4 h-4" />{workbench.entry?.kind === 'author_plan' ? (workbench.stage === 'intent' ? '重新解析' : '补齐本阶段候选') : '重新生成'}
          </Button>
          {workbench.entry?.kind !== 'author_plan' && <Button disabled={generating || busy} onClick={() => void run(() => studio.saveCreationIntent(intent.trim() || workbench.intentText))}>保存草稿</Button>}
          <Button variant="primary" disabled={generating || busy} onClick={() => void confirmNext()} className="gap-1">
            {['proposal', 'people_world', 'roadmap'].includes(workbench.stage) ? '接受并进入下一步' : '确认进入下一步'} <ChevronRight className="w-4 h-4" />
          </Button>
        </div>
      </footer>

      <Modal open={Boolean(preview)} onClose={() => setPreview(null)} title="确认生成费用">
        {preview && (
          <div className="space-y-4 p-1">
            <p className="text-sm text-text-sub">预计调用 {preview.estimatedCalls} 次。确认后才会创建运行；运行开始后取消仍可能产生已发生的费用。</p>
            <p className="text-sm text-text-sub">Token 上限：{preview.runBudgetUnlimited ? '未设置' : preview.maxRunTokens}；金额估算未知，请核对模型价格。</p>
            {(preview.roleModels ?? []).map((model) => <p key={model.role} className="text-xs text-text-sub">{model.role}：{model.provider} / {model.model}</p>)}
            <div className="flex justify-end gap-2">
              <Button onClick={() => setPreview(null)}>取消</Button>
              <Button variant="primary" onClick={() => void confirmGenerate()}>确认生成</Button>
            </div>
          </div>
        )}
      </Modal>

      <Modal open={compareOpen} onClose={() => setCompareOpen(false)} title="比较故事方向">
        <div className="space-y-4 max-h-[70vh] overflow-auto">
          {cards.map((card, index) => (
            <div key={card.id} className="p-4 border border-border rounded-lg">
              <p className="text-sm font-medium text-text mb-2">方向 {String.fromCharCode(65 + index)}</p>
              <p className="text-sm text-text-sub">{card.hook}</p>
            </div>
          ))}
        </div>
      </Modal>
    </div>
  );
}
