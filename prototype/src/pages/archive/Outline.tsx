import { useState } from 'react';
import { Save, Check, BookOpen, User, Eye, Palette, Swords } from 'lucide-react';
import { currentProject } from '../../mock/data';
import { Card } from '../../components/ui/Card';
import { Textarea } from '../../components/ui/Input';
import { Button } from '../../components/ui/Button';
import { StatusBadge } from '../../components/ui/StatusBadge';

const progress = Math.min(
  100,
  Math.round((currentProject.wordCount / currentProject.targetWordCount) * 100),
);

const metaItems = [
  { icon: BookOpen, label: '类型', value: '悬疑' },
  { icon: Eye, label: '视角', value: '第三人称限知' },
  { icon: Palette, label: '基调', value: '克制、潮湿、悬疑' },
  { icon: Swords, label: '核心冲突', value: '真相与遗忘的对抗' },
];

export default function Outline() {
  const [pitch, setPitch] = useState(
    '一个归国的青年画师，一座沉默的江心灯塔，十七封从未寄出的信。当浓雾升起时，他必须在黑暗中找到那盏唯一可信的光。',
  );
  const [saved, setSaved] = useState(false);

  const handleSave = () => {
    setSaved(true);
    setTimeout(() => setSaved(false), 2000);
  };

  return (
    <div className="space-y-6 animate-fade-up">
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <Card className="lg:col-span-2 space-y-4">
          <div className="flex items-start justify-between gap-4">
            <div>
              <h2 className="text-xl font-display text-text">{currentProject.title}</h2>
              <p className="text-sm text-text-sub mt-1">{currentProject.subtitle}</p>
            </div>
            <StatusBadge variant="info">{currentProject.phase}</StatusBadge>
          </div>

          <p className="text-sm text-text-sub leading-relaxed">{currentProject.synopsis}</p>

          <div className="flex items-center gap-6 text-sm">
            <div className="flex items-center gap-2 text-text-sub">
              <User className="w-4 h-4" />
              <span>{currentProject.author}</span>
            </div>
            <div className="text-text-sub">
              目标字数 <span className="text-text font-medium">{currentProject.targetWordCount.toLocaleString()}</span>
            </div>
            <div className="text-text-sub">
              当前字数 <span className="text-text font-medium">{currentProject.wordCount.toLocaleString()}</span>
            </div>
          </div>

          <div className="space-y-2">
            <div className="flex justify-between text-xs text-text-sub">
              <span>完成进度</span>
              <span>{progress}%</span>
            </div>
            <div className="h-2 bg-surface-hover rounded-full overflow-hidden">
              <div
                className="h-full bg-primary rounded-full transition-all"
                style={{ width: `${progress}%` }}
              />
            </div>
          </div>
        </Card>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-1 gap-4">
          {metaItems.map((item, idx) => {
            const Icon = item.icon;
            return (
              <Card key={item.label} className="flex items-center gap-4 stagger-1" style={{ animationDelay: `${idx * 60}ms` }}>
                <div className="w-9 h-9 rounded-md bg-primary-subtle flex items-center justify-center shrink-0">
                  <Icon className="w-4 h-4 text-primary" />
                </div>
                <div>
                  <p className="text-xs text-text-muted">{item.label}</p>
                  <p className="text-sm text-text font-medium">{item.value}</p>
                </div>
              </Card>
            );
          })}
        </div>
      </div>

      <Card className="space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="text-base font-medium text-text">一句话推介</h3>
            <p className="text-xs text-text-sub mt-0.5">用一段话概括作品卖点，便于随时回看创作核心。</p>
          </div>
          <Button
            variant={saved ? 'default' : 'primary'}
            size="sm"
            onClick={handleSave}
            className={saved ? 'text-success border-success/30' : ''}
          >
            {saved ? <Check className="w-4 h-4" /> : <Save className="w-4 h-4" />}
            {saved ? '已保存' : '保存'}
          </Button>
        </div>
        <Textarea
          value={pitch}
          onChange={(e) => setPitch(e.target.value)}
          rows={4}
          placeholder="输入作品的一句话推介..."
        />
      </Card>
    </div>
  );
}
