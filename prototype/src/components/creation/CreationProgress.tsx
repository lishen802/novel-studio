import { useEffect, useState } from 'react';
import { Check, Circle, Loader2, AlertCircle } from 'lucide-react';
import { Button } from '../ui/Button';

const stages: Record<string, [string, string]> = {
 context_preparing: ['整理创作素材', '正在整理本次需要的设定、约束与参考内容。'],
 story_planning: ['构思故事方案', '正在推敲人物、冲突与情节，整理成可供你选择的方案。'],
 chapter_directing: ['安排本章情节', '正在安排场景、人物行动与本章要推进的内容。'],
 professional_analysis: ['核对人物与设定', '正在检查人物状态和故事规则，为创作补齐依据。'],
 prose_writing: ['创作正文', '正在将情节写成正文，完成后会交给审阅环节检查。'],
 independent_review: ['审阅创作内容', '正在检查前后衔接、人物动机和表达，找出需要调整的地方。'],
 bounded_revision: ['完善创作内容', '正在根据审阅意见进行修改。'],
 revision_verify: ['复核修改结果', '正在检查修改是否解决问题，以及是否引入新的冲突。'],
 candidate_ready: ['候选内容已就绪', '你可以查看候选，再决定接受、修改或放弃。'],
 planner: ['安排本章情节', '正在整理本章的场景、人物行动与情节安排。'],
 writer: ['创作正文', '正在按本次要求生成正文候选。'],
 continuity_reviewer: ['审阅创作内容', '正在检查正文的前后衔接和设定一致性。'],
};
const statuses: Record<string, string> = { queued:'等待开始', pending:'等待开始', running:'进行中', completed:'已完成', failed:'未完成', cancelled:'已取消', skipped:'无需执行', recoverable:'等待恢复', needs_confirmation:'等待确认' };
const activeStatuses = ['queued','pending','running','cancelling'];
export interface ProgressStep { stage: string; status: string }
interface Props {
 status: string; stage?: string; steps?: ProgressStep[]; createdAt?: string; updatedAt?: string;
 onCancel?: () => Promise<void>; cancelLabel?: string; cancelDisabled?: boolean;
}
export function CreationProgress({status,stage,steps=[],createdAt,updatedAt,onCancel,cancelLabel='取消运行',cancelDisabled}: Props) {
 const [now,setNow]=useState(Date.now());
 const [cancelling,setCancelling]=useState(false);
 const [cancelError,setCancelError]=useState('');
 const active=activeStatuses.includes(status);
 useEffect(()=>{if(!active)return;setNow(Date.now());const timer=setInterval(()=>setNow(Date.now()),1000);return ()=>clearInterval(timer);},[active,createdAt]);
 const currentStage=stage || steps.find(step=>step.status==='running')?.stage || '';
 const [label,description]=stages[currentStage] ?? ['处理创作任务','正在处理本次创作请求，进展会在这里更新。'];
 const phase = status==='completed' ? '本次创作已完成' : status==='failed' ? '本次创作未完成' : status==='cancelled' ? '本次创作已取消' : status==='cancelling' ? '正在停止任务' : (status==='queued'||status==='pending') ? '准备开始创作' : status==='needs_confirmation' ? '等待你的确认' : !active ? '运行状态待确认' : label;
 const explanation=status==='completed' ? '请查看候选内容，是否采纳由你决定。' : status==='failed' ? '请查看错误提示，已有内容不会被覆盖。' : status==='cancelled' ? '任务已停止，已保存的内容仍会保留。' : status==='cancelling' ? '已提交取消请求，正在等待当前步骤停止。' : (status==='queued'||status==='pending') ? '任务已提交，正在等待开始。' : !active ? '请检查任务状态后再继续，不会自动重新生成。' : description;
 const completed=steps.filter(step=>step.status==='completed').length;
 const start=Date.parse(createdAt ?? '');
 const end=active ? now : Date.parse(updatedAt ?? '');
 const seconds=Number.isFinite(start)&&Number.isFinite(end) ? Math.max(0,Math.floor((end-start)/1000)) : null;
 const duration=seconds===null ? '' : seconds<60 ? `${seconds} 秒` : `${Math.floor(seconds/60)} 分 ${seconds%60} 秒`;
 const cancel=async()=>{setCancelling(true);setCancelError('');try{await onCancel?.();}catch(e){setCancelError(e instanceof Error ? e.message : '取消未成功，请重试。');}finally{setCancelling(false);}};
 return <section aria-label="创作进度" className="rounded-xl border border-border bg-surface p-4 space-y-4" data-testid="run-panel">
  <div className="flex items-center gap-2 text-sm font-medium text-text"><span className="inline-flex rounded-lg bg-primary-subtle p-2">{active ? <Loader2 className="h-4 w-4 motion-safe:animate-spin"/> : status==='completed' ? <Check className="h-4 w-4 text-success"/> : <AlertCircle className="h-4 w-4 text-warning"/>}</span>创作进度</div>
  <div role="status" aria-live="polite"><h4 className="text-sm font-medium text-text">{phase}</h4><p className="mt-1.5 text-xs leading-relaxed text-text-sub">{explanation}</p></div>
  {active && <div role="progressbar" aria-label="创作正在进行" aria-valuetext={phase} className="h-1.5 overflow-hidden rounded-full bg-primary-subtle"><div className="h-full w-1/3 rounded-full bg-primary/60 motion-safe:animate-[creation-progress_1.6s_ease-in-out_infinite]"/></div>}
  <div className="flex flex-wrap justify-between gap-2 text-xs text-text-muted"><span>{completed>0 ? `已完成 ${completed} 个步骤` : active ? '正在等待步骤进展' : '以当前任务状态为准'}</span>{duration && <span>{active ? '已等待' : '本次用时'} {duration}</span>}</div>
  {steps.length>0 && <ol aria-label="本次步骤" className="max-h-52 overflow-auto space-y-2 border-t border-border pt-3">{steps.map((step,index)=><li key={index} className="flex items-center gap-2 text-xs">{step.status==='completed' ? <Check className="h-3.5 w-3.5 shrink-0 text-success"/> : step.status==='running' && active ? <Loader2 className="h-3.5 w-3.5 shrink-0 text-primary motion-safe:animate-spin"/> : <Circle className="h-3.5 w-3.5 shrink-0 text-text-muted"/>}<span className="flex-1 text-text">{stages[step.stage]?.[0] ?? '处理创作内容'}</span><span className="text-text-muted">{step.status==='running'&&!active ? '已中断' : (step.status==='queued'||step.status==='pending')&&!active ? '未执行' : statuses[step.status] ?? '待确认'}</span></li>)}</ol>}
  {onCancel && active && <Button variant="ghost" size="sm" className="w-full border border-border text-text-sub" disabled={cancelling || cancelDisabled || status==='cancelling'} onClick={()=>void cancel()}>{cancelling || status==='cancelling' ? '正在取消…' : cancelLabel}</Button>}
  {cancelError && <p role="alert" className="text-xs text-danger">{cancelError}</p>}
 </section>;
}
