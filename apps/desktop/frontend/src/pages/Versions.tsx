import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
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
  const [versions, setVersions] = useState<Version[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [label, setLabel] = useState('');
  const [note, setNote] = useState('');
  const [exporting, setExporting] = useState<{ key: string; label: string } | null>(null);
  const [checks, setChecks] = useState<Record<string, boolean>>({
    candidates: false,
    characters: true,
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
    });
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
    } finally {
      setLoading(false);
    }
  };

  const handleExport = (format: (typeof exportFormats)[number]) => {
    setExporting({ key: format.key, label: format.label });
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
                              <span>约 {v.wordCount.toLocaleString()} 字</span>
                            </div>
                          </div>
                          <Button variant="ghost" size="sm" onClick={() => navigate('/chapters')}>
                            查看
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
                    <span className="text-sm text-text">{fmt.label}（尚未就绪）</span>
                  </button>
                ))}
              </div>
            </section>
          </div>

          <div className="col-span-4">
            <Card className="sticky top-0">
              <h2 className="text-sm font-medium text-text mb-4">交付清单</h2>
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
          <div className="text-center py-2">
            <p className="text-sm text-text mb-4">
              <strong className="text-primary">{exporting.label}</strong> 尚未接入后端导出能力。
            </p>
            <p className="text-xs text-text-sub mb-5">当前不会创建文件或伪造导出任务。</p>
            <Button variant="primary" onClick={() => setExporting(null)}>
              知道了
            </Button>
          </div>
        )}
      </Modal>
    </div>
  );
}
