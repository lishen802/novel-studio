import { z } from "zod";
import {
  RevisionConflictError,
  StudioClientError,
  type ChapterView,
  type CreateProjectInput,
  type ProjectView,
  type StudioClient,
} from "./studio-client";

const projectSchema = z.object({
  id: z.string(),
  title: z.string(),
  genre: z.string(),
  summary: z.string(),
  path: z.string(),
});

const chapterSchema = z.object({
  id: z.string(),
  projectId: z.string(),
  title: z.string(),
  position: z.number(),
  body: z.string(),
  revision: z.number(),
  updatedAt: z.string(),
  recoveryBody: z.string().optional(),
  recoveryUpdatedAt: z.string().optional(),
  hasRecoveryDraft: z.boolean(),
});

type NativeDesktopAPI = {
  ChooseProjectParent(): Promise<string>;
  ChooseProjectToOpen(): Promise<string>;
  CreateProject(input: CreateProjectInput): Promise<unknown>;
  OpenProject(path: string): Promise<unknown>;
  ListChapters(): Promise<unknown>;
  CreateChapter(title: string): Promise<unknown>;
  OpenChapter(id: string): Promise<unknown>;
  SaveDraft(id: string, body: string, revision: number): Promise<void>;
  DiscardDraft(id: string, revision: number): Promise<void>;
  CommitChapter(id: string, body: string, revision: number): Promise<unknown>;
  CompleteClose(): Promise<void>;
};

type NativeRuntime = {
  EventsOnMultiple(
    eventName: string,
    callback: (...data: unknown[]) => void,
    maxCallbacks: number,
  ): () => void;
};

declare global {
  interface Window {
    go?: { main?: { DesktopAPI?: NativeDesktopAPI } };
    runtime?: NativeRuntime;
  }
}

function nativeAPI(): NativeDesktopAPI {
  const api = window.go?.main?.DesktopAPI;
  if (!api) throw new Error("桌面服务尚未就绪");
  return api;
}

function projectView(value: unknown): ProjectView {
  return projectSchema.parse(value);
}

function chapterView(value: unknown): ChapterView {
  return chapterSchema.parse(value);
}

function chapterViews(value: unknown): ChapterView[] {
  return z.array(chapterSchema).parse(value);
}

function safeCall<T>(operation: () => Promise<T>): Promise<T> {
  return operation().catch((error: unknown) => {
    const message = error instanceof Error ? error.message : String(error);
    const coded = message.match(/([A-Z][A-Z_]+)\|(.+)$/s);
    if (coded) {
      const [, code, safeMessage] = coded;
      if (code === "REVISION_CONFLICT") throw new RevisionConflictError();
      throw new StudioClientError(code, safeMessage);
    }
    if (/revision|版本冲突/i.test(message)) throw new RevisionConflictError();
    throw error instanceof Error ? error : new Error("桌面操作失败");
  });
}

export const wailsStudioClient: StudioClient = {
  async chooseProjectParent() {
    return (await safeCall(() => nativeAPI().ChooseProjectParent())) || null;
  },
  async chooseProjectToOpen() {
    return (await safeCall(() => nativeAPI().ChooseProjectToOpen())) || null;
  },
  async createProject(input) {
    return projectView(await safeCall(() => nativeAPI().CreateProject(input)));
  },
  async openProject(path) {
    return projectView(await safeCall(() => nativeAPI().OpenProject(path)));
  },
  async listChapters() {
    return chapterViews(await safeCall(() => nativeAPI().ListChapters()));
  },
  async createChapter(title) {
    return chapterView(await safeCall(() => nativeAPI().CreateChapter(title)));
  },
  async openChapter(id) {
    return chapterView(await safeCall(() => nativeAPI().OpenChapter(id)));
  },
  async saveDraft(id, body, revision) {
    await safeCall(() => nativeAPI().SaveDraft(id, body, revision));
  },
  async discardDraft(id, revision) {
    await safeCall(() => nativeAPI().DiscardDraft(id, revision));
  },
  async commitChapter(id, body, revision) {
    return chapterView(
      await safeCall(() => nativeAPI().CommitChapter(id, body, revision)),
    );
  },
  onBeforeClose(handler) {
    if (!window.runtime) return () => undefined;
    return window.runtime.EventsOnMultiple(
      "app:before-close",
      () => void handler().catch(() => undefined),
      -1,
    );
  },
  async completeClose() {
    await safeCall(() => nativeAPI().CompleteClose());
  },
};
