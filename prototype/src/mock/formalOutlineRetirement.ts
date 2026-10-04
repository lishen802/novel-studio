import type { FormalOutlineRetireAPI } from '../components/FormalOutlineRetire';

// 原型仅演示停用预览和确认，不写入正式项目。
export const formalOutlineRetirementDemo: FormalOutlineRetireAPI = {
  listChapters: async () => [{id:'later',projectId:'demo',title:'后续空章',position:13,body:'',revision:1,updatedAt:'',hasRecoveryDraft:false}],
  planningRevisions: async () => [{assetId:'demo-new',kind:'outline_node',typedId:'demo-node'},{assetId:'demo-target-asset',kind:'outline_node',typedId:'demo-target'}],
  formalOutlineReferences: async assetId => ({assetId,nodeId:'demo-node',title:'近期章节目标',references:[{kind:'story_line',id:'demo-story-line-a',label:'主线'}]}),
  previewFormalOutlineRetire: async (assetId, chapterId, reason, unlinkStoryLineId, unlinkStoryLineIds, rebindStoryLineId, rebindTargetAssetId) => ({request:{projectId:'demo',branchId:'main',assetId,chapterId,reason,unlinkStoryLineId,unlinkStoryLineIds,rebindStoryLineId,rebindTargetAssetId},digest:'demo-retirement',nodeId:'demo-node',title:'近期章节目标',introducedAt:0,position:13,...(rebindTargetAssetId?{reboundStoryLine:{storyLineId:rebindStoryLineId,name:'主线',position:1,createdAt:'2026-01-01T00:00:00Z',targetAssetId:rebindTargetAssetId,targetNodeId:'demo-target',targetTitle:'新章节目标',targetRevision:1,targetParentId:'demo-arc',targetIntroducedAt:0}}:{unlinkedStoryLine:{storyLineId:unlinkStoryLineId,name:'主线',position:1,createdAt:'2026-01-01T00:00:00Z'}})}),
  confirmFormalOutlineRetire: async command => ({request:command.request,digest:command.digest,nodeId:'demo-node',title:'近期章节目标',introducedAt:0,position:13}),
};
