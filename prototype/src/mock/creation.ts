import type { RevisionCard } from '../components/creation/RevisionPanel';

// 创意构建阶段的本地演示数据：逐项审阅、候选版本、结构集合与修订结果。
// 全部内容只存在于当前页面的本地状态里，不调用模型，也不写入正式作品档案。

// —— 逐项审阅 ——
export type ReviewScope = 'section' | 'entity' | 'roadmap';

export interface ReviewUnitView {
  id: string;
  scope: ReviewScope;
  group: string;
  title: string;
  summary: string;
  accepted: boolean;
  rejected: boolean;
}

export const reviewScopes: ReviewScope[] = ['section', 'entity', 'roadmap'];

export const reviewScopeLabels: Record<ReviewScope, string> = {
  section: '作品方案分节',
  entity: '人物世界实体',
  roadmap: '路线图节点',
};

function unit(
  id: string,
  scope: ReviewScope,
  group: string,
  title: string,
  summary: string,
  state: 'accepted' | 'rejected' | 'unreviewed',
): ReviewUnitView {
  return { id, scope, group, title, summary, accepted: state === 'accepted', rejected: state === 'rejected' };
}

// 当前候选带已接受、已排除与未审阅三种状态，覆盖方案分节、人物世界实体与路线图节点三类单位。
export const initialReviewUnits: ReviewUnitView[] = [
  unit('position', 'section', '作品方案', '故事定位', '民国末期江心孤岛，悬疑为明线、信约为暗线。', 'accepted'),
  unit('character', 'section', '作品方案', '人物弧', '沈知秋从旁观者到必须点燃灯塔的人。', 'unreviewed'),
  unit('world', 'section', '作品方案', '世界规则', '灯塔每夜须有人值守，超自然现象须有现实解释。', 'unreviewed'),
  unit('mainplot', 'section', '作品方案', '主线', '登岛调查 → 发现书信 → 登上灯塔 → 与真凶对峙。', 'unreviewed'),
  unit('subplot', 'section', '作品方案', '支线', '阿沅寻父线与周砚白信约线。', 'unreviewed'),
  unit('ending', 'section', '作品方案', '结局', '真相揭晓后继续守护灯塔，开放式收尾。', 'unreviewed'),
  unit('forbidden', 'section', '作品方案', '禁写项', '不引入真实鬼魂，不将悬疑归结为精神病。', 'rejected'),
  unit('char-shen', 'entity', '人物', '沈知秋', '归国画师，冷静观察者，因委托回到故乡岛屿。', 'accepted'),
  unit('char-yuan', 'entity', '人物', '阿沅', '渔家少女，为寻父失踪真相而同行。', 'unreviewed'),
  unit('char-zhou', 'entity', '人物', '周砚白', '前任守塔人，十七封信的收件人。', 'unreviewed'),
  unit('fac-shanghui', 'entity', '势力', '临江商会', '控制沿江航运与灯塔修缮经费的商业联盟。', 'unreviewed'),
  unit('loc-jiangxin', 'entity', '地点', '江心岛', '岛东礁石滩、岛西渔村，灯塔位于岛北高地。', 'unreviewed'),
  unit('rule-duty', 'entity', '世界规则', '守塔人的规矩', '守夜、点灯、记录风向与水位，离任时亲手交接灯芯。', 'unreviewed'),
  unit('rule-lamp', 'entity', '世界规则', '雾中灯语', '浓雾中的闪烁节奏可传递讯息，被视为迷信。', 'unreviewed'),
  unit('v1', 'roadmap', '卷', '第一卷 · 归途', '调查线铺开，主角登岛建立关系。', 'unreviewed'),
  unit('a1', 'roadmap', '故事弧', '追查旧信', '从收到来信到确认周砚白失踪。', 'accepted'),
  unit('v2', 'roadmap', '卷', '第二卷 · 归途', '兑现承诺，回收全部伏笔。', 'unreviewed'),
  unit('a2', 'roadmap', '故事弧', '兑现承诺', '父辈信约落地，灯塔重新点亮。', 'unreviewed'),
  unit('c1', 'roadmap', '近期章节', '来信（第一卷）', '沈知秋在石屋抽屉里发现十七封信。', 'unreviewed'),
  unit('c2', 'roadmap', '近期章节', '来信（第二卷）', '第二卷开篇再次收到同样的信。', 'rejected'),
];

// —— 候选版本 ——
export type CandidateOrigin = 'model' | 'author';
export type CandidateStatus = 'draft' | 'selected' | 'confirmed' | 'superseded' | 'stale' | 'discarded';
export type CandidateQuality = '' | 'passed' | 'needs_review' | 'blocked';

export interface CandidateVersionView {
  candidateId: string;
  version: number;
  origin: CandidateOrigin;
  status: CandidateStatus;
  quality: CandidateQuality;
  summary: string;
  createdAt: string;
  current: boolean;
}

export interface CandidateCompareDiff {
  path: string;
  label: string;
  group: string;
  before: string;
  after: string;
}

export interface CandidateCompareView {
  baseCandidateId: string;
  baseVersion: number;
  versions: Array<{
    candidateId: string;
    version: number;
    origin: CandidateOrigin;
    status: CandidateStatus;
    current: boolean;
    diff: CandidateCompareDiff[];
  }>;
}

export const initialCandidateVersions: CandidateVersionView[] = [
  {
    candidateId: 'cand-v3',
    version: 3,
    origin: 'model',
    status: 'confirmed',
    quality: 'passed',
    summary: '按独立审阅意见收敛了世界规则的表述，并把灯语解释收紧为灯号密码。',
    createdAt: '2026-08-12T21:43:00+08:00',
    current: true,
  },
  {
    candidateId: 'cand-v2',
    version: 2,
    origin: 'model',
    status: 'superseded',
    quality: 'needs_review',
    summary: '补充阿沅的同行动机与灯语线索，独立审阅仍要求收紧规则边界。',
    createdAt: '2026-08-11T09:20:00+08:00',
    current: false,
  },
  {
    candidateId: 'cand-v1',
    version: 1,
    origin: 'author',
    status: 'discarded',
    quality: 'blocked',
    summary: '作者手写初稿转写：阿沅只在结尾出现，灯语未收录。',
    createdAt: '2026-08-09T15:05:00+08:00',
    current: false,
  },
];

// 每个版本保存字段原值，比较时按基线逐字段求差，基线自身永远显示「与基线一致」。
const versionSnapshots: Record<string, Array<{ path: string; label: string; group: string; value: string }>> = {
  'cand-v1': [
    { path: '/proposal/sections/@character/content', label: '人物弧', group: '作品方案分节', value: '沈知秋独自完成调查，阿沅只在结尾出现一次。' },
    { path: '/world_rules/@lantern_language', label: '雾中灯语', group: '世界规则', value: '（本版本未收录）' },
  ],
  'cand-v2': [
    { path: '/proposal/sections/@character/content', label: '人物弧', group: '作品方案分节', value: '沈知秋与阿沅共同完成调查，两人不构成爱情线。' },
    { path: '/world_rules/@lantern_language', label: '雾中灯语', group: '世界规则', value: '老人口中的迷信传说，暂不解释。' },
  ],
  'cand-v3': [
    { path: '/proposal/sections/@character/content', label: '人物弧', group: '作品方案分节', value: '沈知秋与阿沅共同完成调查，两人不构成爱情线。' },
    { path: '/world_rules/@lantern_language', label: '雾中灯语', group: '世界规则', value: '雾中灯语实为守塔人与渔船约定的灯号密码。' },
  ],
};

export function compareCandidateVersions(baseCandidateId: string): CandidateCompareView {
  const base = initialCandidateVersions.find((item) => item.candidateId === baseCandidateId)
    ?? initialCandidateVersions.find((item) => item.current)!;
  const baseFields = versionSnapshots[base.candidateId] ?? [];
  return {
    baseCandidateId: base.candidateId,
    baseVersion: base.version,
    versions: initialCandidateVersions.map((version) => {
      const fields = versionSnapshots[version.candidateId] ?? [];
      const diff: CandidateCompareDiff[] = [];
      if (version.candidateId !== base.candidateId) {
        for (const field of fields) {
          const before = baseFields.find((item) => item.path === field.path)?.value ?? '';
          if (before !== field.value) diff.push({ path: field.path, label: field.label, group: field.group, before, after: field.value });
        }
        for (const field of baseFields) {
          if (!fields.some((item) => item.path === field.path)) {
            diff.push({ path: field.path, label: field.label, group: field.group, before: field.value, after: '' });
          }
        }
      }
      return {
        candidateId: version.candidateId,
        version: version.version,
        origin: version.origin,
        status: version.status,
        current: version.current,
        diff,
      };
    }),
  };
}

// —— 结构增删 ——
export interface StructureEntityView {
  key: string;
  title: string;
}

export interface StructureCollectionView {
  collection: string;
  title: string;
  entities: StructureEntityView[];
}

export const initialStructureCollections: StructureCollectionView[] = [
  { collection: 'characters', title: '人物', entities: [{ key: 'char-1', title: '沈知秋' }, { key: 'char-2', title: '周砚白' }] },
  { collection: 'relationships', title: '人物关系', entities: [{ key: 'rel-1', title: '沈知秋 受教于 周砚白' }] },
  { collection: 'factions', title: '势力', entities: [{ key: '江心渔村', title: '江心渔村' }, { key: '临江商会', title: '临江商会' }] },
  { collection: 'locations', title: '地点', entities: [{ key: '江心岛', title: '江心岛' }, { key: '雾中灯塔', title: '雾中灯塔' }] },
  { collection: 'world_rules', title: '世界规则', entities: [{ key: '守塔人的规矩', title: '守塔人的规矩' }] },
  { collection: 'volumes', title: '卷', entities: [{ key: 'vol-1', title: '第一卷 · 归途' }] },
  { collection: 'arcs', title: '故事弧', entities: [{ key: 'arc-1', title: '追查旧信' }] },
  { collection: 'near_term_chapters', title: '近期章节', entities: [{ key: 'ch-1', title: '第一章 · 雾起' }] },
];

// 实体标题取自与候选载荷一致的字段；界面只展示中文标题，标识用于增删。
const entityTitleFields: Record<string, string[]> = {
  characters: ['name'],
  relationships: ['label'],
  factions: ['name'],
  locations: ['name'],
  world_rules: ['name'],
  volumes: ['title'],
  arcs: ['title'],
  near_term_chapters: ['title'],
};

export function structureEntityTitle(collection: string, value: Record<string, unknown>, entityKey: string): string {
  for (const key of entityTitleFields[collection] ?? []) {
    const raw = value[key];
    if (typeof raw === 'string' && raw.trim()) return raw.trim();
  }
  if (collection === 'relationships') {
    const source = String(value.source_character_ref ?? '').trim();
    const target = String(value.target_character_ref ?? '').trim();
    if (source || target) return `${source || '未指定'} → ${target || '未指定'}`;
  }
  return entityKey;
}

export function structureCollectionTitle(collection: string): string {
  return initialStructureCollections.find((item) => item.collection === collection)?.title ?? collection;
}

// —— 已完成的修订结果（1 条文本差异 + 1 条新增实体差异）——
export const demoRevisionAction: RevisionCard = {
  actionId: 'rev-demo-1',
  argumentsHash: 'demo-hash-1',
  idempotencyKey: 'demo-key-1',
  candidateId: 'demo-1',
  version: 1,
  state: 'succeeded',
  instruction: '让灯芯伏笔提前呼应，并为岛上补一位渔家少女。',
  targets: [{ path: '/directions/@a/hook', label: '卖点', group: '故事方向 · A', value: '一座雾中灯塔，三任守塔人接连失踪。', locked: false }],
  preserved: [],
  cost: { estimatedCalls: 2, maxRunTokens: 30000, runBudgetUnlimited: false, roleModels: [{ role: 'planner', provider: '示例服务', model: '修订模型' }, { role: 'continuity_reviewer', provider: '示例服务', model: '审阅模型' }] },
  runId: 'run-demo-1',
  quality: 'needs_review',
  summary: '修订已完成并通过格式修复，独立审阅建议采用后按待复核候选处理。',
  stale: false,
  canApply: true,
  applied: false,
  diffs: [
    {
      path: '/directions/@a/hook',
      label: '卖点',
      group: '故事方向 · A',
      kind: 'text',
      before: '一座雾中灯塔，三任守塔人接连失踪。',
      after: '一座雾中灯塔，三任守塔人接连失踪；灯芯上的棉布擦痕成为贯穿全书的线索。',
    },
    {
      path: '/characters/@char-3',
      label: '新增 阿沅',
      group: '人物',
      kind: 'add_entity',
      before: '',
      after: JSON.stringify({ id: 'char-3', name: '阿沅', role: 'supporting', summary: '岛上渔村的少女，常帮守塔人送补给，熟悉岛上旧闻。' }, null, 2),
    },
  ],
};
