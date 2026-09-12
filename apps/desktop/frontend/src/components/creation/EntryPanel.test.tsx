import { cleanup, render, screen, fireEvent, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { EntryPanel } from './EntryPanel';

afterEach(cleanup);
describe('作者材料入口', () => {
  it('保留原始文本并提交编辑开始时的会话版本', async () => {
    const save = vi.fn().mockResolvedValue(undefined);
    const props = { entry: { id: 'old', kind: 'author_plan' as const, material: '旧材料' }, contentVersion: 3, runSlotVersion: 1, onSave: save };
    const { rerender } = render(<EntryPanel {...props} />);
    fireEvent.change(screen.getByLabelText('已有大纲、人物与设定'), { target: { value: '  作者新材料😀\n  ' } });
    rerender(<EntryPanel {...props} entry={{ ...props.entry, id: 'external', material: '别处修改' }} contentVersion={4} />);
    expect((screen.getByLabelText('已有大纲、人物与设定') as HTMLTextAreaElement).value).toBe('  作者新材料😀\n  ');
    fireEvent.click(screen.getByRole('button', { name: '保存材料与起点' }));
    await waitFor(() => expect(save).toHaveBeenCalledWith('author_plan', '  作者新材料😀\n  ', 3, 1));
  });
  it('冲突后保留输入，不伪装保存成功', async () => {
    const save = vi.fn().mockRejectedValue(new Error('会话版本已变化'));
    render(<EntryPanel contentVersion={2} runSlotVersion={0} onSave={save} />);
    fireEvent.click(screen.getByRole('button', { name: '我已有方案' }));
    fireEvent.change(screen.getByLabelText('已有大纲、人物与设定'), { target: { value: '不可丢失的构思' } });
    fireEvent.click(screen.getByRole('button', { name: '保存材料与起点' }));
    await screen.findByRole('alert');
    expect((screen.getByLabelText('已有大纲、人物与设定') as HTMLTextAreaElement).value).toBe('不可丢失的构思');
  });
});
