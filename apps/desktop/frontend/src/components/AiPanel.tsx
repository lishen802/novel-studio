import { useEffect, useRef, useState } from 'react';
import { clsx } from 'clsx';
import { Send, X, Bot, User, Sparkles } from 'lucide-react';
import { useApp } from '../store/useApp';
import { Button } from './ui/Button';
import { Textarea } from './ui/Input';
import * as api from '../api/stubs';
import type { AiMessage } from '../types';

const agentLabels: Record<string, string> = {
  editor: '编辑官',
  plot: '情节师',
  world: '世界观顾问',
  style: '文风教练',
  general: 'AI 助手',
};

interface AiPanelProps {
  mode: 'writing' | 'review';
}

export function AiPanel({ mode }: AiPanelProps) {
  const { state, dispatch } = useApp();
  const [messages, setMessages] = useState<AiMessage[]>([]);
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (state.aiPanelOpen) {
      api.fetchChatHistory().then(setMessages);
    }
  }, [state.aiPanelOpen]);

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: 'smooth' });
  }, [messages]);

  const handleSend = async () => {
    if (!input.trim() || loading) return;
    const userMsg: AiMessage = {
      id: `u-${Date.now()}`,
      projectId: 'proj-01',
      role: 'author',
      content: input,
      timestamp: new Date().toISOString().slice(0, 16).replace('T', ' '),
    };
    setMessages((m) => [...m, userMsg]);
    setInput('');
    setLoading(true);
    const reply = await api.sendChatMessage(userMsg.content, 'plot');
    setMessages((m) => [...m, reply]);
    setLoading(false);
  };

  return (
    <>
      {/* Toggle button when closed */}
      {!state.aiPanelOpen && (
        <button
          onClick={() => dispatch({ type: 'TOGGLE_AI_PANEL' })}
          className="fixed right-5 bottom-5 z-30 w-10 h-10 rounded-full bg-primary text-white shadow-md flex items-center justify-center hover:bg-primary-hover transition-colors"
          title="打开 AI 面板"
        >
          <Sparkles className="w-5 h-5" />
        </button>
      )}

      {/* Panel */}
      <div
        className={clsx(
          'fixed top-10 right-0 bottom-0 w-[360px] bg-surface border-l border-border shadow-lg z-30 transition-transform duration-220 flex flex-col',
          state.aiPanelOpen ? 'translate-x-0' : 'translate-x-full',
        )}
        style={{ transitionTimingFunction: 'cubic-bezier(0.16, 1, 0.3, 1)' }}
      >
        <div className="h-12 border-b border-border flex items-center justify-between px-4 shrink-0">
          <div className="flex items-center gap-2">
            <Sparkles className="w-4 h-4 text-primary" />
            <span className="text-sm font-medium text-text">{mode === 'review' ? '审阅助手' : '创作助手'}</span>
          </div>
          <Button variant="ghost" size="sm" onClick={() => dispatch({ type: 'TOGGLE_AI_PANEL' })} className="px-2">
            <X className="w-4 h-4" />
          </Button>
        </div>

        <div ref={scrollRef} className="flex-1 overflow-auto p-4 space-y-4">
          <div className="p-3 bg-primary-subtle border border-primary/20 rounded-md text-xs text-primary leading-relaxed">
            {mode === 'review'
              ? '针对标注或候选提出意见时，AI 会先给出影响说明与修订候选，不会直接改写正文或档案。'
              : '需要时再唤起助手讨论选中文本或下一步写作。AI 不会直接修改正文，所有生成内容都会以候选形式交给你决定。'}
          </div>
          {messages.map((msg) => (
            <div
              key={msg.id}
              className={clsx('flex gap-3', msg.role === 'author' ? 'flex-row-reverse' : '')}
            >
              <div
                className={clsx(
                  'w-7 h-7 rounded-full flex items-center justify-center shrink-0',
                  msg.role === 'author' ? 'bg-surface-hover text-text' : 'bg-primary text-white',
                )}
              >
                {msg.role === 'author' ? <User className="w-4 h-4" /> : <Bot className="w-4 h-4" />}
              </div>
              <div
                className={clsx(
                  'max-w-[240px] p-3 rounded-lg text-sm leading-relaxed',
                  msg.role === 'author'
                    ? 'bg-surface-elevated text-text'
                    : 'bg-surface-hover border border-border text-text-sub',
                )}
              >
                {msg.role === 'ai' && msg.agent && (
                  <div className="text-xs text-primary mb-1">{agentLabels[msg.agent]}</div>
                )}
                <div className="whitespace-pre-wrap">{msg.content}</div>
              </div>
            </div>
          ))}
          {loading && (
            <div className="flex gap-3">
              <div className="w-7 h-7 rounded-full bg-primary text-white flex items-center justify-center">
                <Bot className="w-4 h-4" />
              </div>
              <div className="p-3 rounded-lg bg-surface-hover border border-border text-sm text-text-sub">
                <span className="inline-flex gap-1">
                  <span className="w-1.5 h-1.5 rounded-full bg-text-muted animate-bounce" />
                  <span className="w-1.5 h-1.5 rounded-full bg-text-muted animate-bounce [animation-delay:120ms]" />
                  <span className="w-1.5 h-1.5 rounded-full bg-text-muted animate-bounce [animation-delay:240ms]" />
                </span>
              </div>
            </div>
          )}
        </div>

        <div className="p-4 border-t border-border shrink-0">
          <div className="flex gap-2">
            <Textarea
              value={input}
              onChange={(e) => setInput(e.target.value)}
              placeholder="输入你的想法…"
              className="min-h-[72px] text-sm"
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault();
                  handleSend();
                }
              }}
            />
            <Button
              variant="primary"
              size="sm"
              onClick={handleSend}
              disabled={loading || !input.trim()}
              className="self-end h-9 px-3"
            >
              <Send className="w-4 h-4" />
            </Button>
          </div>
        </div>
      </div>
    </>
  );
}
