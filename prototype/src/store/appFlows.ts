import type { Project, ProjectStatus } from '../types';

export interface OriginalProjectInput {
  title: string;
  genre: string;
  author: string;
}

export interface WholeBookArchiveItem {
  accepted: boolean;
  conflicts?: string[];
}

/** 创建无需模型配置的原创作品原型数据。 */
export function createOriginalProject(input: OriginalProjectInput): Project {
  return {
    id: `project-${Date.now()}`,
    title: input.title.trim(),
    subtitle: input.genre.trim() || '未分类长篇',
    author: input.author.trim() || '未署名作者',
    wordCount: 0,
    targetWordCount: 120000,
    status: 'creative',
    phase: '从一个想法开始',
    updatedAt: '刚刚',
    coverColor: '#3a4a5c',
    synopsis: '',
  };
}

/** 按作品阶段恢复作者上一次合适的工作面。 */
export function projectRouteForStatus(status: ProjectStatus): string {
  switch (status) {
    case 'creative':
      return '/cockpit';
    case 'review':
      return '/review';
    case 'finished':
      return '/versions';
    case 'writing':
    default:
      return '/chapters';
  }
}

/** 首页、应用设置和导入入口不加载项目侧栏和项目级 AI 面板。 */
export function isApplicationWorkspace(pathname: string): boolean {
  return pathname === '/' || pathname === '/settings' || pathname === '/import';
}

/** 整书档案必须整包审阅完毕且不存在阻断冲突才能确认入库。 */
export function canConfirmWholeBookArchive(items: WholeBookArchiveItem[]): boolean {
  return items.length > 0 && items.every((item) => item.accepted && (item.conflicts?.length || 0) === 0);
}
