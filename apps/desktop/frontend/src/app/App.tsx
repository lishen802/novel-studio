import { BookOpenText, FolderOpen, Plus } from "lucide-react";
import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useStore } from "zustand";
import { Workspace } from "../features/workspace/Workspace";
import { createWorkspaceStore } from "../features/workspace/workspace-store";
import type { ProjectView, StudioClient } from "../lib/studio-client";
import { wailsStudioClient } from "../lib/wails-studio-client";
import "./styles.css";

type AppProps = {
  client?: StudioClient;
  initialProject?: ProjectView;
};

export function App({
  client = wailsStudioClient,
  initialProject,
}: AppProps) {
  const store = useMemo(() => createWorkspaceStore(client), [client]);
  const state = useStore(store);
  const [title, setTitle] = useState("");
  const [genre, setGenre] = useState("");
  const [summary, setSummary] = useState("");

  useEffect(() => {
    if (initialProject) void store.getState().loadProject(initialProject);
  }, [initialProject, store]);

  const submitProject = async (event: FormEvent) => {
    event.preventDefault();
    if (!title.trim()) return;
    await state.createProject({ title, genre, summary });
  };

  if (state.status.kind === "ready") {
    return (
      <Workspace
        workspace={state.status}
        error={state.workspaceError}
        client={client}
        onCreateChapter={state.createChapter}
        onSelectChapter={state.selectChapter}
        onChapterUpdated={state.updateChapter}
        onDismissError={state.clearWorkspaceError}
      />
    );
  }

  const isOpening = state.status.kind === "opening";
  const error = state.status.kind === "error" ? state.status.message : null;

  return (
    <main className="app-shell">
      <div className="app-shell__glow app-shell__glow--left" />
      <div className="app-shell__glow app-shell__glow--right" />

      <section className="welcome-card" aria-labelledby="product-title">
        <div className="product-mark" aria-hidden="true">
          <BookOpenText size={30} strokeWidth={1.65} />
        </div>
        <p className="eyebrow">长篇小说智能创作工作台</p>
        <h1 id="product-title">织境 Story Studio</h1>
        <p className="welcome-card__description">
          从故事工程开始，让智能体参与规划、写作与审阅，同时由作者掌握最终决定。
        </p>
        {error && !state.pendingParentDirectory && <p role="alert" className="entry-error">{error}</p>}
        <div className="welcome-actions">
          <button
            type="button"
            className="primary-action"
            disabled={isOpening}
            onClick={() => void state.beginCreate()}
          >
            <Plus size={18} aria-hidden="true" />
            创建项目
          </button>
          <button
            type="button"
            className="secondary-action"
            disabled={isOpening}
            onClick={() => void state.openProject()}
          >
            <FolderOpen size={18} aria-hidden="true" />
            打开项目
          </button>
        </div>
        <p className="local-first-note">
          {isOpening ? "正在准备本地创作空间…" : "作品默认保存在本地，未经确认不会覆盖正文。"}
        </p>
      </section>

      {state.pendingParentDirectory && (
        <div className="dialog-backdrop">
          <form className="project-dialog" role="dialog" aria-label="创建小说项目" onSubmit={submitProject}>
            <p className="eyebrow">新作品</p>
            <h2>创建小说项目</h2>
            <p className="dialog-path">保存到：{state.pendingParentDirectory}</p>
            {error && <p role="alert" className="entry-error">{error}</p>}
            <label>
              书名
              <input autoFocus required value={title} onChange={(event) => setTitle(event.target.value)} />
            </label>
            <label>
              类型
              <input value={genre} onChange={(event) => setGenre(event.target.value)} />
            </label>
            <label>
              简介
              <textarea value={summary} onChange={(event) => setSummary(event.target.value)} />
            </label>
            <div className="dialog-actions">
              <button
                type="button"
                className="secondary-action"
                disabled={isOpening}
                onClick={state.cancelCreate}
              >
                取消
              </button>
              <button type="submit" className="primary-action" disabled={isOpening}>
                确认创建
              </button>
            </div>
          </form>
        </div>
      )}
    </main>
  );
}
