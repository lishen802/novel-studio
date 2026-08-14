import { clsx } from 'clsx';
import type { LucideIcon } from 'lucide-react';
import { Button } from './Button';

interface EmptyStateProps {
  icon: LucideIcon;
  title: string;
  description: string;
  action?: { label: string; onClick: () => void };
  className?: string;
}

export function EmptyState({ icon: Icon, title, description, action, className }: EmptyStateProps) {
  return (
    <div className={clsx('flex flex-col items-center justify-center text-center py-16', className)}>
      <div className="w-12 h-12 rounded-full bg-surface flex items-center justify-center mb-4 border border-border">
        <Icon className="w-6 h-6 text-text-muted" />
      </div>
      <h3 className="text-base font-medium text-text mb-1">{title}</h3>
      <p className="text-sm text-text-sub max-w-xs mb-5">{description}</p>
      {action && <Button onClick={action.onClick}>{action.label}</Button>}
    </div>
  );
}
