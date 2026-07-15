import { createDemoProject } from '../domain/demoData'
import { advanceRun, createWorkflowRun, pauseRun, resumeRun, validateAgentDiversity } from './agentService'

test('requires heterogeneous blind reviewers', () => {
  const project = createDemoProject()
  const agents = project.agents.map((agent) =>
    agent.role.includes('auditor') ? { ...agent, family: 'openai' as const, provider: 'OpenAI' } : agent,
  )
  expect(validateAgentDiversity(agents)).toContain('盲审至少需要两个模型家族')
})

test('pauses and resumes a running workflow', () => {
  const run = advanceRun(createWorkflowRun('demo-chapter-38'))
  expect(pauseRun(run).status).toBe('paused')
  expect(resumeRun(pauseRun(run)).status).toBe('running')
})
