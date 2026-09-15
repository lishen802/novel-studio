import type { AssistantMessageView } from '../../api/studio';
import { DiscussionSuggestionCard } from './DiscussionSuggestionCard';

const statusLabels: Record<string, string> = {
  pending: '正在分析', running: '正在分析', answered: '分析完成',
  failed: '讨论未完成', outcome_unknown: '需要核对运行结果',
  pending_confirmation: '修改提案待确认', executing: '正在修改与审阅',
  succeeded: '修改结果已就绪', rejected: '提案已放弃', expired: '提案已过期',
  conflict: '目标内容已变化',
};

interface Props {
  message: AssistantMessageView;
  ignored: Set<string>;
  disabled?: boolean;
  onPreviewSuggestion?: (messageId: string, suggestionId: string) => Promise<void>;
  onIgnore: (id: string) => void;
}

export function DiscussionMessage({message, ignored, disabled, onPreviewSuggestion, onIgnore}: Props) {
  const status = message.kind === 'discussion' ? (statusLabels[message.status ?? ''] ?? message.status) : (statusLabels[message.actionState] ?? message.actionState);
  const suggestions = (message.suggestions ?? []).filter(item => !ignored.has(item.id));
  return <article className={`rounded-lg p-3 text-sm ${message.role === 'author' ? 'ml-6 bg-primary-subtle' : 'mr-6 bg-surface-hover'}`}>
    <p className="whitespace-pre-wrap leading-relaxed">{message.text}</p>
    <p className="mt-2 text-[11px] text-text-muted">
      {message.role === 'author' ? '你' : 'AI 共创'}
      {message.candidateId ? ` · ${message.entityRef || message.fieldPath || message.candidateId}` : ''}
      {status ? ` · ${status}` : ''}
    </p>
    {message.status === 'outcome_unknown' && <p className="mt-2 text-xs text-warning">运行结果未知，请核对后决定是否重新提问；不会自动再次调用模型。</p>}
    {suggestions.map(item => <DiscussionSuggestionCard key={item.id} suggestion={item} disabled={disabled} onPreview={() => onPreviewSuggestion?.(message.id, item.id)} onIgnore={() => onIgnore(item.id)} />)}
  </article>;
}
