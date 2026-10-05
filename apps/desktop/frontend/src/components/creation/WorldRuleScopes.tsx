import { useEffect, useState } from 'react';
import { studio } from '../../api/studio';
import { Button } from '../ui/Button';
import { Card } from '../ui/Card';

type Rule = { name: string; description: string; scope_type: string };
type CastPayload = Record<string, unknown> & { world_rules: Rule[] };

function parseCast(payloadJson: string): CastPayload {
  const value: unknown = JSON.parse(payloadJson);
  if (!value || typeof value !== 'object' || !('world_rules' in value) || !Array.isArray(value.world_rules) ||
      value.world_rules.some((rule: unknown) => !rule || typeof rule !== 'object' || !('name' in rule) || typeof rule.name !== 'string' || !('scope_type' in rule) || typeof rule.scope_type !== 'string')) {
    throw new Error('世界规则格式无效，请切换到完整候选版本。');
  }
  return value as CastPayload;
}

// 当前候选没有区域或地点的稳定目标，只有作者逐条确认后才能扩展为作品级规则。
export function resolveWorldRuleScopes(payloadJson: string, selected: number[]): string {
  const payload = parseCast(payloadJson);
  const indices = new Set(selected);
  if (!indices.size || [...indices].some(index => !Number.isInteger(index) || index < 0 || index >= payload.world_rules.length)) {
    throw new Error('请先选择需要修改作用范围的世界规则。');
  }
  return JSON.stringify({ ...payload, world_rules: payload.world_rules.map((rule, index) =>
    indices.has(index) ? { ...rule, scope_type: 'project' } : rule) });
}

export function WorldRuleScopes({ candidateId, disabled, onSaved }: {
  candidateId: string; disabled: boolean; onSaved: () => Promise<void>;
}) {
  const [payloadJson, setPayloadJson] = useState('');
  const [rules, setRules] = useState<Rule[]>([]);
  const [selected, setSelected] = useState<number[]>([]);
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    let active = true;
    setPayloadJson(''); setRules([]); setSelected([]); setError('');
    if (!candidateId) return;
    void studio.getCreationCandidate(candidateId).then(candidate => {
      if (!active) return;
      if (candidate.id !== candidateId || candidate.stage !== 'people_world') throw new Error('人物世界版本已变化，请刷新后重试。');
      setRules(parseCast(candidate.payloadJson).world_rules);
      setPayloadJson(candidate.payloadJson);
    }).catch(cause => { if (active) setError(cause instanceof Error ? cause.message : '无法读取世界规则。'); });
    return () => { active = false; };
  }, [candidateId]);

  if (!candidateId || (!rules.length && !error)) return null;
  const unresolved = rules.map((rule, index) => ({ ...rule, index })).filter(rule => rule.scope_type !== 'project');
  return <Card className="max-w-4xl space-y-3">
    <div>
      <h2 className="text-sm font-medium text-text">世界规则作用范围</h2>
      <p className="text-xs text-text-sub mt-1">当前候选没有区域或地点的稳定引用。若确认某条规则适用于整部作品，可逐条勾选并设为作品级；范围仍未解析的规则会阻止正式入档。</p>
    </div>
    {unresolved.length === 0 && !error && <p className="text-sm text-text-sub">所有世界规则均为作品级，可进入入档核对。</p>}
    {unresolved.map(rule => <label key={rule.index} className="flex gap-3 p-3 rounded-lg border border-border text-sm cursor-pointer">
      <input type="checkbox" aria-label={`设为作品级规则：${rule.name}`} checked={selected.includes(rule.index)} disabled={disabled || saving}
        onChange={event => setSelected(current => event.target.checked ? [...current, rule.index] : current.filter(index => index !== rule.index))} />
      <span><span className="text-text">{rule.name}</span><span className="block text-xs text-text-muted mt-1">原范围：{rule.scope_type} · {rule.description}</span></span>
    </label>)}
    {error && <p role="alert" className="text-sm text-danger">{error}</p>}
    {unresolved.length > 0 && <Button size="sm" disabled={disabled || saving || selected.length === 0} onClick={async () => {
      setSaving(true); setError('');
      try {
        await studio.saveCreationCandidateDraft(candidateId, resolveWorldRuleScopes(payloadJson, selected));
        await onSaved();
      } catch (cause) { setError(cause instanceof Error ? cause.message : '世界规则保存失败。'); }
      finally { setSaving(false); }
    }}>确认将已选 {selected.length} 条规则设为作品级</Button>}
  </Card>;
}
