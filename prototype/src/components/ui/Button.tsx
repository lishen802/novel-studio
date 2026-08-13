import { forwardRef, type ButtonHTMLAttributes } from 'react';
import { clsx } from 'clsx';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'default' | 'primary' | 'ghost' | 'danger';
  size?: 'sm' | 'md' | 'lg';
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = 'default', size = 'md', className, children, ...props }, ref) => (
    <button
      ref={ref}
      className={clsx(
        'inline-flex items-center justify-center gap-2 rounded-md font-ui transition-colors focus-ring',
        size === 'sm' && 'h-7 px-3 text-xs',
        size === 'md' && 'h-9 px-4 text-sm',
        size === 'lg' && 'h-11 px-6 text-base',
        variant === 'default' &&
          'bg-surface-elevated border border-border text-text hover:bg-surface-hover hover:border-border-strong',
        variant === 'primary' &&
          'bg-primary text-white border border-primary hover:bg-primary-hover font-medium',
        variant === 'ghost' &&
          'bg-transparent text-text-sub hover:text-text hover:bg-surface-hover',
        variant === 'danger' &&
          'bg-transparent text-danger border border-danger/30 hover:bg-danger/10',
        props.disabled && 'opacity-40 cursor-not-allowed',
        className,
      )}
      {...props}
    >
      {children}
    </button>
  ),
);
Button.displayName = 'Button';
