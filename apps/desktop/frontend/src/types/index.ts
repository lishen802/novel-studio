export type ProjectStatus = 'creative' | 'writing' | 'review' | 'finished';

export interface Project {
  id: string;
  title: string;
  subtitle: string;
  author: string;
  wordCount: number;
  targetWordCount: number;
  status: ProjectStatus;
  phase: string;
  updatedAt: string;
  coverColor: string;
  synopsis: string;
}

export type ChapterStatus = 'draft' | 'review' | 'approved' | 'empty';

export interface Chapter {
  id: string;
  projectId: string;
  title: string;
  order: number;
  wordCount: number;
  status: ChapterStatus;
  content: string;
  summary: string;
  aiNotes: string;
}

export type CharacterRole = 'protagonist' | 'supporting' | 'antagonist' | 'minor';

export interface Character {
  id: string;
  projectId: string;
  name: string;
  role: CharacterRole;
  factionId?: string;
  tags: string[];
  summary: string;
  appearance: string;
  motivation: string;
}

export type FactionType = 'family' | 'organization' | 'nation' | 'sect' | 'other';

export interface Faction {
  id: string;
  projectId: string;
  name: string;
  type: FactionType;
  description: string;
  goals: string;
  conflicts: string;
}

export type WorldCategory = 'geography' | 'history' | 'culture' | 'magic' | 'technology' | 'politics';

export interface WorldEntry {
  id: string;
  projectId: string;
  category: WorldCategory;
  title: string;
  content: string;
  importance: 'core' | 'secondary' | 'detail';
}

export interface TimelineEvent {
  id: string;
  projectId: string;
  date: string;
  event: string;
  importance: 'major' | 'minor';
  linkedCharacters: string[];
  linkedChapters: string[];
}

export type ForeshadowStatus = 'planted' | 'growing' | 'resolved' | 'dropped';

export interface Foreshadowing {
  id: string;
  projectId: string;
  title: string;
  description: string;
  status: ForeshadowStatus;
  linkedChapters: string[];
  payoffChapterId?: string;
}

export type CandidateType = 'rewrite' | 'continuation' | 'insert';
export type CandidateStatus = 'pending' | 'accepted' | 'rejected';

export interface Candidate {
  id: string;
  projectId: string;
  chapterId: string;
  type: CandidateType;
  content: string;
  diffFrom: string;
  status: CandidateStatus;
  aiReason: string;
  createdAt: string;
}

export interface Version {
  id: string;
  projectId: string;
  label: string;
  wordCount: number;
  createdAt: string;
  note: string;
  isTagged: boolean;
}

export type AiAgent = 'editor' | 'plot' | 'world' | 'style' | 'general';

export interface AiMessage {
  id: string;
  projectId: string;
  role: 'author' | 'ai';
  agent?: AiAgent;
  content: string;
  timestamp: string;
}

export interface ModelConfig {
  id: string;
  name: string;
  provider: string;
  baseUrl?: string;
  apiKeyConfigured: boolean;
}

export type CreativeRole = 'planner' | 'writer' | 'continuity_reviewer';

/** 创作方案中单个角色的模型绑定，模型名称属于角色绑定而非连接。 */
export interface CreativeProfileRole {
  connectionId: string;
  model: string;
  maxOutputTokens: number;
  temperature: number;
}

export interface CreativeProfile {
  id: string;
  name: string;
  version: number;
  published: boolean;
  roles: Record<CreativeRole, CreativeProfileRole>;
  /** 运行 Token 预算；发布时冻结到方案版本。 */
  maxRunTokens?: number;
  /** 上下文字符预算；发布时冻结到方案版本。 */
  maxContextChars?: number;
}

export interface ProjectProfileSelection {
  profileId: string;
  version: number;
  roleOverrides: Partial<Record<CreativeRole, string>>;
}

export interface AppSettings {
  models: ModelConfig[];
  activeModelId: string;
  creativeProfiles: CreativeProfile[];
  defaultCreativeProfileId: string;
  writingPlan: string;
  theme: 'dark' | 'light' | 'system';
  fontSize: number;
  lineHeight: number;
  canvasColorId: string;
  marginSize: 'narrow' | 'medium' | 'wide';
  indent: boolean;
}

export function getActiveModel(settings: AppSettings): ModelConfig {
  return settings.models.find((m) => m.id === settings.activeModelId) || settings.models[0];
}

import type { ApplicationBootstrapView, ProjectView } from '../api/studio';

export interface AppState {
  currentProject: Project | null;
  backendProject: ProjectView | null;
  bootstrap: ApplicationBootstrapView | null;
  desktopAvailable: boolean;
  busy: boolean;
  error: string | null;
  settings: AppSettings;
  aiPanelOpen: boolean;
  importStatus: 'idle' | 'processing' | 'error' | 'done';
  importProgress: number;
}
