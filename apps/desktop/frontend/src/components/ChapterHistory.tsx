import { useEffect, useState } from 'react';
import { studio, type ChapterRevisionView } from '../api/studio';
import { Modal } from './ui/Modal';
import { Button } from './ui/Button';

// 历史预览显示选中版本正文，恢复前由编辑会话保存当前版本。
export function ChapterHistory({chapterId, open, readOnly, onClose, onRestore}: {
  chapterId:string; open:boolean; readOnly:boolean; onClose:()=>void; onRestore:(revision:number)=>Promise<void>;
}) {
  const [items,setItems]=useState<ChapterRevisionView[]>([]);
  const [selected,setSelected]=useState<number>();
  const [busy,setBusy]=useState(false);
  const [error,setError]=useState('');
  useEffect(()=>{
    if(!open)return;
    let active=true;setBusy(true);setError('');setItems([]);
    void studio.chapterHistory(chapterId).then(data=>{if(active){setItems(data);setSelected(data[0]?.revision);}})
      .catch(err=>{if(active)setError(err instanceof Error?err.message:'读取历史失败');})
      .finally(()=>{if(active)setBusy(false);});
    return()=>{active=false;};
  },[chapterId,open]);
  const version=items.find(item=>item.revision===selected);
  return <Modal open={open} onClose={()=>{if(!busy)onClose();}} title="本章历史版本" className="max-w-3xl">
    <div className="space-y-4">
      {busy && <p className="text-sm">处理中…</p>}
      {error && <p role="alert" className="text-sm text-danger">{error}</p>}
      <select aria-label="历史版本" className="w-full border border-border rounded p-2 bg-surface" value={selected ?? ''} disabled={busy} onChange={e=>setSelected(Number(e.target.value))}>
        {items.map(item=><option key={item.revision} value={item.revision}>第 {item.revision} 版 · {item.createdAt}{item.isCurrent?'（当前）':''}</option>)}
      </select>
      {version && <><p className="text-sm text-text-sub">{version.summary}</p><pre className="whitespace-pre-wrap max-h-[45vh] overflow-auto text-sm p-4 bg-surface-elevated border border-border rounded">{version.body || '（空白正文）'}</pre></>}
      <p className="text-xs text-text-sub">恢复会先保存当前输入为版本，再生成一个恢复版本，已有历史会保留。</p>
      <div className="flex justify-end gap-2"><Button disabled={busy} onClick={onClose}>关闭</Button><Button variant="primary" disabled={busy || readOnly || !version || version.isCurrent} onClick={()=>{
        if(!version)return;setBusy(true);setError('');
        void onRestore(version.revision).then(onClose).catch(err=>setError(err instanceof Error?err.message:'恢复失败，当前输入已保留')).finally(()=>setBusy(false));
      }}>保存当前版本并恢复此版</Button></div>
    </div>
  </Modal>;
}
