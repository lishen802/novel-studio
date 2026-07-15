import { z } from 'zod'
import type { Project } from './types'

const WorldRegionSchema = z.object({
  id: z.string().min(1),
  parentId: z.string().nullable(),
  name: z.string().trim().min(1),
  type: z.enum(['world', 'continent', 'sea', 'state', 'island', 'city', 'stronghold', 'terrain', 'secret-realm', 'resource']),
  direction: z.enum(['north', 'north-east', 'east', 'south-east', 'south', 'south-west', 'west', 'north-west', 'center', 'surrounding']),
  order: z.number(),
  summary: z.string(),
  terrain: z.string(),
  climate: z.string(),
  rules: z.string(),
  canonStatus: z.enum(['author-confirmed', 'text-evidenced', 'ai-inferred', 'conflicted']),
  adjacentRegionIds: z.array(z.string()),
  travelNotes: z.string(),
  locationIds: z.array(z.string()),
  factionIds: z.array(z.string()),
  characterIds: z.array(z.string()),
  eventIds: z.array(z.string()),
  chapterIds: z.array(z.string()),
  factIds: z.array(z.string()),
  layout: z.object({ x: z.number(), y: z.number(), width: z.number(), height: z.number() }),
})

const ProjectSchema = z.object({
  id: z.string().min(1),
  title: z.string().trim().min(1),
  genre: z.string(),
  summary: z.string(),
  targetWords: z.number().nonnegative(),
  chapterWords: z.number().positive(),
  status: z.enum(['planning', 'writing', 'paused', 'completed']),
  currentChapterId: z.string(),
  writingPreferenceRevisionId: z.string(),
  writingPreferences: z.object({
    targetAudience: z.string(), narrativePerson: z.enum(['first', 'third-limited', 'third-omniscient']), povStrategy: z.string(), tone: z.string(), pacing: z.enum(['slow', 'balanced', 'fast']), styleGuide: z.string(), forbiddenPatterns: z.array(z.string()),
  }),
  chapters: z.array(z.any()), outlineNodes: z.array(z.any()), characters: z.array(z.any()), factions: z.array(z.any()), locations: z.array(z.any()), items: z.array(z.any()), hooks: z.array(z.any()), relationships: z.array(z.any()), facts: z.array(z.any()), events: z.array(z.any()),
  graphPositions: z.record(z.string(), z.object({ x: z.number(), y: z.number() })), mapBackground: z.string().nullable(),
  worldRegions: z.array(WorldRegionSchema).default([]),
  worldMapState: z.object({ mode: z.enum(['map', 'tree']), focusRegionId: z.string().nullable() }).default({ mode: 'map', focusRegionId: null }),
  agents: z.array(z.any()), reviews: z.array(z.any()), runs: z.array(z.any()), snapshots: z.array(z.any()), branches: z.array(z.any()), activeBranchId: z.string(), createdAt: z.string(), updatedAt: z.string(),
})

export const StudioDataSchema = z.object({
  schemaVersion: z.literal(1),
  selectedProjectId: z.string().nullable(),
  projects: z.array(ProjectSchema),
})

export function parseProject(value: unknown): Project {
  return ProjectSchema.parse(value) as Project
}
