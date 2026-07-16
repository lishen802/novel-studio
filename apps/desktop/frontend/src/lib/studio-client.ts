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
  recoveryBody?: string;
  hasRecoveryDraft: boolean;
};

export type CreateProjectInput = {
  parentDirectory: string;
  title: string;
  genre: string;
  summary: string;
};

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
  commitChapter(
    chapterId: string,
    body: string,
    baseRevision: number,
  ): Promise<ChapterView>;
}

export class RevisionConflictError extends Error {
  constructor() {
    super("章节版本冲突");
    this.name = "RevisionConflictError";
  }
}
