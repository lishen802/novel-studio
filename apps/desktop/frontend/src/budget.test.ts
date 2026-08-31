import { describe, expect, it } from 'vitest';
import {
  DEFAULT_CUSTOM_BUDGET_WAN,
  MIN_CUSTOM_BUDGET_WAN,
  baseToWan,
  deriveBudgetChoice,
  formatBudget,
  wanToBase,
} from './budget';

describe('方案预算换算', () => {
  it('把界面“万”换算为基础单位，自定义最小为 1 万', () => {
    expect(wanToBase(1000)).toBe(10_000_000);
    expect(wanToBase(DEFAULT_CUSTOM_BUDGET_WAN)).toBe(100_000_000);
    expect(baseToWan(10_000_000)).toBe(1000);
    expect(MIN_CUSTOM_BUDGET_WAN).toBe(1);
  });

  it('预置值、不限制和自定义互斥', () => {
    expect(deriveBudgetChoice(true, 1000)).toBe('unlimited');
    expect(deriveBudgetChoice(false, 5000)).toBe('5000');
    expect(deriveBudgetChoice(false, 2000)).toBe('custom');
    expect(formatBudget(0, true)).toBe('不限制');
    expect(formatBudget(50_000_000, false).replace(/[^\d]/g, '')).toBe('5000');
  });
});
