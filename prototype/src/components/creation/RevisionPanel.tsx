import { useState } from 'react';
import type { StructureCollectionView } from '../../mock/creation';
import { structureCollectionTitle } from '../../mock/creation';
import { Button } from '../ui/Button';
import { Textarea } from '../ui/Input';
import type { FieldEditor } from './CandidateFields';

// 单条差异：path 是稳定字段路径，部分采用时按界面顺序原样回传；界面只允许从展示的差异里勾选。
// kind 区分文本修改与结构增删：结构差异按实体整组结算，界面据此限制部分采用。
export interface RevisionDiff {path:string;label:string;group:string;kind:'text'|'add_entity'|'remove_entity';before:string;after:string}
// 结构授权只带被勾选的集合名；两者都为空表示本次修订不允许任何结构增删。
export interface RevisionAllow {allowAdd?:string[];allowRemove?:string[]}
export interface RevisionCard {
 actionId:string;argumentsHash:string;idempotencyKey:string;candidateId:string;version:number;state:string;instruction:string;
 targets:FieldEditor['fields'];preserved:FieldEditor['fields'];cost:{estimatedCalls:number;maxRunTokens:number;runBudgetUnlimited:boolean;roleModels:{role:string;provider:string;model:string}[]};
 runId:string;quality:string;summary:string;stale:boolean;canApply:boolean;applied:boolean;
 diffs:RevisionDiff[];appliedPaths?:string[];allow?:RevisionAllow;
}
interface Props {editor?:FieldEditor;actions:RevisionCard[];structure?:StructureCollectionView[];disabled?:boolean;preview?:boolean;onPreview:(candidateId:string,path:string,instruction:string,key:string,allow:RevisionAllow)=>Promise<void>;onConfirm:(action:RevisionCard)=>Promise<void>;onReject:(id:string)=>Promise<void>;onApply:(action:RevisionCard,paths?:string[])=>Promise<void>}
const labels:Record<string,string>={pending_confirmation:'待你确认',executing:'正在修订与审阅',succeeded:'修订结果',failed:'运行已停止',expired:'提案已过期',rejected:'已放弃',conflict:'内容已变化',outcome_unknown:'需要核对运行结果'};
// 结构差异展示实体名称：优先读 name/title/id，再退回稳定标识；路径末段是最后的兜底。
function revisionEntity(raw:string,path:string):{name:string;content:string}{
 const fallback=path.split('/@').pop()||path;
 let value:unknown;
 try{value=JSON.parse(raw);}catch{return {name:fallback,content:raw};}
 if(!value||typeof value!=='object'||Array.isArray(value))return {name:fallback,content:raw};
 const entity=value as Record<string,unknown>;
 const name=['name','title','id'].map(key=>entity[key]).find(item=>typeof item==='string'&&item.trim());
 return {name:typeof name==='string'?name.trim():fallback,content:JSON.stringify(entity,null,2)};
}
export function RevisionPanel({editor,actions,structure,disabled,preview,onPreview,onConfirm,onReject,onApply}:Props){
 const [draft,setDraft]=useState({base:editor?.candidateId??'',path:editor?.fields?.find(f=>!f.locked)?.path??'',text:'',key:crypto.randomUUID()});
 const [busy,setBusy]=useState(false);const [error,setError]=useState('');
 // 结构授权默认全部不勾选：未勾选的集合一旦被 AI 改动，整个修订会被拒绝。
 const [allow,setAllow]=useState<{add:string[];remove:string[]}>({add:[],remove:[]});
 const toggleAllow=(kind:'add'|'remove',collection:string)=>setAllow(old=>({...old,[kind]:old[kind].includes(collection)?old[kind].filter(item=>item!==collection):[...old[kind],collection]}));
 const allowRequest:RevisionAllow={...(allow.add.length?{allowAdd:allow.add}:{}),...(allow.remove.length?{allowRemove:allow.remove}:{})};
 // 勾选按「被取消勾选的路径」记录：默认全部勾选，刷新后新出现的差异仍默认勾选。
 const [deselected,setDeselected]=useState<Record<string,string[]>>({});
 const toggleDiff=(actionId:string,path:string)=>setDeselected(old=>{const current=old[actionId]??[];return {...old,[actionId]:current.includes(path)?current.filter(item=>item!==path):[...current,path]};});
 const act=async(work:()=>Promise<void>)=>{setBusy(true);setError('');try{await work();}catch(e){setError(e instanceof Error?e.message:'操作失败，请重试。');}finally{setBusy(false);}};
 const visible=actions.filter(a=>a.state!=='rejected');
 const staleDraft=!!draft.text&&draft.base!==editor?.candidateId;
 const fields=editor?.fields ?? [];
 const collections=structure ?? [];
 const path=draft.base===editor?.candidateId?draft.path:fields.find(f=>!f.locked)?.path??'';
 return <section aria-label="AI 定向修改" className="mb-6 rounded-xl border border-border bg-surface p-4 space-y-4">
  <h3 className="text-sm font-medium text-text">让 AI 修改指定内容</h3>
  <p className="text-xs text-text-sub leading-relaxed">选定要改的文本，说明你的要求。其余内容保持不变；生成后先看差异与独立审阅，再决定是否采用。</p>
  {preview&&<p className="text-xs text-text-muted">原型演示：仅本地模拟，不调用模型。</p>}
  {editor&&<><label className="block text-sm">修改范围<select aria-label="AI 修改范围" className="mt-2 w-full rounded-md border border-border bg-surface p-2" disabled={disabled||busy||staleDraft} value={path} onChange={e=>setDraft(old=>({...old,base:editor.candidateId,path:e.target.value,key:crypto.randomUUID()}))}>{fields.map(f=><option key={f.path} value={f.path} disabled={f.locked}>{f.group} · {f.label}{f.locked?'（已锁定）':''}</option>)}</select></label>
  <Textarea aria-label="修改要求" placeholder="例如：保留人物动机，让这段冲突更紧凑。" maxLength={4000} value={draft.text} disabled={disabled||busy} onChange={e=>setDraft(old=>({...old,base:old.text?old.base:editor.candidateId,path,text:e.target.value,key:crypto.randomUUID()}))}/>
  {collections.length>0&&<details className="rounded-lg border border-border p-3">
   <summary aria-label="允许 AI 增删实体（可选）" className="cursor-pointer text-xs font-medium text-text-sub">允许 AI 增删实体（可选）</summary>
   <p className="mt-2 text-xs leading-relaxed text-text-sub">默认不允许任何结构增删。只有在这里勾选的集合，AI 才能在该次修订内新增或删除实体；未勾选的集合若被 AI 改动，整个修订会被拒绝。授权只在该次修订内有效。</p>
   <div className="mt-3 grid gap-3 md:grid-cols-2">
    <fieldset><legend className="text-xs font-medium text-text">允许新增</legend>
     <div className="mt-1 space-y-1">{collections.map(c=><label key={c.collection} className="flex items-center gap-2 cursor-pointer"><input type="checkbox" className="h-4 w-4 accent-primary" aria-label={`允许新增 ${c.title}`} checked={allow.add.includes(c.collection)} disabled={disabled||busy} onChange={()=>toggleAllow('add',c.collection)}/><span className="text-xs">{c.title}</span></label>)}</div>
    </fieldset>
    <fieldset><legend className="text-xs font-medium text-text">允许删除</legend>
     <div className="mt-1 space-y-1">{collections.map(c=><label key={c.collection} className="flex items-center gap-2 cursor-pointer"><input type="checkbox" className="h-4 w-4 accent-primary" aria-label={`允许删除 ${c.title}`} checked={allow.remove.includes(c.collection)} disabled={disabled||busy} onChange={()=>toggleAllow('remove',c.collection)}/><span className="text-xs">{c.title}</span></label>)}</div>
    </fieldset>
   </div>
  </details>}
  {staleDraft?<p role="alert" className="text-sm text-warning">候选已更新，输入已保留。<Button onClick={()=>setDraft(old=>({...old,base:editor.candidateId,path,key:crypto.randomUUID()}))}>核对后使用当前候选</Button></p>:<Button disabled={disabled||busy||!draft.text.trim()||!path||fields.find(f=>f.path===path)?.locked} onClick={()=>void act(async()=>{await onPreview(editor.candidateId,path,draft.text,draft.key,allowRequest);setDraft(old=>({...old,text:'',key:crypto.randomUUID()}));})}>查看修改提案与费用</Button>}
  </>}
  {visible.map(action=><article key={action.actionId} aria-label={`修订结果 第 ${action.version} 版`} className="rounded-lg border border-border p-3 space-y-3">
   <div className="flex flex-wrap items-center justify-between gap-2"><h4 className="text-sm font-medium">{action.applied?'已采用，待阶段确认':labels[action.state]??'待处理'} · 第 {action.version} 版</h4>{action.stale&&!action.applied&&<span className="text-xs text-warning">基线已变化</span>}</div>
   <p className="text-sm whitespace-pre-wrap">{action.instruction}</p>
   <p className="text-xs text-text-sub">修改：{action.targets.map(f=>`${f.group} · ${f.label}`).join('、')}。其余文本、结构与顺序保持不变。</p>
   {!!action.preserved.length&&<p className="text-xs text-text-sub">锁定保留：{action.preserved.map(f=>f.label).join('、')}</p>}
   {action.state==='pending_confirmation'&&<><div className="rounded-md bg-bg p-3 text-xs leading-relaxed space-y-1">{action.cost.roleModels.map((m,i)=><p key={i}>{m.role==='planner'?'修订':'独立审阅'}：{m.provider} / {m.model}</p>)}<p>预计 {action.cost.estimatedCalls} 次调用，格式修复可能增加调用。{action.cost.runBudgetUnlimited?'当前方案未设置总用量上限。':`本次预算上限 ${action.cost.maxRunTokens.toLocaleString()} Token。`}</p><p>金额暂不可估算，确认后按你的模型服务商实际用量计费。</p></div><div className="flex gap-2"><Button variant="primary" disabled={disabled||busy||action.stale} onClick={()=>void act(()=>onConfirm(action))}>确认费用并开始修改</Button><Button disabled={busy||disabled} onClick={()=>void act(()=>onReject(action.actionId))}>放弃此提案</Button></div></>}
   {action.state==='expired'&&<Button disabled={disabled||busy} onClick={()=>void act(()=>onReject(action.actionId))}>关闭过期提案</Button>}
   {action.summary&&<p className="text-xs text-text-sub whitespace-pre-wrap">{action.summary}</p>}
   {action.applied&&<p className="text-xs text-text-sub">{action.appliedPaths&&action.appliedPaths.length<action.diffs.length?`已采用选中的 ${action.appliedPaths.length} 项修改 · 其余字段保持基线原文，生成待复核的新候选。`:'已采用全部修改为新候选。'}</p>}
   {action.state==='pending_confirmation'&&<p className="text-xs text-text-sub">{action.allow&&((action.allow.allowAdd??[]).length||(action.allow.allowRemove??[]).length)?`本次修订授权：${[...(action.allow.allowAdd??[]).map(c=>`允许新增 ${structureCollectionTitle(c)}`),...(action.allow.allowRemove??[]).map(c=>`允许删除 ${structureCollectionTitle(c)}`)].join('、')}。授权只在该次修订内有效。`:'本次修订未授权任何结构增删，若出现结构改动会被拒绝。'}</p>}
   {action.diffs.map(diff=>{const selectable=action.state==='succeeded'&&!action.applied;const entity=diff.kind==='text'?null:revisionEntity(diff.kind==='add_entity'?diff.after:diff.before,diff.path);const collection=diff.path.split('/')[1]??'';const heading=[diff.group,diff.label].filter(Boolean).join(' · ')||(entity?`${structureCollectionTitle(collection)} · ${diff.kind==='add_entity'?'新增':'删除'} ${entity.name}`:diff.path);return <div key={diff.path} className="space-y-2">{selectable?<label className="flex items-center gap-2 cursor-pointer"><input type="checkbox" className="h-4 w-4 accent-primary" aria-label={heading} checked={!(deselected[action.actionId]??[]).includes(diff.path)} disabled={disabled||busy||!action.canApply} onChange={()=>toggleDiff(action.actionId,diff.path)}/><span className="text-xs font-medium">{heading}</span></label>:<h5 className="text-xs font-medium">{heading}</h5>}{entity?<><p className="flex flex-wrap items-center gap-2 text-xs"><span className="rounded-full bg-primary-subtle px-2 py-0.5 font-medium text-text">{diff.kind==='add_entity'?'新增实体':'删除实体'}</span><span className="font-medium text-text">{entity.name}</span></p><pre className="overflow-auto whitespace-pre-wrap rounded-md bg-bg p-3 font-mono text-xs">{entity.content}</pre></>:<div className="grid gap-2 md:grid-cols-2 text-sm"><div className="rounded-md bg-bg p-3 whitespace-pre-wrap"><span className="block text-xs text-text-muted mb-1">修改前</span>{diff.before}</div><div className="rounded-md bg-primary-subtle p-3 whitespace-pre-wrap"><span className="block text-xs text-text-muted mb-1">修改后</span>{diff.after}</div></div>}</div>;})}
   {action.state==='succeeded'&&!action.applied&&(action.canApply?(()=>{const dropped=deselected[action.actionId]??[];const paths=action.diffs.filter(d=>!dropped.includes(d.path)).map(d=>d.path);const structureDropped=action.diffs.some(d=>d.kind!=='text'&&dropped.includes(d.path));return <div className="space-y-2">{structureDropped&&<p className="text-xs text-warning">结构增删必须整组采用</p>}{paths.length<action.diffs.length&&<p className="text-xs text-text-sub">只采用选中的修改会生成待复核的新候选，其余字段保持基线原文。</p>}<div className="flex flex-wrap gap-2"><Button variant="primary" aria-label="全部采用" disabled={disabled||busy} onClick={()=>void act(()=>onApply(action))}>全部采用</Button><Button aria-label={`采用选中的 ${paths.length} 项修改`} disabled={disabled||busy||paths.length===0||structureDropped} onClick={()=>void act(()=>onApply(action,paths))}>采用选中的 {paths.length} 项修改</Button></div></div>;})():<Button variant="primary" disabled={disabled||busy||!action.canApply}>{action.quality!=='passed'?'审阅未通过，暂不可采用':'采用修改为新候选'}</Button>)}
  </article>)}
  {error&&<p role="alert" className="text-sm text-danger">{error}</p>}
 </section>;
}
