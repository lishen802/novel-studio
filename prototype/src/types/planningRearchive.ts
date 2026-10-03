export type { ChapterView } from './constraint';
export interface PlanningRearchivePreview {request:{projectId:string;branchId:string;chapterId:string;reason:string};digest:string;position:number;protectedChapters:number;futureChapters:number;changes:Array<{assetId:string;kind:string;before:string;after:string}>}
