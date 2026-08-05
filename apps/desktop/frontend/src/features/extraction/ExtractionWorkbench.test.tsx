import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { FakeStudioClient } from "../../lib/fake-studio-client";
import type {
  ChangeSetView,
  ExtractionRunView,
  ExtractionTaskView,
  ImportJobView,
  ProviderConnectionView,
} from "../../lib/studio-client";
import { ExtractionWorkbench } from "./ExtractionWorkbench";

function appliedImportJob(): ImportJobView {
  return {
    id: "import-applied-1",
    status: "applied",
    sourceName: "雾港夜话.md",
    sourceFormat: "markdown",
    sourceEncoding: "UTF-8",
    sourceHashDigest: "0123456789ab",
    sourceBytes: 2048,
    chapterCount: 2,
    selectedChapterCount: 2,
    extractionStatus: "none",
    extractionRunId: "",
    errorCode: "",
    errorMessage: "",
    createdAt: "2026-08-03T08:00:00Z",
    updatedAt: "2026-08-03T09:00:00Z",
    appliedAt: "2026-08-03T09:00:00Z",
    chapters: [
      {
        id: "candidate-1",
        volumeTitle: "",
        title: "第一章 潮声",
        wordCount: 9876,
        bodyPreview: "潮声越过防波堤。",
        position: 1,
        sourceStartLine: 1,
        sourceEndLine: 50,
        contentHashSummary: "abcdef012345",
        selected: true,
        createdChapterId: "chapter-1",
      },
      {
        id: "candidate-2",
        volumeTitle: "",
        title: "第二章 雪线",
        wordCount: 8765,
        bodyPreview: "雪落在旧界碑上。",
        position: 2,
        sourceStartLine: 51,
        sourceEndLine: 100,
        contentHashSummary: "123456abcdef",
        selected: true,
        createdChapterId: "chapter-2",
      },
    ],
  };
}

function providerConnection(): ProviderConnectionView {
  return {
    id: "conn-1",
    name: "OpenAI",
    kind: "openai",
    baseUrl: "https://api.openai.com/v1",
    hasCredential: true,
    updatedAt: "2026-08-01T00:00:00Z",
  };
}

function queuedTask(overrides: Partial<ExtractionTaskView> = {}): ExtractionTaskView {
  return {
    id: "extraction-1",
    importJobId: "import-applied-1",
    connectionId: "conn-1",
    model: "gpt-4o",
    prompt: "抽取人物与世界规则",
    status: "queued",
    errorCode: "",
    errorMessage: "",
    changeSetId: "",
    retryOf: "",
    tokenBudget: 30000,
    contextCharBudget: 120000,
    inputTokens: 0,
    outputTokens: 0,
    steps: [],
    createdAt: "2026-08-04T08:00:00Z",
    updatedAt: "2026-08-04T08:00:00Z",
    ...overrides,
  };
}

function changeSetWithCreateCharacter(): ChangeSetView {
  return {
    id: "changeset-1",
    title: "雾港夜话 设定抽取",
    summary: "已合并 2 章设定",
    createdBy: "extraction",
    status: "pending",
    operations: [
      {
        id: "op-1",
        kind: "create_character",
        targetId: "",
        baseRevision: 1,
        baseFingerprint: "",
        beforeValue: "{}",
        afterValue: JSON.stringify({
          name: "苏沉",
          aliases: ["沉哥"],
          role: "protagonist",
          summary: "守灯人，负责雾港灯塔。",
        }),
        reason: "第一章主角出现并展现核心动机。",
        status: "pending",
        evidence: [
          {
            chapterId: "chapter-1",
            chapterTitle: "第一章 潮声",
            startLine: 3,
            endLine: 12,
            excerpt: "苏沉推开灯塔木门，海风裹着盐粒扑面而来。",
          },
        ],
      },
      {
        id: "op-2",
        kind: "create_world_rule",
        targetId: "",
        baseRevision: 1,
        baseFingerprint: "",
        beforeValue: "{}",
        afterValue: JSON.stringify({
          name: "雾港灯规",
          description: "雾港灯塔必须在子夜前点亮。",
          scopeType: "project",
          scopeId: "",
          status: "active",
        }),
        reason: "第二章揭示的世界规则。",
        status: "pending",
        evidence: [
          {
            chapterId: "chapter-2",
            chapterTitle: "第二章 雪线",
            startLine: 60,
            endLine: 75,
            excerpt: "若子夜前灯未亮，雾兽便会爬上岸。",
          },
        ],
      },
    ],
    createdAt: "2026-08-04T09:00:00Z",
    updatedAt: "2026-08-04T09:00:00Z",
  };
}

function completedTask(): ExtractionTaskView {
  return queuedTask({
    id: "extraction-completed",
    status: "completed",
    changeSetId: "changeset-1",
    steps: [
      {
        id: "step-1",
        importChapterId: "candidate-1",
        createdChapterId: "chapter-1",
        chapterTitle: "第一章 潮声",
        position: 1,
        status: "completed",
        output: "",
        errorCode: "",
        errorMessage: "",
        providerRequestId: "req-1",
        inputTokens: 1200,
        outputTokens: 800,
      },
      {
        id: "step-2",
        importChapterId: "candidate-2",
        createdChapterId: "chapter-2",
        chapterTitle: "第二章 雪线",
        position: 2,
        status: "completed",
        output: "",
        errorCode: "",
        errorMessage: "",
        providerRequestId: "req-2",
        inputTokens: 1500,
        outputTokens: 900,
      },
    ],
  });
}

describe("ExtractionWorkbench", () => {
  it("renders the empty state when no tasks exist", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.importJobsResult = [appliedImportJob()];
    client.providerConnectionsResult = [providerConnection()];

    render(<ExtractionWorkbench client={client} onExtractionApplied={vi.fn()} />);

    expect(
      await screen.findByRole("heading", { name: "设定抽取" }),
    ).toBeInTheDocument();
    expect(
      await screen.findByText("暂无抽取任务"),
    ).toBeInTheDocument();
  });

  it("creates a new extraction run from an applied import job", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.importJobsResult = [appliedImportJob()];
    client.providerConnectionsResult = [providerConnection()];
    const startedTask = queuedTask();
    client.extractionStartResult = {
      task: startedTask,
      changeSet: null,
    };

    const user = userEvent.setup();
    render(<ExtractionWorkbench client={client} onExtractionApplied={vi.fn()} />);

    const modelInput = await screen.findByLabelText("模型名称");
    await user.type(modelInput, "gpt-4o");

    const createButton = screen.getByRole("button", {
      name: "创建抽取任务",
    });
    await user.click(createButton);

    expect(client.startExtractionRunCalls).toHaveLength(1);
    expect(client.startExtractionRunCalls[0].importJobId).toBe(
      "import-applied-1",
    );
    expect(client.startExtractionRunCalls[0].model).toBe("gpt-4o");
    expect(
      await screen.findByText(/抽取任务已创建/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "执行抽取" }),
    ).toBeInTheDocument();
  });

  it("requires a model name before starting a run", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.importJobsResult = [appliedImportJob()];
    client.providerConnectionsResult = [providerConnection()];

    const user = userEvent.setup();
    render(<ExtractionWorkbench client={client} onExtractionApplied={vi.fn()} />);

    await screen.findByRole("button", { name: "创建抽取任务" });
    const createButton = screen.getByRole("button", {
      name: "创建抽取任务",
    });
    await user.click(createButton);

    expect(client.startExtractionRunCalls).toHaveLength(0);
    expect(
      screen.getByText(/请配置模型连接与模型名称/),
    ).toBeInTheDocument();
  });

  it("lists existing tasks and reveals step progress", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.importJobsResult = [appliedImportJob()];
    client.providerConnectionsResult = [providerConnection()];
    const task = completedTask();
    client.extractionTasksResult = [task];
    client.extractionRunResult = {
      task,
      changeSet: changeSetWithCreateCharacter(),
    };

    render(<ExtractionWorkbench client={client} onExtractionApplied={vi.fn()} />);

    const taskButton = await screen.findByRole("button", {
      name: /雾港夜话\.md/,
    });
    await userEvent.click(taskButton);

    expect(
      await screen.findByText("第一章 潮声"),
    ).toBeInTheDocument();
    expect(screen.getByText("第二章 雪线")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", {
        name: "雾港夜话 设定抽取",
      }),
    ).toBeInTheDocument();
  });

  it("shows evidence for create_* candidates", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.importJobsResult = [appliedImportJob()];
    client.providerConnectionsResult = [providerConnection()];
    const task = completedTask();
    client.extractionTasksResult = [task];
    client.extractionRunResult = {
      task,
      changeSet: changeSetWithCreateCharacter(),
    };

    render(<ExtractionWorkbench client={client} onExtractionApplied={vi.fn()} />);

    await userEvent.click(
      await screen.findByRole("button", { name: /雾港夜话\.md/ }),
    );

    const evidenceSummaries = await screen.findAllByText(
      /原稿证据（1 条）/,
      { exact: false },
    );
    expect(evidenceSummaries).toHaveLength(2);
    await userEvent.click(evidenceSummaries[0]);

    expect(
      screen.getByText(/第一章 潮声（第 3–12 行）/),
    ).toBeInTheDocument();
    expect(
      screen.getByText("苏沉推开灯塔木门，海风裹着盐粒扑面而来。"),
    ).toBeInTheDocument();
  });

  it("applies the change set when the author confirms", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.importJobsResult = [appliedImportJob()];
    client.providerConnectionsResult = [providerConnection()];
    const task = completedTask();
    client.extractionTasksResult = [task];
    client.extractionRunResult = {
      task,
      changeSet: changeSetWithCreateCharacter(),
    };
    const applySpy = vi
      .spyOn(client, "applyChangeSet")
      .mockResolvedValue({ changeSet: changeSetWithCreateCharacter(), chapters: [] });
    const onApplied = vi.fn();

    const user = userEvent.setup();
    render(<ExtractionWorkbench client={client} onExtractionApplied={onApplied} />);

    await user.click(
      await screen.findByRole("button", { name: /雾港夜话\.md/ }),
    );
    await screen.findByRole("heading", {
      name: "雾港夜话 设定抽取",
    });

    const applyButton = screen.getByRole("button", {
      name: "应用到设定库",
    });
    await user.click(applyButton);

    await screen.findByText(/候选变更已应用到设定库/);
    expect(applySpy).toHaveBeenCalledWith("changeset-1");
    expect(onApplied).toHaveBeenCalled();
  });

  it("cancels a running extraction task", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.importJobsResult = [appliedImportJob()];
    client.providerConnectionsResult = [providerConnection()];
    const runningTask = queuedTask({
      id: "extraction-running",
      status: "running",
    });
    client.extractionTasksResult = [runningTask];
    client.extractionRunResult = { task: runningTask, changeSet: null };

    const user = userEvent.setup();
    render(<ExtractionWorkbench client={client} onExtractionApplied={vi.fn()} />);

    await user.click(
      await screen.findByRole("button", { name: /雾港夜话\.md/ }),
    );

    const cancelButton = await screen.findByRole("button", {
      name: "取消任务",
    });
    await user.click(cancelButton);

    expect(client.cancelExtractionRunCalls).toEqual(["extraction-running"]);
    await screen.findByText(/已取消抽取任务/);
  });

  it("retries a failed extraction task", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.importJobsResult = [appliedImportJob()];
    client.providerConnectionsResult = [providerConnection()];
    const failedTask = queuedTask({
      id: "extraction-failed",
      status: "failed",
      errorCode: "EXTRACTION_PROVIDER_TIMEOUT",
      errorMessage: "模型响应超时。",
    });
    client.extractionTasksResult = [failedTask];
    client.extractionRunResult = { task: failedTask, changeSet: null };
    const retriedTask = queuedTask({
      id: "extraction-failed",
      status: "queued",
      retryOf: "extraction-failed",
    });
    client.extractionRetryResult = {
      task: retriedTask,
      changeSet: null,
    };

    const user = userEvent.setup();
    render(<ExtractionWorkbench client={client} onExtractionApplied={vi.fn()} />);

    await user.click(
      await screen.findByRole("button", { name: /雾港夜话\.md/ }),
    );

    const retryButton = await screen.findByRole("button", {
      name: "重置重试",
    });
    await user.click(retryButton);

    expect(client.retryExtractionRunCalls).toEqual(["extraction-failed"]);
    await screen.findByText(/已重置抽取任务/);
  });

  it("renders the extraction entry from ImportWorkbench", async () => {
    // Sanity check that the ImportWorkbench toggle wires the extraction view.
    const { ImportWorkbench } = await import("../imports/ImportWorkbench");
    const client = FakeStudioClient.withReadyProject();
    client.importJobsResult = [appliedImportJob()];
    client.providerConnectionsResult = [providerConnection()];
    client.extractionTasksResult = [];
    client.extractionRunResult = null;

    const user = userEvent.setup();
    render(
      <ImportWorkbench client={client} onChaptersImported={vi.fn()} />,
    );

    await screen.findByRole("heading", { name: "导入分析" });
    const extractionToggle = screen.getByRole("button", {
      name: /设定抽取/,
    });
    await user.click(extractionToggle);

    // The ExtractionWorkbench renders a "启动新抽取" heading.
    expect(
      await screen.findByRole("heading", { name: "启动新抽取" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "返回导入任务" }),
    ).toBeInTheDocument();
  });
});
