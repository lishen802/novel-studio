import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { FakeStudioClient } from "../../lib/fake-studio-client";
import type {
  ApplicationBootstrapView,
  ProviderView,
} from "../../lib/studio-client";
import { ApplicationSettings } from "./ApplicationSettings";

const baseBootstrap: ApplicationBootstrapView = {
  preferences: {
    theme: "system",
    defaultCreativeProfileId: "",
    updatedAt: "2026-08-12T10:00:00Z",
  },
  recentProjects: [],
  providers: [],
  profiles: [],
};

describe("ApplicationSettings", () => {
  it("clears API key field after save and never renders the secret", async () => {
    const client = new FakeStudioClient();
    const user = userEvent.setup();
    const onSavedProvider = vi.fn();
    render(
      <ApplicationSettings
        bootstrap={baseBootstrap}
        client={client}
        onSavedPreferences={vi.fn()}
        onSavedProvider={onSavedProvider}
        onPublishedProfile={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    await user.type(screen.getByLabelText("名称"), "OpenAI 主线");
    await user.type(screen.getByLabelText("API Key"), "sk-test-123456");
    await user.click(screen.getByRole("button", { name: "保存模型连接" }));

    expect(client.saveApplicationProviderCalls).toEqual([
      {
        name: "OpenAI 主线",
        kind: "openai",
        baseUrl: "",
        apiKey: "sk-test-123456",
      },
    ]);
    // The secret must never be rendered back to the user.
    expect(screen.queryByText("sk-test-123456")).not.toBeInTheDocument();
    expect(screen.getByLabelText("API Key")).toHaveValue("");
    expect(screen.getByLabelText("名称")).toHaveValue("");
    expect(onSavedProvider).toHaveBeenCalledOnce();
    const saved: ProviderView = onSavedProvider.mock.calls[0][0];
    expect(saved).toMatchObject({
      name: "OpenAI 主线",
      hasCredential: true,
    });
  });

  it("lists saved providers with credential status badges", () => {
    const client = new FakeStudioClient();
    const bootstrap: ApplicationBootstrapView = {
      ...baseBootstrap,
      providers: [
        {
          id: "provider-1",
          name: "OpenAI",
          kind: "openai",
          baseUrl: "https://api.openai.com/v1",
          hasCredential: true,
          createdAt: "2026-08-12T10:00:00Z",
          updatedAt: "2026-08-12T10:00:00Z",
        },
        {
          id: "provider-2",
          name: "Anthropic",
          kind: "anthropic",
          baseUrl: "",
          hasCredential: false,
          createdAt: "2026-08-12T10:00:00Z",
          updatedAt: "2026-08-12T10:00:00Z",
        },
      ],
    };
    render(
      <ApplicationSettings
        bootstrap={bootstrap}
        client={client}
        onSavedPreferences={vi.fn()}
        onSavedProvider={vi.fn()}
        onPublishedProfile={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText("已保存凭据")).toBeInTheDocument();
    expect(screen.getByText("需要凭据")).toBeInTheDocument();
  });

  it("announces profile version publication and future-project scope", async () => {
    const client = new FakeStudioClient();
    const user = userEvent.setup();
    const onPublishedProfile = vi.fn();
    render(
      <ApplicationSettings
        bootstrap={baseBootstrap}
        client={client}
        onSavedPreferences={vi.fn()}
        onSavedProvider={vi.fn()}
        onPublishedProfile={onPublishedProfile}
        onClose={vi.fn()}
      />,
    );

    await user.type(screen.getByLabelText("名称"), "默认方案");
    await user.click(screen.getByRole("button", { name: "发布新版本" }));

    const announcement = await screen.findByRole("status");
    expect(announcement.textContent).toMatch(/已发布方案/);
    expect(announcement.textContent).toMatch(/只影响以后新建的作品/);
    expect(client.publishCreativeProfileCalls).toHaveLength(1);
    expect(onPublishedProfile).toHaveBeenCalledWith(2);
  });

  it("saves theme preference and updates the active selector", async () => {
    const client = new FakeStudioClient();
    const user = userEvent.setup();
    const onSavedPreferences = vi.fn();
    render(
      <ApplicationSettings
        bootstrap={baseBootstrap}
        client={client}
        onSavedPreferences={onSavedPreferences}
        onSavedProvider={vi.fn()}
        onPublishedProfile={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    await user.click(screen.getByRole("button", { name: "深色" }));

    expect(client.saveApplicationPreferencesCalls).toEqual([
      {
        theme: "dark",
        defaultCreativeProfileId: "",
      },
    ]);
    expect(onSavedPreferences).toHaveBeenCalledWith({
      theme: "dark",
      defaultCreativeProfileId: "",
    });
    expect(screen.getByRole("button", { name: "深色" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });

  it("announces future-project scope when setting default creative profile", async () => {
    const client = new FakeStudioClient();
    const user = userEvent.setup();
    const bootstrap: ApplicationBootstrapView = {
      ...baseBootstrap,
      profiles: [
        {
          profileId: "profile-1",
          version: 1,
          name: "默认方案",
          roles: [],
          maxRunTokens: 30000,
          maxContextChars: 120000,
          createdAt: "2026-08-12T10:00:00Z",
        },
      ],
    };
    render(
      <ApplicationSettings
        bootstrap={bootstrap}
        client={client}
        onSavedPreferences={vi.fn()}
        onSavedProvider={vi.fn()}
        onPublishedProfile={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    await user.selectOptions(
      screen.getByLabelText("设为默认方案"),
      "profile-1",
    );
    await user.click(screen.getByRole("button", { name: "保存默认方案" }));

    const notice = await screen.findByText(
      /已设为默认创作方案。仅影响以后新建的作品，不会修改已有项目。/,
    );
    expect(notice).toBeInTheDocument();
    expect(client.saveApplicationPreferencesCalls).toEqual([
      {
        theme: "system",
        defaultCreativeProfileId: "profile-1",
      },
    ]);
  });
});
