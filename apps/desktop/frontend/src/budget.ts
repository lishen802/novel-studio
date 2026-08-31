export const BUDGET_WAN_UNIT = 10_000;
export const BUDGET_PRESETS_WAN = [1000, 5000, 10000] as const;
export const DEFAULT_CUSTOM_BUDGET_WAN = 10_000;
export const MIN_CUSTOM_BUDGET_WAN = 1;

export type BudgetChoice = '1000' | '5000' | '10000' | 'unlimited' | 'custom';

export const reasoningEffortOptions = [
  ['auto', '自动'],
  ['none', '关闭'],
  ['low', '低'],
  ['medium', '中'],
  ['high', '高'],
  ['max', '最大'],
] as const;

export function wanToBase(wan: number): number {
  return wan * BUDGET_WAN_UNIT;
}

export function baseToWan(base: number): number {
  if (base < 1) return DEFAULT_CUSTOM_BUDGET_WAN;
  return Math.max(MIN_CUSTOM_BUDGET_WAN, Math.round(base / BUDGET_WAN_UNIT));
}

export function formatBudget(base: number, unlimited: boolean): string {
  if (unlimited) return '不限制';
  return `${baseToWan(base).toLocaleString()} 万`;
}

export function deriveBudgetChoice(unlimited: boolean, wan: number): BudgetChoice {
  if (unlimited) return 'unlimited';
  if (wan === 1000 || wan === 5000 || wan === 10000) return String(wan) as BudgetChoice;
  return 'custom';
}

export function reasoningLabel(effort: string): string {
  const found = reasoningEffortOptions.find(([value]) => value === effort);
  return found ? found[1] : '自动';
}
