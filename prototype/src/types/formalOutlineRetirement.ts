export interface ChapterView {id:string;projectId:string;title:string;position:number;body:string;revision:number;updatedAt:string;hasRecoveryDraft:boolean}
export interface ConstraintView {assetId:string;kind:string;typedId?:string}
export interface FormalOutlineRetireRequest {projectId:string;branchId:string;assetId:string;chapterId:string;reason:string;unlinkStoryLineId?:string;unlinkStoryLineIds?:string[]}
export interface FormalOutlineRetirePreview {request:FormalOutlineRetireRequest;digest:string;nodeId:string;title:string;introducedAt:number;position:number;unlinkedStoryLine?:{storyLineId:string;name:string;position:number;createdAt:string};unlinkedStoryLines?:Array<{storyLineId:string;name:string;position:number;createdAt:string}>}
export interface FormalOutlineReferenceView {assetId:string;nodeId:string;title:string;references:Array<{kind:string;id:string;label:string}>}
