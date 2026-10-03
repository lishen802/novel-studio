import { useEffect, useState } from 'react';
import { BookOpen, GitBranch, Layers } from 'lucide-react';
import { studio, type StoryStructureView } from '../../api/studio';
import { PlanningRevisions } from '../../components/PlanningRevisions';
import { PlanningRearchive } from '../../components/PlanningRearchive';
import { Card } from '../../components/ui/Card';
import { useApp } from '../../store/useApp';
import { useNavigate } from 'react-router-dom';
import { Button } from '../../components/ui/Button';

/** 从后端结构档案汇总作品大纲，不再显示固定样例。 */
export default function Outline() {
  const { state,registerBeforeClose } = useApp();
  const navigate = useNavigate();
  const [structure, setStructure] = useState<StoryStructureView | null>(null);
  const [revisionEpoch, setRevisionEpoch] = useState(0);
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
      <PlanningRevisions key={revisionEpoch} api={studio} kind="proposal" registerBeforeLeave={registerBeforeClose} readOnly={state.backendProject?.accessMode === "read_only"} />
      {state.backendProject?.workbench.archiveStatus === 'confirmed' && <div className="space-y-2"><PlanningRearchive api={studio} readOnly={state.backendProject?.accessMode === 'read_only'} registerBeforeLeave={registerBeforeClose} onApplied={async()=>{setRevisionEpoch(value=>value+1);setStructure(await studio.storyStructure());}} /><Button onClick={()=>navigate('/cockpit')}>返回驾驶舱审阅规划候选</Button></div>}
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
