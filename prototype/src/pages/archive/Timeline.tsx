import { useMemo, useState } from 'react';
import { timeline, characters } from '../../mock/data';
import { Card } from '../../components/ui/Card';
import { Input } from '../../components/ui/Input';
import { EmptyState } from '../../components/ui/EmptyState';
import { Clock, User, Calendar } from 'lucide-react';

interface Fact {
  id: string;
  content: string;
  source: string;
}

const initialFacts: Fact[] = [
  { id: 'f1', content: '江心灯塔由临江商会接管维护后，守塔人失踪传闻频发。', source: '民国二十三年事件' },
  { id: 'f2', content: '周砚白在民国二十六年七月后失去踪迹。', source: '日志签名记录' },
  { id: 'f3', content: '阿沅父亲的失踪与灯塔区域有直接关联。', source: '民国二十四年事件' },
];

export default function TimelinePage() {
  const [facts, setFacts] = useState<Fact[]>(initialFacts);

  const characterMap = useMemo(() => {
    const map = new Map<string, string>();
    characters.forEach((c) => map.set(c.id, c.name));
    return map;
  }, []);

  const updateFact = (id: string, patch: Partial<Fact>) => {
    setFacts((prev) => prev.map((f) => (f.id === id ? { ...f, ...patch } : f)));
  };

  return (
    <div className="space-y-8 animate-fade-up">
      <div>
        <h2 className="text-lg font-display text-text">时间线与事实</h2>
        <p className="text-sm text-text-sub mt-0.5">梳理故事内外的时间节点，并记录由事件推导出的关键事实。</p>
      </div>

      <Card className="space-y-6">
        <h3 className="text-base font-medium text-text flex items-center gap-2">
          <Calendar className="w-4 h-4 text-primary" />
          事件时间线
        </h3>

        {timeline.length === 0 ? (
          <EmptyState icon={Clock} title="还没有事件" description="添加故事中的关键节点，构建时间脉络。" />
        ) : (
          <div className="relative pl-4">
            <div className="absolute left-[11px] top-2 bottom-2 w-px bg-border" />
            <div className="space-y-6">
              {timeline.map((event) => {
                const isMajor = event.importance === 'major';
                return (
                  <div key={event.id} className="relative flex gap-4">
                    <div
                      className={[
                        'absolute left-0 top-1 rounded-full border-2 shrink-0 z-10',
                        isMajor
                          ? 'w-5 h-5 -translate-x-[2px] bg-bg border-primary'
                          : 'w-3 h-3 translate-x-[1px] bg-text-muted border-bg',
                      ].join(' ')}
                    />
                    <div className="pl-7 flex-1">
                      <div className="flex flex-wrap items-center gap-2 mb-1">
                        <span
                          className={[
                            'font-display',
                            isMajor ? 'text-base text-text' : 'text-sm text-text-sub',
                          ].join(' ')}
                        >
                          {event.date}
                        </span>
                        {isMajor && (
                          <span className="text-xs px-1.5 py-0.5 rounded bg-primary-subtle text-primary border border-primary/10">
                            重大
                          </span>
                        )}
                      </div>
                      <p
                        className={[
                          'leading-relaxed',
                          isMajor ? 'text-sm text-text' : 'text-sm text-text-sub',
                        ].join(' ')}
                      >
                        {event.event}
                      </p>
                      {event.linkedCharacters.length > 0 && (
                        <div className="flex flex-wrap items-center gap-2 mt-2">
                          <User className="w-3.5 h-3.5 text-text-muted" />
                          {event.linkedCharacters.map((cid) => (
                            <span
                              key={cid}
                              className="text-xs px-2 py-0.5 rounded bg-surface-hover text-text-sub border border-border"
                            >
                              {characterMap.get(cid) ?? cid}
                            </span>
                          ))}
                        </div>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        )}
      </Card>

      <Card className="space-y-4">
        <h3 className="text-base font-medium text-text">事实表</h3>
        <p className="text-xs text-text-sub">从时间线事件中提炼的确定性信息，可作为后续创作的约束条件。</p>

        {facts.length === 0 ? (
          <p className="text-sm text-text-muted py-4">暂无记录的事实。</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border">
                  <th className="text-left py-2 pr-4 text-xs text-text-muted font-medium w-1/2">事实</th>
                  <th className="text-left py-2 pr-4 text-xs text-text-muted font-medium">来源事件</th>
                </tr>
              </thead>
              <tbody>
                {facts.map((fact) => (
                  <tr key={fact.id} className="border-b border-border/50 last:border-0">
                    <td className="py-2 pr-4 align-top">
                      <Input
                        value={fact.content}
                        onChange={(e) => updateFact(fact.id, { content: e.target.value })}
                        className="h-8 text-sm"
                      />
                    </td>
                    <td className="py-2 pr-4 align-top">
                      <Input
                        value={fact.source}
                        onChange={(e) => updateFact(fact.id, { source: e.target.value })}
                        className="h-8 text-sm"
                      />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </div>
  );
}
