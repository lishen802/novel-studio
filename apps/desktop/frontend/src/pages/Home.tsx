import { useState } from 'react';
import { useEffect } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import {
  Sparkles,
  Upload,
  FolderOpen,
  BookOpen,
  Clock,
  ChevronRight,
  RotateCcw,
} from 'lucide-react';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { Modal } from '../components/ui/Modal';
import { useApp } from '../store/useApp';
import { createOriginalProject, projectRouteForStatus } from '../store/appFlows';
import { recentProjects as storedRecentProjects } from '../mock/data';

interface RecentProject {
  id: string;
  title: string;
  subtitle: string;
  genre: string;
  lastLocation: string;
  lastOpened: string;
  pendingReviews?: number;
  recoverableTask?: string;
  color: string;
}

const demoProjects: RecentProject[] = [
  {
    id: 'proj-01',
    title: '雾中灯塔',
    subtitle: '民国悬疑长篇',
    genre: '悬疑 · 民国 · 江南',
    lastLocation: '正文编辑',
    lastOpened: '10 分钟前',
    pendingReviews: 2,
    recoverableTask: 'AI 为第三章生成了 2 条候选，等待审阅',
    color: '#3a4a5c',
  },
  {
    id: 'proj-02',
    title: '东海提灯人',
    subtitle: '奇幻冒险长篇',
    genre: '奇幻 · 冒险',
    lastLocation: '作者审阅',
    lastOpened: '昨天 16:43',
    color: '#2d4a5c',
  },
  {
    id: 'proj-03',
    title: '夏末书简',
    subtitle: '都市情感长篇',
    genre: '都市 · 情感',
    lastLocation: '导入分析中',
    lastOpened: '8 月 8 日',
    color: '#5c4a3a',
  },
];

export default function Home() {
  const navigate = useNavigate();
  const location = useLocation();
  const { dispatch } = useApp();
  const [createOpen, setCreateOpen] = useState(false);
  const [openProjectOpen, setOpenProjectOpen] = useState(false);
  const [title, setTitle] = useState('');
  const [genre, setGenre] = useState('');
  const [author, setAuthor] = useState('');
  const recentProjects = demoProjects;
  const latest = recentProjects[0];

  useEffect(() => {
    const mode = new URLSearchParams(location.search);
    if (mode.get('new') === '1') setCreateOpen(true);
    if (mode.get('open') === '1') setOpenProjectOpen(true);
  }, [location.search]);

  const openRecent = (id: string) => {
    const project = storedRecentProjects.find((item) => item.id === id);
    if (!project) return;
    dispatch({ type: 'SET_PROJECT', payload: project });
    navigate(projectRouteForStatus(project.status));
  };

  const createProject = () => {
    if (!title.trim()) return;
    const project = createOriginalProject({ title, genre, author });
    dispatch({ type: 'SET_PROJECT', payload: project });
    setCreateOpen(false);
    navigate('/cockpit');
  };

  return (
    <div className="h-full flex animate-fade-up -m-6">
      {/* Left panel */}
      <aside className="w-[300px] shrink-0 border-r border-border bg-surface flex flex-col shadow-[1px_0_0_0_rgba(0,0,0,0.03)]">
        <div className="p-4 space-y-3">
          {/* Primary entry */}
          <button
            onClick={() => setCreateOpen(true)}
            className="w-full group flex items-center justify-between p-4 rounded-xl bg-primary text-white shadow-md hover:bg-primary-hover transition-colors text-left"
          >
            <div>
              <p className="text-[15px] font-medium mb-0.5">从一个想法开始</p>
              <p className="text-xs text-white/70">创建全新作品</p>
            </div>
            <div className="w-9 h-9 rounded-full bg-white/10 flex items-center justify-center">
              <Sparkles className="w-5 h-5 text-white/90" />
            </div>
          </button>

          {/* Secondary entries */}
          <button
            onClick={() => navigate('/import')}
            className="w-full group flex items-center justify-between p-3.5 rounded-xl bg-surface-elevated border border-border hover:border-border-strong hover:bg-surface-hover transition-colors text-left"
          >
            <div>
              <p className="text-sm font-medium text-text">导入小说并续写</p>
              <p className="text-xs text-text-muted mt-0.5">导入后立即阅读与续写</p>
            </div>
            <Upload className="w-5 h-5 text-text-muted group-hover:text-text transition-colors" />
          </button>

          <button
            onClick={() => setOpenProjectOpen(true)}
            className="w-full group flex items-center justify-between p-3.5 rounded-xl bg-surface-elevated border border-border hover:border-border-strong hover:bg-surface-hover transition-colors text-left"
          >
            <div>
              <p className="text-sm font-medium text-text">打开项目</p>
              <p className="text-xs text-text-muted mt-0.5">浏览本地作品文件夹</p>
            </div>
            <FolderOpen className="w-5 h-5 text-text-muted group-hover:text-text transition-colors" />
          </button>
        </div>

        <div className="flex-1 overflow-auto px-4 pb-4">
          <h3 className="text-xs font-medium text-text-sub uppercase tracking-wider mb-3 px-1">
            最近项目
          </h3>
          {recentProjects.length === 0 ? (
            <p className="text-sm text-text-muted px-1">暂无最近项目</p>
          ) : (
            <div className="space-y-2">
              {recentProjects.map((p) => (
                <button
                  key={p.id}
                  onClick={() => openRecent(p.id)}
                  className="w-full text-left p-3 rounded-xl border border-border bg-surface-elevated hover:border-border-strong hover:bg-surface-hover transition-colors group"
                >
                  <div className="flex items-start justify-between gap-2">
                    <div className="flex items-start gap-3 min-w-0">
                      <div
                        className="w-9 h-12 rounded-md shadow-sm shrink-0"
                        style={{ backgroundColor: p.color }}
                      />
                      <div className="min-w-0">
                        <p className="text-sm font-medium text-text truncate">{p.title}</p>
                        <p className="text-xs text-text-sub truncate">{p.subtitle}</p>
                        <div className="flex items-center gap-1 text-xs text-text-muted mt-1.5">
                          <Clock className="w-3 h-3" />
                          <span>{p.lastOpened}</span>
                        </div>
                      </div>
                    </div>
                    {p.pendingReviews ? (
                      <span className="shrink-0 inline-flex items-center px-2 py-0.5 rounded-full bg-warning/10 text-warning text-[11px] font-medium border border-warning/15">
                        待审阅 {p.pendingReviews} 章
                      </span>
                    ) : (
                      <span className="shrink-0 text-[11px] text-text-muted">{p.lastLocation}</span>
                    )}
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>

      </aside>

      {/* Right main area */}
      <main className="flex-1 bg-atmosphere flex flex-col">
        <div className="flex-1 flex items-center justify-center p-8 md:p-12">
          {recentProjects.length === 0 || !latest ? (
            <div className="max-w-md text-center animate-fade-up">
              <div className="w-16 h-16 rounded-2xl border-2 border-border flex items-center justify-center mx-auto mb-6 text-text-muted">
                <BookOpen className="w-8 h-8" />
              </div>
              <h1 className="text-3xl font-display font-semibold text-text mb-4 tracking-wide">
                开始一本新作品
              </h1>
              <p className="text-base text-text-sub leading-relaxed mb-8">
                从一句想法开始构建原创故事，或导入已有手稿继续写作。
              </p>
              <div className="inline-flex items-center gap-4 text-xs text-text-sub mb-8">
                <span className="inline-flex items-center gap-1.5">
                  <span className="w-1.5 h-1.5 rounded-full bg-success" />
                  本地优先
                </span>
                <span className="inline-flex items-center gap-1.5">
                  <span className="w-1.5 h-1.5 rounded-full bg-success" />
                  自动保存
                </span>
                <span className="inline-flex items-center gap-1.5">
                  <span className="w-1.5 h-1.5 rounded-full bg-success" />
                  AI 辅助创作
                </span>
              </div>
              <div className="inline-flex items-center gap-2 text-xs text-text-muted bg-surface border border-border rounded-full px-4 py-2 shadow-sm">
                <RotateCcw className="w-3.5 h-3.5" />
                作品默认保存在本地，未经确认不会覆盖正文。
              </div>
            </div>
          ) : (
            <div className="w-full max-w-2xl animate-fade-up">
              <div className="bg-surface border border-border rounded-2xl shadow-sm p-8 relative overflow-hidden">
                <div className="absolute top-0 right-0 w-48 h-48 bg-primary-subtle rounded-full blur-3xl opacity-60 -translate-y-1/2 translate-x-1/2" />

                <div className="relative">
                  <p className="text-xs font-medium text-text-sub uppercase tracking-wider mb-4">最近继续</p>
                  <div className="flex items-start gap-5 mb-6">
                    <div
                      className="w-20 h-28 rounded-lg shadow-md shrink-0"
                      style={{ backgroundColor: latest.color }}
                    />
                    <div className="flex-1">
                      <h1 className="text-3xl font-display font-semibold text-text mb-2">
                        《{latest.title}》
                      </h1>
                      <p className="text-sm text-text-sub mb-4">{latest.genre}</p>

                      <div className="flex flex-wrap gap-3 mb-4">
                        <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md bg-surface-hover border border-border text-xs text-text-sub">
                          <BookOpen className="w-3.5 h-3.5" />
                          上次在：{latest.lastLocation}
                        </span>
                        <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md bg-surface-hover border border-border text-xs text-text-sub">
                          <Clock className="w-3.5 h-3.5" />
                          {latest.lastOpened}
                        </span>
                      </div>

                      {latest.pendingReviews ? (
                        <div className="flex items-start gap-2 p-3 bg-warning/10 border border-warning/20 rounded-lg mb-4">
                          <span className="text-warning text-sm font-medium">
                            待审阅 {latest.pendingReviews} 章
                          </span>
                          {latest.recoverableTask && (
                            <p className="text-xs text-text-sub mt-1">{latest.recoverableTask}</p>
                          )}
                        </div>
                      ) : null}
                    </div>
                  </div>

                  <div className="flex items-center gap-3">
                    <Button
                      variant="primary"
                      size="lg"
                      onClick={() => openRecent(latest.id)}
                      className="gap-1"
                    >
                      继续《{latest.title}》
                      <ChevronRight className="w-4 h-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="lg"
                      onClick={() => openRecent(latest.id)}
                    >
                      去审阅
                    </Button>
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>

        <div className="px-8 py-4 flex justify-center">
          <div className="inline-flex items-center gap-2 text-xs text-text-muted">
            <RotateCcw className="w-3.5 h-3.5" />
            作品默认保存在本地，未经确认不会覆盖正文。
          </div>
        </div>
      </main>

      <Modal open={createOpen} onClose={() => setCreateOpen(false)} title="创建原创作品">
        <div className="space-y-4">
          <p className="text-sm text-text-sub leading-relaxed">
            先建立本地作品空间，再在驾驶舱里用自然语言讨论创意。未配置模型时，你仍可创建作品与手工写作。
          </p>
          <div>
            <label className="block text-xs text-text-sub mb-1.5">书名</label>
            <Input value={title} onChange={(event) => setTitle(event.target.value)} autoFocus placeholder="例如：射雕英雄传" />
          </div>
          <div>
            <label className="block text-xs text-text-sub mb-1.5">题材（可选）</label>
            <Input value={genre} onChange={(event) => setGenre(event.target.value)} placeholder="例如：武侠" />
          </div>
          <div>
            <label className="block text-xs text-text-sub mb-1.5">作者署名（可选）</label>
            <Input value={author} onChange={(event) => setAuthor(event.target.value)} placeholder="可稍后在项目设置中修改" />
          </div>
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="ghost" onClick={() => setCreateOpen(false)}>取消</Button>
            <Button variant="primary" onClick={createProject} disabled={!title.trim()}>创建并开始构思</Button>
          </div>
        </div>
      </Modal>

      <Modal open={openProjectOpen} onClose={() => setOpenProjectOpen(false)} title="打开项目">
        <div className="space-y-3">
          <p className="text-sm text-text-sub">生产版会在此唤起系统文件选择器；原型中可用已有本地项目演示打开流程。</p>
          {storedRecentProjects.map((project) => (
            <button
              key={project.id}
              onClick={() => {
                setOpenProjectOpen(false);
                openRecent(project.id);
              }}
              className="w-full text-left p-3 rounded-lg border border-border hover:border-border-strong hover:bg-surface-hover"
            >
              <p className="text-sm font-medium text-text">《{project.title}》</p>
              <p className="text-xs text-text-sub mt-1">{project.subtitle} · {project.updatedAt}</p>
            </button>
          ))}
        </div>
      </Modal>
    </div>
  );
}
