import { describe, expect, it } from 'vitest';
import { resolveProposalAnchorScopes } from './ProposalAnchorScopes';

describe('方案锚点作用范围', () => {
  it('只修改作者明确选择的锚点，并保留其余提示与方案字段', () => {
    const original = { title: '潮汐关', anchors: [
      { strength: 'hard', statement: '钟楼三响', scope_hint: '结局' },
      { strength: 'soft', statement: '海雾再现', scope_hint: '第九章' },
    ] };
    const result = JSON.parse(resolveProposalAnchorScopes(JSON.stringify(original), [0]));
    expect(result.title).toBe('潮汐关');
    expect(result.anchors[0].scope_hint).toBe('proposal');
    expect(result.anchors[1]).toEqual(original.anchors[1]);
  });

  it('拒绝空选择和越界选择', () => {
    const payload = JSON.stringify({ anchors: [{ strength: 'hard', statement: '钟楼三响' }] });
    expect(() => resolveProposalAnchorScopes(payload, [])).toThrow();
    expect(() => resolveProposalAnchorScopes(payload, [1])).toThrow();
  });
});
