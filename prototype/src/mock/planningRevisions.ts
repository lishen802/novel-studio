import { chapters } from './data';
import type { ConstraintView, ConstraintRequest, ConstraintCommand, ConstraintPreview, ChapterView } from '../types/constraint';
const values = [
 {kind:'proposal', name:'雾中灯塔', fields:[{path:'/title',label:'标题',value:'雾中灯塔'},{path:'/core_conflict',label:'核心冲突',value:'真相与遗忘的对抗'},{path:'/ending',label:'结局方向',value:'带着真相离开灯塔'}]},
 {kind:'outline_node', name:'第一幕 · 雾起', fields:[{path:'/title',label:'标题',value:'第一幕 · 雾起'},{path:'/goal',label:'目标',value:'找到失踪守塔人的线索'},{path:'/planning/reader_expectation',label:'读者期待',value:'灯塔为何不亮'},{path:'/planning/commitments/0/statement',label:'承诺 1 · 内容',value:'揭开未寄出的信的来源'}]},
];
const rows:ConstraintView[]=values.map((value,index)=>{const text=JSON.stringify(Object.fromEntries(value.fields.map((field)=>[field.path,field.value])));return {assetId:`planning-demo-${index}`,typedId:`node-${index}`,kind:value.kind,name:value.name,original:text,current:text,revision:1,locked:false,fields:value.fields,changes:[]};});
const receipts=new Map<string,ConstraintPreview>();
export const planningRevisionDemo={
 planningRevisions:async()=>structuredClone(rows),
 listChapters:async()=>chapters.map((chapter,index)=>({id:chapter.id,title:chapter.title,position:index+1})) as ChapterView[],
 previewConstraintChange:async(request:ConstraintRequest):Promise<ConstraintPreview>=>{const item=rows.find((row)=>row.assetId===request.assetId)!;const position=chapters.findIndex((chapter)=>chapter.id===request.chapterId)+1;return {request,digest:JSON.stringify(request),before:item.current,revision:item.revision+1,position,protectedChapters:position-1,futureChapters:chapters.length-position+1,canApply:request.text!==item.current && position>(item.changes.at(-1)?.position || 0),blockedReason:'新旧文字相同或边界没有递增'};},
 applyConstraintChange:async(command:ConstraintCommand):Promise<ConstraintPreview>=>{const old=receipts.get(command.idempotencyKey);if(old){if(old.digest!==command.digest)throw new Error('确认参数已变化');return old;}const preview=await planningRevisionDemo.previewConstraintChange(command.request);if(!preview.canApply || preview.digest!==command.digest)throw new Error('原型预览已过期');const item=rows.find((row)=>row.assetId===command.request.assetId)!;item.current=command.request.text;item.revision=preview.revision;item.changes.push({assetId:item.assetId,revision:preview.revision,chapterId:command.request.chapterId,position:preview.position,text:command.request.text,reason:command.request.reason});receipts.set(command.idempotencyKey,preview);return preview;},
};
