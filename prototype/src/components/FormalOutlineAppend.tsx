import { useEffect, useRef, useState } from 'react';
import type { ChapterView, ConstraintView, FormalOutlineAppendPreview, FormalOutlineAppendRequest } from '../types/formalOutline';
import { Button } from './ui/Button';
import { Card } from './ui/Card';
import { Input, Textarea } from './ui/Input';
import { Modal } from './ui/Modal';

export interface FormalOutlineAppendAPI {
  listChapters:()=>Promise<ChapterView[]>;
  planningRevisions:()=>Promise<ConstraintView[]>;
  previewFormalOutlineAppend:(request:FormalOutlineAppendRequest)=>Promise<FormalOutlineAppendPreview>;
  confirmFormalOutlineAppend:(command:{request:FormalOutlineAppendRequest;digest:string;idempotencyKey:string})=>Promise<FormalOutlineAppendPreview>;
}

/** 新的近期章节目标只从选定的未来空章起进入正式规划。 */
export function FormalOutlineAppend({api,arcs,readOnly,registerBeforeLeave,onApplied}:{api:FormalOutlineAppendAPI;arcs:Array<{id:string;title:string}>;readOnly:boolean;registerBeforeLeave?:(handler:()=>Promise<void>)=>()=>void;onApplied?:()=>Promise<void>}) {
  const [open,setOpen]=useState(false);
  const [chapters,setChapters]=useState<ChapterView[]>([]);
  const [assets,setAssets]=useState<ConstraintView[]>([]);
  const [parentAssetId,setParentAssetId]=useState('');
  const [chapterId,setChapterId]=useState('');
  const [title,setTitle]=useState('');
  const [summary,setSummary]=useState('');
  const [goal,setGoal]=useState('');
  const [conflict,setConflict]=useState('');
  const [outcome,setOutcome]=useState('');
  const [reason,setReason]=useState('');
  const [preview,setPreview]=useState<FormalOutlineAppendPreview|null>(null);
  const [busy,setBusy]=useState(false);
  const [error,setError]=useState('');
  const [success,setSuccess]=useState('');
  const key=useRef(crypto.randomUUID());
  const pending=useRef(false);pending.current=open && (busy || !!title.trim() || !!summary.trim() || !!goal.trim() || !!conflict.trim() || !!outcome.trim() || !!reason.trim() || !!preview);
  useEffect(()=>{if(!open || !registerBeforeLeave)return;return registerBeforeLeave(async()=>{if(pending.current)throw new Error('未来章节目标尚未确认，请先完成或明确取消。');});},[open,registerBeforeLeave]);
  const options=arcs.flatMap(arc=>assets.filter(item=>item.kind==='outline_node' && item.typedId===arc.id).map(item=>({assetId:item.assetId,title:arc.title})));
  const start=()=>{if(readOnly)return;setOpen(true);setError('');void Promise.all([api.listChapters(),api.planningRevisions()]).then(([nextChapters,nextAssets])=>{setChapters(nextChapters);setAssets(nextAssets);setChapterId(nextChapters.at(-1)?.id||'');}).catch(e=>setError(e instanceof Error?e.message:String(e)));};
  const request=():FormalOutlineAppendRequest=>({parentAssetId,chapterId,title,summary,goal,conflict,outcome,reason});
  const changed=()=>setPreview(null);
  const run=async(work:()=>Promise<void>)=>{setBusy(true);setError('');try{await work();}catch(e){setError(e instanceof Error?e.message:String(e));}finally{setBusy(false);}};
  const askPreview=()=>void run(async()=>{const next=await api.previewFormalOutlineAppend(request());setPreview(next);key.current=crypto.randomUUID();});
  const confirm=()=>{if(!preview)return;void run(async()=>{await api.confirmFormalOutlineAppend({request:preview.request,digest:preview.digest,idempotencyKey:key.current});setSuccess(`“${preview.request.title}”已从第 ${preview.position} 章起加入正式规划。`);setOpen(false);setPreview(null);setTitle('');setSummary('');setGoal('');setConflict('');setOutcome('');setReason('');try{await onApplied?.();}catch{setSuccess('节点已加入正式规划，刷新失败，请重新打开此页。');}});};
  const close=()=>{if(busy)return;if(pending.current){setError('请确认新增，或使用“取消输入并关闭”。');return;}setOpen(false);};
  return <Card className="space-y-3"><h3 className="text-lg font-display text-text">补充未来近期章节</h3><p className="text-sm text-text-sub">在已有正式故事弧末尾新增一个章节目标，从指定的未来空章起供创作使用。</p><Button disabled={readOnly} onClick={start}>新增未来章节目标</Button>{success && <p role="status" className="text-sm text-primary">{success}</p>}
    <Modal open={open} onClose={close} title="新增正式近期章节目标"><div className="space-y-3 max-h-[70vh] overflow-auto">
      <label className="block text-sm">所属故事弧<select aria-label="所属故事弧" value={parentAssetId} disabled={busy} onChange={e=>{setParentAssetId(e.target.value);changed();}} className="w-full mt-1 p-2 bg-surface border border-border rounded-md"><option value="">选择正式故事弧</option>{options.map(item=><option key={item.assetId} value={item.assetId}>{item.title}</option>)}</select></label>
      <label className="block text-sm">从哪一章起生效<select aria-label="新增节点生效章节" value={chapterId} disabled={busy} onChange={e=>{setChapterId(e.target.value);changed();}} className="w-full mt-1 p-2 bg-surface border border-border rounded-md"><option value="">选择未来空章节</option>{chapters.map(chapter=><option key={chapter.id} value={chapter.id}>第 {chapter.position} 章 · {chapter.title}</option>)}</select></label>
      <label className="block text-sm">标题<Input aria-label="未来节点标题" value={title} disabled={busy} onChange={e=>{setTitle(e.target.value);changed();}} /></label>
      <label className="block text-sm">概要<Textarea aria-label="未来节点概要" rows={2} value={summary} disabled={busy} onChange={e=>{setSummary(e.target.value);changed();}} /></label>
      <label className="block text-sm">目标<Textarea aria-label="未来节点目标" rows={2} value={goal} disabled={busy} onChange={e=>{setGoal(e.target.value);changed();}} /></label>
      <label className="block text-sm">冲突<Textarea aria-label="未来节点冲突" rows={2} value={conflict} disabled={busy} onChange={e=>{setConflict(e.target.value);changed();}} /></label>
      <label className="block text-sm">结果<Textarea aria-label="未来节点结果" rows={2} value={outcome} disabled={busy} onChange={e=>{setOutcome(e.target.value);changed();}} /></label>
      <label className="block text-sm">新增理由<Textarea aria-label="新增节点理由" rows={2} value={reason} disabled={busy} onChange={e=>{setReason(e.target.value);changed();}} /></label>
      <Button disabled={busy || !parentAssetId || !chapterId || !title.trim() || !reason.trim()} onClick={askPreview}>预览新增范围</Button>
      {preview && <Card className="space-y-2"><p>在“{preview.parentTitle}”末尾新增第 {preview.nodePosition} 项：{preview.request.title}</p><p className="text-sm text-text-sub">从第 {preview.position} 章起生效；既有节点与历史正文不会修改。</p><Button variant="primary" disabled={busy} onClick={confirm}>确认新增正式节点</Button></Card>}
      {error && <p role="alert" className="text-sm text-danger">{error}</p>}
      <div className="flex justify-end"><Button disabled={busy} onClick={()=>{setOpen(false);setTitle('');setSummary('');setGoal('');setConflict('');setOutcome('');setReason('');setPreview(null);}}>取消输入并关闭</Button></div>
    </div></Modal>
  </Card>;
}
