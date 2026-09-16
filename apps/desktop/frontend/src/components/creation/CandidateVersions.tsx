import { useState } from 'react';
import type { CandidateCompareView, CandidateReviewDiff, CandidateVersionSummaryView } from '../../api/studio';
import { Button } from '../ui/Button';
import { Card } from '../ui/Card';
import { Modal } from '../ui/Modal';
import { StatusBadge } from '../ui/StatusBadge';

// 版本来源、状态与质量结论的中文投影，界面不直接暴露后端枚举。
const originLabels: Record<string, string> = { model: '模型', author: '作者' };
const statusLabels: Record<string, string> = {
  draft: '草稿', selected: '已选定', confirmed: '已确认', superseded: '已替代', stale: '已过期', discarded: '已废弃',
};
const qualityLabels: Record<string, string> = { passed: '质检通过', needs_review: '待复核', blocked: '质检阻断' };
const qualityVariants: Record<string, 'success' | 'warning' | 'danger'> = {
  passed: 'success', needs_review: 'warning', blocked: 'danger',
};

// 差异按分组归并，保持后端返回的字段顺序。
function diffGroups(diffs: CandidateReviewDiff[]): Array<{ group: string; items: CandidateReviewDiff[] }> {
  const groups: Array<{ group: string; items: CandidateReviewDiff[] }> = [];
  for (const diff of diffs) {
    const existing = groups.find((item) => item.group === diff.group);
    if (existing) existing.items.push(diff);
    else groups.push({ group: diff.group, items: [diff] });
  }
  return groups;
}

function QualityBadge({ quality }: { quality: string }) {
  if (!quality) return null;
  const variant = qualityVariants[quality];
  return <StatusBadge variant={variant ?? 'default'}>{qualityLabels[quality] ?? quality}</StatusBadge>;
}

interface Props {
  stage: string;
  baseCandidateId: string;
  versions: CandidateVersionSummaryView[];
  disabled?: boolean;
  canUseVersion: boolean;
  canCompare: boolean;
  onUseVersion: (candidateId: string) => void;
  onCompare: (stage: string, baseCandidateId: string) => Promise<CandidateCompareView>;
}

// 候选版本区：列出当前阶段历史版本，非当前版本可显式切换为工作基线，比较始终只读。
export function CandidateVersions({
  stage, baseCandidateId, versions, disabled, canUseVersion, canCompare, onUseVersion, onCompare,
}: Props) {
  const [open, setOpen] = useState(false);
  const [comparison, setComparison] = useState<CandidateCompareView | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const baseVersion = versions.find((version) => version.current)?.version;

  // 比较只读取差异，不切换工作基线。
  const openCompare = async () => {
    setOpen(true);
    setLoading(true);
    setError('');
    try {
      setComparison(await onCompare(stage, baseCandidateId));
    } catch (err) {
      setComparison(null);
      setError(err instanceof Error ? err.message : '版本比较失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <section aria-label="候选版本" className="space-y-3">
      <div className="flex items-center justify-between">
        <h2 className="text-base font-medium text-text">候选版本</h2>
        {canCompare && (
          <Button size="sm" aria-label="比较版本" disabled={disabled || loading} onClick={() => void openCompare()}>
            比较版本
          </Button>
        )}
      </div>
      {versions.map((version) => (
        <Card key={version.candidateId} padding="none">
          <div className="px-5 py-3 flex flex-wrap items-center gap-2 border-b border-border bg-surface-hover">
            <span className="text-sm font-medium text-text">版本 {version.version}</span>
            {version.current && <StatusBadge variant="info">当前工作版本</StatusBadge>}
            <StatusBadge variant="default">来源：{originLabels[version.origin] ?? version.origin}</StatusBadge>
            <StatusBadge variant={version.status === 'stale' ? 'warning' : version.status === 'confirmed' ? 'success' : 'default'}>
              {statusLabels[version.status] ?? version.status}
            </StatusBadge>
            <QualityBadge quality={version.quality} />
            <span className="text-xs text-text-muted ml-auto">
              {version.createdAt ? new Date(version.createdAt).toLocaleString('zh-CN') : ''}
            </span>
          </div>
          <div className="px-5 py-3 flex items-start justify-between gap-3">
            <p className="text-sm text-text-sub whitespace-pre-wrap flex-1">{version.summary || '暂无版本摘要。'}</p>
            {canUseVersion && !version.current && (
              <Button
                size="sm"
                aria-label={`用作工作基线 版本 ${version.version}`}
                disabled={disabled}
                onClick={() => onUseVersion(version.candidateId)}
              >
                用作工作基线
              </Button>
            )}
          </div>
        </Card>
      ))}

      <Modal open={open} onClose={() => setOpen(false)} title="比较候选版本" className="max-w-3xl">
        <div className="space-y-4 max-h-[70vh] overflow-auto">
          <p className="text-sm text-text-sub">
            以当前工作版本{baseVersion ? `（版本 ${baseVersion}）` : ''}为基线，逐字段展示各版本的差异。比较不会切换工作基线。
          </p>
          {loading && <p className="text-sm text-text-sub">正在读取版本差异…</p>}
          {error && <p role="alert" className="text-sm text-danger">{error}</p>}
          {(comparison?.versions ?? []).map((version) => (
            <div key={version.candidateId} className="p-4 border border-border rounded-lg space-y-3">
              <div className="flex flex-wrap items-center gap-2">
                <p className="text-sm font-medium text-text">版本 {version.version}</p>
                {version.current && <StatusBadge variant="info">基线</StatusBadge>}
                <StatusBadge variant="default">来源：{originLabels[version.origin] ?? version.origin}</StatusBadge>
                <StatusBadge variant="default">{statusLabels[version.status] ?? version.status}</StatusBadge>
              </div>
              {(version.diff ?? []).length === 0 ? (
                <p className="text-sm text-text-sub">与基线一致</p>
              ) : diffGroups(version.diff ?? []).map((group) => (
                <div key={`${version.candidateId}-${group.group}`} className="space-y-2">
                  <p className="text-xs font-medium text-text-sub">{group.group}</p>
                  {group.items.map((diff) => (
                    <div key={`${version.candidateId}-${diff.path}`} className="text-sm border-t border-border pt-2">
                      <p className="text-text-sub">{diff.label}</p>
                      <div className="grid grid-cols-2 gap-2 mt-1">
                        <div>
                          <p className="text-xs text-text-muted">修改前</p>
                          <p className="text-text whitespace-pre-wrap">{diff.before || '（空）'}</p>
                        </div>
                        <div>
                          <p className="text-xs text-text-muted">修改后</p>
                          <p className="text-text whitespace-pre-wrap">{diff.after || '（空）'}</p>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              ))}
            </div>
          ))}
        </div>
      </Modal>
    </section>
  );
}
