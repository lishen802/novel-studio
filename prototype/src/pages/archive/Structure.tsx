import { useState } from 'react';
import { clsx } from 'clsx';
import { ChevronDown, ChevronRight, Plus, Sparkles } from 'lucide-react';
import { PlanningRevisions } from '../../components/PlanningRevisions';
import { FormalOutlineAppend } from '../../components/FormalOutlineAppend';
import { FormalOutlineRetire } from '../../components/FormalOutlineRetire';
import { formalOutlineDemo } from '../../mock/formalOutline';
import { formalOutlineRetirementDemo } from '../../mock/formalOutlineRetirement';
import { planningRevisionDemo } from '../../mock/planningRevisions';
import { Card } from '../../components/ui/Card';
import { Button } from '../../components/ui/Button';
import { Modal } from '../../components/ui/Modal';
import { Input, Textarea } from '../../components/ui/Input';
import { EmptyState } from '../../components/ui/EmptyState';

interface Beat {
  id: string;
  title: string;
  description: string;
}

interface Arc {
  id: string;
  title: string;
  summary: string;
  beats: Beat[];
}

const initialArcs: Arc[] = [
  {
    id: 'arc-1',
    title: '第一幕 · 雾起',
    summary: '建立悬疑氛围，让主人公进入灯塔世界，埋下核心伏笔。',
    beats: [
      { id: 'b1', title: '登岛', description: '沈知秋抵达江心岛，发现灯塔昨夜未亮。' },
      { id: 'b2', title: '未寄出的信', description: '在石屋中发现十七封收件人为周砚白的信。' },
      { id: 'b3', title: '灯芯', description: '上塔检查，发现灯芯被更换，暗示另有其人。' },
    ],
  },
  {
    id: 'arc-2',
    title: '第二幕 · 深入',
    summary: '揭开各方势力，推动人物关系升级，线索开始交织。',
    beats: [
      { id: 'b4', title: '渔村旧闻', description: '阿沅讲述父亲失踪与灯塔的传说。' },
      { id: 'b5', title: '商会账本', description: '赵掌柜对修缮经费讳莫如深，矛盾浮现。' },
      { id: 'b6', title: '雾中人影', description: '沈知秋在塔中察觉有人暗中跟随。' },
    ],
  },
  {
    id: 'arc-3',
    title: '第三幕 · 真相',
    summary: '回收伏笔，揭示灯塔与人物命运的核心秘密。',
    beats: [
      { id: 'b7', title: '灯语解密', description: '灯塔的闪烁节奏传递了隐藏信息。' },
      { id: 'b8', title: '水位退去', description: '海图红圈指向的地点随退潮显露。' },
      { id: 'b9', title: '最后一封信', description: '周砚白的真实身份与动机大白。' },
    ],
  },
];

export default function Structure() {
  const [arcs, setArcs] = useState<Arc[]>(initialArcs);
  const [expanded, setExpanded] = useState<Set<string>>(new Set(initialArcs.map((a) => a.id)));
  const [modalOpen, setModalOpen] = useState(false);
  const [activeArcId, setActiveArcId] = useState<string | null>(null);
  const [newBeatTitle, setNewBeatTitle] = useState('');
  const [newBeatDesc, setNewBeatDesc] = useState('');

  const toggleArc = (id: string) => {
    const next = new Set(expanded);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    setExpanded(next);
  };

  const openAddBeat = (arcId: string) => {
    setActiveArcId(arcId);
    setNewBeatTitle('');
    setNewBeatDesc('');
    setModalOpen(true);
  };

  const handleAddBeat = () => {
    if (!activeArcId || !newBeatTitle.trim()) return;
    setArcs((prev) =>
      prev.map((arc) =>
        arc.id === activeArcId
          ? {
              ...arc,
              beats: [
                ...arc.beats,
                { id: `b-${Date.now()}`, title: newBeatTitle.trim(), description: newBeatDesc.trim() },
              ],
            }
          : arc,
      ),
    );
    setModalOpen(false);
  };

  return (
    <div className="space-y-6 animate-fade-up">
      <p className="text-xs text-text-muted">原型仅演示规划修订，正式保存尚未就绪。</p>
      <PlanningRevisions api={planningRevisionDemo} kind="outline_node" />
      <FormalOutlineAppend api={formalOutlineDemo} arcs={initialArcs.map(arc=>({id:arc.id,title:arc.title}))} readOnly={false} />
      <FormalOutlineRetire api={formalOutlineRetirementDemo} nodes={[{id:'demo-node',title:'近期章节目标',level:'chapter',introducedAt:0,retiredAt:0}]} readOnly={false} />
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-display text-text">故事架构</h2>
          <p className="text-sm text-text-sub mt-0.5">以幕为单位组织关键节拍，随时调整叙事节奏。</p>
        </div>
      </div>

      {arcs.length === 0 ? (
        <EmptyState
          icon={Sparkles}
          title="还没有故事幕"
          description="先创建第一幕，把核心事件按顺序排列起来。"
        />
      ) : (
        <div className="space-y-4">
          {arcs.map((arc, idx) => {
            const isExpanded = expanded.has(arc.id);
            return (
              <Card
                key={arc.id}
                padding="none"
                className="overflow-hidden"
                style={{ animationDelay: `${idx * 80}ms` }}
              >
                <button
                  onClick={() => toggleArc(arc.id)}
                  className="w-full flex items-center justify-between px-5 py-4 hover:bg-surface-hover transition-colors text-left"
                >
                  <div>
                    <h3 className="text-base font-medium text-text">{arc.title}</h3>
                    <p className="text-xs text-text-sub mt-0.5">{arc.summary}</p>
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="text-xs text-text-muted">{arc.beats.length} 节拍</span>
                    {isExpanded ? (
                      <ChevronDown className="w-4 h-4 text-text-sub" />
                    ) : (
                      <ChevronRight className="w-4 h-4 text-text-sub" />
                    )}
                  </div>
                </button>

                <div
                  className={clsx(
                    'grid transition-all',
                    isExpanded ? 'grid-rows-[1fr] opacity-100' : 'grid-rows-[0fr] opacity-0',
                  )}
                >
                  <div className="overflow-hidden">
                    <div className="px-5 pb-5 pt-0 space-y-3">
                      {arc.beats.length === 0 ? (
                        <p className="text-sm text-text-muted py-4">此幕暂无节拍。</p>
                      ) : (
                        arc.beats.map((beat, bIdx) => (
                          <div
                            key={beat.id}
                            className="flex gap-4 p-3 bg-surface rounded-md border border-border"
                          >
                            <div className="flex flex-col items-center gap-1 pt-1">
                              <span className="w-6 h-6 rounded-full bg-surface-elevated border border-border flex items-center justify-center text-xs text-text-sub">
                                {bIdx + 1}
                              </span>
                            </div>
                            <div>
                              <h4 className="text-sm font-medium text-text">{beat.title}</h4>
                              <p className="text-sm text-text-sub mt-0.5">{beat.description}</p>
                            </div>
                          </div>
                        ))
                      )}
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => openAddBeat(arc.id)}
                        className="mt-1"
                      >
                        <Plus className="w-4 h-4" />
                        添加节拍
                      </Button>
                    </div>
                  </div>
                </div>
              </Card>
            );
          })}
        </div>
      )}

      <Modal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        title="添加新节拍"
      >
        <div className="space-y-4">
          <div>
            <label className="block text-xs text-text-sub mb-1.5">节拍标题</label>
            <Input
              value={newBeatTitle}
              onChange={(e) => setNewBeatTitle(e.target.value)}
              placeholder="例如：密室发现"
            />
          </div>
          <div>
            <label className="block text-xs text-text-sub mb-1.5">描述</label>
            <Textarea
              value={newBeatDesc}
              onChange={(e) => setNewBeatDesc(e.target.value)}
              rows={3}
              placeholder="简述该节拍推动的情节或作用..."
            />
          </div>
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="ghost" size="sm" onClick={() => setModalOpen(false)}>
              取消
            </Button>
            <Button
              variant="primary"
              size="sm"
              onClick={handleAddBeat}
              disabled={!newBeatTitle.trim()}
            >
              添加
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
