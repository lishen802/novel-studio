import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { PlanningRevisions, type PlanningRevisionAPI } from './PlanningRevisions';
import type { ConstraintPreview, ConstraintRequest } from '../api/studio';
const original = JSON.stringify({ '/title': '潮汐', '/goal': '守城', '/planning/commitments/0/statement': '解开旧信' });
function setup(kind = 'outline_node') {
 const api: PlanningRevisionAPI = {
  planningRevisions: vi.fn().mockResolvedValue([{ assetId: 'plan', typedId: 'node', kind, name: '潮汐', original, current: original, revision: 1, locked: false, fields: [{ path: '/title', label: '标题', value: '潮汐' }, { path: '/goal', label: '目标', value: '守城' }, { path: '/planning/commitments/0/statement', label: '承诺 1 · 内容', value: '解开旧信' }], changes: [] }]),
  listChapters: vi.fn().mockResolvedValue([{ id: 'future', title: '新章', position: 2 }]),
  previewConstraintChange: vi.fn().mockImplementation(async (request: ConstraintRequest): Promise<ConstraintPreview> => ({ request: { ...request, projectId: 'project', branchId: 'main' }, digest: 'frozen', before: original, revision: 2, position: 2, protectedChapters: 1, futureChapters: 1, canApply: true, blockedReason: '' })),
  applyConstraintChange: vi.fn().mockResolvedValue({ revision: 2, position: 2 }),
 };
 return api;
}
async function open(kind: 'proposal' | 'outline_node' = 'outline_node', registerBeforeLeave?: (handler: () => Promise<void>) => () => void) {
 const api = setup(kind); render(<PlanningRevisions api={api} kind={kind} registerBeforeLeave={registerBeforeLeave} />);
 const button = screen.getByText(kind === 'proposal' ? '修订作品方案' : '修订路线图'); await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false)); fireEvent.click(button);
 return api;
}
async function prepare() {
 fireEvent.change(screen.getByLabelText('承诺 1 · 内容'), { target: { value: '在潮退前解开旧信' } });
 fireEvent.change(screen.getByLabelText('规划修订理由'), { target: { value: '调整后续回报' } });
 fireEvent.change(screen.getByLabelText('规划生效章节'), { target: { value: 'future' } });
 fireEvent.click(screen.getByText('预览规划差异')); await screen.findByText('修改后：在潮退前解开旧信');
}
afterEach(cleanup);
it('显示承诺字段真实差异，修改文字后旧确认失效', async () => {
 const api = await open(); await prepare(); expect(screen.getByText('修改前：解开旧信')).toBeTruthy();
 expect(JSON.parse(vi.mocked(api.previewConstraintChange).mock.calls[0][0].text)).toEqual({ ...JSON.parse(original), '/planning/commitments/0/statement': '在潮退前解开旧信' });
 fireEvent.change(screen.getByLabelText('目标'), { target: { value: '保护居民' } }); expect(screen.queryByText('修改后：在潮退前解开旧信')).toBeNull(); expect((screen.getByText('确认规划修订') as HTMLButtonElement).disabled).toBe(true);
});
it('确认失败保留文字与同一幂等键，成功后关闭并读取历史', async () => {
 const api = await open(); vi.mocked(api.applyConstraintChange).mockRejectedValueOnce(new Error('回执响应丢失')).mockResolvedValueOnce({ revision: 2, position: 2 } as ConstraintPreview);
 await prepare(); fireEvent.click(screen.getByText('确认规划修订')); await screen.findByText('回执响应丢失'); expect((screen.getByLabelText('承诺 1 · 内容') as HTMLTextAreaElement).value).toBe('在潮退前解开旧信');
 fireEvent.click(screen.getByText('确认规划修订')); await screen.findByText('规划版本 2 已保存，从第 2 章起生效。'); const calls = vi.mocked(api.applyConstraintChange).mock.calls; expect(calls[1][0]).toEqual(calls[0][0]); expect(calls[0][0].request.branchId).toBe('main'); expect(screen.queryByRole('dialog')).toBeNull();
});
it('导航和 Esc 保留输入，明确取消才退出', async () => {
 let handler: (() => Promise<void>) | undefined; const unregister = vi.fn(); await open('outline_node', (guard) => { handler = guard; return unregister; }); await prepare();
 fireEvent.keyDown(window, { key: 'Escape' }); expect(screen.getByRole('dialog')).toBeTruthy(); await expect(handler!()).rejects.toThrow('规划修订尚未确认'); fireEvent.click(screen.getByText('取消修订')); expect(screen.queryByRole('dialog')).toBeNull(); expect(unregister).toHaveBeenCalled();
});
it('作品方案标题修订说明不改项目名称，采用仍由作者确认', async () => {
 const api = await open('proposal'); expect(screen.getByText('未来方案标题、类型和卖点属于规划版本，不改变项目名称和项目设置。')).toBeTruthy(); await prepare(); expect(api.applyConstraintChange).not.toHaveBeenCalled();
});
it('读取失败可重试，锁定或只读时禁止编辑', async () => {
 const api = setup(); vi.mocked(api.planningRevisions).mockRejectedValueOnce(new Error('读取失败')); const view = render(<PlanningRevisions api={api} kind="outline_node" readOnly />); await screen.findByText('读取规划版本失败：读取失败'); fireEvent.click(screen.getByText('重新读取规划')); await waitFor(() => expect(api.planningRevisions).toHaveBeenCalledTimes(2)); expect((screen.getByText('修订路线图') as HTMLButtonElement).disabled).toBe(true); view.unmount();
 const locked = setup(); const rows = await locked.planningRevisions(); rows[0].locked = true; vi.mocked(locked.planningRevisions).mockResolvedValue(rows); render(<PlanningRevisions api={locked} kind="outline_node" />); await waitFor(() => expect((screen.getByText('修订路线图') as HTMLButtonElement).disabled).toBe(false)); fireEvent.click(screen.getByText('修订路线图')); expect((screen.getByLabelText('目标') as HTMLTextAreaElement).disabled).toBe(true);
});
