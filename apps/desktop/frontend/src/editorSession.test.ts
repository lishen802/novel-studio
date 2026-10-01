import { describe, expect, it, vi } from 'vitest';
import { EditorSession } from './editorSession';
function deferred<T>() { let resolve!: (value:T)=>void; const promise = new Promise<T>(yes=>{resolve=yes;}); return {promise,resolve}; }
describe('编辑会话的保存顺序', () => {
  it('保存中继续输入时等待最新内容落盘再结束', async () => {
    const first = deferred<void>();
    const save = vi.fn().mockImplementationOnce(()=>first.promise).mockResolvedValue(undefined);
    const session = new EditorSession('chapter', '原文', 1, save);
    session.body = '第一次输入'; const flush = session.flush();
    await Promise.resolve(); await Promise.resolve();
    expect(save).toHaveBeenCalledWith('chapter','第一次输入',1);
    session.body = '保存期间的新输入'; const second = session.flush();
    expect(save).toHaveBeenCalledTimes(1);
    first.resolve(); await Promise.all([flush,second]);
    expect(save.mock.calls).toEqual([['chapter','第一次输入',1],['chapter','保存期间的新输入',1]]);
    expect(session.dirty).toBe(false);
  });
  it('保存失败保留脏状态，重试不被失败队列阻塞', async () => {
    const save = vi.fn().mockRejectedValueOnce(new Error('磁盘满')).mockResolvedValue(undefined);
    const session = new EditorSession('chapter','原文',1,save); session.body='新文';
    await expect(session.flush()).rejects.toThrow('磁盘满'); expect(session.dirty).toBe(true);
    await session.flush(); expect(session.dirty).toBe(false);
  });
  it('提交版本期间的新输入使用新 revision 保存', async () => {
    const commit = deferred<{body:string;revision:number}>();
    const save = vi.fn().mockResolvedValue(undefined); const version = vi.fn(()=>commit.promise);
    const session = new EditorSession('chapter','原文',1,save); session.body='版本正文';
    const result = session.commit(version); await vi.waitFor(()=>expect(version).toHaveBeenCalled());
    session.body='提交期间的新内容'; commit.resolve({body:'版本正文',revision:2}); await result;
    expect(save).toHaveBeenLastCalledWith('chapter','提交期间的新内容',2);
    expect(session.body).toBe('提交期间的新内容'); expect(session.revision).toBe(2);
  });
});
