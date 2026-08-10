import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ProjectLeaseConflictCard } from "./ProjectLeaseConflictCard";

describe("ProjectLeaseConflictCard", () => {
  it("shows holder details and routes every recovery action", async () => {
    const retry = vi.fn();
    const readOnly = vi.fn();
    const recoveryCopy = vi.fn();
    const cancel = vi.fn();
    const user = userEvent.setup();
    render(
      <ProjectLeaseConflictCard
        conflict={{
          path: "/books/长夜织境.zjproject",
          retryable: true,
          holder: {
            pid: 42,
            hostname: "author-mac",
            applicationVersion: "0.1.0",
            startedAt: "2026-08-10T09:00:00Z",
          },
        }}
        onRetry={retry}
        onOpenReadOnly={readOnly}
        onCreateRecoveryCopy={recoveryCopy}
        onCancel={cancel}
      />,
    );

    expect(screen.getByText("author-mac")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "重试" }));
    await user.click(screen.getByRole("button", { name: "只读打开" }));
    await user.click(screen.getByRole("button", { name: "创建恢复副本" }));
    await user.click(screen.getByRole("button", { name: "取消" }));

    expect(retry).toHaveBeenCalledOnce();
    expect(readOnly).toHaveBeenCalledOnce();
    expect(recoveryCopy).toHaveBeenCalledOnce();
    expect(cancel).toHaveBeenCalledOnce();
  });

  it("explains the safe options when holder metadata is unavailable", () => {
    render(
      <ProjectLeaseConflictCard
        conflict={{ path: "/books/长夜织境.zjproject", retryable: true }}
        onRetry={() => undefined}
        onOpenReadOnly={() => undefined}
        onCreateRecoveryCopy={() => undefined}
        onCancel={() => undefined}
      />,
    );

    expect(screen.getByText(/详细信息不可用/)).toBeInTheDocument();
  });
});
