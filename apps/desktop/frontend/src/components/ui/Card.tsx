import React from 'react';
import { clsx } from 'clsx';

interface CardProps extends React.HTMLAttributes<HTMLDivElement> {
  padding?: 'none' | 'sm' | 'md' | 'lg';
  hover?: boolean;
}

export const Card = React.forwardRef<HTMLDivElement, CardProps>(
  ({ padding = 'md', hover = false, className, children, ...props }, ref) => (
    <div
      ref={ref}
      className={clsx(
        'bg-surface-elevated border border-border rounded-lg shadow-sm transition-colors',
        hover && 'hover:border-border-strong cursor-pointer',
        padding === 'none' && '',
        padding === 'sm' && 'p-3',
        padding === 'md' && 'p-4',
        padding === 'lg' && 'p-6',
        className,
      )}
      {...props}
    >
      {children}
    </div>
  ),
);
Card.displayName = 'Card';
