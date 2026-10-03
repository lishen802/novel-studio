import { useEffect, useRef, useState } from 'react';
import type { ConstraintView } from '../api/studio';
import type { RevisionCard } from './creation/RevisionPanel';
import { Button } from './ui/Button';
import { Card } from './ui/Card';
import { Textarea } from './ui/Input';
import { Modal } from './ui/Modal';

export interface FormalCharacterAPI {
 previewCharacterRevision?: (request: { assetId: string; expectedRevision: number; field: string; instruction: string; idempotencyKey: string }) => Promise<RevisionCard>;
 characterRevisionActions?: () => Promise<RevisionCard[]>;
 confirmCreationRevision?: (command: { actionId: string; argumentsHash: string; idempotencyKey: string }) => Promise<RevisionCard>;
 rejectCreationRevision?: (id: string) => Promise<unknown>;
}
const fields = { name: '姓名', summary: '简介', motivation: '动机', fear: '恐惧', secret: '秘密', arc: '人物弧' };
const states: Record<string, string> = { pending_confirmation: '待确认费用', executing: '正在修改与独立审阅', succeeded: '修改结果已返回', failed: '生成失败，请重新提出修改', expired: '费用提案已过期', rejected: '已放弃', conflict: '基线已变化', outcome_unknown: '结果未知，请核对运行记录' };
const errorMessage = (error: unknown) => error instanceof Error ? error.message : String(error);

/** 正式人物 AI 修改独立冻结费用与最新版本，结果通过后由作者另行确认生效章节。 */
export function FormalCharacterAI({ api, items, readOnly, registerBeforeLeave, onReady }: { api: FormalCharacterAPI; items: ConstraintView[]; readOnly: boolean; registerBeforeLeave?: (handler: () => Promise<void>) => () => void; onReady: () => Promise<void> }) {
 const [open, setOpen] = useState(false);
 const [assetId, setAssetId] = useState('');
 const [field, setField] = useState('motivation');
 const [instruction, setInstruction] = useState('');
 const [actions, setActions] = useState<RevisionCard[]>([]);
 const [busy, setBusy] = useState(false);
 const [error, setError] = useState('');
 const key = useRef(crypto.randomUUID());
 const selected = items.find((item) => item.assetId === assetId);
 const pending = useRef({ dirty: open && !!instruction.trim(), busy }); pending.current = { dirty: open && !!instruction.trim(), busy };
 const available = !!api.previewCharacterRevision && !!api.characterRevisionActions && !!api.confirmCreationRevision;
 useEffect(() => { if ((!pending.current.dirty && !busy) || !registerBeforeLeave) return; return registerBeforeLeave(async () => { if (pending.current.dirty || pending.current.busy) throw new Error('人物 AI 修改要求尚未处理，请完成提案或明确取消输入。'); }); }, [open, instruction, busy, registerBeforeLeave]);
 const load = async () => { if (api.characterRevisionActions) setActions(await api.characterRevisionActions()); await onReady(); };
 const running = actions.some((action) => action.state === 'executing');
 useEffect(() => { if (!available || !open) return; let active = true; void api.characterRevisionActions!().then((views) => { if (active) setActions(views); }).catch((e) => { if (active) setError(errorMessage(e)); }); return () => { active = false; }; }, [available, api, open]);
 useEffect(() => { if (!open || !running) return; const timer = window.setInterval(() => { void load().catch((e) => setError(errorMessage(e))); }, 2000); return () => window.clearInterval(timer); }, [open, running, api, onReady]);
 const act = async (work: () => Promise<void>) => { setBusy(true); setError(''); try { await work(); } catch (e) { setError(errorMessage(e)); } finally { setBusy(false); } };
 const close = () => { if (busy) return; if (instruction.trim()) { setError('修改要求尚未处理，请继续或明确取消输入。'); return; } setOpen(false); };
 const propose = () => { if (!selected || !api.previewCharacterRevision) return; void act(async () => { const view = await api.previewCharacterRevision!({ assetId, expectedRevision: selected.revision, field, instruction, idempotencyKey: key.current }); setActions((old) => [view, ...old.filter((action) => action.actionId !== view.actionId)]); }); };
 const confirm = (action: RevisionCard) => { void act(async () => { const view = await api.confirmCreationRevision!({ actionId: action.actionId, argumentsHash: action.argumentsHash, idempotencyKey: action.idempotencyKey }); setActions((old) => old.map((item) => item.actionId === view.actionId ? view : item)); await onReady(); setInstruction(''); key.current = crypto.randomUUID(); }); };
 return <>
  <Button disabled={readOnly || !items.length || busy || !available} onClick={() => { setAssetId(items[0].assetId); setOpen(true); setError(''); }}>让 AI 修改正式人物</Button>
  {!available && <p className="text-xs text-text-muted">从最新正式版本发起 AI 修改尚未就绪。</p>}
  <Modal open={open} onClose={close} title="让 AI 修改正式人物"><div className="space-y-4 max-h-[70vh] overflow-auto">
   <p className="text-sm text-text-sub">以最新正式版本为基线，只修改选定字段。先确认费用，再生成与独立审阅；结果不会自动入档。</p>
   <label className="block text-sm">正式人物<select aria-label="AI 正式人物" disabled={busy || !!instruction.trim()} value={assetId} onChange={(e) => { setAssetId(e.target.value); key.current = crypto.randomUUID(); }} className="w-full mt-1 p-2 bg-surface border border-border rounded-md">{items.map((item) => <option key={item.assetId} value={item.assetId}>{JSON.parse(item.current).name} · 版本 {item.revision}</option>)}</select></label>
   {selected?.locked && <p role="alert" className="text-warning text-sm">人物已锁定，不能发起 AI 修改。</p>}
   <label className="block text-sm">修改字段<select aria-label="AI 人物修改字段" disabled={busy} value={field} onChange={(e) => { setField(e.target.value); key.current = crypto.randomUUID(); }} className="w-full mt-1 p-2 bg-surface border border-border rounded-md">{Object.entries(fields).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
   {selected && <p className="text-sm whitespace-pre-wrap text-text-sub">当前正式原文：{JSON.parse(selected.current)[field] || '未填写'}</p>}
   <label className="block text-sm">修改要求<Textarea aria-label="正式人物 AI 修改要求" rows={3} maxLength={4000} disabled={busy || readOnly || selected?.locked} value={instruction} onChange={(e) => { setInstruction(e.target.value); key.current = crypto.randomUUID(); }} /></label>
   <Button disabled={busy || readOnly || selected?.locked || !instruction.trim() || running} onClick={propose}>预览人物 AI 修改费用</Button>
   <p className="text-xs text-text-muted">别名、角色定位、关系和其他人物保持原样。旧手工或缺少可靠入档来源的档案暂不能使用此入口。</p>
   {actions.filter((action) => action.formalAssetId === assetId && action.state !== 'rejected').map((action) => <Card key={action.actionId} className="space-y-2 text-sm">
    <p className="font-medium">{states[action.state] || action.state} · 基准版本 {action.formalRevision}</p><p className="whitespace-pre-wrap">{action.instruction}</p>
    {action.targets.map((target) => <p key={target.path}>{target.label}原文：{target.value || '未填写'}</p>)}
    <p>最多 {action.cost.estimatedCalls} 次模型调用 · {action.cost.runBudgetUnlimited ? '运行 token 不设上限' : `运行预算 ${action.cost.maxRunTokens} token`} · 金额暂不可估算，按服务商实际用量计费。</p>
    {action.cost.roleModels.map((model) => <p key={model.role}>{model.role === 'planner' ? '修改' : '独立审阅'}：{model.provider} / {model.model}</p>)}
    {action.stale && <p role="alert" className="text-warning">正式版本或保护输入已变化，此提案/结果不再适用，请重新提出修改。</p>}
    {action.diffs.map((diff) => <div key={diff.path}><p>{diff.label}</p><p className="whitespace-pre-wrap text-text-sub">修改前：{diff.before}</p><p className="whitespace-pre-wrap">修改后：{diff.after}</p></div>)}
    {action.summary && <p className="whitespace-pre-wrap text-text-sub">{action.summary}</p>}
    {action.state === 'succeeded' && action.quality === 'passed' && !action.stale && <p className="text-primary">已审阅候选可在“修订人物档案”的修改来源中选择，再确认理由与未来空章节。</p>}
    {action.state === 'pending_confirmation' && <div className="flex gap-2"><Button disabled={busy || readOnly || action.stale || running} variant="primary" onClick={() => confirm(action)}>确认费用并生成候选</Button>{api.rejectCreationRevision && <Button disabled={busy || readOnly} onClick={() => void act(async () => { await api.rejectCreationRevision!(action.actionId); await load(); })}>放弃提案</Button>}</div>}
   </Card>)}
   {error && <p role="alert" className="text-danger text-sm">{error}</p>}
   <div className="flex flex-wrap justify-end gap-2"><Button disabled={busy} onClick={() => void act(load)}>刷新任务与候选</Button><Button disabled={busy} onClick={() => { setInstruction(''); key.current = crypto.randomUUID(); setOpen(false); }}>取消输入并关闭</Button></div>
  </div></Modal>
 </>;
}
