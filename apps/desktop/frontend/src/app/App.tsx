import { BookOpenText } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { useStore } from "zustand";
import { Workspace } from "../features/workspace/Workspace";
import { ProjectLeaseConflictCard } from "../features/workspace/ProjectLeaseConflictCard";
import { createWorkspaceStore } from "../features/workspace/workspace-store";
import { HomeScreen } from "../features/home/HomeScreen";
import { ApplicationSettings } from "../features/settings/ApplicationSettings";
import { createAppShellStore } from "./app-shell-store";
import type {
  ApplicationPreferencesInput,
  ProjectView,
  ProviderView,
  StudioClient,
} from "../lib/studio-client";
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
  const appShell = useMemo(() => createAppShellStore(client), [client]);
  const workspaceStore = useMemo(() => createWorkspaceStore(client), [client]);
  const appState = useStore(appShell);
  const workspaceState = useStore(workspaceStore);

  const [title, setTitle] = useState("");
  const [genre, setGenre] = useState("");
  const [summary, setSummary] = useState("");
  const flushActiveChapter = useRef<() => Promise<void>>(async () => undefined);
  const pendingChapterFlushes = useRef(new Set<Promise<void>>());

  useEffect(() => {
    void appShell.getState().load();
  }, [appShell]);

  // Bridge workspace-store → app-shell-store: when a project becomes ready,
  // transition the app shell into project mode so HomeScreen unmounts.
  useEffect(() => {
    if (workspaceState.status.kind === "ready") {
      appShell.getState().enterProject(workspaceState.status.project);
    }
  }, [workspaceState.status, appShell]);

  const onFlushAvailable = useCallback(
    (flush: (() => Promise<void>) | null) => {
      if (flush) {
        flushActiveChapter.current = flush;
        return;
      }

      const pending = flushActiveChapter.current();
      pendingChapterFlushes.current.add(pending);
      void pending.then(
        () => pendingChapterFlushes.current.delete(pending),
        () => pendingChapterFlushes.current.delete(pending),
      );
      flushActiveChapter.current = async () => undefined;
    },
    [],
  );

  const flushPendingChapterChanges = useCallback(async () => {
    await flushActiveChapter.current();
    while (pendingChapterFlushes.current.size > 0) {
      await Promise.all([...pendingChapterFlushes.current]);
    }
  }, []);

  useEffect(
    () =>
      client.onBeforeClose(async () => {
        await flushPendingChapterChanges();
        await client.completeClose();
      }),
    [client, flushPendingChapterChanges],
  );

  useEffect(() => {
    if (initialProject) void workspaceStore.getState().loadProject(initialProject);
  }, [initialProject, workspaceStore]);

  // Subscribe to menu commands. The close_project command flushes any pending
  // chapter changes, then asks the shell to call CloseProject on the backend
  // and reload the bootstrap so recent projects reflect the latest state.
  useEffect(() => {
    return appShell.getState().subscribeMenuCommands(client, {
      onBeforeProjectClose: flushPendingChapterChanges,
    });
  }, [appShell, client, flushPendingChapterChanges]);

  const handleBeginCreate = useCallback(() => {
    void workspaceStore.getState().beginCreate();
  }, [workspaceStore]);

  const handleOpenProject = useCallback(() => {
    void workspaceStore.getState().openProject();
  }, [workspaceStore]);

  const handleImportManuscript = useCallback(() => {
    // Import is a project-level flow: open the workspace shell in welcome mode
    // so ImportWorkbench can take over once a project is open. For now we
    // delegate to the open project flow; a dedicated import entry will be
    // wired up in a follow-up task.
    void workspaceStore.getState().openProject();
  }, [workspaceStore]);

  const handleOpenRecent = useCallback(
    async (projectId: string) => {
      const result = await client.openRecentProject(projectId);
      if (result.outcome === "lease_conflict") {
        // Surface the conflict via the workspace store's lease_conflict path.
        workspaceStore.getState().cancelProjectOpen();
        return;
      }
      await workspaceStore.getState().loadProject(result.project);
    },
    [client, workspaceStore],
  );

  const handleSavedPreferences = useCallback(
    (input: ApplicationPreferencesInput) => {
      const current = appShell.getState().bootstrap;
      if (!current) return;
      appShell.setState({
        bootstrap: {
          ...current,
          preferences: {
            theme: input.theme,
            defaultCreativeProfileId: input.defaultCreativeProfileId,
          },
        },
      });
    },
    [appShell],
  );

  const handleSavedProvider = useCallback(
    (provider: ProviderView) => {
      const current = appShell.getState().bootstrap;
      if (!current) return;
      appShell.setState({
        bootstrap: {
          ...current,
          providers: [
            ...current.providers.filter((item) => item.id !== provider.id),
            provider,
          ],
        },
      });
    },
    [appShell],
  );

  const handlePublishedProfile = useCallback(
    (_version: number) => {
      // Re-read bootstrap from the backend to reflect the new profile version.
      void appShell.getState().reloadBootstrap();
    },
    [appShell],
  );

  const submitProject = async (event: FormEvent) => {
    event.preventDefault();
    if (!title.trim()) return;
    await workspaceState.createProject({ title, genre, summary });
  };

  // 项目模式或从项目模式打开的设置：保持 Workspace 挂载，设置作为覆盖层
  const isProjectMounted =
    appState.status.kind === "project" ||
    (appState.status.kind === "settings" && Boolean(appState.status.project));
  const settingsBootstrap =
    appState.status.kind === "settings" ? appState.status.bootstrap : null;
  const isSettingsOverlayOpen = appState.status.kind === "settings";

  if (isProjectMounted) {
    if (workspaceState.status.kind === "ready") {
      return (
        <>
          <Workspace
            workspace={workspaceState.status}
            error={workspaceState.workspaceError}
            client={client}
            currentPage={workspaceState.currentPage}
            workbench={workspaceState.workbench}
            onCreateChapter={workspaceState.createChapter}
            onSelectChapter={workspaceState.selectChapter}
            onChapterUpdated={workspaceState.updateChapter}
            onChaptersImported={workspaceState.replaceChapters}
            onWorkspaceRestored={workspaceState.restoreWorkspace}
            onBeforeVersionOperation={flushPendingChapterChanges}
            onDismissError={workspaceState.clearWorkspaceError}
            onFlushAvailable={onFlushAvailable}
            onCreateRecoveryCopy={workspaceState.createRecoveryCopy}
            onRefreshFilesystemHealth={workspaceState.refreshFilesystemHealth}
            onSetCurrentPage={workspaceState.setCurrentPage}
          />
          {isSettingsOverlayOpen && settingsBootstrap && (
            <div className="dialog-backdrop">
              <ApplicationSettings
                bootstrap={settingsBootstrap}
                client={client}
                onSavedPreferences={handleSavedPreferences}
                onSavedProvider={handleSavedProvider}
                onPublishedProfile={handlePublishedProfile}
                onClose={() => appState.closeSettings()}
              />
            </div>
          )}
        </>
      );
    }
    // Project mode but workspace is mid-transition: fall back to a loading shell.
    return (
      <main className="app-shell">
        <div className="app-shell__glow app-shell__glow--left" />
        <div className="app-shell__glow app-shell__glow--right" />
        <section className="welcome-card" aria-live="polite">
          <div className="product-mark" aria-hidden="true">
            <BookOpenText size={30} strokeWidth={1.65} />
          </div>
          <p className="eyebrow">长篇小说智能创作工作台</p>
          <h1>织境 Story Studio</h1>
          <p className="welcome-card__description">正在加载作品工作台…</p>
        </section>
      </main>
    );
  }

  const error = appState.status.kind === "error" ? appState.status.message : null;
  const recovery = appState.status.kind === "error" ? appState.status.recovery : null;
  const bootstrap =
    appState.status.kind === "home" || appState.status.kind === "settings"
      ? appState.status.bootstrap
      : appState.bootstrap;
  const isSettingsOpen = appState.status.kind === "settings";
  const workspaceOpening = workspaceState.status.kind === "opening";
  const leaseConflict =
    workspaceState.status.kind === "lease_conflict"
      ? workspaceState.status.conflict
      : null;
  const leaseConflictBusy =
    workspaceState.status.kind === "lease_conflict"
      ? workspaceState.status.busy ?? false
      : false;
  const pendingParentDirectory = workspaceState.pendingParentDirectory;
  const workspaceErrorMessage =
    workspaceState.status.kind === "error"
      ? workspaceState.status.message
      : null;
  const workspaceError =
    workspaceState.status.kind === "error" && !pendingParentDirectory
      ? workspaceState.status.message
      : null;
  const publishedRecoveryPath =
    workspaceState.status.kind === "error"
      ? workspaceState.status.publishedRecoveryPath
      : null;

  // Loading state: bootstrap not yet fetched and no error to surface.
  if (appState.status.kind === "loading" && !bootstrap) {
    return (
      <main className="app-shell">
        <div className="app-shell__glow app-shell__glow--left" />
        <div className="app-shell__glow app-shell__glow--right" />
        <section className="welcome-card" aria-live="polite">
          <div className="product-mark" aria-hidden="true">
            <BookOpenText size={30} strokeWidth={1.65} />
          </div>
          <p className="eyebrow">长篇小说智能创作工作台</p>
          <h1>织境 Story Studio</h1>
          <p className="welcome-card__description">正在准备本地创作空间…</p>
        </section>
      </main>
    );
  }

  return (
    <main className="app-shell">
      <div className="app-shell__glow app-shell__glow--left" />
      <div className="app-shell__glow app-shell__glow--right" />

      {bootstrap ? (
        <HomeScreen
          bootstrap={bootstrap}
          busy={workspaceOpening || appState.busy}
          onBeginCreate={handleBeginCreate}
          onOpenProject={handleOpenProject}
          onImportManuscript={handleImportManuscript}
          onOpenRecent={(project) => void handleOpenRecent(project.projectId)}
        />
      ) : (
        <section className="welcome-card" aria-labelledby="product-title">
          <div className="product-mark" aria-hidden="true">
            <BookOpenText size={30} strokeWidth={1.65} />
          </div>
          <p className="eyebrow">长篇小说智能创作工作台</p>
          <h1 id="product-title">织境 Story Studio</h1>
          <p className="welcome-card__description">
            从故事工程开始，让智能体参与规划、写作与审阅，同时由作者掌握最终决定。
          </p>
          {workspaceError && (
            <p role="alert" className="entry-error">
              {workspaceError}
            </p>
          )}
          {publishedRecoveryPath && (
            <p className="dialog-path">恢复副本：{publishedRecoveryPath}</p>
          )}
        </section>
      )}

      {bootstrap && workspaceError && (
        <div className="dialog-backdrop">
          <section
            className="project-dialog"
            role="alertdialog"
            aria-label="项目操作错误"
          >
            <p className="eyebrow">项目操作错误</p>
            <h2>无法继续</h2>
            <p role="alert" className="entry-error">
              {workspaceError}
            </p>
            {publishedRecoveryPath && (
              <p className="dialog-path">恢复副本：{publishedRecoveryPath}</p>
            )}
            <div className="dialog-actions">
              <button
                type="button"
                className="secondary-action"
                onClick={workspaceState.clearWorkspaceError}
              >
                关闭
              </button>
            </div>
          </section>
        </div>
      )}

      {error && (
        <div className="dialog-backdrop">
          <section className="project-dialog" role="alertdialog" aria-label="应用错误">
            <p className="eyebrow">应用错误</p>
            <h2>无法继续</h2>
            {error && <p role="alert" className="entry-error">{error}</p>}
            <div className="dialog-actions">
              <button
                type="button"
                className="secondary-action"
                onClick={() => appState.dismissError()}
              >
                关闭
              </button>
              {recovery && (
                <button
                  type="button"
                  className="primary-action"
                  onClick={() => void recovery.action()}
                >
                  {recovery.label}
                </button>
              )}
            </div>
          </section>
        </div>
      )}

      {isSettingsOpen && bootstrap && (
        <div className="dialog-backdrop">
          <ApplicationSettings
            bootstrap={bootstrap}
            client={client}
            onSavedPreferences={handleSavedPreferences}
            onSavedProvider={handleSavedProvider}
            onPublishedProfile={handlePublishedProfile}
            onClose={() => appState.closeSettings()}
          />
        </div>
      )}

      {leaseConflict && (
        <div className="dialog-backdrop">
          <ProjectLeaseConflictCard
            conflict={leaseConflict}
            busy={workspaceOpening || leaseConflictBusy}
            onRetry={() => void workspaceState.retryProjectOpen()}
            onOpenReadOnly={() => void workspaceState.openProjectReadOnly()}
            onCreateRecoveryCopy={() => void workspaceState.createRecoveryCopy()}
            onCancel={workspaceState.cancelProjectOpen}
          />
        </div>
      )}

      {pendingParentDirectory && (
        <div className="dialog-backdrop">
          <form
            className="project-dialog"
            role="dialog"
            aria-label="创建小说项目"
            onSubmit={submitProject}
          >
            <p className="eyebrow">新作品</p>
            <h2>创建小说项目</h2>
            <p className="dialog-path">保存到：{pendingParentDirectory}</p>
            <label>
              书名
              <input
                autoFocus
                required
                value={title}
                onChange={(event) => setTitle(event.target.value)}
              />
            </label>
            <label>
              类型
              <input
                value={genre}
                onChange={(event) => setGenre(event.target.value)}
              />
            </label>
            <label>
              简介
              <textarea
                value={summary}
                onChange={(event) => setSummary(event.target.value)}
              />
            </label>
            {workspaceErrorMessage && (
              <p role="alert" className="entry-error">
                {workspaceErrorMessage}
              </p>
            )}
            <div className="dialog-actions">
              <button
                type="button"
                className="secondary-action"
                disabled={workspaceOpening}
                onClick={workspaceState.cancelCreate}
              >
                取消
              </button>
              <button
                type="submit"
                className="primary-action"
                disabled={workspaceOpening}
              >
                确认创建
              </button>
            </div>
          </form>
        </div>
      )}
    </main>
  );
}
