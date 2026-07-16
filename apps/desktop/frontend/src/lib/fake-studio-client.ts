import type {
  ChapterView,
  CreateProjectInput,
  CreateOutlineNodeInput,
  CreateSceneCardInput,
  CreateStoryLineInput,
  LinkStoryLineInput,
  OutlineNodeView,
  ProjectView,
  SceneCardView,
  StoryLineLinkView,
  StoryLineView,
  StoryStructureView,
  StudioClient,
} from "./studio-client";

type ChapterWriteCall = {
  chapterId: string;
  body: string;
  baseRevision: number;
};

export class FakeStudioClient implements StudioClient {
  projectParent: string | null = null;
  projectToOpen: string | null = null;
  createProjectResult: ProjectView = {
    id: "project-1",
    title: "长夜织境",
    genre: "东方奇幻",
    summary: "",
    path: "/books/长夜织境.zjproject",
  };
  openProjectResult: ProjectView = this.createProjectResult;
  chaptersResult: ChapterView[] = [];
  createChapterResult: ChapterView = {
    id: "chapter-1",
    projectId: "project-1",
    title: "第一章",
    position: 1,
    body: "",
    revision: 1,
    updatedAt: "2026-07-15T08:00:00Z",
    hasRecoveryDraft: false,
  };
  openChapterResult: ChapterView = this.createChapterResult;
  commitChapterResult: ChapterView = {
    ...this.createChapterResult,
    revision: 2,
  };
  storyStructureResult: StoryStructureView = { outlineNodes: [], storyLines: [], storyLineLinks: [], sceneCards: [] };

  createProjectError: Error | null = null;
  openProjectError: Error | null = null;
  createChapterError: Error | null = null;
  openChapterError: Error | null = null;
  saveDraftError: Error | null = null;
  discardDraftError: Error | null = null;
  commitChapterError: Error | null = null;

  createProjectCalls: CreateProjectInput[] = [];
  openProjectCalls: string[] = [];
  createChapterCalls: string[] = [];
  openChapterCalls: string[] = [];
  saveDraftCalls: ChapterWriteCall[] = [];
  discardDraftCalls: Array<{ chapterId: string; baseRevision: number }> = [];
  commitChapterCalls: ChapterWriteCall[] = [];
  createOutlineNodeCalls: CreateOutlineNodeInput[] = [];
  createStoryLineCalls: CreateStoryLineInput[] = [];
  linkStoryLineCalls: LinkStoryLineInput[] = [];
  createSceneCardCalls: CreateSceneCardInput[] = [];
  archiveOutlineNodeCalls: string[] = [];
  moveOutlineNodeCalls: Array<{ id: string; direction: "up" | "down" }> = [];
  completeCloseCalls = 0;
  beforeCloseHandler: (() => Promise<void>) | null = null;

  static withReadyProject(): FakeStudioClient {
    const client = new FakeStudioClient();
    client.chaptersResult = [];
    return client;
  }

  async chooseProjectParent(): Promise<string | null> {
    return this.projectParent;
  }

  async chooseProjectToOpen(): Promise<string | null> {
    return this.projectToOpen;
  }

  async createProject(input: CreateProjectInput): Promise<ProjectView> {
    this.createProjectCalls.push(input);
    if (this.createProjectError) throw this.createProjectError;
    return this.createProjectResult;
  }

  async openProject(path: string): Promise<ProjectView> {
    this.openProjectCalls.push(path);
    if (this.openProjectError) throw this.openProjectError;
    return this.openProjectResult;
  }

  async listChapters(): Promise<ChapterView[]> {
    return this.chaptersResult;
  }

  async createChapter(title: string): Promise<ChapterView> {
    this.createChapterCalls.push(title);
    if (this.createChapterError) throw this.createChapterError;
    const created = { ...this.createChapterResult, title };
    this.openChapterResult = created;
    this.commitChapterResult = { ...this.commitChapterResult, title };
    return created;
  }

  async openChapter(chapterId: string): Promise<ChapterView> {
    this.openChapterCalls.push(chapterId);
    if (this.openChapterError) throw this.openChapterError;
    const listed = this.chaptersResult.find((chapter) => chapter.id === chapterId);
    return listed ?? this.openChapterResult;
  }

  async saveDraft(
    chapterId: string,
    body: string,
    baseRevision: number,
  ): Promise<void> {
    this.saveDraftCalls.push({ chapterId, body, baseRevision });
    if (this.saveDraftError) throw this.saveDraftError;
  }

  async discardDraft(chapterId: string, baseRevision: number): Promise<void> {
    this.discardDraftCalls.push({ chapterId, baseRevision });
    if (this.discardDraftError) throw this.discardDraftError;
  }

  async commitChapter(
    chapterId: string,
    body: string,
    baseRevision: number,
  ): Promise<ChapterView> {
    this.commitChapterCalls.push({ chapterId, body, baseRevision });
    if (this.commitChapterError) throw this.commitChapterError;
    return { ...this.commitChapterResult, id: chapterId, body };
  }

  async storyStructure(): Promise<StoryStructureView> {
    return this.storyStructureResult;
  }

  async createOutlineNode(input: CreateOutlineNodeInput): Promise<OutlineNodeView> {
    this.createOutlineNodeCalls.push(input);
    const node: OutlineNodeView = { ...input, id: `outline-${this.storyStructureResult.outlineNodes.length + 1}`, position: this.storyStructureResult.outlineNodes.filter((item) => item.parentId === input.parentId).length + 1 };
    this.storyStructureResult = { ...this.storyStructureResult, outlineNodes: [...this.storyStructureResult.outlineNodes, node] };
    return node;
  }

  async createStoryLine(input: CreateStoryLineInput): Promise<StoryLineView> {
    this.createStoryLineCalls.push(input);
    const line = { ...input, id: `line-${this.storyStructureResult.storyLines.length + 1}`, status: "planned" };
    this.storyStructureResult = { ...this.storyStructureResult, storyLines: [...this.storyStructureResult.storyLines, line] };
    return line;
  }

  async linkStoryLine(input: LinkStoryLineInput): Promise<StoryLineLinkView> {
    this.linkStoryLineCalls.push(input);
    const link = { ...input, position: this.storyStructureResult.storyLineLinks.filter((item) => item.storyLineId === input.storyLineId).length + 1 };
    this.storyStructureResult = { ...this.storyStructureResult, storyLineLinks: [...this.storyStructureResult.storyLineLinks, link] };
    return link;
  }

  async createSceneCard(input: CreateSceneCardInput): Promise<SceneCardView> {
    this.createSceneCardCalls.push(input);
    const card = { ...input, id: `card-${this.storyStructureResult.sceneCards.length + 1}`, position: this.storyStructureResult.sceneCards.filter((item) => item.chapterId === input.chapterId).length + 1, status: "planned" };
    this.storyStructureResult = { ...this.storyStructureResult, sceneCards: [...this.storyStructureResult.sceneCards, card] };
    return card;
  }

  async archiveOutlineNode(id: string): Promise<void> {
    this.archiveOutlineNodeCalls.push(id);
    this.storyStructureResult = { ...this.storyStructureResult, outlineNodes: this.storyStructureResult.outlineNodes.filter((item) => item.id !== id) };
  }

  async moveOutlineNode(id: string, direction: "up" | "down"): Promise<void> {
    this.moveOutlineNodeCalls.push({ id, direction });
  }

  onBeforeClose(handler: () => Promise<void>): () => void {
    this.beforeCloseHandler = handler;
    return () => {
      if (this.beforeCloseHandler === handler) this.beforeCloseHandler = null;
    };
  }

  async completeClose(): Promise<void> {
    this.completeCloseCalls++;
  }

  async requestNativeClose(): Promise<void> {
    await this.beforeCloseHandler?.();
  }
}
