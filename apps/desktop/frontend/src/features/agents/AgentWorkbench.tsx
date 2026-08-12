import { Bot } from "lucide-react";
import type {
  ChapterView,
  ProjectView,
  StudioClient,
} from "../../lib/studio-client";
import { AgentRunPanel } from "./AgentRunPanel";
import { ProjectCreativeProfile } from "./ProjectCreativeProfile";

type AgentWorkbenchProps = {
  client: StudioClient;
  chapters: ChapterView[];
  project?: ProjectView;
  onOpenChanges(): void;
};

// AgentWorkbench 是一个临时组合组件，把项目级创作方案面板和任务运行面板组合在一起。
// 连接、API Key 和全局 profile 管理在 ApplicationSettings 中，不在此处。
// 待所有调用者迁移到直接使用 ProjectCreativeProfile + AgentRunPanel 后可移除。
export function AgentWorkbench({
  client,
  chapters,
  project,
  onOpenChanges,
}: AgentWorkbenchProps) {
  const resolvedProject: ProjectView =
    project ?? {
      id: "",
      title: "",
      genre: "",
      summary: "",
      path: "",
      accessMode: "editable",
      filesystemHealth: { level: "normal", category: "", actions: [] },
    };

  return (
    <div className="agent-workbench">
      <header className="workbench-heading">
        <div>
          <p className="eyebrow">MULTI-AGENT AUTHORING</p>
          <h2>模型与智能体</h2>
          <p>规划、正文和连续性审阅各自留痕，最终结果只进入候选变更。</p>
        </div>
        <Bot size={30} aria-hidden="true" />
      </header>

      <ProjectCreativeProfile client={client} project={resolvedProject} />
      <AgentRunPanel
        client={client}
        chapters={chapters}
        onOpenChanges={onOpenChanges}
      />
    </div>
  );
}
