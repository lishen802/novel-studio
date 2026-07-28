import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it } from "vitest";
import { FakeStudioClient } from "../../lib/fake-studio-client";
import { WorldAssetsWorkbench } from "./WorldAssetsWorkbench";

function readyClient() {
  const client = FakeStudioClient.withReadyProject();
  client.chaptersResult = [client.createChapterResult];
  client.storyWorldResult = {
    ...client.storyWorldResult,
    characters: [
      {
        id: "character-1",
        name: "墨老",
        aliases: [],
        role: "supporting",
        summary: "",
        motivation: "",
        fear: "",
        secret: "",
        arc: "",
        status: "active",
      },
    ],
  };
  client.worldAssetsResult = {
    regions: [
      {
        id: "region-1",
        parentId: "",
        kind: "world",
        name: "镜海世界",
        direction: "center",
        summary: "群岛漂浮于雾海之上。",
        terrain: "雾海",
        climate: "潮湿",
      },
    ],
    regionAdjacencies: [],
    locations: [
      {
        id: "location-1",
        regionId: "region-1",
        kind: "city",
        name: "照夜城",
        summary: "",
        status: "active",
      },
    ],
    locationLinks: [],
    items: [
      {
        id: "item-1",
        name: "照骨镜",
        summary: "可显露真名。",
        status: "active",
        holderType: "character",
        holderId: "character-1",
      },
    ],
    itemTransfers: [],
    rules: [
      {
        id: "rule-1",
        name: "真名约束",
        description: "被知晓真名者会受到拘束。",
        scopeType: "project",
        scopeId: "",
        status: "active",
      },
    ],
    hooks: [
      {
        id: "hook-1",
        title: "镜中裂纹",
        setup: "镜面出现一道不会愈合的裂纹。",
        plantedChapterId: "chapter-1",
        dueChapterId: "chapter-1",
        resolvedChapterId: "",
        status: "planted",
        notes: "",
      },
    ],
    consistencyIssues: [
      {
        code: "OVERDUE_HOOK",
        severity: "warning",
        message: "伏笔“镜中裂纹”已到计划回收章节。",
        entityIds: ["hook-1"],
      },
    ],
  };
  return client;
}

describe("WorldAssetsWorkbench", () => {
  let client: FakeStudioClient;

  beforeEach(() => {
    client = readyClient();
  });

  it("shows structured geography and consistency warnings", async () => {
    render(
      <WorldAssetsWorkbench client={client} chapters={client.chaptersResult} />,
    );

    expect(
      await screen.findByRole("heading", { name: "世界设定" }),
    ).toBeInTheDocument();
    expect(screen.getAllByText("镜海世界").length).toBeGreaterThan(0);
    expect(screen.getByText("照夜城")).toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent(
      "伏笔“镜中裂纹”已到计划回收章节。",
    );
  });

  it("creates a child region from the geography view", async () => {
    const user = userEvent.setup();
    render(
      <WorldAssetsWorkbench client={client} chapters={client.chaptersResult} />,
    );

    await user.click(await screen.findByRole("button", { name: "新建区域" }));
    const dialog = screen.getByRole("dialog", { name: "新建区域" });
    await user.selectOptions(within(dialog).getByLabelText("上级区域"), "region-1");
    await user.selectOptions(within(dialog).getByLabelText("区域类型"), "territory");
    await user.type(within(dialog).getByLabelText("区域名称"), "北境");
    await user.selectOptions(within(dialog).getByLabelText("相对方位"), "north");
    await user.click(within(dialog).getByRole("button", { name: "保存区域" }));

    await waitFor(() =>
      expect(client.createWorldRegionCalls).toContainEqual({
        parentId: "region-1",
        kind: "territory",
        name: "北境",
        direction: "north",
        summary: "",
        terrain: "",
        climate: "",
      }),
    );
    expect((await screen.findAllByText("北境")).length).toBeGreaterThan(0);
  });

  it("shows the formal region hierarchy and saves an undirected adjacency", async () => {
    const user = userEvent.setup();
    client.worldAssetsResult = {
      ...client.worldAssetsResult,
      regions: [
        ...client.worldAssetsResult.regions,
        {
          id: "region-2",
          parentId: "region-1",
          kind: "territory",
          name: "北境",
          direction: "north",
          summary: "",
          terrain: "雪原",
          climate: "严寒",
        },
        {
          id: "region-3",
          parentId: "region-1",
          kind: "territory",
          name: "东海",
          direction: "east",
          summary: "",
          terrain: "海域",
          climate: "潮湿",
        },
      ],
    };
    render(
      <WorldAssetsWorkbench client={client} chapters={client.chaptersResult} />,
    );

    expect(await screen.findByRole("heading", { name: "区域层级" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "选择北境" }));
    await user.click(screen.getByRole("button", { name: "新增相邻区域" }));
    const dialog = screen.getByRole("dialog", { name: "新增区域相邻关系" });
    await user.selectOptions(within(dialog).getByLabelText("相邻区域"), "region-3");
    await user.type(within(dialog).getByLabelText("通行说明"), "沿海船路可直达");
    await user.click(within(dialog).getByRole("button", { name: "保存相邻关系" }));

    await waitFor(() =>
      expect(client.upsertRegionAdjacencyCalls).toContainEqual({
        firstRegionId: "region-2",
        secondRegionId: "region-3",
        travelNote: "沿海船路可直达",
      }),
    );
    expect(await screen.findByText("北境 ↔ 东海")).toBeInTheDocument();
  });

  it("moves an item and records the chapter context", async () => {
    const user = userEvent.setup();
    render(
      <WorldAssetsWorkbench client={client} chapters={client.chaptersResult} />,
    );

    await user.click(await screen.findByRole("button", { name: "物品" }));
    await user.click(screen.getByRole("button", { name: "流转照骨镜" }));
    const dialog = screen.getByRole("dialog", { name: "流转物品" });
    await user.selectOptions(within(dialog).getByLabelText("持有者类型"), "location");
    await user.selectOptions(within(dialog).getByLabelText("新持有者"), "location-1");
    await user.selectOptions(within(dialog).getByLabelText("关联章节"), "chapter-1");
    await user.type(within(dialog).getByLabelText("流转说明"), "寄存在城主府");
    await user.click(within(dialog).getByRole("button", { name: "确认流转" }));

    await waitFor(() =>
      expect(client.transferItemCalls).toContainEqual({
        itemId: "item-1",
        holderType: "location",
        holderId: "location-1",
        chapterId: "chapter-1",
        note: "寄存在城主府",
      }),
    );
    expect(await screen.findByText(/照夜城/)).toBeInTheDocument();
  });

  it("advances a foreshadowing hook and safely archives it", async () => {
    const user = userEvent.setup();
    render(
      <WorldAssetsWorkbench client={client} chapters={client.chaptersResult} />,
    );

    await user.click(await screen.findByRole("button", { name: "伏笔" }));
    await user.click(screen.getByRole("button", { name: "编辑镜中裂纹" }));
    const editDialog = screen.getByRole("dialog", { name: "编辑伏笔" });
    await user.selectOptions(within(editDialog).getByLabelText("状态"), "resolved");
    await user.selectOptions(
      within(editDialog).getByLabelText("实际回收章节"),
      "chapter-1",
    );
    await user.click(within(editDialog).getByRole("button", { name: "保存伏笔" }));

    await waitFor(() =>
      expect(client.updateForeshadowingHookCalls[0]?.input).toMatchObject({
        status: "resolved",
        resolvedChapterId: "chapter-1",
      }),
    );

    await user.click(screen.getByRole("button", { name: "删除镜中裂纹" }));
    const confirm = screen.getByRole("dialog", { name: "确认删除" });
    expect(confirm).toHaveTextContent("镜中裂纹");
    await user.click(within(confirm).getByRole("button", { name: "确认删除" }));

    await waitFor(() =>
      expect(client.archiveForeshadowingHookCalls).toEqual(["hook-1"]),
    );
    expect(screen.queryByText("镜中裂纹")).not.toBeInTheDocument();
  });
});
