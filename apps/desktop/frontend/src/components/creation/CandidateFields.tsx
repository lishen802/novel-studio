import { useEffect, useRef, useState } from 'react';
import { Button } from '../ui/Button';
import { Textarea } from '../ui/Input';
export interface TextField { path:string; ownerRef?:string; label:string; group:string; value:string; locked:boolean }
export interface FieldEditor { candidateId:string; version:number; fields:TextField[] }
interface Props {
  editor:FieldEditor; ownerRef?:string; targetLabel?:string; disabled?:boolean; preview?:boolean;
  onDirtyChange?:(dirty:boolean)=>void;
  registerBeforeLeave?:(handler:()=>Promise<void>)=>()=>void;
  onSave:(candidateId:string,changes:Record<string,string>)=>Promise<void>;
  onLock:(candidateId:string,path:string,locked:boolean)=>Promise<void>;
}

// 对象切换只改变可见字段，未保存补丁和不可变候选基线属于同一个编辑会话。
export function CandidateFields({editor,ownerRef,targetLabel,disabled,preview,onSave,onLock,onDirtyChange,registerBeforeLeave}:Props) {
  const [base,setBase]=useState(editor);
  const [changes,setChanges]=useState<Record<string,string>>({});
  const [busy,setBusy]=useState(false);
  const [error,setError]=useState('');
  const pending=useRef<Promise<void>|null>(null);
  const changesRef=useRef(changes);
  const saveRef=useRef<()=>Promise<void>>(async()=>{});
  const dirty=Object.keys(changes).length>0;
  useEffect(()=>{onDirtyChange?.(dirty);},[dirty,onDirtyChange]);
  useEffect(()=>()=>onDirtyChange?.(false),[onDirtyChange]);
  useEffect(()=>{if(!dirty && !pending.current)setBase(editor);},[editor,dirty,busy]);
  const all=base.fields ?? [];
  const fields=ownerRef && all.some(field=>field.ownerRef) ? all.filter(field=>field.ownerRef===ownerRef) : all;
  const hiddenChanges=Object.keys(changes).filter(path=>!fields.some(field=>field.path===path)).length;
  const stale=dirty && (base.candidateId!==editor.candidateId || base.version!==editor.version);

  const save=():Promise<void>=>{
    if(pending.current)return pending.current;
    const patch={...changesRef.current};
    if(!Object.keys(patch).length)return Promise.resolve();
    setBusy(true);setError('');
    const request=Promise.resolve().then(()=>onSave(base.candidateId,patch)).then(()=>{
      const remaining={...changesRef.current};
      for(const [path,value] of Object.entries(patch))if(remaining[path]===value)delete remaining[path];
      changesRef.current=remaining;setChanges(remaining);
    }).catch(reason=>{setError(reason instanceof Error?reason.message:'保存失败，请重试。');throw reason;})
      .finally(()=>{pending.current=null;setBusy(false);});
    pending.current=request;return request;
  };
  saveRef.current=save;
  useEffect(()=>registerBeforeLeave?.(()=>saveRef.current()),[registerBeforeLeave]);
  const lock=async(field:TextField)=>{
    setBusy(true);setError('');
    try{await onLock(base.candidateId,field.path,!field.locked);}
    catch(reason){setError(reason instanceof Error?reason.message:'字段锁更新失败。');}
    finally{setBusy(false);}
  };
  if(!all.length && !dirty)return null;
  return <section aria-label="当前对象文本编辑" className="my-5 rounded-xl border border-border bg-surface p-4 space-y-4">
    <header><h3 className="text-sm font-medium text-text">编辑{targetLabel || '当前候选'} · 第 {base.version} 版</h3>
      <p className="mt-1 text-xs text-text-sub">保存为新候选，不会自动接受或入档。已锁定内容需要先解锁。</p></header>
    {preview && <p className="text-xs text-text-muted">原型演示：修改仅保存在当前页面。</p>}
    {stale && <p role="alert" className="text-sm text-warning">候选版本已变化，未保存输入仍绑定第 {base.version} 版。请保留输入并核对新版本，保存冲突不会覆盖任何版本。</p>}
    {hiddenChanges>0 && <p className="text-sm text-warning">其他对象另有 {hiddenChanges} 处修改，保存时会一并提交。</p>}
    {!fields.length && <p className="text-sm text-text-sub">当前对象没有可直接编辑的文本字段。列表及引用调整仍由结构操作处理。</p>}
    {fields.map(field=><div key={field.path} className="space-y-2">
      <div className="flex items-center justify-between gap-3"><label htmlFor={`field:${field.path}`} className="text-sm text-text">{field.label}<span className="text-xs text-text-muted ml-2">{field.group}</span></label>
        <Button size="sm" disabled={disabled||busy||dirty} aria-label={`${field.locked?'解锁':'锁定'} ${field.label}`} onClick={()=>void lock(field)}>{field.locked?'解锁此字段':'锁定此字段'}</Button></div>
      <Textarea id={`field:${field.path}`} aria-label={fields.length===1?'候选字段内容':`${field.group} · ${field.label}`} className="min-h-24" value={changes[field.path] ?? field.value} disabled={disabled||busy||field.locked} onChange={event=>{
        const next={...changesRef.current};if(event.target.value===field.value)delete next[field.path];else next[field.path]=event.target.value;
        changesRef.current=next;setChanges(next);
      }}/>
    </div>)}
    <div className="flex items-center gap-3"><Button variant="primary" disabled={disabled||busy||!dirty} onClick={()=>void save().catch(()=>undefined)}>{busy?'处理中…':'保存文本修改'}</Button><span className="text-xs text-text-sub">{dirty?`${Object.keys(changes).length} 处未保存`:'文本已保存'}</span></div>
    {error&&<p role="alert" className="text-sm text-danger">{error} 未保存的输入已保留。</p>}
  </section>;
}
