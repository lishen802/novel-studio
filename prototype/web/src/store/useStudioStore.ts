import { create } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'
import { createDemoProject, createDemoStudioData } from '../domain/demoData'
import type { Project, ProjectSnapshot, StudioData } from '../domain/types'
import { createSafeStorage, STORAGE_KEY } from './persistence'

interface NewProjectInput { title: string; genre: string; summary: string }
interface StudioState extends StudioData {
  createProject(input: NewProjectInput): string
  duplicateProject(projectId: string): string
  deleteProject(projectId: string): void
  selectProject(projectId: string): void
  updateProject(patch: Partial<Project>): void
  replaceProject(project: Project): void
  createSnapshot(name: string, kind?: ProjectSnapshot['kind']): string
  restoreSnapshot(snapshotId: string): void
  resetDemo(): void
}

function serializeForSnapshot(project: Project): string {
  return JSON.stringify({ ...project, snapshots: [] })
}

export const useStudioStore = create<StudioState>()(persist((set, get) => ({
  ...createDemoStudioData(),
  createProject(input) {
    const id = crypto.randomUUID()
    const project = createDemoProject({ id, title: input.title, genre: input.genre, summary: input.summary, chapters: [], currentChapterId: '', outlineNodes: [], characters: [], factions: [], locations: [], items: [], hooks: [], relationships: [], facts: [], events: [], reviews: [], runs: [], snapshots: [], graphPositions: {} })
    set((state) => ({ projects: [...state.projects, project], selectedProjectId: id }))
    return id
  },
  duplicateProject(projectId) {
    const source = get().projects.find((project) => project.id === projectId)
    if (!source) throw new Error('作品不存在')
    const id = crypto.randomUUID()
    const copy = structuredClone(source)
    copy.id = id
    copy.title = `${source.title}（副本）`
    copy.createdAt = new Date().toISOString()
    copy.updatedAt = copy.createdAt
    set((state) => ({ projects: [...state.projects, copy], selectedProjectId: id }))
    return id
  },
  deleteProject(projectId) {
    set((state) => {
      const projects = state.projects.filter((project) => project.id !== projectId)
      return { projects, selectedProjectId: state.selectedProjectId === projectId ? projects[0]?.id ?? null : state.selectedProjectId }
    })
  },
  selectProject(projectId) { set({ selectedProjectId: projectId }) },
  updateProject(patch) {
    set((state) => ({ projects: state.projects.map((project) => project.id === state.selectedProjectId ? { ...project, ...patch, updatedAt: new Date().toISOString() } : project) }))
  },
  replaceProject(next) {
    set((state) => ({ projects: state.projects.map((project) => project.id === next.id ? next : project) }))
  },
  createSnapshot(name, kind = 'manual') {
    const project = get().projects.find((item) => item.id === get().selectedProjectId)
    if (!project) throw new Error('未选择作品')
    const id = crypto.randomUUID()
    const snapshot: ProjectSnapshot = { id, name, kind, createdAt: new Date().toISOString(), branchId: project.activeBranchId, payload: serializeForSnapshot(project), summary: `${project.chapters.length} 章 · ${project.characters.length} 个角色` }
    get().updateProject({ snapshots: [...project.snapshots, snapshot] })
    return id
  },
  restoreSnapshot(snapshotId) {
    const project = get().projects.find((item) => item.id === get().selectedProjectId)
    const target = project?.snapshots.find((snapshot) => snapshot.id === snapshotId)
    if (!project || !target) throw new Error('快照不存在')
    const safety: ProjectSnapshot = { id: crypto.randomUUID(), name: '恢复前快照', kind: 'pre-restore', createdAt: new Date().toISOString(), branchId: project.activeBranchId, payload: serializeForSnapshot(project), summary: '自动保存恢复前状态' }
    const restored = JSON.parse(target.payload) as Project
    restored.snapshots = [...project.snapshots, safety]
    get().replaceProject(restored)
  },
  resetDemo() { set(createDemoStudioData()) },
}), {
  name: STORAGE_KEY,
  version: 1,
  storage: createJSONStorage(() => createSafeStorage(localStorage)),
  partialize: (state) => ({ schemaVersion: state.schemaVersion, selectedProjectId: state.selectedProjectId, projects: state.projects }),
}))
