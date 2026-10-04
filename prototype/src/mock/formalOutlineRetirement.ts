import type { FormalOutlineRetireAPI } from '../components/FormalOutlineRetire';

// 原型仅演示停用预览和确认，不写入正式项目。
export const formalOutlineRetirementDemo: FormalOutlineRetireAPI = {
  listChapters: async () => [{id:'later',projectId:'demo',title:'后续空章',position:13,body:'',revision:1,updatedAt:'',hasRecoveryDraft:false}],
  planningRevisions: async () => [{assetId:'demo-new',kind:'outline_node',typedId:'demo-node'},{assetId:'demo-target-asset',kind:'outline_node',typedId:'demo-target'}],
  formalOutlineReferences: async assetId => ({assetId,nodeId:'demo-node',title:'近期章节目标',references:[{kind:'story_line',id:'demo-story-line-a',label:'主线'},{kind:'story_line',id:'demo-story-line-b',label:'支线'}]}),
  previewFormalOutlineRetire: async (assetId, chapterId, reason, unlinkStoryLineId, unlinkStoryLineIds, rebindStoryLineId, rebindTargetAssetId, rebindStoryLineIds) => ({request:{projectId:'demo',branchId:'main',assetId,chapterId,reason,unlinkStoryLineId,unlinkStoryLineIds,rebindStoryLineId,rebindStoryLineIds,rebindTargetAssetId},digest:'demo-retirement',nodeId:'demo-node',title:'近期章节目标',introducedAt:0,position:13,...(rebindTargetAssetId?{reboundStoryLines:rebindStoryLineIds.map((storyLineId,index)=>({storyLineId,name:index===0?'主线':'支线',position:1,createdAt:'2026-01-01T00:00:00Z',targetAssetId:rebindTargetAssetId,targetNodeId:'demo-target',targetTitle:'新章节目标',targetRevision:1,targetParentId:'demo-arc',targetIntroducedAt:0}))}:{unlinkedStoryLines:unlinkStoryLineIds.map((storyLineId,index)=>({storyLineId,name:index===0?'主线':'支线',position:1,createdAt:'2026-01-01T00:00:00Z'}))})}),
  confirmFormalOutlineRetire: async command => ({request:command.request,digest:command.digest,nodeId:'demo-node',title:'近期章节目标',introducedAt:0,position:13}),
};
