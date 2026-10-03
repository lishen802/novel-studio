import { useCallback, useEffect, useRef, useState } from 'react';
import type { ChapterView, ConstraintCommand, ConstraintPreview, ConstraintRequest, ConstraintView } from '../api/studio';
import { FormalCharacterAI, type FormalCharacterAPI } from './FormalCharacterAI';
import { Button } from './ui/Button';
import { Card } from './ui/Card';
import { Input, Textarea } from './ui/Input';
import { Modal } from './ui/Modal';

export interface CharacterRevisionAPI extends FormalCharacterAPI {
 characterRevisions: () => Promise<ConstraintView[]>;
 listChapters: () => Promise<ChapterView[]>;
 previewConstraintChange: (request: ConstraintRequest) => Promise<ConstraintPreview>;
 applyConstraintChange: (command: ConstraintCommand) => Promise<ConstraintPreview>;
}
interface Profile { name: string; aliases: string[]; role: string; summary: string; motivation: string; fear: string; secret: string; arc: string }
const fields = { name: '姓名', aliases: '别名', role: '角色定位', summary: '简介', motivation: '动机', fear: '恐惧', secret: '秘密', arc: '人物弧' } as const;
const roles: Record<string, string> = { protagonist: '主角', supporting: '配角', antagonist: '反派', minor: '次要' };
const empty: Profile = { name: '', aliases: [], role: 'minor', summary: '', motivation: '', fear: '', secret: '', arc: '' };
const parse = (text: string): Profile => ({ ...empty, ...JSON.parse(text) });
const message = (e: unknown) => e instanceof Error ? e.message : String(e);
const display = (profile: Profile, field: keyof Profile) => field === 'aliases' ? profile.aliases.join('、') : field === 'role' ? roles[profile.role] || profile.role : profile[field];

/** 正式人物编辑先核对逐字段差异，未确认输入不得因关闭或导航而丢失。 */
export function CharacterRevisions({ api, registerBeforeLeave, readOnly = false }: { api: CharacterRevisionAPI; registerBeforeLeave?: (handler: () => Promise<void>) => () => void; readOnly?: boolean }) {
 const [items, setItems] = useState<ConstraintView[]>([]);
 const [chapters, setChapters] = useState<ChapterView[]>([]);
 const [open, setOpen] = useState(false);
 const [candidateId, setCandidateId] = useState('');
 const [assetId, setAssetId] = useState('');
 const [draft, setDraft] = useState<Profile>(empty);
 const [aliases, setAliases] = useState('');
 const [reason, setReason] = useState('');
 const [chapterId, setChapterId] = useState('');
 const [preview, setPreview] = useState<ConstraintPreview | null>(null);
 const [busy, setBusy] = useState(false);
 const [loading, setLoading] = useState(true);
 const [error, setError] = useState('');
 const [notice, setNotice] = useState('');
 const [reload, setReload] = useState(0);
 const refreshCandidates = useCallback(async () => { const views = await api.characterRevisions(); views.forEach((view) => parse(view.current)); setItems(views); }, [api]);
 const key = useRef('');
 const authorDraft = useRef({ profile: empty, aliases: '' });
 const selected = items.find((item) => item.assetId === assetId);
 const text = JSON.stringify({ ...draft, aliases: aliases.split(/[、,，\n]/).map((value) => value.trim()).filter(Boolean) });
 const dirty = open && (!!candidateId || !!reason || !!chapterId || text !== (selected ? JSON.stringify(parse(selected.current)) : JSON.stringify(empty)));
 const pending = useRef({ dirty, busy }); pending.current = { dirty, busy };
 useEffect(() => {
  if ((!dirty && !busy) || !registerBeforeLeave) return;
  return registerBeforeLeave(async () => { if (pending.current.dirty || pending.current.busy) throw new Error('人物修订尚未确认，请完成核对或明确取消修订。'); });
 }, [dirty, busy, registerBeforeLeave]);
 useEffect(() => {
  let active = true; setLoading(true);
  void Promise.all([api.characterRevisions(), api.listChapters()]).then(([views, rows]) => {
   if (active) { views.forEach((view) => { parse(view.current); parse(view.original); view.changes.forEach((change) => parse(change.text)); }); setItems(views); setChapters(rows); setError(''); }
  }).catch((e) => { if (active) setError(`读取人物版本失败：${message(e)}`); }).finally(() => { if (active) setLoading(false); });
  return () => { active = false; };
 }, [api, reload]);
 const invalidate = () => { setPreview(null); key.current = ''; setError(''); };
 const choose = (id: string) => { const item = items.find((item) => item.assetId === id); setAssetId(id); setCandidateId(''); const profile = item ? parse(item.current) : empty; setDraft(profile); setAliases(profile.aliases.join('、')); authorDraft.current = { profile, aliases: profile.aliases.join('、') }; setReason(''); setChapterId(''); invalidate(); };
 const discard = () => { if (!busy) { setOpen(false); invalidate(); } };
 const close = () => { if (busy) return; if (dirty) { setError('还有未确认的人物修改，请继续核对或点击取消修订。'); return; } discard(); };
 const prepare = async () => {
  if (busy || readOnly || !selected) return;
  setBusy(true); setError(''); setPreview(null);
  try { setPreview(await api.previewConstraintChange({ assetId, intent: 'future_only', chapterId, text, reason, ...(candidateId ? { candidateId } : {}) })); key.current = crypto.randomUUID(); }
  catch (e) { setError(message(e)); } finally { setBusy(false); }
 };
 const confirm = async () => {
  if (busy || readOnly || !preview?.canApply) return;
  setBusy(true); setError('');
  try {
   const receipt = await api.applyConstraintChange({ request: preview.request, digest: preview.digest, idempotencyKey: key.current });
   setOpen(false); setPreview(null); setReason(''); setChapterId('');
   setNotice(`人物版本 ${receipt.revision} 已保存，从第 ${receipt.position} 章起生效。`); setReload((value) => value + 1);
  } catch (e) { setError(message(e)); } finally { setBusy(false); }
 };
 const before = preview ? parse(preview.before) : null;
 const after = preview ? parse(preview.request.text) : null;
 const changed = before && after ? (Object.keys(fields) as (keyof Profile)[]).filter((field) => JSON.stringify(before[field]) !== JSON.stringify(after[field])) : [];
 return <Card className="space-y-3">
  <div className="flex flex-wrap justify-between items-center gap-3"><div><h3 className="font-medium">正式人物修订</h3><p className="text-sm text-text-sub">保留原档案，新设定从选定章节起使用。</p></div><Button disabled={loading || busy || readOnly || !items.length} onClick={() => { choose(items[0].assetId); setNotice(''); setOpen(true); }}>修订人物档案</Button></div>
  <FormalCharacterAI api={api} items={items} readOnly={readOnly} registerBeforeLeave={registerBeforeLeave} onReady={refreshCandidates} />
  {loading && <p role="status" className="text-sm text-text-sub">正在读取人物版本…</p>}
  {!loading && !items.length && !error && <p className="text-sm text-text-muted">正式入档后可修订人物。旧手工或导入人物仍可查看，纳入版本管理尚未就绪。</p>}
  {readOnly && <p className="text-sm text-text-muted">当前项目为只读，不能保存修订。</p>}
  {items.filter((item) => item.changes.length > 0).map((item) => <details key={item.assetId} className="border-t border-border pt-2 text-sm"><summary className="cursor-pointer">{item.name} · 当前版本 {item.revision} · 查看修订记录</summary><div className="space-y-3 mt-3"><ProfileDetails profile={parse(item.original)} title="原始档案" />{item.changes.map((change) => <ProfileDetails key={change.revision} profile={parse(change.text)} title={`从第 ${change.position} 章起 · 版本 ${change.revision} · ${change.reason}${change.candidateId ? ` · AI 候选 ${change.candidateId}` : ""}`} />)}</div></details>)}
  {notice && <p role="status" className="text-sm text-primary">{notice}</p>}
  {error && !open && <div><p role="alert" className="text-sm text-danger">{error}</p><Button disabled={loading} onClick={() => setReload((value) => value + 1)}>重新读取版本</Button></div>}
  <Modal open={open} onClose={close} title="修订正式人物档案">
   <div className="space-y-4 max-h-[70vh] overflow-auto">
    <p className="text-sm text-text-sub">仅从未来空章节起生效；旧章和原始档案保留。故事内状态变化和历史重写尚未就绪。</p>
    <label className="block text-sm">正式人物<select aria-label="正式人物" disabled={busy || dirty} value={assetId} onChange={(e) => choose(e.target.value)} className="w-full mt-1 p-2 bg-surface border border-border rounded-md">{items.map((item) => <option key={item.assetId} value={item.assetId}>{item.name} · 版本 {item.revision}</option>)}</select></label>
    {selected?.locked && <p role="alert" className="text-sm text-warning">人物或来源候选的人物字段已锁定，不能直接修订。解锁入口尚未就绪。</p>}
    <label className="block text-sm">修改来源<select aria-label="人物修改来源" disabled={busy || readOnly || selected?.locked} value={candidateId} onChange={(e) => {
     const id = e.target.value; if (!candidateId) authorDraft.current = { profile: draft, aliases }; setCandidateId(id); const suggestion = selected?.candidateSuggestions?.find((item) => item.candidateId === id); const profile = suggestion ? parse(suggestion.text) : authorDraft.current.profile; setDraft(profile); setAliases(suggestion ? profile.aliases.join('、') : authorDraft.current.aliases); invalidate();
    }} className="w-full mt-1 p-2 bg-surface border border-border rounded-md"><option value="">作者直接编辑</option>{selected?.candidateSuggestions?.map((item, index) => <option key={item.candidateId} value={item.candidateId}>已审阅 AI 候选 {index + 1} · {item.candidateId.startsWith('formal-revision:') ? '正式人物修改' : '共创修改'}</option>)}</select></label>
    {candidateId ? <p className="text-sm text-text-sub">仅采用当前人物的档案字段；同组关系、规则和其他人物不会入档。候选来源：{candidateId.startsWith('formal-revision:') ? '最新正式人物版本的 AI 修改' : '共创字段修改'}。如需继续手工编辑，请切回作者直接编辑。</p> : <p className="text-xs text-text-muted">共创中整组保存且通过审阅的字段修改候选，基线与当前档案一致时可在此选择。重新生成、部分采用或已过期的候选暂不能入档。</p>}
    {(Object.keys(fields) as (keyof Profile)[]).map((field) => <label key={field} className="block text-sm">{fields[field]}{field === 'role' ? <select aria-label={fields[field]} value={draft.role} disabled={busy || readOnly || selected?.locked || !!candidateId} onChange={(e) => { setDraft({ ...draft, role: e.target.value }); invalidate(); }} className="w-full mt-1 p-2 bg-surface border border-border rounded-md">{Object.entries(roles).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select> : field === 'name' || field === 'aliases' ? <Input aria-label={fields[field]} maxLength={field === 'name' ? 200 : 2000} disabled={busy || readOnly || selected?.locked || !!candidateId} value={field === 'aliases' ? aliases : draft.name} onChange={(e) => { if (field === 'aliases') setAliases(e.target.value); else setDraft({ ...draft, name: e.target.value }); invalidate(); }} /> : <Textarea aria-label={fields[field]} maxLength={8000} rows={2} disabled={busy || readOnly || selected?.locked || !!candidateId} value={draft[field]} onChange={(e) => { setDraft({ ...draft, [field]: e.target.value }); invalidate(); }} />}</label>)}
    <p className="text-xs text-text-muted">别名用顿号或逗号分隔。角色定位是叙事职责；人物状态和关系不在本次修改范围。</p>
    <label className="block text-sm">修订理由<Input aria-label="人物修订理由" maxLength={2000} disabled={busy || readOnly} value={reason} onChange={(e) => { setReason(e.target.value); invalidate(); }} /></label>
    <label className="block text-sm">从该章起（含本章）<select aria-label="人物生效章节" disabled={busy || readOnly} value={chapterId} onChange={(e) => { setChapterId(e.target.value); invalidate(); }} className="w-full mt-1 p-2 bg-surface border border-border rounded-md"><option value="">选择未来空章节</option>{chapters.map((chapter) => <option key={chapter.id} value={chapter.id}>第 {chapter.position} 章 · {chapter.title}</option>)}</select></label>
    {!chapters.length && <p className="text-sm text-warning">请先在正文页创建一个用于后续写作的空章节。</p>}
    {preview && before && after && <div className="space-y-3 border border-border rounded-md p-3 text-sm"><p>从第 {preview.position} 章起 · 新版本 {preview.revision}</p>{changed.map((field) => <div key={field}><p className="font-medium">{fields[field]}</p><p className="whitespace-pre-wrap text-text-sub">修改前：{display(before, field) || '未填写'}</p><p className="whitespace-pre-wrap">修改后：{display(after, field) || '未填写'}</p></div>)}{!changed.length && <p>没有字段变化。</p>}<p>保留此前 {preview.protectedChapters} 章；后续 {preview.futureChapters} 章的路线、场景及人物引用需要人工复核。</p><p className="text-text-sub">旧设定下的共创候选将失效，未来章节与全局资产的待处理修改将标为冲突。不会自动重写正文。</p>{!preview.canApply && <p role="alert" className="text-warning">{preview.blockedReason}</p>}</div>}
    {error && <p role="alert" className="text-sm text-danger">{error}</p>}
    <div className="flex flex-wrap justify-end gap-2"><Button disabled={busy} onClick={discard}>取消修订</Button><Button disabled={busy || readOnly || selected?.locked || !draft.name.trim() || !chapterId || !reason.trim()} onClick={() => void prepare()}>预览人物差异</Button><Button variant="primary" disabled={busy || readOnly || !preview?.canApply} onClick={() => void confirm()}>确认人物修订</Button></div>
   </div>
  </Modal>
 </Card>;
}
function ProfileDetails({ profile, title }: { profile: Profile; title: string }) {
 return <div className="space-y-1"><p className="font-medium">{title}</p>{(Object.keys(fields) as (keyof Profile)[]).map((field) => <p key={field} className="whitespace-pre-wrap text-text-sub">{fields[field]}：{display(profile, field) || '未填写'}</p>)}</div>;
}
