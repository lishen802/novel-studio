import type {ConstraintAPI} from '../components/ConstraintChanges';
import type {ConstraintView} from '../types/constraint';
const rows:ConstraintView[]=[{assetId:'rule',kind:'world_rule',name:'潮汐航路',original:'潮汐决定航路',current:'潮汐决定航路',revision:1,locked:false,changes:[]}];
/** 本地交互样例不调用模型，也不写入正式项目。 */
export const constraintAPI:ConstraintAPI={
 constraints:async()=>[...rows],
 listChapters:async()=>[{id:'future',title:'后续空章节',position:8,body:'',revision:1,projectId:'demo',updatedAt:''}],
 previewConstraintChange:async request=>({request,digest:'prototype',before:rows[0].current,revision:rows[0].revision+1,position:8,protectedChapters:7,futureChapters:1,canApply:rows[0].changes.length===0,blockedReason:'同一生效边界已存在，本地样例仅演示一次修订'}),
 applyConstraintChange:async command=>{const p=await constraintAPI.previewConstraintChange(command.request);rows[0]={...rows[0],current:command.request.text,revision:p.revision,changes:[...rows[0].changes,{assetId:'rule',revision:p.revision,chapterId:'future',position:8,text:command.request.text,reason:command.request.reason}]};return p;},
};
