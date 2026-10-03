import type { FormalOutlineAppendAPI } from '../components/FormalOutlineAppend';

// 原型只演示交互；正式写入由桌面版的项目数据库完成。
export const formalOutlineDemo:FormalOutlineAppendAPI={
  listChapters:async()=>[{id:'future',projectId:'demo',title:'下一章',position:12,body:'',revision:1,updatedAt:'',hasRecoveryDraft:false}],
  planningRevisions:async()=>[{assetId:'demo-arc-one',kind:'outline_node',typedId:'arc-1'},{assetId:'demo-arc-two',kind:'outline_node',typedId:'arc-2'},{assetId:'demo-arc-three',kind:'outline_node',typedId:'arc-3'}],
  previewFormalOutlineAppend:async request=>({request:{...request,projectId:'demo',branchId:'main'},digest:'demo-preview',position:12,nodePosition:4,parentTitle:'正式故事弧'}),
  confirmFormalOutlineAppend:async command=>({...await formalOutlineDemo.previewFormalOutlineAppend(command.request),assetId:'demo-new',nodeId:'demo-node'}),
};
