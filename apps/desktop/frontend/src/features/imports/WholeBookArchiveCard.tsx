import type { ArchiveStatus } from "../../lib/studio-client";

type WholeBookArchiveCardProps = {
  status: ArchiveStatus;
  onStart: () => void;
};

// WholeBookArchiveCard 渲染整书档案生成的入口卡片。R2 仅允许 not_generated
// 状态；按钮在 not_generated 时禁用，表达"将在后续能力阶段开放"，不能伪造
// 已完成效果。running 状态下按钮可点击，以表达档案生成进行中的可见性。
export function WholeBookArchiveCard({ status, onStart }: WholeBookArchiveCardProps) {
  const disabled = status === "not_generated";

  let statusText: string;
  if (status === "not_generated") {
    statusText = "尚未生成整本小说档案。";
  } else if (status === "running") {
    statusText = "整书档案生成中……";
  } else if (status === "completed") {
    statusText = "整书档案已生成。";
  } else {
    statusText = "整书档案生成失败，请重试。";
  }

  return (
    <section className="whole-book-archive-card" aria-label="整书档案">
      <h3>整书档案</h3>
      <p>{statusText}</p>
      <button
        type="button"
        onClick={disabled ? undefined : onStart}
        disabled={disabled}
      >
        AI 生成整书档案
      </button>
    </section>
  );
}
