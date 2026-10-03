import { characters, chapters } from './data';
import type { ConstraintView, ConstraintRequest, ConstraintCommand, ConstraintPreview, ChapterView } from '../types/constraint';
const rows: ConstraintView[] = characters.map((character) => {
 const profile = JSON.stringify({name:character.name,aliases:character.tags,role:character.role,summary:character.summary,motivation:character.motivation,fear:character.fear || '',secret:character.secret || '',arc:character.arc || ''});
 return {assetId:`formal-${character.id}`,typedId:character.id,kind:'character',name:character.name,original:profile,current:profile,revision:1,locked:false,changes:[],candidateSuggestions:[{candidateId:`demo-${character.id}`,entityId:character.id,actionId:"demo-review",text:JSON.stringify({...JSON.parse(profile),motivation:"保护同伴并承担选择的代价"})}]};
});
const receipts = new Map<string, ConstraintPreview>();
export const characterRevisionDemo = {
 previewCharacterRevision: async (request:{assetId:string;expectedRevision:number;field:string;instruction:string;idempotencyKey:string}):Promise<RevisionCard> => {
  const old=aiActions.find(action=>action.idempotencyKey===request.idempotencyKey);if(old){if(old.argumentsHash!==JSON.stringify(request))throw new Error('演示提案参数冲突');return structuredClone(old);}
  const item=rows.find(row=>row.assetId===request.assetId);if(!item || item.locked || item.revision!==request.expectedRevision)throw new Error('人物基线已变化');
  const value=JSON.parse(item.current)[request.field] || '';
  const action:RevisionCard={formalAssetId:item.assetId,formalRevision:item.revision,actionId:crypto.randomUUID(),argumentsHash:JSON.stringify(request),idempotencyKey:request.idempotencyKey,candidateId:'',version:item.revision,state:'pending_confirmation',instruction:request.instruction,targets:[{ownerRef:item.typedId || item.assetId,path:request.field,label:labels[request.field],group:'人物',value,locked:false}],preserved:[],cost:{estimatedCalls:2,maxRunTokens:30000,runBudgetUnlimited:false,roleModels:[{role:'planner',provider:'原型演示',model:'不调用模型'},{role:'continuity_reviewer',provider:'原型演示',model:'不调用模型'}]},runId:'',quality:'',summary:'原型仅演示费用确认与候选交互。',stale:false,canApply:false,applied:false,diffs:[]};aiActions.unshift(action);return structuredClone(action);
 },
 characterRevisionActions: async ():Promise<RevisionCard[]> => structuredClone(aiActions.map(action=>({...action,stale:rows.find(row=>row.assetId===action.formalAssetId)?.revision!==action.formalRevision}))),
 confirmCreationRevision: async (command:{actionId:string;argumentsHash:string;idempotencyKey:string}):Promise<RevisionCard> => {
  const action=aiActions.find(item=>item.actionId===command.actionId)!;if(action.argumentsHash!==command.argumentsHash || action.idempotencyKey!==command.idempotencyKey)throw new Error('演示费用提案已变化');
  if(action.state==='succeeded')return structuredClone(action);
  const item=rows.find(row=>row.assetId===action.formalAssetId)!;if(item.revision!==action.formalRevision || item.locked)throw new Error('正式基线已变化');
  const target=action.targets[0];const after=target.value+'（演示：'+action.instruction+'）';const profile={...JSON.parse(item.current),[target.path]:after};
  action.state='succeeded';action.quality='passed';action.runId='demo-run';action.canApply=true;action.diffs=[{path:target.path,label:target.label,group:'人物',kind:'text',before:target.value,after}];action.summary='原型候选演示；没有调用真实模型。';
  item.candidateSuggestions=[{candidateId:'formal-revision:'+action.actionId,entityId:item.typedId || item.assetId,actionId:action.actionId,text:JSON.stringify(profile)}];return structuredClone(action);
 },
 rejectCreationRevision: async(id:string)=>{const action=aiActions.find(item=>item.actionId===id)!;action.state='rejected';return {};},
 characterRevisions: async () => structuredClone(rows),
 listChapters: async () => chapters.map((chapter,index) => ({id:chapter.id,title:chapter.title,position:index+1})) as ChapterView[],
 previewConstraintChange: async (request:ConstraintRequest):Promise<ConstraintPreview> => {
  const item=rows.find((row)=>row.assetId===request.assetId)!;
  if(request.candidateId && !item.candidateSuggestions?.some((suggestion)=>suggestion.candidateId===request.candidateId && suggestion.text===request.text))throw new Error('演示候选来源已变化，请重新选择');
  const position=chapters.findIndex((chapter)=>chapter.id===request.chapterId)+1;
  return {request,digest:JSON.stringify(request),before:item.current,revision:item.revision+1,position,protectedChapters:position-1,futureChapters:chapters.length-position+1,canApply:request.text!==item.current && position>(item.changes.at(-1)?.position || 0),blockedReason:'新旧档案相同或边界没有递增'};
 },
 applyConstraintChange: async (command:ConstraintCommand):Promise<ConstraintPreview> => {
  const old=receipts.get(command.idempotencyKey);if(old)return old;
  const preview=await characterRevisionDemo.previewConstraintChange(command.request);
  if(!preview.canApply || preview.digest!==command.digest)throw new Error('原型预览已过期');
  const item=rows.find((row)=>row.assetId===command.request.assetId)!;
  item.candidateSuggestions=[];item.current=command.request.text;item.revision=preview.revision;
  item.changes.push({assetId:item.assetId,revision:preview.revision,chapterId:command.request.chapterId,position:preview.position,text:command.request.text,reason:command.request.reason,...(command.request.candidateId ? {candidateId:command.request.candidateId} : {})});
  receipts.set(command.idempotencyKey,preview);return preview;
 },
};

// 正式人物生成演示只保存本地状态，不调用模型或写入正式数据库。
import type { RevisionCard } from '../types/formalCharacter';
const aiActions: RevisionCard[] = [];
const labels: Record<string,string> = {name:'姓名',summary:'简介',motivation:'动机',fear:'恐惧',secret:'秘密',arc:'人物弧'};
