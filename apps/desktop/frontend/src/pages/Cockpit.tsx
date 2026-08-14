import { Compass, FileText, Settings } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { Button } from '../components/ui/Button';
import { Card } from '../components/ui/Card';
import { useApp } from '../store/useApp';

/** 创意驾驶舱属于后续里程碑；当前只提供真实可用功能的入口。 */
export default function Cockpit() {
  const navigate = useNavigate();
  const { state } = useApp();

  return (
    <div className="h-full flex items-center justify-center animate-fade-up">
      <Card className="max-w-2xl text-center space-y-5 p-10">
        <div className="mx-auto w-12 h-12 rounded-xl bg-primary-subtle flex items-center justify-center">
          <Compass className="w-6 h-6 text-primary" />
        </div>
        <div>
          <h1 className="text-xl font-display text-text">{state.currentProject?.title || '创意驾驶舱'}</h1>
          <p className="mt-2 text-sm leading-6 text-text-sub">
            多智能体创意方向生成尚未进入当前后端里程碑，因此这里不再展示或提交模拟候选。
            你可以先使用已经接通的章节编辑、审阅和项目档案功能。
          </p>
        </div>
        <div className="flex justify-center gap-3">
          <Button variant="primary" onClick={() => navigate('/chapters')}>
            <FileText className="w-4 h-4" />进入正文编辑
          </Button>
          <Button onClick={() => navigate('/settings')}>
            <Settings className="w-4 h-4" />配置模型
          </Button>
        </div>
      </Card>
    </div>
  );
}
