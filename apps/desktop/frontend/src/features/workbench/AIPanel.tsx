import { Sparkles, X } from "lucide-react";
import { useState } from "react";
import type { AIPanelMode, AIPanelSelection } from "../../lib/studio-client";

type AIPanelProps = {
  projectId: string;
  mode: AIPanelMode;
  input: string;
  onInput: (input: string) => void;
  selection: AIPanelSelection | null;
  onSelectionReview: (selection: AIPanelSelection) => void;
  onDismissSelection: () => void;
  // R2 不会提交选区意见，保留可选参数仅兼容现有调用方。
  onSubmitSelectionReview?: (payload: { selection: AIPanelSelection; input: string }) => void;
  // 关闭权交给工作区统一处理，确保与 Cmd/Ctrl+J 使用同一展示状态。
  onClose?: () => void;
};

function AIPanelHeader({
  onClose,
}: {
  onClose?: () => void;
}) {
  return (
    <header className="ai-panel__header">
      <div>
        <Sparkles size={17} aria-hidden="true" />
        <h2>AI 创作</h2>
      </div>
      {onClose && (
        <button
          type="button"
          className="ai-panel__close"
          aria-label="关闭 AI 面板"
          onClick={onClose}
        >
          <X size={17} aria-hidden="true" />
        </button>
      )}
    </header>
  );
}

// AIPanel 是受控 AI 面板：仅由 mode 决定可见性与形态，不允许本地组件自行推断。
// hidden 返回 null；expanded 展示输入区；collapsed 折叠为可展开的补充区域；
// selection_review 展示选区元数据与未就绪说明，不伪造提交结果。
export function AIPanel({
  mode,
  input,
  onInput,
  selection,
  onDismissSelection,
  onClose,
}: AIPanelProps) {
  const [expanded, setExpanded] = useState(false);

  if (mode === "hidden") {
    return null;
  }

  if (mode === "collapsed") {
    return (
      <aside
        className="ai-panel ai-panel--collapsed"
        aria-label="AI 创作"
        role="complementary"
      >
        <AIPanelHeader onClose={onClose} />
        <button
          type="button"
          onClick={() => setExpanded(true)}
          hidden={expanded}
        >
          展开 AI 面板
        </button>
        {expanded && (
          <div className="ai-panel__body">
            <p className="ai-panel__notice" role="status">
              AI 创作能力尚未就绪，不会生成候选或改写正文。
            </p>
            <textarea
              aria-label="给 AI 的说明"
              value={input}
              onChange={(e) => onInput(e.target.value)}
              placeholder="给 AI 的说明……"
            />
          </div>
        )}
      </aside>
    );
  }

  if (mode === "selection_review" && selection) {
    return (
      <aside
        className="ai-panel ai-panel--selection-review"
        aria-label="AI 创作"
        role="complementary"
      >
        <AIPanelHeader />
        <div className="ai-panel__body">
          <div className="ai-panel__selection">
            <span>
              Revision {selection.baseRevision} · 字符 {selection.start}–{selection.end}
            </span>
            <blockquote>{selection.text}</blockquote>
          </div>
          <textarea
            aria-label="给 AI 的说明"
            value={input}
            onChange={(e) => onInput(e.target.value)}
            placeholder="告诉 AI 这里的问题……"
          />
          <p className="ai-panel__notice" role="status">
            选区意见提交将在后续阶段提供；当前不会生成或提交 AI 请求。
          </p>
          <div className="ai-panel__actions">
            <button type="button" onClick={onDismissSelection}>
              取消选区意见
            </button>
          </div>
        </div>
      </aside>
    );
  }

  // expanded 模式：展示完整的输入区
  return (
    <aside
      className="ai-panel ai-panel--expanded"
      aria-label="AI 创作"
      role="complementary"
    >
      <AIPanelHeader onClose={onClose} />
      <div className="ai-panel__body">
        <p className="ai-panel__notice" role="status">
          AI 创作能力尚未就绪，不会生成候选或改写正文。
        </p>
        <textarea
          aria-label="给 AI 的说明"
          value={input}
          onChange={(e) => onInput(e.target.value)}
          placeholder="给 AI 的说明……"
        />
      </div>
    </aside>
  );
}
