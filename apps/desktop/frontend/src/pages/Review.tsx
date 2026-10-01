import { useEffect, useMemo, useState } from 'react';
import { Check, FileDiff, GitPullRequest, Loader2, Trash2 } from 'lucide-react';
import { studio, type ChangeOperationView, type ChangeSetView } from '../api/studio';
import { Button } from '../components/ui/Button';
import { Card } from '../components/ui/Card';
import { EmptyState } from '../components/ui/EmptyState';
import { StatusBadge } from '../components/ui/StatusBadge';

function reviewLabel(value: string) {
 return ({pending:'待审阅',applied:'已采用',rejected:'已拒绝',conflicted:'基准已变化',partially_applied:'部分采用',chapter_body:'章节正文',replace_chapter_body:'替换章节正文',update_outline_node:'更新大纲',update_character:'更新人物',update_fact:'更新事实',create_outline_node:'新增大纲',create_character:'新增人物',create_relationship:'新增关系',create_world_rule:'新增世界规则',create_story_event:'新增事件',create_fact:'新增事实',create_foreshadowing_hook:'新增伏笔',create_faction:'新增势力',create_location:'新增地点',create_anchor:'新增锚点',create_story_asset:'新增故事资产',update_project:'更新作品信息'} as Record<string,string>)[value] ?? value;
}

/** 审阅页只呈现后端持久化 ChangeSet，避免把原型候选误报为真实结果。 */
export default function Review() {
  const [changeSets, setChangeSets] = useState<ChangeSetView[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [selectedOperationId, setSelectedOperationId] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refresh = async () => {
    try {
      const items = await studio.changeSets();
      setChangeSets(items);
      setSelectedId((current) => current && items.some((item) => item.id === current) ? current : items[0]?.id || null);
      setError(null);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : '无法读取候选');
    }
  };

  useEffect(() => { void refresh(); }, []);

  const selected = useMemo(
    () => changeSets.find((item) => item.id === selectedId) || null,
    [changeSets, selectedId],
  );
  const operation = useMemo<ChangeOperationView | null>(
    () => selected?.operations.find((item) => item.id === selectedOperationId) || selected?.operations[0] || null,
    [selected, selectedOperationId],
  );

  const decideSet = async (decision: 'apply' | 'reject') => {
    if (!selected) return;
    setBusy(true);
    try {
      if (decision === 'apply') await studio.applyChangeSet(selected.id);
      else await studio.rejectChangeSet(selected.id);
      await refresh();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "操作失败，候选已保留，请重试。");
    } finally {
      setBusy(false);
    }
  };

  const decideOperation = async (decision: 'apply' | 'reject') => {
    if (!selected || !operation) return;
    setBusy(true);
    try {
      if (decision === 'apply') await studio.applyChangeOperation(selected.id, operation.id);
      else await studio.rejectChangeOperation(selected.id, operation.id);
      await refresh();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "操作失败，候选已保留，请重试。");
    } finally {
      setBusy(false);
    }
  };

  if (changeSets.length === 0 && !error) {
    return (
      <Card className="m-auto max-w-xl">
        <EmptyState
          icon={GitPullRequest}
          title="没有待审阅候选"
          description="AI 或作者提出的正文与结构化修改会作为 ChangeSet 出现在这里。"
        />
      </Card>
    );
  }

  return (
    <div className="h-[calc(100%+3rem)] flex animate-fade-up -m-6">
      <aside className="w-[280px] shrink-0 border-r border-border bg-surface p-4 overflow-auto">
        <h1 className="text-base font-medium text-text mb-1">审阅与候选</h1>
        <p className="text-xs text-text-sub mb-4">所有内容均来自项目中的持久化 ChangeSet。</p>
        {error && <p className="text-sm text-danger mb-3">{error}</p>}
        <div className="space-y-2">
          {changeSets.map((item) => (
            <button
              key={item.id}
              onClick={() => { setSelectedId(item.id); setSelectedOperationId(null); }}
              className={`w-full text-left rounded-lg border p-3 ${selectedId === item.id ? 'border-primary/30 bg-primary-subtle' : 'border-border bg-surface-elevated'}`}
            >
              <div className="flex items-center justify-between gap-2">
                <span className="text-sm font-medium text-text truncate">{item.title}</span>
                <StatusBadge variant={item.status === 'pending' ? 'warning' : 'default'}>{reviewLabel(item.status)}</StatusBadge>
              </div>
              <p className="text-xs text-text-sub mt-2 line-clamp-2">{item.summary}</p>
              <p className="text-xs text-text-muted mt-2">{item.operations.length} 项修改</p>
            </button>
          ))}
        </div>
      </aside>

      <main className="flex-1 min-w-0 bg-atmosphere p-6 overflow-auto">
        {selected && (
          <div className="max-w-4xl mx-auto space-y-4">
            <div className="flex items-start justify-between gap-4">
              <div>
                <h2 className="text-xl font-display text-text">{selected.title}</h2>
                <p className="text-sm text-text-sub mt-1 whitespace-pre-wrap">{selected.summary}</p>
              </div>
              <div className="flex gap-2">
                <Button variant="default" disabled={busy || !selected.operations.some(item => item.status === 'pending')} onClick={() => void decideSet('reject')}>
                  <Trash2 className="w-4 h-4" />拒绝整组
                </Button>
                <Button variant="primary" disabled={busy || !selected.operations.some(item => item.status === 'pending')} onClick={() => void decideSet('apply')}>
                  {busy ? <Loader2 className="w-4 h-4 animate-spin" /> : <Check className="w-4 h-4" />}{selected.summary.startsWith('审阅未通过') ? '已核对问题，仍采用整组' : '应用整组'}
                </Button>
              </div>
            </div>

            <div className="grid grid-cols-[220px_1fr] gap-4">
              <Card className="p-3 h-fit">
                <h3 className="text-xs text-text-muted mb-2">修改项目</h3>
                <div className="space-y-1">
                  {selected.operations.map((item) => (
                    <button
                      key={item.id}
                      onClick={() => setSelectedOperationId(item.id)}
                      className={`w-full text-left rounded-md p-2 text-sm ${operation?.id === item.id ? 'bg-primary-subtle text-primary' : 'hover:bg-surface-hover text-text-sub'}`}
                    >
                      {reviewLabel(item.kind)} · {reviewLabel(item.status)}
                    </button>
                  ))}
                </div>
              </Card>

              {operation && (
                <Card>
                  <div className="flex items-center justify-between mb-4">
                    <h3 className="font-medium text-text flex items-center gap-2"><FileDiff className="w-4 h-4" />修改差异</h3>
                    <div className="flex gap-2">
                      <Button size="sm" disabled={busy || operation.status !== 'pending'} onClick={() => void decideOperation('reject')}>拒绝此项</Button>
                      <Button size="sm" variant="primary" disabled={busy || operation.status !== 'pending'} onClick={() => void decideOperation('apply')}>应用此项</Button>
                    </div>
                  </div>
                  <p className="text-sm text-text-sub mb-4">{operation.reason}</p>
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <p className="text-xs text-text-muted mb-2">修改前</p>
                      <pre className="whitespace-pre-wrap text-sm bg-danger/5 border border-danger/15 rounded-lg p-4 min-h-48">{operation.beforeValue || '（新增内容）'}</pre>
                    </div>
                    <div>
                      <p className="text-xs text-text-muted mb-2">候选版本</p>
                      <pre className="whitespace-pre-wrap text-sm bg-success/5 border border-success/15 rounded-lg p-4 min-h-48">{operation.afterValue}</pre>
                    </div>
                  </div>
                </Card>
              )}
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
