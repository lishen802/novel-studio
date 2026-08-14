import { useEffect, useState } from 'react';
import { BookOpen, GitBranch, Layers } from 'lucide-react';
import { studio, type StoryStructureView } from '../../api/studio';
import { Card } from '../../components/ui/Card';
import { useApp } from '../../store/useApp';

/** 从后端结构档案汇总作品大纲，不再显示固定样例。 */
export default function Outline() {
  const { state } = useApp();
  const [structure, setStructure] = useState<StoryStructureView | null>(null);
  useEffect(() => { void studio.storyStructure().then(setStructure); }, []);

  const project = state.backendProject;
  const counts = [
    { label: '大纲节点', value: structure?.outlineNodes.length || 0, icon: Layers },
    { label: '故事线', value: structure?.storyLines.length || 0, icon: GitBranch },
    { label: '场景卡', value: structure?.sceneCards.length || 0, icon: BookOpen },
  ];
  return (
    <div className="space-y-6 animate-fade-up">
      <Card className="space-y-3">
        <h2 className="text-xl font-display text-text">{project?.title || '作品大纲'}</h2>
        <p className="text-sm text-text-sub">{project?.genre || '未设置类型'}</p>
        <p className="text-sm leading-6 text-text-sub">{project?.summary || '尚未填写作品简介。'}</p>
      </Card>
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        {counts.map(({ label, value, icon: Icon }) => (
          <Card key={label} className="flex items-center gap-4">
            <Icon className="w-5 h-5 text-primary" /><div><p className="text-2xl text-text">{value}</p><p className="text-xs text-text-sub">{label}</p></div>
          </Card>
        ))}
      </div>
    </div>
  );
}
