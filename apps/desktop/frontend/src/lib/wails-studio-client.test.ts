import { afterEach, describe, expect, it, vi } from "vitest";
import { RevisionConflictError, StudioClientError } from "./studio-client";
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
    DiscardDraft: vi.fn(async () => undefined),
    CommitChapter: vi.fn(),
    CompleteClose: vi.fn(async () => undefined),
    ...overrides,
  };
  window.go = { main: { DesktopAPI: api } } as typeof window.go;
  return api;
}

afterEach(() => {
  delete window.go;
  delete window.runtime;
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

  it("rejects an invalid native DTO", async () => {
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

  it("forwards explicit recovery draft deletion", async () => {
    const api = installNative();

    await wailsStudioClient.discardDraft("chapter-1", 4);

    expect(api.DiscardDraft).toHaveBeenCalledWith("chapter-1", 4);
  });

  it("registers the native close event and completes the close handshake", async () => {
    const api = installNative();
    let beforeClose: (() => void) | undefined;
    const cancel = vi.fn();
    window.runtime = {
      EventsOnMultiple: vi.fn((_event, callback) => {
        beforeClose = callback as () => void;
        return cancel;
      }),
    };
    const handler = vi.fn(async () => undefined);

    const unsubscribe = wailsStudioClient.onBeforeClose(handler);
    beforeClose?.();
    await Promise.resolve();
    await wailsStudioClient.completeClose();
    unsubscribe();

    expect(window.runtime.EventsOnMultiple).toHaveBeenCalledWith(
      "app:before-close",
      expect.any(Function),
      -1,
    );
    expect(handler).toHaveBeenCalledOnce();
    expect(api.CompleteClose).toHaveBeenCalledOnce();
    expect(cancel).toHaveBeenCalledOnce();
  });

  it("shows only the safe message from a coded desktop error", async () => {
    installNative({
      OpenProject: vi.fn(async () => {
        throw new Error("PROJECT_CORRUPT|项目格式无效或数据已损坏。");
      }),
    });

    const operation = wailsStudioClient.openProject("/books/broken.zjproject");
    await expect(operation).rejects.toBeInstanceOf(StudioClientError);
    await expect(operation).rejects.toMatchObject({
      code: "PROJECT_CORRUPT",
      message: "项目格式无效或数据已损坏。",
    });
  });
});
