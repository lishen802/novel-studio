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
import { studio, StudioClientError, type CreationCostPreview, type CreationWorkbenchView, type DirectionActionView, type RoadmapNodeView } from '../api/studio';
import { roadmapNavItems, RoadmapNavigation } from '../components/creation/RoadmapNavigation';
import { CandidateVersions } from '../components/creation/CandidateVersions';
import { ReviewDecisionButtons, ReviewStateBadge, reviewNavSuffix, type ReviewDecision } from '../components/creation/ReviewDecision';
import { Button } from '../components/ui/Button';
import { Card } from '../components/ui/Card';
import { Textarea } from '../components/ui/Input';
import { Modal } from '../components/ui/Modal';
import { StatusBadge } from '../components/ui/StatusBadge';
import { SamplePanel } from '../components/creation/SamplePanel';
import { CreationProgress } from '../components/creation/CreationProgress';
import { RevisionPanel } from '../components/creation/RevisionPanel';
import { CandidateFields } from '../components/creation/CandidateFields';
import { StructureEditor } from '../components/creation/StructureEditor';
import { CoCreationPanel } from '../components/creation/CoCreationPanel';
import { EntryPanel } from '../components/creation/EntryPanel';
import { useApp } from '../store/useApp';

const emptyFieldEditor = {candidateId:'',version:0,fields:[]};
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
  directionSet: { candidateId: '', selectedCandidateId: '', version: 0, selectedId: '', status: '', cards: [], combined: [] },
  proposal: { candidateId: '', version: 0, status: '', title: '', sections: [] },
  peopleWorld: { candidateId: '', version: 0, status: '', entities: [] },
  roadmap: {
    candidateId: '', version: 0, status: '',
    book: { id: '', title: '', summary: '', position: 0, accepted: false, rejected: false },
    volumes: [], arcs: [], chapters: [],
  },
  currentRun: { runId: '', status: '', cancellable: false, publicStage: '' },
  assistant: { pendingActionCount: 0 },
  impact: {
    items: [], touchesLocked: false, touchesConfirmed: false, canAdvance: true,
    regenerateStages: [], estimatedCalls: 0, evidence: '',
  },
  capabilities: [],
  structure: [],
  review: { candidateId: '', stage: '', reviewVersion: 0, accepted: [], rejected: [], unreviewed: [], complete: false, updatedAt: '' },
  reviewCursor: { stage: '', candidateId: '', entityId: '', sectionId: '', comparisonRef: '', viewVersion: 0, updatedAt: '' },
  candidateVersions: [],
  fieldEditor: { candidateId: '', version: 0, fields: [] },
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

// 当前阶段工作版本的候选 ID：逐项审阅、版本切换与版本比较都以它为目标。
function stageCandidateId(workbench: CreationWorkbenchView): string {
  switch (workbench.stage) {
    case 'proposal':
      return workbench.proposal.candidateId;
    case 'people_world':
      return workbench.peopleWorld.candidateId;
    case 'roadmap':
      return workbench.roadmap.candidateId;
    default:
      return '';
  }
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
  const { state, dispatch, registerBeforeClose } = useApp();
  const [workbench, setWorkbench] = useState<CreationWorkbenchView>(emptyWorkbench);
  const [intent, setIntent] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [materialDirty, setMaterialDirty] = useState(false);
  const [fieldDirty,setFieldDirty] = useState(false);
  const [generating, setGenerating] = useState(false);
  const [preview, setPreview] = useState<CreationCostPreview | null>(null);
  const [compareOpen, setCompareOpen] = useState(false);
  const [directionDialog, setDirectionDialog] = useState<{mode:'regenerate_one'|'combine';targetCardId?:string} | null>(null);
  const [directionInstruction, setDirectionInstruction] = useState('');
  const [directionPreview, setDirectionPreview] = useState<DirectionActionView | null>(null);
  const [dismissedDirectionActionId, setDismissedDirectionActionId] = useState('');
  const awaitingRun = generating && !['queued','pending','running','cancelling'].includes(workbench.currentRun.status);
  const keptIds = workbench.directionSet.keptIds ?? [];
  const [entityId, setEntityId] = useState<string | null>(null);
  const [sectionId, setSectionId] = useState<string | null>(null);
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
    setGenerating((['queued', 'pending', 'running', 'cancelling'].includes(view.currentRun.status) && !view.recoverable) || !!view.revisions?.some(action => action.state === 'executing') || !!view.directionActions?.some(action => action.state === 'executing') || !!(view.assistant.messages ?? []).some(message => message.kind === 'discussion' && (message.status === 'pending' || message.status === 'running')));
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
    const pending = workbench.directionActions?.find(action => action.state === 'pending_confirmation');
    if (pending && pending.actionId !== dismissedDirectionActionId && !directionPreview && !directionDialog) setDirectionPreview(pending);
  }, [workbench.directionActions, directionPreview, directionDialog, dismissedDirectionActionId]);

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

  // 审阅位置恢复：仅当游标落在当前阶段的当前候选上时套用一次，不覆盖作者随后的新选择。
  const restoredCursor = useRef('');
  const selectionContext = useRef('');
  useEffect(() => {
    const context = `${workbench.sessionId}:${workbench.stage}`;
    if (selectionContext.current !== context) {
      selectionContext.current = context;
      restoredCursor.current = '';
      setEntityId(null); setSectionId(null);
    }
    const cursor = workbench.reviewCursor;
    const candidateId = stageCandidateId(workbench);
    if (!cursor?.stage || cursor.stage !== workbench.stage || !candidateId || cursor.candidateId !== candidateId) return;
    const key = `${workbench.sessionId}:${workbench.stage}`;
    if (restoredCursor.current === key) return;
    restoredCursor.current = key;
    if (cursor.entityId) {
      setEntityId(cursor.entityId);
      setSectionId(null);
    } else if (cursor.sectionId) {
      setSectionId(cursor.sectionId);
      setEntityId(null);
    }
  }, [workbench]);

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

  const previewDirectionAction = async () => {
    if (!directionDialog) return;
    setBusy(true);
    setError(null);
    try {
      const idempotencyKey = `direction-${directionDialog.mode}-${Date.now()}`;
      const next = directionDialog.mode === 'combine'
        ? await studio.previewCombineDirections({ candidateId: workbench.directionSet.candidateId, sourceCardIds: keptIds, instruction: directionInstruction.trim(), idempotencyKey })
        : await studio.previewRegenerateDirection({ candidateId: workbench.directionSet.candidateId, targetCardId: directionDialog.targetCardId ?? '', instruction: directionInstruction.trim(), idempotencyKey });
      setDirectionDialog(null);
      setDismissedDirectionActionId('');
      setDirectionPreview(next);
    } catch (err) {
      setError(err instanceof Error ? err.message : '方向操作预览失败');
    } finally {
      setBusy(false);
    }
  };

  const confirmDirectionAction = async () => {
    if (!directionPreview) return;
    setBusy(true);
    setError(null);
    try {
      const command = { actionId: directionPreview.actionId, argumentsHash: directionPreview.argumentsHash, idempotencyKey: directionPreview.idempotencyKey };
      if (directionPreview.mode === 'combine') await studio.confirmCombineDirections(command);
      else await studio.confirmRegenerateDirection(command);
      setDirectionPreview(null);
      setDirectionInstruction('');
      setGenerating(true);
      await load(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : '方向操作启动失败');
    } finally {
      setBusy(false);
    }
  };

  const rejectDirectionAction = async () => {
    const pending = directionPreview;
    if (pending) setDismissedDirectionActionId(pending.actionId);
    setDirectionPreview(null);
    if (!pending || pending.state !== 'pending_confirmation') return;
    try {
      const view = await studio.rejectDirectionAction(pending.actionId);
      setWorkbench(view);
    } catch (err) {
      setError(err instanceof Error ? err.message : '取消方向操作失败');
    }
  };

  const [archivePreview, setArchivePreview] = useState<Awaited<ReturnType<typeof studio.previewCreationArchive>> | null>(null);
  const archiveKey = useRef('');
  const confirmArchive = async () => {
    if (!archivePreview || busy) return;
    setBusy(true); setError('');
    try {
      await studio.confirmCreationArchive({confirmationDigest:archivePreview.confirmationDigest,idempotencyKey:archiveKey.current});
      const workbenchState = await studio.projectWorkbench();
      if (state.backendProject) dispatch({type:'SET_BACKEND_PROJECT',payload:{...state.backendProject,workbench:workbenchState}});
      setArchivePreview(null); navigate('/archive/outline');
    } catch (err) { setError(err instanceof Error ? err.message : '入档失败，请重新核对预览。'); }
    finally { setBusy(false); }
  };
  const confirmNext = async (cardId?: string) => {
    if (fieldDirty) { setError('请先保存文本修改，再继续生成或进入下一阶段。'); return; }
    if (materialDirty) { setError("请先保存材料，再审阅下一阶段。"); return; }
    if (workbench.stage === 'archive_confirm') {
      if (busy) return;
      setBusy(true); setError('');
      try { setArchivePreview(await studio.previewCreationArchive()); archiveKey.current = crypto.randomUUID(); }
      catch (err) { setError(err instanceof Error ? err.message : '无法获取入档预览。'); }
      finally { setBusy(false); }
      return;
    }
    if (workbench.stage === 'directions' && workbench.directionSet.candidateId) {
      const selectedId = cardId ?? workbench.directionSet.selectedId;
      if (!selectedId) { setError('请先选择一张故事方向卡。'); return; }
      const selectedCandidateId = cardId ? workbench.directionSet.candidateId : (workbench.directionSet.selectedCandidateId || workbench.directionSet.candidateId);
      await run(() => studio.selectCreationDirection(selectedCandidateId, selectedId));
    }
    const candidateId = stageCandidateId(workbench);
    if (candidateId) await run(() => studio.acceptCreationCandidate(candidateId));
    await run(() => studio.advanceCreationStage());
  };

  // 逐项决定只改审阅元数据，接受、锁定与入档互不影响。
  const capabilities = workbench.capabilities ?? [];
  const review = workbench.review;
  const reviewUnits = (review?.accepted?.length ?? 0) + (review?.rejected?.length ?? 0) + (review?.unreviewed?.length ?? 0);
  const reviewDisabled = busy || generating || workbench.readOnly || fieldDirty || materialDirty;
  const canReview = capabilities.includes('SetReview');
  const activeCandidateId = stageCandidateId(workbench);

  const decideReview = async (scope: 'entity' | 'section', refId: string, decision: ReviewDecision) => {
    if (!activeCandidateId) return;
    await run(() => studio.setCreationReview({ candidateId: activeCandidateId, scope, refId, decision }));
  };

  const acceptAllReview = async () => {
    if (!activeCandidateId) return;
    await run(() => studio.acceptAllCreationReview(activeCandidateId));
  };

  // 结构增删保存为新候选版本：不一致的接受状态与后续审阅由后端投影返回，界面不自行推断。
  const editStructure = async (collection: string, operation: 'add' | 'remove', entityKey: string, value: string) => {
    if (!activeCandidateId) return;
    await run(() => studio.saveCreationStructure({ candidateId: activeCandidateId, collection, operation, entityKey, value }));
  };

  // 审阅位置只用于重启后恢复，保存失败只提示错误，不打断审阅本身。
  const moveCursor = async (next: { entityId: string; sectionId: string }) => {
    if (!capabilities.includes('SaveReviewCursor') || !activeCandidateId) return;
    try {
      await studio.saveCreationReviewCursor({
        stage: workbench.stage, candidateId: activeCandidateId, entityId: next.entityId, sectionId: next.sectionId, comparisonRef: '',
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : '审阅位置保存失败');
    }
  };

  const selectEntity = (id: string) => {
    restoredCursor.current = `${workbench.sessionId}:${workbench.stage}`;
    setEntityId(id);
    setSectionId(null);
    void moveCursor({ entityId: id, sectionId: '' });
  };

  const selectSection = (id: string) => {
    restoredCursor.current = `${workbench.sessionId}:${workbench.stage}`;
    setEntityId(null);
    setSectionId(id);
    void moveCursor({ entityId: '', sectionId: id });
  };

  const hint = useMemo(() => {
    if (generating) return '正在生成候选，完成后不会自动跳转正文。';
    if (workbench.recoverable) return '检测到未完成的付费运行，已暂停，不会自动续跑。';
    if (workbench.impact.items.length > 0) return workbench.impact.evidence || '上游修改影响了下游候选。';
    if (workbench.stage === 'intent') return '输入你的想法，确认费用后生成三套故事方向。';
    if (workbench.stage === 'directions') return '比较、保留或选择一套方向，再进入作品方案。';
    return '直接编辑、锁定后确认进入下一步；确认档案前不会写入正式资产。';
  }, [generating, workbench]);

  const cards = workbench.directionSet?.cards ?? [];
  const peopleEntities = workbench.peopleWorld?.entities ?? [];
  const selectedEntity = peopleEntities.find((item) => item.id === entityId) ?? peopleEntities[0];
  // 路线图左侧导航复用既有 RoadmapNavigation，审阅状态以标题后缀呈现。
  const roadmapItems = roadmapNavItems(workbench.roadmap).map((item) => ({
    ...item,
    title: `${item.title}${reviewNavSuffix(item.node.accepted, item.node.rejected)}`,
  }));
  const selectedRoadmap = roadmapItems.find((item) => item.id === entityId) ?? roadmapItems.find((item) => item.kind === 'arc') ?? roadmapItems[0];
  const proposalSections = workbench.proposal?.sections ?? [];
  const selectedSection = proposalSections.find((item) => item.id === sectionId) ?? proposalSections[0];
  // 编辑和共创使用当前阶段的同一个稳定对象标识，不能借用上个阶段的实体。
  const contextualTarget = entityId === 'candidate' ? {id:'candidate',title:'其他策划文本'}
    : workbench.stage === 'people_world' ? selectedEntity
    : workbench.stage === 'roadmap' ? selectedRoadmap?.node
    : workbench.stage === 'proposal' ? selectedSection : undefined;
  const hasOtherFields = workbench.fieldEditor?.fields?.some(field => field.ownerRef === 'candidate');

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
        {(workbench.stageRail ?? []).map((stop) => (
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

      {(workbench.impact.items ?? []).filter((item) => !item.excepted).length > 0 && (
        <div className="px-8 py-3 border-b border-border flex items-center gap-2 flex-wrap">
          <p className="text-sm text-text-sub flex-1">{workbench.impact.evidence || '下游候选已过期。'}</p>
          <Button size="sm" onClick={() => void run(() => studio.resolveCreationImpact('keep_exception').then(() => studio.creationWorkbench()))}>保留为例外</Button>
          <Button size="sm" onClick={() => void run(() => studio.resolveCreationImpact('abandon_upstream').then(() => studio.creationWorkbench()))}>放弃上游修改</Button>
          <Button size="sm" variant="ghost" disabled>局部重算（尚未自动开跑）</Button>
        </div>
      )}

      <div className="flex-1 flex overflow-hidden">
        {(workbench.stage === 'people_world' || workbench.stage === 'roadmap' || workbench.stage === 'proposal') && (
          <aside className="w-[220px] shrink-0 border-r border-border bg-surface p-4 overflow-auto">
            <h3 className="text-xs font-medium text-text-sub uppercase tracking-wider mb-3">
              {workbench.stage === 'people_world' ? '人物与世界' : workbench.stage === 'proposal' ? '作品方案' : '路线图'}
            </h3>
            {workbench.stage === 'people_world' && peopleEntities.map((entity) => (
              <button
                key={entity.id}
                aria-label={entity.title}
                aria-current={contextualTarget?.id === entity.id ? 'true' : undefined}
                className={clsx('w-full text-left px-2 py-1.5 rounded-md text-sm mb-1 flex items-center justify-between gap-2', contextualTarget?.id === entity.id ? 'bg-primary-subtle text-primary' : 'text-text-sub hover:bg-surface-hover')}
                onClick={() => selectEntity(entity.id)}
              >
                <span>{entity.title}</span>
                <ReviewStateBadge accepted={entity.accepted} rejected={entity.rejected} />
              </button>
            ))}
            {workbench.stage === 'proposal' && proposalSections.map(section => <button key={section.id} aria-label={`编辑${section.title}`} aria-current={contextualTarget?.id === section.id ? 'true' : undefined} className="w-full text-left px-2 py-1.5 text-sm rounded-md hover:bg-surface-hover" onClick={() => selectSection(section.id)}>{section.title}</button>)}
            {hasOtherFields && <button className="w-full text-left px-2 py-1.5 text-sm rounded-md hover:bg-surface-hover" aria-current={entityId === 'candidate' ? 'true' : undefined} onClick={() => { setEntityId('candidate'); setSectionId(null); restoredCursor.current = `${workbench.sessionId}:${workbench.stage}`; }}>其他策划文本</button>}
            {workbench.stage === 'roadmap' && <RoadmapNavigation items={roadmapItems} selectedId={contextualTarget?.id} onSelect={selectEntity} />}
          </aside>
        )}

        <main className="flex-1 overflow-auto p-8">
          {canReview && activeCandidateId && (
            <section aria-label="逐项审阅" className="max-w-4xl mx-auto mb-6 rounded-xl border border-border bg-surface p-4 flex items-center justify-between gap-3 flex-wrap">
              <p className="text-sm text-text-sub">
                未审阅 {(review?.unreviewed ?? []).length} 项 · 已接受 {(review?.accepted ?? []).length} 项 · 已排除 {(review?.rejected ?? []).length} 项
                {review?.complete ? ' · 已全部决定' : ''}
              </p>
              {capabilities.includes('AcceptAllReview') && (
                <Button
                  size="sm"
                  aria-label={`全部接受（${reviewUnits} 项）`}
                  disabled={reviewDisabled || reviewUnits === 0}
                  onClick={() => void acceptAllReview().catch(() => undefined)}
                >
                  全部接受（{reviewUnits} 项）
                </Button>
              )}
            </section>
          )}
          {(workbench.fieldEditor?.fields?.length || workbench.revisions?.length) ? <RevisionPanel key={`revision:${workbench.sessionId}`} editor={workbench.fieldEditor} actions={workbench.revisions ?? []} structure={workbench.structure ?? []} disabled={busy||generating||workbench.readOnly||fieldDirty||materialDirty} onPreview={async(candidateId,path,instruction,key,allow)=>{await studio.previewCreationRevision({candidateId,paths:[path],instruction,idempotencyKey:key,...allow});await load();}} onConfirm={async(action)=>{await studio.confirmCreationRevision({actionId:action.actionId,argumentsHash:action.argumentsHash,idempotencyKey:action.idempotencyKey});await load();}} onReject={async(id)=>{await run(()=>studio.rejectCreationRevision(id));}} onApply={async(action,paths)=>{const command={actionId:action.actionId,argumentsHash:action.argumentsHash,idempotencyKey:action.idempotencyKey,...(paths?.length?{paths}:{})};await run(()=>studio.applyCreationRevision(command));}}/> : null}
          {(workbench.fieldEditor?.fields?.length || fieldDirty) ? <CandidateFields ownerRef={contextualTarget?.id} targetLabel={contextualTarget?.title} registerBeforeLeave={registerBeforeClose} onDirtyChange={setFieldDirty} key={`fields:${workbench.sessionId}`} editor={workbench.fieldEditor ?? emptyFieldEditor} disabled={busy||generating||workbench.readOnly||materialDirty} onSave={async(candidateId,changes)=>{await run(()=>studio.saveCreationFields({candidateId,changes}));}} onLock={async(candidateId,path,locked)=>{await run(()=>(locked?studio.lockCreationField:studio.unlockCreationField)(candidateId,path));}}/> : null}
          {capabilities.includes('SaveStructure') && (workbench.structure ?? []).length > 0 && <StructureEditor key={`structure:${workbench.sessionId}`} collections={workbench.structure ?? []} disabled={busy||generating||workbench.readOnly||fieldDirty||materialDirty} onAdd={async(collection,entityKey,value)=>{await editStructure(collection,'add',entityKey,value);}} onDelete={async(collection,entityKey)=>{await editStructure(collection,'remove',entityKey,'');}}/>}
          {(workbench.canPrepareSample || workbench.sampleHandoff) && <SamplePanel key={`sample:${workbench.sessionId}`} sample={workbench.sampleHandoff} version={workbench.contentVersion ?? 0} runVersion={workbench.runSlotVersion ?? 0} directionCandidateId={workbench.directionSet.selectedCandidateId || workbench.directionSet.candidateId} directionCardId={workbench.directionSet.selectedId} disabled={!workbench.canPrepareSample || busy || generating || workbench.readOnly || materialDirty} onSave={async (value) => { await run(() => studio.saveCreationSample({ ...value, projectId: workbench.projectId, sessionId: workbench.sessionId })); }} />}
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
              {(workbench.material.gaps ?? []).map((gap) => <div key={gap.id} className="rounded-lg border border-border p-3 text-sm">
                <p className="font-medium">{gap.kind === 'conflict' ? '冲突' : gap.kind === 'required' ? '必要缺口' : '可选建议'} · {({ proposal: '作品方案', people_world: '人物世界', roadmap: '路线图' } as Record<string, string>)[gap.stage]}</p>
                <p>{gap.reason}</p><p className="text-text-sub">{gap.suggested_action}</p>
                {gap.quote && <blockquote className="mt-2 border-l-2 border-border pl-2 text-text-sub">{gap.quote}</blockquote>}
              </div>)}
              {(workbench.material.availableStages ?? []).includes(workbench.stage) && !({ proposal: workbench.proposal.candidateId, people_world: workbench.peopleWorld.candidateId, roadmap: workbench.roadmap.candidateId } as Record<string, string>)[workbench.stage] &&
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
                      <Button size="sm" disabled={busy || generating || workbench.readOnly} onClick={() => { setDirectionInstruction(''); setDirectionDialog({mode:'combine'}); }}>组合 {keptIds.length} 个保留方向</Button>
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
                          <Button variant="ghost" size="sm" className="w-full mt-2" disabled={generating || busy || workbench.readOnly} onClick={() => { setDirectionInstruction(''); setDirectionDialog({mode:'regenerate_one',targetCardId:card.id}); }}>只重做这张</Button>
                        </div>
                      </Card>
                    ))}
                  </div>
                )}
                {(workbench.directionSet.combined ?? []).length > 0 && (
                  <div className="mt-6 space-y-3">
                    <h3 className="text-sm font-medium text-text">组合方向</h3>
                    {(workbench.directionSet.combined ?? []).map(item => (
                      <Card key={item.candidateId} className={clsx(item.card.selected && 'ring-1 ring-primary')}>
                        <p className="text-sm font-medium text-text">{item.card.hook}</p>
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-2 mt-3">
                          {[
                            ['类型', item.card.genre], ['主角', item.card.protagonist], ['核心冲突', item.card.coreConflict],
                            ['成长代价', item.card.growthCost], ['节奏', item.card.pacing], ['结局方向', item.card.ending],
                          ].map(([label, value]) => <p key={label} className="text-sm text-text-sub"><span className="text-text-muted">{label}：</span>{value}</p>)}
                        </div>
                        <p className="text-xs text-text-muted mt-3">来源：{workbench.directionActions?.find(action => action.resultCandidateId === item.candidateId)?.sourceCardIds?.join('、') || '已保留方向'}</p>
                        <Button variant="primary" size="sm" className="mt-3" disabled={busy || generating || workbench.readOnly} onClick={() => void run(() => studio.selectCreationDirection(item.candidateId, item.card.id)).catch(() => undefined)}>采用这个组合方向</Button>
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
              {proposalSections.map((section) => {
                const Icon = sectionIcons[section.id as keyof typeof sectionIcons] ?? Sparkles;
                return (
                  <Card key={section.id} padding="none" className={clsx(sectionId === section.id && 'ring-1 ring-primary')}>
                    <div className="px-5 py-3 border-b border-border bg-surface-hover flex items-center justify-between gap-3 flex-wrap">
                      <div className="flex items-center gap-2">
                        <Icon className="w-4 h-4 text-text-sub" />
                        <button
                          className="text-sm font-medium text-text hover:text-primary"
                          aria-label={`选中分节 ${section.title}`}
                          aria-current={sectionId === section.id ? 'true' : undefined}
                          onClick={() => selectSection(section.id)}
                        >
                          {section.title}
                        </button>
                        {section.locked && <Lock className="w-3.5 h-3.5 text-text-muted" />}
                        <ReviewStateBadge accepted={section.accepted} rejected={section.rejected} />
                      </div>
                      <div className="flex items-center gap-2">
                        {canReview && (
                          <ReviewDecisionButtons
                            name={section.title}
                            accepted={section.accepted}
                            rejected={section.rejected}
                            disabled={reviewDisabled}
                            onDecide={(decision) => void decideReview('section', section.id, decision).catch(() => undefined)}
                          />
                        )}
                        {section.lockPath && (
                          <Button size="sm" variant="ghost" disabled={reviewDisabled} onClick={() => void run(() => (section.locked ? studio.unlockCreationField : studio.lockCreationField)(workbench.proposal.candidateId, section.lockPath))}>
                            {section.locked ? '解锁' : '锁定'}
                          </Button>
                        )}
                      </div>
                    </div>
                    <p className="p-5 text-sm text-text leading-relaxed whitespace-pre-wrap">{section.content}</p>
                  </Card>
                );
              })}
            </div>
          )}

          {workbench.stage === 'people_world' && selectedEntity && (
            <Card className="max-w-3xl">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <h2 className="text-base font-medium text-text mb-2">{selectedEntity.title}</h2>
                  <p className="text-sm text-text-sub">{selectedEntity.summary}</p>
                </div>
                <ReviewStateBadge accepted={selectedEntity.accepted} rejected={selectedEntity.rejected} />
              </div>
              {canReview && (
                <div className="mt-4 pt-3 border-t border-border">
                  <ReviewDecisionButtons
                    name={selectedEntity.title}
                    accepted={selectedEntity.accepted}
                    rejected={selectedEntity.rejected}
                    disabled={reviewDisabled}
                    onDecide={(decision) => void decideReview('entity', selectedEntity.id, decision).catch(() => undefined)}
                  />
                  <p className="text-xs text-text-muted mt-2">接受只记录审阅决定；排除表示入档时跳过该单位。两者都不会改写候选内容。</p>
                </div>
              )}
            </Card>
          )}

          {workbench.stage === 'roadmap' && selectedRoadmap && (
            <Card className="max-w-3xl">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <p className="text-xs text-text-sub mb-1">{selectedRoadmap.kind === 'book' ? '全书' : selectedRoadmap.kind === 'volume' ? '卷' : selectedRoadmap.kind === 'arc' ? '故事弧' : '近期章节'}</p>
                  <h2 className="text-base font-medium text-text">{selectedRoadmap.node.title || selectedRoadmap.title}</h2>
                  <p className="text-sm text-text-sub mt-1">{selectedRoadmap.node.summary}</p>
                </div>
                <ReviewStateBadge accepted={selectedRoadmap.node.accepted} rejected={selectedRoadmap.node.rejected} />
              </div>
              {selectedRoadmap.kind === 'arc' && <RoadmapGoalBlock node={selectedRoadmap.node} />}
              {canReview && (
                <div className="mt-4 pt-3 border-t border-border">
                  <ReviewDecisionButtons
                    name={selectedRoadmap.node.title || selectedRoadmap.title}
                    accepted={selectedRoadmap.node.accepted}
                    rejected={selectedRoadmap.node.rejected}
                    disabled={reviewDisabled}
                    onDecide={(decision) => void decideReview('entity', selectedRoadmap.id, decision).catch(() => undefined)}
                  />
                  <p className="text-xs text-text-muted mt-2">接受只记录审阅决定；排除表示入档时跳过该节点。两者都不会改写候选内容。</p>
                </div>
              )}
            </Card>
          )}

          {workbench.stage === 'archive_confirm' && (
            <Card className="max-w-xl mx-auto text-center space-y-3">
              <h2 className="text-lg font-medium text-text">确认作品档案</h2>
              <p className="text-sm text-text-sub">确认后才会写入人物、世界规则和大纲等正式资产，并进入可写状态。</p>
              <Button variant="primary" onClick={() => void confirmNext()}>确认作品档案</Button>
            </Card>
          )}

          {activeCandidateId && (workbench.candidateVersions ?? []).length > 0 && (
            <div className="max-w-4xl mx-auto mt-8">
              <CandidateVersions
                stage={workbench.stage}
                baseCandidateId={activeCandidateId}
                versions={workbench.candidateVersions ?? []}
                disabled={reviewDisabled}
                canUseVersion={capabilities.includes('UseCandidateVersion')}
                canCompare={capabilities.includes('CompareCandidateVersions')}
                onUseVersion={(candidateId) => void run(() => studio.useCreationCandidateVersion(candidateId)).catch(() => undefined)}
                onCompare={(stage, baseCandidateId) => studio.compareCreationCandidateVersions(stage, baseCandidateId)}
              />
            </div>
          )}
        </main>

        <aside className="w-[340px] shrink-0 border-l border-border bg-surface flex flex-col overflow-hidden">
          {(generating || workbench.currentRun.runId) && <div className="p-4 border-b border-border"><CreationProgress status={awaitingRun ? 'queued' : workbench.currentRun.status} stage={awaitingRun ? undefined : workbench.currentRun.publicStage} steps={awaitingRun ? [] : workbench.currentRun.steps} createdAt={awaitingRun ? undefined : workbench.currentRun.createdAt} updatedAt={workbench.currentRun.updatedAt} onCancel={generating || workbench.currentRun.cancellable ? cancelRun : undefined} /></div>}
          <CoCreationPanel
            key={`cocreation:${workbench.projectId}:${workbench.sessionId}`}
            editor={workbench.fieldEditor}
            entityRef={contextualTarget?.id ?? ''}
            targetLabel={contextualTarget?.title}
            messages={(workbench.assistant.messages ?? []).map(message => ({
              ...message,
              actionState: workbench.revisions?.find(action => action.actionId === message.actionId)?.state ?? message.actionState,
            }))}
            authorization={workbench.assistant.authorization}
			authorizations={workbench.assistant.authorizations ?? []}
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

      <Modal open={Boolean(archivePreview)} onClose={() => { if (!busy) setArchivePreview(null); }} title="核对正式入档内容">
        {archivePreview && <div className="space-y-4">
          <p className="font-medium">{archivePreview.title}</p>
          <p className="text-sm text-text-sub">人物 {archivePreview.characterCount} 项 · 大纲 {archivePreview.outlineCount} 项 · 锚点 {archivePreview.anchorCount ?? 0} 项</p>
          <ul className="max-h-[40vh] overflow-auto text-sm space-y-2">{(archivePreview.operations ?? []).map((item,index) => <li key={index}>{item.action === 'create' ? '新增' : '更新'} · {item.summary}</li>)}</ul>
          {(archivePreview.excluded?.length ?? 0) > 0 && <p className="text-sm text-warning">{archivePreview.excluded.length} 项已排除，不会入档：{archivePreview.excluded.map(item => item.id).join("、")}</p>}
          {error && <p role="alert" className="text-sm text-danger">{error}</p>}
          <p className="text-xs text-text-sub">确认后才会写入正式档案；候选发生变化时需要重新预览。</p>
          <div className="flex justify-end gap-2"><Button disabled={busy} onClick={() => setArchivePreview(null)}>返回审阅</Button><Button variant="primary" disabled={busy} onClick={() => void confirmArchive()}>{busy ? '入档中…' : '确认写入正式档案'}</Button></div>
        </div>}
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

      <Modal open={Boolean(directionDialog)} onClose={() => setDirectionDialog(null)} title={directionDialog?.mode === 'combine' ? '组合保留方向' : '重做方向卡'}>
        {directionDialog && (
          <div className="space-y-4">
            <p className="text-sm text-text-sub">{directionDialog.mode === 'combine' ? `将 ${keptIds.length} 张已保留方向组合成一个独立候选，原方向卡会继续保留。` : '只更新目标卡，另外两张方向卡保持不变。'}</p>
            <Textarea value={directionInstruction} onChange={event => setDirectionInstruction(event.target.value)} placeholder={directionDialog.mode === 'combine' ? '说明希望保留的卖点、冲突取舍和组合方式…' : '可选：说明希望调整的方向…'} className="min-h-[110px]" />
            <div className="flex justify-end gap-2">
              <Button onClick={() => setDirectionDialog(null)}>取消</Button>
              <Button variant="primary" disabled={busy || (directionDialog.mode === 'combine' && !directionInstruction.trim())} onClick={() => void previewDirectionAction()}>查看费用与影响</Button>
            </div>
          </div>
        )}
      </Modal>

      <Modal open={Boolean(directionPreview)} onClose={() => void rejectDirectionAction()} title="确认方向操作">
        {directionPreview && (
          <div className="space-y-4">
            <p className="text-sm text-text-sub">{directionPreview.mode === 'combine' ? `将精确使用 ${directionPreview.sourceCardIds?.length ?? 0} 张已保留卡。` : `只重做方向卡 ${directionPreview.targetCardId}。`}</p>
            <p className="text-sm text-text-sub">预计调用 {directionPreview.cost.estimatedCalls} 次；Token 上限：{directionPreview.cost.runBudgetUnlimited ? '未设置' : directionPreview.cost.maxRunTokens}。金额按模型服务商实际用量计算。</p>
            {(directionPreview.cost.roleModels ?? []).map(model => <p key={model.role} className="text-xs text-text-sub">{model.role}：{model.provider} / {model.model}</p>)}
            <div className="flex justify-end gap-2">
              <Button onClick={() => void rejectDirectionAction()}>取消</Button>
              <Button variant="primary" disabled={busy || directionPreview.stale} onClick={() => void confirmDirectionAction()}>确认执行</Button>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
}
