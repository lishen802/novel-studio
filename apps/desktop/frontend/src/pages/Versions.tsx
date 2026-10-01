import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useApp } from '../store/useApp';
import {
  GitBranch,
  Plus,
  Tag,
  FileText,
  BookOpen,
  FileCode2,
  Download,
  CheckCircle2,
  Circle,
} from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { clsx } from 'clsx';
import type { Version } from '../types';
import { studio } from '../api/studio';
import { Button } from '../components/ui/Button';
import { Card } from '../components/ui/Card';
import { Input, Textarea } from '../components/ui/Input';
import { Modal } from '../components/ui/Modal';
import { EmptyState } from '../components/ui/EmptyState';

const exportFormats: { key: string; label: string; icon: LucideIcon }[] = [
  { key: 'word', label: '导出为 Word', icon: FileText },
  { key: 'markdown', label: '导出为 Markdown', icon: FileCode2 },
  { key: 'plain', label: '导出为纯文本', icon: BookOpen },
  { key: 'epub', label: '生成 EPUB', icon: Download },
];

const checklistItems = [
  { id: 'candidates', label: '所有候选已处理' },
  { id: 'characters', label: '人物设定完整' },
  { id: 'timeline', label: '时间线自洽' },
];

export default function Versions() {
  const navigate = useNavigate();
  const {dispatch} = useApp();
  const [restoreTarget,setRestoreTarget] = useState<Version|null>(null);
  const [error,setError] = useState('');
  const [exportResult,setExportResult] = useState<Awaited<ReturnType<typeof studio.exportManuscript>>|null>(null);
  const [versions, setVersions] = useState<Version[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [label, setLabel] = useState('');
  const [note, setNote] = useState('');
  const [exporting, setExporting] = useState<{ key: string; label: string } | null>(null);
  const [checks, setChecks] = useState<Record<string, boolean>>({
    candidates: false,
    characters: false,
    timeline: false,
  });

  useEffect(() => {
    let mounted = true;
    studio.snapshots().then((data) => {
      if (mounted) setVersions(data.map((snapshot) => ({
        id: snapshot.id,
        projectId: '',
        label: snapshot.name,
        wordCount: 0,
        createdAt: snapshot.createdAt,
        note: snapshot.kind === 'manual' ? '手动项目快照' : snapshot.kind,
        isTagged: true,
      })));
    }).catch(reason => {if(mounted)setError(reason instanceof Error ? reason.message : '快照读取失败');});
    return () => {
      mounted = false;
    };
  }, []);

  const openModal = () => {
    setLabel(`v0.${versions.length + 1}`);
    setNote('');
    setModalOpen(true);
  };

  const handleCreate = async () => {
    if (!label.trim()) return;
    setLoading(true);
    try {
      const snapshot = await studio.createSnapshot(label.trim());
      const v: Version = {
        id: snapshot.id,
        projectId: '',
        label: snapshot.name,
        wordCount: 0,
        createdAt: snapshot.createdAt,
        note: note.trim() || '手动项目快照',
        isTagged: true,
      };
      setVersions((prev) => [v, ...prev]);
      setModalOpen(false);
    } catch(reason) {setError(reason instanceof Error ? reason.message : '创建快照失败');}
    finally {
      setLoading(false);
    }
  };

  const handleExport = (format: (typeof exportFormats)[number]) => {
    setExportResult(null);setError('');setExporting({ key: format.key, label: format.label });
  };

  const toggleCheck = (id: string) => {
    setChecks((prev) => ({ ...prev, [id]: !prev[id] }));
  };

  const completedCount = Object.values(checks).filter(Boolean).length;

  return (
    <div className="h-full flex flex-col animate-fade-up">
      <header className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-xl font-medium text-text flex items-center gap-2">
            <GitBranch className="w-5 h-5 text-primary" />
            版本与交付
          </h1>
          <p className="text-sm text-text-sub mt-1">管理作品版本，导出稿件并核对交付清单。</p>
        </div>
        <Button variant="primary" onClick={openModal}>
          <Plus className="w-4 h-4" /> 创建新版本
        </Button>
      </header>

      {error && <p role="alert" className="text-sm text-danger mb-3">{error}</p>}
      <div className="flex-1 overflow-auto">
        <div className="grid grid-cols-12 gap-6">
          <div className="col-span-8 space-y-6">
            <section>
              <h2 className="text-sm font-medium text-text mb-3">版本时间线</h2>
              {versions.length === 0 ? (
                <Card>
                  <EmptyState
                    icon={GitBranch}
                    title="暂无版本"
                    description="点击右上角创建第一个版本，保存当前进度。"
                    action={{ label: '创建版本', onClick: openModal }}
                  />
                </Card>
              ) : (
                <div className="relative pl-4 border-l border-border space-y-4">
                  {versions.map((v) => (
                    <div key={v.id} className="relative">
                      <span className="absolute -left-[21px] top-4 w-2.5 h-2.5 rounded-full bg-primary border-2 border-surface" />
                      <Card className="ml-3">
                        <div className="flex items-start justify-between">
                          <div>
                            <div className="flex items-center gap-2 mb-1">
                              <h3 className="text-base font-medium text-text">{v.label}</h3>
                              {v.isTagged && (
                                <span className="inline-flex items-center gap-1 text-xs text-primary bg-primary-subtle px-2 py-0.5 rounded border border-primary/20">
                                  <Tag className="w-3 h-3" /> 已标记
                                </span>
                              )}
                            </div>
                            <p className="text-xs text-text-muted mb-2">{v.createdAt}</p>
                            {v.note && <p className="text-sm text-text-sub mb-3">{v.note}</p>}
                            <div className="flex items-center gap-4 text-xs text-text-sub">
                              <span>完整项目快照</span>
                            </div>
                          </div>
                          <Button variant="ghost" size="sm" onClick={() => setRestoreTarget(v)}>
                            恢复快照
                          </Button>
                        </div>
                      </Card>
                    </div>
                  ))}
                </div>
              )}
            </section>

            <section>
              <h2 className="text-sm font-medium text-text mb-3">导出稿件</h2>
              <div className="grid grid-cols-2 gap-3">
                {exportFormats.map((fmt) => (
                  <button
                    key={fmt.key}
                    onClick={() => handleExport(fmt)}
                    className="flex items-center gap-3 p-4 bg-surface-elevated border border-border rounded-lg hover:border-primary/40 hover:bg-surface-hover transition-colors text-left"
                  >
                    <fmt.icon className="w-5 h-5 text-primary" />
                    <span className="text-sm text-text">{fmt.label}{['plain','markdown'].includes(fmt.key) ? '' : '（尚未就绪）'}</span>
                  </button>
                ))}
              </div>
            </section>
          </div>

          <div className="col-span-4">
            <Card className="sticky top-0">
              <h2 className="text-sm font-medium text-text mb-4">交付自查</h2>
              <p className="text-xs text-text-sub mb-3">以下为本次人工自查，不代表系统已自动校验。</p>
              <div className="space-y-3">
                {checklistItems.map((item) => {
                  const checked = !!checks[item.id];
                  return (
                    <button
                      key={item.id}
                      onClick={() => toggleCheck(item.id)}
                      className="w-full flex items-start gap-3 text-left group"
                    >
                      {checked ? (
                        <CheckCircle2 className="w-5 h-5 text-success shrink-0" />
                      ) : (
                        <Circle className="w-5 h-5 text-text-muted group-hover:text-text transition-colors shrink-0" />
                      )}
                      <span className={clsx('text-sm', checked ? 'text-text-sub line-through' : 'text-text')}>
                        {item.label}
                      </span>
                    </button>
                  );
                })}
              </div>
              <div className="mt-5 pt-4 border-t border-border">
                <p className="text-xs text-text-muted">
                  已完成 {completedCount} / {checklistItems.length} 项
                </p>
              </div>
            </Card>
          </div>
        </div>
      </div>

      <Modal open={Boolean(restoreTarget)} onClose={()=>{if(!loading)setRestoreTarget(null);}} title="恢复项目快照">
        <div className="space-y-4"><p className="text-sm">恢复「{restoreTarget?.label}」将替换当前项目内容。恢复前系统会自动建立安全快照，可从版本列表恢复。</p>
        {error && <p role="alert" className="text-sm text-danger">{error}</p>}
        <div className="flex justify-end gap-2"><Button disabled={loading} onClick={()=>setRestoreTarget(null)}>取消</Button><Button variant="primary" disabled={loading} onClick={()=>{
          if(!restoreTarget)return;setLoading(true);setError('');
          void studio.restoreSnapshot(restoreTarget.id).then(result=>{dispatch({type:'SET_BACKEND_PROJECT',payload:result.project});setRestoreTarget(null);navigate('/chapters');}).catch(reason=>setError(reason instanceof Error?reason.message:'恢复失败')).finally(()=>setLoading(false));
        }}>确认恢复快照</Button></div></div>
      </Modal>
      <Modal open={modalOpen} onClose={() => setModalOpen(false)} title="创建新版本">
        <div className="space-y-4">
          <div>
            <label className="block text-sm text-text-sub mb-1">版本标签</label>
            <Input value={label} onChange={(e) => setLabel(e.target.value)} placeholder="例如 v0.4 修订" />
          </div>
          <div>
            <label className="block text-sm text-text-sub mb-1">版本备注</label>
            <Textarea
              value={note}
              onChange={(e) => setNote(e.target.value)}
              placeholder="记录本次版本的主要改动……"
              className="h-24"
            />
          </div>
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="ghost" onClick={() => setModalOpen(false)}>
              取消
            </Button>
            <Button variant="primary" onClick={handleCreate} disabled={!label.trim() || loading}>
              {loading ? '创建中……' : '创建'}
            </Button>
          </div>
        </div>
      </Modal>

      <Modal open={!!exporting} onClose={() => setExporting(null)} title="导出确认">
        {exporting && (
          <div className="space-y-4">
            {['plain','markdown'].includes(exporting.key) ? <>
              <p className="text-sm text-text">按章节顺序导出最新自动保存工作稿（含空白稿），文件保存到项目的 exports 文件夹。</p>
              {exportResult && <div className="text-sm space-y-2"><p>已导出 {exportResult.chapterCount} 章，其中 {exportResult.draftCount} 章使用工作稿。</p><p className="break-all select-all" aria-label="导出文件路径">{exportResult.path}</p></div>}
              {error && <p role="alert" className="text-sm text-danger">{error}</p>}
              <Button variant="primary" disabled={loading} onClick={()=>{
                setLoading(true);setError('');
                void studio.exportManuscript(exporting.key).then(setExportResult).catch(reason=>setError(reason instanceof Error?reason.message:'导出失败')).finally(()=>setLoading(false));
              }}>{loading ? '导出中…' : exportResult ? '再次导出' : '确认导出'}</Button>
            </> : <p className="text-sm text-text">{exporting.label} 尚未接入后端导出能力。当前可使用 TXT 或 Markdown。</p>}
          </div>
        )}
      </Modal>
    </div>
  );
}
