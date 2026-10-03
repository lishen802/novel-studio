import { useEffect, useMemo, useState } from 'react';
import { Plus, Sparkles } from 'lucide-react';
import { studio, type StoryStructureView } from '../../api/studio';
import { PlanningRevisions } from '../../components/PlanningRevisions';
import { useApp } from '../../store/useApp';
import { Button } from '../../components/ui/Button';
import { Card } from '../../components/ui/Card';
import { EmptyState } from '../../components/ui/EmptyState';
import { Input, Textarea } from '../../components/ui/Input';
import { Modal } from '../../components/ui/Modal';

type NodeView = Record<string, unknown>;
const nextLevel: Record<string, string> = { book: 'volume', volume: 'arc', arc: 'chapter', chapter: 'scene' };
const levelLabel: Record<string, string> = { book: '全书', volume: '卷', arc: '故事弧', chapter: '章节', scene: '场景' };

function planningLines(node: NodeView): Array<[string, string]> {
  const planning = node.planning;
  if (!planning || typeof planning !== 'object') return [];
  const value = planning as Record<string, unknown>;
  const lines: Array<[string, string]> = [];
  ([
    ['读者期待', String(value.reader_expectation || '')],
    ['阶段回报', String(value.stage_payoff || '')],
    ['不可逆变化', String(value.irreversible_change || '')],
    ['终局贡献', String(value.ending_contribution || '')],
  ] as Array<[string, string]>).forEach(([label, text]) => {
    if (text.trim()) lines.push([label, text]);
  });
  const commitments = Array.isArray(value.commitments) ? value.commitments : [];
  commitments.forEach((item) => {
    if (!item || typeof item !== 'object') return;
    const row = item as Record<string, unknown>;
    const statement = String(row.statement || '').trim();
    if (!statement) return;
    const range = String(row.fulfillment_range || '').trim();
    lines.push(['待兑现承诺', range ? `${statement}（${range}）` : statement]);
  });
  const questions = Array.isArray(value.retained_questions) ? value.retained_questions : [];
  questions.forEach((item) => {
    if (!item || typeof item !== 'object') return;
    const row = item as Record<string, unknown>;
    const statement = String(row.statement || '').trim();
    if (!statement) return;
    const reason = String(row.reason || '').trim();
    lines.push(['保留悬念', reason ? `${statement}（${reason}）` : statement]);
  });
  return lines;
}

/** 读取并创建真实大纲节点，层级校验由后端统一完成。 */
export default function Structure() {
 const {state,registerBeforeClose}=useApp();
 const [formalTree,setFormalTree]=useState(true);
  const [structure, setStructure] = useState<StoryStructureView | null>(null);
  const [parentId, setParentId] = useState('');
  const [title, setTitle] = useState('');
  const [summary, setSummary] = useState('');
  const [open, setOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const nodes = structure?.outlineNodes || [];
  const byParent = useMemo(() => nodes.reduce((groups, node) => {
    const key = String(node.parentId || '');
    groups.set(key, [...(groups.get(key) || []), node]);
    return groups;
  }, new Map<string, NodeView[]>()), [nodes]);

  const refresh = () => studio.storyStructure().then(setStructure).catch((reason) => setError(String(reason)));
  useEffect(() => { void refresh(); }, []);

  const create = async () => {
    const parent = nodes.find((node) => String(node.id) === parentId);
    const level = parent ? nextLevel[String(parent.level)] : 'book';
    if (!level || !title.trim()) return;
    try {
      await studio.createOutlineNode({ parentId, level, title: title.trim(), summary: summary.trim(), goal: '', conflict: '', outcome: '', chapterId: '' });
      setOpen(false); setTitle(''); setSummary(''); await refresh();
    } catch (reason) { setError(reason instanceof Error ? reason.message : '创建大纲节点失败'); }
  };

  const renderNodes = (id = '', depth = 0): React.ReactNode => (byParent.get(id) || []).map((node) => (
    <div key={String(node.id)} className="space-y-2" style={{ marginLeft: depth * 20 }}>
      <Card className="py-3">
        <div className="flex justify-between gap-3">
          <div>
            <h3 className="text-sm font-medium text-text">{String(node.title)}</h3>
            <p className="text-xs text-text-sub mt-1">{String(node.summary || '暂无说明')}</p>
            {planningLines(node).map(([label, text]) => (
              <p key={`${String(node.id)}-${label}-${text}`} className="text-xs text-text-sub mt-1"><span className="text-text-muted">{label} · </span>{text}</p>
            ))}
          </div>
          <span className="text-xs text-text-muted">{levelLabel[String(node.level)] || String(node.level)}</span>
        </div>
      </Card>
      {renderNodes(String(node.id), depth + 1)}
    </div>
  ));

  return (
    <div className="space-y-5 animate-fade-up">
      <div className="flex justify-between"><div><h2 className="text-lg font-display text-text">故事架构</h2><p className="text-sm text-text-sub">数据已连接项目结构档案。</p></div><Button variant="primary" disabled={formalTree || state.backendProject?.accessMode === "read_only"} onClick={() => setOpen(true)}><Plus className="w-4 h-4" />添加节点</Button></div>
      <PlanningRevisions api={studio} kind="outline_node" registerBeforeLeave={registerBeforeClose} readOnly={state.backendProject?.accessMode === "read_only"} onFormalTree={setFormalTree} />
      <p className="text-xs text-text-muted">下方展示原始结构；按章节生效的最新目标请查看修订记录。正式规划的节点增删和重排尚未就绪。</p>
      {error && <p className="text-sm text-danger">{error}</p>}
      {nodes.length ? <div className="space-y-3">{renderNodes()}</div> : <EmptyState icon={Sparkles} title="还没有故事架构" description="创建全书节点后，再逐层添加卷、故事弧与章节。" />}
      <Modal open={open} onClose={() => setOpen(false)} title="添加大纲节点">
        <div className="space-y-4">
          <div><label className="block text-xs text-text-sub mb-1.5">父节点</label><select value={parentId} onChange={(e) => setParentId(e.target.value)} className="h-9 px-3 w-full bg-surface border border-border rounded-md text-sm"><option value="">无（创建全书节点）</option>{nodes.filter((node) => nextLevel[String(node.level)]).map((node) => <option key={String(node.id)} value={String(node.id)}>{String(node.title)} → {nextLevel[String(node.level)]}</option>)}</select></div>
          <Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="节点标题" />
          <Textarea value={summary} onChange={(e) => setSummary(e.target.value)} placeholder="节点概要" rows={3} />
          <div className="flex justify-end gap-2"><Button onClick={() => setOpen(false)}>取消</Button><Button variant="primary" onClick={() => void create()} disabled={!title.trim()}>创建</Button></div>
        </div>
      </Modal>
    </div>
  );
}
