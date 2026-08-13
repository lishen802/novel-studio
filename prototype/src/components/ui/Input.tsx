import React from 'react';
import { clsx } from 'clsx';

interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {}

export const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, ...props }, ref) => (
    <input
      ref={ref}
      className={clsx(
        'w-full h-9 px-3 bg-surface border border-border rounded-md text-sm text-text placeholder:text-text-muted focus-ring outline-none transition-colors',
        className,
      )}
      {...props}
    />
  ),
);
Input.displayName = 'Input';

interface TextareaProps extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {}

export const Textarea = React.forwardRef<HTMLTextAreaElement, TextareaProps>(
  ({ className, ...props }, ref) => (
    <textarea
      ref={ref}
      className={clsx(
        'w-full px-3 py-2 bg-surface border border-border rounded-md text-sm text-text placeholder:text-text-muted focus-ring outline-none transition-colors resize-none',
        className,
      )}
      {...props}
    />
  ),
);
Textarea.displayName = 'Textarea';
