import { useState, useMemo } from 'react';
import { clsx } from 'clsx';
import { Search, Globe, BookMarked, Flame, Sparkles, Settings2 } from 'lucide-react';
import { worldEntries } from '../../mock/data';
import type { WorldCategory } from '../../types';
import { Card } from '../../components/ui/Card';
import { Input } from '../../components/ui/Input';
import { EmptyState } from '../../components/ui/EmptyState';
import { StatusBadge } from '../../components/ui/StatusBadge';

const tabs: { key: 'all' | 'geography' | 'history' | 'culture' | 'system'; label: string; icon: typeof Globe }[] = [
  { key: 'all', label: '全部', icon: BookMarked },
  { key: 'geography', label: '地理', icon: Globe },
  { key: 'history', label: '历史', icon: BookMarked },
  { key: 'culture', label: '文化', icon: Flame },
  { key: 'system', label: '设定', icon: Settings2 },
];

const categoryLabels: Record<WorldCategory, string> = {
  geography: '地理',
  history: '历史',
  culture: '文化',
  magic: '术法',
  technology: '技术',
  politics: '政治',
};

const importanceVariant: Record<'core' | 'secondary' | 'detail', 'warning' | 'info' | 'default'> = {
  core: 'warning',
  secondary: 'info',
  detail: 'default',
};

export default function World() {
  const [activeTab, setActiveTab] = useState<'all' | 'geography' | 'history' | 'culture' | 'system'>('all');
  const [query, setQuery] = useState('');

  const filtered = useMemo(() => {
    return worldEntries.filter((entry) => {
      const matchesTab =
        activeTab === 'all' ||
        entry.category === activeTab ||
        (activeTab === 'system' && ['magic', 'technology', 'politics'].includes(entry.category));
      const q = query.trim().toLowerCase();
      const matchesSearch =
        !q ||
        entry.title.toLowerCase().includes(q) ||
        entry.content.toLowerCase().includes(q);
      return matchesTab && matchesSearch;
    });
  }, [activeTab, query]);

  return (
    <div className="space-y-6 animate-fade-up">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h2 className="text-lg font-display text-text">世界设定</h2>
          <p className="text-sm text-text-sub mt-0.5">维护地理、历史、文化与规则设定，保证叙事一致性。</p>
        </div>
        <div className="relative w-full sm:w-64">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-muted" />
          <Input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="搜索设定标题或内容..."
            className="pl-9"
          />
        </div>
      </div>

      <div className="flex flex-wrap gap-2">
        {tabs.map((tab) => {
          const Icon = tab.icon;
          return (
            <button
              key={tab.key}
              onClick={() => setActiveTab(tab.key)}
              className={clsx(
                'flex items-center gap-2 px-3 py-1.5 text-sm rounded-md border transition-colors',
                activeTab === tab.key
                  ? 'bg-primary-subtle border-primary/20 text-primary'
                  : 'bg-surface border-border text-text-sub hover:text-text hover:border-border-strong',
              )}
            >
              <Icon className="w-4 h-4" />
              {tab.label}
            </button>
          );
        })}
      </div>

      {filtered.length === 0 ? (
        <EmptyState
          icon={Sparkles}
          title="没有找到相关设定"
          description="切换分类或清除搜索词后再试。"
          action={{ label: '清除搜索', onClick: () => setQuery('') }}
        />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {filtered.map((entry, idx) => (
            <Card key={entry.id} style={{ animationDelay: `${idx * 60}ms` }}>
              <div className="flex items-start justify-between gap-3 mb-2">
                <h3 className="text-base font-medium text-text">{entry.title}</h3>
                <div className="flex items-center gap-2 shrink-0">
                  <StatusBadge variant={importanceVariant[entry.importance]}>
                    {entry.importance === 'core' ? '核心' : entry.importance === 'secondary' ? '次要' : '细节'}
                  </StatusBadge>
                </div>
              </div>
              <p className="text-sm text-text-sub leading-relaxed mb-3">{entry.content}</p>
              <span className="inline-flex items-center text-xs px-2 py-0.5 rounded bg-surface-hover text-text-sub border border-border">
                {categoryLabels[entry.category]}
              </span>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
