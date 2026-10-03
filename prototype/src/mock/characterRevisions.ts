import { characters, chapters } from './data';
import type { ConstraintView, ConstraintRequest, ConstraintCommand, ConstraintPreview, ChapterView } from '../types/constraint';
const rows: ConstraintView[] = characters.map((character) => {
 const profile = JSON.stringify({name:character.name,aliases:character.tags,role:character.role,summary:character.summary,motivation:character.motivation,fear:character.fear || '',secret:character.secret || '',arc:character.arc || ''});
 return {assetId:`formal-${character.id}`,typedId:character.id,kind:'character',name:character.name,original:profile,current:profile,revision:1,locked:false,changes:[],candidateSuggestions:[{candidateId:`demo-${character.id}`,entityId:character.id,actionId:"demo-review",text:JSON.stringify({...JSON.parse(profile),motivation:"保护同伴并承担选择的代价"})}]};
});
const receipts = new Map<string, ConstraintPreview>();
export const characterRevisionDemo = {
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
