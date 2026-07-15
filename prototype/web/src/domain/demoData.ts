import type { AgentProfile, Project, StudioData, WorkflowStep } from './types'

const now = '2026-07-15T00:00:00.000Z'

export const workflowTemplate: WorkflowStep[] = [
  ['plan', '章节规划', 'agent-planner'],
  ['context', '上下文装配', 'agent-context'],
  ['write', '正文写作', 'agent-writer'],
  ['review', '四路盲审', 'agent-continuity'],
  ['arbitrate', '问题仲裁', 'agent-arbiter'],
  ['revise', '定向修订', 'agent-reviser'],
  ['extract', '事实提取', 'agent-extractor'],
  ['final-review', '最终审校', 'agent-final'],
].map(([id, label, agentId]) => ({ id: id as WorkflowStep['id'], label, agentId, status: 'pending' }))

export function createDemoAgents(): AgentProfile[] {
  return [
    ['agent-planner', '剧情规划师', 'planner', 'OpenAI', 'openai', 'gpt-5.2', 0.45],
    ['agent-context', '上下文编辑', 'context-curator', '本地模型', 'local', 'qwen3-32b', 0.1],
    ['agent-writer', '正文写手', 'writer', 'Anthropic', 'anthropic', 'claude-sonnet-4.5', 0.8],
    ['agent-continuity', '连续性审查', 'continuity-auditor', 'OpenAI', 'openai', 'gpt-5.2', 0.1],
    ['agent-character', '人物审查', 'character-auditor', 'Google', 'google', 'gemini-3-pro', 0.2],
    ['agent-causality', '因果审查', 'causality-auditor', 'DeepSeek', 'deepseek', 'deepseek-reasoner', 0.1],
    ['agent-pacing', '节奏审查', 'pacing-auditor', 'Anthropic', 'anthropic', 'claude-haiku-4.5', 0.3],
    ['agent-arbiter', '审稿仲裁', 'arbiter', 'Google', 'google', 'gemini-3-pro', 0.1],
    ['agent-reviser', '修订编辑', 'reviser', 'OpenAI', 'openai', 'gpt-5.2', 0.35],
    ['agent-extractor', '事实提取', 'fact-extractor', '本地模型', 'local', 'qwen3-32b', 0],
    ['agent-final', '最终审校', 'final-auditor', 'DeepSeek', 'deepseek', 'deepseek-reasoner', 0.1],
  ].map(([id, name, role, provider, family, model, temperature]) => ({
    id: String(id), name: String(name), role: role as AgentProfile['role'], provider: String(provider), family: family as AgentProfile['family'], model: String(model), temperature: Number(temperature), enabled: true, instructions: `${name}只根据证据输出结构化结论。`,
  }))
}

export function createDemoProject(overrides: Partial<Project> = {}): Project {
  const chapters = [
    { id: 'demo-chapter-36', number: 36, title: '雾中的来信', summary: '沈砚收到失踪父亲的匿名来信。', content: '潮声像一把钝刀，在凌晨两点反复刮过旧港的堤岸。沈砚拆开那封没有邮戳的信，看见父亲失踪前惯用的蓝黑墨水。', status: 'approved' as const, locked: false, lockedPassages: [], scenes: [], updatedAt: now },
    { id: 'demo-chapter-37', number: 37, title: '灯塔会', summary: '许棠带沈砚进入地下档案室。', content: '许棠把钥匙压在掌心，领他穿过废弃钟楼。档案室里只有一盏绿灯，照着港务集团三十年前的航线图。', status: 'review' as const, locked: false, lockedPassages: [], scenes: [], updatedAt: now },
    { id: 'demo-chapter-38', number: 38, title: '逆潮', summary: '调查小组准备潜入北堤仓库。', content: '退潮前还有四十分钟。沈砚站在北堤仓库外，听见铁门后传来第三个人的脚步。', status: 'draft' as const, locked: false, lockedPassages: [], scenes: [], updatedAt: now },
  ]
  const project: Project = {
    id: 'demo-project', title: '雾港来信', genre: '悬疑 / 都市奇谭', summary: '记者沈砚追查父亲失踪真相，逐渐发现雾港的记忆会被潮汐改写。', targetWords: 520000, chapterWords: 3200, status: 'writing', currentChapterId: 'demo-chapter-38', writingPreferenceRevisionId: 'pref-1',
    writingPreferences: { targetAudience: '18–35 岁类型小说读者', narrativePerson: 'third-limited', povStrategy: '以沈砚为主，关键章节切换许棠', tone: '冷峻、潮湿、克制', pacing: 'balanced', styleGuide: '短句推进悬念，避免解释性总结；意象集中在潮汐、锈蚀与灯光。', forbiddenPatterns: ['突然想到', '不禁倒吸一口凉气'] },
    chapters,
    outlineNodes: [
      { id: 'outline-book', parentId: null, level: 'book', title: '雾港来信', summary: '寻找父亲与夺回城市记忆。', order: 1, status: 'active', linkedEntityIds: [] },
      { id: 'outline-volume', parentId: 'outline-book', level: 'volume', title: '第一卷：失踪航线', summary: '发现港务集团隐藏的旧航线。', order: 1, status: 'active', linkedEntityIds: ['faction-port'] },
      { id: 'outline-arc', parentId: 'outline-volume', level: 'arc', title: '旧港调查弧', summary: '匿名信、档案室和北堤仓库形成证据链。', order: 1, status: 'active', linkedEntityIds: ['char-shen', 'char-xu'] },
      { id: 'outline-chapter-38', parentId: 'outline-arc', level: 'chapter', title: '第38章 逆潮', summary: '潜入仓库并发现第三方。', order: 38, status: 'active', linkedEntityIds: ['loc-warehouse'] },
      { id: 'outline-scene-38-1', parentId: 'outline-chapter-38', level: 'scene', title: '仓库外等待', summary: '脚步声打破计划。', order: 1, status: 'planned', linkedEntityIds: ['char-shen'] },
    ],
    characters: [
      { id: 'char-shen', name: '沈砚', role: 'protagonist', summary: '调查记者，执着于可验证的事实。', personality: ['克制', '敏锐'], motivation: '找到父亲并证明雾港记忆篡改存在', secrets: ['他也曾接受过一次记忆修订'], currentStatus: '调查北堤仓库', factionIds: ['faction-lighthouse'], targetShare: 46, arc: [{ chapter: 36, beat: '接受匿名信' }, { chapter: 60, beat: '怀疑自身记忆' }], knowledge: [] },
      { id: 'char-xu', name: '许棠', role: 'supporting', summary: '档案修复师，灯塔会成员。', personality: ['果断', '谨慎'], motivation: '保存城市被删除的历史', secrets: ['母亲曾参与潮汐实验'], currentStatus: '协助潜入', factionIds: ['faction-lighthouse'], targetShare: 28, arc: [{ chapter: 37, beat: '向沈砚开放档案室' }], knowledge: [] },
      { id: 'char-lu', name: '陆放', role: 'antagonist', summary: '港务集团安全主管。', personality: ['礼貌', '冷酷'], motivation: '维持雾港秩序', secrets: ['受命保护沈砚'], currentStatus: '监控调查组', factionIds: ['faction-port'], targetShare: 16, arc: [{ chapter: 40, beat: '第一次正面交锋' }], knowledge: [] },
    ],
    factions: [
      { id: 'faction-lighthouse', name: '灯塔会', goal: '保存未被改写的城市历史', resources: ['地下档案室', '旧灯塔通讯网'], stance: '秘密协助沈砚', memberIds: ['char-shen', 'char-xu'] },
      { id: 'faction-port', name: '港务集团', goal: '控制航线与潮汐实验', resources: ['港区安保', '市政关系'], stance: '封锁旧港真相', memberIds: ['char-lu'] },
    ],
    locations: [
      { id: 'loc-oldport', name: '旧港堤岸', summary: '匿名信出现的地方。', region: '西港区', x: 210, y: 410, visitChapterIds: ['demo-chapter-36'], eventIds: ['event-letter'] },
      { id: 'loc-lighthouse', name: '废弃钟楼', summary: '灯塔会入口。', region: '中城区', x: 510, y: 260, visitChapterIds: ['demo-chapter-37'], eventIds: ['event-archive'] },
      { id: 'loc-warehouse', name: '北堤仓库', summary: '港务集团封存实验器材的仓库。', region: '北堤', x: 760, y: 180, visitChapterIds: ['demo-chapter-38'], eventIds: ['event-infiltrate'] },
    ],
    items: [{ id: 'item-key', name: '旧港钥匙', holderEntityId: 'char-xu', status: '使用中', origin: '许棠母亲留下', transfers: [] }],
    hooks: [
      { id: 'hook-letter', title: '蓝黑墨水的匿名信', plantedChapterId: 'demo-chapter-36', progressChapterIds: ['demo-chapter-38'], deadlineChapter: 55, status: 'developing', evidence: ['父亲惯用墨水'] },
      { id: 'hook-footsteps', title: '仓库里的第三个人', plantedChapterId: 'demo-chapter-38', progressChapterIds: [], deadlineChapter: 42, status: 'planted', evidence: ['铁门后的脚步'] },
    ],
    relationships: [
      { id: 'rel-shen-xu', sourceId: 'char-shen', targetId: 'char-xu', type: '互相信任', strength: 72, publicState: '调查搭档', trueState: '彼此保留关键秘密', startChapter: 36 },
      { id: 'rel-xu-light', sourceId: 'char-xu', targetId: 'faction-lighthouse', type: '核心成员', strength: 92, publicState: '无', trueState: '档案保管人', startChapter: 18 },
      { id: 'rel-lu-port', sourceId: 'char-lu', targetId: 'faction-port', type: '效忠', strength: 80, publicState: '安全主管', trueState: '实验监督者', startChapter: 1 },
    ],
    facts: [
      { id: 'fact-letter', text: '匿名信使用沈砚父亲惯用的蓝黑墨水', category: 'evidence', status: 'text-evidenced', confidence: 0.95, evidence: [{ chapterId: 'demo-chapter-36', quote: '父亲失踪前惯用的蓝黑墨水' }], scopeEntityIds: ['char-shen'] },
      { id: 'fact-tide', text: '雾港潮汐会影响人的记忆', category: 'world-rule', status: 'ai-inferred', confidence: 0.68, evidence: [{ chapterId: 'demo-chapter-37', quote: '三十年前的航线图' }], scopeEntityIds: [] },
      { id: 'fact-xu', text: '许棠是灯塔会档案保管人', category: 'identity', status: 'author-confirmed', confidence: 1, evidence: [{ chapterId: 'demo-chapter-37', quote: '领他穿过废弃钟楼' }], scopeEntityIds: ['char-xu'] },
      { id: 'fact-conflict', text: '沈砚父亲于十年前死亡', category: 'history', status: 'conflicted', confidence: 0.5, evidence: [{ chapterId: 'demo-chapter-36', quote: '没有邮戳的信' }], scopeEntityIds: ['char-shen'] },
    ],
    events: [
      { id: 'event-letter', title: '收到匿名信', summary: '主线启动', cause: '有人希望沈砚重启调查', result: '沈砚前往档案室', storyTime: '2009-07-10 02:00', narrativeOrder: 1, status: 'occurred', participantIds: ['char-shen'], factionIds: [], locationId: 'loc-oldport', chapterIds: ['demo-chapter-36'] },
      { id: 'event-archive', title: '进入地下档案室', summary: '发现旧航线', cause: '许棠验证匿名信', result: '锁定北堤仓库', storyTime: '2009-07-11 23:20', narrativeOrder: 2, status: 'occurred', participantIds: ['char-shen', 'char-xu'], factionIds: ['faction-lighthouse'], locationId: 'loc-lighthouse', chapterIds: ['demo-chapter-37'] },
      { id: 'event-infiltrate', title: '潜入北堤仓库', summary: '遭遇未知第三人', cause: '旧航线指向仓库', result: '计划被打乱', storyTime: '2009-07-12 03:10', narrativeOrder: 3, status: 'planned', participantIds: ['char-shen', 'char-xu'], factionIds: ['faction-port'], locationId: 'loc-warehouse', chapterIds: ['demo-chapter-38'] },
    ],
    graphPositions: { 'char-shen': { x: 240, y: 220 }, 'char-xu': { x: 520, y: 180 }, 'char-lu': { x: 760, y: 330 }, 'faction-lighthouse': { x: 430, y: 420 }, 'faction-port': { x: 760, y: 500 } },
    mapBackground: null,
    agents: createDemoAgents(),
    reviews: [
      { id: 'review-1', chapterId: 'demo-chapter-38', reviewerRole: 'continuity-auditor', reviewerModel: 'gpt-5.2', severity: 'warning', category: '时间连续性', description: '第37章到第38章之间缺少从钟楼前往北堤的时间交代。', evidence: [{ source: '第37章', quote: '档案室里只有一盏绿灯' }, { source: '第38章', quote: '退潮前还有四十分钟' }], suggestion: '增加一句离开钟楼时的时间锚点。', disposition: 'open' },
      { id: 'review-2', chapterId: 'demo-chapter-38', reviewerRole: 'character-auditor', reviewerModel: 'gemini-3-pro', severity: 'suggestion', category: '人物动机', description: '许棠在高风险行动中的个人动机可以更明确。', evidence: [{ source: '角色设定', quote: '保存城市被删除的历史' }], suggestion: '让许棠主动提及仓库与母亲的关联。', disposition: 'open' },
    ],
    runs: [], snapshots: [], branches: [{ id: 'branch-main', name: '主线' }], activeBranchId: 'branch-main', createdAt: now, updatedAt: now,
  }
  return { ...project, ...overrides }
}

export function createDemoStudioData(): StudioData {
  return { schemaVersion: 1, selectedProjectId: 'demo-project', projects: [createDemoProject()] }
}
