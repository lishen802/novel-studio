import { useEffect, useMemo, useState } from 'react';
import { Send } from 'lucide-react';
import { Button } from '../ui/Button';
import { Textarea } from '../ui/Input';
import type { AssistantMessageView } from '../../api/studio';
import type { FieldEditor } from './CandidateFields';

interface Props {
  editor?: FieldEditor;
  entityRef?: string;
  messages: AssistantMessageView[];
  disabled?: boolean;
  onSend: (request: {candidateId:string;entityRef:string;fieldPath:string;text:string;idempotencyKey:string}) => Promise<void>;
}

const actionLabels: Record<string, string> = {
  pending_confirmation: '修改提案待确认', executing: '正在修改与审阅',
  succeeded: '修改结果已就绪', rejected: '提案已放弃', expired: '提案已过期',
  conflict: '目标内容已变化', failed: '修改未完成', outcome_unknown: '需要核对运行结果',
};

// CoCreationPanel 将每条消息绑定到发送时的候选和字段，不随导航静默换绑。
export function CoCreationPanel({editor, entityRef = '', messages, disabled, onSend}: Props) {
  const available = useMemo(() => editor?.fields.filter(field => !field.locked) ?? [], [editor]);
  const [fieldPath, setFieldPath] = useState(available[0]?.path ?? '');
  const [text, setText] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!available.some(field => field.path === fieldPath)) setFieldPath(available[0]?.path ?? '');
  }, [editor?.candidateId, available, fieldPath]);

  const send = async () => {
    if (!editor || !fieldPath || !text.trim()) return;
    setBusy(true); setError('');
    try {
      await onSend({candidateId: editor.candidateId, entityRef, fieldPath, text: text.trim(), idempotencyKey: crypto.randomUUID()});
      setText('');
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : '消息发送失败，请重试。');
    } finally { setBusy(false); }
  };

  return <section className="min-h-0 flex-1 bg-surface flex flex-col" aria-label="AI 共创对话">
    <div className="p-4 border-b border-border">
      <h2 className="text-sm font-medium text-text">AI 共创</h2>
      <p className="mt-1 text-xs text-text-sub">消息绑定当前候选和字段。先形成提案，确认费用后才调用模型。</p>
    </div>
    <div className="flex-1 overflow-auto p-4 space-y-3">
      {messages.length === 0 && <p className="text-sm text-text-muted">选择一个字段，说说希望保留和调整什么。</p>}
      {messages.map(message => <article key={message.id} className={`rounded-lg p-3 text-sm ${message.role === 'author' ? 'ml-6 bg-primary-subtle' : 'mr-6 bg-surface-hover'}`}>
        <p className="whitespace-pre-wrap leading-relaxed">{message.text}</p>
        <p className="mt-2 text-[11px] text-text-muted">{message.role === 'author' ? '你' : 'AI 共创'}{message.actionState ? ` · ${actionLabels[message.actionState] ?? message.actionState}` : ''}</p>
      </article>)}
    </div>
    <div className="p-4 border-t border-border space-y-3">
      <select aria-label="共创目标字段" className="w-full rounded-md border border-border bg-surface p-2 text-sm" value={fieldPath} disabled={disabled || busy || available.length === 0} onChange={event => setFieldPath(event.target.value)}>
        {available.map(field => <option key={field.path} value={field.path}>{field.group} · {field.label}</option>)}
      </select>
      <Textarea aria-label="共创消息" placeholder="例如：保留人物的秘密，但让公开动机更可信。" maxLength={4000} value={text} disabled={disabled || busy || !editor} onChange={event => setText(event.target.value)} />
      {error && <p role="alert" className="text-xs text-danger">{error}</p>}
      <Button variant="primary" className="w-full gap-2" disabled={disabled || busy || !editor || !fieldPath || !text.trim()} onClick={() => void send()}>
        <Send className="w-4 h-4" />{busy ? '正在整理提案…' : '发送并查看修改提案'}
      </Button>
    </div>
  </section>;
}
