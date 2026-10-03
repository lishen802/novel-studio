import {useEffect,useRef,useState} from 'react';
import type {ChapterView,ConstraintCommand,ConstraintPreview,ConstraintRequest,ConstraintView} from '../types/constraint';
import {Button} from './ui/Button';
import {Card} from './ui/Card';
import {Input,Textarea} from './ui/Input';
import {Modal} from './ui/Modal';

export interface ConstraintAPI {
 constraints:()=>Promise<ConstraintView[]>;
 listChapters:()=>Promise<ChapterView[]>;
 previewConstraintChange:(request:ConstraintRequest)=>Promise<ConstraintPreview>;
 applyConstraintChange:(command:ConstraintCommand)=>Promise<ConstraintPreview>;
}
const labels:Record<string,string>={world_rule:'世界规则',hard_anchor:'硬锚点',soft_anchor:'软锚点',forbidden:'禁写项'};
const message=(error:unknown)=>error instanceof Error?error.message:String(error);

/** 明确核对生效范围；失败保留输入，修改表单后必须重新预览。 */
export function ConstraintChanges({api,registerBeforeLeave}:{api:ConstraintAPI;registerBeforeLeave?:(handler:()=>Promise<void>)=>()=>void}) {
 const [constraints,setConstraints]=useState<ConstraintView[]>([]);
 const [chapters,setChapters]=useState<ChapterView[]>([]);
 const [open,setOpen]=useState(false);
 const [assetId,setAssetId]=useState('');
 const [chapterId,setChapterId]=useState('');
 const [text,setText]=useState('');
 const [reason,setReason]=useState('');
 const [preview,setPreview]=useState<ConstraintPreview|null>(null);
 const [error,setError]=useState('');
 const [notice,setNotice]=useState('');
 const [busy,setBusy]=useState(false);
 const key=useRef('');
 const selected=constraints.find(item=>item.assetId===assetId);
 const dirty=open&&(text!==(selected?.current??'')||reason.trim()!==''||chapterId!=='');
 const state=useRef({dirty,busy});state.current={dirty,busy};
 useEffect(()=>{
  if(!registerBeforeLeave||!dirty&&!busy)return;
  return registerBeforeLeave(async()=>{if(state.current.busy||state.current.dirty)throw new Error('设定修订尚未确认，请回到世界设定完成核对或取消修订。');});
 },[dirty,busy,registerBeforeLeave]);
 useEffect(()=>{
  let active=true;
  void Promise.all([api.constraints(),api.listChapters()]).then(([items,rows])=>{if(active){setConstraints(items);setChapters(rows);}}).catch(e=>{if(active)setError(message(e));});
  return ()=>{active=false;};
 },[api]);
 const invalidate=()=>{setPreview(null);setError('');key.current='';};
 const choose=(id:string)=>{setAssetId(id);setText(constraints.find(item=>item.assetId===id)?.current??'');setChapterId('');setReason('');invalidate();};
 const discard=()=>{if(busy)return;setOpen(false);invalidate();};
 const close=()=>{if(busy)return;if(dirty){setError('还有未确认的修订，请继续核对或点击取消修订。');return;}discard();};
 const prepare=async()=>{
  if(busy||!selected)return;
  setBusy(true);setError('');setPreview(null);
  try {const result=await api.previewConstraintChange({assetId,intent:'future_only',chapterId,text,reason});setPreview(result);key.current=crypto.randomUUID();}
  catch(e){setError(message(e));}finally{setBusy(false);}
 };
 const confirm=async()=>{
  if(!preview?.canApply||busy)return;
  setBusy(true);setError('');
  try {
   await api.applyConstraintChange({request:preview.request,digest:preview.digest,idempotencyKey:key.current});
   setOpen(false);setPreview(null);setReason('');setChapterId('');setNotice(`已保存：从第 ${preview.position} 章起使用新约束，之前的章节与正文保持原样。`);
   setConstraints(await api.constraints());
  }catch(e){setError(message(e));}finally{setBusy(false);}
 };
 return <Card className="space-y-3">
  <div className="flex flex-wrap items-center justify-between gap-3"><div><h3 className="font-medium">按章节修订规则与约束</h3><p className="text-sm text-text-sub">新语句从选定章节起使用；原设定及历史正文保留。</p></div><Button disabled={!constraints.length} onClick={()=>{choose(constraints[0].assetId);setNotice('');setOpen(true);}}>修订生效范围</Button></div>
  {!constraints.length&&!error&&<p className="text-sm text-text-muted">正式入档后，可修订世界规则、锚点和禁写项。人物、方案及大纲结构的生效范围尚未接通。</p>}
  {constraints.filter(item=>item.changes.length>0).map(item=><div key={item.assetId} className="text-sm border-t border-border pt-2"><p>{labels[item.kind]} · {item.name}</p><p className="text-text-sub">原设定：{item.original}</p>{item.changes.map(change=><p key={change.revision} className="text-text-sub">第 {change.position} 章起 · 版本 {change.revision}：{change.text}（{change.reason}）</p>)}</div>)}
  {notice&&<p role="status" className="text-sm text-primary">{notice}</p>}
  {error&&!open&&<p role="alert" className="text-sm text-danger">{error}</p>}
  <Modal open={open} onClose={close} title="核对设定修订与生效范围">
   <div className="space-y-4 max-h-[75vh] overflow-auto">
    <p className="text-sm text-text-sub">仅从指定章节起生效。故事内变化、追溯修改和事件边界尚未接通。生效范围必须没有正文、历史正文或恢复草稿。</p>
    <label className="block text-sm">正式约束<select aria-label="正式约束" disabled={busy||dirty} value={assetId} onChange={e=>choose(e.target.value)} className="mt-1 w-full border border-border bg-surface rounded-md p-2">{constraints.map(item=><option key={item.assetId} value={item.assetId}>{labels[item.kind]} · {item.name}（版本 {item.revision}）</option>)}</select></label>
    <p className="text-sm whitespace-pre-wrap text-text-sub">当前语句：{selected?.current}</p>
    <label className="block text-sm">新语句<Textarea maxLength={12000} aria-label="新语句" disabled={busy||selected?.locked} value={text} onChange={e=>{setText(e.target.value);invalidate();}} rows={4}/></label>
    <label className="block text-sm">修订理由<Input maxLength={2000} aria-label="修订理由" disabled={busy} value={reason} onChange={e=>{setReason(e.target.value);invalidate();}}/></label>
    <label className="block text-sm">从该章起（含本章）<select aria-label="生效章节" disabled={busy} value={chapterId} onChange={e=>{setChapterId(e.target.value);invalidate();}} className="mt-1 w-full border border-border bg-surface rounded-md p-2"><option value="">选择生效章节</option>{chapters.map(chapter=><option key={chapter.id} value={chapter.id}>第 {chapter.position} 章 · {chapter.title}</option>)}</select></label>
    {!chapters.length&&<p className="text-sm text-warning">请先创建一个用于后续写作的空章节。</p>}
    {selected?.locked&&<p className="text-sm text-warning">该约束已锁定，不能直接修订。</p>}
    {preview&&<div className="border border-border rounded-md p-3 space-y-2 text-sm"><p>第 {preview.position} 章起 · 新版本 {preview.revision}</p><p className="whitespace-pre-wrap">原语句：{preview.before}</p><p className="whitespace-pre-wrap">新语句：{preview.request.text}</p><p>保留此前 {preview.protectedChapters} 章；后续 {preview.futureChapters} 章的路线、场景和事实需要人工复核。</p><p className="text-text-sub">旧设定下的待处理共创候选将失效，未来章节和全局资产的待处理修改将标为冲突。不会自动改写档案或正文。</p>{!preview.canApply&&<p role="alert" className="text-warning">{preview.blockedReason}</p>}</div>}
    {error&&<p role="alert" className="text-sm text-danger">{error}</p>}
    <div className="flex flex-wrap justify-end gap-2"><Button disabled={busy} onClick={discard}>取消修订</Button><Button disabled={busy||!chapterId||!reason.trim()||!text.trim()||selected?.locked} onClick={()=>void prepare()}>预览影响</Button><Button variant="primary" disabled={busy||!preview?.canApply} onClick={()=>void confirm()}>确认从该章起生效</Button></div>
   </div>
  </Modal>
 </Card>;
}
