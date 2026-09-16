import { useEffect, useState } from 'react';
import { Button } from '../ui/Button';
import { Textarea } from '../ui/Input';
export interface FieldEditor {candidateId:string;version:number;fields:Array<{path:string;label:string;group:string;value:string;locked:boolean}>}
interface Props {editor:FieldEditor;disabled?:boolean;preview?:boolean;onDirtyChange?:(dirty:boolean)=>void;onSave:(candidateId:string,changes:Record<string,string>)=>Promise<void>;onLock:(candidateId:string,path:string,locked:boolean)=>Promise<void>}
export function CandidateFields({editor,disabled,preview,onSave,onLock,onDirtyChange}:Props){
 const [base,setBase]=useState(editor);
 const [path,setPath]=useState(editor.fields?.[0]?.path ?? '');
 const [changes,setChanges]=useState<Record<string,string>>({});
 const [busy,setBusy]=useState(false);
 const [error,setError]=useState('');
 const dirty=Object.keys(changes).length>0;
 useEffect(()=>{onDirtyChange?.(dirty);},[dirty,onDirtyChange]);
 useEffect(()=>()=>onDirtyChange?.(false),[onDirtyChange]);
 useEffect(()=>{if(!dirty){setBase(editor);setPath(old=>(editor.fields ?? []).some(f=>f.path===old)?old:editor.fields?.[0]?.path ?? '');}},[editor,dirty]);
 const fields=base.fields ?? [];
 const field=fields.find(f=>f.path===path);
 const save=async()=>{setBusy(true);setError('');try{await onSave(base.candidateId,changes);setChanges({});}catch(e){setError(e instanceof Error?e.message:'保存失败，请保留输入后重试。');}finally{setBusy(false);}};
 const lock=async()=>{if(!field)return;setBusy(true);setError('');try{await onLock(base.candidateId,field.path,!field.locked);}catch(e){setError(e instanceof Error?e.message:'字段锁更新失败。');}finally{setBusy(false);}};
 return <details open={dirty || undefined} className="mb-6 rounded-xl border border-border bg-surface p-4">
  <summary className="cursor-pointer text-sm font-medium text-text">编辑本阶段文本 · 第 {base.version} 版{dirty?' · 有未保存修改':''}</summary>
  <div className="mt-4 space-y-3">
   <p className="text-xs leading-relaxed text-text-sub">修改会保存为新候选，仍需重新确认。锁定与接受独立；已锁定内容需先解锁才能修改。实体增删见下方「结构增删」区块，数组内数量调整暂未开放。</p>
   {preview&&<p className="text-xs text-text-muted">原型演示：修改仅保存在当前页面。</p>}
   <label className="block text-sm text-text">选择文本字段<select aria-label="选择文本字段" className="mt-2 w-full rounded-md border border-border bg-surface p-2" value={path} disabled={disabled||busy} onChange={e=>setPath(e.target.value)}>{fields.map(f=><option key={f.path} value={f.path}>{f.group} · {f.label}{f.locked?'（已锁定）':''}</option>)}</select></label>
   {field&&<><label className="block text-sm text-text">{field.label}<Textarea aria-label="候选字段内容" className="mt-2 min-h-40" value={changes[field.path] ?? field.value} disabled={disabled||busy||field.locked} onChange={e=>setChanges(old=>({...old,[field.path]:e.target.value}))}/></label>
    <div className="flex flex-wrap gap-2"><Button variant="primary" disabled={disabled||busy||!dirty} onClick={()=>void save()}>{busy?'处理中…':'保存文本修改'}</Button><Button disabled={disabled||busy||dirty} onClick={()=>void lock()}>{field.locked?'解锁此字段':'锁定此字段'}</Button></div></>}
   {error&&<p role="alert" className="text-sm text-danger">{error} 未保存的输入已保留。</p>}
  </div>
 </details>;
}
