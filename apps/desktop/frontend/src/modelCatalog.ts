export interface CatalogModel {
  id: string;
  label: string;
}

export interface ModelVendor {
  id: string;
  label: string;
  protocol: 'openai' | 'anthropic';
  defaultBaseUrl: string;
  models: CatalogModel[];
  endpointHint?: string;
}

/**
 * 内置目录只提供公开协议、默认地址和常用模型快捷项；最终可用范围仍由用户账户决定。
 * 模型 ID 会变化，因此所有厂商都必须保留自定义模型入口。
 */
export const modelVendors: ModelVendor[] = [
  {
    id: 'openai',
    label: 'OpenAI',
    protocol: 'openai',
    defaultBaseUrl: 'https://api.openai.com/v1',
    models: [
      { id: 'gpt-4.1', label: 'GPT-4.1' },
      { id: 'gpt-4.1-mini', label: 'GPT-4.1 mini' },
      { id: 'gpt-4.1-nano', label: 'GPT-4.1 nano' },
    ],
  },
  {
    id: 'anthropic',
    label: 'Anthropic',
    protocol: 'anthropic',
    defaultBaseUrl: 'https://api.anthropic.com',
    models: [
      { id: 'claude-sonnet-4-6', label: 'Claude Sonnet 4.6' },
      { id: 'claude-opus-4-6', label: 'Claude Opus 4.6' },
      { id: 'claude-haiku-4-5', label: 'Claude Haiku 4.5' },
    ],
  },
  {
    id: 'deepseek',
    label: 'DeepSeek',
    protocol: 'openai',
    defaultBaseUrl: 'https://api.deepseek.com',
    models: [
      { id: 'deepseek-v4-flash', label: 'DeepSeek V4 Flash' },
      { id: 'deepseek-v4-pro', label: 'DeepSeek V4 Pro' },
    ],
  },
  {
    id: 'qwen',
    label: '阿里云百炼（千问）',
    protocol: 'openai',
    defaultBaseUrl: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    endpointHint: '专属业务空间或其他地域请按百炼控制台修改接口地址。',
    models: [
      { id: 'qwen3.8-max', label: 'Qwen3.8 Max' },
      { id: 'qwen3.7-plus', label: 'Qwen3.7 Plus' },
      { id: 'qwen3.7-flash', label: 'Qwen3.7 Flash' },
    ],
  },
  {
    id: 'custom-openai',
    label: '自定义 OpenAI 兼容',
    protocol: 'openai',
    defaultBaseUrl: '',
    models: [],
  },
];

export const defaultModelVendor = modelVendors[0];

export function getModelVendor(id: string): ModelVendor {
  return modelVendors.find((vendor) => vendor.id === id) || defaultModelVendor;
}

/** 根据已保存的协议和地址恢复内置厂商；未知地址归入自定义兼容服务。 */
export function inferModelVendor(protocol: string, baseUrl?: string): ModelVendor {
  const normalized = (baseUrl || '').toLowerCase();
  if (protocol === 'anthropic') return getModelVendor('anthropic');
  if (normalized.includes('api.deepseek.com')) return getModelVendor('deepseek');
  if (normalized.includes('aliyuncs.com') && normalized.includes('compatible-mode')) return getModelVendor('qwen');
  if (!normalized || normalized.includes('api.openai.com')) return getModelVendor('openai');
  return getModelVendor('custom-openai');
}
