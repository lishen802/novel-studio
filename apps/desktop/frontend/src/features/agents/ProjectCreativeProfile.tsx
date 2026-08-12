import { Lock, ShieldCheck } from "lucide-react";
import { useEffect, useState } from "react";
import type {
  ApplicationBootstrapView,
  CreativeProfileView,
  ProjectView,
  StudioClient,
} from "../../lib/studio-client";

type ProjectCreativeProfileProps = {
  client: StudioClient;
  project: ProjectView;
};

const ROLE_LABELS: Record<string, string> = {
  planner: "章节规划",
  writer: "正文写作",
  continuity_reviewer: "连续性审阅",
};

function findPinnedProfile(
  bootstrap: ApplicationBootstrapView | null,
  selection: { profileId: string; version: number } | undefined,
): CreativeProfileView | null {
  if (!bootstrap || !selection) return null;
  return (
    bootstrap.profiles.find(
      (profile) =>
        profile.profileId === selection.profileId &&
        profile.version === selection.version,
    ) ?? null
  );
}

// ProjectCreativeProfile 只展示项目 pin 的方案版本与角色绑定，
// 不提供 API Key 或连接表单（这些在 ApplicationSettings 中），
// 也不修改已存在 run 的冻结 profile_id/model。
export function ProjectCreativeProfile({
  client,
  project,
}: ProjectCreativeProfileProps) {
  const [bootstrap, setBootstrap] = useState<ApplicationBootstrapView | null>(
    null,
  );
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const next = await client.applicationBootstrap();
        if (cancelled) return;
        setBootstrap(next);
        setError(null);
      } catch (cause) {
        if (cancelled) return;
        setError(cause instanceof Error ? cause.message : "加载方案失败");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [client]);

  const selection = project.profileSelection;
  const pinned = findPinnedProfile(bootstrap, selection);

  return (
    <section className="agent-panel project-creative-profile" aria-label="项目创作方案">
      <h3>
        <ShieldCheck size={18} />
        项目创作方案
      </h3>
      <p className="agent-hint">
        项目固定使用某一版本创作方案，不会随全局默认自动变化。
      </p>

      {error && (
        <p className="entry-error" role="alert">
          {error}
        </p>
      )}

      <div className="profile-pinned">
        {selection ? (
          <div className="profile-pinned__header">
            <span className="profile-pinned__name">
              {pinned?.name ?? "默认方案"}
            </span>
            <span className="profile-pinned__version">v{selection.version}</span>
            <span className="profile-pinned__stability" title="不会随全局默认自动变化">
              <Lock size={13} aria-hidden="true" />
              不会随全局默认自动变化
            </span>
          </div>
        ) : (
          <p className="profile-pinned__empty">尚未为该项目固定创作方案。</p>
        )}

        {pinned && (
          <ul className="profile-pinned__roles">
            {pinned.roles.map((role) => (
              <li key={role.role}>
                <span className="profile-pinned__role-label">
                  {ROLE_LABELS[role.role] ?? role.role}
                </span>
                <span className="profile-pinned__role-model">
                  {role.model || "未配置模型"}
                </span>
                <span className="profile-pinned__role-state">
                  {role.enabled ? "已启用" : "未启用"}
                </span>
              </li>
            ))}
            {pinned.roles.length === 0 && (
              <li className="profile-pinned__empty">该方案尚未配置任何角色。</li>
            )}
          </ul>
        )}

        {selection && !pinned && (
          <p className="profile-pinned__missing">
            已固定方案 v{selection.version}，但当前应用设置中找不到该版本。
            请在应用设置中确认方案未被打断或迁移。
          </p>
        )}
      </div>
    </section>
  );
}
