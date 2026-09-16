import { Button } from '../ui/Button';
import { StatusBadge } from '../ui/StatusBadge';

// 逐项审阅决定：接受只记录审阅元数据，排除表示入档时跳过该单位，撤销决定回到未审阅。
export type ReviewDecision = 'accept' | 'reject' | 'unreviewed';

interface ReviewState {
  label: string;
  variant: 'default' | 'success' | 'danger';
}

// 审阅状态文案与配色：接受与排除必须一眼可分。
export function reviewState(accepted: boolean, rejected: boolean): ReviewState {
  if (accepted) return { label: '已接受', variant: 'success' };
  if (rejected) return { label: '已排除', variant: 'danger' };
  return { label: '未审阅', variant: 'default' };
}

export function ReviewStateBadge({ accepted, rejected }: { accepted: boolean; rejected: boolean }) {
  const state = reviewState(accepted, rejected);
  return (
    <StatusBadge variant={state.variant} className="shrink-0">{state.label}</StatusBadge>
  );
}

// 路线图导航复用既有 RoadmapNavigation，只能通过标题后缀呈现审阅状态；未决定时不加后缀。
export function reviewNavSuffix(accepted: boolean, rejected: boolean): string {
  if (accepted) return ' · 已接受';
  if (rejected) return ' · 已排除';
  return '';
}

interface Props {
  name: string;
  accepted: boolean;
  rejected: boolean;
  disabled?: boolean;
  onDecide: (decision: ReviewDecision) => void;
}

// 接受、排除、撤销决定三个按钮；已生效的决定按钮不再重复触发同一命令。
export function ReviewDecisionButtons({ name, accepted, rejected, disabled, onDecide }: Props) {
  return (
    <div className="flex items-center gap-2">
      <Button
        size="sm"
        variant={accepted ? 'primary' : 'default'}
        aria-label={`接受 ${name}`}
        disabled={disabled || accepted}
        onClick={() => onDecide('accept')}
      >
        接受
      </Button>
      <Button
        size="sm"
        variant={rejected ? 'danger' : 'default'}
        aria-label={`排除 ${name}`}
        disabled={disabled || rejected}
        onClick={() => onDecide('reject')}
      >
        排除
      </Button>
      <Button
        size="sm"
        variant="ghost"
        aria-label={`撤销决定 ${name}`}
        disabled={disabled || (!accepted && !rejected)}
        onClick={() => onDecide('unreviewed')}
      >
        撤销决定
      </Button>
    </div>
  );
}
