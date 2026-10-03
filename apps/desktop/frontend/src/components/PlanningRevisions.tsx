import { useEffect, useRef, useState } from 'react';
import type { ChapterView, ConstraintCommand, ConstraintPreview, ConstraintRequest, ConstraintView } from '../api/studio';
import { Button } from './ui/Button';
import { Card } from './ui/Card';
import { Input, Textarea } from './ui/Input';
import { Modal } from './ui/Modal';

export interface PlanningRevisionAPI {
 planningRevisions: () => Promise<ConstraintView[]>;
 listChapters: () => Promise<ChapterView[]>;
 previewConstraintChange: (request: ConstraintRequest) => Promise<ConstraintPreview>;
 applyConstraintChange: (command: ConstraintCommand) => Promise<ConstraintPreview>;
}
const parse = (text: string): Record<string, string> => JSON.parse(text);
const message = (e: unknown) => e instanceof Error ? e.message : String(e);

/** 规划修订保留原结构，所有文字变化先核对差异和生效章节，失败不清空作者输入。 */
export function PlanningRevisions({ api, kind, registerBeforeLeave, readOnly = false, onFormalTree }: { api: PlanningRevisionAPI; kind: 'proposal' | 'outline_node'; registerBeforeLeave?: (handler: () => Promise<void>) => () => void; readOnly?: boolean; onFormalTree?: (formal: boolean) => void }) {
 const title = kind === 'proposal' ? '作品方案' : '路线图';
 const [items, setItems] = useState<ConstraintView[]>([]);
 const [chapters, setChapters] = useState<ChapterView[]>([]);
 const [open, setOpen] = useState(false);
 const [assetId, setAssetId] = useState('');
 const [draft, setDraft] = useState<Record<string, string>>({});
 const [chapterId, setChapterId] = useState('');
 const [reason, setReason] = useState('');
 const [preview, setPreview] = useState<ConstraintPreview | null>(null);
 const [busy, setBusy] = useState(false);
 const [loading, setLoading] = useState(true);
 const [error, setError] = useState('');
 const [notice, setNotice] = useState('');
 const [reload, setReload] = useState(0);
 const key = useRef('');
 const selected = items.find((item) => item.assetId === assetId);
 const text = JSON.stringify(draft);
 const dirty = open && (!!chapterId || !!reason || text !== JSON.stringify(selected ? parse(selected.current) : {}));
 const pending = useRef({ dirty, busy }); pending.current = { dirty, busy };
 useEffect(() => {
  if ((!dirty && !busy) || !registerBeforeLeave) return;
  return registerBeforeLeave(async () => { if (pending.current.dirty || pending.current.busy) throw new Error('规划修订尚未确认，请完成核对或明确取消修订。'); });
 }, [dirty, busy, registerBeforeLeave]);
 useEffect(() => {
  let active = true; setLoading(true);
  void Promise.all([api.planningRevisions(), api.listChapters()]).then(([views, rows]) => {
   if (active) { const filtered = views.filter((item) => item.kind === kind); filtered.forEach((item) => { parse(item.current); parse(item.original); item.changes.forEach((change) => parse(change.text)); }); setItems(filtered); setChapters(rows); setError(''); onFormalTree?.(filtered.length > 0); }
  }).catch((e) => { if (active) setError(`读取规划版本失败：${message(e)}`); }).finally(() => { if (active) setLoading(false); });
  return () => { active = false; };
 }, [api, kind, reload, onFormalTree]);
 const invalidate = () => { setPreview(null); key.current = ''; setError(''); };
 const choose = (id: string) => { const item = items.find((item) => item.assetId === id); setAssetId(id); setDraft(item ? parse(item.current) : {}); setReason(''); setChapterId(''); invalidate(); };
 const discard = () => { if (!busy) { setOpen(false); invalidate(); } };
 const close = () => { if (busy) return; if (dirty) { setError('还有未确认的规划修改，请继续核对或点击取消修订。'); return; } discard(); };
 const prepare = async () => {
  if (busy || readOnly || !selected) return;
  setBusy(true); setError(''); setPreview(null);
  try { setPreview(await api.previewConstraintChange({ assetId, intent: 'future_only', chapterId, text, reason })); key.current = crypto.randomUUID(); }
  catch (e) { setError(message(e)); } finally { setBusy(false); }
 };
 const confirm = async () => {
  if (busy || readOnly || !preview?.canApply) return;
  setBusy(true); setError('');
  try { const receipt = await api.applyConstraintChange({ request: preview.request, digest: preview.digest, idempotencyKey: key.current }); setOpen(false); setPreview(null); setReason(''); setChapterId(''); setNotice(`规划版本 ${receipt.revision} 已保存，从第 ${receipt.position} 章起生效。`); setReload((value) => value + 1); }
  catch (e) { setError(message(e)); } finally { setBusy(false); }
 };
 const before = preview ? parse(preview.before) : {};
 const after = preview ? parse(preview.request.text) : {};
 const changed = selected?.fields?.filter((field) => before[field.path] !== after[field.path]) || [];
 return <Card className="space-y-3">
  <div className="flex flex-wrap justify-between items-center gap-3"><div><h3 className="font-medium">正式{title}修订</h3><p className="text-sm text-text-sub">保留原规划，新文字从选定章节起使用。</p></div><Button disabled={loading || busy || readOnly || !items.length} onClick={() => { choose(items[0].assetId); setNotice(''); setOpen(true); }}>修订{title}</Button></div>
  {loading && <p role="status" className="text-sm text-text-sub">正在读取规划版本…</p>}
  {!loading && !items.length && !error && <p className="text-sm text-text-muted">正式入档后可修订{title}。旧手工或导入档案纳入版本管理尚未就绪。</p>}
  {readOnly && <p className="text-sm text-text-muted">当前项目为只读，不能保存修订。</p>}
  {items.map((item) => <details key={item.assetId} className="border-t border-border pt-2 text-sm"><summary className="cursor-pointer">{item.name} · 当前版本 {item.revision} · 查看规划与修订记录</summary><div className="space-y-3 mt-3"><PlanningDetails item={item} text={item.original} title="原始规划" />{item.changes.map((change) => <PlanningDetails key={change.revision} item={item} text={change.text} title={`从第 ${change.position} 章起 · 版本 ${change.revision} · ${change.reason}`} />)}</div></details>)}
  {notice && <p role="status" className="text-sm text-primary">{notice}</p>}
  {error && !open && <div><p role="alert" className="text-sm text-danger">{error}</p><Button disabled={loading} onClick={() => setReload((value) => value + 1)}>重新读取规划</Button></div>}
  <Modal open={open} onClose={close} title={`修订正式${title}`}><div className="space-y-4 max-h-[70vh] overflow-auto">
   <p className="text-sm text-text-sub">仅从未来空章节起生效，旧章和原规划保留。节点增删、重排、引用、禁写及锚点变更和整组 AI 规划候选再次入档尚未就绪。</p>
   {kind === 'proposal' && <p className="text-xs text-text-muted">未来方案标题、类型和卖点属于规划版本，不改变项目名称和项目设置。</p>}
   <label className="block text-sm">正式规划<select aria-label="正式规划" disabled={busy || dirty} value={assetId} onChange={(e) => choose(e.target.value)} className="w-full mt-1 p-2 bg-surface border border-border rounded-md">{items.map((item) => <option key={item.assetId} value={item.assetId}>{item.name} · 版本 {item.revision}</option>)}</select></label>
   {selected?.locked && <p role="alert" className="text-warning text-sm">该资产或来源候选已锁定，不能修订。解锁入口尚未就绪。</p>}
   {selected?.fields?.map((field) => <label key={field.path} className="block text-sm">{field.label}<Textarea aria-label={field.label} rows={2} maxLength={field.path === '/title' ? 200 : 8000} value={draft[field.path] || ''} disabled={busy || readOnly || selected.locked} onChange={(e) => { setDraft({ ...draft, [field.path]: e.target.value }); invalidate(); }} /></label>)}
   <label className="block text-sm">修订理由<Input aria-label="规划修订理由" maxLength={2000} disabled={busy || readOnly} value={reason} onChange={(e) => { setReason(e.target.value); invalidate(); }} /></label>
   <label className="block text-sm">从该章起（含本章）<select aria-label="规划生效章节" disabled={busy || readOnly} value={chapterId} onChange={(e) => { setChapterId(e.target.value); invalidate(); }} className="w-full mt-1 p-2 bg-surface border border-border rounded-md"><option value="">选择未来空章节</option>{chapters.map((chapter) => <option key={chapter.id} value={chapter.id}>第 {chapter.position} 章 · {chapter.title}</option>)}</select></label>
   {!chapters.length && <p className="text-sm text-warning">请先在正文页创建后续写作的空章节。</p>}
   {preview && <div className="space-y-3 border border-border rounded-md p-3 text-sm"><p>从第 {preview.position} 章起 · 新版本 {preview.revision}</p>{changed.map((field) => <div key={field.path}><p className="font-medium">{field.label}</p><p className="whitespace-pre-wrap text-text-sub">修改前：{before[field.path] || '未填写'}</p><p className="whitespace-pre-wrap">修改后：{after[field.path] || '未填写'}</p></div>)}{!changed.length && <p>没有文字变化。</p>}<p>保留此前 {preview.protectedChapters} 章；后续 {preview.futureChapters} 章的路线、场景、人物及约束需要人工复核。</p><p className="text-text-sub">旧设定下的共创候选将失效，未来章节与全局资产的待处理修改将标为冲突。不会自动重写正文。</p>{!preview.canApply && <p role="alert" className="text-warning">{preview.blockedReason}</p>}</div>}
   {error && <p role="alert" className="text-danger text-sm">{error}</p>}
   <div className="flex flex-wrap justify-end gap-2"><Button disabled={busy} onClick={discard}>取消修订</Button><Button disabled={busy || readOnly || selected?.locked || !draft['/title']?.trim() || !chapterId || !reason.trim()} onClick={() => void prepare()}>预览规划差异</Button><Button variant="primary" disabled={busy || readOnly || !preview?.canApply} onClick={() => void confirm()}>确认规划修订</Button></div>
  </div></Modal>
 </Card>;
}
function PlanningDetails({ item, text, title }: { item: ConstraintView; text: string; title: string }) { const fields = parse(text); return <div className="space-y-1"><p className="font-medium">{title}</p>{item.fields?.map((field) => <p key={field.path} className="whitespace-pre-wrap text-text-sub">{field.label}：{fields[field.path] || '未填写'}</p>)}</div>; }
