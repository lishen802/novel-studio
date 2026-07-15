export type ProjectStatus = 'planning' | 'writing' | 'paused' | 'completed'
export type ChapterStatus = 'planned' | 'draft' | 'review' | 'approved' | 'published'
export type CanonStatus = 'author-confirmed' | 'text-evidenced' | 'ai-inferred' | 'conflicted' | 'rejected'
export type EntityKind = 'character' | 'faction' | 'location'
export type AgentFamily = 'openai' | 'anthropic' | 'google' | 'deepseek' | 'local' | 'custom'
export type AgentRole = 'planner' | 'context-curator' | 'writer' | 'continuity-auditor' | 'character-auditor' | 'causality-auditor' | 'pacing-auditor' | 'arbiter' | 'reviser' | 'fact-extractor' | 'final-auditor'
export type WorldRegionType = 'world' | 'continent' | 'sea' | 'state' | 'island' | 'city' | 'stronghold' | 'terrain' | 'secret-realm' | 'resource'
export type RelativeDirection = 'north' | 'north-east' | 'east' | 'south-east' | 'south' | 'south-west' | 'west' | 'north-west' | 'center' | 'surrounding'

export interface Scene {
  id: string
  title: string
  summary: string
  content: string
  storyTime: string
  povCharacterId?: string
  locationId?: string
  participantIds: string[]
  goal: string
  conflict: string
  outcome: string
}

export interface Chapter {
  id: string
  number: number
  title: string
  content: string
  summary: string
  status: ChapterStatus
  locked: boolean
  lockedPassages: Array<{ id: string; text: string; occurrence: number }>
  scenes: Scene[]
  updatedAt: string
}

export interface OutlineNode {
  id: string
  parentId: string | null
  level: 'book' | 'volume' | 'arc' | 'chapter' | 'scene'
  title: string
  summary: string
  order: number
  status: 'planned' | 'active' | 'completed'
  linkedEntityIds: string[]
}

export interface Character {
  id: string
  name: string
  role: 'protagonist' | 'supporting' | 'antagonist' | 'minor'
  summary: string
  personality: string[]
  motivation: string
  secrets: string[]
  currentStatus: string
  factionIds: string[]
  targetShare: number
  arc: Array<{ chapter: number; beat: string }>
  knowledge: Array<{ factId: string; learnedChapter: number; source: string; belief: 'true' | 'false' | 'uncertain'; misled: boolean }>
}

export interface Faction {
  id: string
  name: string
  goal: string
  resources: string[]
  stance: string
  memberIds: string[]
  parentFactionId?: string
}

export interface Location {
  id: string
  name: string
  summary: string
  region: string
  x: number
  y: number
  visitChapterIds: string[]
  eventIds: string[]
}

export interface WorldRegion {
  id: string
  parentId: string | null
  name: string
  type: WorldRegionType
  direction: RelativeDirection
  order: number
  summary: string
  terrain: string
  climate: string
  rules: string
  canonStatus: Extract<CanonStatus, 'author-confirmed' | 'text-evidenced' | 'ai-inferred' | 'conflicted'>
  adjacentRegionIds: string[]
  travelNotes: string
  locationIds: string[]
  factionIds: string[]
  characterIds: string[]
  eventIds: string[]
  chapterIds: string[]
  factIds: string[]
  layout: { x: number; y: number; width: number; height: number }
}

export interface StoryItem {
  id: string
  name: string
  holderEntityId?: string
  status: string
  origin: string
  transfers: Array<{ chapterId: string; fromEntityId?: string; toEntityId?: string }>
}

export interface Hook {
  id: string
  title: string
  plantedChapterId: string
  progressChapterIds: string[]
  payoffChapterId?: string
  deadlineChapter?: number
  status: 'planted' | 'developing' | 'paid-off' | 'overdue'
  evidence: string[]
}

export interface Relationship {
  id: string
  sourceId: string
  targetId: string
  type: string
  strength: number
  publicState: string
  trueState: string
  startChapter: number
  endChapter?: number
}

export interface CanonFact {
  id: string
  text: string
  category: string
  status: CanonStatus
  confidence: number
  evidence: Array<{ chapterId: string; quote: string }>
  scopeEntityIds: string[]
}

export interface StoryEvent {
  id: string
  title: string
  summary: string
  cause: string
  result: string
  storyTime: string
  narrativeOrder: number
  status: 'occurred' | 'planned' | 'uncertain'
  participantIds: string[]
  factionIds: string[]
  locationId?: string
  chapterIds: string[]
}

export interface AgentProfile {
  id: string
  name: string
  role: AgentRole
  provider: string
  family: AgentFamily
  model: string
  temperature: number
  enabled: boolean
  instructions: string
}

export interface ReviewIssue {
  id: string
  chapterId: string
  reviewerRole: AgentRole
  reviewerModel: string
  severity: 'critical' | 'warning' | 'suggestion'
  category: string
  description: string
  evidence: Array<{ source: string; quote: string }>
  suggestion: string
  disposition: 'open' | 'accepted' | 'ignored' | 'rechecking' | 'resolved'
  ignoreReason?: string
  secondOpinion?: string
}

export interface WorkflowStep {
  id: 'plan' | 'context' | 'write' | 'review' | 'arbitrate' | 'revise' | 'extract' | 'final-review'
  label: string
  status: 'pending' | 'running' | 'completed' | 'failed' | 'skipped'
  agentId: string
}

export interface WorkflowRun {
  id: string
  chapterId: string
  status: 'idle' | 'running' | 'paused' | 'completed' | 'cancelled' | 'failed'
  progress: number
  currentStep: number
  steps: WorkflowStep[]
  logs: string[]
  createdAt: string
  mockTokens: number
  mockCost: number
}

export interface ProjectSnapshot {
  id: string
  name: string
  kind: 'manual' | 'pre-run' | 'post-run' | 'pre-restore' | 'import'
  createdAt: string
  branchId: string
  payload: string
  summary: string
}

export interface Project {
  id: string
  title: string
  genre: string
  summary: string
  targetWords: number
  chapterWords: number
  status: ProjectStatus
  currentChapterId: string
  writingPreferenceRevisionId: string
  writingPreferences: {
    targetAudience: string
    narrativePerson: 'first' | 'third-limited' | 'third-omniscient'
    povStrategy: string
    tone: string
    pacing: 'slow' | 'balanced' | 'fast'
    styleGuide: string
    forbiddenPatterns: string[]
  }
  chapters: Chapter[]
  outlineNodes: OutlineNode[]
  characters: Character[]
  factions: Faction[]
  locations: Location[]
  items: StoryItem[]
  hooks: Hook[]
  relationships: Relationship[]
  facts: CanonFact[]
  events: StoryEvent[]
  graphPositions: Record<string, { x: number; y: number }>
  mapBackground: string | null
  worldRegions: WorldRegion[]
  worldMapState: { mode: 'map' | 'tree'; focusRegionId: string | null }
  agents: AgentProfile[]
  reviews: ReviewIssue[]
  runs: WorkflowRun[]
  snapshots: ProjectSnapshot[]
  branches: Array<{ id: string; name: string; headSnapshotId?: string }>
  activeBranchId: string
  createdAt: string
  updatedAt: string
}

export interface StudioData {
  schemaVersion: 1
  selectedProjectId: string | null
  projects: Project[]
}

export interface ImportedChapter {
  title: string
  content: string
}

export interface ImportAnalysisResult {
  characters: Character[]
  factions: Faction[]
  locations: Location[]
  relationships: Relationship[]
  facts: CanonFact[]
  events: StoryEvent[]
  outlineNodes: OutlineNode[]
}
