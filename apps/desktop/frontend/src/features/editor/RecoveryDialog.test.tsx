import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { FakeStudioClient } from "../../lib/fake-studio-client";
import { RecoveryDialog } from "./RecoveryDialog";

const recoveryChapter = {
  ...FakeStudioClient.withReadyProject().createChapterResult,
  body: "已提交正文",
  updatedAt: "2026-07-15T08:00:00Z",
  recoveryBody: "中断前的恢复草稿",
  recoveryUpdatedAt: "2026-07-15T08:05:00Z",
  hasRecoveryDraft: true,
};

describe("RecoveryDialog", () => {
  it("shows timestamps and character summaries for both versions", () => {
    render(
      <RecoveryDialog
        chapter={recoveryChapter}
        onRestore={vi.fn()}
        onDiscard={vi.fn()}
        onLater={vi.fn()}
      />,
    );

    expect(screen.getByText("已提交版本")).toBeInTheDocument();
    expect(screen.getByText("恢复草稿", { selector: "h3" })).toBeInTheDocument();
    expect(screen.getByText("5 字")).toBeInTheDocument();
    expect(screen.getByText("8 字")).toBeInTheDocument();
    expect(screen.getAllByText(/2026/)).toHaveLength(2);
  });

  it("requires confirmation before deleting the recovery draft", async () => {
    const onDiscard = vi.fn(async () => undefined);
    render(
      <RecoveryDialog
        chapter={recoveryChapter}
        onRestore={vi.fn()}
        onDiscard={onDiscard}
        onLater={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "保留已提交版" }));
    expect(onDiscard).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "确认丢弃恢复草稿" }));

    expect(onDiscard).toHaveBeenCalledTimes(1);
  });

  it("can defer the decision without deleting the draft", () => {
    const onLater = vi.fn();
    const onDiscard = vi.fn();
    render(
      <RecoveryDialog
        chapter={recoveryChapter}
        onRestore={vi.fn()}
        onDiscard={onDiscard}
        onLater={onLater}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "稍后决定" }));

    expect(onLater).toHaveBeenCalledTimes(1);
    expect(onDiscard).not.toHaveBeenCalled();
  });
});
