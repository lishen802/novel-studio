import { useEffect, useRef, useState } from 'react';
import type { ChapterView, PlanningRearchivePreview } from '../api/studio';
import { Button } from './ui/Button';
import { Card } from './ui/Card';
import { Textarea } from './ui/Input';
import { Modal } from './ui/Modal';

export interface PlanningRearchiveAPI {
  listChapters: () => Promise<ChapterView[]>;
  previewPlanningRearchive: (chapterId:string, reason:string) => Promise<PlanningRearchivePreview>;
  confirmPlanningRearchive: (command:{request:PlanningRearchivePreview['request'];digest:string;idempotencyKey:string}) => Promise<PlanningRearchivePreview>;
}

const labels:Record<string,string> = {title:'标题',summary:'概要',goal:'目标',conflict:'冲突',outcome:'结果',positioning:'作品定位',sell_point:'卖点',core_conflict:'核心冲突',protagonist_arc:'主角弧',growth_cost:'成长代价',pacing:'节奏',ending:'结局方向',genre:'类型'};
const fieldLabel=(path:string) => labels[path.split('/').filter(Boolean).at(-1) || ''] || '规划文字';
const parseFields=(value:string):Record<string,string> => { try { const parsed=JSON.parse(value); return parsed && typeof parsed==='object' ? parsed : {}; } catch { return {}; } };
const errorMessage=(e:unknown) => e instanceof Error ? e.message : String(e);

/** 整组规划候选按未来章节入档；预览失败和退出保护均保留作者填写的理由。 */
export function PlanningRearchive({api,readOnly,registerBeforeLeave,onApplied}:{api:PlanningRearchiveAPI;readOnly:boolean;registerBeforeLeave?:(handler:()=>Promise<void>)=>()=>void;onApplied?:()=>Promise<void>}) {
  const [open,setOpen]=useState(false);
  const [chapters,setChapters]=useState<ChapterView[]>([]);
  const [chapterId,setChapterId]=useState('');
  const [reason,setReason]=useState('');
  const [preview,setPreview]=useState<PlanningRearchivePreview|null>(null);
  const [busy,setBusy]=useState(false);
  const [error,setError]=useState('');
  const [success,setSuccess]=useState('');
  const key=useRef(crypto.randomUUID());
  const pending=useRef({dirty:false,busy:false}); pending.current={dirty:open && (!!reason.trim() || !!preview),busy};
  useEffect(()=>{ if (!open || !registerBeforeLeave) return; return registerBeforeLeave(async()=>{ if (pending.current.dirty || pending.current.busy) throw new Error('整组规划入档尚未确认，请先完成或明确取消。'); }); },[open,registerBeforeLeave]);
  const start=()=>{ if (readOnly) return; setOpen(true); setError(''); void api.listChapters().then(items=>{setChapters(items);setChapterId(items.at(-1)?.id || '');}).catch(e=>setError(errorMessage(e))); };
  const run=async(work:()=>Promise<void>)=>{setBusy(true);setError('');try{await work();}catch(e){setError(errorMessage(e));}finally{setBusy(false);}};
  const askPreview=()=>void run(async()=>{const next=await api.previewPlanningRearchive(chapterId,reason);setPreview(next);key.current=crypto.randomUUID();});
  const confirm=()=>{if(!preview)return;void run(async()=>{await api.confirmPlanningRearchive({request:preview.request,digest:preview.digest,idempotencyKey:key.current});setSuccess(`规划候选已从第 ${preview.position} 章起入档，更新 ${preview.changes.length} 项。`);setPreview(null);setReason('');setOpen(false);try{await onApplied?.();}catch{setSuccess(`规划候选已入档，刷新档案失败，请重新打开此页。`);}});};
  const close=()=>{if(busy)return;if(reason.trim() || preview){setError('请完成确认，或使用“取消输入并关闭”。');return;}setOpen(false);};
  return <Card className="space-y-3"><h3 className="text-lg font-display text-text">整组规划候选再次入档</h3><p className="text-sm text-text-sub">在驾驶舱完成方案与路线图的候选审阅后，核对整组文字差异，并选择未来空章节生效。</p>
    <Button disabled={readOnly} onClick={start}>预览再次入档</Button>{success && <p role="status" className="text-sm text-primary">{success}</p>}
    <Modal open={open} onClose={close} title="核对规划候选再次入档"><div className="space-y-4 max-h-[70vh] overflow-auto">
      <p className="text-sm text-text-sub">本次仅更新已有方案与路线图节点的文字。人物、结构、关系和历史正文保持现状。</p>
      <label className="block text-sm">从哪一章起生效<select aria-label="规划入档生效章节" value={chapterId} disabled={busy} onChange={e=>{setChapterId(e.target.value);setPreview(null);}} className="w-full mt-1 p-2 bg-surface border border-border rounded-md"><option value="">选择已有空章节</option>{chapters.map(chapter=><option key={chapter.id} value={chapter.id}>第 {chapter.position} 章 · {chapter.title}</option>)}</select></label>
      <label className="block text-sm">修改理由<Textarea aria-label="规划再次入档理由" rows={3} maxLength={2000} value={reason} disabled={busy} onChange={e=>{setReason(e.target.value);setPreview(null);}} /></label>
      {!chapters.length && <p className="text-sm text-warning">请先建立一个用于后续创作的空章节。</p>}
      <Button disabled={busy || !chapterId || !reason.trim()} onClick={askPreview}>预览整组差异</Button>
      {preview && <div className="space-y-3"><p className="text-sm">从第 {preview.position} 章起生效 · 保留前 {preview.protectedChapters} 章 · 更新 {preview.changes.length} 项</p>{preview.changes.map(change=>{const before=parseFields(change.before),after=parseFields(change.after);return <Card key={change.assetId} className="space-y-2"><p className="font-medium">{change.kind==='proposal'?'作品方案':'路线图节点'} · {before['/title'] || after['/title']}</p>{Object.keys(after).filter(path=>before[path]!==after[path]).map(path=><div key={path} className="text-sm"><p className="text-text-sub">{fieldLabel(path)}：{before[path] || '未填写'}</p><p>{fieldLabel(path)}：{after[path] || '未填写'}</p></div>)}</Card>;})}<Button variant="primary" disabled={busy} onClick={confirm}>确认从该章起入档</Button></div>}
      {error && <p role="alert" className="text-sm text-danger">{error}</p>}
      <div className="flex justify-end"><Button disabled={busy} onClick={()=>{setReason('');setPreview(null);setOpen(false);}}>取消输入并关闭</Button></div>
    </div></Modal>
  </Card>;
}
