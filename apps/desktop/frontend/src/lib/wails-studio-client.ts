import { z } from "zod";
import {
  RevisionConflictError,
  StudioClientError,
  type ChapterView,
  type BranchComparisonView,
  type BranchMergePreviewView,
  type BranchSwitchView,
  type BranchView,
  type ChapterChangeInput,
  type ChapterRevisionView,
  type ChangeSetView,
  type ChangeOperationEditInput,
  type ChangeSetApplyView,
  type StructuredChangeInput,
  type AppearanceInput,
  type CharacterInput,
  type CharacterView,
  type CreateOutlineNodeInput,
  type CreateProjectInput,
  type CreateBranchInput,
  type CreateSceneCardInput,
  type CreateStoryLineInput,
  type FactionInput,
  type FactionView,
  type LinkStoryLineInput,
  type LocationInput,
  type LocationLinkInput,
  type LocationLinkView,
  type LocationTargetType,
  type LocationView,
  type MembershipInput,
  type MembershipView,
  type OutlineNodeView,
  type ProjectView,
  type RelationshipInput,
  type RelationshipView,
  type RegionAdjacencyInput,
  type RegionAdjacencyView,
  type SceneCardView,
  type SnapshotRestoreView,
  type SnapshotView,
  type StoryLineLinkView,
  type StoryLineView,
  type StoryStructureView,
  type StoryWorldView,
  type StudioClient,
  type ItemInput,
  type ItemTransferInput,
  type ItemTransferView,
  type ItemView,
  type WorldAssetsView,
  type WorldRegionInput,
  type WorldRegionView,
  type WorldRuleInput,
  type WorldRuleView,
  type ForeshadowingHookInput,
  type ForeshadowingHookView,
  type ContinuityView,
  type StoryEventInput,
  type StoryEventView,
  type StoryEventLinkInput,
  type StoryEventLinkView,
  type EventTargetType,
  type FactInput,
  type FactView,
  type FactEvidenceInput,
  type FactEvidenceView,
  type FactScopeInput,
  type FactScopeView,
  type FactConflictInput,
  type FactConflictView,
  type CharacterKnowledgeInput,
  type CharacterKnowledgeView,
  type ProviderConnectionInput,
  type ProviderConnectionView,
  type AgentProfileInput,
  type AgentProfileView,
  type AgentRuntimeSettingsInput,
  type AgentRuntimeSettingsView,
  type AgentRunView,
  type ImportApplyView,
  type ImportJobView,
  type ImportPrepareView,
  type ExtractionRunInput,
  type ExtractionRunView,
  type ExtractionStepView,
  type ExtractionTaskView,
  type EvidenceView,
  type JobEventView,
  type JobEventsPage,
  type JobSnapshotView,
  type JobSummaryView,
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
const changeSetStatusSchema = z.enum([
  "pending",
  "partially_applied",
  "applied",
  "rejected",
  "conflicted",
]);
const evidenceSchema = z.object({
  chapterId: z.string(),
  chapterTitle: z.string(),
  startLine: z.number().int().nonnegative(),
  endLine: z.number().int().nonnegative(),
  excerpt: z.string(),
});
const changeOperationSchema = z.object({
  id: z.string(),
  kind: z.enum([
    "replace_chapter_body",
    "update_outline_node",
    "update_character",
    "update_fact",
    "create_outline_node",
    "create_character",
    "create_relationship",
    "create_world_rule",
    "create_story_event",
    "create_fact",
    "create_foreshadowing_hook",
  ]),
  targetId: z.string(),
  baseRevision: z.number(),
  baseFingerprint: z.string(),
  beforeValue: z.string(),
  afterValue: z.string(),
  reason: z.string(),
  status: changeSetStatusSchema,
  evidence: z.array(evidenceSchema).optional(),
});
const changeSetSchema = z.object({
  id: z.string(),
  title: z.string(),
  summary: z.string(),
  createdBy: z.string(),
  status: changeSetStatusSchema,
  operations: z.array(changeOperationSchema),
  createdAt: z.string(),
  updatedAt: z.string(),
});
const changeSetApplySchema = z.object({
  changeSet: changeSetSchema,
  chapters: z.array(chapterSchema),
});
const revisionSourceSchema = z.enum(["initial", "author", "agent", "restore"]);
const chapterRevisionSchema = z.object({
  chapterId: z.string(),
  revision: z.number(),
  title: z.string(),
  body: z.string(),
  source: revisionSourceSchema,
  summary: z.string(),
  createdAt: z.string(),
  isCurrent: z.boolean(),
});
const snapshotSchema = z.object({
  id: z.string(),
  name: z.string(),
  kind: z.enum(["named", "recovery"]),
  createdAt: z.string(),
});
const snapshotRestoreSchema = z.object({
  snapshotId: z.string(),
  project: projectSchema,
  chapters: z.array(chapterSchema),
});
const branchSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string(),
  parentId: z.string(),
  active: z.boolean(),
  main: z.boolean(),
  generation: z.number(),
  createdAt: z.string(),
  updatedAt: z.string(),
});
const branchSwitchSchema = z.object({
  activeBranchId: z.string(),
  project: projectSchema,
  chapters: z.array(chapterSchema),
});
const branchComparisonSchema = z.object({
  source: branchSchema,
  target: branchSchema,
  addedChapters: z.number(),
  modifiedChapters: z.number(),
  removedChapters: z.number(),
  chapterChanges: z.array(
    z.object({
      chapterId: z.string(),
      title: z.string(),
      kind: z.enum(["added", "modified", "removed"]),
    }),
  ),
  domains: z.array(
    z.object({
      key: z.string(),
      label: z.string(),
      sourceCount: z.number(),
      targetCount: z.number(),
      changed: z.boolean(),
    }),
  ),
});
const branchMergePreviewSchema = z.object({
  token: z.string(),
  sourceId: z.string(),
  targetId: z.string(),
  comparison: branchComparisonSchema,
});

const providerConnectionSchema = z.object({
  id: z.string(),
  name: z.string(),
  kind: z.enum(["openai", "anthropic"]),
  baseUrl: z.string(),
  hasCredential: z.boolean(),
  updatedAt: z.string(),
});
const agentProfileSchema = z.object({
  id: z.string(),
  role: z.enum(["planner", "writer", "continuity_reviewer"]),
  connectionId: z.string(),
  model: z.string(),
  maxOutputTokens: z.number(),
  temperature: z.number(),
  enabled: z.boolean(),
  updatedAt: z.string(),
});
const agentRuntimeSettingsSchema = z.object({
  maxRunTokens: z.number(),
  maxContextChars: z.number(),
  updatedAt: z.string(),
});
const agentRunStepSchema = z.object({
  id: z.string(),
  role: z.enum(["planner", "writer", "continuity_reviewer"]),
  position: z.number(),
  status: z.enum(["queued", "running", "completed", "failed", "cancelled"]),
  output: z.string(),
  errorCode: z.string(),
  errorMessage: z.string(),
  providerRequestId: z.string(),
  inputTokens: z.number(),
  outputTokens: z.number(),
});
const agentRunSchema = z.object({
  id: z.string(),
  chapterId: z.string(),
  prompt: z.string(),
  status: z.enum([
    "queued",
    "running",
    "completed",
    "failed",
    "cancelled",
    "recoverable",
  ]),
  errorCode: z.string(),
  errorMessage: z.string(),
  changeSetId: z.string(),
  retryOf: z.string(),
  tokenBudget: z.number(),
  contextCharBudget: z.number(),
  inputTokens: z.number(),
  outputTokens: z.number(),
  contextChars: z.number(),
  contextTruncated: z.boolean(),
  steps: z.array(agentRunStepSchema),
  createdAt: z.string(),
  updatedAt: z.string(),
});

const extractionStepSchema = z.object({
  id: z.string(),
  importChapterId: z.string(),
  createdChapterId: z.string(),
  chapterTitle: z.string(),
  position: z.number().int().positive(),
  status: z.enum(["queued", "running", "completed", "failed", "cancelled"]),
  output: z.string(),
  errorCode: z.string(),
  errorMessage: z.string(),
  providerRequestId: z.string(),
  inputTokens: z.number().int().nonnegative(),
  outputTokens: z.number().int().nonnegative(),
});
const extractionTaskSchema = z.object({
  id: z.string(),
  importJobId: z.string(),
  connectionId: z.string(),
  model: z.string(),
  prompt: z.string(),
  status: z.enum([
    "queued",
    "running",
    "completed",
    "failed",
    "cancelled",
    "recoverable",
  ]),
  errorCode: z.string(),
  errorMessage: z.string(),
  changeSetId: z.string(),
  retryOf: z.string(),
  tokenBudget: z.number().int().nonnegative(),
  contextCharBudget: z.number().int().nonnegative(),
  inputTokens: z.number().int().nonnegative(),
  outputTokens: z.number().int().nonnegative(),
  steps: z.array(extractionStepSchema),
  createdAt: z.string(),
  updatedAt: z.string(),
});
const extractionRunSchema = z.object({
  task: extractionTaskSchema,
  changeSet: changeSetSchema.nullable(),
});

const jobEventSchema = z.object({
  jobId: z.string(),
  jobKind: z.enum(["agent", "extraction"]),
  projectId: z.string(),
  sequenceNumber: z.number().int().nonnegative(),
  eventType: z.enum([
    "job_queued",
    "job_started",
    "step_started",
    "step_delta",
    "step_completed",
    "step_failed",
    "job_completed",
    "job_failed",
    "job_cancelled",
    "job_recovered",
  ]),
  stepId: z.string().optional(),
  payload: z.string().optional(),
  createdAt: z.string(),
});
const jobSnapshotSchema = z.object({
  jobId: z.string(),
  jobKind: z.enum(["agent", "extraction"]),
  projectId: z.string(),
  status: z.string(),
  maxSequence: z.number().int().nonnegative(),
  earliestReplayableSequence: z.number().int().nonnegative(),
  inputTokens: z.number().int().nonnegative(),
  outputTokens: z.number().int().nonnegative(),
  pendingAction: z.string().optional(),
  terminalInfo: z.string().optional(),
  steps: z
    .array(
      z.object({
        stepId: z.string(),
        status: z.enum([
          "queued",
          "running",
          "started",
          "completed",
          "failed",
          "cancelled",
        ]),
      }),
    )
    .optional(),
  updatedAt: z.string(),
});
const jobEventsPageSchema = z.object({
  events: z.array(jobEventSchema),
  nextCursor: z.number().int().nonnegative(),
  hasMore: z.boolean(),
  snapshotRequired: z.boolean(),
});
const jobSummarySchema = z.object({
  jobId: z.string(),
  jobKind: z.enum(["agent", "extraction"]),
  projectId: z.string(),
  status: z.string(),
  maxSequence: z.number().int().nonnegative(),
  updatedAt: z.string(),
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

const worldRegionSchema = z.object({
  id: z.string(),
  parentId: z.string(),
  kind: z.enum([
    "world",
    "continent",
    "territory",
    "settlement",
    "terrain",
    "secret_realm",
    "resource",
  ]),
  name: z.string(),
  direction: z.enum([
    "north",
    "north_east",
    "east",
    "south_east",
    "south",
    "south_west",
    "west",
    "north_west",
    "center",
    "surrounding",
  ]),
  summary: z.string(),
  terrain: z.string(),
  climate: z.string(),
});
const locationSchema = z.object({
  id: z.string(),
  regionId: z.string(),
  kind: z.enum([
    "city",
    "settlement",
    "building",
    "landmark",
    "wilderness",
    "other",
  ]),
  name: z.string(),
  summary: z.string(),
  status: z.enum(["active", "inactive", "destroyed"]),
});
const locationLinkSchema = z.object({
  locationId: z.string(),
  targetType: z.enum(["chapter", "scene", "character", "faction"]),
  targetId: z.string(),
  role: z.string(),
});
const regionAdjacencySchema = z.object({
  firstRegionId: z.string(),
  secondRegionId: z.string(),
  travelNote: z.string(),
});
const itemSchema = z.object({
  id: z.string(),
  name: z.string(),
  summary: z.string(),
  status: z.enum(["active", "lost", "destroyed"]),
  holderType: z.union([
    z.enum(["character", "faction", "location"]),
    z.literal(""),
  ]),
  holderId: z.string(),
});
const itemTransferSchema = z.object({
  id: z.string(),
  itemId: z.string(),
  fromType: z.union([
    z.enum(["character", "faction", "location"]),
    z.literal(""),
  ]),
  fromId: z.string(),
  toType: z.enum(["character", "faction", "location"]),
  toId: z.string(),
  chapterId: z.string(),
  note: z.string(),
});
const worldRuleSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string(),
  scopeType: z.enum(["project", "region", "location"]),
  scopeId: z.string(),
  status: z.enum(["active", "inactive"]),
});
const foreshadowingHookSchema = z.object({
  id: z.string(),
  title: z.string(),
  setup: z.string(),
  plantedChapterId: z.string(),
  dueChapterId: z.string(),
  resolvedChapterId: z.string(),
  status: z.enum(["planted", "progressing", "resolved", "abandoned"]),
  notes: z.string(),
});
const consistencyIssueSchema = z.object({
  code: z.string(),
  severity: z.string(),
  message: z.string(),
  entityIds: z.array(z.string()),
});
const worldAssetsSchema = z.object({
  regions: z.array(worldRegionSchema),
  regionAdjacencies: z.array(regionAdjacencySchema).default([]),
  locations: z.array(locationSchema),
  locationLinks: z.array(locationLinkSchema),
  items: z.array(itemSchema),
  itemTransfers: z.array(itemTransferSchema),
  rules: z.array(worldRuleSchema),
  hooks: z.array(foreshadowingHookSchema),
  consistencyIssues: z.array(consistencyIssueSchema),
});
const storyEventSchema = z.object({
  id: z.string(),
  title: z.string(),
  summary: z.string(),
  cause: z.string(),
  result: z.string(),
  storyTime: z.string(),
  storyOrder: z.number(),
  narrativeOrder: z.number(),
  status: z.enum(["planned", "occurred", "uncertain"]),
});
const storyEventLinkSchema = z.object({
  eventId: z.string(),
  targetType: z.enum(["chapter", "character", "faction", "location", "item"]),
  targetId: z.string(),
  role: z.string(),
});
const factSchema = z.object({
  id: z.string(),
  text: z.string(),
  category: z.string(),
  origin: z.enum(["author", "text", "model", "rule"]),
  status: z.enum([
    "author_confirmed",
    "text_evidenced",
    "ai_inferred",
    "conflicted",
    "rejected",
  ]),
  confidence: z.number(),
  validFromEventId: z.string(),
  validUntilEventId: z.string(),
  createdBy: z.string(),
  confirmedBy: z.string(),
});
const factEvidenceSchema = z.object({
  id: z.string(),
  factId: z.string(),
  chapterId: z.string(),
  quote: z.string(),
  startOffset: z.number(),
  endOffset: z.number(),
  chapterRevision: z.number(),
});
const factScopeSchema = z.object({
  id: z.string(),
  factId: z.string(),
  targetType: z.enum([
    "character",
    "faction",
    "location",
    "item",
    "region",
    "rule",
    "event",
  ]),
  targetId: z.string(),
});
const factConflictSchema = z.object({
  id: z.string(),
  factId: z.string(),
  otherFactId: z.string(),
  reason: z.string(),
  status: z.enum(["open", "resolved"]),
});
const characterKnowledgeSchema = z.object({
  id: z.string(),
  characterId: z.string(),
  factId: z.string(),
  state: z.enum(["known", "believed", "misinformed"]),
  beliefText: z.string(),
  availableFromChapterId: z.string(),
  notes: z.string(),
});
const continuitySchema = z.object({
  events: z.array(storyEventSchema),
  eventLinks: z.array(storyEventLinkSchema),
  facts: z.array(factSchema),
  evidence: z.array(factEvidenceSchema),
  scopes: z.array(factScopeSchema),
  conflicts: z.array(factConflictSchema),
  consistencyIssues: z.array(consistencyIssueSchema),
});

const importBodyPreviewSchema = z.string().superRefine((value, context) => {
  let runeCount = 0;
  for (const _rune of value) {
    runeCount++;
    if (runeCount > 600) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        message: "body preview exceeds the boundary limit",
      });
      break;
    }
  }
});
const importChapterSchema = z
  .object({
    id: z.string().min(1),
    volumeTitle: z.string(),
    title: z.string().min(1),
    wordCount: z.number().int().nonnegative(),
    bodyPreview: importBodyPreviewSchema,
    position: z.number().int().positive(),
    sourceStartLine: z.number().int().positive(),
    sourceEndLine: z.number().int().positive(),
    contentHashSummary: z.string().regex(/^[a-f0-9]{12}$/),
    selected: z.boolean(),
    createdChapterId: z.string().optional(),
  })
  .strict()
  .superRefine((value, context) => {
    if (value.sourceEndLine < value.sourceStartLine) {
      context.addIssue({
        code: z.ZodIssueCode.custom,
        message: "source line range is reversed",
        path: ["sourceEndLine"],
      });
    }
  })
  .transform((value) => ({
    ...value,
    createdChapterId: value.createdChapterId ?? "",
  }));
const rawImportJobSchema = z
  .object({
    id: z.string().min(1),
    status: z.enum(["prepared", "applied", "failed"]),
    sourceName: z.string().min(1),
    sourceFormat: z.enum(["txt", "markdown"]),
    sourceEncoding: z.string().min(1),
    sourceHashDigest: z.string().regex(/^[a-f0-9]{12}$/),
    sourceBytes: z.number().int().nonnegative(),
    archivedRelativePath: z.string(),
    chapterCount: z.number().int().nonnegative(),
    selectedChapterCount: z.number().int().nonnegative(),
    extractionStatus: z.string().optional(),
    extractionRunId: z.string().optional(),
    errorCode: z.string().optional(),
    errorMessage: z.string().optional(),
    createdAt: z.string(),
    updatedAt: z.string(),
    appliedAt: z.string().optional(),
    chapters: z.array(importChapterSchema),
  })
  .strict();

type RawImportJob = z.infer<typeof rawImportJobSchema>;

function addImportInvariantIssue(
  context: z.RefinementCtx,
  path: Array<string | number>,
  message: string,
) {
  context.addIssue({
    code: z.ZodIssueCode.custom,
    message,
    path,
  });
}

function validateImportJob(
  value: RawImportJob,
  context: z.RefinementCtx,
  requireDetails: boolean,
) {
  if (
    value.sourceName === "." ||
    value.sourceName === ".." ||
    /[/\\]/u.test(value.sourceName)
  ) {
    addImportInvariantIssue(
      context,
      ["sourceName"],
      "source name must be a basename",
    );
  }

  const extractionStatus = value.extractionStatus ?? "";
  const extractionRunId = value.extractionRunId ?? "";
  if (
    extractionStatus !== "" &&
    extractionStatus !== "none" &&
    extractionStatus !== "running" &&
    extractionStatus !== "completed" &&
    extractionStatus !== "failed"
  ) {
    addImportInvariantIssue(
      context,
      ["extractionStatus"],
      "extraction status must be one of none, running, completed, or failed",
    );
  }
  const hasRun = extractionStatus !== "" && extractionStatus !== "none";
  if (hasRun && extractionRunId === "") {
    addImportInvariantIssue(
      context,
      ["extractionRunId"],
      "extraction run id is required when extraction status is active",
    );
  }
  if (!hasRun && extractionRunId !== "") {
    addImportInvariantIssue(
      context,
      ["extractionRunId"],
      "extraction run id must be empty when extraction status is none",
    );
  }
  if (value.status !== "applied" && hasRun) {
    addImportInvariantIssue(
      context,
      ["extractionStatus"],
      "only applied import jobs may have an extraction run",
    );
  }

  const candidateIDs = new Set<string>();
  const positions = new Set<number>();
  for (const [index, chapter] of value.chapters.entries()) {
    if (candidateIDs.has(chapter.id)) {
      addImportInvariantIssue(
        context,
        ["chapters", index, "id"],
        "candidate IDs must be unique",
      );
    }
    candidateIDs.add(chapter.id);
    if (positions.has(chapter.position)) {
      addImportInvariantIssue(
        context,
        ["chapters", index, "position"],
        "candidate positions must be unique",
      );
    }
    positions.add(chapter.position);
  }

  if (
    (requireDetails || value.chapters.length > 0) &&
    value.chapterCount !== value.chapters.length
  ) {
    addImportInvariantIssue(
      context,
      ["chapterCount"],
      "chapter count does not match candidates",
    );
  }

  const selected = value.chapters.filter((chapter) => chapter.selected);
  const hasAppliedAt = Boolean(value.appliedAt);
  const hasError = Boolean(value.errorCode) || Boolean(value.errorMessage);
  switch (value.status) {
    case "prepared":
      if (value.chapterCount < 1 || value.selectedChapterCount !== 0) {
        addImportInvariantIssue(
          context,
          ["selectedChapterCount"],
          "prepared task has an invalid selection count",
        );
      }
      if (hasAppliedAt || hasError) {
        addImportInvariantIssue(
          context,
          ["status"],
          "prepared task has terminal metadata",
        );
      }
      for (const [index, chapter] of value.chapters.entries()) {
        if (chapter.selected || chapter.createdChapterId) {
          addImportInvariantIssue(
            context,
            ["chapters", index],
            "prepared candidate has applied metadata",
          );
        }
      }
      break;
    case "applied":
      if (
        value.chapterCount < 1 ||
        value.selectedChapterCount < 1 ||
        value.selectedChapterCount > value.chapterCount ||
        !hasAppliedAt ||
        hasError
      ) {
        addImportInvariantIssue(
          context,
          ["status"],
          "applied task metadata is inconsistent",
        );
      }
      if (
        value.chapters.length > 0 &&
        selected.length !== value.selectedChapterCount
      ) {
        addImportInvariantIssue(
          context,
          ["selectedChapterCount"],
          "selected count does not match candidates",
        );
      }
      for (const [index, chapter] of value.chapters.entries()) {
        if (chapter.selected !== Boolean(chapter.createdChapterId)) {
          addImportInvariantIssue(
            context,
            ["chapters", index, "createdChapterId"],
            "created chapter mapping is inconsistent",
          );
        }
      }
      break;
    case "failed":
      if (
        value.chapterCount !== 0 ||
        value.selectedChapterCount !== 0 ||
        value.chapters.length !== 0 ||
        hasAppliedAt ||
        !value.errorCode ||
        !value.errorMessage
      ) {
        addImportInvariantIssue(
          context,
          ["status"],
          "failed task metadata is inconsistent",
        );
      }
      break;
  }
}

const importJobSchema = rawImportJobSchema
  .superRefine((value, context) => validateImportJob(value, context, false))
  .transform(({ archivedRelativePath: _archive, ...value }) => ({
    ...value,
    errorCode: value.errorCode ?? "",
    errorMessage: value.errorMessage ?? "",
    appliedAt: value.appliedAt ?? "",
    extractionStatus: value.extractionStatus ?? "none",
    extractionRunId: value.extractionRunId ?? "",
  }));
const importJobDetailSchema = rawImportJobSchema
  .superRefine((value, context) => validateImportJob(value, context, true))
  .transform(({ archivedRelativePath: _archive, ...value }) => ({
    ...value,
    errorCode: value.errorCode ?? "",
    errorMessage: value.errorMessage ?? "",
    appliedAt: value.appliedAt ?? "",
    extractionStatus: value.extractionStatus ?? "none",
    extractionRunId: value.extractionRunId ?? "",
  }));
const importPrepareSchema = z.object({ job: importJobDetailSchema }).strict();
const importApplySchema = z
  .object({
    job: importJobDetailSchema,
    chapters: z.array(chapterSchema),
  })
  .strict()
  .superRefine((value, context) => {
    if (value.chapters.length === 0) {
      addImportInvariantIssue(
        context,
        ["chapters"],
        "applied workspace chapters must not be empty",
      );
      return;
    }
    const chapterIDs = new Set<string>();
    const positions = new Set<number>();
    const projectID = value.chapters[0].projectId;
    for (const [index, chapter] of value.chapters.entries()) {
      if (
        !chapter.id ||
        !chapter.projectId ||
        !Number.isInteger(chapter.position) ||
        chapter.position < 1
      ) {
        addImportInvariantIssue(
          context,
          ["chapters", index],
          "workspace chapter identity is invalid",
        );
      }
      if (chapterIDs.has(chapter.id)) {
        addImportInvariantIssue(
          context,
          ["chapters", index, "id"],
          "workspace chapter IDs must be unique",
        );
      }
      chapterIDs.add(chapter.id);
      if (positions.has(chapter.position)) {
        addImportInvariantIssue(
          context,
          ["chapters", index, "position"],
          "workspace chapter positions must be unique",
        );
      }
      positions.add(chapter.position);
      if (chapter.projectId !== projectID) {
        addImportInvariantIssue(
          context,
          ["chapters", index, "projectId"],
          "workspace chapters must belong to one project",
        );
      }
    }
    for (const candidate of value.job.chapters) {
      if (candidate.selected && !chapterIDs.has(candidate.createdChapterId)) {
        addImportInvariantIssue(
          context,
          ["job", "chapters"],
          "created chapter mapping is missing from workspace",
        );
      }
    }
  });

type NativeDesktopAPI = {
  ChooseProjectParent(): Promise<string>;
  ChooseProjectToOpen(): Promise<string>;
  ChooseManuscriptToImport(): Promise<string>;
  PrepareManuscriptImport(sourcePath: string): Promise<unknown>;
  ImportJobs(): Promise<unknown>;
  ImportJob(id: string): Promise<unknown>;
  DeleteImportJob(id: string): Promise<void>;
  ApplyImportJob(id: string, selectedChapterIds: string[]): Promise<unknown>;
  ExtractionTasks(): Promise<unknown>;
  ExtractionTask(id: string): Promise<unknown>;
  StartExtractionRun(input: ExtractionRunInput): Promise<unknown>;
  ExecuteExtractionRun(id: string): Promise<unknown>;
  CancelExtractionRun(id: string): Promise<void>;
  RetryExtractionRun(id: string): Promise<unknown>;
  CreateProject(input: CreateProjectInput): Promise<unknown>;
  OpenProject(path: string): Promise<unknown>;
  ListChapters(): Promise<unknown>;
  CreateChapter(title: string): Promise<unknown>;
  OpenChapter(id: string): Promise<unknown>;
  SaveDraft(id: string, body: string, revision: number): Promise<void>;
  DiscardDraft(id: string, revision: number): Promise<void>;
  CommitChapter(id: string, body: string, revision: number): Promise<unknown>;
  ChangeSets(): Promise<unknown>;
  ProposeChapterChange(input: ChapterChangeInput): Promise<unknown>;
  ProposeStructuredChange(input: StructuredChangeInput): Promise<unknown>;
  EditChangeOperation(input: ChangeOperationEditInput): Promise<unknown>;
  ApplyChangeSet(id: string): Promise<unknown>;
  ApplyChangeOperation(
    changeSetId: string,
    operationId: string,
  ): Promise<unknown>;
  RejectChangeSet(id: string): Promise<void>;
  RejectChangeOperation(
    changeSetId: string,
    operationId: string,
  ): Promise<void>;
  ChapterHistory(id: string): Promise<unknown>;
  RestoreChapterRevision(
    id: string,
    targetRevision: number,
    expectedRevision: number,
  ): Promise<unknown>;
  Snapshots(): Promise<unknown>;
  CreateSnapshot(name: string): Promise<unknown>;
  RestoreSnapshot(id: string): Promise<unknown>;
  Branches(): Promise<unknown>;
  CreateBranch(input: CreateBranchInput): Promise<unknown>;
  SwitchBranch(id: string): Promise<unknown>;
  CompareBranches(sourceId: string, targetId: string): Promise<unknown>;
  PreviewBranchMerge(sourceId: string): Promise<unknown>;
  ConfirmBranchMerge(token: string): Promise<unknown>;
  DeleteBranch(id: string): Promise<void>;
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
  WorldAssets(): Promise<unknown>;
  CreateWorldRegion(input: WorldRegionInput): Promise<unknown>;
  UpdateWorldRegion(id: string, input: WorldRegionInput): Promise<unknown>;
  ArchiveWorldRegion(id: string): Promise<void>;
  UpsertRegionAdjacency(input: RegionAdjacencyInput): Promise<unknown>;
  DeleteRegionAdjacency(
    firstRegionId: string,
    secondRegionId: string,
  ): Promise<void>;
  CreateLocation(input: LocationInput): Promise<unknown>;
  UpdateLocation(id: string, input: LocationInput): Promise<unknown>;
  ArchiveLocation(id: string): Promise<void>;
  LinkLocation(input: LocationLinkInput): Promise<unknown>;
  UnlinkLocation(
    locationId: string,
    targetType: LocationTargetType,
    targetId: string,
  ): Promise<void>;
  CreateItem(input: ItemInput): Promise<unknown>;
  UpdateItem(id: string, input: ItemInput): Promise<unknown>;
  TransferItem(input: ItemTransferInput): Promise<unknown>;
  ArchiveItem(id: string): Promise<void>;
  CreateWorldRule(input: WorldRuleInput): Promise<unknown>;
  UpdateWorldRule(id: string, input: WorldRuleInput): Promise<unknown>;
  ArchiveWorldRule(id: string): Promise<void>;
  CreateForeshadowingHook(input: ForeshadowingHookInput): Promise<unknown>;
  UpdateForeshadowingHook(
    id: string,
    input: ForeshadowingHookInput,
  ): Promise<unknown>;
  ArchiveForeshadowingHook(id: string): Promise<void>;
  Continuity(): Promise<unknown>;
  CreateStoryEvent(input: StoryEventInput): Promise<unknown>;
  UpdateStoryEvent(id: string, input: StoryEventInput): Promise<unknown>;
  ArchiveStoryEvent(id: string): Promise<void>;
  LinkStoryEvent(input: StoryEventLinkInput): Promise<unknown>;
  UnlinkStoryEvent(
    eventId: string,
    targetType: EventTargetType,
    targetId: string,
  ): Promise<void>;
  CreateFact(input: FactInput): Promise<unknown>;
  UpdateFact(id: string, input: FactInput): Promise<unknown>;
  ConfirmFact(id: string): Promise<unknown>;
  ArchiveFact(id: string): Promise<void>;
  CharacterKnowledge(): Promise<unknown>;
  CreateCharacterKnowledge(input: CharacterKnowledgeInput): Promise<unknown>;
  ArchiveCharacterKnowledge(id: string): Promise<void>;
  AddFactEvidence(input: FactEvidenceInput): Promise<unknown>;
  RemoveFactEvidence(id: string): Promise<void>;
  AddFactScope(input: FactScopeInput): Promise<unknown>;
  RemoveFactScope(id: string): Promise<void>;
  CreateFactConflict(input: FactConflictInput): Promise<unknown>;
  ResolveFactConflict(id: string): Promise<unknown>;
  DeleteFactConflict(id: string): Promise<void>;
  ProviderConnections(): Promise<unknown>;
  SaveProviderConnection(input: ProviderConnectionInput): Promise<unknown>;
  DeleteProviderConnection(id: string): Promise<void>;
  TestProviderConnection(id: string): Promise<void>;
  AgentProfiles(): Promise<unknown>;
  SaveAgentProfile(input: AgentProfileInput): Promise<unknown>;
  AgentRuntimeSettings(): Promise<unknown>;
  SaveAgentRuntimeSettings(input: AgentRuntimeSettingsInput): Promise<unknown>;
  CreateChapterAgentRun(chapterId: string, prompt: string): Promise<unknown>;
  ExecuteAgentRun(id: string): Promise<unknown>;
  CancelAgentRun(id: string): Promise<void>;
  RetryAgentRun(id: string): Promise<unknown>;
  AgentRuns(): Promise<unknown>;
  AgentRun(id: string): Promise<unknown>;
  GetJobSnapshot(jobId: string): Promise<unknown>;
  GetJobEvents(
    jobId: string,
    afterSequence: number,
    limit: number,
  ): Promise<unknown>;
  ListRecoverableJobs(): Promise<unknown>;
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

const changeSetView = (value: unknown): ChangeSetView =>
  changeSetSchema.parse(value);
const changeSetViews = (value: unknown): ChangeSetView[] =>
  z.array(changeSetSchema).parse(value);
const changeSetApplyView = (value: unknown): ChangeSetApplyView =>
  changeSetApplySchema.parse(value);
const chapterRevisionViews = (value: unknown): ChapterRevisionView[] =>
  z.array(chapterRevisionSchema).parse(value);
const snapshotView = (value: unknown): SnapshotView =>
  snapshotSchema.parse(value);
const snapshotViews = (value: unknown): SnapshotView[] =>
  z.array(snapshotSchema).parse(value);
const snapshotRestoreView = (value: unknown): SnapshotRestoreView =>
  snapshotRestoreSchema.parse(value);
const branchView = (value: unknown): BranchView => branchSchema.parse(value);
const branchViews = (value: unknown): BranchView[] =>
  z.array(branchSchema).parse(value);
const branchSwitchView = (value: unknown): BranchSwitchView =>
  branchSwitchSchema.parse(value);
const branchComparisonView = (value: unknown): BranchComparisonView =>
  branchComparisonSchema.parse(value);
const branchMergePreviewView = (value: unknown): BranchMergePreviewView =>
  branchMergePreviewSchema.parse(value);
const providerConnectionView = (value: unknown): ProviderConnectionView =>
  providerConnectionSchema.parse(value);
const providerConnectionViews = (value: unknown): ProviderConnectionView[] =>
  z.array(providerConnectionSchema).parse(value);
const agentProfileView = (value: unknown): AgentProfileView =>
  agentProfileSchema.parse(value);
const agentProfileViews = (value: unknown): AgentProfileView[] =>
  z.array(agentProfileSchema).parse(value);
const agentRuntimeSettingsView = (value: unknown): AgentRuntimeSettingsView =>
  agentRuntimeSettingsSchema.parse(value);
const agentRunView = (value: unknown): AgentRunView =>
  agentRunSchema.parse(value);
const agentRunViews = (value: unknown): AgentRunView[] =>
  z.array(agentRunSchema).parse(value);
const extractionStepView = (value: unknown): ExtractionStepView =>
  extractionStepSchema.parse(value);
const extractionTaskView = (value: unknown): ExtractionTaskView =>
  extractionTaskSchema.parse(value);
const extractionTaskViews = (value: unknown): ExtractionTaskView[] =>
  z.array(extractionTaskSchema).parse(value);
const extractionRunView = (value: unknown): ExtractionRunView =>
  extractionRunSchema.parse(value);
const jobEventView = (value: unknown): JobEventView =>
  jobEventSchema.parse(value);
const jobEventsPageView = (value: unknown): JobEventsPage =>
  jobEventsPageSchema.parse(value);
const jobSnapshotView = (value: unknown): JobSnapshotView =>
  jobSnapshotSchema.parse(value);
const jobSummaryViews = (value: unknown): JobSummaryView[] =>
  z.array(jobSummarySchema).parse(value);

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
const worldAssetsView = (value: unknown): WorldAssetsView =>
  worldAssetsSchema.parse(value);
const worldRegionView = (value: unknown): WorldRegionView =>
  worldRegionSchema.parse(value);
const locationView = (value: unknown): LocationView =>
  locationSchema.parse(value);
const locationLinkView = (value: unknown): LocationLinkView =>
  locationLinkSchema.parse(value);
const regionAdjacencyView = (value: unknown): RegionAdjacencyView =>
  regionAdjacencySchema.parse(value);
const itemView = (value: unknown): ItemView => itemSchema.parse(value);
const itemTransferView = (value: unknown): ItemTransferView =>
  itemTransferSchema.parse(value);
const worldRuleView = (value: unknown): WorldRuleView =>
  worldRuleSchema.parse(value);
const foreshadowingHookView = (value: unknown): ForeshadowingHookView =>
  foreshadowingHookSchema.parse(value);
const continuityView = (value: unknown): ContinuityView =>
  continuitySchema.parse(value);
const storyEventView = (value: unknown): StoryEventView =>
  storyEventSchema.parse(value);
const storyEventLinkView = (value: unknown): StoryEventLinkView =>
  storyEventLinkSchema.parse(value);
const factView = (value: unknown): FactView => factSchema.parse(value);
const factEvidenceView = (value: unknown): FactEvidenceView =>
  factEvidenceSchema.parse(value);
const factScopeView = (value: unknown): FactScopeView =>
  factScopeSchema.parse(value);
const factConflictView = (value: unknown): FactConflictView =>
  factConflictSchema.parse(value);
const characterKnowledgeView = (value: unknown): CharacterKnowledgeView =>
  characterKnowledgeSchema.parse(value);
const characterKnowledgeViews = (value: unknown): CharacterKnowledgeView[] =>
  z.array(characterKnowledgeSchema).parse(value);
const importPrepareView = (value: unknown): ImportPrepareView =>
  importPrepareSchema.parse(value);
const importJobView = (value: unknown): ImportJobView =>
  importJobDetailSchema.parse(value);
const importJobViews = (value: unknown): ImportJobView[] =>
  z.array(importJobSchema).parse(value);
const importApplyView = (
  value: unknown,
  expectedJobID: string,
  expectedSelectedChapterIDs: string[],
): ImportApplyView => {
  const result = importApplySchema.parse(value);
  if (result.job.status !== "applied") {
    throw new Error("invalid apply response status");
  }
  if (result.job.id !== expectedJobID) {
    throw new Error("apply response job does not match request");
  }

  const expectedCandidates = new Set(expectedSelectedChapterIDs);
  if (expectedCandidates.size !== expectedSelectedChapterIDs.length) {
    throw new Error("apply request candidate IDs must be unique");
  }
  const selectedCandidates = result.job.chapters.filter(
    (candidate) => candidate.selected,
  );
  if (
    selectedCandidates.length !== expectedCandidates.size ||
    selectedCandidates.some(
      (candidate) => !expectedCandidates.has(candidate.id),
    )
  ) {
    throw new Error("apply response candidates do not match request");
  }

  const workspaceChapterIDs = new Set(
    result.chapters.map((chapter) => chapter.id),
  );
  const createdChapterIDs = new Set<string>();
  for (const candidate of selectedCandidates) {
    if (
      !candidate.createdChapterId ||
      createdChapterIDs.has(candidate.createdChapterId) ||
      !workspaceChapterIDs.has(candidate.createdChapterId)
    ) {
      throw new Error("apply response chapter mapping is invalid");
    }
    createdChapterIDs.add(candidate.createdChapterId);
  }
  return result;
};

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
  async chooseManuscriptToImport() {
    return (
      (await safeCall(() => nativeAPI().ChooseManuscriptToImport())) || null
    );
  },
  async prepareManuscriptImport(sourcePath) {
    return importPrepareView(
      await safeCall(() => nativeAPI().PrepareManuscriptImport(sourcePath)),
    );
  },
  async importJobs() {
    return importJobViews(await safeCall(() => nativeAPI().ImportJobs()));
  },
  async importJob(id) {
    return importJobView(await safeCall(() => nativeAPI().ImportJob(id)));
  },
  async deleteImportJob(id) {
    await safeCall(() => nativeAPI().DeleteImportJob(id));
  },
  async applyImportJob(id, selectedChapterIds) {
    return importApplyView(
      await safeCall(() => nativeAPI().ApplyImportJob(id, selectedChapterIds)),
      id,
      selectedChapterIds,
    );
  },
  async extractionTasks() {
    return extractionTaskViews(
      await safeCall(() => nativeAPI().ExtractionTasks()),
    );
  },
  async extractionTask(id) {
    return extractionRunView(
      await safeCall(() => nativeAPI().ExtractionTask(id)),
    );
  },
  async startExtractionRun(input) {
    return extractionRunView(
      await safeCall(() => nativeAPI().StartExtractionRun(input)),
    );
  },
  async executeExtractionRun(id) {
    return extractionRunView(
      await safeCall(() => nativeAPI().ExecuteExtractionRun(id)),
    );
  },
  async cancelExtractionRun(id) {
    await safeCall(() => nativeAPI().CancelExtractionRun(id));
  },
  async retryExtractionRun(id) {
    return extractionRunView(
      await safeCall(() => nativeAPI().RetryExtractionRun(id)),
    );
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
  async changeSets() {
    return changeSetViews(await safeCall(() => nativeAPI().ChangeSets()));
  },
  async proposeChapterChange(input) {
    return changeSetView(
      await safeCall(() => nativeAPI().ProposeChapterChange(input)),
    );
  },
  async proposeStructuredChange(input) {
    return changeSetView(
      await safeCall(() => nativeAPI().ProposeStructuredChange(input)),
    );
  },
  async editChangeOperation(input) {
    return changeSetView(
      await safeCall(() => nativeAPI().EditChangeOperation(input)),
    );
  },
  async applyChangeSet(id) {
    return changeSetApplyView(
      await safeCall(() => nativeAPI().ApplyChangeSet(id)),
    );
  },
  async applyChangeOperation(changeSetId, operationId) {
    return changeSetApplyView(
      await safeCall(() =>
        nativeAPI().ApplyChangeOperation(changeSetId, operationId),
      ),
    );
  },
  async rejectChangeSet(id) {
    await safeCall(() => nativeAPI().RejectChangeSet(id));
  },
  async rejectChangeOperation(changeSetId, operationId) {
    await safeCall(() =>
      nativeAPI().RejectChangeOperation(changeSetId, operationId),
    );
  },
  async chapterHistory(chapterId) {
    return chapterRevisionViews(
      await safeCall(() => nativeAPI().ChapterHistory(chapterId)),
    );
  },
  async restoreChapterRevision(chapterId, targetRevision, expectedRevision) {
    return chapterView(
      await safeCall(() =>
        nativeAPI().RestoreChapterRevision(
          chapterId,
          targetRevision,
          expectedRevision,
        ),
      ),
    );
  },
  async snapshots() {
    return snapshotViews(await safeCall(() => nativeAPI().Snapshots()));
  },
  async createSnapshot(name) {
    return snapshotView(await safeCall(() => nativeAPI().CreateSnapshot(name)));
  },
  async restoreSnapshot(id) {
    return snapshotRestoreView(
      await safeCall(() => nativeAPI().RestoreSnapshot(id)),
    );
  },
  async branches() {
    return branchViews(await safeCall(() => nativeAPI().Branches()));
  },
  async createBranch(input) {
    return branchView(await safeCall(() => nativeAPI().CreateBranch(input)));
  },
  async switchBranch(id) {
    return branchSwitchView(await safeCall(() => nativeAPI().SwitchBranch(id)));
  },
  async compareBranches(sourceId, targetId) {
    return branchComparisonView(
      await safeCall(() => nativeAPI().CompareBranches(sourceId, targetId)),
    );
  },
  async previewBranchMerge(sourceId) {
    return branchMergePreviewView(
      await safeCall(() => nativeAPI().PreviewBranchMerge(sourceId)),
    );
  },
  async confirmBranchMerge(token) {
    return branchSwitchView(
      await safeCall(() => nativeAPI().ConfirmBranchMerge(token)),
    );
  },
  async deleteBranch(id) {
    await safeCall(() => nativeAPI().DeleteBranch(id));
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
  async worldAssets() {
    return worldAssetsView(await safeCall(() => nativeAPI().WorldAssets()));
  },
  async createWorldRegion(input) {
    return worldRegionView(
      await safeCall(() => nativeAPI().CreateWorldRegion(input)),
    );
  },
  async updateWorldRegion(id, input) {
    return worldRegionView(
      await safeCall(() => nativeAPI().UpdateWorldRegion(id, input)),
    );
  },
  async archiveWorldRegion(id) {
    await safeCall(() => nativeAPI().ArchiveWorldRegion(id));
  },
  async upsertRegionAdjacency(input) {
    return regionAdjacencyView(
      await safeCall(() => nativeAPI().UpsertRegionAdjacency(input)),
    );
  },
  async deleteRegionAdjacency(firstRegionId, secondRegionId) {
    await safeCall(() =>
      nativeAPI().DeleteRegionAdjacency(firstRegionId, secondRegionId),
    );
  },
  async createLocation(input) {
    return locationView(
      await safeCall(() => nativeAPI().CreateLocation(input)),
    );
  },
  async updateLocation(id, input) {
    return locationView(
      await safeCall(() => nativeAPI().UpdateLocation(id, input)),
    );
  },
  async archiveLocation(id) {
    await safeCall(() => nativeAPI().ArchiveLocation(id));
  },
  async linkLocation(input) {
    return locationLinkView(
      await safeCall(() => nativeAPI().LinkLocation(input)),
    );
  },
  async unlinkLocation(locationId, targetType, targetId) {
    await safeCall(() =>
      nativeAPI().UnlinkLocation(locationId, targetType, targetId),
    );
  },
  async createItem(input) {
    return itemView(await safeCall(() => nativeAPI().CreateItem(input)));
  },
  async updateItem(id, input) {
    return itemView(await safeCall(() => nativeAPI().UpdateItem(id, input)));
  },
  async transferItem(input) {
    return itemTransferView(
      await safeCall(() => nativeAPI().TransferItem(input)),
    );
  },
  async archiveItem(id) {
    await safeCall(() => nativeAPI().ArchiveItem(id));
  },
  async createWorldRule(input) {
    return worldRuleView(
      await safeCall(() => nativeAPI().CreateWorldRule(input)),
    );
  },
  async updateWorldRule(id, input) {
    return worldRuleView(
      await safeCall(() => nativeAPI().UpdateWorldRule(id, input)),
    );
  },
  async archiveWorldRule(id) {
    await safeCall(() => nativeAPI().ArchiveWorldRule(id));
  },
  async createForeshadowingHook(input) {
    return foreshadowingHookView(
      await safeCall(() => nativeAPI().CreateForeshadowingHook(input)),
    );
  },
  async updateForeshadowingHook(id, input) {
    return foreshadowingHookView(
      await safeCall(() => nativeAPI().UpdateForeshadowingHook(id, input)),
    );
  },
  async archiveForeshadowingHook(id) {
    await safeCall(() => nativeAPI().ArchiveForeshadowingHook(id));
  },
  async continuity() {
    return continuityView(await safeCall(() => nativeAPI().Continuity()));
  },
  async createStoryEvent(input) {
    return storyEventView(
      await safeCall(() => nativeAPI().CreateStoryEvent(input)),
    );
  },
  async updateStoryEvent(id, input) {
    return storyEventView(
      await safeCall(() => nativeAPI().UpdateStoryEvent(id, input)),
    );
  },
  async archiveStoryEvent(id) {
    await safeCall(() => nativeAPI().ArchiveStoryEvent(id));
  },
  async linkStoryEvent(input) {
    return storyEventLinkView(
      await safeCall(() => nativeAPI().LinkStoryEvent(input)),
    );
  },
  async unlinkStoryEvent(eventId, targetType, targetId) {
    await safeCall(() =>
      nativeAPI().UnlinkStoryEvent(eventId, targetType, targetId),
    );
  },
  async createFact(input) {
    return factView(await safeCall(() => nativeAPI().CreateFact(input)));
  },
  async updateFact(id, input) {
    return factView(await safeCall(() => nativeAPI().UpdateFact(id, input)));
  },
  async confirmFact(id) {
    return factView(await safeCall(() => nativeAPI().ConfirmFact(id)));
  },
  async archiveFact(id) {
    await safeCall(() => nativeAPI().ArchiveFact(id));
  },
  async characterKnowledge() {
    return characterKnowledgeViews(
      await safeCall(() => nativeAPI().CharacterKnowledge()),
    );
  },
  async createCharacterKnowledge(input) {
    return characterKnowledgeView(
      await safeCall(() => nativeAPI().CreateCharacterKnowledge(input)),
    );
  },
  async archiveCharacterKnowledge(id) {
    await safeCall(() => nativeAPI().ArchiveCharacterKnowledge(id));
  },
  async addFactEvidence(input) {
    return factEvidenceView(
      await safeCall(() => nativeAPI().AddFactEvidence(input)),
    );
  },
  async removeFactEvidence(id) {
    await safeCall(() => nativeAPI().RemoveFactEvidence(id));
  },
  async addFactScope(input) {
    return factScopeView(await safeCall(() => nativeAPI().AddFactScope(input)));
  },
  async removeFactScope(id) {
    await safeCall(() => nativeAPI().RemoveFactScope(id));
  },
  async createFactConflict(input) {
    return factConflictView(
      await safeCall(() => nativeAPI().CreateFactConflict(input)),
    );
  },
  async resolveFactConflict(id) {
    return factConflictView(
      await safeCall(() => nativeAPI().ResolveFactConflict(id)),
    );
  },
  async deleteFactConflict(id) {
    await safeCall(() => nativeAPI().DeleteFactConflict(id));
  },
  async providerConnections() {
    return providerConnectionViews(
      await safeCall(() => nativeAPI().ProviderConnections()),
    );
  },
  async saveProviderConnection(input) {
    return providerConnectionView(
      await safeCall(() => nativeAPI().SaveProviderConnection(input)),
    );
  },
  async deleteProviderConnection(id) {
    await safeCall(() => nativeAPI().DeleteProviderConnection(id));
  },
  async testProviderConnection(id) {
    await safeCall(() => nativeAPI().TestProviderConnection(id));
  },
  async agentProfiles() {
    return agentProfileViews(await safeCall(() => nativeAPI().AgentProfiles()));
  },
  async saveAgentProfile(input) {
    return agentProfileView(
      await safeCall(() => nativeAPI().SaveAgentProfile(input)),
    );
  },
  async agentRuntimeSettings() {
    return agentRuntimeSettingsView(
      await safeCall(() => nativeAPI().AgentRuntimeSettings()),
    );
  },
  async saveAgentRuntimeSettings(input) {
    return agentRuntimeSettingsView(
      await safeCall(() => nativeAPI().SaveAgentRuntimeSettings(input)),
    );
  },
  async createChapterAgentRun(chapterId, prompt) {
    return agentRunView(
      await safeCall(() =>
        nativeAPI().CreateChapterAgentRun(chapterId, prompt),
      ),
    );
  },
  async executeAgentRun(id) {
    return agentRunView(await safeCall(() => nativeAPI().ExecuteAgentRun(id)));
  },
  async cancelAgentRun(id) {
    await safeCall(() => nativeAPI().CancelAgentRun(id));
  },
  async retryAgentRun(id) {
    return agentRunView(await safeCall(() => nativeAPI().RetryAgentRun(id)));
  },
  async agentRuns() {
    return agentRunViews(await safeCall(() => nativeAPI().AgentRuns()));
  },
  async agentRun(id) {
    return agentRunView(await safeCall(() => nativeAPI().AgentRun(id)));
  },
  async getJobSnapshot(jobId) {
    return jobSnapshotView(
      await safeCall(() => nativeAPI().GetJobSnapshot(jobId)),
    );
  },
  async getJobEvents(jobId, afterSequence, limit) {
    return jobEventsPageView(
      await safeCall(() =>
        nativeAPI().GetJobEvents(jobId, afterSequence, limit),
      ),
    );
  },
  async listRecoverableJobs() {
    return jobSummaryViews(
      await safeCall(() => nativeAPI().ListRecoverableJobs()),
    );
  },
  onJobUpdated(handler) {
    if (!window.runtime) return () => undefined;
    return window.runtime.EventsOnMultiple(
      "job:updated",
      (payload: unknown) => {
        if (typeof payload === "object" && payload !== null) {
          const p = payload as {
            jobId?: string;
            jobKind?: string;
            latestSequence?: number;
          };
          if (
            typeof p.jobId === "string" &&
            typeof p.jobKind === "string" &&
            typeof p.latestSequence === "number"
          ) {
            handler(p.jobId, p.jobKind, p.latestSequence);
          }
        }
      },
      -1,
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
