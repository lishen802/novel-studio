export interface ConstraintChange {assetId:string;revision:number;chapterId:string;position:number;text:string;reason:string}
export interface ConstraintView {assetId:string;kind:string;name:string;original:string;current:string;revision:number;locked:boolean;changes:ConstraintChange[]}
export interface ConstraintRequest {projectId?:string;branchId?:string;assetId:string;intent:string;chapterId:string;text:string;reason:string}
export interface ConstraintPreview {request:ConstraintRequest;digest:string;before:string;revision:number;position:number;protectedChapters:number;futureChapters:number;canApply:boolean;blockedReason:string}
export interface ConstraintCommand {request:ConstraintRequest;digest:string;idempotencyKey:string}

export interface ChapterView {id:string;title:string;position:number;body:string;revision:number;projectId:string;updatedAt:string}
