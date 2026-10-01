import { useEffect, useMemo, useState } from 'react';
import { MessageSquare, Send } from 'lucide-react';
import { Button } from '../ui/Button';
import { Textarea } from '../ui/Input';
import type { AssistantMessageView, DiscussionAuthorizationView, DiscussionTurnView } from '../../api/studio';
import type { FieldEditor } from './CandidateFields';
import { DiscussionAuthorizationModal } from './DiscussionAuthorizationModal';
import { DiscussionMessage } from './DiscussionMessage';

interface Props {
  editor?: FieldEditor;
  entityRef?: string;
  messages: AssistantMessageView[];
  authorization?: DiscussionAuthorizationView;
	 authorizations?: DiscussionAuthorizationView[];
	 pendingDiscussion?: DiscussionTurnView;
  disabled?: boolean;
  onSend: (request: {candidateId:string;entityRef:string;fieldPath:string;text:string;idempotencyKey:string}) => Promise<void>;
  onPreviewDiscussion?: (request: {candidateId:string;entityRef:string;fieldPaths:string[];message:string;idempotencyKey:string}) => Promise<DiscussionTurnView>;
  onConfirmDiscussion?: (command: {actionId:string;argumentsHash:string;idempotencyKey:string;message:string;messageIdempotencyKey:string}) => Promise<void>;
  onSendDiscussion?: (request: {authorizationId:string;candidateId:string;entityRef:string;fieldPaths:string[];text:string;idempotencyKey:string}) => Promise<void>;
  onRevokeDiscussion?: (id: string) => Promise<void>;
  onPreviewSuggestion?: (messageId: string, suggestionId: string) => Promise<void>;
}

function boundToCurrent(auth: DiscussionAuthorizationView | undefined, candidateId: string, entityRef: string, requestedPaths: string[]) {
	if (!auth || auth.candidateId !== candidateId) return false;
  if ((auth.entityRef ?? '') !== entityRef) return false;
  const paths = auth.fieldPaths ?? [];
  return paths.length === 0 || requestedPaths.every(path => paths.includes(path));
}

export function CoCreationPanel({editor, entityRef = '', messages, authorization, authorizations = [], pendingDiscussion, disabled, onSend, onPreviewDiscussion, onConfirmDiscussion, onSendDiscussion, onRevokeDiscussion, onPreviewSuggestion}: Props) {
  const available = useMemo(() => editor?.fields ?? [], [editor]);
  const pendingPaths = pendingDiscussion?.authorization?.fieldPaths ?? [];
  const pendingFieldPath = pendingPaths.length > 1 ? '*' : pendingPaths[0] ?? '';
  const [fieldPath, setFieldPath] = useState(pendingFieldPath || available[0]?.path || '');
  const [text, setText] = useState(pendingDiscussion?.draftMessage ?? '');
  const bindingKey = (path: string) => `${editor?.candidateId ?? ''}\u0000${editor?.version ?? 0}\u0000${entityRef}\u0000${path}`;
  const [draftBase, setDraftBase] = useState(bindingKey(pendingFieldPath || available[0]?.path || ''));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [preview, setPreview] = useState<DiscussionTurnView | null>(pendingDiscussion ?? null);
  const [ignored, setIgnored] = useState<Set<string>>(new Set());

  useEffect(() => {
    if (fieldPath !== '*' && !available.some(field => field.path === fieldPath)) setFieldPath(available[0]?.path ?? '');
  }, [editor?.candidateId, available, fieldPath]);

  const fieldPaths = fieldPath === '*' ? available.map(field => field.path) : [fieldPath];
  const discussionEntity = fieldPath === '*' ? '' : entityRef;
  const staleDraft = !!text.trim() && draftBase !== bindingKey(fieldPath);
	const targetAuth = [...(authorizations ?? []), ...(authorization ? [authorization] : [])].reverse().find(auth => boundToCurrent(auth, editor?.candidateId ?? '', discussionEntity, fieldPaths));
	const currentAuth = targetAuth && !targetAuth.stale && targetAuth.state === 'active' ? targetAuth : undefined;
	const staleAuth = targetAuth && (targetAuth.stale || targetAuth.state === 'stale');

  const act = async (work: () => Promise<void>, clear = false) => {
    setBusy(true); setError('');
    try {
      await work();
      if (clear) { setText(''); setDraftBase(bindingKey(fieldPath)); }
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : '操作失败，请重试。');
    } finally { setBusy(false); }
  };

  const propose = () => {
    if (!editor || !fieldPath || !text.trim() || staleDraft || fieldPath === '*' || available.find(field => field.path === fieldPath)?.locked) return;
    void act(async () => { await onSend({candidateId: editor.candidateId, entityRef, fieldPath, text: text.trim(), idempotencyKey: crypto.randomUUID()}); }, true);
  };

  const discuss = () => {
    if (!editor || !fieldPath || !text.trim() || staleDraft || !onPreviewDiscussion) return;
    if (currentAuth?.id && onSendDiscussion) {
      void act(async () => { await onSendDiscussion({authorizationId: currentAuth.id ?? '', candidateId: editor.candidateId, entityRef:discussionEntity, fieldPaths, text: text.trim(), idempotencyKey: crypto.randomUUID()}); }, true);
      return;
    }
    void act(async () => {
      const next = await onPreviewDiscussion({candidateId: editor.candidateId, entityRef:discussionEntity, fieldPaths, message: text.trim(), idempotencyKey: crypto.randomUUID()});
      setPreview(next);
    });
  };

  return <section className="min-h-0 flex-1 bg-surface flex flex-col" aria-label="AI 共创对话">
    <div className="p-4 border-b border-border">
      <h2 className="text-sm font-medium text-text">AI 共创</h2>
      <p className="mt-1 text-xs text-text-sub">可以先讨论分析，再决定是否形成修改提案。讨论授权绑定当前候选版本，不会随导航换绑。</p>
      {currentAuth && <p className="mt-2 text-xs text-text-sub">剩余 {currentAuth.remainingCalls ?? 0} 次调用 / {(currentAuth.remainingTokens ?? 0).toLocaleString()} Token · 到期 {currentAuth.expiresAt} · 绑定 {currentAuth.entityRef || currentAuth.fieldPaths?.[0] || currentAuth.candidateId}</p>}
      {staleAuth && <p className="mt-2 text-xs text-warning">基准版本已变化，当前讨论授权不能继续发送。</p>}
      {currentAuth?.id && onRevokeDiscussion && <Button size="sm" className="mt-2" disabled={disabled || busy} onClick={() => void act(async () => { await onRevokeDiscussion(currentAuth.id ?? ''); })}>撤销讨论授权</Button>}
    </div>
    <div className="flex-1 overflow-auto p-4 space-y-3">
      {messages.length === 0 && <p className="text-sm text-text-muted">可以讨论整个当前候选或某个字段；提出修改需要选定具体字段。</p>}
      {messages.map(message => <DiscussionMessage key={message.id} message={message} ignored={ignored} disabled={disabled || busy} onPreviewSuggestion={onPreviewSuggestion} onIgnore={id => setIgnored(old => new Set(old).add(id))} />)}
    </div>
    <div className="p-4 border-t border-border space-y-3">
      <select aria-label="共创目标字段" className="w-full rounded-md border border-border bg-surface p-2 text-sm" value={fieldPath} disabled={disabled || busy || available.length === 0} onChange={event => setFieldPath(event.target.value)}>
        <option value="*">当前候选整体（仅讨论）</option>
        {available.map(field => <option key={field.path} value={field.path}>{field.group} · {field.label}{field.locked ? "（已锁定，可讨论）" : ""}</option>)}
      </select>
      <Textarea aria-label="共创消息" placeholder="例如：这个人物的动机哪里不够可信？" maxLength={4000} value={text} disabled={disabled || busy || !editor} onChange={event => { const next = event.target.value; if (!text.trim() && next.trim()) setDraftBase(bindingKey(fieldPath)); setText(next); }} />
      {staleDraft && <p role="alert" className="text-xs text-warning">候选、对象或字段已变化，输入已保留。<Button size="sm" onClick={() => setDraftBase(bindingKey(fieldPath))}>核对后绑定当前目标</Button></p>}
      {error && <p role="alert" className="text-xs text-danger">{error}</p>}
      <div className="grid grid-cols-2 gap-2">
        <Button className="w-full gap-2" disabled={disabled || busy || !editor || !fieldPath || !text.trim() || staleDraft || staleAuth} onClick={() => void discuss()}>
          <MessageSquare className="w-4 h-4" />{busy ? '处理中…' : '讨论这个问题'}
        </Button>
        <Button variant="primary" className="w-full gap-2" disabled={disabled || busy || !editor || !fieldPath || !text.trim() || staleDraft || fieldPath === '*' || available.find(field => field.path === fieldPath)?.locked} onClick={() => void propose()}>
          <Send className="w-4 h-4" />提出修改
        </Button>
      </div>
    </div>
    <DiscussionAuthorizationModal
      open={Boolean(preview)}
      preview={preview?.authorization}
      confirming={busy}
      onClose={() => setPreview(null)}
      onConfirm={() => {
        if (!preview || !onConfirmDiscussion || !text.trim()) return;
        void act(async () => {
          await onConfirmDiscussion({actionId: preview.actionId, argumentsHash: preview.argumentsHash, idempotencyKey: preview.idempotencyKey, message: text.trim(), messageIdempotencyKey: crypto.randomUUID()});
          setPreview(null);
        }, true);
      }}
    />
  </section>;
}
