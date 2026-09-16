import { cleanup, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it } from 'vitest';
import Cockpit from './Cockpit';
import { AppProvider } from '../store/AppContext';

afterEach(() => {
  cleanup();
  window.localStorage.clear();
});

// 工作台演示只使用本地状态与假数据，这里逐项验证 R4 四项交互的可见行为。
function renderCockpit() {
  return render(
    <MemoryRouter>
      <AppProvider>
        <Cockpit />
      </AppProvider>
    </MemoryRouter>,
  );
}

describe('创意构建工作台 R4 交互', () => {
  it('接受单个审阅单位后状态标记与已接受计数一起更新', async () => {
    const user = userEvent.setup();
    renderCockpit();

    expect(screen.getByText(/已接受 3 · 已排除 2 · 未审阅 15/)).toBeTruthy();

    const row = screen.getByRole('listitem', { name: '作品方案 · 主线' });
    expect(within(row).getByText('未审阅')).toBeTruthy();

    await user.click(within(row).getByRole('button', { name: '接受 主线' }));

    expect(within(row).getByText('已接受')).toBeTruthy();
    expect(screen.getByText(/已接受 4 · 已排除 2 · 未审阅 14/)).toBeTruthy();
  });

  it('全部接受把每个单位置为已接受并清空排除与未审阅', async () => {
    const user = userEvent.setup();
    renderCockpit();

    await user.click(screen.getByRole('button', { name: '全部接受（20 项）' }));

    expect(screen.getByText(/已接受 20 · 已排除 0 · 未审阅 0/)).toBeTruthy();
    expect(within(screen.getByRole('listitem', { name: '作品方案 · 禁写项' })).getByText('已接受')).toBeTruthy();
    expect(within(screen.getByRole('listitem', { name: '近期章节 · 来信（第二卷）' })).getByText('已接受')).toBeTruthy();
    expect((screen.getByRole('button', { name: '接受 故事定位' }) as HTMLButtonElement).disabled).toBe(true);
  });

  it('非当前版本用作工作基线后当前标记切换', async () => {
    const user = userEvent.setup();
    renderCockpit();

    const version2 = screen.getByRole('article', { name: '版本 2' });
    expect(within(version2).queryByText('当前工作版本')).toBeNull();
    expect(within(screen.getByRole('article', { name: '版本 3' })).getByText('当前工作版本')).toBeTruthy();

    await user.click(within(version2).getByRole('button', { name: '用作工作基线 版本 2' }));

    expect(within(screen.getByRole('article', { name: '版本 2' })).getByText('当前工作版本')).toBeTruthy();
    expect(within(screen.getByRole('article', { name: '版本 3' })).queryByText('当前工作版本')).toBeNull();
    expect(screen.queryByRole('button', { name: '用作工作基线 版本 2' })).toBeNull();
  });

  it('比较版本弹窗按基线展示修改前与修改后', async () => {
    const user = userEvent.setup();
    renderCockpit();

    await user.click(screen.getByRole('button', { name: '比较版本' }));
    const dialog = await screen.findByRole('dialog');

    expect(await within(dialog).findByText('与基线一致')).toBeTruthy();
    expect((await within(dialog).findAllByText('修改前')).length).toBeGreaterThan(0);
    expect(within(dialog).getAllByText('修改后').length).toBeGreaterThan(0);
    expect(within(dialog).getByText('老人口中的迷信传说，暂不解释。')).toBeTruthy();
    expect(within(dialog).getAllByText('雾中灯语实为守塔人与渔船约定的灯号密码。').length).toBeGreaterThan(0);
  });

  it('删除结构条目需要二次确认，确认后实体从列表消失', async () => {
    const user = userEvent.setup();
    renderCockpit();

    const characters = screen.getByLabelText('人物集合');
    expect(within(characters).getByText('周砚白')).toBeTruthy();

    await user.click(within(characters).getByRole('button', { name: '删除 人物 周砚白' }));
    const dialog = await screen.findByRole('dialog');

    // 确认之前实体仍然留在列表里。
    expect(screen.getByRole('button', { name: '删除 人物 周砚白' })).toBeTruthy();

    await user.click(within(dialog).getByRole('button', { name: '确认删除 人物 周砚白' }));

    expect(screen.queryByRole('button', { name: '删除 人物 周砚白' })).toBeNull();
    expect(within(screen.getByLabelText('人物集合')).getByText('共 1 个')).toBeTruthy();
  });

  it('结构新增缺必填项时本地提示，补齐后实体出现在列表', async () => {
    const user = userEvent.setup();
    renderCockpit();

    await user.click(screen.getByRole('button', { name: '新增人物' }));
    await user.click(screen.getByRole('button', { name: '提交新增人物' }));

    expect(await screen.findByText(/请先填写必填字段：名称、角色、概要、动机、人物弧。/)).toBeTruthy();

    await user.type(screen.getByLabelText('名称 *'), '阿沅');
    await user.selectOptions(screen.getByLabelText('角色 *'), 'supporting');
    await user.type(screen.getByLabelText('概要 *'), '岛上渔村的少女，常帮守塔人送补给。');
    await user.type(screen.getByLabelText('动机 *'), '解开父亲多年前失踪的谜团。');
    await user.type(screen.getByLabelText('人物弧 *'), '从向导变成真相的守护者。');
    await user.click(screen.getByRole('button', { name: '提交新增人物' }));

    const characters = screen.getByLabelText('人物集合');
    expect(await within(characters).findByText('阿沅')).toBeTruthy();
    expect(within(characters).getByText('共 3 个')).toBeTruthy();
  });

  it('修订结果勾选子集时部分采用，缺少结构差异时禁用并提示', async () => {
    const user = userEvent.setup();
    renderCockpit();

    const panel = screen.getByRole('region', { name: 'AI 定向修改' });
    const textDiff = within(panel).getByRole('checkbox', { name: '故事方向 · A · 卖点' }) as HTMLInputElement;
    const structureDiff = within(panel).getByRole('checkbox', { name: '人物 · 新增 阿沅' }) as HTMLInputElement;

    expect(textDiff.checked).toBe(true);
    expect(structureDiff.checked).toBe(true);

    await user.click(structureDiff);
    expect(within(panel).getByText('结构增删必须整组采用')).toBeTruthy();
    expect((within(panel).getByRole('button', { name: '采用选中的 1 项修改' }) as HTMLButtonElement).disabled).toBe(true);

    await user.click(structureDiff);
    expect(within(panel).queryByText('结构增删必须整组采用')).toBeNull();

    await user.click(textDiff);
    expect(within(panel).getByText('只采用选中的修改会生成待复核的新候选，其余字段保持基线原文。')).toBeTruthy();
    const partial = within(panel).getByRole('button', { name: '采用选中的 1 项修改' }) as HTMLButtonElement;
    expect(partial.disabled).toBe(false);

    await user.click(partial);
    expect(within(panel).getByText(/已采用选中的 1 项修改/)).toBeTruthy();
  });
});
