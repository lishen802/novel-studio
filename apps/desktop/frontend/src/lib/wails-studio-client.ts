import { z } from "zod";
import {
  RevisionConflictError,
  StudioClientError,
  type ChapterView,
  type AppearanceInput,
  type CharacterInput,
  type CharacterView,
  type CreateOutlineNodeInput,
  type CreateProjectInput,
  type CreateSceneCardInput,
  type CreateStoryLineInput,
  type FactionInput,
  type FactionView,
  type LinkStoryLineInput,
  type MembershipInput,
  type MembershipView,
  type OutlineNodeView,
  type ProjectView,
  type RelationshipInput,
  type RelationshipView,
  type SceneCardView,
  type StoryLineLinkView,
  type StoryLineView,
  type StoryStructureView,
  type StoryWorldView,
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

const outlineNodeSchema = z.object({
  id: z.string(),
  parentId: z.string(),
  level: z.enum(["book", "volume", "arc", "chapter", "scene"]),
  title: z.string(),
  summary: z.string(),
  goal: z.string(),
  conflict: z.string(),
  outcome: z.string(),
  position: z.number(),
  chapterId: z.string(),
});
const storyLineSchema = z.object({
  id: z.string(),
  name: z.string(),
  summary: z.string(),
  status: z.string(),
});
const storyLineLinkSchema = z.object({
  storyLineId: z.string(),
  outlineNodeId: z.string(),
  position: z.number(),
});
const sceneCardSchema = z.object({
  id: z.string(),
  chapterId: z.string(),
  outlineNodeId: z.string(),
  title: z.string(),
  pov: z.string(),
  location: z.string(),
  goal: z.string(),
  conflict: z.string(),
  outcome: z.string(),
  position: z.number(),
  status: z.string(),
});
const storyStructureSchema = z.object({
  outlineNodes: z.array(outlineNodeSchema),
  storyLines: z.array(storyLineSchema),
  storyLineLinks: z.array(storyLineLinkSchema),
  sceneCards: z.array(sceneCardSchema),
});

const characterSchema = z.object({
  id: z.string(),
  name: z.string(),
  aliases: z.array(z.string()),
  role: z.enum(["protagonist", "supporting", "antagonist", "minor"]),
  summary: z.string(),
  motivation: z.string(),
  fear: z.string(),
  secret: z.string(),
  arc: z.string(),
  status: z.enum(["active", "inactive", "deceased"]),
});
const factionSchema = z.object({
  id: z.string(),
  name: z.string(),
  summary: z.string(),
  goal: z.string(),
  status: z.enum(["active", "inactive", "disbanded"]),
});
const membershipSchema = z.object({
  characterId: z.string(),
  factionId: z.string(),
  role: z.string(),
  status: z.enum(["active", "former", "secret"]),
});
const relationshipSchema = z.object({
  id: z.string(),
  sourceCharacterId: z.string(),
  targetCharacterId: z.string(),
  kind: z.enum([
    "ally",
    "family",
    "romantic",
    "rival",
    "enemy",
    "mentor",
    "other",
  ]),
  label: z.string(),
  strength: z.number(),
  publicState: z.string(),
  trueState: z.string(),
});
const appearanceRoleSchema = z.enum(["pov", "present", "mentioned"]);
const storyWorldSchema = z.object({
  characters: z.array(characterSchema),
  factions: z.array(factionSchema),
  memberships: z.array(membershipSchema),
  relationships: z.array(relationshipSchema),
  chapterAppearances: z.array(
    z.object({
      characterId: z.string(),
      chapterId: z.string(),
      role: appearanceRoleSchema,
    }),
  ),
  sceneAppearances: z.array(
    z.object({
      characterId: z.string(),
      sceneCardId: z.string(),
      role: appearanceRoleSchema,
    }),
  ),
  consistencyIssues: z.array(
    z.object({
      code: z.string(),
      severity: z.string(),
      message: z.string(),
      entityIds: z.array(z.string()),
    }),
  ),
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
  StoryStructure(): Promise<unknown>;
  CreateOutlineNode(input: CreateOutlineNodeInput): Promise<unknown>;
  CreateStoryLine(input: CreateStoryLineInput): Promise<unknown>;
  LinkStoryLine(input: LinkStoryLineInput): Promise<unknown>;
  CreateSceneCard(input: CreateSceneCardInput): Promise<unknown>;
  ArchiveOutlineNode(id: string): Promise<void>;
  MoveOutlineNode(id: string, direction: string): Promise<void>;
  StoryWorld(): Promise<unknown>;
  CreateCharacter(input: CharacterInput): Promise<unknown>;
  UpdateCharacter(id: string, input: CharacterInput): Promise<unknown>;
  ArchiveCharacter(id: string): Promise<void>;
  CreateFaction(input: FactionInput): Promise<unknown>;
  UpdateFaction(id: string, input: FactionInput): Promise<unknown>;
  ArchiveFaction(id: string): Promise<void>;
  UpsertMembership(input: MembershipInput): Promise<unknown>;
  DeleteMembership(characterId: string, factionId: string): Promise<void>;
  CreateRelationship(input: RelationshipInput): Promise<unknown>;
  ArchiveRelationship(id: string): Promise<void>;
  LinkCharacterToChapter(input: AppearanceInput): Promise<void>;
  UnlinkCharacterFromChapter(
    characterId: string,
    chapterId: string,
  ): Promise<void>;
  LinkCharacterToScene(input: AppearanceInput): Promise<void>;
  UnlinkCharacterFromScene(
    characterId: string,
    sceneCardId: string,
  ): Promise<void>;
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

const storyStructureView = (value: unknown): StoryStructureView =>
  storyStructureSchema.parse(value);
const outlineNodeView = (value: unknown): OutlineNodeView =>
  outlineNodeSchema.parse(value);
const storyLineView = (value: unknown): StoryLineView =>
  storyLineSchema.parse(value);
const storyLineLinkView = (value: unknown): StoryLineLinkView =>
  storyLineLinkSchema.parse(value);
const sceneCardView = (value: unknown): SceneCardView =>
  sceneCardSchema.parse(value);
const storyWorldView = (value: unknown): StoryWorldView =>
  storyWorldSchema.parse(value);
const characterView = (value: unknown): CharacterView =>
  characterSchema.parse(value);
const factionView = (value: unknown): FactionView => factionSchema.parse(value);
const membershipView = (value: unknown): MembershipView =>
  membershipSchema.parse(value);
const relationshipView = (value: unknown): RelationshipView =>
  relationshipSchema.parse(value);

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
  async storyStructure() {
    return storyStructureView(
      await safeCall(() => nativeAPI().StoryStructure()),
    );
  },
  async createOutlineNode(input) {
    return outlineNodeView(
      await safeCall(() => nativeAPI().CreateOutlineNode(input)),
    );
  },
  async createStoryLine(input) {
    return storyLineView(
      await safeCall(() => nativeAPI().CreateStoryLine(input)),
    );
  },
  async linkStoryLine(input) {
    return storyLineLinkView(
      await safeCall(() => nativeAPI().LinkStoryLine(input)),
    );
  },
  async createSceneCard(input) {
    return sceneCardView(
      await safeCall(() => nativeAPI().CreateSceneCard(input)),
    );
  },
  async archiveOutlineNode(id) {
    await safeCall(() => nativeAPI().ArchiveOutlineNode(id));
  },
  async moveOutlineNode(id, direction) {
    await safeCall(() => nativeAPI().MoveOutlineNode(id, direction));
  },
  async storyWorld() {
    return storyWorldView(await safeCall(() => nativeAPI().StoryWorld()));
  },
  async createCharacter(input) {
    return characterView(
      await safeCall(() => nativeAPI().CreateCharacter(input)),
    );
  },
  async updateCharacter(id, input) {
    return characterView(
      await safeCall(() => nativeAPI().UpdateCharacter(id, input)),
    );
  },
  async archiveCharacter(id) {
    await safeCall(() => nativeAPI().ArchiveCharacter(id));
  },
  async createFaction(input) {
    return factionView(await safeCall(() => nativeAPI().CreateFaction(input)));
  },
  async updateFaction(id, input) {
    return factionView(
      await safeCall(() => nativeAPI().UpdateFaction(id, input)),
    );
  },
  async archiveFaction(id) {
    await safeCall(() => nativeAPI().ArchiveFaction(id));
  },
  async upsertMembership(input) {
    return membershipView(
      await safeCall(() => nativeAPI().UpsertMembership(input)),
    );
  },
  async deleteMembership(characterId, factionId) {
    await safeCall(() => nativeAPI().DeleteMembership(characterId, factionId));
  },
  async createRelationship(input) {
    return relationshipView(
      await safeCall(() => nativeAPI().CreateRelationship(input)),
    );
  },
  async archiveRelationship(id) {
    await safeCall(() => nativeAPI().ArchiveRelationship(id));
  },
  async linkCharacterToChapter(input) {
    await safeCall(() => nativeAPI().LinkCharacterToChapter(input));
  },
  async unlinkCharacterFromChapter(characterId, chapterId) {
    await safeCall(() =>
      nativeAPI().UnlinkCharacterFromChapter(characterId, chapterId),
    );
  },
  async linkCharacterToScene(input) {
    await safeCall(() => nativeAPI().LinkCharacterToScene(input));
  },
  async unlinkCharacterFromScene(characterId, sceneCardId) {
    await safeCall(() =>
      nativeAPI().UnlinkCharacterFromScene(characterId, sceneCardId),
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
