import { chapters } from './data';
import type { PlanningRearchiveAPI } from '../components/PlanningRearchive';
import type { PlanningRearchivePreview } from '../types/planningRearchive';

let last:PlanningRearchivePreview|null=null;
export const planningRearchiveDemo:PlanningRearchiveAPI={
  listChapters:async()=>chapters.map((chapter,index)=>({...chapter,position:index+1,projectId:'demo',revision:1,updatedAt:'',body:''})),
  previewPlanningRearchive:async(chapterId,reason)=>{
    if(!chapterId || !reason.trim()) throw new Error('请选择章节并说明理由。');
    last={request:{projectId:'demo',branchId:'main',chapterId,reason},digest:`demo-${chapterId}-${reason}`,position:4,protectedChapters:3,futureChapters:1,changes:[{assetId:'demo-proposal',kind:'proposal',before:JSON.stringify({'/title':'雾中灯塔','/sell_point':'追寻旧信'}),after:JSON.stringify({'/title':'雾中灯塔','/sell_point':'追寻旧信，也守住同伴'})}]};
    return last;
  },
  confirmPlanningRearchive:async({request,digest})=>{if(!last || last.digest!==digest || last.request.chapterId!==request.chapterId)throw new Error('预览已变化，请重新核对。');return last;},
};
