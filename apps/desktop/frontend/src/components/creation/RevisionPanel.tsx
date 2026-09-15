import { useState } from 'react';
import { Button } from '../ui/Button';
import { Textarea } from '../ui/Input';
import type { FieldEditor } from './CandidateFields';

export interface RevisionCard {
 actionId:string;argumentsHash:string;idempotencyKey:string;candidateId:string;version:number;state:string;instruction:string;
 targets:FieldEditor['fields'];preserved:FieldEditor['fields'];cost:{estimatedCalls:number;maxRunTokens:number;runBudgetUnlimited:boolean;roleModels:{role:string;provider:string;model:string}[]};
 runId:string;quality:string;summary:string;stale:boolean;canApply:boolean;applied:boolean;
 diffs:{label:string;group:string;before:string;after:string}[];
}
interface Props {editor?:FieldEditor;actions:RevisionCard[];disabled?:boolean;preview?:boolean;onPreview:(candidateId:string,path:string,instruction:string,key:string)=>Promise<void>;onConfirm:(action:RevisionCard)=>Promise<void>;onReject:(id:string)=>Promise<void>;onApply:(action:RevisionCard)=>Promise<void>}
const labels:Record<string,string>={pending_confirmation:'待你确认',executing:'正在修订与审阅',succeeded:'修订结果',failed:'运行已停止',expired:'提案已过期',rejected:'已放弃',conflict:'内容已变化',outcome_unknown:'需要核对运行结果'};
export function RevisionPanel({editor,actions,disabled,preview,onPreview,onConfirm,onReject,onApply}:Props){
 const [draft,setDraft]=useState({base:editor?.candidateId??'',path:editor?.fields?.find(f=>!f.locked)?.path??'',text:'',key:crypto.randomUUID()});
 const [busy,setBusy]=useState(false);const [error,setError]=useState('');
 const act=async(work:()=>Promise<void>)=>{setBusy(true);setError('');try{await work();}catch(e){setError(e instanceof Error?e.message:'操作失败，请重试。');}finally{setBusy(false);}};
 const visible=actions.filter(a=>a.state!=='rejected');
 const staleDraft=!!draft.text&&draft.base!==editor?.candidateId;
 const fields=editor?.fields ?? [];
 const path=draft.base===editor?.candidateId?draft.path:fields.find(f=>!f.locked)?.path??'';
 return <section aria-label="AI 定向修改" className="mb-6 rounded-xl border border-border bg-surface p-4 space-y-4">
  <h3 className="text-sm font-medium text-text">让 AI 修改指定内容</h3>
  <p className="text-xs text-text-sub leading-relaxed">选定要改的文本，说明你的要求。其余内容保持不变；生成后先看差异与独立审阅，再决定是否采用。</p>
  {preview&&<p className="text-xs text-text-muted">原型演示：仅本地模拟，不调用模型。</p>}
  {editor&&<><label className="block text-sm">修改范围<select aria-label="AI 修改范围" className="mt-2 w-full rounded-md border border-border bg-surface p-2" disabled={disabled||busy||staleDraft} value={path} onChange={e=>setDraft(old=>({...old,base:editor.candidateId,path:e.target.value,key:crypto.randomUUID()}))}>{fields.map(f=><option key={f.path} value={f.path} disabled={f.locked}>{f.group} · {f.label}{f.locked?'（已锁定）':''}</option>)}</select></label>
  <Textarea aria-label="修改要求" placeholder="例如：保留人物动机，让这段冲突更紧凑。" maxLength={4000} value={draft.text} disabled={disabled||busy} onChange={e=>setDraft(old=>({...old,base:old.text?old.base:editor.candidateId,path,text:e.target.value,key:crypto.randomUUID()}))}/>
  {staleDraft?<p role="alert" className="text-sm text-warning">候选已更新，输入已保留。<Button onClick={()=>setDraft(old=>({...old,base:editor.candidateId,path,key:crypto.randomUUID()}))}>核对后使用当前候选</Button></p>:<Button disabled={disabled||busy||!draft.text.trim()||!path||fields.find(f=>f.path===path)?.locked} onClick={()=>void act(async()=>{await onPreview(editor.candidateId,path,draft.text,draft.key);setDraft(old=>({...old,text:'',key:crypto.randomUUID()}));})}>查看修改提案与费用</Button>}
  </>}
  {visible.map(action=><article key={action.actionId} className="rounded-lg border border-border p-3 space-y-3">
   <div className="flex flex-wrap items-center justify-between gap-2"><h4 className="text-sm font-medium">{action.applied?'已采用，待阶段确认':labels[action.state]??'待处理'} · 第 {action.version} 版</h4>{action.stale&&!action.applied&&<span className="text-xs text-warning">基线已变化</span>}</div>
   <p className="text-sm whitespace-pre-wrap">{action.instruction}</p>
   <p className="text-xs text-text-sub">修改：{action.targets.map(f=>`${f.group} · ${f.label}`).join('、')}。其余文本、结构与顺序保持不变。</p>
   {!!action.preserved.length&&<p className="text-xs text-text-sub">锁定保留：{action.preserved.map(f=>f.label).join('、')}</p>}
   {action.state==='pending_confirmation'&&<><div className="rounded-md bg-bg p-3 text-xs leading-relaxed space-y-1">{action.cost.roleModels.map((m,i)=><p key={i}>{m.role==='planner'?'修订':'独立审阅'}：{m.provider} / {m.model}</p>)}<p>预计 {action.cost.estimatedCalls} 次调用，格式修复可能增加调用。{action.cost.runBudgetUnlimited?'当前方案未设置总用量上限。':`本次预算上限 ${action.cost.maxRunTokens.toLocaleString()} Token。`}</p><p>金额暂不可估算，确认后按你的模型服务商实际用量计费。</p></div><div className="flex gap-2"><Button variant="primary" disabled={disabled||busy||action.stale} onClick={()=>void act(()=>onConfirm(action))}>确认费用并开始修改</Button><Button disabled={busy||disabled} onClick={()=>void act(()=>onReject(action.actionId))}>放弃此提案</Button></div></>}
   {action.state==='expired'&&<Button disabled={disabled||busy} onClick={()=>void act(()=>onReject(action.actionId))}>关闭过期提案</Button>}
   {action.summary&&<p className="text-xs text-text-sub whitespace-pre-wrap">{action.summary}</p>}
   {action.diffs.map((diff,i)=><div key={i} className="space-y-2"><h5 className="text-xs font-medium">{diff.group} · {diff.label}</h5><div className="grid gap-2 md:grid-cols-2 text-sm"><div className="rounded-md bg-bg p-3 whitespace-pre-wrap"><span className="block text-xs text-text-muted mb-1">修改前</span>{diff.before}</div><div className="rounded-md bg-primary-subtle p-3 whitespace-pre-wrap"><span className="block text-xs text-text-muted mb-1">修改后</span>{diff.after}</div></div></div>)}
   {action.state==='succeeded'&&!action.applied&&<Button variant="primary" disabled={disabled||busy||!action.canApply} onClick={()=>void act(()=>onApply(action))}>{action.quality!=='passed'?'审阅未通过，暂不可采用':'采用修改为新候选'}</Button>}
  </article>)}
  {error&&<p role="alert" className="text-sm text-danger">{error}</p>}
 </section>;
}
