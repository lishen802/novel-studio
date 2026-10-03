import type { FormalOutlineRetireAPI } from '../components/FormalOutlineRetire';

// 原型仅演示停用预览和确认，不写入正式项目。
export const formalOutlineRetirementDemo: FormalOutlineRetireAPI = {
  listChapters: async () => [{id:'later',projectId:'demo',title:'后续空章',position:13,body:'',revision:1,updatedAt:'',hasRecoveryDraft:false}],
  planningRevisions: async () => [{assetId:'demo-new',kind:'outline_node',typedId:'demo-node'}],
  previewFormalOutlineRetire: async (assetId, chapterId, reason) => ({request:{projectId:'demo',branchId:'main',assetId,chapterId,reason},digest:'demo-retirement',nodeId:'demo-node',title:'近期章节目标',introducedAt:0,position:13}),
  confirmFormalOutlineRetire: async command => ({request:command.request,digest:command.digest,nodeId:'demo-node',title:'近期章节目标',introducedAt:0,position:13}),
};
