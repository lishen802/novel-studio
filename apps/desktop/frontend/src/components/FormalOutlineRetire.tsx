import { useEffect, useRef, useState } from 'react';
import type { ChapterView, ConstraintView, FormalOutlineReferenceView, FormalOutlineRetirePreview } from '../api/studio';
import { Button } from './ui/Button';
import { Card } from './ui/Card';
import { Textarea } from './ui/Input';
import { Modal } from './ui/Modal';

export interface FormalOutlineRetireAPI {
 listChapters:()=>Promise<ChapterView[]>;
 planningRevisions:()=>Promise<ConstraintView[]>;
 formalOutlineReferences:(assetId:string)=>Promise<FormalOutlineReferenceView>;
 previewFormalOutlineRetire:(assetId:string,chapterId:string,reason:string,unlinkStoryLineId:string,unlinkStoryLineIds:string[],rebindStoryLineId:string,rebindTargetAssetId:string)=>Promise<FormalOutlineRetirePreview>;
 confirmFormalOutlineRetire:(command:{request:FormalOutlineRetirePreview['request'];digest:string;idempotencyKey:string})=>Promise<FormalOutlineRetirePreview>;
}

/** 未来停用核对正式来源与结构引用，并将故事线解除和节点停用交给同一次确认。 */
export function FormalOutlineRetire({api,nodes,readOnly,registerBeforeLeave,onApplied}:{api:FormalOutlineRetireAPI;nodes:Array<{id:string;title:string;level:string;parentId?:string;introducedAt:number;retiredAt:number}>;readOnly:boolean;registerBeforeLeave?:(handler:()=>Promise<void>)=>()=>void;onApplied?:()=>Promise<void>}) {
 const [open,setOpen]=useState(false);
 const [chapters,setChapters]=useState<ChapterView[]>([]);
 const [assets,setAssets]=useState<ConstraintView[]>([]);
 const [assetId,setAssetId]=useState('');
 const [chapterId,setChapterId]=useState('');
 const [reason,setReason]=useState('');
 const [unlinkStoryLineIds,setUnlinkStoryLineIds]=useState<string[]>([]);
 const [rebindTargetAssetId,setRebindTargetAssetId]=useState('');
 const [preview,setPreview]=useState<FormalOutlineRetirePreview|null>(null);
 const [busy,setBusy]=useState(false);
 const [error,setError]=useState('');
 const [success,setSuccess]=useState('');
 const [inspection,setInspection]=useState<FormalOutlineReferenceView|null>(null);
 const [inspectionError,setInspectionError]=useState('');
 const [inspectionEpoch,setInspectionEpoch]=useState(0);
 const key=useRef(crypto.randomUUID());
 const dirty=useRef(false);dirty.current=open && (busy || !!reason.trim() || !!preview || unlinkStoryLineIds.length>0 || !!rebindTargetAssetId);
 useEffect(()=>{if(!open || !registerBeforeLeave)return;return registerBeforeLeave(async()=>{if(dirty.current)throw new Error('未来停用尚未确认，请先完成或明确取消。');});},[open,registerBeforeLeave]);
 useEffect(()=>{if(!open||!assetId){setInspection(null);setInspectionError('');return;}let cancelled=false;setInspection(null);setInspectionError('');void api.formalOutlineReferences(assetId).then(value=>{if(!cancelled)setInspection(value);}).catch(e=>{if(!cancelled)setInspectionError(e instanceof Error?e.message:String(e));});return()=>{cancelled=true;};},[api,open,assetId,inspectionEpoch]);
 const options=nodes.filter(node=>node.level==='chapter' && node.retiredAt===0).flatMap(node=>assets.filter(a=>a.kind==='outline_node' && a.typedId===node.id).map(a=>({assetId:a.assetId,title:node.title,parentId:node.parentId,introducedAt:node.introducedAt})));
 const selected=options.find(item=>item.assetId===assetId);
 const futureChapters=chapters.filter(chapter=>chapter.body === '' && chapter.position>(selected?.introducedAt||0));
 const storyLineReferences=inspection?.references.length && inspection.references.every(item=>item.kind==='story_line') ? inspection.references : null;
 const rebindTargets=options.filter(item=>item.assetId!==assetId && item.parentId===selected?.parentId && (!chapterId || item.introducedAt<=(chapters.find(ch=>ch.id===chapterId)?.position||0)));
 const referencesResolved=!!inspection && (inspection.references.length===0 || (!!storyLineReferences && unlinkStoryLineIds.length===storyLineReferences.length && storyLineReferences.every(item=>unlinkStoryLineIds.includes(item.id))) || (storyLineReferences?.length===1 && !!rebindTargetAssetId && rebindTargets.some(item=>item.assetId===rebindTargetAssetId)));
 const start=()=>{if(readOnly)return;setOpen(true);setError('');setUnlinkStoryLineIds([]);setRebindTargetAssetId('');void Promise.all([api.listChapters(),api.planningRevisions()]).then(([nextChapters,nextAssets])=>{setChapters(nextChapters);setAssets(nextAssets);setChapterId('');}).catch(e=>setError(e instanceof Error?e.message:String(e)));};
 const run=async(work:()=>Promise<void>)=>{setBusy(true);setError('');try{await work();}catch(e){setError(e instanceof Error?e.message:String(e));setInspectionEpoch(value=>value+1);}finally{setBusy(false);}};
 const askPreview=()=>void run(async()=>{const ids=[...unlinkStoryLineIds].sort();const next=await api.previewFormalOutlineRetire(assetId,chapterId,reason,ids.length===1?ids[0]:'',ids.length>1?ids:[],rebindTargetAssetId?storyLineReferences?.[0].id||'':'',rebindTargetAssetId);setPreview(next);key.current=crypto.randomUUID();});
 const confirm=()=>{if(!preview)return;void run(async()=>{await api.confirmFormalOutlineRetire({request:preview.request,digest:preview.digest,idempotencyKey:key.current});setSuccess(`“${preview.title}”已安排从第 ${preview.position} 章起停用。`);setOpen(false);setPreview(null);setReason('');setUnlinkStoryLineIds([]);setRebindTargetAssetId('');try{await onApplied?.();}catch{setSuccess('节点停用已提交，刷新失败，请重新打开此页。');}});};
 const close=()=>{if(busy)return;if(dirty.current){setError('请确认停用，或使用“取消输入并关闭”。');return;}setOpen(false);};
 return <Card className="space-y-3"><h3 className="text-lg font-display text-text">停用未来章节目标</h3><p className="text-sm text-text-sub">可停用无引用的近期章节目标。尚未写正文的项目可一并解除全部故事线链接；若只有一条链接，也可改绑到同一故事弧的其他章节目标。</p><Button disabled={readOnly || !nodes.some(node=>node.level==='chapter' && node.retiredAt===0)} onClick={start}>预览未来停用</Button>{success && <p role="status" className="text-sm text-primary">{success}</p>}
 <Modal open={open} onClose={close} title="核对未来节点停用"><div className="space-y-4 max-h-[70vh] overflow-auto">
  <label className="block text-sm">目标节点<select aria-label="待停用节点" value={assetId} disabled={busy} onChange={e=>{setAssetId(e.target.value);setChapterId('');setUnlinkStoryLineIds([]);setRebindTargetAssetId('');setPreview(null);}} className="w-full mt-1 p-2 bg-surface border border-border rounded-md"><option value="">选择正式章节节点</option>{options.map(item=><option key={item.assetId} value={item.assetId}>{item.title} · {item.introducedAt>0?`第 ${item.introducedAt} 章起`:'首次入档'}</option>)}</select></label>
  {assetId && <div className="space-y-2 text-sm"><div className="flex items-center justify-between"><span className="font-medium">直接结构引用</span><Button disabled={busy} onClick={()=>{setPreview(null);setUnlinkStoryLineIds([]);setRebindTargetAssetId('');setInspectionEpoch(value=>value+1);}}>重新检查引用</Button></div>{inspectionError && <p role="alert" className="text-danger">{inspectionError}</p>}{!inspection && !inspectionError && <p className="text-text-sub">正在检查引用…</p>}{inspection && (inspection.references.length ? <div role="status" className="space-y-1"><p className="text-danger">发现 {inspection.references.length} 项引用：</p><ul className="list-disc pl-5 space-y-1">{inspection.references.map(item=><li key={`${item.kind}-${item.id}`}>{({child:'子节点',story_line:'故事线',scene_card:'场景卡',anchor:'锚点',formal_asset:'正式资产'} as Record<string,string>)[item.kind]||item.kind} · {item.label}</li>)}</ul>{!storyLineReferences && <p>需先处理这些引用才能停用。</p>}</div> : <p role="status" className="text-text-sub">未发现直接结构引用；仍须核对未来章节和正式版本。</p>)}</div>}
  {storyLineReferences && <label className="flex items-start gap-2 text-sm"><input type="checkbox" aria-label="同时解除故事线关联" checked={unlinkStoryLineIds.length===storyLineReferences.length && storyLineReferences.every(item=>unlinkStoryLineIds.includes(item.id))} disabled={busy} onChange={e=>{setUnlinkStoryLineIds(e.target.checked?storyLineReferences.map(item=>item.id):[]);setRebindTargetAssetId('');setPreview(null);}} /><span>同时解除{storyLineReferences.map(item=>`“${item.label}”`).join('、')}与此节点的关联。仅限全项目从未写入正文或恢复稿；确认后旧链接快照保留在操作回执中。</span></label>}
  {storyLineReferences?.length===1 && rebindTargets.length>0 && <label className="block text-sm">或将故事线改绑到同一故事弧的章节目标<select aria-label="故事线改绑目标" value={rebindTargetAssetId} disabled={busy} onChange={e=>{setRebindTargetAssetId(e.target.value);setUnlinkStoryLineIds([]);setPreview(null);}} className="w-full mt-1 p-2 bg-surface border border-border rounded-md"><option value="">不改绑</option>{rebindTargets.map(item=><option key={item.assetId} value={item.assetId}>{item.title}</option>)}</select></label>}
  <label className="block text-sm">从哪一章起停用<select aria-label="停用生效章节" value={chapterId} disabled={busy} onChange={e=>{setChapterId(e.target.value);setPreview(null);}} className="w-full mt-1 p-2 bg-surface border border-border rounded-md"><option value="">选择未来空章节</option>{futureChapters.map(chapter=><option key={chapter.id} value={chapter.id}>第 {chapter.position} 章 · {chapter.title}</option>)}</select></label>
  <label className="block text-sm">停用理由<Textarea aria-label="节点停用理由" rows={3} maxLength={2000} value={reason} disabled={busy} onChange={e=>{setReason(e.target.value);setPreview(null);}} /></label>
  <Button disabled={busy || !assetId || !chapterId || !reason.trim() || !referencesResolved} onClick={askPreview}>预览停用范围</Button>
  {preview && <Card className="space-y-2"><p>“{preview.title}”{preview.introducedAt>0?`从第 ${preview.introducedAt} 章起加入`:'属于首次入档规划'}，拟从第 {preview.position} 章起停用。</p>{preview.unlinkedStoryLine && <p className="text-sm text-danger">将同时解除“{preview.unlinkedStoryLine.name}”故事线与此节点的关联。</p>}{preview.unlinkedStoryLines && <p className="text-sm text-danger">将同时解除{preview.unlinkedStoryLines.map(item=>`“${item.name}”`).join('、')}故事线与此节点的关联。</p>}{preview.reboundStoryLine && <p className="text-sm text-danger">将把“{preview.reboundStoryLine.name}”故事线从此节点改绑到“{preview.reboundStoryLine.targetTitle}”。</p>}<p className="text-sm text-text-sub">旧章节继续读取该节点，正式资料和历史版本不会删除。</p><Button variant="primary" disabled={busy || !referencesResolved} onClick={confirm}>确认从该章起停用</Button></Card>}
  {error && <p role="alert" className="text-sm text-danger">{error}</p>}
  <div className="flex justify-end"><Button disabled={busy} onClick={()=>{setOpen(false);setReason('');setUnlinkStoryLineIds([]);setRebindTargetAssetId('');setPreview(null);}}>取消输入并关闭</Button></div>
 </div></Modal></Card>;
}
