import { useEffect, useState } from 'react';
import { Button } from '../ui/Button';
import { Textarea } from '../ui/Input';

type Input = { pointOfView: string; tone: string; constraints: string[]; targetRunes: number };
type Snapshot = Input & { expectedVersion: number; expectedRunSlotVersion: number; directionCandidateId: string; directionCardId: string };
interface Props {
 sample?: (Input & { id: string; status: string }) | null;
 version: number; runVersion: number; directionCandidateId: string; directionCardId: string;
 disabled?: boolean; preview?: boolean; onSave: (value: Snapshot) => Promise<void>;
}
export function SamplePanel(props: Props) {
 const initial = (): Snapshot => ({ pointOfView: props.sample?.pointOfView ?? '第三人称限知', tone: props.sample?.tone ?? '自然、克制', constraints: props.sample?.constraints ?? [], targetRunes: props.sample?.targetRunes ?? 1200, expectedVersion: props.version, expectedRunSlotVersion: props.runVersion, directionCandidateId: props.directionCandidateId, directionCardId: props.directionCardId });
 const [value,setValue] = useState(initial);
 const [dirty,setDirty] = useState(false);
 const [error,setError] = useState('');
 const [saving,setSaving] = useState(false);
 useEffect(() => { if (!dirty) setValue(initial()); }, [props.sample?.id,props.version,props.runVersion,props.directionCandidateId,props.directionCardId,dirty]);
 const change = (patch: Partial<Input>) => { setValue(v => ({...v,...patch})); setDirty(true); };
 const save = async () => { setSaving(true);setError('');try { await props.onSave({...value, constraints: value.constraints.filter(line => line.trim() !== '')});setDirty(false); } catch(e) {setError(e instanceof Error ? e.message : '保存失败，输入已保留。');} finally {setSaving(false);} };
 return <details className="mb-6 rounded-xl border border-border bg-surface p-4" open>
  <summary className="cursor-pointer text-sm font-medium text-text">开篇试写准备{props.sample ? (props.sample.status === 'stale' ? ' · 来源已变化，请重新核对' : ' · 已保存') : ''}</summary>
  <div className="mt-4 space-y-3">
   <p className="text-xs text-text-sub">只保存方向、视角和约束，不调用模型，不创建正式章节。试写生成将在后续阶段开放。</p>
   {props.preview && <p className="text-xs text-text-sub">原型演示：准备仅保存在当前页面，持久化未就绪。</p>}
   <label className="block text-sm">视角<Textarea aria-label="试写视角" value={value.pointOfView} disabled={props.disabled || saving} onChange={e=>change({pointOfView:e.target.value})}/></label>
   <label className="block text-sm">语气<Textarea aria-label="试写语气" value={value.tone} disabled={props.disabled || saving} onChange={e=>change({tone:e.target.value})}/></label>
   <label className="block text-sm">约束（每行一条）<Textarea aria-label="试写约束" value={value.constraints.join('\n')} disabled={props.disabled || saving} onChange={e=>change({constraints:e.target.value.split('\n')})}/></label>
   <label className="block text-sm">目标字数 <input aria-label="试写目标字数" className="rounded-md border border-border bg-surface p-2" type="number" min={1} max={10000} value={value.targetRunes} disabled={props.disabled || saving} onChange={e=>change({targetRunes:Number(e.target.value)})}/></label>
   {error && <p role="alert" className="text-sm text-danger">{error} 当前输入已保留。</p>}
   <div className="flex flex-wrap gap-2"><Button disabled={props.disabled || saving || !value.pointOfView.trim() || !value.tone.trim() || !Number.isInteger(value.targetRunes) || value.targetRunes < 1 || value.targetRunes > 10000} onClick={()=>void save()}>{saving ? '保存中…' : '保存试写准备'}</Button><Button disabled>生成试写（尚未就绪）</Button></div>
  </div>
 </details>;
}
