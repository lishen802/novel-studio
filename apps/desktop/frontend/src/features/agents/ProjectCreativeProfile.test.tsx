import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { FakeStudioClient } from "../../lib/fake-studio-client";
import type { ProjectView } from "../../lib/studio-client";
import { ProjectCreativeProfile } from "./ProjectCreativeProfile";

function project(overrides: Partial<ProjectView> = {}): ProjectView {
  const client = new FakeStudioClient();
  return { ...client.openProjectResult, ...overrides };
}

describe("ProjectCreativeProfile", () => {
  it("shows the pinned profile version when project has a profile selection", async () => {
    const client = new FakeStudioClient();
    client.applicationBootstrapResult = {
      preferences: { theme: "system", defaultCreativeProfileId: "" },
      recentProjects: [],
      providers: [],
      profiles: [
        {
          profileId: "profile-1",
          version: 3,
          name: "默认方案",
          roles: [
            {
              role: "planner",
              connectionId: "provider-1",
              model: "gpt-5",
              maxOutputTokens: 1800,
              temperature: 0.3,
              enabled: true,
            },
          ],
          maxRunTokens: 30000,
          maxContextChars: 120000,
        },
      ],
    };

    render(
      <ProjectCreativeProfile
        client={client}
        project={project({
          profileSelection: { profileId: "profile-1", version: 3 },
        })}
      />,
    );

    expect(await screen.findByText("默认方案")).toBeVisible();
    expect(screen.getByText("v3")).toBeVisible();
    expect(screen.getByText("gpt-5")).toBeVisible();
  });

  it("shows the stability notice so authors know pinned profiles do not follow the global default", async () => {
    const client = new FakeStudioClient();
    client.applicationBootstrapResult = {
      preferences: { theme: "system", defaultCreativeProfileId: "" },
      recentProjects: [],
      providers: [],
      profiles: [
        {
          profileId: "profile-1",
          version: 1,
          name: "默认方案",
          roles: [],
          maxRunTokens: 30000,
          maxContextChars: 120000,
        },
      ],
    };

    render(
      <ProjectCreativeProfile
        client={client}
        project={project({
          profileSelection: { profileId: "profile-1", version: 1 },
        })}
      />,
    );

    // 稳定性提示既出现在 hint 段落，也出现在 pin 徽章上
    const notices = await screen.findAllByText(/不会随全局默认自动变化/);
    expect(notices.length).toBeGreaterThanOrEqual(1);
    for (const node of notices) {
      expect(node).toBeVisible();
    }
  });

  it("lists each role with its model and enabled state", async () => {
    const client = new FakeStudioClient();
    client.applicationBootstrapResult = {
      preferences: { theme: "system", defaultCreativeProfileId: "" },
      recentProjects: [],
      providers: [],
      profiles: [
        {
          profileId: "profile-1",
          version: 2,
          name: "默认方案",
          roles: [
            {
              role: "planner",
              connectionId: "p1",
              model: "plan-model",
              maxOutputTokens: 1800,
              temperature: 0.3,
              enabled: true,
            },
            {
              role: "writer",
              connectionId: "p1",
              model: "writer-model",
              maxOutputTokens: 6000,
              temperature: 0.75,
              enabled: true,
            },
            {
              role: "continuity_reviewer",
              connectionId: "p2",
              model: "review-model",
              maxOutputTokens: 1800,
              temperature: 0.15,
              enabled: false,
            },
          ],
          maxRunTokens: 30000,
          maxContextChars: 120000,
        },
      ],
    };

    render(
      <ProjectCreativeProfile
        client={client}
        project={project({
          profileSelection: { profileId: "profile-1", version: 2 },
        })}
      />,
    );

    expect(await screen.findByText("plan-model")).toBeVisible();
    expect(screen.getByText("writer-model")).toBeVisible();
    expect(screen.getByText("review-model")).toBeVisible();
    expect(screen.getAllByText("已启用")).toHaveLength(2);
    expect(screen.getByText("未启用")).toBeVisible();
  });

  it("shows a helpful message when no profile is pinned", async () => {
    const client = new FakeStudioClient();

    render(<ProjectCreativeProfile client={client} project={project()} />);

    expect(
      await screen.findByText("尚未为该项目固定创作方案。"),
    ).toBeVisible();
  });

  it("shows a recovery hint when the pinned version cannot be found in the application settings", async () => {
    const client = new FakeStudioClient();
    client.applicationBootstrapResult = {
      preferences: { theme: "system", defaultCreativeProfileId: "" },
      recentProjects: [],
      providers: [],
      profiles: [],
    };

    render(
      <ProjectCreativeProfile
        client={client}
        project={project({
          profileSelection: { profileId: "profile-1", version: 9 },
        })}
      />,
    );

    expect(
      await screen.findByText(/当前应用设置中找不到该版本/),
    ).toBeVisible();
  });

  it("does not render API key or connection form fields", async () => {
    const client = new FakeStudioClient();
    client.applicationBootstrapResult = {
      preferences: { theme: "system", defaultCreativeProfileId: "" },
      recentProjects: [],
      providers: [],
      profiles: [
        {
          profileId: "profile-1",
          version: 1,
          name: "默认方案",
          roles: [],
          maxRunTokens: 30000,
          maxContextChars: 120000,
        },
      ],
    };

    render(
      <ProjectCreativeProfile
        client={client}
        project={project({
          profileSelection: { profileId: "profile-1", version: 1 },
        })}
      />,
    );

    await screen.findByText("默认方案");
    expect(screen.queryByLabelText("API Key")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("接口地址")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "安全保存连接" })).not.toBeInTheDocument();
  });
});
