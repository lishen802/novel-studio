import type { AppSettings } from '../types';

/** 桌面服务返回前使用的空白设置表单，不代表任何已持久化配置。 */
export const defaultSettings: AppSettings = {
  models: [{
    id: 'm-new',
    name: '新模型连接',
    provider: 'openai',
    baseUrl: 'https://api.openai.com/v1',
    modelName: '',
    apiKeyConfigured: false,
  }],
  activeModelId: 'm-new',
  creativeProfiles: [],
  defaultCreativeProfileId: '',
  writingPlan: '尚未配置创作方案',
  theme: 'light',
  fontSize: 18,
  lineHeight: 1.9,
  canvasColorId: 'warm-white',
  marginSize: 'medium',
  indent: true,
};
