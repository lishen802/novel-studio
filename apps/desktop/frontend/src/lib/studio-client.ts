export type ProjectView = {
  id: string;
  title: string;
  genre: string;
  summary: string;
  path: string;
};

export type ChapterView = {
  id: string;
  projectId: string;
  title: string;
  position: number;
  body: string;
  revision: number;
  updatedAt: string;
  recoveryBody?: string;
  recoveryUpdatedAt?: string;
  hasRecoveryDraft: boolean;
};

export type CreateProjectInput = {
  parentDirectory: string;
  title: string;
  genre: string;
  summary: string;
};

export type OutlineLevel = "book" | "volume" | "arc" | "chapter" | "scene";

export type OutlineNodeView = {
  id: string; parentId: string; level: OutlineLevel; title: string;
  summary: string; goal: string; conflict: string; outcome: string;
  position: number; chapterId: string;
};

export type StoryLineView = { id: string; name: string; summary: string; status: string };
export type StoryLineLinkView = { storyLineId: string; outlineNodeId: string; position: number };
export type SceneCardView = {
  id: string; chapterId: string; outlineNodeId: string; title: string;
  pov: string; location: string; goal: string; conflict: string; outcome: string;
  position: number; status: string;
};
export type StoryStructureView = {
  outlineNodes: OutlineNodeView[];
  storyLines: StoryLineView[];
  storyLineLinks: StoryLineLinkView[];
  sceneCards: SceneCardView[];
};
export type CreateOutlineNodeInput = Omit<OutlineNodeView, "id" | "position">;
export type CreateStoryLineInput = { name: string; summary: string };
export type LinkStoryLineInput = { storyLineId: string; outlineNodeId: string };
export type CreateSceneCardInput = Omit<SceneCardView, "id" | "position" | "status">;

export interface StudioClient {
  chooseProjectParent(): Promise<string | null>;
  chooseProjectToOpen(): Promise<string | null>;
  createProject(input: CreateProjectInput): Promise<ProjectView>;
  openProject(path: string): Promise<ProjectView>;
  listChapters(): Promise<ChapterView[]>;
  createChapter(title: string): Promise<ChapterView>;
  openChapter(chapterId: string): Promise<ChapterView>;
  saveDraft(
    chapterId: string,
    body: string,
    baseRevision: number,
  ): Promise<void>;
  discardDraft(chapterId: string, baseRevision: number): Promise<void>;
  commitChapter(
    chapterId: string,
    body: string,
    baseRevision: number,
  ): Promise<ChapterView>;
  storyStructure(): Promise<StoryStructureView>;
  createOutlineNode(input: CreateOutlineNodeInput): Promise<OutlineNodeView>;
  createStoryLine(input: CreateStoryLineInput): Promise<StoryLineView>;
  linkStoryLine(input: LinkStoryLineInput): Promise<StoryLineLinkView>;
  createSceneCard(input: CreateSceneCardInput): Promise<SceneCardView>;
  archiveOutlineNode(id: string): Promise<void>;
  moveOutlineNode(id: string, direction: "up" | "down"): Promise<void>;
  onBeforeClose(handler: () => Promise<void>): () => void;
  completeClose(): Promise<void>;
}

export class RevisionConflictError extends Error {
  constructor() {
    super("章节版本冲突");
    this.name = "RevisionConflictError";
  }
}

export class StudioClientError extends Error {
  constructor(
    public readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = "StudioClientError";
  }
}
