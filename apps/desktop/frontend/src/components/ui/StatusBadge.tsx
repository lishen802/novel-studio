import React from 'react';
import { clsx } from 'clsx';

interface StatusBadgeProps {
  children: React.ReactNode;
  variant?: 'default' | 'success' | 'warning' | 'danger' | 'info';
  className?: string;
}

export function StatusBadge({ children, variant = 'default', className }: StatusBadgeProps) {
  return (
    <span
      className={clsx(
        'inline-flex items-center px-2 py-0.5 rounded text-xs font-medium border',
        variant === 'default' && 'bg-surface-hover text-text-sub border-border',
        variant === 'success' && 'bg-success/10 text-success border-success/20',
        variant === 'warning' && 'bg-warning/10 text-warning border-warning/20',
        variant === 'danger' && 'bg-danger/10 text-danger border-danger/20',
        variant === 'info' && 'bg-info/10 text-info border-info/20',
        className,
      )}
    >
      {children}
    </span>
  );
}
