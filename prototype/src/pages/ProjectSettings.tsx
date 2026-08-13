import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Save, Check, Trash2, AlertTriangle } from 'lucide-react';
import { currentProject } from '../mock/data';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { Input, Textarea } from '../components/ui/Input';
import { Modal } from '../components/ui/Modal';
import { useApp } from '../store/useApp';
import type { CreativeRole } from '../types';

const profileRoleLabels: Record<CreativeRole, string> = {
  planning: '规划',
  writing: '写作',
  review: '审阅',
  memory: '记忆',
  impact: '影响分析',
};

export default function ProjectSettings() {
  const navigate = useNavigate();
  const { state, dispatch } = useApp();
  const project = state.currentProject || currentProject;
  const [title, setTitle] = useState(project.title);
  const [subtitle, setSubtitle] = useState(project.subtitle);
  const [author, setAuthor] = useState(project.author);
  const [target, setTarget] = useState(project.targetWordCount);
  const [synopsis, setSynopsis] = useState(project.synopsis);
  const [coverColor, setCoverColor] = useState(project.coverColor);
  const [profileId, setProfileId] = useState(state.settings.defaultCreativeProfileId);
  const [roleOverrides, setRoleOverrides] = useState<Partial<Record<CreativeRole, string>>>({});
  const selectedProfile = state.settings.creativeProfiles.find((profile) => profile.id === profileId) || state.settings.creativeProfiles[0];
  const [saved, setSaved] = useState(false);
  const [deleteModalOpen, setDeleteModalOpen] = useState(false);

  const handleSave = () => {
    setSaved(true);
    setTimeout(() => setSaved(false), 2000);
  };

  const handleDelete = () => {
    setDeleteModalOpen(false);
    dispatch({ type: 'SET_PROJECT', payload: null });
    navigate('/');
  };

  return (
    <div className="h-full flex flex-col animate-fade-up">
      <div className="mb-6">
        <h1 className="text-2xl font-display text-text mb-1">项目设置</h1>
        <p className="text-sm text-text-sub">管理当前作品的基本信息与封面样式。</p>
      </div>

      <div className="flex-1 min-h-0 overflow-auto pr-1 pb-6">
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 max-w-5xl">
          <Card className="lg:col-span-2 space-y-5">
            <h2 className="text-base font-medium text-text">基本信息</h2>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label className="block text-xs text-text-sub mb-1.5">作品标题</label>
                <Input value={title} onChange={(e) => setTitle(e.target.value)} />
              </div>
              <div>
                <label className="block text-xs text-text-sub mb-1.5">副标题</label>
                <Input value={subtitle} onChange={(e) => setSubtitle(e.target.value)} />
              </div>
              <div>
                <label className="block text-xs text-text-sub mb-1.5">作者</label>
                <Input value={author} onChange={(e) => setAuthor(e.target.value)} />
              </div>
              <div>
                <label className="block text-xs text-text-sub mb-1.5">目标字数</label>
                <Input
                  type="number"
                  min={0}
                  step={1000}
                  value={target}
                  onChange={(e) => setTarget(Number(e.target.value))}
                />
              </div>
            </div>
            <div>
              <label className="block text-xs text-text-sub mb-1.5">简介</label>
              <Textarea
                value={synopsis}
                onChange={(e) => setSynopsis(e.target.value)}
                rows={6}
                placeholder="输入作品简介..."
              />
            </div>
          </Card>

          <Card className="space-y-5">
            <h2 className="text-base font-medium text-text">封面颜色</h2>
            <div
              className="h-32 rounded-lg border border-border shadow-sm"
              style={{ backgroundColor: coverColor }}
            />
            <div>
              <label className="block text-xs text-text-sub mb-1.5">选择颜色</label>
              <div className="flex items-center gap-3">
                <input
                  type="color"
                  value={coverColor}
                  onChange={(e) => setCoverColor(e.target.value)}
                  className="w-10 h-10 p-0 border-0 rounded cursor-pointer bg-transparent"
                />
                <Input
                  value={coverColor}
                  onChange={(e) => setCoverColor(e.target.value)}
                  placeholder="#3a4a5c"
                  className="flex-1"
                />
              </div>
            </div>
          </Card>
        </div>

        <Card className="max-w-5xl mt-6 border-danger/30">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div>
              <h2 className="text-base font-medium text-danger flex items-center gap-2">
                <AlertTriangle className="w-4 h-4" />
                危险区域
              </h2>
              <p className="text-xs text-text-sub mt-1">删除项目将清空所有章节、设定与历史记录，此操作无法撤销。</p>
            </div>
            <Button variant="danger" onClick={() => setDeleteModalOpen(true)}>
              <Trash2 className="w-4 h-4" />
              删除项目
            </Button>
          </div>
        </Card>

        {selectedProfile && (
          <Card className="max-w-5xl mt-6 space-y-5">
            <div>
              <h2 className="text-base font-medium text-text">本作品的创作方案</h2>
              <p className="text-xs text-text-sub mt-1">项目只选择应用中已经发布的方案；不会在这里保存 API Key 或重复配置模型连接。</p>
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-[1fr_auto] gap-4 items-end">
              <div>
                <label className="block text-xs text-text-sub mb-1.5">使用的应用级方案</label>
                <select
                  value={profileId}
                  onChange={(event) => {
                    setProfileId(event.target.value);
                    setRoleOverrides({});
                  }}
                  className="h-9 px-3 w-full bg-surface border border-border rounded-md text-sm text-text focus:outline-none focus:ring-2 focus:ring-primary/35"
                >
                  {state.settings.creativeProfiles.map((profile) => (
                    <option key={profile.id} value={profile.id}>{profile.name} · v{profile.version}</option>
                  ))}
                </select>
              </div>
              <div className="text-xs text-text-sub bg-surface-hover border border-border rounded-md px-3 py-2">已固定 v{selectedProfile.version}</div>
            </div>
            <div className="rounded-lg border border-border bg-surface-hover px-3 py-2 text-xs text-text-sub">
              应用默认方案改变不会自动影响本作品。只有保存这里的选择或角色覆盖后，后续任务才会切换。
            </div>
            <div>
              <h3 className="text-sm font-medium text-text mb-3">按角色覆盖（可选）</h3>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                {(Object.keys(profileRoleLabels) as CreativeRole[]).map((role) => (
                  <div key={role} className="grid grid-cols-[88px_1fr] gap-3 items-center">
                    <label htmlFor={`override-${role}`} className="text-sm text-text-sub">{profileRoleLabels[role]}</label>
                    <select
                      id={`override-${role}`}
                      value={roleOverrides[role] || ''}
                      onChange={(event) => setRoleOverrides((overrides) => ({
                        ...overrides,
                        [role]: event.target.value || undefined,
                      }))}
                      className="h-9 px-3 w-full bg-surface border border-border rounded-md text-sm text-text focus:outline-none focus:ring-2 focus:ring-primary/35"
                    >
                      <option value="">沿用 {selectedProfile.roleModels[role]}</option>
                      {state.settings.models.map((model) => <option key={model.id} value={model.id}>{model.name}</option>)}
                    </select>
                  </div>
                ))}
              </div>
            </div>
          </Card>
        )}
      </div>

      <div className="pt-4 border-t border-border flex justify-end">
        <Button
          variant={saved ? 'default' : 'primary'}
          onClick={handleSave}
          className={saved ? 'text-success border-success/30' : ''}
        >
          {saved ? <Check className="w-4 h-4" /> : <Save className="w-4 h-4" />}
          {saved ? '已保存' : '保存项目'}
        </Button>
      </div>

      <Modal
        open={deleteModalOpen}
        onClose={() => setDeleteModalOpen(false)}
        title="确认删除项目"
      >
        <div className="space-y-4">
          <p className="text-sm text-text-sub">
            确定要删除《{title}》吗？所有章节、设定与历史记录都会被移除，且无法恢复。
          </p>
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="ghost" size="sm" onClick={() => setDeleteModalOpen(false)}>
              取消
            </Button>
            <Button variant="danger" size="sm" onClick={handleDelete}>
              确认删除
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
