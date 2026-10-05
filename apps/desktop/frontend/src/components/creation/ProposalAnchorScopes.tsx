import { useEffect, useState } from 'react';
import { studio } from '../../api/studio';
import { Button } from '../ui/Button';
import { Card } from '../ui/Card';

type Anchor = { strength: string; statement: string; scope_hint?: string };
type ProposalPayload = Record<string, unknown> & { anchors: Anchor[] };

function parseProposal(payloadJson: string): ProposalPayload {
  const value: unknown = JSON.parse(payloadJson);
  if (!value || typeof value !== 'object' || !('anchors' in value) || !Array.isArray(value.anchors) ||
      value.anchors.some((anchor: unknown) => !anchor || typeof anchor !== 'object' || !('statement' in anchor) || typeof anchor.statement !== 'string')) {
    throw new Error('方案锚点格式无效，请切换到完整候选版本。');
  }
  return value as ProposalPayload;
}

// 只有作者明确勾选的锚点才会改为作品方案级，其余提示和候选字段原样保留。
export function resolveProposalAnchorScopes(payloadJson: string, selected: number[]): string {
  const payload = parseProposal(payloadJson);
  const indices = new Set(selected);
  if (!indices.size || [...indices].some(index => !Number.isInteger(index) || index < 0 || index >= payload.anchors.length)) {
    throw new Error('请先选择需要指定作用范围的锚点。');
  }
  return JSON.stringify({ ...payload, anchors: payload.anchors.map((anchor, index) =>
    indices.has(index) ? { ...anchor, scope_hint: 'proposal' } : anchor) });
}

export function ProposalAnchorScopes({ candidateId, disabled, onSaved }: {
  candidateId: string; disabled: boolean; onSaved: () => Promise<void>;
}) {
  const [payloadJson, setPayloadJson] = useState('');
  const [anchors, setAnchors] = useState<Anchor[]>([]);
  const [selected, setSelected] = useState<number[]>([]);
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    let active = true;
    setPayloadJson(''); setAnchors([]); setSelected([]); setError('');
    if (!candidateId) return;
    void studio.getCreationCandidate(candidateId).then(candidate => {
      if (!active) return;
      if (candidate.id !== candidateId || candidate.stage !== 'proposal') throw new Error('方案版本已变化，请刷新后重试。');
      setAnchors(parseProposal(candidate.payloadJson).anchors);
      setPayloadJson(candidate.payloadJson);
    }).catch(cause => { if (active) setError(cause instanceof Error ? cause.message : '无法读取方案锚点。'); });
    return () => { active = false; };
  }, [candidateId]);

  if (!candidateId || (!anchors.length && !error)) return null;
  const unresolved = anchors.map((anchor, index) => ({ ...anchor, index })).filter(anchor => anchor.scope_hint !== 'proposal');
  return <Card className="max-w-4xl mx-auto space-y-3">
    <div>
      <h2 className="text-sm font-medium text-text">方案锚点作用范围</h2>
      <p className="text-xs text-text-sub mt-1">模型给出的范围提示不会自动转成正式引用。请逐条核对；只有勾选并保存的锚点会设为作品方案级。其他锚点仍会阻止入档。</p>
    </div>
    {unresolved.length === 0 && !error && <p className="text-sm text-text-sub">所有方案锚点都已明确设为作品方案级。</p>}
    {unresolved.map(anchor => <label key={anchor.index} className="flex gap-3 p-3 rounded-lg border border-border text-sm cursor-pointer">
      <input type="checkbox" aria-label={`设为方案级：${anchor.statement}`} checked={selected.includes(anchor.index)} disabled={disabled || saving}
        onChange={event => setSelected(current => event.target.checked ? [...current, anchor.index] : current.filter(index => index !== anchor.index))} />
      <span><span className="text-text">{anchor.statement}</span><span className="block text-xs text-text-muted mt-1">原提示：{anchor.scope_hint || '未指定'} · 强度：{anchor.strength}</span></span>
    </label>)}
    {error && <p role="alert" className="text-sm text-danger">{error}</p>}
    {unresolved.length > 0 && <Button size="sm" disabled={disabled || saving || selected.length === 0} onClick={async () => {
      setSaving(true); setError('');
      try {
        await studio.saveCreationCandidateDraft(candidateId, resolveProposalAnchorScopes(payloadJson, selected));
        await onSaved();
      } catch (cause) { setError(cause instanceof Error ? cause.message : '锚点保存失败。'); }
      finally { setSaving(false); }
    }}>确认将已选 {selected.length} 条锚点设为作品方案级</Button>}
  </Card>;
}
