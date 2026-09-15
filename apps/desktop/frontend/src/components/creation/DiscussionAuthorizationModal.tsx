import { Button } from '../ui/Button';
import { Modal } from '../ui/Modal';
import type { DiscussionAuthorizationView } from '../../api/studio';

interface Props {
  open: boolean;
  preview?: DiscussionAuthorizationView;
  confirming?: boolean;
  onClose: () => void;
  onConfirm: () => void;
}

export function DiscussionAuthorizationModal({open, preview, confirming, onClose, onConfirm}: Props) {
  return <Modal open={open} onClose={onClose} title="确认讨论授权">
    {preview && <div className="space-y-3 p-1">
      <p className="text-sm text-text-sub">确认后才会调用模型。同一目标和预算内可以连续讨论，不必每条消息再确认费用。</p>
      <p className="text-sm">模型：{preview.provider} / {preview.model}</p>
      <p className="text-sm">最多 {preview.maxCalls} 次调用，单次 {(preview.maxTurnTokens ?? 0).toLocaleString()} Token，累计 {(preview.maxTotalTokens ?? 0).toLocaleString()} Token。</p>
      <p className="text-sm">目标：候选第 {preview.candidateVersion} 版{preview.entityRef ? ` · ${preview.entityRef}` : ''}{(preview.fieldPaths ?? []).length ? ` · ${(preview.fieldPaths ?? []).join('、')}` : ''}</p>
      <p className="text-sm">有效期至 {preview.expiresAt}</p>
      <p className="text-sm text-text-sub">金额暂不可估算，确认后按你的模型服务商实际用量计费。</p>
      <div className="flex justify-end gap-2 pt-2">
        <Button disabled={confirming} onClick={onClose}>取消</Button>
        <Button variant="primary" disabled={confirming} onClick={onConfirm}>{confirming ? '正在确认…' : '确认并开始讨论'}</Button>
      </div>
    </div>}
  </Modal>;
}
