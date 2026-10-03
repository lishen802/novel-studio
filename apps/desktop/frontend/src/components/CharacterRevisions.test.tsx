import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { CharacterRevisions, type CharacterRevisionAPI } from './CharacterRevisions';
import type { ConstraintPreview, ConstraintRequest } from '../api/studio';
const profile = { name: '林舟', aliases: ['小舟'], role: 'protagonist', summary: '原简介', motivation: '寻找故乡', fear: '遗忘', secret: '身世', arc: '学会信任' };
const original = JSON.stringify(profile);
function setup() {
 const api: CharacterRevisionAPI = {
  characterRevisions: vi.fn().mockResolvedValue([{ assetId: 'a1', typedId: 'c1', name: '林舟', kind: 'character', original, current: original, revision: 1, locked: false, changes: [] }]),
  listChapters: vi.fn().mockResolvedValue([{ id: 'future', title: '新章', position: 2 }]),
  previewConstraintChange: vi.fn().mockImplementation(async (request: ConstraintRequest): Promise<ConstraintPreview> => ({ request: { ...request, projectId: 'project', branchId: 'main' }, digest: 'frozen', before: original, revision: 2, position: 2, protectedChapters: 1, futureChapters: 1, canApply: true, blockedReason: '' })),
  applyConstraintChange: vi.fn().mockResolvedValue({ revision: 2, position: 2 }),
 };
 return api;
}
async function edit(api: CharacterRevisionAPI, registerBeforeLeave?: (handler: () => Promise<void>) => () => void) {
 render(<CharacterRevisions api={api} registerBeforeLeave={registerBeforeLeave} />);
 await waitFor(() => expect((screen.getByText('修订人物档案') as HTMLButtonElement).disabled).toBe(false));
 fireEvent.click(screen.getByText('修订人物档案'));
 fireEvent.change(screen.getByLabelText('动机'), { target: { value: '保护故乡' } });
 fireEvent.change(screen.getByLabelText('人物修订理由'), { target: { value: '调整后续方向' } });
 fireEvent.change(screen.getByLabelText('人物生效章节'), { target: { value: 'future' } });
 fireEvent.click(screen.getByText('预览人物差异'));
 await screen.findByText('修改前：寻找故乡');
}
afterEach(cleanup);
it('展示真实字段差异，编辑后旧预览失效', async () => {
 const api = setup(); await edit(api);
 expect(screen.getByText('修改后：保护故乡')).toBeTruthy();
 const request = vi.mocked(api.previewConstraintChange).mock.calls[0][0];
 expect(JSON.parse(request.text)).toEqual({ ...profile, motivation: '保护故乡' });
 fireEvent.change(screen.getByLabelText('秘密'), { target: { value: '新的秘密' } });
 expect(screen.queryByText('修改前：寻找故乡')).toBeNull();
 expect((screen.getByText('确认人物修订') as HTMLButtonElement).disabled).toBe(true);
});
it('失败保留输入和幂等键，重试成功才关闭', async () => {
 const api = setup(); vi.mocked(api.applyConstraintChange).mockRejectedValueOnce(new Error('响应丢失')).mockResolvedValueOnce({ revision: 2, position: 2 } as ConstraintPreview);
 await edit(api); fireEvent.click(screen.getByText('确认人物修订'));
 await screen.findByText('响应丢失');
 expect((screen.getByLabelText('动机') as HTMLTextAreaElement).value).toBe('保护故乡');
 fireEvent.click(screen.getByText('确认人物修订'));
 await screen.findByText('人物版本 2 已保存，从第 2 章起生效。');
 const calls = vi.mocked(api.applyConstraintChange).mock.calls;
 expect(calls[1][0]).toEqual(calls[0][0]);
 expect(calls[0][0].request.projectId).toBe('project');
 expect(screen.queryByRole('dialog')).toBeNull();
});
it('Esc 和导航不能丢弃待确认输入，明确取消解除保护', async () => {
 const api = setup(); let handler: (() => Promise<void>) | undefined;
 const unregister = vi.fn(); const register = vi.fn((guard: () => Promise<void>) => { handler = guard; return unregister; });
 await edit(api, register);
 fireEvent.keyDown(window, { key: 'Escape' });
 expect(screen.getByRole('dialog')).toBeTruthy();
 await expect(handler!()).rejects.toThrow('人物修订尚未确认');
 fireEvent.click(screen.getByText('取消修订'));
 expect(screen.queryByRole('dialog')).toBeNull();
 expect(unregister).toHaveBeenCalled();
 expect(api.applyConstraintChange).not.toHaveBeenCalled();
});
it('锁定档案不能修改，读取失败可重新读取', async () => {
 const api = setup(); vi.mocked(api.characterRevisions).mockRejectedValueOnce(new Error('数据库暂不可读')).mockResolvedValueOnce([{ assetId: 'a1', typedId: 'c1', name: '林舟', kind: 'character', original, current: original, revision: 1, locked: true, changes: [] }]);
 render(<CharacterRevisions api={api} />);
 await screen.findByText('读取人物版本失败：数据库暂不可读');
 fireEvent.click(screen.getByText('重新读取版本'));
 await waitFor(() => expect((screen.getByText('修订人物档案') as HTMLButtonElement).disabled).toBe(false));
 fireEvent.click(screen.getByText('修订人物档案'));
 expect((screen.getByLabelText('动机') as HTMLTextAreaElement).disabled).toBe(true);
 expect((screen.getByText('预览人物差异') as HTMLButtonElement).disabled).toBe(true);
});
