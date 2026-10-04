import { useEffect, useMemo, useRef, useState } from 'react';
import type { ChapterView, ConstraintView, FormalOutlineReorderPreview } from '../api/studio';
import { Button } from './ui/Button';
import { Card } from './ui/Card';
import { Input } from './ui/Input';
import { Modal } from './ui/Modal';

export interface FormalOutlineReorderAPI {
  listChapters: () => Promise<ChapterView[]>;
  planningRevisions: () => Promise<ConstraintView[]>;
  formalOutlinePositionsAtChapter: (chapterId: string) => Promise<Record<string, number>>;
  previewFormalOutlineReorder: (firstAssetId: string, secondAssetId: string, chapterId: string, reason: string) => Promise<FormalOutlineReorderPreview>;
  confirmFormalOutlineReorder: (command: { request: FormalOutlineReorderPreview['request']; digest: string; idempotencyKey: string }) => Promise<FormalOutlineReorderPreview>;
}

type Node = { id: string; title: string; level: string; parentId: string; introducedAt: number; retiredAt: number };

// 正式节点重排只提交相邻目标的未来位置修订，预览和确认均由后端复核。
export function FormalOutlineReorder({ api, nodes, readOnly, registerBeforeLeave, onApplied }: {
  api: FormalOutlineReorderAPI; nodes: Node[]; readOnly: boolean;
  registerBeforeLeave?: (handler: () => Promise<void>) => () => void; onApplied?: () => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const [chapters, setChapters] = useState<ChapterView[]>([]);
  const [assets, setAssets] = useState<ConstraintView[]>([]);
  const [positions, setPositions] = useState<Record<string, number>>({});
  const [chapterId, setChapterId] = useState('');
  const [firstAssetId, setFirstAssetId] = useState('');
  const [secondAssetId, setSecondAssetId] = useState('');
  const [reason, setReason] = useState('');
  const [preview, setPreview] = useState<FormalOutlineReorderPreview | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const key = useRef(crypto.randomUUID());
  const chapterRequest = useRef(0);
  const dirty = useRef(false);
  dirty.current = open && (busy || !!chapterId || !!firstAssetId || !!secondAssetId || !!reason.trim() || !!preview);
  useEffect(() => {
    if (!open || !registerBeforeLeave) return;
    return registerBeforeLeave(async () => { if (dirty.current) throw new Error('节点顺序尚未确认，请先完成或明确取消。'); });
  }, [open, registerBeforeLeave]);

  const options = useMemo(() => nodes.filter(node => node.level === 'chapter' && node.retiredAt === 0).flatMap(node =>
    assets.filter(asset => asset.kind === 'outline_node' && asset.typedId === node.id && !asset.locked).map(asset => ({ ...node, assetId: asset.assetId }))), [nodes, assets]);
  const first = options.find(item => item.assetId === firstAssetId);
  const secondOptions = options.filter(item => first && item.parentId === first.parentId && positions[item.id] === positions[first.id] + 1);
  const selectedChapter = chapters.find(item => item.id === chapterId);
  const currentOrder = options.filter(item => first && item.parentId === first.parentId && positions[item.id] > 0).sort((a, b) => positions[a.id] - positions[b.id]);

  const start = async () => {
    if (readOnly) return;
    setOpen(true); setBusy(true); setError(''); setNotice('');
    try {
      const [nextChapters, nextAssets] = await Promise.all([api.listChapters(), api.planningRevisions()]);
      setChapters(nextChapters); setAssets(nextAssets);
    } catch (e) { setError(e instanceof Error ? e.message : String(e)); }
    finally { setBusy(false); }
  };
  const chooseChapter = async (id: string) => {
    const request = ++chapterRequest.current;
    setError('');
    setChapterId(id); setFirstAssetId(''); setSecondAssetId(''); setPreview(null); setPositions({});
    if (!id) return;
    try { const next = await api.formalOutlinePositionsAtChapter(id); if (request === chapterRequest.current) setPositions(next); }
    catch (e) { if (request === chapterRequest.current) setError(e instanceof Error ? e.message : String(e)); }
  };
  const askPreview = async () => {
    setBusy(true); setError(''); setPreview(null);
    try {
      const result = await api.previewFormalOutlineReorder(firstAssetId, secondAssetId, chapterId, reason);
      setPreview(result); key.current = crypto.randomUUID();
    } catch (e) { setError(e instanceof Error ? e.message : String(e)); }
    finally { setBusy(false); }
  };
  const confirm = async () => {
    if (!preview) return;
    setBusy(true); setError('');
    try {
      const result = await api.confirmFormalOutlineReorder({ request: preview.request, digest: preview.digest, idempotencyKey: key.current });
      setNotice(`已从第 ${result.position} 章起将“${result.secondTitle}”排在“${result.firstTitle}”之前。`);
      setOpen(false); setPreview(null); setReason(''); setChapterId(''); setFirstAssetId(''); setSecondAssetId('');
      try { await onApplied?.(); }
      catch { setNotice('未来顺序已保存，但档案刷新失败，请重新打开此页。'); }
    } catch (e) { setError(e instanceof Error ? e.message : String(e)); }
    finally { setBusy(false); }
  };
  const close = () => { if (busy) return; if (dirty.current) { setError('请完成确认，或点击“取消输入并关闭”。'); return; } setOpen(false); };

  return <Card className="space-y-3">
    <h3 className="text-lg font-display text-text">调整未来章节目标顺序</h3>
    <p className="text-sm text-text-sub">仅交换同一故事弧下相邻的两个正式章节目标，从未来空章起生效；已写旧章与原始结构保留。</p>
    <Button disabled={readOnly || !nodes.some(node => node.level === 'chapter' && node.retiredAt === 0)} onClick={() => void start()}>预览未来顺序</Button>
    {notice && <p role="status" className="text-sm text-primary">{notice}</p>}
    <Modal open={open} onClose={close} title="核对未来章节顺序"><div className="space-y-4 max-h-[70vh] overflow-auto">
      <label className="block text-sm">从哪一章起生效<select aria-label="重排生效章节" value={chapterId} disabled={busy} onChange={e => void chooseChapter(e.target.value)} className="w-full mt-1 p-2 bg-surface border border-border rounded-md"><option value="">选择未来空章节</option>{chapters.filter(chapter => chapter.body === '').map(chapter => <option key={chapter.id} value={chapter.id}>第 {chapter.position} 章 · {chapter.title}</option>)}</select></label>
      <label className="block text-sm">原本在前的目标<select aria-label="前一个正式节点" value={firstAssetId} disabled={busy || !chapterId} onChange={e => { setFirstAssetId(e.target.value); setSecondAssetId(''); setPreview(null); }} className="w-full mt-1 p-2 bg-surface border border-border rounded-md"><option value="">选择正式节点</option>{options.filter(item => positions[item.id] > 0 && item.introducedAt <= (selectedChapter?.position ?? 0)).map(item => <option key={item.assetId} value={item.assetId}>{item.title}</option>)}</select></label>
      {currentOrder.length > 0 && <p className="text-xs text-text-sub">该故事弧当前顺序：{currentOrder.map(item => item.title).join(' → ')}</p>}
      <label className="block text-sm">紧随其后的目标<select aria-label="后一个正式节点" value={secondAssetId} disabled={busy || !firstAssetId} onChange={e => { setSecondAssetId(e.target.value); setPreview(null); }} className="w-full mt-1 p-2 bg-surface border border-border rounded-md"><option value="">选择相邻节点</option>{secondOptions.map(item => <option key={item.assetId} value={item.assetId}>{item.title}</option>)}</select></label>
      <label className="block text-sm">调整理由<Input aria-label="节点重排理由" value={reason} maxLength={2000} disabled={busy} onChange={e => { setReason(e.target.value); setPreview(null); }} /></label>
      <Button disabled={busy || !firstAssetId || !secondAssetId || !chapterId || !reason.trim()} onClick={() => void askPreview()}>预览调整范围</Button>
      {preview && <Card className="space-y-2 text-sm"><p>第 {preview.position} 章起：{preview.secondTitle} → {preview.firstTitle}。</p><p className="text-text-sub">两个正式节点各新增一个未来生效版本；旧章、原始节点和正文不修改。下游候选需要重新核对。</p><Button variant="primary" disabled={busy} onClick={() => void confirm()}>确认未来顺序</Button></Card>}
      {error && <p role="alert" className="text-sm text-danger">{error}</p>}
      <div className="flex justify-end"><Button disabled={busy} onClick={() => { setOpen(false); setPreview(null); setReason(''); setChapterId(''); setFirstAssetId(''); setSecondAssetId(''); }}>取消输入并关闭</Button></div>
    </div></Modal>
  </Card>;
}
