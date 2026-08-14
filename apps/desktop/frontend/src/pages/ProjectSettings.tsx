import { FolderOpen, Info } from 'lucide-react';
import { Card } from '../components/ui/Card';
import { Input, Textarea } from '../components/ui/Input';
import { useApp } from '../store/useApp';

/** 展示后端项目元数据；未存在的修改与删除命令不会伪装为已保存。 */
export default function ProjectSettings() {
  const { state } = useApp();
  const project = state.backendProject;

  if (!project) return null;
  return (
    <div className="space-y-6 animate-fade-up max-w-4xl">
      <div>
        <h1 className="text-2xl font-display text-text">项目设置</h1>
        <p className="text-sm text-text-sub mt-1">当前项目的权威信息来自项目数据库。</p>
      </div>
      <Card className="space-y-5">
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div><label className="block text-xs text-text-sub mb-1.5">作品标题</label><Input value={project.title} readOnly /></div>
          <div><label className="block text-xs text-text-sub mb-1.5">类型</label><Input value={project.genre} readOnly /></div>
        </div>
        <div><label className="block text-xs text-text-sub mb-1.5">简介</label><Textarea value={project.summary} readOnly rows={5} /></div>
        <div>
          <label className="block text-xs text-text-sub mb-1.5">项目目录</label>
          <div className="flex items-center gap-2 text-sm text-text-sub bg-surface-hover border border-border rounded-md px-3 py-2">
            <FolderOpen className="w-4 h-4 shrink-0" /><span className="break-all">{project.path}</span>
          </div>
        </div>
      </Card>
      <div className="flex gap-3 rounded-lg border border-primary/15 bg-primary-subtle p-4 text-sm text-text-sub">
        <Info className="w-4 h-4 text-primary shrink-0 mt-0.5" />
        <p>当前后端尚未提供项目元数据修改和删除命令，因此这些危险操作不会在界面中伪造成功。模型连接与应用级创作方案请从顶部“配置模型”进入。</p>
      </div>
    </div>
  );
}
