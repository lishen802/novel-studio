import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import Foreshadowing from './Foreshadowing';
import { studio } from '../../api/studio';
vi.mock('../../api/studio', () => ({ studio: { worldAssets: vi.fn(), storyWorld: vi.fn(), listChapters: vi.fn(), updateForeshadowingHookStatus: vi.fn() } }));
const hook = { id: 'h1', title: '断剑', setup: '剑上有字', status: 'progressing', plantedChapterId: 'c1', dueChapterId: 'c1', notes: '备注' };
function prepare() {
 vi.mocked(studio.worldAssets).mockResolvedValue({ hooks: [hook] } as never);
 vi.mocked(studio.storyWorld).mockResolvedValue({ characters: [], factions: [], relationships: [] } as never);
 vi.mocked(studio.listChapters).mockResolvedValue([{ id: 'c1', title: '第一章' }] as never);
}
afterEach(() => { cleanup(); vi.resetAllMocks(); });
it('转换生命周期值，失败保留原状态并允许重试', async () => {
 prepare();
 vi.mocked(studio.updateForeshadowingHookStatus).mockRejectedValueOnce(new Error('磁盘不可写')).mockResolvedValueOnce({ ...hook, status: 'abandoned' });
 render(<Foreshadowing />);
 const select = await screen.findByLabelText('断剑状态');
 expect((select as HTMLSelectElement).value).toBe('growing');
 fireEvent.change(select, { target: { value: 'dropped' } });
 await screen.findByText('保存失败：磁盘不可写');
 expect((select as HTMLSelectElement).value).toBe('growing');
 fireEvent.change(select, { target: { value: 'dropped' } });
 await waitFor(() => expect((select as HTMLSelectElement).value).toBe('dropped'));
 expect(studio.updateForeshadowingHookStatus).toHaveBeenLastCalledWith('h1', 'abandoned', '');
});
it('回收前必须选择章节，取消不保存', async () => {
 prepare();
 vi.mocked(studio.updateForeshadowingHookStatus).mockResolvedValue({ ...hook, status: 'resolved', resolvedChapterId: 'c1' });
 render(<Foreshadowing />);
 const select = await screen.findByLabelText('断剑状态');
 fireEvent.change(select, { target: { value: 'resolved' } });
 expect((screen.getByText('确认回收') as HTMLButtonElement).disabled).toBe(true);
 fireEvent.click(screen.getByText('取消'));
 expect(studio.updateForeshadowingHookStatus).not.toHaveBeenCalled();
 fireEvent.change(select, { target: { value: 'resolved' } });
 fireEvent.change(screen.getByLabelText('回收章节'), { target: { value: 'c1' } });
 fireEvent.click(screen.getByText('确认回收'));
 await waitFor(() => expect((select as HTMLSelectElement).value).toBe('resolved'));
 expect(studio.updateForeshadowingHookStatus).toHaveBeenCalledWith('h1', 'resolved', 'c1');
});
it('加载失败显示原因而非空列表，支持重试', async () => {
 prepare();
 vi.mocked(studio.worldAssets).mockRejectedValueOnce(new Error('项目不可读'));
 render(<Foreshadowing />);
 await screen.findByText('加载失败：项目不可读');
 expect(screen.queryByText('还没有伏笔')).toBeNull();
 fireEvent.click(screen.getByText('重试加载'));
 await screen.findByLabelText('断剑状态');
});
it('保存期间禁止重复切换，成功使用后端返回的状态', async () => {
 prepare();
 let finish!: (value: Record<string, unknown>) => void;
 vi.mocked(studio.updateForeshadowingHookStatus).mockImplementation(() => new Promise((resolve) => { finish = resolve; }));
 render(<Foreshadowing />);
 const select = await screen.findByLabelText('断剑状态');
 fireEvent.change(select, { target: { value: 'planted' } });
 expect((select as HTMLSelectElement).disabled).toBe(true);
 finish({ ...hook, status: 'planted' });
 await waitFor(() => expect((select as HTMLSelectElement).disabled).toBe(false));
 expect((select as HTMLSelectElement).value).toBe('planted');
 expect(studio.updateForeshadowingHookStatus).toHaveBeenCalledTimes(1);
});
