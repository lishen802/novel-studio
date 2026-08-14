import type {
  Project, Chapter, Candidate, Version, AiMessage, AppSettings,
} from '../types';
import {
  currentProject, chapters, candidates, versions, aiMessages, defaultSettings,
} from '../mock/data';

const delay = (ms: number) => new Promise((res) => setTimeout(res, ms));

// TODO: replace with GET /api/projects/current
export async function fetchCurrentProject(): Promise<Project> {
  await delay(400);
  return currentProject;
}

// TODO: replace with GET /api/projects/:id/chapters
export async function fetchChapters(): Promise<Chapter[]> {
  await delay(350);
  return chapters;
}

// TODO: replace with PUT /api/chapters/:id
export async function saveChapter(chapter: Chapter): Promise<Chapter> {
  await delay(600);
  return chapter;
}

// TODO: replace with GET /api/projects/:id/candidates
export async function fetchCandidates(): Promise<Candidate[]> {
  await delay(400);
  return candidates;
}

// TODO: replace with POST /api/candidates/:id/decide
export async function decideCandidate(
  candidateId: string,
  decision: 'accepted' | 'rejected',
): Promise<Candidate> {
  await delay(500);
  const candidate = candidates.find((c) => c.id === candidateId);
  if (!candidate) throw new Error('Candidate not found');
  return { ...candidate, status: decision };
}

// TODO: replace with GET /api/projects/:id/versions
export async function fetchVersions(): Promise<Version[]> {
  await delay(300);
  return versions;
}

// TODO: replace with POST /api/versions
export async function createVersion(label: string, note: string): Promise<Version> {
  await delay(700);
  return {
    id: `ver-${Date.now()}`,
    projectId: currentProject.id,
    label,
    wordCount: currentProject.wordCount,
    createdAt: new Date().toISOString().slice(0, 16).replace('T', ' '),
    note,
    isTagged: true,
  };
}

// TODO: replace with GET /api/projects/:id/chat
export async function fetchChatHistory(): Promise<AiMessage[]> {
  await delay(350);
  return aiMessages;
}

// TODO: replace with POST /api/chat
export async function sendChatMessage(content: string, agent?: string): Promise<AiMessage> {
  await delay(900);
  return {
    id: `aim-${Date.now()}`,
    projectId: currentProject.id,
    role: 'ai',
    agent: (agent as any) || 'general',
    content: `我已收到你的想法：“${content.slice(0, 40)}${content.length > 40 ? '…' : ''}”。\n\n在当前原型中，AI 会基于上下文给出建议，但不会直接修改正文。你可以在「审阅与候选」中查看它生成的候选文本，并决定是否采纳。`,
    timestamp: new Date().toISOString().slice(0, 16).replace('T', ' '),
  };
}

// TODO: replace with POST /api/import
export async function importManuscript(source: string): Promise<{ ok: boolean; error?: string }> {
  await delay(2500);
  if (source.length < 20) {
    return { ok: false, error: '导入内容过短，无法识别为有效小说文本。' };
  }
  return { ok: true };
}

// TODO: replace with GET /api/settings
export async function fetchSettings(): Promise<AppSettings> {
  await delay(300);
  return defaultSettings;
}

// TODO: replace with PUT /api/settings
export async function saveSettings(settings: AppSettings): Promise<AppSettings> {
  await delay(500);
  return settings;
}
