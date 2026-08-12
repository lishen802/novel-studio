import type { ReactNode } from "react";
import type { WorkbenchProjection } from "../../lib/studio-client";
import { WholeBookArchiveCard } from "../imports/WholeBookArchiveCard";

type StageCanvasProps = {
  projection: WorkbenchProjection;
  onStartWholeBookArchive: () => void;
  children?: ReactNode;
};

// StageCanvas 根据 WorkbenchProjection 渲染不同阶段的画布内容。仅 manuscript
// 画布展示正文与章节入口和整书档案卡片；其它画布展示诚实的未就绪状态，不能
// 伪造档案入口或已完成效果。
export function StageCanvas({
  projection,
  onStartWholeBookArchive,
  children,
}: StageCanvasProps) {
  if (projection.canvas !== "manuscript") {
    return (
      <div className="stage-canvas stage-canvas--not-ready">
        <p>尚未进入正文写作，当前阶段未就绪。</p>
      </div>
    );
  }

  return (
    <div className="stage-canvas stage-canvas--manuscript">
      <header className="stage-canvas__header">
        <h2>正文与章节</h2>
      </header>
      <WholeBookArchiveCard
        status={projection.archiveStatus}
        onStart={onStartWholeBookArchive}
      />
      {children}
    </div>
  );
}
