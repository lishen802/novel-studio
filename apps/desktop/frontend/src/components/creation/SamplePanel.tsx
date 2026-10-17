import { useEffect, useState } from 'react';
import { Button } from '../ui/Button';
import { Textarea } from '../ui/Input';

type Input = { pointOfView: string; tone: string; constraints: string[]; targetRunes: number };
type Snapshot = Input & { expectedVersion: number; expectedRunSlotVersion: number; directionCandidateId: string; directionCardId: string };
export type OpeningSampleCard = { id: string; body: string; status: string; feedback?: string };
interface Props {
 sample?: (Input & { id: string; status: string }) | null;
 opening?: OpeningSampleCard | null;
 version: number; runVersion: number; directionCandidateId: string; directionCardId: string;
 disabled?: boolean; preview?: boolean; onSave: (value: Snapshot) => Promise<void>;
 onGenerate?: () => Promise<OpeningSampleCard | void>;
 onFeedback?: (kind: 'continue_plan' | 'adjust' | 'discard') => Promise<OpeningSampleCard | void>;
 onPromote?: () => Promise<OpeningSampleCard | void>;
}
export function SamplePanel(props: Props) {
 const initial = (): Snapshot => ({ pointOfView: props.sample?.pointOfView ?? '第三人称限知', tone: props.sample?.tone ?? '自然、克制', constraints: props.sample?.constraints ?? [], targetRunes: props.sample?.targetRunes ?? 1200, expectedVersion: props.version, expectedRunSlotVersion: props.runVersion, directionCandidateId: props.directionCandidateId, directionCardId: props.directionCardId });
 const [value,setValue] = useState(initial);
 const [dirty,setDirty] = useState(false);
 const [holding,setHolding] = useState(false);
 const [error,setError] = useState('');
 const [saving,setSaving] = useState(false);
 const [generating,setGenerating] = useState(false);
 const [localOpening,setLocalOpening] = useState<OpeningSampleCard | null>(null);
 useEffect(() => { if (!dirty && !holding) setValue(initial()); }, [props.sample?.id,props.version,props.runVersion,props.directionCandidateId,props.directionCardId,dirty,holding]);
 const change = (patch: Partial<Input>) => { setValue(v => ({...v,...patch})); setDirty(true); };
 const opening = props.opening ?? localOpening;
 const canGenerate = Boolean(props.sample && props.sample.status !== 'stale' && (props.onGenerate || props.preview));
 const save = async () => { setSaving(true);setError('');setHolding(true);try { await props.onSave({...value, constraints: value.constraints.filter(line => line.trim() !== '')});setDirty(false); } catch(e) {setError(e instanceof Error ? e.message : '保存失败，输入已保留。');} finally {setSaving(false);setHolding(false);} };
 const remember = (next?: OpeningSampleCard | void) => { if (next && next.body) setLocalOpening(next); };
 const generate = async () => {
  setGenerating(true); setError(''); setHolding(true);
  try {
   if (props.onGenerate) remember(await props.onGenerate());
   else setLocalOpening({ id: 'preview', body: `${value.pointOfView}。${value.tone}。灯塔在雾里熄着。`, status: 'candidate' });
  } catch (e) { setError(e instanceof Error ? e.message : '试写失败，输入已保留。'); }
  finally { setGenerating(false); setHolding(false); }
 };
 const feedback = async (kind: 'continue_plan' | 'adjust' | 'discard') => {
  setHolding(true); setError('');
  try {
   if (props.onFeedback) remember(await props.onFeedback(kind));
   else if (opening) setLocalOpening({ ...opening, status: kind === 'discard' ? 'discarded' : 'feedback', feedback: kind });
  } catch (e) { setError(e instanceof Error ? e.message : '反馈失败，输入已保留。'); }
  finally { setHolding(false); }
 };
 const promote = async () => {
  setHolding(true); setError('');
  try {
   if (props.onPromote) remember(await props.onPromote());
   else setError('转正需要已确认档案、复检和显式接受。原型演示不会写入正式记忆。');
  } catch (e) { setError(e instanceof Error ? e.message : '转正失败，试写仍保留。'); }
  finally { setHolding(false); }
 };
 return <details className="mb-6 rounded-xl border border-border bg-surface p-4" open>
  <summary className="cursor-pointer text-sm font-medium text-text">开篇试写{props.sample ? (props.sample.status === 'stale' ? ' · 来源已变化，请重新核对' : ' · 已保存准备') : ''}</summary>
  <div className="mt-4 space-y-3">
   <p className="text-xs text-text-sub">试写只使用当前方向、视角和约束。它不进入正式记忆，也不计入定稿字数。方向变化后旧试写失效。</p>
   {props.preview && <p className="text-xs text-text-sub">原型演示：试写只留在当前页面。</p>}
   <label className="block text-sm">视角<Textarea aria-label="试写视角" value={value.pointOfView} disabled={props.disabled || saving} onChange={e=>change({pointOfView:e.target.value})}/></label>
   <label className="block text-sm">语气<Textarea aria-label="试写语气" value={value.tone} disabled={props.disabled || saving} onChange={e=>change({tone:e.target.value})}/></label>
   <label className="block text-sm">约束（每行一条）<Textarea aria-label="试写约束" value={value.constraints.join('\n')} disabled={props.disabled || saving} onChange={e=>change({constraints:e.target.value.split('\n')})}/></label>
   <label className="block text-sm">目标字数 <input aria-label="试写目标字数" className="rounded-md border border-border bg-surface p-2" type="number" min={1} max={10000} value={value.targetRunes} disabled={props.disabled || saving} onChange={e=>change({targetRunes:Number(e.target.value)})}/></label>
   {error && <p role="alert" className="text-sm text-danger">{error} 当前输入已保留。</p>}
   {opening && <article className="rounded-md border border-border p-3 text-sm whitespace-pre-wrap" aria-label="试写正文">{opening.body}<p className="mt-2 text-xs text-text-sub">状态：{opening.status === 'promoted' ? '已转正，仍非正式记忆' : opening.status}</p></article>}
   <div className="flex flex-wrap gap-2">
    <Button disabled={props.disabled || saving || !value.pointOfView.trim() || !value.tone.trim() || !Number.isInteger(value.targetRunes) || value.targetRunes < 1 || value.targetRunes > 10000} onClick={()=>void save()}>{saving ? '保存中…' : '保存试写准备'}</Button>
    <Button disabled={props.disabled || saving || generating || !canGenerate} onClick={()=>void generate()}>{generating ? '生成中…' : '生成试写'}</Button>
    {opening && opening.status !== 'discarded' && <>
     <Button disabled={props.disabled || generating} onClick={()=>void feedback('continue_plan')}>回到方案</Button>
     <Button disabled={props.disabled || generating} onClick={()=>void feedback('adjust')}>调整试写</Button>
     <Button disabled={props.disabled || generating} onClick={()=>void feedback('discard')}>放弃试写</Button>
     <Button disabled={props.disabled || generating || opening.status === 'promoted'} onClick={()=>void promote()}>转正试写</Button>
    </>}
   </div>
  </div>
 </details>;
}
