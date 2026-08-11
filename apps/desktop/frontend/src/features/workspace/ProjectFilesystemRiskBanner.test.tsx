import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ProjectFilesystemRiskBanner } from "./ProjectFilesystemRiskBanner";

describe("ProjectFilesystemRiskBanner", () => {
  it("does not render for a normal project filesystem", () => {
    render(<ProjectFilesystemRiskBanner health={{ level: "normal", category: "", actions: [] }} />);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  it("shows a non-blocking warning", () => {
    render(<ProjectFilesystemRiskBanner health={{ level: "warning", category: "sync_risk", actions: [] }} />);
    expect(screen.getByRole("status")).toHaveTextContent("项目文件系统存在风险");
    expect(screen.queryByRole("button", { name: "创建恢复副本" })).not.toBeInTheDocument();
  });

  it("shows degraded recovery action without unsafe filesystem details", () => {
    const onCreateRecoveryCopy = vi.fn();
    render(<ProjectFilesystemRiskBanner health={{ level: "degraded", category: "sharing_violation", actions: ["retry", "copy_text", "create_recovery_copy"] }} onCreateRecoveryCopy={onCreateRecoveryCopy} />);
    const status = screen.getByRole("status");
    expect(status).toHaveTextContent("项目写入遇到风险");
    expect(status).toHaveTextContent("未保存文本仍保留在当前编辑器中");
    expect(status).not.toHaveTextContent("/Users");
    expect(status).not.toHaveTextContent("SQLite");
    fireEvent.click(screen.getByRole("button", { name: "创建恢复副本" }));
    expect(onCreateRecoveryCopy).toHaveBeenCalledOnce();
  });
});
