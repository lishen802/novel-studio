import { workflowTemplate } from '../domain/demoData'
import type { AgentProfile, WorkflowRun } from '../domain/types'

export function validateAgentDiversity(agents: AgentProfile[]): string[] {
  const enabled = agents.filter((agent) => agent.enabled)
  const writer = enabled.find((agent) => agent.role === 'writer')
  const final = enabled.find((agent) => agent.role === 'final-auditor')
  const reviewers = enabled.filter((agent) => agent.role.endsWith('-auditor') && agent.role !== 'final-auditor')
  const errors: string[] = []
  if (writer && final && writer.family === final.family) errors.push('正文写手与最终审稿人不能使用同一模型家族')
  if (new Set(reviewers.map((agent) => agent.family)).size < 2) errors.push('盲审至少需要两个模型家族')
  return errors
}

export function createWorkflowRun(chapterId: string): WorkflowRun {
  return { id: crypto.randomUUID(), chapterId, status: 'idle', progress: 0, currentStep: 0, steps: structuredClone(workflowTemplate), logs: ['任务已创建'], createdAt: new Date().toISOString(), mockTokens: 0, mockCost: 0 }
}

export function advanceRun(run: WorkflowRun): WorkflowRun {
  if (['cancelled', 'completed'].includes(run.status)) return run
  const steps = run.steps.map((step, index) => ({ ...step, status: index < run.currentStep ? 'completed' as const : index === run.currentStep ? 'running' as const : step.status }))
  return { ...run, status: 'running', steps, logs: [...run.logs, `${steps[run.currentStep]?.label ?? '任务'}开始`] }
}

export function pauseRun(run: WorkflowRun): WorkflowRun {
  return run.status === 'running' ? { ...run, status: 'paused', logs: [...run.logs, '作者暂停任务'] } : run
}

export function resumeRun(run: WorkflowRun): WorkflowRun {
  return run.status === 'paused' ? { ...run, status: 'running', logs: [...run.logs, '作者继续任务'] } : run
}
