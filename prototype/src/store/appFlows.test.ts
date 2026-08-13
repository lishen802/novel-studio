import { describe, expect, it } from 'vitest';
import {
  canConfirmWholeBookArchive,
  createOriginalProject,
  isApplicationWorkspace,
  projectRouteForStatus,
} from './appFlows';

describe('原型应用流程', () => {
  it('新建原创作品不会要求先配置模型，并进入创意阶段', () => {
    const project = createOriginalProject({
      title: '射雕英雄传',
      genre: '武侠',
      author: '作者',
    });

    expect(project.status).toBe('creative');
    expect(project.phase).toBe('从一个想法开始');
    expect(project.title).toBe('射雕英雄传');
  });

  it('最近作品会回到与其阶段对应的工作面', () => {
    expect(projectRouteForStatus('creative')).toBe('/cockpit');
    expect(projectRouteForStatus('writing')).toBe('/chapters');
    expect(projectRouteForStatus('review')).toBe('/review');
  });

  it('应用设置和首页不承载当前项目的导航壳', () => {
    expect(isApplicationWorkspace('/')).toBe(true);
    expect(isApplicationWorkspace('/settings')).toBe(true);
    expect(isApplicationWorkspace('/chapters')).toBe(false);
  });

  it('整书档案只有全部分类确认且阻断冲突解决后才能入库', () => {
    expect(canConfirmWholeBookArchive([
      { accepted: true, conflicts: [] },
      { accepted: true, conflicts: [] },
    ])).toBe(true);
    expect(canConfirmWholeBookArchive([
      { accepted: true, conflicts: [] },
      { accepted: false, conflicts: [] },
    ])).toBe(false);
    expect(canConfirmWholeBookArchive([
      { accepted: true, conflicts: ['人物名称冲突'] },
    ])).toBe(false);
  });
});
