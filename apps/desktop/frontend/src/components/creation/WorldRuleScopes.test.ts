import { describe, expect, it } from 'vitest';
import { resolveWorldRuleScopes } from './WorldRuleScopes';

describe('世界规则作用范围', () => {
  it('只修改作者选择的规则，其他候选内容和范围保持不变', () => {
    const cast = { characters: [{ id: 'c1' }], world_rules: [
      { name: '月蚀', description: '每次月蚀投票', scope_type: 'region' },
      { name: '灯塔', description: '透镜留痕', scope_type: 'location' },
    ] };
    const resolved = JSON.parse(resolveWorldRuleScopes(JSON.stringify(cast), [0]));
    expect(resolved.characters).toEqual(cast.characters);
    expect(resolved.world_rules[0].scope_type).toBe('project');
    expect(resolved.world_rules[1]).toEqual(cast.world_rules[1]);
  });

  it('拒绝未选择和越界索引', () => {
    const payload = JSON.stringify({ world_rules: [{ name: '月蚀', scope_type: 'region' }] });
    expect(() => resolveWorldRuleScopes(payload, [])).toThrow();
    expect(() => resolveWorldRuleScopes(payload, [-1])).toThrow();
  });
});
