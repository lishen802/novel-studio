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
  onSubmitSelectionReview: (payload: { selection: AIPanelSelection; input: string }) => void;
};

// AIPanel 是受控 AI 面板：仅由 mode 决定可见性与形态，不允许本地组件自行推断。
// hidden 返回 null；expanded 展示输入区；collapsed 折叠为可展开的补充区域；
// selection_review 展示选区元数据和意见提交入口。
export function AIPanel({
  mode,
  input,
  onInput,
  selection,
  onDismissSelection,
  onSubmitSelectionReview,
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
        <button
          type="button"
          onClick={() => setExpanded(true)}
          hidden={expanded}
        >
          展开 AI 面板
        </button>
        {expanded && (
          <textarea
            aria-label="给 AI 的说明"
            value={input}
            onChange={(e) => onInput(e.target.value)}
            placeholder="给 AI 的说明……"
          />
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
        <div className="ai-panel__actions">
          <button
            type="button"
            onClick={() =>
              onSubmitSelectionReview({ selection, input })
            }
          >
            提交选区意见
          </button>
          <button type="button" onClick={onDismissSelection}>
            取消选区意见
          </button>
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
      <textarea
        aria-label="给 AI 的说明"
        value={input}
        onChange={(e) => onInput(e.target.value)}
        placeholder="给 AI 的说明……"
      />
    </aside>
  );
}
