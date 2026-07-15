import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { App } from "./App";

describe("App", () => {
  it("shows the product identity and project entry actions", () => {
    render(<App />);

    expect(
      screen.getByRole("heading", { name: "织境 Story Studio" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "创建项目" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "打开项目" })).toBeEnabled();
  });
});
