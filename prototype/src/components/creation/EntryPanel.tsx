import { useEffect, useState } from 'react';
import { Button } from '../ui/Button';
import { Textarea } from '../ui/Input';

type Kind = 'idea' | 'author_plan';
export interface EntryPanelProps {
  entry?: { id: string; kind: Kind; material: string };
  contentVersion: number;
  runSlotVersion: number;
  disabled?: boolean;
  onDirtyChange?: (dirty: boolean) => void;
  onSave: (kind: Kind, material: string, contentVersion: number, runSlotVersion: number) => Promise<void>;
}

export function EntryPanel({ entry, contentVersion, runSlotVersion, disabled, onSave, onDirtyChange }: EntryPanelProps) {
  const [kind, setKind] = useState<Kind>(entry?.kind ?? 'idea');
  const [material, setMaterial] = useState(entry?.material ?? '');
  const [dirty, setDirty] = useState(false);
  const [base, setBase] = useState({ contentVersion, runSlotVersion });
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);
  useEffect(() => { onDirtyChange?.(dirty); }, [dirty, onDirtyChange]);
  useEffect(() => {
    if (dirty) return;
    setKind(entry?.kind ?? 'idea');
    setMaterial(entry?.material ?? '');
    setBase({ contentVersion, runSlotVersion });
  }, [entry?.id, entry?.kind, entry?.material, contentVersion, runSlotVersion, dirty]);
  const save = async () => {
    setSaving(true); setError('');
    try { await onSave(kind, material, base.contentVersion, base.runSlotVersion); setDirty(false); }
    catch (err) { setError(err instanceof Error ? err.message : '材料保存失败，请保留输入后重试。'); }
    finally { setSaving(false); }
  };
  return (
    <details className="mb-6 rounded-xl border border-border bg-surface p-4" open={!entry?.id || dirty || undefined}>
      <summary className="cursor-pointer text-sm font-medium text-text">创作起点 · {entry?.kind === 'author_plan' ? '我已有方案' : '帮我构思'}</summary>
      <div className="mt-4 space-y-4">
        <div className="flex gap-2" role="group" aria-label="创作起点">
          <Button aria-pressed={kind === 'idea'} disabled={disabled || saving} onClick={() => { setKind('idea'); setDirty(true); }}>帮我构思</Button>
          <Button aria-pressed={kind === 'author_plan'} disabled={disabled || saving} onClick={() => { setKind('author_plan'); setDirty(true); }}>我已有方案</Button>
        </div>
        {kind === 'author_plan' && <div className="space-y-2">
          <label htmlFor="author-material" className="text-sm text-text">已有大纲、人物与设定</label>
          <Textarea id="author-material" value={material} disabled={disabled || saving} onChange={(event) => { setMaterial(event.target.value); setDirty(true); }} className="min-h-48" placeholder="粘贴你的方案。已有内容会保留来源，只检查缺口，不要求重新选择三套方向。" />
          <p className="text-xs text-text-muted">{Array.from(material).length.toLocaleString()} / 100,000 字</p>
        </div>}
        <p className="text-xs leading-relaxed text-text-sub">保存不调用模型。修改材料或切换起点会保留旧候选，回到起点检查；后续候选需要重新核对来源，继续生成时重新确认费用。</p>
        {error && <p role="alert" className="text-sm text-danger">{error} 当前输入已保留。</p>}
        <Button variant="primary" disabled={!dirty || disabled || saving || (kind === 'author_plan' && !material.trim()) || Array.from(material).length > 100000} onClick={() => void save()}>{saving ? '保存中…' : '保存材料与起点'}</Button>
      </div>
    </details>
  );
}
