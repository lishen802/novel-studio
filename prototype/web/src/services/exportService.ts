import { parseProject } from '../domain/schema'
import type { Project } from '../domain/types'

export function buildMarkdownExport(project: Project): string {
  return [`# ${project.title}`, '', `> ${project.genre} · ${project.summary}`, '', ...project.chapters.sort((a, b) => a.number - b.number).flatMap((chapter) => [`# 第${chapter.number}章 ${chapter.title}`, '', chapter.content, ''])].join('\n')
}

export function buildTextExport(project: Project): string {
  return [`《${project.title}》`, project.summary, '', ...project.chapters.sort((a, b) => a.number - b.number).flatMap((chapter) => [`第${chapter.number}章 ${chapter.title}`, chapter.content, ''])].join('\n')
}

export function buildProjectJson(project: Project): string {
  return JSON.stringify({ schemaVersion: 1, exportedAt: new Date().toISOString(), project }, null, 2)
}

export function parseProjectJson(json: string): { success: true; data: Project } | { success: false; error: string } {
  try {
    const value = JSON.parse(json) as { schemaVersion?: number; project?: unknown }
    if (value.schemaVersion !== 1) return { success: false, error: '不支持的项目版本' }
    return { success: true, data: parseProject(value.project) }
  } catch (error) {
    return { success: false, error: error instanceof Error ? error.message : '项目文件无效' }
  }
}

export function downloadText(fileName: string, content: string, type = 'text/plain;charset=utf-8') {
  const url = URL.createObjectURL(new Blob([content], { type }))
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = fileName
  anchor.click()
  URL.revokeObjectURL(url)
}
