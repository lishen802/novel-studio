import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { StageCanvas } from "./StageCanvas";
import type { WorkbenchProjection } from "../../lib/studio-client";

function writingProjection(
  overrides: Partial<WorkbenchProjection> = {},
): WorkbenchProjection {
  return {
    stage: "writing",
    canvas: "manuscript",
    aiPanel: "hidden",
    pendingReviewCount: 0,
    recoverableJobCount: 0,
    archiveStatus: "not_generated",
    ...overrides,
  };
}

describe("StageCanvas", () => {
  it("renders the manuscript canvas with a not_generated archive card", () => {
    render(
      <StageCanvas
        projection={writingProjection()}
        onStartWholeBookArchive={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("heading", { name: "正文与章节" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "AI 生成整书档案" }),
    ).toBeDisabled();
  });

  it("renders an honest not-ready state when the canvas is not manuscript", () => {
    render(
      <StageCanvas
        projection={writingProjection({ canvas: "idea", stage: "idea" })}
        onStartWholeBookArchive={vi.fn()}
      />,
    );
    expect(
      screen.getByText(/尚未进入正文写作|未就绪/),
    ).toBeInTheDocument();
    // 非正文画布不能伪造整书档案入口。
    expect(
      screen.queryByRole("button", { name: "AI 生成整书档案" }),
    ).not.toBeInTheDocument();
  });

  it("does not fabricate archive completion when archiveStatus is not_generated", () => {
    render(
      <StageCanvas
        projection={writingProjection({ archiveStatus: "not_generated" })}
        onStartWholeBookArchive={vi.fn()}
      />,
    );
    expect(
      screen.queryByText(/档案已生成|档案已完成/),
    ).not.toBeInTheDocument();
  });
});
