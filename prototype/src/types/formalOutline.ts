export interface ChapterView {id:string;projectId:string;title:string;position:number;body:string;revision:number;updatedAt:string;hasRecoveryDraft:boolean}
export interface ConstraintView {assetId:string;kind:string;typedId?:string}
export interface FormalOutlineAppendRequest {projectId?:string;branchId?:string;parentAssetId:string;chapterId:string;title:string;summary:string;goal:string;conflict:string;outcome:string;reason:string}
export interface FormalOutlineAppendPreview {request:FormalOutlineAppendRequest;digest:string;position:number;nodePosition:number;parentTitle:string;assetId?:string;nodeId?:string}
