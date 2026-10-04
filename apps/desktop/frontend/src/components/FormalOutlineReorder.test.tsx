import { cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';
import { FormalOutlineReorder, type FormalOutlineReorderAPI } from './FormalOutlineReorder';

afterEach(cleanup);

it('未来相邻节点重排先预览再确认，保留后端回执', async () => {
  const user = userEvent.setup();
  const request = { projectId: 'project', branchId: 'main', firstAssetId: 'first-asset', secondAssetId: 'second-asset', chapterId: 'future', reason: '调整节奏' };
  const receipt = { request, digest: 'digest', firstNodeId: 'first', secondNodeId: 'second', firstTitle: '先守城', secondTitle: '后破局', parentNodeId: 'arc', firstPosition: 1, secondPosition: 2, position: 3, firstRevision: 2, secondRevision: 2 };
  const preview = vi.fn().mockResolvedValue(receipt);
  const confirm = vi.fn().mockResolvedValue(receipt);
  const api = {
    listChapters: vi.fn().mockResolvedValue([{ id: 'future', title: '第三章', position: 3, body: '' }]),
    planningRevisions: vi.fn().mockResolvedValue([
      { assetId: 'first-asset', kind: 'outline_node', typedId: 'first', locked: false },
      { assetId: 'second-asset', kind: 'outline_node', typedId: 'second', locked: false },
    ]),
    formalOutlinePositionsAtChapter: vi.fn().mockResolvedValue({ first: 1, second: 2 }),
    previewFormalOutlineReorder: preview,
    confirmFormalOutlineReorder: confirm,
  } as unknown as FormalOutlineReorderAPI;
  const onApplied = vi.fn().mockResolvedValue(undefined);
  render(<FormalOutlineReorder api={api} nodes={[
    { id: 'first', title: '先守城', level: 'chapter', parentId: 'arc', introducedAt: 2, retiredAt: 0 },
    { id: 'second', title: '后破局', level: 'chapter', parentId: 'arc', introducedAt: 2, retiredAt: 0 },
  ]} readOnly={false} onApplied={onApplied} />);

  await user.click(screen.getByRole('button', { name: '预览未来顺序' }));
  await user.selectOptions(await screen.findByLabelText('重排生效章节'), 'future');
  await waitFor(() => expect(api.formalOutlinePositionsAtChapter).toHaveBeenCalledWith('future'));
  await user.selectOptions(screen.getByLabelText('前一个正式节点'), 'first-asset');
  await user.selectOptions(screen.getByLabelText('后一个正式节点'), 'second-asset');
  await user.type(screen.getByLabelText('节点重排理由'), '调整节奏');
  await user.click(screen.getByRole('button', { name: '预览调整范围' }));
  expect(preview).toHaveBeenCalledWith('first-asset', 'second-asset', 'future', '调整节奏');
  expect(confirm).not.toHaveBeenCalled();
  await user.click(await screen.findByRole('button', { name: '确认未来顺序' }));
  await waitFor(() => expect(confirm).toHaveBeenCalledWith(expect.objectContaining({ request, digest: 'digest' })));
  expect(onApplied).toHaveBeenCalledTimes(1);
  expect(await screen.findByText(/已从第 3 章起/)).toBeTruthy();
});
