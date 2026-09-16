import { cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';
import { StructureEditor } from './StructureEditor';
import type { StructureCollectionView } from '../../api/studio';

afterEach(cleanup);

const collections: StructureCollectionView[] = [
  {
    collection: 'characters', title: '人物', identityField: 'id',
    entities: [{ key: 'char-1', title: '阿绫' }, { key: 'char-2', title: '守塔人' }],
  },
  { collection: 'relationships', title: '人物关系', identityField: 'id', entities: [] },
  {
    collection: 'factions', title: '势力', identityField: 'name',
    entities: [{ key: '潮汐会', title: '潮汐会' }],
  },
];

it('新增人物关系提交标识字段与 JSON 字段，空的选填字段不写入', async () => {
  const user = userEvent.setup();
  const onAdd = vi.fn().mockResolvedValue(undefined);
  render(<StructureEditor collections={collections} onAdd={onAdd} onDelete={vi.fn()} />);

  await user.click(screen.getByRole('button', { name: '新增人物关系' }));
  await user.selectOptions(screen.getByLabelText('来源人物 *'), 'char-1');
  await user.selectOptions(screen.getByLabelText('目标人物 *'), 'char-2');
  await user.selectOptions(screen.getByLabelText('关系类型 *'), 'rival');
  await user.type(screen.getByLabelText('关系强度'), '40');
  await user.click(screen.getByRole('button', { name: '提交新增人物关系' }));

  await waitFor(() => expect(onAdd).toHaveBeenCalledTimes(1));
  expect(onAdd.mock.calls[0][0]).toBe('relationships');
  // 标识字段默认预填 rel-3，作为 entityKey 与 JSON 的 id 保持一致。
  expect(onAdd.mock.calls[0][1]).toBe('rel-3');
  expect(JSON.parse(onAdd.mock.calls[0][2])).toEqual({
    id: 'rel-3', source_character_ref: 'char-1', target_character_ref: 'char-2', kind: 'rival', strength: 40,
  });
  // 提交成功后收起表单，等待父层用新投影刷新。
  expect(screen.queryByLabelText('关系类型 *')).toBeNull();
});

it('标识字段为唯一名称的集合用名称作为标识提交', async () => {
  const user = userEvent.setup();
  const onAdd = vi.fn().mockResolvedValue(undefined);
  render(<StructureEditor collections={collections} onAdd={onAdd} onDelete={vi.fn()} />);

  await user.click(screen.getByRole('button', { name: '新增势力' }));
  await user.type(screen.getByLabelText('势力名称 *'), '灯塔修会');
  await user.type(screen.getByLabelText('简介 *'), '守着灯塔的秘密团体');
  await user.type(screen.getByLabelText('目标 *'), '让灯塔不再熄灭');
  await user.click(screen.getByRole('button', { name: '提交新增势力' }));

  await waitFor(() => expect(onAdd).toHaveBeenCalledTimes(1));
  expect(onAdd.mock.calls[0]).toEqual([
    'factions', '灯塔修会',
    JSON.stringify({ name: '灯塔修会', summary: '守着灯塔的秘密团体', goal: '让灯塔不再熄灭' }),
  ]);
});

it('必填字段缺失时本地拦截并提示，不发请求且保留已填内容', async () => {
  const user = userEvent.setup();
  const onAdd = vi.fn();
  render(<StructureEditor collections={collections} onAdd={onAdd} onDelete={vi.fn()} />);

  await user.click(screen.getByRole('button', { name: '新增人物' }));
  await user.type(screen.getByLabelText('姓名 *'), '阿绫');
  await user.click(screen.getByRole('button', { name: '提交新增人物' }));

  expect(onAdd).not.toHaveBeenCalled();
  expect((await screen.findByRole('alert')).textContent).toContain('请先填写必填字段');
  expect((screen.getByLabelText('姓名 *') as HTMLInputElement).value).toBe('阿绫');
});

it('上游集合为空时不允许提交，并提示先新增上游实体', async () => {
  const user = userEvent.setup();
  const onAdd = vi.fn();
  const empty = collections.map((item) => item.collection === 'characters' ? { ...item, entities: [] } : item);
  render(<StructureEditor collections={empty} onAdd={onAdd} onDelete={vi.fn()} />);

  await user.click(screen.getByRole('button', { name: '新增人物关系' }));

  expect(screen.getByRole('status').textContent).toContain('请先新增「人物」');
  expect((screen.getByRole('button', { name: '提交新增人物关系' }) as HTMLButtonElement).disabled).toBe(true);
  expect(onAdd).not.toHaveBeenCalled();
});

it('删除先二次确认，确认后按标识调用删除并关闭确认框', async () => {
  const user = userEvent.setup();
  const onDelete = vi.fn().mockResolvedValue(undefined);
  render(<StructureEditor collections={collections} onAdd={vi.fn()} onDelete={onDelete} />);

  await user.click(screen.getByRole('button', { name: '删除 人物 阿绫' }));

  expect(screen.getByRole('dialog')).toBeTruthy();
  expect(screen.getByText(/将从「人物」中删除「阿绫」/)).toBeTruthy();
  expect((screen.getByRole('button', { name: '确认删除 人物 阿绫' }) as HTMLButtonElement).disabled).toBe(false);
  expect(onDelete).not.toHaveBeenCalled();

  await user.click(screen.getByRole('button', { name: '确认删除 人物 阿绫' }));

  await waitFor(() => expect(onDelete).toHaveBeenCalledWith('characters', 'char-1'));
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
});

it('删除被后端拒绝时原样展示错误、保留确认框且不重试', async () => {
  const user = userEvent.setup();
  const onDelete = vi.fn().mockRejectedValue(new Error('LOCKED_ENTITY|该实体已锁定，无法删除。'));
  render(<StructureEditor collections={collections} onAdd={vi.fn()} onDelete={onDelete} />);

  await user.click(screen.getByRole('button', { name: '删除 人物 阿绫' }));
  await user.click(screen.getByRole('button', { name: '确认删除 人物 阿绫' }));

  const alert = await screen.findByRole('alert');
  expect(alert.textContent).toContain('LOCKED_ENTITY|该实体已锁定，无法删除。');
  expect(screen.getByRole('dialog')).toBeTruthy();
  expect(onDelete).toHaveBeenCalledTimes(1);
});

it('新增被后端拒绝时展示后端错误并保留已填内容', async () => {
  const user = userEvent.setup();
  const onAdd = vi.fn().mockRejectedValue(new Error('SCHEMA_REQUIRED|角色缺少必填字段 arc。'));
  render(<StructureEditor collections={collections} onAdd={onAdd} onDelete={vi.fn()} />);

  await user.click(screen.getByRole('button', { name: '新增人物' }));
  await user.type(screen.getByLabelText('姓名 *'), '阿绫');
  await user.type(screen.getByLabelText('简介 *'), '灯塔守望者的女儿');
  await user.type(screen.getByLabelText('动机 *'), '找回失踪的父亲');
  await user.type(screen.getByLabelText('人物弧光 *'), '从依赖到守护');
  await user.selectOptions(screen.getByLabelText('角色定位 *'), 'protagonist');
  await user.click(screen.getByRole('button', { name: '提交新增人物' }));

  expect((await screen.findByRole('alert')).textContent).toContain('SCHEMA_REQUIRED|角色缺少必填字段 arc。');
  expect((screen.getByLabelText('姓名 *') as HTMLInputElement).value).toBe('阿绫');
  expect(onAdd).toHaveBeenCalledTimes(1);
});
