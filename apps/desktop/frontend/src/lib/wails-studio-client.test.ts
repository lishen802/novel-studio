import { afterEach, describe, expect, it, vi } from "vitest";
import { RevisionConflictError } from "./studio-client";
import { wailsStudioClient } from "./wails-studio-client";

const project = {
  id: "project-1",
  title: "长夜织境",
  genre: "东方奇幻",
  summary: "",
  path: "/books/长夜织境.zjproject",
};

function installNative(overrides: Record<string, unknown> = {}) {
  const api = {
    ChooseProjectParent: vi.fn(async () => ""),
    ChooseProjectToOpen: vi.fn(async () => ""),
    CreateProject: vi.fn(async () => project),
    OpenProject: vi.fn(async () => project),
    ListChapters: vi.fn(async () => []),
    CreateChapter: vi.fn(),
    OpenChapter: vi.fn(),
    SaveDraft: vi.fn(async () => undefined),
    CommitChapter: vi.fn(),
    ...overrides,
  };
  window.go = { main: { DesktopAPI: api } } as typeof window.go;
  return api;
}

afterEach(() => {
  delete window.go;
});

describe("wailsStudioClient", () => {
  it("maps an empty native directory selection to cancellation", async () => {
    installNative();

    await expect(wailsStudioClient.chooseProjectParent()).resolves.toBeNull();
  });

  it("forwards create input and validates the returned project DTO", async () => {
    const api = installNative();
    const input = {
      parentDirectory: "/books",
      title: "长夜织境",
      genre: "东方奇幻",
      summary: "",
    };

    await expect(wailsStudioClient.createProject(input)).resolves.toEqual(project);
    expect(api.CreateProject).toHaveBeenCalledWith(input);
  });

  it("rejects an invalid native DTO during development", async () => {
    installNative({ CreateProject: vi.fn(async () => ({ title: "missing fields" })) });

    await expect(
      wailsStudioClient.createProject({
        parentDirectory: "/books",
        title: "长夜织境",
        genre: "",
        summary: "",
      }),
    ).rejects.toThrow();
  });

  it("maps a native revision error to a typed conflict", async () => {
    installNative({
      CommitChapter: vi.fn(async () => {
        throw new Error("commit chapter: revision conflict");
      }),
    });

    await expect(
      wailsStudioClient.commitChapter("chapter-1", "正文", 1),
    ).rejects.toBeInstanceOf(RevisionConflictError);
  });
});
