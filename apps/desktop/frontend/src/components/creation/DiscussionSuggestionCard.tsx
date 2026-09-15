import { Button } from '../ui/Button';
import type { DiscussionSuggestionView } from '../../api/studio';

interface Props {
  suggestion: DiscussionSuggestionView;
  disabled?: boolean;
  onPreview?: () => Promise<void> | void;
  onIgnore: () => void;
}

export function DiscussionSuggestionCard({suggestion, disabled, onPreview, onIgnore}: Props) {
  return <div className="mt-3 rounded-md border border-border bg-surface p-3 space-y-2">
    <p className="text-xs text-text-sub">建议修改 {(suggestion.paths ?? []).join('、')}</p>
    <p className="text-sm whitespace-pre-wrap">{suggestion.instruction}</p>
    {(suggestion.preservePaths ?? []).length > 0 && <p className="text-xs text-text-sub">保持不变：{(suggestion.preservePaths ?? []).join('、')}</p>}
    {suggestion.rationale && <p className="text-xs text-text-muted">可能影响：{suggestion.rationale}</p>}
    <div className="flex flex-wrap gap-2">
      <Button size="sm" variant="primary" disabled={disabled || !onPreview} onClick={() => void onPreview?.()}>查看修改提案与费用</Button>
      <Button size="sm" disabled={disabled} onClick={onIgnore}>忽略建议</Button>
    </div>
  </div>;
}
