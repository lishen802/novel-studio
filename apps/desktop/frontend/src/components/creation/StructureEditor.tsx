import { useMemo, useState } from 'react';
import { Button } from '../ui/Button';
import { Input, Textarea } from '../ui/Input';
import { Modal } from '../ui/Modal';
import type { StructureCollectionView } from '../../api/studio';

// 新增表单的字段定义：name 与候选载荷的 JSON 字段一致，界面只显示中文标签。
// 标识字段（id 或唯一名称）同时是提交时的 entityKey，必须与后端的 identityField 一致。
type StructureFieldType = 'text' | 'number' | 'select' | 'ref';

interface StructureField {
  name: string;
  label: string;
  type: StructureFieldType;
  required?: boolean;
  options?: Array<{ value: string; label: string }>;
  // ref 字段的候选项来自当前候选的另一个集合，上游为空时无法提交。
  refCollection?: string;
  long?: boolean;
  placeholder?: string;
  min?: number;
  max?: number;
}

interface StructureCollectionSpec {
  identityField: string;
  suggestion: string;
  fields: StructureField[];
}

const roleOptions = [
  { value: 'protagonist', label: '主角' },
  { value: 'supporting', label: '配角' },
  { value: 'antagonist', label: '反派' },
  { value: 'minor', label: '次要' },
];

const relationshipKindOptions = [
  { value: 'ally', label: '盟友' },
  { value: 'family', label: '家人' },
  { value: 'romantic', label: '恋人' },
  { value: 'rival', label: '对手' },
  { value: 'enemy', label: '宿敌' },
  { value: 'mentor', label: '导师' },
  { value: 'other', label: '其他' },
];

const locationKindOptions = [
  { value: 'city', label: '城市' },
  { value: 'settlement', label: '聚落' },
  { value: 'building', label: '建筑' },
  { value: 'landmark', label: '地标' },
  { value: 'wilderness', label: '荒野' },
  { value: 'other', label: '其他' },
];

const scopeOptions = [
  { value: 'project', label: '全作' },
  { value: 'region', label: '区域' },
  { value: 'location', label: '地点' },
];

// position 为该阶段的顺序字段：必须从 1 起连续填写，界面只做本地校验。
const positionField: StructureField = {
  name: 'position', label: '顺序', type: 'number', required: true, min: 1, placeholder: '从 1 连续',
};

const collectionSpecs: Record<string, StructureCollectionSpec> = {
  characters: {
    identityField: 'id',
    suggestion: 'char-3',
    fields: [
      { name: 'id', label: '标识', type: 'text', required: true },
      { name: 'name', label: '姓名', type: 'text', required: true },
      { name: 'role', label: '角色定位', type: 'select', required: true, options: roleOptions },
      { name: 'summary', label: '简介', type: 'text', required: true, long: true },
      { name: 'motivation', label: '动机', type: 'text', required: true, long: true },
      { name: 'fear', label: '恐惧', type: 'text', long: true },
      { name: 'secret', label: '秘密', type: 'text', long: true },
      { name: 'arc', label: '人物弧光', type: 'text', required: true, long: true },
    ],
  },
  relationships: {
    identityField: 'id',
    suggestion: 'rel-3',
    fields: [
      { name: 'id', label: '标识', type: 'text', required: true },
      { name: 'source_character_ref', label: '来源人物', type: 'ref', required: true, refCollection: 'characters' },
      { name: 'target_character_ref', label: '目标人物', type: 'ref', required: true, refCollection: 'characters' },
      { name: 'kind', label: '关系类型', type: 'select', required: true, options: relationshipKindOptions },
      { name: 'label', label: '关系标签', type: 'text' },
      { name: 'strength', label: '关系强度', type: 'number', min: -100, max: 100, placeholder: '-100 到 100' },
    ],
  },
  factions: {
    identityField: 'name',
    suggestion: '',
    fields: [
      { name: 'name', label: '势力名称', type: 'text', required: true },
      { name: 'summary', label: '简介', type: 'text', required: true, long: true },
      { name: 'goal', label: '目标', type: 'text', required: true, long: true },
    ],
  },
  locations: {
    identityField: 'name',
    suggestion: '',
    fields: [
      { name: 'name', label: '地点名称', type: 'text', required: true },
      { name: 'kind', label: '地点类型', type: 'select', required: true, options: locationKindOptions },
      { name: 'summary', label: '简介', type: 'text', required: true, long: true },
    ],
  },
  world_rules: {
    identityField: 'name',
    suggestion: '',
    fields: [
      { name: 'name', label: '规则名称', type: 'text', required: true },
      { name: 'description', label: '规则说明', type: 'text', required: true, long: true },
      { name: 'scope_type', label: '作用范围', type: 'select', required: true, options: scopeOptions },
    ],
  },
  volumes: {
    identityField: 'id',
    suggestion: 'vol-2',
    fields: [
      { name: 'id', label: '标识', type: 'text', required: true },
      { name: 'title', label: '卷名', type: 'text', required: true },
      { name: 'goal', label: '本卷目标', type: 'text', required: true, long: true },
      { name: 'conflict', label: '本卷冲突', type: 'text', required: true, long: true },
      { name: 'payoff', label: '本卷回报', type: 'text', required: true, long: true },
      positionField,
    ],
  },
  arcs: {
    identityField: 'id',
    suggestion: 'arc-2',
    fields: [
      { name: 'id', label: '标识', type: 'text', required: true },
      { name: 'volume_ref', label: '所属卷', type: 'ref', required: true, refCollection: 'volumes' },
      { name: 'title', label: '故事弧标题', type: 'text', required: true },
      { name: 'causal_chain', label: '因果链', type: 'text', required: true, long: true },
      { name: 'character_change', label: '人物变化', type: 'text', required: true, long: true },
      positionField,
    ],
  },
  near_term_chapters: {
    identityField: 'id',
    suggestion: 'ch-4',
    fields: [
      { name: 'id', label: '标识', type: 'text', required: true },
      { name: 'arc_ref', label: '所属故事弧', type: 'ref', required: true, refCollection: 'arcs' },
      { name: 'title', label: '章节标题', type: 'text', required: true },
      { name: 'goal', label: '章节目标', type: 'text', required: true, long: true },
      { name: 'conflict', label: '章节冲突', type: 'text', required: true, long: true },
      { name: 'outcome', label: '章节结果', type: 'text', required: true, long: true },
      positionField,
    ],
  },
};

interface Props {
  collections: StructureCollectionView[];
  disabled?: boolean;
  // 父层负责调用后端并采用返回的投影；失败时抛错，本组件据此保留作者输入。
  onAdd: (collection: string, entityKey: string, value: string) => Promise<void>;
  onDelete: (collection: string, entityKey: string) => Promise<void>;
}

export function StructureEditor({ collections, disabled, onAdd, onDelete }: Props) {
  const [openCollection, setOpenCollection] = useState('');
  const [draft, setDraft] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState('');
  const [deleteError, setDeleteError] = useState('');
  const [saving, setSaving] = useState(false);
  const [pending, setPending] = useState<{ collection: string; title: string; key: string; entityTitle: string } | null>(null);

  const entityOptions = useMemo(() => {
    const table: Record<string, Array<{ value: string; label: string }>> = {};
    for (const item of collections) table[item.collection] = item.entities.map((entity) => ({ value: entity.key, label: entity.title || entity.key }));
    return table;
  }, [collections]);

  const toggleForm = (collection: string) => {
    setFormError('');
    if (openCollection === collection) {
      setOpenCollection('');
      return;
    }
    const spec = collectionSpecs[collection];
    setOpenCollection(collection);
    setDraft(spec ? { [spec.identityField]: spec.suggestion } : {});
  };

  // 上游集合为空时不允许提交：ref 字段没有可选项，先补齐上游实体再回来。
  const blockedUpstream = (spec: StructureCollectionSpec) => {
    const missing: string[] = [];
    for (const field of spec.fields) {
      if (field.type !== 'ref' || !field.refCollection) continue;
      if ((entityOptions[field.refCollection] ?? []).length) continue;
      const title = collections.find((item) => item.collection === field.refCollection)?.title ?? field.label;
      if (!missing.includes(title)) missing.push(title);
    }
    return missing;
  };

  const submit = async (collection: string) => {
    const spec = collectionSpecs[collection];
    if (!spec) return;
    const missing = spec.fields.filter((field) => field.required && !String(draft[field.name] ?? '').trim());
    if (missing.length) {
      setFormError(`请先填写必填字段：${missing.map((field) => field.label).join('、')}。`);
      return;
    }
    for (const field of spec.fields) {
      if (field.type !== 'number') continue;
      const raw = String(draft[field.name] ?? '').trim();
      if (!raw) continue;
      const parsed = Number(raw);
      const range = field.min !== undefined && field.max !== undefined
        ? `${field.min} 到 ${field.max} 之间`
        : field.min !== undefined ? `${field.min} 以上` : `${field.max} 以下`;
      if (!Number.isInteger(parsed) || (field.min !== undefined && parsed < field.min) || (field.max !== undefined && parsed > field.max)) {
        setFormError(`${field.label}需要是${range}的整数。`);
        return;
      }
    }
    const upstream = blockedUpstream(spec);
    if (upstream.length) {
      setFormError(`请先新增${upstream.map((title) => `「${title}」`).join('')}，再提交此条目。`);
      return;
    }
    const value: Record<string, unknown> = {};
    for (const field of spec.fields) {
      const raw = String(draft[field.name] ?? '').trim();
      if (!raw) continue;
      value[field.name] = field.type === 'number' ? Number(raw) : raw;
    }
    const entityKey = String(draft[spec.identityField] ?? '').trim();
    setSaving(true);
    setFormError('');
    try {
      await onAdd(collection, entityKey, JSON.stringify(value));
      setDraft({});
      setOpenCollection('');
    } catch (error) {
      // 后端拒绝（必填字段、引用或锁）时保留已填内容，由作者修正后重试。
      setFormError(error instanceof Error && error.message.trim() ? error.message : '操作失败，请重试。');
    } finally {
      setSaving(false);
    }
  };

  const confirmDelete = async () => {
    if (!pending) return;
    setSaving(true);
    setDeleteError('');
    try {
      await onDelete(pending.collection, pending.key);
      setPending(null);
    } catch (error) {
      // 锁或跨阶段引用导致的拒绝原样展示，不重试也不自动解锁。
      setDeleteError(error instanceof Error && error.message.trim() ? error.message : '操作失败，请重试。');
    } finally {
      setSaving(false);
    }
  };

  return (
    <section aria-label="结构增删" className="mb-6 rounded-xl border border-border bg-surface p-4 space-y-4">
      <h3 className="text-sm font-medium text-text">结构增删</h3>
      <p className="text-xs leading-relaxed text-text-sub">
        增删会保存为新的候选版本，旧版本保留为历史版本，仍需重新审阅与阶段接受；已锁定的实体不能删除，
        被后续阶段引用的实体也不能删除。此处不会调用模型，也不会写入正式档案。
      </p>

      {collections.map((item) => {
        const spec = collectionSpecs[item.collection];
        const upstream = spec ? blockedUpstream(spec) : [];
        return (
          <div key={item.collection} className="rounded-lg border border-border p-3">
            <div className="flex items-center justify-between gap-2">
              <h4 className="text-sm font-medium text-text">
                {item.title}
                <span className="ml-2 text-xs text-text-sub">共 {item.entities.length} 个</span>
              </h4>
              {spec && (
                <Button size="sm" aria-label={`新增${item.title}`} disabled={disabled || saving} onClick={() => toggleForm(item.collection)}>
                  {openCollection === item.collection ? '收起' : '新增'}
                </Button>
              )}
            </div>

            {item.entities.length ? (
              <ul className="mt-2 space-y-1">
                {item.entities.map((entity) => (
                  <li key={entity.key} className="flex items-center justify-between gap-2 rounded-md bg-surface-hover px-2 py-1">
                    <span className="truncate text-sm text-text">{entity.title || entity.key}</span>
                    <Button
                      size="sm"
                      variant="ghost"
                      className="shrink-0 text-danger"
                      aria-label={`删除 ${item.title} ${entity.title || entity.key}`}
                      disabled={disabled || saving}
                      onClick={() => { setDeleteError(''); setPending({ collection: item.collection, title: item.title, key: entity.key, entityTitle: entity.title || entity.key }); }}
                    >
                      删除
                    </Button>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="mt-2 text-xs text-text-muted">暂无条目。</p>
            )}

            {spec && openCollection === item.collection && (
              <form
                className="mt-3 space-y-3 border-t border-border pt-3"
                onSubmit={(event) => { event.preventDefault(); void submit(item.collection); }}
              >
                <p className="text-xs text-text-sub">{item.title} · 新增条目；带 * 的字段必填，留空的选填字段不会写入候选。</p>
                {upstream.length > 0 && (
                  <p role="status" className="text-sm text-warning">
                    请先新增{upstream.map((title) => `「${title}」`).join('')}，再填写此表单。
                  </p>
                )}
                {spec.fields.map((field) => {
                  const label = `${field.label}${field.required ? ' *' : ''}`;
                  if (field.type === 'select' || field.type === 'ref') {
                    const options = field.type === 'ref' ? entityOptions[field.refCollection ?? ''] ?? [] : field.options ?? [];
                    return (
                      <label key={field.name} className="block text-sm text-text">
                        {label}
                        <select
                          aria-label={label}
                          className="mt-2 w-full rounded-md border border-border bg-surface p-2 text-sm"
                          value={draft[field.name] ?? ''}
                          disabled={disabled || saving}
                          onChange={(event) => setDraft((old) => ({ ...old, [field.name]: event.target.value }))}
                        >
                          <option value="">请选择</option>
                          {options.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                        </select>
                      </label>
                    );
                  }
                  return (
                    <label key={field.name} className="block text-sm text-text">
                      {label}
                      {field.long ? (
                        <Textarea
                          aria-label={label}
                          className="mt-2 min-h-20"
                          placeholder={field.placeholder}
                          value={draft[field.name] ?? ''}
                          disabled={disabled || saving}
                          onChange={(event) => setDraft((old) => ({ ...old, [field.name]: event.target.value }))}
                        />
                      ) : (
                        <Input
                          aria-label={label}
                          className="mt-2"
                          type={field.type === 'number' ? 'number' : 'text'}
                          min={field.min}
                          max={field.max}
                          placeholder={field.placeholder}
                          value={draft[field.name] ?? ''}
                          disabled={disabled || saving}
                          onChange={(event) => setDraft((old) => ({ ...old, [field.name]: event.target.value }))}
                        />
                      )}
                    </label>
                  );
                })}
                {formError && <p role="alert" className="text-sm text-danger">{formError} 已填内容已保留。</p>}
                <div className="flex flex-wrap gap-2">
                  <Button variant="primary" type="submit" aria-label={`提交新增${item.title}`} disabled={disabled || saving || upstream.length > 0}>
                    {saving ? '提交中…' : '提交'}
                  </Button>
                  <Button type="button" disabled={saving} onClick={() => { setFormError(''); setOpenCollection(''); }}>取消</Button>
                </div>
              </form>
            )}
          </div>
        );
      })}

      <Modal open={!!pending} onClose={() => setPending(null)} title="删除结构条目">
        {pending && (
          <>
            <p className="text-sm leading-relaxed text-text-sub">
              将从「{pending.title}」中删除「{pending.entityTitle}」。删除会生成新的候选版本，旧版本保留为历史版本，
              需要重新审阅与阶段接受；已锁定的实体无法删除。
            </p>
            {deleteError && <p role="alert" className="mt-3 text-sm text-danger">{deleteError}</p>}
            <div className="mt-4 flex justify-end gap-2">
              <Button disabled={saving} onClick={() => setPending(null)}>取消</Button>
              <Button
                variant="danger"
                aria-label={`确认删除 ${pending.title} ${pending.entityTitle}`}
                disabled={saving}
                onClick={() => void confirmDelete()}
              >
                {saving ? '删除中…' : '确认删除'}
              </Button>
            </div>
          </>
        )}
      </Modal>
    </section>
  );
}
