import type {
  Project, Chapter, Character, Faction, WorldEntry,
  TimelineEvent, Foreshadowing, Candidate, Version, AiMessage, AppSettings,
} from '../types';

export const currentProject: Project = {
  id: 'proj-01',
  title: '雾中灯塔',
  subtitle: '一部长篇悬疑小说',
  author: '林未然',
  wordCount: 48620,
  targetWordCount: 120000,
  status: 'writing',
  phase: '正式写作阶段',
  updatedAt: '2026-08-12 21:43',
  coverColor: '#3a4a5c',
  synopsis: '民国末期，一座江心灯塔在浓雾中接连发生守塔人失踪事件。旅居归来的青年画师沈知秋受托前往调查，却发现每一盏灯背后都藏着一封未寄出的信，而雾中似乎有人一直在等他。',
};

export const recentProjects: Project[] = [
  currentProject,
  {
    id: 'proj-02',
    title: '夏末书简',
    subtitle: '都市情感长篇',
    author: '林未然',
    wordCount: 12400,
    targetWordCount: 90000,
    status: 'creative',
    phase: '创意构建阶段',
    updatedAt: '2026-08-10 14:22',
    coverColor: '#5c4a3a',
    synopsis: '',
  },
  {
    id: 'proj-03',
    title: '银海拾遗',
    subtitle: '科幻中篇',
    author: '林未然',
    wordCount: 35200,
    targetWordCount: 60000,
    status: 'review',
    phase: '作者审阅阶段',
    updatedAt: '2026-08-08 09:15',
    coverColor: '#3a5c4a',
    synopsis: '',
  },
];

export const defaultSettings: AppSettings = {
  models: [
    {
      id: 'model-openai',
      name: '主力 API 模型',
      provider: 'OpenAI 兼容',
      modelName: 'gpt-4.1-mini',
      apiKeyConfigured: false,
    },
    {
      id: 'model-claude',
      name: '审阅模型',
      provider: 'Anthropic',
      modelName: 'claude-sonnet',
      apiKeyConfigured: false,
    },
  ],
  activeModelId: 'model-openai',
  creativeProfiles: [
    {
      id: 'profile-default',
      name: '长篇创作默认方案',
      version: 1,
      published: true,
      roleModels: {
        planning: 'model-openai',
        writing: 'model-openai',
        review: 'model-claude',
        memory: 'model-openai',
        impact: 'model-claude',
      },
    },
  ],
  defaultCreativeProfileId: 'profile-default',
  writingPlan: '长篇创作默认方案',
  theme: 'light',
  fontSize: 18,
  lineHeight: 1.9,
  canvasColorId: 'warm-white',
  marginSize: 'medium',
  indent: true,
};

export const chapters: Chapter[] = [
  {
    id: 'ch-01',
    projectId: 'proj-01',
    title: '第一章 · 雾起',
    order: 1,
    wordCount: 4820,
    status: 'approved',
    content: `沈知秋抵达江心岛时，雾正从水面升起。

码头上只剩一盏马灯，被风推着晃。船夫不肯靠岸，只把行李扔在栈桥尽头，低声说：“灯塔上的灯，昨夜没亮。”

他抬眼望去，灰色的塔身隐没在雾中，像一只沉默的巨兽。三日前收到的那封信还在内袋里，字迹潦草，末尾只有一句：“请来接替我。”

登岛的木板路湿滑，两侧芦苇高过人头。沈知秋走了约莫一刻钟，才看见灯塔下那间石屋。门没锁，推开时发出干涩的呻吟。屋内陈设简单：一张床、一张桌、半架旧书，以及满墙用图钉固定的海图。海图上画满了红圈， latest 一个圈在灯塔东北方约两里处——那里只有水。`,
    summary: '沈知秋抵达江心岛，发现灯塔昨夜未亮，屋内海图留有神秘红圈。',
    aiNotes: '开篇气氛营造较好，建议增加一句对气味的描写以强化沉浸感。',
  },
  {
    id: 'ch-02',
    projectId: 'proj-01',
    title: '第二章 · 未寄出的信',
    order: 2,
    wordCount: 5150,
    status: 'approved',
    content: `石屋抽屉里有十七封信。收件人都是同一个人：周砚白。`,
    summary: '发现前任守塔人未寄出的信件，揭示周砚白这一关键人物。',
    aiNotes: '',
  },
  {
    id: 'ch-03',
    projectId: 'proj-01',
    title: '第三章 · 灯芯',
    order: 3,
    wordCount: 4200,
    status: 'review',
    content: `灯芯是新的。这说明有人最近上过塔。`,
    summary: '沈知秋上塔检查，发现灯芯被更换，怀疑岛上还有其他人。',
    aiNotes: '此处可加入一段对灯塔内部螺旋楼梯的感官描写。',
  },
  {
    id: 'ch-04',
    projectId: 'proj-01',
    title: '第四章 · 雾中人影',
    order: 4,
    wordCount: 0,
    status: 'empty',
    content: '',
    summary: '',
    aiNotes: '',
  },
  {
    id: 'ch-05',
    projectId: 'proj-01',
    title: '第五章 · 潮间带',
    order: 5,
    wordCount: 0,
    status: 'empty',
    content: '',
    summary: '',
    aiNotes: '',
  },
];

export const characters: Character[] = [
  {
    id: 'char-01',
    projectId: 'proj-01',
    name: '沈知秋',
    role: 'protagonist',
    tags: ['画师', '归国旅人', '观察敏锐'],
    summary: '旅居海外多年的青年画师，因受托调查灯塔事件回到故乡江心岛。性格沉静，善于从细节中还原真相。',
    appearance: '身形修长，常穿一件洗得发白的灰色长衫，右手食指有炭笔留下的薄茧。',
    motivation: '查清守塔人失踪真相，并寻找与自己童年有关的某个人。',
  },
  {
    id: 'char-02',
    projectId: 'proj-01',
    name: '周砚白',
    role: 'supporting',
    tags: ['前任守塔人', '神秘', '信件的收件人'],
    summary: '灯塔前任守塔人，在信中向沈知秋求救，但岛上无人知晓其下落。',
    appearance: '年近五十，鬓角斑白，走路微跛。',
    motivation: '守护灯塔中的某个秘密，并引导沈知秋发现真相。',
  },
  {
    id: 'char-03',
    projectId: 'proj-01',
    name: '阿沅',
    role: 'supporting',
    factionId: 'fac-01',
    tags: ['渔家少女', '向导', '消息灵通'],
    summary: '岛上渔村的少女，常帮守塔人送补给，知道许多岛上旧闻。',
    appearance: '肤色微黑，眼睛明亮，说话时带着江口音。',
    motivation: '希望解开父亲多年前在灯塔附近失踪的谜团。',
  },
  {
    id: 'char-04',
    projectId: 'proj-01',
    name: '赵掌柜',
    role: 'antagonist',
    factionId: 'fac-02',
    tags: ['商会管事', '精明', '隐瞒'],
    summary: '镇上商会派驻岛上的管事，负责灯塔修缮经费，对守塔人的事了如指掌却讳莫如深。',
    appearance: '微胖，戴圆框眼镜，惯于把算盘拨得噼啪响。',
    motivation: '掩盖商会与灯塔之间的利益往来。',
  },
];

export const factions: Faction[] = [
  {
    id: 'fac-01',
    projectId: 'proj-01',
    name: '江心渔村',
    type: 'family',
    description: '世代居住江心岛的渔民家族，以捕鱼和为灯塔运送物资为生。',
    goals: '维持岛上生计，保护祖辈留下的水域。',
    conflicts: '与镇上商会因灯塔租金和捕鱼权时有摩擦。',
  },
  {
    id: 'fac-02',
    projectId: 'proj-01',
    name: '临江商会',
    type: 'organization',
    description: '控制沿江航运与灯塔修缮经费的商业联盟。',
    goals: '压低灯塔维护成本，扩大航运控制范围。',
    conflicts: '被怀疑私吞灯塔经费并掩盖守塔人失踪真相。',
  },
];

export const worldEntries: WorldEntry[] = [
  {
    id: 'world-01',
    projectId: 'proj-01',
    category: 'geography',
    title: '江心岛',
    content: '位于江心的一处小岛，面积约半平方公里。岛东有礁石滩，岛西是渔村聚落。灯塔位于岛北高地。',
    importance: 'core',
  },
  {
    id: 'world-02',
    projectId: 'proj-01',
    category: 'history',
    title: '雾中灯塔的由来',
    content: '建于清光绪年间，最初为指引商船而设。民国后由临江商会接管维护，却屡次传出守塔人失踪的传闻。',
    importance: 'core',
  },
  {
    id: 'world-03',
    projectId: 'proj-01',
    category: 'culture',
    title: '守塔人的规矩',
    content: '守塔人须独自在塔上守夜，逢大雾天点亮主灯，并在日志中记录风向与水位。前任离任时须亲手将灯芯传给继任者。',
    importance: 'secondary',
  },
  {
    id: 'world-04',
    projectId: 'proj-01',
    category: 'magic',
    title: '雾中灯语',
    content: '岛上老人传说，浓雾中灯塔会以特定节奏闪烁，向江中某人传递讯息。该传说被视为迷信，但沈知秋发现日志中确有类似的灯号记录。',
    importance: 'secondary',
  },
];

export const timeline: TimelineEvent[] = [
  {
    id: 'tl-01',
    projectId: 'proj-01',
    date: '清光绪十二年',
    event: '江心灯塔建成',
    importance: 'major',
    linkedCharacters: [],
    linkedChapters: [],
  },
  {
    id: 'tl-02',
    projectId: 'proj-01',
    date: '民国二十三年',
    event: '临江商会接管灯塔维护',
    importance: 'major',
    linkedCharacters: ['char-04'],
    linkedChapters: [],
  },
  {
    id: 'tl-03',
    projectId: 'proj-01',
    date: '民国二十六年七月',
    event: '守塔人周砚白最后一次在日志上签名',
    importance: 'major',
    linkedCharacters: ['char-02'],
    linkedChapters: ['ch-01'],
  },
  {
    id: 'tl-04',
    projectId: 'proj-01',
    date: '民国二十六年八月',
    event: '沈知秋受托登岛调查',
    importance: 'major',
    linkedCharacters: ['char-01'],
    linkedChapters: ['ch-01'],
  },
  {
    id: 'tl-05',
    projectId: 'proj-01',
    date: '民国二十四年',
    event: '阿沅父亲在灯塔附近失踪',
    importance: 'minor',
    linkedCharacters: ['char-03'],
    linkedChapters: ['ch-04'],
  },
];

export const foreshadowing: Foreshadowing[] = [
  {
    id: 'fs-01',
    projectId: 'proj-01',
    title: '未寄出的十七封信',
    description: '抽屉中收件人均为周砚白的信件，为何从未寄出？',
    status: 'planted',
    linkedChapters: ['ch-02'],
  },
  {
    id: 'fs-02',
    projectId: 'proj-01',
    title: '海图上的红圈',
    description: '海图上最后一个红圈指向只有水的区域，是否暗示水位变化或隐藏地点？',
    status: 'growing',
    linkedChapters: ['ch-01'],
  },
  {
    id: 'fs-03',
    projectId: 'proj-01',
    title: '灯芯被更换',
    description: '新的灯芯说明有人上过塔，这个人是谁？',
    status: 'planted',
    linkedChapters: ['ch-03'],
  },
  {
    id: 'fs-04',
    projectId: 'proj-01',
    title: '雾中灯语',
    description: '老人口中的迷信传说，或许是真实传递信息的密码。',
    status: 'planted',
    linkedChapters: ['ch-04'],
  },
];

export const candidates: Candidate[] = [
  {
    id: 'cand-01',
    projectId: 'proj-01',
    chapterId: 'ch-03',
    type: 'rewrite',
    content: `灯芯是新的。灯油也只用了不到一成。这说明有人最近上过塔，而且不是匆匆一瞥——他花时间添了油、换了芯，甚至还像真正的守塔人一样，把灯罩擦了一遍。

沈知秋的手指抚过灯罩边缘，那里有一道新鲜的棉布擦痕。谁会做这些？如果周砚白已经失踪，那么替他来守灯的人，又是谁？`,
    diffFrom: `灯芯是新的。这说明有人最近上过塔。`,
    status: 'pending',
    aiReason: '增加灯油与灯罩的细节，强化“有人代替守塔”的悬疑感，同时为后续揭示身份埋下感官线索。',
    createdAt: '2026-08-12 20:15',
  },
  {
    id: 'cand-02',
    projectId: 'proj-01',
    chapterId: 'ch-04',
    type: 'continuation',
    content: `雾更浓了。

沈知秋提着马灯走下螺旋楼梯，每一步都踩在陈年的灯油气味里。走到第七级时，他忽然停住——楼下传来轻微的木板响动，像是有人正贴着墙根移动。

“谁？”

没有回答。雾气从门缝渗进来，把他的声音吞掉了一半。`,
    diffFrom: '',
    status: 'pending',
    aiReason: '延续第三章氛围，引入“雾中人影”的悬念，保持克制不写破对方身份。',
    createdAt: '2026-08-12 21:02',
  },
];

export const versions: Version[] = [
  {
    id: 'ver-01',
    projectId: 'proj-01',
    label: 'v0.1 初稿',
    wordCount: 12400,
    createdAt: '2026-07-15 10:30',
    note: '仅完成前二章初稿',
    isTagged: true,
  },
  {
    id: 'ver-02',
    projectId: 'proj-01',
    label: 'v0.2 扩展',
    wordCount: 28900,
    createdAt: '2026-07-28 16:45',
    note: '新增第三章与人物设定',
    isTagged: true,
  },
  {
    id: 'ver-03',
    projectId: 'proj-01',
    label: 'v0.3 当前工作版本',
    wordCount: 48620,
    createdAt: '2026-08-12 21:43',
    note: '完成审阅前的最新进度',
    isTagged: false,
  },
];

export const aiMessages: AiMessage[] = [
  {
    id: 'aim-01',
    projectId: 'proj-01',
    role: 'ai',
    agent: 'plot',
    content: '第三章结尾的“灯芯是新的”是一个有效的钩子。建议下一章先不揭示换灯人身份，而是通过声音或影子建立威胁感。',
    timestamp: '2026-08-12 19:30',
  },
  {
    id: 'aim-02',
    projectId: 'proj-01',
    role: 'author',
    content: '我想让沈知秋在第四章发现灯塔里有一条通往礁石滩的密道，这个设定会不会破坏悬疑节奏？',
    timestamp: '2026-08-12 19:45',
  },
  {
    id: 'aim-03',
    projectId: 'proj-01',
    role: 'ai',
    agent: 'plot',
    content: '不会破坏节奏，但建议把“发现密道”放在第四章后半段，并且让密道的入口与某个伏笔（比如海图红圈或灯语）关联起来，这样 revelation 会更有回收感。',
    timestamp: '2026-08-12 19:46',
  },
];
