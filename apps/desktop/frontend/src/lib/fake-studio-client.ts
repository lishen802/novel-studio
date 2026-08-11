import { RevisionConflictError } from "./studio-client";
import type {
  AppearanceInput,
  BranchComparisonView,
  BranchMergePreviewView,
  BranchSwitchView,
  BranchView,
  ChapterView,
  ChapterChangeInput,
  ChangeOperationEditInput,
  ChangeOperationView,
  ChangeSetApplyView,
  ChangeSetStatus,
  ChapterRevisionView,
  ChangeSetView,
  CharacterInput,
  CharacterView,
  CreateProjectInput,
  CreateBranchInput,
  CreateOutlineNodeInput,
  CreateSceneCardInput,
  CreateStoryLineInput,
  FactionInput,
  FactionView,
  ForeshadowingHookInput,
  ForeshadowingHookView,
  ItemInput,
  ItemTransferInput,
  ItemTransferView,
  ItemView,
  LinkStoryLineInput,
  LocationInput,
  LocationLinkInput,
  LocationLinkView,
  LocationTargetType,
  LocationView,
  MembershipInput,
  MembershipView,
  OutlineNodeView,
  ProjectView,
  ProjectAccessMode,
  ProjectOpenResultView,
  RecoveryCopyInput,
  RecoveryCopyResultView,
  RelationshipInput,
  RelationshipView,
  RegionAdjacencyInput,
  RegionAdjacencyView,
  SceneCardView,
  SnapshotRestoreView,
  SnapshotView,
  StoryLineLinkView,
  StoryLineView,
  StoryStructureView,
  StoryWorldView,
  StudioClient,
  StructuredChangeInput,
  WorldAssetsView,
  WorldRegionInput,
  WorldRegionView,
  WorldRuleInput,
  WorldRuleView,
  ContinuityView,
  StoryEventInput,
  StoryEventView,
  StoryEventLinkInput,
  StoryEventLinkView,
  EventTargetType,
  FactInput,
  FactView,
  FactEvidenceInput,
  FactEvidenceView,
  FactScopeInput,
  FactScopeView,
  FactConflictInput,
  FactConflictView,
  CharacterKnowledgeInput,
  CharacterKnowledgeView,
  ProviderConnectionInput,
  ProviderConnectionView,
  AgentProfileInput,
  AgentProfileView,
  AgentRuntimeSettingsInput,
  AgentRuntimeSettingsView,
  AgentRunView,
  ImportApplyView,
  ImportJobView,
  ImportPrepareView,
  ExtractionRunInput,
  ExtractionRunView,
  ExtractionStepView,
  ExtractionTaskView,
  EvidenceView,
  JobEventView,
  JobEventsPage,
  JobSnapshotView,
  JobSummaryView,
} from "./studio-client";

type ChapterWriteCall = {
  chapterId: string;
  body: string;
  baseRevision: number;
};

function deriveFakeChangeSetStatus(
  operations: ChangeOperationView[],
): ChangeSetStatus {
  if (operations.some((item) => item.status === "conflicted")) {
    return "conflicted";
  }
  if (operations.some((item) => item.status === "pending")) {
    return operations.some((item) => item.status === "applied")
      ? "partially_applied"
      : "pending";
  }
  return operations.some((item) => item.status === "applied")
    ? "applied"
    : "rejected";
}

export class FakeStudioClient implements StudioClient {
  projectParent: string | null = null;
  projectToOpen: string | null = null;
  manuscriptToImport: string | null = null;
  createProjectResult: ProjectView = {
    id: "project-1",
    title: "长夜织境",
    genre: "东方奇幻",
    summary: "",
    path: "/books/长夜织境.zjproject",
    accessMode: "editable",
    filesystemHealth: { level: "normal", category: "", actions: [] },
  };
  openProjectResult: ProjectView = this.createProjectResult;
  projectOpenResponse: ProjectOpenResultView | null = null;
  recoveryCopyResponse: RecoveryCopyResultView | null = null;
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
  changeSetsResult: ChangeSetView[] = [];
  chapterHistoryResult: ChapterRevisionView[] = [];
  snapshotsResult: SnapshotView[] = [];
  snapshotRestoreResult: SnapshotRestoreView = {
    snapshotId: "snapshot-1",
    project: this.createProjectResult,
    chapters: [],
  };
  branchesResult: BranchView[] = [
    {
      id: "main",
      name: "主分支",
      description: "",
      parentId: "",
      active: true,
      main: true,
      generation: 1,
      createdAt: "2026-07-28T05:00:00Z",
      updatedAt: "2026-07-28T05:00:00Z",
    },
  ];
  storyStructureResult: StoryStructureView = {
    outlineNodes: [],
    storyLines: [],
    storyLineLinks: [],
    sceneCards: [],
  };
  storyWorldResult: StoryWorldView = {
    characters: [],
    factions: [],
    memberships: [],
    relationships: [],
    chapterAppearances: [],
    sceneAppearances: [],
    consistencyIssues: [],
  };
  worldAssetsResult: WorldAssetsView = {
    regions: [],
    regionAdjacencies: [],
    locations: [],
    locationLinks: [],
    items: [],
    itemTransfers: [],
    rules: [],
    hooks: [],
    consistencyIssues: [],
  };
  continuityResult: ContinuityView = {
    events: [],
    eventLinks: [],
    facts: [],
    evidence: [],
    scopes: [],
    conflicts: [],
    consistencyIssues: [],
  };
  characterKnowledgeResult: CharacterKnowledgeView[] = [];
  providerConnectionsResult: ProviderConnectionView[] = [];
  agentProfilesResult: AgentProfileView[] = [];
  agentRuntimeSettingsResult: AgentRuntimeSettingsView = {
    maxRunTokens: 30000,
    maxContextChars: 120000,
    updatedAt: "2026-07-30T10:00:00Z",
  };
  agentRunsResult: AgentRunView[] = [];
  importJobsResult: ImportJobView[] = [];
  prepareManuscriptImportResult: ImportPrepareView | null = null;
  importApplyResult: ImportApplyView | null = null;
  extractionTasksResult: ExtractionTaskView[] = [];
  extractionRunResult: ExtractionRunView | null = null;
  extractionStartResult: ExtractionRunView | null = null;
  extractionExecuteResult: ExtractionRunView | null = null;
  extractionRetryResult: ExtractionRunView | null = null;

  createProjectError: Error | null = null;
  openProjectError: Error | null = null;
  createChapterError: Error | null = null;
  openChapterError: Error | null = null;
  saveDraftError: Error | null = null;
  discardDraftError: Error | null = null;
  commitChapterError: Error | null = null;
  storyWorldError: Error | null = null;
  worldMutationError: Error | null = null;
  worldAssetMutationError: Error | null = null;
  importJobsError: Error | null = null;
  prepareManuscriptImportError: Error | null = null;
  importMutationError: Error | null = null;
  extractionError: Error | null = null;

  createProjectCalls: CreateProjectInput[] = [];
  openProjectCalls: string[] = [];
  openProjectInputs: Array<{ path: string; mode: ProjectAccessMode }> = [];
  createProjectRecoveryCopyCalls: RecoveryCopyInput[] = [];
  createChapterCalls: string[] = [];
  openChapterCalls: string[] = [];
  saveDraftCalls: ChapterWriteCall[] = [];
  discardDraftCalls: Array<{ chapterId: string; baseRevision: number }> = [];
  commitChapterCalls: ChapterWriteCall[] = [];
  proposeChapterChangeCalls: ChapterChangeInput[] = [];
  proposeStructuredChangeCalls: StructuredChangeInput[] = [];
  editChangeOperationCalls: ChangeOperationEditInput[] = [];
  applyChangeSetCalls: string[] = [];
  applyChangeOperationCalls: Array<{
    changeSetId: string;
    operationId: string;
  }> = [];
  rejectChangeSetCalls: string[] = [];
  rejectChangeOperationCalls: Array<{
    changeSetId: string;
    operationId: string;
  }> = [];
  restoreChapterRevisionCalls: Array<{
    chapterId: string;
    targetRevision: number;
    expectedRevision: number;
  }> = [];
  createSnapshotCalls: string[] = [];
  restoreSnapshotCalls: string[] = [];
  createBranchCalls: CreateBranchInput[] = [];
  switchBranchCalls: string[] = [];
  compareBranchCalls: Array<{ sourceId: string; targetId: string }> = [];
  previewBranchMergeCalls: string[] = [];
  confirmBranchMergeCalls: string[] = [];
  deleteBranchCalls: string[] = [];
  createOutlineNodeCalls: CreateOutlineNodeInput[] = [];
  createStoryLineCalls: CreateStoryLineInput[] = [];
  linkStoryLineCalls: LinkStoryLineInput[] = [];
  createSceneCardCalls: CreateSceneCardInput[] = [];
  archiveOutlineNodeCalls: string[] = [];
  moveOutlineNodeCalls: Array<{ id: string; direction: "up" | "down" }> = [];
  createCharacterCalls: CharacterInput[] = [];
  updateCharacterCalls: Array<{ id: string; input: CharacterInput }> = [];
  archiveCharacterCalls: string[] = [];
  createFactionCalls: FactionInput[] = [];
  updateFactionCalls: Array<{ id: string; input: FactionInput }> = [];
  archiveFactionCalls: string[] = [];
  upsertMembershipCalls: MembershipInput[] = [];
  deleteMembershipCalls: Array<{ characterId: string; factionId: string }> = [];
  createRelationshipCalls: RelationshipInput[] = [];
  archiveRelationshipCalls: string[] = [];
  linkChapterCalls: AppearanceInput[] = [];
  unlinkChapterCalls: Array<{ characterId: string; chapterId: string }> = [];
  linkSceneCalls: AppearanceInput[] = [];
  unlinkSceneCalls: Array<{ characterId: string; sceneCardId: string }> = [];
  createWorldRegionCalls: WorldRegionInput[] = [];
  updateWorldRegionCalls: Array<{ id: string; input: WorldRegionInput }> = [];
  archiveWorldRegionCalls: string[] = [];
  upsertRegionAdjacencyCalls: RegionAdjacencyInput[] = [];
  deleteRegionAdjacencyCalls: Array<{
    firstRegionId: string;
    secondRegionId: string;
  }> = [];
  createLocationCalls: LocationInput[] = [];
  updateLocationCalls: Array<{ id: string; input: LocationInput }> = [];
  archiveLocationCalls: string[] = [];
  linkLocationCalls: LocationLinkInput[] = [];
  unlinkLocationCalls: Array<{
    locationId: string;
    targetType: LocationTargetType;
    targetId: string;
  }> = [];
  createItemCalls: ItemInput[] = [];
  updateItemCalls: Array<{ id: string; input: ItemInput }> = [];
  transferItemCalls: ItemTransferInput[] = [];
  archiveItemCalls: string[] = [];
  createWorldRuleCalls: WorldRuleInput[] = [];
  updateWorldRuleCalls: Array<{ id: string; input: WorldRuleInput }> = [];
  archiveWorldRuleCalls: string[] = [];
  createForeshadowingHookCalls: ForeshadowingHookInput[] = [];
  updateForeshadowingHookCalls: Array<{
    id: string;
    input: ForeshadowingHookInput;
  }> = [];
  archiveForeshadowingHookCalls: string[] = [];
  createStoryEventCalls: StoryEventInput[] = [];
  updateStoryEventCalls: Array<{ id: string; input: StoryEventInput }> = [];
  archiveStoryEventCalls: string[] = [];
  linkStoryEventCalls: StoryEventLinkInput[] = [];
  unlinkStoryEventCalls: Array<{
    eventId: string;
    targetType: EventTargetType;
    targetId: string;
  }> = [];
  createFactCalls: FactInput[] = [];
  updateFactCalls: Array<{ id: string; input: FactInput }> = [];
  confirmFactCalls: string[] = [];
  archiveFactCalls: string[] = [];
  createCharacterKnowledgeCalls: CharacterKnowledgeInput[] = [];
  archiveCharacterKnowledgeCalls: string[] = [];
  addFactEvidenceCalls: FactEvidenceInput[] = [];
  removeFactEvidenceCalls: string[] = [];
  addFactScopeCalls: FactScopeInput[] = [];
  removeFactScopeCalls: string[] = [];
  createFactConflictCalls: FactConflictInput[] = [];
  resolveFactConflictCalls: string[] = [];
  deleteFactConflictCalls: string[] = [];
  saveProviderConnectionCalls: ProviderConnectionInput[] = [];
  deleteProviderConnectionCalls: string[] = [];
  testProviderConnectionCalls: string[] = [];
  saveAgentProfileCalls: AgentProfileInput[] = [];
  saveAgentRuntimeSettingsCalls: AgentRuntimeSettingsInput[] = [];
  createChapterAgentRunCalls: Array<{ chapterId: string; prompt: string }> = [];
  executeAgentRunCalls: string[] = [];
  cancelAgentRunCalls: string[] = [];
  retryAgentRunCalls: string[] = [];
  importJobsCalls = 0;
  prepareManuscriptImportCalls: string[] = [];
  importJobCalls: string[] = [];
  deleteImportJobCalls: string[] = [];
  applyImportJobCalls: Array<{
    id: string;
    selectedChapterIds: string[];
  }> = [];
  extractionTasksCalls = 0;
  extractionTaskCalls: string[] = [];
  startExtractionRunCalls: ExtractionRunInput[] = [];
  executeExtractionRunCalls: string[] = [];
  cancelExtractionRunCalls: string[] = [];
  retryExtractionRunCalls: string[] = [];
  completeCloseCalls = 0;
  beforeCloseHandler: (() => Promise<void>) | null = null;
  getJobSnapshotCalls: string[] = [];
  jobSnapshotResult: JobSnapshotView | null = null;
  getJobEventsCalls: Array<{
    jobId: string;
    afterSequence: number;
    limit: number;
  }> = [];
  jobEventsResult: JobEventsPage = {
    events: [],
    nextCursor: 0,
    hasMore: false,
    snapshotRequired: false,
  };
  listRecoverableJobsCalls = 0;
  recoverableJobsResult: JobSummaryView[] = [];
  jobUpdatedHandlers: Array<
    (jobId: string, jobKind: string, latestSequence: number) => void
  > = [];
  /**
   * Per-job event log used by getJobEvents to serve cursor-based replay.
   * Tests can either pre-populate this map or rely on executeAgentRun /
   * executeExtractionRun to seed events when they transition a job to
   * running and later to terminal.
   */
  jobEventsByJobId: Map<string, JobEventView[]> = new Map();
  jobSnapshotsByJobId: Map<string, JobSnapshotView> = new Map();
  /**
   * When true (default), executeAgentRun / executeExtractionRun schedule an
   * async emitJobUpdated call after updating internal state, simulating the
   * real backend's asynchronous progression. Tests that need deterministic
   * sync behavior can disable this.
   */
  emitJobProgressionOnExecute: boolean = true;

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

  async chooseManuscriptToImport(): Promise<string | null> {
    return this.manuscriptToImport;
  }

  async prepareManuscriptImport(
    sourcePath: string,
  ): Promise<ImportPrepareView> {
    this.prepareManuscriptImportCalls.push(sourcePath);
    if (this.prepareManuscriptImportError) {
      throw this.prepareManuscriptImportError;
    }
    if (!this.prepareManuscriptImportResult) {
      throw new Error("未配置导入准备结果");
    }
    const result = this.prepareManuscriptImportResult;
    this.importJobsResult = [
      result.job,
      ...this.importJobsResult.filter((item) => item.id !== result.job.id),
    ];
    return result;
  }

  async importJobs(): Promise<ImportJobView[]> {
    this.importJobsCalls++;
    if (this.importJobsError) throw this.importJobsError;
    return this.importJobsResult;
  }

  async importJob(id: string): Promise<ImportJobView> {
    this.importJobCalls.push(id);
    const job = this.importJobsResult.find((item) => item.id === id);
    if (!job) throw new Error("导入任务不存在");
    return job;
  }

  async deleteImportJob(id: string): Promise<void> {
    this.deleteImportJobCalls.push(id);
    if (this.importMutationError) throw this.importMutationError;
    const job = this.importJobsResult.find((item) => item.id === id);
    if (job?.status === "applied") throw new Error("已应用任务不可删除");
    this.importJobsResult = this.importJobsResult.filter(
      (item) => item.id !== id,
    );
  }

  async applyImportJob(
    id: string,
    selectedChapterIds: string[],
  ): Promise<ImportApplyView> {
    this.applyImportJobCalls.push({
      id,
      selectedChapterIds: [...selectedChapterIds],
    });
    if (this.importMutationError) throw this.importMutationError;
    if (!this.importApplyResult) throw new Error("未配置导入应用结果");
    this.importJobsResult = this.importJobsResult.map((item) =>
      item.id === id ? this.importApplyResult!.job : item,
    );
    this.chaptersResult = [...this.importApplyResult.chapters];
    return this.importApplyResult;
  }

  async extractionTasks(): Promise<ExtractionTaskView[]> {
    this.extractionTasksCalls++;
    if (this.extractionError) throw this.extractionError;
    return this.extractionTasksResult;
  }

  async extractionTask(id: string): Promise<ExtractionRunView> {
    this.extractionTaskCalls.push(id);
    if (this.extractionError) throw this.extractionError;
    if (this.extractionRunResult) return this.extractionRunResult;
    const task = this.extractionTasksResult.find((item) => item.id === id);
    if (!task) throw new Error("抽取任务不存在");
    return { task, changeSet: null };
  }

  async startExtractionRun(
    input: ExtractionRunInput,
  ): Promise<ExtractionRunView> {
    this.startExtractionRunCalls.push(input);
    if (this.extractionError) throw this.extractionError;
    if (!this.extractionStartResult) throw new Error("未配置抽取启动结果");
    const run = this.extractionStartResult;
    this.extractionTasksResult = [
      run.task,
      ...this.extractionTasksResult.filter((item) => item.id !== run.task.id),
    ];
    this.appendJobEvent(run.task.id, "extraction", 1, "job_queued", "");
    this.upsertJobSnapshot({
      jobId: run.task.id,
      jobKind: "extraction",
      projectId: this.openProjectResult.id,
      status: "queued",
      maxSequence: 1,
      earliestReplayableSequence: 1,
      inputTokens: 0,
      outputTokens: 0,
      updatedAt: run.task.updatedAt,
    });
    return run;
  }

  async executeExtractionRun(id: string): Promise<ExtractionRunView> {
    this.executeExtractionRunCalls.push(id);
    if (this.extractionError) throw this.extractionError;
    if (!this.extractionExecuteResult) throw new Error("未配置抽取执行结果");
    const terminalRun = this.extractionExecuteResult;
    // Phase 1: persist running state and append job_started event in the
    // same short transaction. Return a running view immediately; the
    // provider pipeline runs in a background goroutine on the real backend.
    const existingTask = this.extractionTasksResult.find(
      (item) => item.id === id,
    );
    const runningTask: ExtractionTaskView = existingTask
      ? { ...existingTask, status: "running" }
      : { ...terminalRun.task, status: "running" };
    this.extractionTasksResult = this.extractionTasksResult.map((item) =>
      item.id === id ? runningTask : item,
    );
    const projectId = this.openProjectResult.id;
    this.appendJobEvent(id, "extraction", 2, "job_started", "");
    this.upsertJobSnapshot({
      jobId: id,
      jobKind: "extraction",
      projectId,
      status: "running",
      maxSequence: 2,
      earliestReplayableSequence: 1,
      inputTokens: 0,
      outputTokens: 0,
      updatedAt: "2026-08-07T00:00:00Z",
    });
    if (this.emitJobProgressionOnExecute) {
      void this.scheduleExtractionRunProgression(id, terminalRun);
    }
    return { task: runningTask, changeSet: null };
  }

  /**
   * scheduleExtractionRunProgression mirrors scheduleAgentRunProgression:
   * after executeExtractionRun returns, the fake backend advances the task
   * to the pre-configured terminal state and emits job:updated.
   */
  private scheduleExtractionRunProgression(
    id: string,
    terminalRun: ExtractionRunView,
  ): void {
    const projectId = this.openProjectResult.id;
    queueMicrotask(() => {
      this.extractionTasksResult = this.extractionTasksResult.map((item) =>
        item.id === id ? terminalRun.task : item,
      );
      this.extractionRunResult = terminalRun;
      const inputTokens = terminalRun.task.steps.reduce(
        (sum, step) => sum + step.inputTokens,
        0,
      );
      const outputTokens = terminalRun.task.steps.reduce(
        (sum, step) => sum + step.outputTokens,
        0,
      );
      this.appendJobEvent(
        id,
        "extraction",
        3,
        "job_completed",
        terminalRun.task.changeSetId
          ? JSON.stringify({ change_set_id: terminalRun.task.changeSetId })
          : "",
      );
      this.upsertJobSnapshot({
        jobId: id,
        jobKind: "extraction",
        projectId,
        status: terminalRun.task.status,
        maxSequence: 3,
        earliestReplayableSequence: 1,
        inputTokens,
        outputTokens,
        pendingAction: terminalRun.task.changeSetId ? "review_change_set" : "",
        terminalInfo: terminalRun.task.changeSetId
          ? JSON.stringify({ changeSetId: terminalRun.task.changeSetId })
          : "",
        updatedAt: "2026-08-07T00:00:00Z",
      });
      this.emitJobUpdated(id, "extraction", 3);
    });
  }

  async cancelExtractionRun(id: string): Promise<void> {
    this.cancelExtractionRunCalls.push(id);
    if (this.extractionError) throw this.extractionError;
    this.extractionTasksResult = this.extractionTasksResult.filter(
      (item) => item.id !== id,
    );
  }

  async retryExtractionRun(id: string): Promise<ExtractionRunView> {
    this.retryExtractionRunCalls.push(id);
    if (this.extractionError) throw this.extractionError;
    if (!this.extractionRetryResult) throw new Error("未配置抽取重试结果");
    const run = this.extractionRetryResult;
    this.extractionTasksResult = this.extractionTasksResult.map((item) =>
      item.id === id ? run.task : item,
    );
    return run;
  }

  async createProject(input: CreateProjectInput): Promise<ProjectView> {
    this.createProjectCalls.push(input);
    if (this.createProjectError) throw this.createProjectError;
    return this.createProjectResult;
  }

  async openProject(
    path: string,
    mode: ProjectAccessMode = "editable",
  ): Promise<ProjectOpenResultView> {
    this.openProjectCalls.push(path);
    this.openProjectInputs.push({ path, mode });
    if (this.openProjectError) throw this.openProjectError;
    return this.projectOpenResponse ?? {
      outcome: "opened",
      project: this.openProjectResult,
    };
  }

  async createProjectRecoveryCopy(
    input: RecoveryCopyInput,
  ): Promise<RecoveryCopyResultView> {
    this.createProjectRecoveryCopyCalls.push(input);
    return this.recoveryCopyResponse ?? {
      outcome: "opened",
      path: this.openProjectResult.path,
      project: this.openProjectResult,
    };
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
    const listed = this.chaptersResult.find(
      (chapter) => chapter.id === chapterId,
    );
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

  async changeSets(): Promise<ChangeSetView[]> {
    return this.changeSetsResult;
  }

  async proposeChapterChange(
    input: ChapterChangeInput,
  ): Promise<ChangeSetView> {
    this.proposeChapterChangeCalls.push(input);
    const value: ChangeSetView = {
      id: `change-set-${this.changeSetsResult.length + 1}`,
      title: input.title,
      summary: input.summary,
      createdBy: input.createdBy,
      status: "pending",
      operations: [
        {
          id: `operation-${this.changeSetsResult.length + 1}`,
          kind: "replace_chapter_body",
          targetId: input.chapterId,
          baseRevision: this.openChapterResult.revision,
          baseFingerprint: "",
          beforeValue: this.openChapterResult.body,
          afterValue: input.proposedBody,
          reason: input.reason,
          status: "pending",
        },
      ],
      createdAt: "2026-07-28T04:00:00Z",
      updatedAt: "2026-07-28T04:00:00Z",
    };
    this.changeSetsResult = [value, ...this.changeSetsResult];
    return value;
  }

  async proposeStructuredChange(
    input: StructuredChangeInput,
  ): Promise<ChangeSetView> {
    this.proposeStructuredChangeCalls.push(input);
    const value: ChangeSetView = {
      id: `change-set-${this.changeSetsResult.length + 1}`,
      title: input.title,
      summary: input.summary,
      createdBy: input.createdBy,
      status: "pending",
      operations: input.operations.map((operation, index) => ({
        id: `operation-${this.changeSetsResult.length + 1}-${index + 1}`,
        kind: operation.kind,
        targetId: operation.targetId,
        baseRevision: 0,
        baseFingerprint: "fake-fingerprint",
        beforeValue: "{}",
        afterValue: JSON.stringify(
          operation.outline ?? operation.character ?? operation.fact ?? {},
        ),
        reason: operation.reason,
        status: "pending",
      })),
      createdAt: "2026-07-28T04:00:00Z",
      updatedAt: "2026-07-28T04:00:00Z",
    };
    this.changeSetsResult = [value, ...this.changeSetsResult];
    return value;
  }

  async editChangeOperation(
    input: ChangeOperationEditInput,
  ): Promise<ChangeSetView> {
    this.editChangeOperationCalls.push(input);
    let updated: ChangeSetView | undefined;
    this.changeSetsResult = this.changeSetsResult.map((changeSet) => {
      if (changeSet.id !== input.changeSetId) return changeSet;
      updated = {
        ...changeSet,
        operations: changeSet.operations.map((operation) =>
          operation.id === input.operationId
            ? {
                ...operation,
                afterValue:
                  operation.kind === "replace_chapter_body"
                    ? input.chapterBody
                    : JSON.stringify(
                        input.outline ?? input.character ?? input.fact ?? {},
                      ),
              }
            : operation,
        ),
      };
      return updated;
    });
    if (!updated) throw new Error("变更集不存在");
    return updated;
  }

  async applyChangeSet(id: string): Promise<ChangeSetApplyView> {
    this.applyChangeSetCalls.push(id);
    const changeSet = this.changeSetsResult.find((item) => item.id === id);
    if (!changeSet) throw new Error("变更集不存在");
    const chapters: ChapterView[] = [];
    for (const operation of changeSet.operations) {
      if (
        operation.status === "pending" &&
        operation.kind === "replace_chapter_body"
      ) {
        const updated = {
          ...this.openChapterResult,
          body: operation.afterValue,
          revision: this.openChapterResult.revision + 1,
        };
        this.openChapterResult = updated;
        chapters.push(updated);
      }
    }
    this.changeSetsResult = this.changeSetsResult.map((item) =>
      item.id === id
        ? {
            ...item,
            status: "applied",
            operations: item.operations.map((candidate) => ({
              ...candidate,
              status:
                candidate.status === "pending" ? "applied" : candidate.status,
            })),
          }
        : item,
    );
    return {
      changeSet: this.changeSetsResult.find((item) => item.id === id)!,
      chapters,
    };
  }

  async applyChangeOperation(
    changeSetId: string,
    operationId: string,
  ): Promise<ChangeSetApplyView> {
    this.applyChangeOperationCalls.push({ changeSetId, operationId });
    let chapters: ChapterView[] = [];
    this.changeSetsResult = this.changeSetsResult.map((item) => {
      if (item.id !== changeSetId) return item;
      const operations = item.operations.map((candidate) => {
        if (candidate.id !== operationId) return candidate;
        if (candidate.kind === "replace_chapter_body") {
          const updated = {
            ...this.openChapterResult,
            body: candidate.afterValue,
            revision: this.openChapterResult.revision + 1,
          };
          this.openChapterResult = updated;
          chapters = [updated];
        }
        return { ...candidate, status: "applied" as const };
      });
      return {
        ...item,
        operations,
        status: deriveFakeChangeSetStatus(operations),
      };
    });
    const changeSet = this.changeSetsResult.find(
      (item) => item.id === changeSetId,
    );
    if (!changeSet) throw new Error("变更集不存在");
    return { changeSet, chapters };
  }

  async rejectChangeSet(id: string): Promise<void> {
    this.rejectChangeSetCalls.push(id);
    this.changeSetsResult = this.changeSetsResult.map((item) =>
      item.id === id
        ? {
            ...item,
            status: "rejected",
            operations: item.operations.map((candidate) => ({
              ...candidate,
              status: "rejected",
            })),
          }
        : item,
    );
  }

  async rejectChangeOperation(
    changeSetId: string,
    operationId: string,
  ): Promise<void> {
    this.rejectChangeOperationCalls.push({ changeSetId, operationId });
    this.changeSetsResult = this.changeSetsResult.map((item) => {
      if (item.id !== changeSetId) return item;
      const operations = item.operations.map((candidate) =>
        candidate.id === operationId
          ? { ...candidate, status: "rejected" as const }
          : candidate,
      );
      return {
        ...item,
        operations,
        status: deriveFakeChangeSetStatus(operations),
      };
    });
  }

  async chapterHistory(chapterId: string): Promise<ChapterRevisionView[]> {
    return this.chapterHistoryResult.filter(
      (item) => item.chapterId === chapterId,
    );
  }

  async restoreChapterRevision(
    chapterId: string,
    targetRevision: number,
    expectedRevision: number,
  ): Promise<ChapterView> {
    this.restoreChapterRevisionCalls.push({
      chapterId,
      targetRevision,
      expectedRevision,
    });
    const target = this.chapterHistoryResult.find(
      (item) =>
        item.chapterId === chapterId && item.revision === targetRevision,
    );
    if (!target) throw new Error("历史版本不存在");
    const current =
      this.chaptersResult.find((item) => item.id === chapterId) ??
      this.openChapterResult;
    if (current.revision !== expectedRevision) {
      throw new RevisionConflictError();
    }
    const updated: ChapterView = {
      ...current,
      title: target.title,
      body: target.body,
      revision: current.revision + 1,
    };
    this.openChapterResult = updated;
    this.chaptersResult = this.chaptersResult.map((item) =>
      item.id === chapterId ? updated : item,
    );
    this.chapterHistoryResult = this.chapterHistoryResult.map((item) => ({
      ...item,
      isCurrent: false,
    }));
    return updated;
  }

  async snapshots(): Promise<SnapshotView[]> {
    return this.snapshotsResult;
  }

  async createSnapshot(name: string): Promise<SnapshotView> {
    this.createSnapshotCalls.push(name);
    const snapshot: SnapshotView = {
      id: `snapshot-${this.snapshotsResult.length + 1}`,
      name,
      kind: "named",
      createdAt: "2026-07-28T05:10:00Z",
    };
    this.snapshotsResult = [snapshot, ...this.snapshotsResult];
    return snapshot;
  }

  async restoreSnapshot(id: string): Promise<SnapshotRestoreView> {
    this.restoreSnapshotCalls.push(id);
    this.openProjectResult = this.snapshotRestoreResult.project;
    this.chaptersResult = this.snapshotRestoreResult.chapters;
    this.openChapterResult =
      this.snapshotRestoreResult.chapters[0] ?? this.openChapterResult;
    return { ...this.snapshotRestoreResult, snapshotId: id };
  }

  async branches(): Promise<BranchView[]> {
    return this.branchesResult;
  }

  async createBranch(input: CreateBranchInput): Promise<BranchView> {
    this.createBranchCalls.push(input);
    const parent =
      this.branchesResult.find((branch) => branch.active) ??
      this.branchesResult[0];
    const created: BranchView = {
      id: `branch-${this.branchesResult.length + 1}`,
      ...input,
      parentId: parent?.id ?? "",
      active: false,
      main: false,
      generation: 1,
      createdAt: "2026-07-28T06:00:00Z",
      updatedAt: "2026-07-28T06:00:00Z",
    };
    this.branchesResult = [...this.branchesResult, created];
    return created;
  }

  async switchBranch(id: string): Promise<BranchSwitchView> {
    this.switchBranchCalls.push(id);
    this.branchesResult = this.branchesResult.map((branch) => ({
      ...branch,
      active: branch.id === id,
    }));
    return {
      activeBranchId: id,
      project: this.openProjectResult,
      chapters: this.chaptersResult,
    };
  }

  async compareBranches(
    sourceId: string,
    targetId: string,
  ): Promise<BranchComparisonView> {
    this.compareBranchCalls.push({ sourceId, targetId });
    const source = this.branchesResult.find((branch) => branch.id === sourceId);
    const target = this.branchesResult.find((branch) => branch.id === targetId);
    if (!source || !target) throw new Error("分支不存在");
    return {
      source,
      target,
      addedChapters: 0,
      modifiedChapters: 0,
      removedChapters: 0,
      chapterChanges: [],
      domains: [],
    };
  }

  async previewBranchMerge(sourceId: string): Promise<BranchMergePreviewView> {
    this.previewBranchMergeCalls.push(sourceId);
    const target = this.branchesResult.find((branch) => branch.active);
    if (!target) throw new Error("当前分支不存在");
    return {
      token: "fake-branch-merge-token",
      sourceId,
      targetId: target.id,
      comparison: await this.compareBranches(sourceId, target.id),
    };
  }

  async confirmBranchMerge(token: string): Promise<BranchSwitchView> {
    this.confirmBranchMergeCalls.push(token);
    const active = this.branchesResult.find((branch) => branch.active);
    if (!active) throw new Error("当前分支不存在");
    return {
      activeBranchId: active.id,
      project: this.openProjectResult,
      chapters: this.chaptersResult,
    };
  }

  async deleteBranch(id: string): Promise<void> {
    this.deleteBranchCalls.push(id);
    this.branchesResult = this.branchesResult.filter(
      (branch) => branch.id !== id,
    );
  }

  async storyStructure(): Promise<StoryStructureView> {
    return this.storyStructureResult;
  }

  async createOutlineNode(
    input: CreateOutlineNodeInput,
  ): Promise<OutlineNodeView> {
    this.createOutlineNodeCalls.push(input);
    const node: OutlineNodeView = {
      ...input,
      id: `outline-${this.storyStructureResult.outlineNodes.length + 1}`,
      position:
        this.storyStructureResult.outlineNodes.filter(
          (item) => item.parentId === input.parentId,
        ).length + 1,
    };
    this.storyStructureResult = {
      ...this.storyStructureResult,
      outlineNodes: [...this.storyStructureResult.outlineNodes, node],
    };
    return node;
  }

  async createStoryLine(input: CreateStoryLineInput): Promise<StoryLineView> {
    this.createStoryLineCalls.push(input);
    const line = {
      ...input,
      id: `line-${this.storyStructureResult.storyLines.length + 1}`,
      status: "planned",
    };
    this.storyStructureResult = {
      ...this.storyStructureResult,
      storyLines: [...this.storyStructureResult.storyLines, line],
    };
    return line;
  }

  async linkStoryLine(input: LinkStoryLineInput): Promise<StoryLineLinkView> {
    this.linkStoryLineCalls.push(input);
    const link = {
      ...input,
      position:
        this.storyStructureResult.storyLineLinks.filter(
          (item) => item.storyLineId === input.storyLineId,
        ).length + 1,
    };
    this.storyStructureResult = {
      ...this.storyStructureResult,
      storyLineLinks: [...this.storyStructureResult.storyLineLinks, link],
    };
    return link;
  }

  async createSceneCard(input: CreateSceneCardInput): Promise<SceneCardView> {
    this.createSceneCardCalls.push(input);
    const card = {
      ...input,
      id: `card-${this.storyStructureResult.sceneCards.length + 1}`,
      position:
        this.storyStructureResult.sceneCards.filter(
          (item) => item.chapterId === input.chapterId,
        ).length + 1,
      status: "planned",
    };
    this.storyStructureResult = {
      ...this.storyStructureResult,
      sceneCards: [...this.storyStructureResult.sceneCards, card],
    };
    return card;
  }

  async archiveOutlineNode(id: string): Promise<void> {
    this.archiveOutlineNodeCalls.push(id);
    this.storyStructureResult = {
      ...this.storyStructureResult,
      outlineNodes: this.storyStructureResult.outlineNodes.filter(
        (item) => item.id !== id,
      ),
    };
  }

  async moveOutlineNode(id: string, direction: "up" | "down"): Promise<void> {
    this.moveOutlineNodeCalls.push({ id, direction });
  }

  async storyWorld(): Promise<StoryWorldView> {
    if (this.storyWorldError) throw this.storyWorldError;
    return this.storyWorldResult;
  }

  async createCharacter(input: CharacterInput): Promise<CharacterView> {
    this.createCharacterCalls.push(input);
    if (this.worldMutationError) throw this.worldMutationError;
    const character: CharacterView = {
      ...input,
      id: `character-${this.storyWorldResult.characters.length + 1}`,
    };
    this.storyWorldResult = {
      ...this.storyWorldResult,
      characters: [...this.storyWorldResult.characters, character],
    };
    return character;
  }

  async updateCharacter(
    id: string,
    input: CharacterInput,
  ): Promise<CharacterView> {
    this.updateCharacterCalls.push({ id, input });
    if (this.worldMutationError) throw this.worldMutationError;
    const character = { ...input, id };
    this.storyWorldResult = {
      ...this.storyWorldResult,
      characters: this.storyWorldResult.characters.map((item) =>
        item.id === id ? character : item,
      ),
    };
    return character;
  }

  async archiveCharacter(id: string): Promise<void> {
    this.archiveCharacterCalls.push(id);
    if (this.worldMutationError) throw this.worldMutationError;
    this.storyWorldResult = {
      ...this.storyWorldResult,
      characters: this.storyWorldResult.characters.filter(
        (item) => item.id !== id,
      ),
    };
  }

  async createFaction(input: FactionInput): Promise<FactionView> {
    this.createFactionCalls.push(input);
    if (this.worldMutationError) throw this.worldMutationError;
    const faction: FactionView = {
      ...input,
      id: `faction-${this.storyWorldResult.factions.length + 1}`,
    };
    this.storyWorldResult = {
      ...this.storyWorldResult,
      factions: [...this.storyWorldResult.factions, faction],
    };
    return faction;
  }

  async updateFaction(id: string, input: FactionInput): Promise<FactionView> {
    this.updateFactionCalls.push({ id, input });
    if (this.worldMutationError) throw this.worldMutationError;
    const faction = { ...input, id };
    this.storyWorldResult = {
      ...this.storyWorldResult,
      factions: this.storyWorldResult.factions.map((item) =>
        item.id === id ? faction : item,
      ),
    };
    return faction;
  }

  async archiveFaction(id: string): Promise<void> {
    this.archiveFactionCalls.push(id);
    if (this.worldMutationError) throw this.worldMutationError;
    this.storyWorldResult = {
      ...this.storyWorldResult,
      factions: this.storyWorldResult.factions.filter((item) => item.id !== id),
    };
  }

  async upsertMembership(input: MembershipInput): Promise<MembershipView> {
    this.upsertMembershipCalls.push(input);
    if (this.worldMutationError) throw this.worldMutationError;
    const others = this.storyWorldResult.memberships.filter(
      (item) =>
        item.characterId !== input.characterId ||
        item.factionId !== input.factionId,
    );
    this.storyWorldResult = {
      ...this.storyWorldResult,
      memberships: [...others, input],
    };
    return input;
  }

  async deleteMembership(
    characterId: string,
    factionId: string,
  ): Promise<void> {
    this.deleteMembershipCalls.push({ characterId, factionId });
    if (this.worldMutationError) throw this.worldMutationError;
    this.storyWorldResult = {
      ...this.storyWorldResult,
      memberships: this.storyWorldResult.memberships.filter(
        (item) =>
          item.characterId !== characterId || item.factionId !== factionId,
      ),
    };
  }

  async createRelationship(
    input: RelationshipInput,
  ): Promise<RelationshipView> {
    this.createRelationshipCalls.push(input);
    if (this.worldMutationError) throw this.worldMutationError;
    const relationship = {
      ...input,
      id: `relationship-${this.storyWorldResult.relationships.length + 1}`,
    };
    this.storyWorldResult = {
      ...this.storyWorldResult,
      relationships: [...this.storyWorldResult.relationships, relationship],
    };
    return relationship;
  }

  async archiveRelationship(id: string): Promise<void> {
    this.archiveRelationshipCalls.push(id);
    if (this.worldMutationError) throw this.worldMutationError;
    this.storyWorldResult = {
      ...this.storyWorldResult,
      relationships: this.storyWorldResult.relationships.filter(
        (item) => item.id !== id,
      ),
    };
  }

  async linkCharacterToChapter(input: AppearanceInput): Promise<void> {
    this.linkChapterCalls.push(input);
    if (this.worldMutationError) throw this.worldMutationError;
    this.storyWorldResult = {
      ...this.storyWorldResult,
      chapterAppearances: [
        ...this.storyWorldResult.chapterAppearances.filter(
          (item) =>
            item.characterId !== input.characterId ||
            item.chapterId !== input.targetId,
        ),
        {
          characterId: input.characterId,
          chapterId: input.targetId,
          role: input.role,
        },
      ],
    };
  }

  async unlinkCharacterFromChapter(
    characterId: string,
    chapterId: string,
  ): Promise<void> {
    this.unlinkChapterCalls.push({ characterId, chapterId });
    if (this.worldMutationError) throw this.worldMutationError;
    this.storyWorldResult = {
      ...this.storyWorldResult,
      chapterAppearances: this.storyWorldResult.chapterAppearances.filter(
        (item) =>
          item.characterId !== characterId || item.chapterId !== chapterId,
      ),
    };
  }

  async linkCharacterToScene(input: AppearanceInput): Promise<void> {
    this.linkSceneCalls.push(input);
    if (this.worldMutationError) throw this.worldMutationError;
    this.storyWorldResult = {
      ...this.storyWorldResult,
      sceneAppearances: [
        ...this.storyWorldResult.sceneAppearances.filter(
          (item) =>
            item.characterId !== input.characterId ||
            item.sceneCardId !== input.targetId,
        ),
        {
          characterId: input.characterId,
          sceneCardId: input.targetId,
          role: input.role,
        },
      ],
    };
  }

  async unlinkCharacterFromScene(
    characterId: string,
    sceneCardId: string,
  ): Promise<void> {
    this.unlinkSceneCalls.push({ characterId, sceneCardId });
    if (this.worldMutationError) throw this.worldMutationError;
    this.storyWorldResult = {
      ...this.storyWorldResult,
      sceneAppearances: this.storyWorldResult.sceneAppearances.filter(
        (item) =>
          item.characterId !== characterId || item.sceneCardId !== sceneCardId,
      ),
    };
  }

  async worldAssets(): Promise<WorldAssetsView> {
    return this.worldAssetsResult;
  }

  async createWorldRegion(input: WorldRegionInput): Promise<WorldRegionView> {
    this.createWorldRegionCalls.push(input);
    if (this.worldAssetMutationError) throw this.worldAssetMutationError;
    const region = {
      ...input,
      id: `region-${this.worldAssetsResult.regions.length + 1}`,
    };
    this.worldAssetsResult = {
      ...this.worldAssetsResult,
      regions: [...this.worldAssetsResult.regions, region],
    };
    return region;
  }

  async updateWorldRegion(
    id: string,
    input: WorldRegionInput,
  ): Promise<WorldRegionView> {
    this.updateWorldRegionCalls.push({ id, input });
    if (this.worldAssetMutationError) throw this.worldAssetMutationError;
    const region = { ...input, id };
    this.worldAssetsResult = {
      ...this.worldAssetsResult,
      regions: this.worldAssetsResult.regions.map((item) =>
        item.id === id ? region : item,
      ),
    };
    return region;
  }

  async archiveWorldRegion(id: string): Promise<void> {
    this.archiveWorldRegionCalls.push(id);
    if (this.worldAssetMutationError) throw this.worldAssetMutationError;
    this.worldAssetsResult = {
      ...this.worldAssetsResult,
      regions: this.worldAssetsResult.regions.filter((item) => item.id !== id),
    };
  }

  async upsertRegionAdjacency(
    input: RegionAdjacencyInput,
  ): Promise<RegionAdjacencyView> {
    this.upsertRegionAdjacencyCalls.push(input);
    if (this.worldAssetMutationError) throw this.worldAssetMutationError;
    const firstRegionId =
      input.firstRegionId < input.secondRegionId
        ? input.firstRegionId
        : input.secondRegionId;
    const secondRegionId =
      input.firstRegionId < input.secondRegionId
        ? input.secondRegionId
        : input.firstRegionId;
    const adjacency = { ...input, firstRegionId, secondRegionId };
    this.worldAssetsResult = {
      ...this.worldAssetsResult,
      regionAdjacencies: [
        ...this.worldAssetsResult.regionAdjacencies.filter(
          (item) =>
            item.firstRegionId !== firstRegionId ||
            item.secondRegionId !== secondRegionId,
        ),
        adjacency,
      ],
    };
    return adjacency;
  }

  async deleteRegionAdjacency(
    firstRegionId: string,
    secondRegionId: string,
  ): Promise<void> {
    this.deleteRegionAdjacencyCalls.push({ firstRegionId, secondRegionId });
    if (this.worldAssetMutationError) throw this.worldAssetMutationError;
    const first =
      firstRegionId < secondRegionId ? firstRegionId : secondRegionId;
    const second =
      firstRegionId < secondRegionId ? secondRegionId : firstRegionId;
    this.worldAssetsResult = {
      ...this.worldAssetsResult,
      regionAdjacencies: this.worldAssetsResult.regionAdjacencies.filter(
        (item) =>
          item.firstRegionId !== first || item.secondRegionId !== second,
      ),
    };
  }

  async createLocation(input: LocationInput): Promise<LocationView> {
    this.createLocationCalls.push(input);
    if (this.worldAssetMutationError) throw this.worldAssetMutationError;
    const location = {
      ...input,
      id: `location-${this.worldAssetsResult.locations.length + 1}`,
    };
    this.worldAssetsResult = {
      ...this.worldAssetsResult,
      locations: [...this.worldAssetsResult.locations, location],
    };
    return location;
  }

  async updateLocation(
    id: string,
    input: LocationInput,
  ): Promise<LocationView> {
    this.updateLocationCalls.push({ id, input });
    if (this.worldAssetMutationError) throw this.worldAssetMutationError;
    const location = { ...input, id };
    this.worldAssetsResult = {
      ...this.worldAssetsResult,
      locations: this.worldAssetsResult.locations.map((item) =>
        item.id === id ? location : item,
      ),
    };
    return location;
  }

  async archiveLocation(id: string): Promise<void> {
    this.archiveLocationCalls.push(id);
    if (this.worldAssetMutationError) throw this.worldAssetMutationError;
    this.worldAssetsResult = {
      ...this.worldAssetsResult,
      locations: this.worldAssetsResult.locations.filter(
        (item) => item.id !== id,
      ),
    };
  }

  async linkLocation(input: LocationLinkInput): Promise<LocationLinkView> {
    this.linkLocationCalls.push(input);
    if (this.worldAssetMutationError) throw this.worldAssetMutationError;
    this.worldAssetsResult = {
      ...this.worldAssetsResult,
      locationLinks: [
        ...this.worldAssetsResult.locationLinks.filter(
          (item) =>
            item.locationId !== input.locationId ||
            item.targetType !== input.targetType ||
            item.targetId !== input.targetId,
        ),
        input,
      ],
    };
    return input;
  }

  async unlinkLocation(
    locationId: string,
    targetType: LocationTargetType,
    targetId: string,
  ): Promise<void> {
    this.unlinkLocationCalls.push({ locationId, targetType, targetId });
    if (this.worldAssetMutationError) throw this.worldAssetMutationError;
    this.worldAssetsResult = {
      ...this.worldAssetsResult,
      locationLinks: this.worldAssetsResult.locationLinks.filter(
        (item) =>
          item.locationId !== locationId ||
          item.targetType !== targetType ||
          item.targetId !== targetId,
      ),
    };
  }

  async createItem(input: ItemInput): Promise<ItemView> {
    this.createItemCalls.push(input);
    if (this.worldAssetMutationError) throw this.worldAssetMutationError;
    const item: ItemView = {
      ...input,
      id: `item-${this.worldAssetsResult.items.length + 1}`,
      holderType: "",
      holderId: "",
    };
    this.worldAssetsResult = {
      ...this.worldAssetsResult,
      items: [...this.worldAssetsResult.items, item],
    };
    return item;
  }

  async updateItem(id: string, input: ItemInput): Promise<ItemView> {
    this.updateItemCalls.push({ id, input });
    if (this.worldAssetMutationError) throw this.worldAssetMutationError;
    const existing = this.worldAssetsResult.items.find(
      (item) => item.id === id,
    );
    const item: ItemView = {
      ...input,
      id,
      holderType: existing?.holderType ?? "",
      holderId: existing?.holderId ?? "",
    };
    this.worldAssetsResult = {
      ...this.worldAssetsResult,
      items: this.worldAssetsResult.items.map((candidate) =>
        candidate.id === id ? item : candidate,
      ),
    };
    return item;
  }

  async transferItem(input: ItemTransferInput): Promise<ItemTransferView> {
    this.transferItemCalls.push(input);
    if (this.worldAssetMutationError) throw this.worldAssetMutationError;
    const item = this.worldAssetsResult.items.find(
      (candidate) => candidate.id === input.itemId,
    );
    const transfer: ItemTransferView = {
      id: `transfer-${this.worldAssetsResult.itemTransfers.length + 1}`,
      itemId: input.itemId,
      fromType: item?.holderType ?? "",
      fromId: item?.holderId ?? "",
      toType: input.holderType,
      toId: input.holderId,
      chapterId: input.chapterId,
      note: input.note,
    };
    this.worldAssetsResult = {
      ...this.worldAssetsResult,
      items: this.worldAssetsResult.items.map((candidate) =>
        candidate.id === input.itemId
          ? {
              ...candidate,
              holderType: input.holderType,
              holderId: input.holderId,
            }
          : candidate,
      ),
      itemTransfers: [...this.worldAssetsResult.itemTransfers, transfer],
    };
    return transfer;
  }

  async archiveItem(id: string): Promise<void> {
    this.archiveItemCalls.push(id);
    if (this.worldAssetMutationError) throw this.worldAssetMutationError;
    this.worldAssetsResult = {
      ...this.worldAssetsResult,
      items: this.worldAssetsResult.items.filter((item) => item.id !== id),
    };
  }

  async createWorldRule(input: WorldRuleInput): Promise<WorldRuleView> {
    this.createWorldRuleCalls.push(input);
    if (this.worldAssetMutationError) throw this.worldAssetMutationError;
    const rule = {
      ...input,
      id: `rule-${this.worldAssetsResult.rules.length + 1}`,
    };
    this.worldAssetsResult = {
      ...this.worldAssetsResult,
      rules: [...this.worldAssetsResult.rules, rule],
    };
    return rule;
  }

  async updateWorldRule(
    id: string,
    input: WorldRuleInput,
  ): Promise<WorldRuleView> {
    this.updateWorldRuleCalls.push({ id, input });
    if (this.worldAssetMutationError) throw this.worldAssetMutationError;
    const rule = { ...input, id };
    this.worldAssetsResult = {
      ...this.worldAssetsResult,
      rules: this.worldAssetsResult.rules.map((item) =>
        item.id === id ? rule : item,
      ),
    };
    return rule;
  }

  async archiveWorldRule(id: string): Promise<void> {
    this.archiveWorldRuleCalls.push(id);
    if (this.worldAssetMutationError) throw this.worldAssetMutationError;
    this.worldAssetsResult = {
      ...this.worldAssetsResult,
      rules: this.worldAssetsResult.rules.filter((item) => item.id !== id),
    };
  }

  async createForeshadowingHook(
    input: ForeshadowingHookInput,
  ): Promise<ForeshadowingHookView> {
    this.createForeshadowingHookCalls.push(input);
    if (this.worldAssetMutationError) throw this.worldAssetMutationError;
    const hook = {
      ...input,
      id: `hook-${this.worldAssetsResult.hooks.length + 1}`,
    };
    this.worldAssetsResult = {
      ...this.worldAssetsResult,
      hooks: [...this.worldAssetsResult.hooks, hook],
    };
    return hook;
  }

  async updateForeshadowingHook(
    id: string,
    input: ForeshadowingHookInput,
  ): Promise<ForeshadowingHookView> {
    this.updateForeshadowingHookCalls.push({ id, input });
    if (this.worldAssetMutationError) throw this.worldAssetMutationError;
    const hook = { ...input, id };
    this.worldAssetsResult = {
      ...this.worldAssetsResult,
      hooks: this.worldAssetsResult.hooks.map((item) =>
        item.id === id ? hook : item,
      ),
    };
    return hook;
  }

  async archiveForeshadowingHook(id: string): Promise<void> {
    this.archiveForeshadowingHookCalls.push(id);
    if (this.worldAssetMutationError) throw this.worldAssetMutationError;
    this.worldAssetsResult = {
      ...this.worldAssetsResult,
      hooks: this.worldAssetsResult.hooks.filter((item) => item.id !== id),
    };
  }

  async continuity(): Promise<ContinuityView> {
    return this.continuityResult;
  }
  async createStoryEvent(input: StoryEventInput): Promise<StoryEventView> {
    this.createStoryEventCalls.push(input);
    const event = {
      ...input,
      id: `event-${this.continuityResult.events.length + 1}`,
    };
    this.continuityResult = {
      ...this.continuityResult,
      events: [...this.continuityResult.events, event],
    };
    return event;
  }
  async updateStoryEvent(
    id: string,
    input: StoryEventInput,
  ): Promise<StoryEventView> {
    this.updateStoryEventCalls.push({ id, input });
    const event = { ...input, id };
    this.continuityResult = {
      ...this.continuityResult,
      events: this.continuityResult.events.map((item) =>
        item.id === id ? event : item,
      ),
    };
    return event;
  }
  async archiveStoryEvent(id: string): Promise<void> {
    this.archiveStoryEventCalls.push(id);
    this.continuityResult = {
      ...this.continuityResult,
      events: this.continuityResult.events.filter((item) => item.id !== id),
    };
  }
  async linkStoryEvent(
    input: StoryEventLinkInput,
  ): Promise<StoryEventLinkView> {
    this.linkStoryEventCalls.push(input);
    this.continuityResult = {
      ...this.continuityResult,
      eventLinks: [...this.continuityResult.eventLinks, input],
    };
    return input;
  }
  async unlinkStoryEvent(
    eventId: string,
    targetType: EventTargetType,
    targetId: string,
  ): Promise<void> {
    this.unlinkStoryEventCalls.push({ eventId, targetType, targetId });
    this.continuityResult = {
      ...this.continuityResult,
      eventLinks: this.continuityResult.eventLinks.filter(
        (item) =>
          item.eventId !== eventId ||
          item.targetType !== targetType ||
          item.targetId !== targetId,
      ),
    };
  }
  async createFact(input: FactInput): Promise<FactView> {
    this.createFactCalls.push(input);
    const fact: FactView = {
      ...input,
      id: `fact-${this.continuityResult.facts.length + 1}`,
      createdBy: input.origin,
      confirmedBy: input.status === "author_confirmed" ? "author" : "",
    };
    this.continuityResult = {
      ...this.continuityResult,
      facts: [...this.continuityResult.facts, fact],
    };
    return fact;
  }
  async updateFact(id: string, input: FactInput): Promise<FactView> {
    this.updateFactCalls.push({ id, input });
    const old = this.continuityResult.facts.find((item) => item.id === id);
    const fact: FactView = {
      ...input,
      id,
      createdBy: old?.createdBy ?? input.origin,
      confirmedBy: input.status === "author_confirmed" ? "author" : "",
    };
    this.continuityResult = {
      ...this.continuityResult,
      facts: this.continuityResult.facts.map((item) =>
        item.id === id ? fact : item,
      ),
    };
    return fact;
  }
  async confirmFact(id: string): Promise<FactView> {
    this.confirmFactCalls.push(id);
    const existing = this.continuityResult.facts.find((item) => item.id === id);
    if (!existing) throw new Error("事实不存在");
    const fact: FactView = {
      ...existing,
      status: "author_confirmed",
      confirmedBy: "author",
    };
    this.continuityResult = {
      ...this.continuityResult,
      facts: this.continuityResult.facts.map((item) =>
        item.id === id ? fact : item,
      ),
    };
    return fact;
  }
  async archiveFact(id: string): Promise<void> {
    this.archiveFactCalls.push(id);
    this.continuityResult = {
      ...this.continuityResult,
      facts: this.continuityResult.facts.filter((item) => item.id !== id),
    };
  }
  async characterKnowledge(): Promise<CharacterKnowledgeView[]> {
    return this.characterKnowledgeResult;
  }
  async createCharacterKnowledge(
    input: CharacterKnowledgeInput,
  ): Promise<CharacterKnowledgeView> {
    this.createCharacterKnowledgeCalls.push(input);
    const value: CharacterKnowledgeView = {
      ...input,
      id: `knowledge-${this.characterKnowledgeResult.length + 1}`,
    };
    this.characterKnowledgeResult = [...this.characterKnowledgeResult, value];
    return value;
  }
  async archiveCharacterKnowledge(id: string): Promise<void> {
    this.archiveCharacterKnowledgeCalls.push(id);
    this.characterKnowledgeResult = this.characterKnowledgeResult.filter(
      (item) => item.id !== id,
    );
  }
  async addFactEvidence(input: FactEvidenceInput): Promise<FactEvidenceView> {
    this.addFactEvidenceCalls.push(input);
    const value = {
      ...input,
      id: `evidence-${this.continuityResult.evidence.length + 1}`,
    };
    this.continuityResult = {
      ...this.continuityResult,
      evidence: [...this.continuityResult.evidence, value],
    };
    return value;
  }
  async removeFactEvidence(id: string): Promise<void> {
    this.removeFactEvidenceCalls.push(id);
    this.continuityResult = {
      ...this.continuityResult,
      evidence: this.continuityResult.evidence.filter((item) => item.id !== id),
    };
  }
  async addFactScope(input: FactScopeInput): Promise<FactScopeView> {
    this.addFactScopeCalls.push(input);
    const value = {
      ...input,
      id: `scope-${this.continuityResult.scopes.length + 1}`,
    };
    this.continuityResult = {
      ...this.continuityResult,
      scopes: [...this.continuityResult.scopes, value],
    };
    return value;
  }
  async removeFactScope(id: string): Promise<void> {
    this.removeFactScopeCalls.push(id);
    this.continuityResult = {
      ...this.continuityResult,
      scopes: this.continuityResult.scopes.filter((item) => item.id !== id),
    };
  }
  async createFactConflict(
    input: FactConflictInput,
  ): Promise<FactConflictView> {
    this.createFactConflictCalls.push(input);
    const value: FactConflictView = {
      ...input,
      id: `conflict-${this.continuityResult.conflicts.length + 1}`,
      status: "open",
    };
    this.continuityResult = {
      ...this.continuityResult,
      conflicts: [...this.continuityResult.conflicts, value],
    };
    return value;
  }
  async resolveFactConflict(id: string): Promise<FactConflictView> {
    this.resolveFactConflictCalls.push(id);
    const old = this.continuityResult.conflicts.find((item) => item.id === id);
    if (!old) throw new Error("冲突不存在");
    const value: FactConflictView = { ...old, status: "resolved" };
    this.continuityResult = {
      ...this.continuityResult,
      conflicts: this.continuityResult.conflicts.map((item) =>
        item.id === id ? value : item,
      ),
    };
    return value;
  }
  async deleteFactConflict(id: string): Promise<void> {
    this.deleteFactConflictCalls.push(id);
    this.continuityResult = {
      ...this.continuityResult,
      conflicts: this.continuityResult.conflicts.filter(
        (item) => item.id !== id,
      ),
    };
  }

  async providerConnections(): Promise<ProviderConnectionView[]> {
    return this.providerConnectionsResult;
  }
  async saveProviderConnection(
    input: ProviderConnectionInput,
  ): Promise<ProviderConnectionView> {
    this.saveProviderConnectionCalls.push(input);
    const value: ProviderConnectionView = {
      id: input.id ?? `provider-${this.providerConnectionsResult.length + 1}`,
      name: input.name,
      kind: input.kind,
      baseUrl: input.baseUrl,
      hasCredential: Boolean(input.apiKey),
      updatedAt: "2026-07-29T10:00:00Z",
    };
    this.providerConnectionsResult = [
      ...this.providerConnectionsResult.filter((item) => item.id !== value.id),
      value,
    ];
    return value;
  }
  async deleteProviderConnection(id: string): Promise<void> {
    this.deleteProviderConnectionCalls.push(id);
    this.providerConnectionsResult = this.providerConnectionsResult.filter(
      (item) => item.id !== id,
    );
  }
  async testProviderConnection(id: string): Promise<void> {
    this.testProviderConnectionCalls.push(id);
  }
  async agentProfiles(): Promise<AgentProfileView[]> {
    return this.agentProfilesResult;
  }
  async saveAgentProfile(input: AgentProfileInput): Promise<AgentProfileView> {
    this.saveAgentProfileCalls.push(input);
    const existing = this.agentProfilesResult.find(
      (item) => item.role === input.role,
    );
    const value: AgentProfileView = {
      ...input,
      id: existing?.id ?? `profile-${this.agentProfilesResult.length + 1}`,
      updatedAt: "2026-07-29T10:00:00Z",
    };
    this.agentProfilesResult = [
      ...this.agentProfilesResult.filter((item) => item.role !== input.role),
      value,
    ];
    return value;
  }
  async agentRuntimeSettings(): Promise<AgentRuntimeSettingsView> {
    return this.agentRuntimeSettingsResult;
  }
  async saveAgentRuntimeSettings(
    input: AgentRuntimeSettingsInput,
  ): Promise<AgentRuntimeSettingsView> {
    this.saveAgentRuntimeSettingsCalls.push(input);
    this.agentRuntimeSettingsResult = {
      ...input,
      updatedAt: "2026-07-30T10:00:00Z",
    };
    return this.agentRuntimeSettingsResult;
  }
  async createChapterAgentRun(
    chapterId: string,
    prompt: string,
  ): Promise<AgentRunView> {
    this.createChapterAgentRunCalls.push({ chapterId, prompt });
    const value: AgentRunView = {
      id: "run-1",
      chapterId,
      prompt,
      status: "queued",
      errorCode: "",
      errorMessage: "",
      changeSetId: "",
      retryOf: "",
      tokenBudget: this.agentRuntimeSettingsResult.maxRunTokens,
      contextCharBudget: this.agentRuntimeSettingsResult.maxContextChars,
      inputTokens: 0,
      outputTokens: 0,
      contextChars: 0,
      contextTruncated: false,
      steps: [],
      createdAt: "2026-07-29T10:00:00Z",
      updatedAt: "2026-07-29T10:00:00Z",
    };
    this.agentRunsResult = [value, ...this.agentRunsResult];
    this.appendJobEvent(value.id, "agent", 1, "job_queued", "");
    this.upsertJobSnapshot({
      jobId: value.id,
      jobKind: "agent",
      projectId: this.openProjectResult.id,
      status: "queued",
      maxSequence: 1,
      earliestReplayableSequence: 1,
      inputTokens: 0,
      outputTokens: 0,
      updatedAt: value.updatedAt,
    });
    return value;
  }
  async executeAgentRun(id: string): Promise<AgentRunView> {
    this.executeAgentRunCalls.push(id);
    const existing = this.agentRunsResult.find((item) => item.id === id);
    if (!existing) throw new Error("运行不存在");
    // Phase 1: persist running state and append job_started event in the
    // same short transaction. Return the running view immediately; the
    // provider pipeline runs in a background goroutine on the real backend.
    const running: AgentRunView = {
      ...existing,
      status: "running",
    };
    this.agentRunsResult = this.agentRunsResult.map((item) =>
      item.id === id ? running : item,
    );
    this.appendJobEvent(id, "agent", 2, "job_started", "");
    this.upsertJobSnapshot({
      jobId: id,
      jobKind: "agent",
      projectId: this.openProjectResult.id,
      status: "running",
      maxSequence: 2,
      earliestReplayableSequence: 1,
      inputTokens: 0,
      outputTokens: 0,
      updatedAt: "2026-08-07T00:00:00Z",
    });
    if (this.emitJobProgressionOnExecute) {
      // Simulate the background goroutine finishing after the call returns.
      // Tests can await screen.findByText(...) to observe the terminal state.
      void this.scheduleAgentRunProgression(id);
    }
    return running;
  }

  /**
   * scheduleAgentRunProgression simulates the backend goroutine completing
   * the planner → writer → continuity_reviewer pipeline. It is scheduled
   * via queueMicrotask so the initial `executeAgentRun` promise resolves
   * before progression begins. Tests that need to assert intermediate
   * running state can disable emitJobProgressionOnExecute.
   */
  private scheduleAgentRunProgression(id: string): void {
    const projectId = this.openProjectResult.id;
    queueMicrotask(() => {
      const terminal: AgentRunView = {
        ...(this.agentRunsResult.find((item) => item.id === id) ?? {
          id,
          chapterId: "",
          prompt: "",
          status: "running",
          errorCode: "",
          errorMessage: "",
          changeSetId: "",
          retryOf: "",
          tokenBudget: 0,
          contextCharBudget: 0,
          inputTokens: 0,
          outputTokens: 0,
          contextChars: 0,
          contextTruncated: false,
          steps: [],
          createdAt: "2026-08-07T00:00:00Z",
          updatedAt: "2026-08-07T00:00:00Z",
        }),
        status: "completed",
        changeSetId: "change-set-1",
      };
      this.agentRunsResult = this.agentRunsResult.map((item) =>
        item.id === id ? terminal : item,
      );
      this.appendJobEvent(
        id,
        "agent",
        3,
        "job_completed",
        JSON.stringify({ change_set_id: "change-set-1" }),
      );
      this.upsertJobSnapshot({
        jobId: id,
        jobKind: "agent",
        projectId,
        status: "completed",
        maxSequence: 3,
        earliestReplayableSequence: 1,
        inputTokens: 0,
        outputTokens: 0,
        pendingAction: "review_change_set",
        terminalInfo: JSON.stringify({ changeSetId: "change-set-1" }),
        updatedAt: "2026-08-07T00:00:00Z",
      });
      this.emitJobUpdated(id, "agent", 3);
    });
  }
  async cancelAgentRun(id: string): Promise<void> {
    this.cancelAgentRunCalls.push(id);
    this.agentRunsResult = this.agentRunsResult.map((item) =>
      item.id === id ? { ...item, status: "cancelled" } : item,
    );
  }
  async retryAgentRun(id: string): Promise<AgentRunView> {
    this.retryAgentRunCalls.push(id);
    const original = this.agentRunsResult.find((item) => item.id === id);
    if (!original) throw new Error("运行不存在");
    const value: AgentRunView = {
      ...original,
      id: `run-${this.agentRunsResult.length + 1}`,
      status: "queued",
      retryOf: original.id,
      errorCode: "",
      errorMessage: "",
      changeSetId: "",
      inputTokens: 0,
      outputTokens: 0,
      contextChars: 0,
      contextTruncated: false,
      steps: original.steps.map((step) => ({
        ...step,
        status: "queued",
        output: "",
        errorCode: "",
        errorMessage: "",
        providerRequestId: "",
        inputTokens: 0,
        outputTokens: 0,
      })),
    };
    this.agentRunsResult = [value, ...this.agentRunsResult];
    this.appendJobEvent(value.id, "agent", 1, "job_queued", "");
    this.upsertJobSnapshot({
      jobId: value.id,
      jobKind: "agent",
      projectId: this.openProjectResult.id,
      status: "queued",
      maxSequence: 1,
      earliestReplayableSequence: 1,
      inputTokens: 0,
      outputTokens: 0,
      updatedAt: value.updatedAt,
    });
    return value;
  }
  async agentRuns(): Promise<AgentRunView[]> {
    return this.agentRunsResult;
  }
  async agentRun(id: string): Promise<AgentRunView> {
    const value = this.agentRunsResult.find((item) => item.id === id);
    if (!value) throw new Error("运行不存在");
    return value;
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

  async getJobSnapshot(jobId: string): Promise<JobSnapshotView> {
    this.getJobSnapshotCalls.push(jobId);
    const fromMap = this.jobSnapshotsByJobId.get(jobId);
    if (fromMap) return fromMap;
    if (!this.jobSnapshotResult) throw new Error("作业快照不存在");
    return this.jobSnapshotResult;
  }

  async getJobEvents(
    jobId: string,
    afterSequence: number,
    limit: number,
  ): Promise<JobEventsPage> {
    this.getJobEventsCalls.push({ jobId, afterSequence, limit });
    // Prefer per-job event log when available so cursor-based replay works.
    const fromMap = this.jobEventsByJobId.get(jobId);
    if (fromMap) {
      const filtered = fromMap.filter(
        (evt) => evt.sequenceNumber > afterSequence,
      );
      const sorted = [...filtered].sort(
        (a, b) => a.sequenceNumber - b.sequenceNumber,
      );
      const slice = sorted.slice(0, Math.max(1, limit));
      const snapshot = this.jobSnapshotsByJobId.get(jobId);
      const earliest = snapshot?.earliestReplayableSequence ?? 1;
      if (afterSequence + 1 < earliest) {
        return {
          events: [],
          nextCursor: afterSequence,
          hasMore: false,
          snapshotRequired: true,
        };
      }
      const nextCursor =
        slice.length > 0
          ? slice[slice.length - 1].sequenceNumber
          : afterSequence;
      const maxSequence = snapshot?.maxSequence ?? nextCursor;
      return {
        events: slice,
        nextCursor,
        hasMore: nextCursor < maxSequence,
        snapshotRequired: false,
      };
    }
    return this.jobEventsResult;
  }

  async listRecoverableJobs(): Promise<JobSummaryView[]> {
    this.listRecoverableJobsCalls++;
    if (this.recoverableJobsResult.length > 0)
      return this.recoverableJobsResult;
    // Derive recoverable jobs from non-terminal agent runs and extraction tasks.
    const summaries: JobSummaryView[] = [];
    const projectId = this.openProjectResult.id;
    for (const run of this.agentRunsResult) {
      if (["queued", "running", "recoverable"].includes(run.status)) {
        const snapshot = this.jobSnapshotsByJobId.get(run.id);
        summaries.push({
          jobId: run.id,
          jobKind: "agent",
          projectId,
          status: run.status,
          maxSequence: snapshot?.maxSequence ?? 0,
          updatedAt: run.updatedAt,
        });
      }
    }
    for (const task of this.extractionTasksResult) {
      if (["queued", "running", "recoverable"].includes(task.status)) {
        const snapshot = this.jobSnapshotsByJobId.get(task.id);
        summaries.push({
          jobId: task.id,
          jobKind: "extraction",
          projectId,
          status: task.status,
          maxSequence: snapshot?.maxSequence ?? 0,
          updatedAt: task.updatedAt,
        });
      }
    }
    return summaries;
  }

  onJobUpdated(
    handler: (jobId: string, jobKind: string, latestSequence: number) => void,
  ): () => void {
    this.jobUpdatedHandlers.push(handler);
    return () => {
      this.jobUpdatedHandlers = this.jobUpdatedHandlers.filter(
        (h) => h !== handler,
      );
    };
  }

  emitJobUpdated(jobId: string, jobKind: string, latestSequence: number): void {
    for (const handler of this.jobUpdatedHandlers) {
      handler(jobId, jobKind, latestSequence);
    }
  }

  /**
   * appendJobEvent records an event in the per-job log and is used by the
   * fake's internal async progression helpers. Tests can also call this
   * directly to seed custom event sequences for cursor replay scenarios.
   */
  appendJobEvent(
    jobId: string,
    jobKind: "agent" | "extraction",
    sequence: number,
    eventType: string,
    payload: string,
    stepId: string = "",
  ): void {
    const events = this.jobEventsByJobId.get(jobId) ?? [];
    events.push({
      jobId,
      jobKind,
      projectId: this.openProjectResult.id,
      sequenceNumber: sequence,
      eventType: eventType as JobEventView["eventType"],
      stepId,
      payload,
      createdAt: "2026-08-07T00:00:00Z",
    });
    this.jobEventsByJobId.set(jobId, events);
  }

  /**
   * upsertJobSnapshot stores or replaces the snapshot for a job. The fake
   * uses this to expose cursor lower-bound semantics via getJobEvents.
   */
  upsertJobSnapshot(snapshot: JobSnapshotView): void {
    this.jobSnapshotsByJobId.set(snapshot.jobId, snapshot);
  }
}
