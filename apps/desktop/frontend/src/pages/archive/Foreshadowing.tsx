import { useEffect, useState } from 'react';
import { clsx } from 'clsx';
import { Link2, Users, ArrowRight, Sparkles } from 'lucide-react';
import type { Foreshadowing, ForeshadowStatus } from '../../types';
import { studio } from '../../api/studio';
import { Card } from '../../components/ui/Card';
import { StatusBadge } from '../../components/ui/StatusBadge';
import { EmptyState } from '../../components/ui/EmptyState';

const statusOptions: { value: ForeshadowStatus; label: string }[] = [
  { value: 'planted', label: '已埋下' },
  { value: 'growing', label: '生长中' },
  { value: 'resolved', label: '已回收' },
  { value: 'dropped', label: '已弃用' },
];

const statusVariant: Record<ForeshadowStatus, 'default' | 'info' | 'success' | 'danger'> = {
  planted: 'default',
  growing: 'info',
  resolved: 'success',
  dropped: 'danger',
};

const statusLabel: Record<ForeshadowStatus, string> = {
  planted: '已埋下',
  growing: '生长中',
  resolved: '已回收',
  dropped: '已弃用',
};

export default function Foreshadowing() {
  const [items, setItems] = useState<Foreshadowing[]>([]);
  const [relationNodes, setRelationNodes] = useState<Array<{ id: string; label: string; type: 'character' | 'faction' }>>([]);
  const [relations, setRelations] = useState<string[]>([]);

  useEffect(() => {
    let active = true;
    void Promise.all([studio.worldAssets(), studio.storyWorld()]).then(([assets, world]) => {
      if (!active) return;
      setItems(assets.hooks.map((item) => ({
        id: String(item.id || ''), projectId: '', title: String(item.title || ''),
        description: String(item.setup || ''),
        status: (['planted', 'growing', 'resolved', 'dropped'].includes(String(item.status)) ? item.status : 'planted') as ForeshadowStatus,
        linkedChapters: [String(item.plantedChapterId || '')].filter(Boolean),
        payoffChapterId: String(item.resolvedChapterId || '') || undefined,
      })));
      const names = new Map<string, string>();
      const nodes: Array<{ id: string; label: string; type: 'character' | 'faction' }> = [];
      world.characters.forEach((item) => {
        names.set(String(item.id || ''), String(item.name || ''));
        nodes.push({ id: String(item.id || ''), label: String(item.name || ''), type: 'character' });
      });
      world.factions.forEach((item) => {
        names.set(String(item.id || ''), String(item.name || ''));
        nodes.push({ id: String(item.id || ''), label: String(item.name || ''), type: 'faction' });
      });
      setRelationNodes(nodes);
      setRelations(world.relationships.map((item) => `${names.get(String(item.sourceCharacterId || '')) || '未知人物'} → ${String(item.label || item.kind || '关系')} → ${names.get(String(item.targetCharacterId || '')) || '未知人物'}`));
    });
    return () => { active = false; };
  }, []);

  const updateStatus = async (id: string, status: ForeshadowStatus) => {
    const item = items.find((candidate) => candidate.id === id);
    if (!item) return;
    await studio.updateForeshadowingHook(id, {
      title: item.title,
      setup: item.description,
      plantedChapterId: item.linkedChapters[0] || '',
      dueChapterId: '',
      resolvedChapterId: item.payoffChapterId || '',
      status,
      notes: '',
    });
    setItems((prev) => prev.map((item) => (item.id === id ? { ...item, status } : item)));
  };

  return (
    <div className="space-y-8 animate-fade-up">
      <div>
        <h2 className="text-lg font-display text-text">伏笔与关系</h2>
        <p className="text-sm text-text-sub mt-0.5">跟踪伏笔的生命周期，梳理人物与势力之间的潜在联系。</p>
      </div>

      <div className="space-y-4">
        {items.length === 0 ? (
          <EmptyState icon={Sparkles} title="还没有伏笔" description="在正文或设定中埋下线索，便于后续回收。" />
        ) : (
          items.map((item, idx) => (
            <Card key={item.id} style={{ animationDelay: `${idx * 60}ms` }}>
              <div className="flex flex-col sm:flex-row sm:items-start justify-between gap-4">
                <div className="flex-1">
                  <div className="flex flex-wrap items-center gap-2 mb-2">
                    <h3 className="text-base font-medium text-text">{item.title}</h3>
                    <StatusBadge variant={statusVariant[item.status]}>{statusLabel[item.status]}</StatusBadge>
                  </div>
                  <p className="text-sm text-text-sub leading-relaxed mb-3">{item.description}</p>
                  <div className="flex flex-wrap items-center gap-2">
                    <Link2 className="w-3.5 h-3.5 text-text-muted" />
                    {item.linkedChapters.map((ch) => (
                      <span
                        key={ch}
                        className="text-xs px-2 py-0.5 rounded bg-surface-hover text-text-sub border border-border"
                      >
                        {ch}
                      </span>
                    ))}
                  </div>
                </div>

                <div className="shrink-0">
                  <label className="block text-xs text-text-muted mb-1.5">状态</label>
                  <select
                    value={item.status}
                    onChange={(e) => void updateStatus(item.id, e.target.value as ForeshadowStatus)}
                    className={clsx(
                      'h-9 px-3 pr-8 bg-surface border border-border rounded-md text-sm text-text',
                      'focus:outline-none focus:ring-2 focus:ring-primary/35',
                    )}
                  >
                    {statusOptions.map((opt) => (
                      <option key={opt.value} value={opt.value}>
                        {opt.label}
                      </option>
                    ))}
                  </select>
                </div>
              </div>
            </Card>
          ))
        )}
      </div>

      <Card className="space-y-4">
        <h3 className="text-base font-medium text-text flex items-center gap-2">
          <Users className="w-4 h-4 text-primary" />
          人物与势力关系图
        </h3>
        <p className="text-xs text-text-sub">当前项目中所有角色与势力的静态关系概览。</p>

        <div className="flex flex-wrap items-center gap-2 py-4">
          {relationNodes.map((node, idx) => (
            <div key={node.id} className="flex items-center">
              <div
                className={clsx(
                  'flex items-center gap-2 px-3 py-2 rounded-md border text-sm',
                  node.type === 'faction'
                    ? 'bg-primary-subtle border-primary/20 text-primary'
                    : 'bg-surface border-border text-text',
                )}
              >
                <span
                  className={clsx(
                    'w-2 h-2 rounded-full',
                    node.type === 'faction' ? 'bg-primary' : 'bg-info',
                  )}
                />
                {node.label}
              </div>
              {idx < relationNodes.length - 1 && (
                <div className="w-6 h-px border-t border-dashed border-border-strong mx-1" />
              )}
            </div>
          ))}
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-sm">
          {relations.map((relation) => (
            <div key={relation} className="p-3 bg-surface rounded-md border border-border flex items-center gap-3">
              <span className="w-2 h-2 rounded-full bg-info" />
              <ArrowRight className="w-3.5 h-3.5 text-text-muted" />
              <span className="text-text-sub">{relation}</span>
            </div>
          ))}
        </div>
      </Card>
    </div>
  );
}
