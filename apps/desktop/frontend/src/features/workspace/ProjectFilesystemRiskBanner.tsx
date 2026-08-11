import type { ProjectFilesystemHealth } from "../../lib/studio-client";

type ProjectFilesystemRiskBannerProps = {
  health: ProjectFilesystemHealth;
  onCreateRecoveryCopy?: () => void;
};

export function ProjectFilesystemRiskBanner({
  health,
  onCreateRecoveryCopy,
}: ProjectFilesystemRiskBannerProps) {
  if (health.level === "normal") return null;
  const degraded = health.level === "degraded";
  const canCreateRecoveryCopy =
    degraded &&
    health.actions.includes("create_recovery_copy") &&
    onCreateRecoveryCopy;

  return (
    <aside
      className={`project-filesystem-risk project-filesystem-risk--${health.level}`}
      role="status"
    >
      <div>
        <strong>{degraded ? "项目写入遇到风险" : "项目文件系统存在风险"}</strong>
        <span>
          {degraded
            ? "未保存文本仍保留在当前编辑器中。请重试或创建恢复副本。"
            : "此位置可能干预项目写入；请保持同步客户端空闲并定期备份。"}
        </span>
      </div>
      {canCreateRecoveryCopy && (
        <button type="button" onClick={onCreateRecoveryCopy}>
          创建恢复副本
        </button>
      )}
    </aside>
  );
}
