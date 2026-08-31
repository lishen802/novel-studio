export interface PrototypeVendor {
  id: string;
  label: string;
  protocol: string;
  defaultBaseUrl: string;
  models: { id: string; label: string }[];
}

/** 原型与生产界面共享的厂商目录视觉契约。 */
export const modelVendors: PrototypeVendor[] = [
  { id: 'openai', label: 'OpenAI', protocol: 'openai', defaultBaseUrl: 'https://api.openai.com/v1', models: [{ id: 'gpt-4.1', label: 'GPT-4.1' }, { id: 'gpt-4.1-mini', label: 'GPT-4.1 mini' }] },
  { id: 'anthropic', label: 'Anthropic', protocol: 'anthropic', defaultBaseUrl: 'https://api.anthropic.com', models: [{ id: 'claude-sonnet-4-6', label: 'Claude Sonnet 4.6' }, { id: 'claude-opus-4-6', label: 'Claude Opus 4.6' }] },
  { id: 'deepseek', label: 'DeepSeek', protocol: 'openai', defaultBaseUrl: 'https://api.deepseek.com', models: [{ id: 'deepseek-v4-flash', label: 'DeepSeek V4 Flash' }, { id: 'deepseek-v4-pro', label: 'DeepSeek V4 Pro' }] },
  { id: 'qwen', label: '阿里云百炼（千问）', protocol: 'openai', defaultBaseUrl: 'https://dashscope.aliyuncs.com/compatible-mode/v1', models: [{ id: 'qwen3.8-max', label: 'Qwen3.8 Max' }, { id: 'qwen3.7-plus', label: 'Qwen3.7 Plus' }] },
  { id: 'custom-openai', label: '自定义 OpenAI 兼容', protocol: 'openai', defaultBaseUrl: '', models: [] },
];

export function inferVendor(provider: string, baseUrl?: string) {
  const normalized = (baseUrl || '').toLowerCase();
  if (provider === 'anthropic') return modelVendors[1];
  if (normalized.includes('deepseek.com')) return modelVendors[2];
  if (normalized.includes('aliyuncs.com')) return modelVendors[3];
  if (!normalized || normalized.includes('openai.com')) return modelVendors[0];
  return modelVendors[4];
}
