import { Settings, Trash2, Zap } from "lucide-react";
import { useState, type FormEvent } from "react";
import type {
  ApplicationBootstrapView,
  ApplicationPreferencesInput,
  ApplicationProviderInput,
  CreativeProfileRoleView,
  ProviderView,
  StudioClient,
} from "../../lib/studio-client";

type ApplicationSettingsProps = {
  bootstrap: ApplicationBootstrapView;
  client: StudioClient;
  onSavedPreferences(input: ApplicationPreferencesInput): void;
  onSavedProvider(provider: ProviderView): void;
  onPublishedProfile(version: number): void;
  onClose(): void;
};

type ProviderDraft = {
  name: string;
  kind: string;
  baseUrl: string;
  apiKey: string;
};

const DEFAULT_PROVIDER_DRAFT: ProviderDraft = {
  name: "",
  kind: "openai",
  baseUrl: "",
  apiKey: "",
};

const PROFILE_ROLES = ["planner", "writer", "continuity_reviewer"];

export function ApplicationSettings({
  bootstrap,
  client,
  onSavedPreferences,
  onSavedProvider,
  onPublishedProfile,
  onClose,
}: ApplicationSettingsProps) {
  const [providerDraft, setProviderDraft] =
    useState<ProviderDraft>(DEFAULT_PROVIDER_DRAFT);
  const [providerSaving, setProviderSaving] = useState(false);
  const [providerError, setProviderError] = useState<string | null>(null);
  const [providerTestError, setProviderTestError] = useState<
    Record<string, string>
  >({});
  const [profileDraft, setProfileDraft] = useState(() =>
    deriveProfileDraft(bootstrap),
  );
  const [profileSaving, setProfileSaving] = useState(false);
  const [profileError, setProfileError] = useState<string | null>(null);
  const [profileAnnouncement, setProfileAnnouncement] = useState<
    string | null
  >(null);
  const [theme, setTheme] = useState(bootstrap.preferences.theme);
  const [themeSaving, setThemeSaving] = useState(false);
  const [themeNotice, setThemeNotice] = useState<string | null>(null);
  const [defaultProfileId, setDefaultProfileId] = useState(
    bootstrap.preferences.defaultCreativeProfileId,
  );
  const [defaultProfileSaving, setDefaultProfileSaving] = useState(false);
  const [defaultProfileNotice, setDefaultProfileNotice] = useState<
    string | null
  >(null);

  const providers = bootstrap.providers;
  const profiles = bootstrap.profiles;

  const submitProvider = async (event: FormEvent) => {
    event.preventDefault();
    if (providerSaving) return;
    if (!providerDraft.name.trim()) {
      setProviderError("请填写名称");
      return;
    }
    setProviderError(null);
    setProviderSaving(true);
    const input: ApplicationProviderInput = {
      name: providerDraft.name.trim(),
      kind: providerDraft.kind,
      baseUrl: providerDraft.baseUrl.trim(),
      apiKey: providerDraft.apiKey,
    };
    try {
      const saved = await client.saveApplicationProvider(input);
      onSavedProvider(saved);
      setProviderDraft(DEFAULT_PROVIDER_DRAFT);
    } catch (error) {
      setProviderError(error instanceof Error ? error.message : "保存失败");
    } finally {
      setProviderSaving(false);
    }
  };

  const removeProvider = async (id: string) => {
    try {
      await client.deleteApplicationProvider(id);
    } catch (error) {
      setProviderError(
        error instanceof Error ? error.message : "删除失败",
      );
    }
  };

  const testProvider = async (id: string) => {
    setProviderTestError((previous) => ({ ...previous, [id]: "" }));
    try {
      await client.testApplicationProvider(id);
    } catch (error) {
      setProviderTestError((previous) => ({
        ...previous,
        [id]: error instanceof Error ? error.message : "测试失败",
      }));
    }
  };

  const submitProfile = async (event: FormEvent) => {
    event.preventDefault();
    if (profileSaving) return;
    setProfileError(null);
    setProfileAnnouncement(null);
    setProfileSaving(true);
    try {
      const next = await client.publishCreativeProfile({
        profileId: profileDraft.profileId,
        name: profileDraft.name,
        roles: profileDraft.roles,
        maxRunTokens: profileDraft.maxRunTokens,
        maxContextChars: profileDraft.maxContextChars,
      });
      onPublishedProfile(next.version);
      setProfileAnnouncement(
        `已发布方案「${next.name}」第 ${next.version} 版。`,
      );
      setProfileDraft((current) => ({ ...current, profileId: next.profileId }));
    } catch (error) {
      setProfileError(error instanceof Error ? error.message : "发布失败");
    } finally {
      setProfileSaving(false);
    }
  };

  const submitTheme = async (next: ApplicationPreferencesInput["theme"]) => {
    if (themeSaving) return;
    setTheme(next);
    setThemeSaving(true);
    setThemeNotice(null);
    try {
      await client.saveApplicationPreferences({
        theme: next,
        defaultCreativeProfileId: defaultProfileId,
      });
      onSavedPreferences({ theme: next, defaultCreativeProfileId: defaultProfileId });
    } catch (error) {
      setThemeNotice(
        error instanceof Error ? error.message : "保存外观偏好失败",
      );
    } finally {
      setThemeSaving(false);
    }
  };

  const submitDefaultProfile = async () => {
    if (defaultProfileSaving) return;
    setDefaultProfileNotice(null);
    setDefaultProfileSaving(true);
    try {
      await client.saveApplicationPreferences({
        theme,
        defaultCreativeProfileId: defaultProfileId,
      });
      onSavedPreferences({
        theme,
        defaultCreativeProfileId: defaultProfileId,
      });
      setDefaultProfileNotice(
        "已设为默认创作方案。仅影响以后新建的作品，不会修改已有项目。",
      );
    } catch (error) {
      setDefaultProfileNotice(
        error instanceof Error ? error.message : "保存默认方案失败",
      );
    } finally {
      setDefaultProfileSaving(false);
    }
  };

  return (
    <section
      className="application-settings"
      role="dialog"
      aria-label="应用设置"
    >
      <header className="application-settings__header">
        <div>
          <p className="eyebrow">应用设置</p>
          <h2>应用设置</h2>
        </div>
        <button
          type="button"
          className="secondary-action"
          onClick={onClose}
        >
          关闭设置
        </button>
      </header>

      <section className="application-settings__section">
        <h3>模型连接</h3>
        <p>新增模型服务并提供 API Key。保存后系统仅保存凭据指纹，不会再次显示密钥。</p>
        <form className="provider-form" onSubmit={submitProvider}>
          <label>
            名称
            <input
              value={providerDraft.name}
              onChange={(event) =>
                setProviderDraft((current) => ({
                  ...current,
                  name: event.target.value,
                }))
              }
              autoComplete="off"
            />
          </label>
          <label>
            类型
            <select
              value={providerDraft.kind}
              onChange={(event) =>
                setProviderDraft((current) => ({
                  ...current,
                  kind: event.target.value,
                }))
              }
            >
              <option value="openai">OpenAI</option>
              <option value="anthropic">Anthropic</option>
            </select>
          </label>
          <label>
            Base URL
            <input
              value={providerDraft.baseUrl}
              onChange={(event) =>
                setProviderDraft((current) => ({
                  ...current,
                  baseUrl: event.target.value,
                }))
              }
              autoComplete="off"
            />
          </label>
          <label>
            API Key
            <input
              type="password"
              value={providerDraft.apiKey}
              onChange={(event) =>
                setProviderDraft((current) => ({
                  ...current,
                  apiKey: event.target.value,
                }))
              }
              autoComplete="off"
            />
          </label>
          <button
            type="submit"
            className="primary-action provider-form__submit"
            disabled={providerSaving}
          >
            保存模型连接
          </button>
        </form>
        {providerError && (
          <p className="entry-error">{providerError}</p>
        )}
        {providers.length > 0 ? (
          <ul className="provider-list">
            {providers.map((provider) => (
              <li key={provider.id} className="provider-list__item">
                <div className="provider-list__item-info">
                  <span className="provider-list__item-name">
                    {provider.name}
                  </span>
                  <span className="provider-list__item-meta">
                    {provider.kind} · {provider.baseUrl || "默认地址"}
                  </span>
                </div>
                <div className="provider-list__item-actions">
                  <span
                    className={
                      provider.hasCredential
                        ? "provider-credential-status provider-credential-status--ok"
                        : "provider-credential-status provider-credential-status--missing"
                    }
                  >
                    {provider.hasCredential ? "已保存凭据" : "需要凭据"}
                  </span>
                  <button
                    type="button"
                    className="secondary-action"
                    onClick={() => void testProvider(provider.id)}
                  >
                    <Zap size={14} aria-hidden="true" />
                    测试
                  </button>
                  <button
                    type="button"
                    className="secondary-action"
                    onClick={() => void removeProvider(provider.id)}
                  >
                    <Trash2 size={14} aria-hidden="true" />
                    删除
                  </button>
                  {providerTestError[provider.id] && (
                    <span className="entry-error">
                      {providerTestError[provider.id]}
                    </span>
                  )}
                </div>
              </li>
            ))}
          </ul>
        ) : (
          <p className="application-settings__notice">
            尚未保存任何模型连接。
          </p>
        )}
      </section>

      <section className="application-settings__section">
        <h3>默认创作方案</h3>
        <p>
          发布方案会创建一个新版本，只影响以后新建的作品；已打开的项目仍使用当前版本。
        </p>
        <form className="profile-form" onSubmit={submitProfile}>
          <label>
            方案
            <select
              value={profileDraft.profileId}
              onChange={(event) => {
                const next = profiles.find(
                  (profile) => profile.profileId === event.target.value,
                );
                if (next) {
                  setProfileDraft(deriveProfileDraftFromProfile(next));
                }
              }}
            >
              {profiles.length === 0 ? (
                <option value="">（暂无方案，将创建新方案）</option>
              ) : (
                profiles.map((profile) => (
                  <option
                    key={profile.profileId}
                    value={profile.profileId}
                  >
                    {profile.name}（v{profile.version}）
                  </option>
                ))
              )}
            </select>
          </label>
          <label>
            方案名称
            <input
              value={profileDraft.name}
              onChange={(event) =>
                setProfileDraft((current) => ({
                  ...current,
                  name: event.target.value,
                }))
              }
            />
          </label>
          <div className="provider-form">
            <label>
              最大运行 Tokens
              <input
                type="number"
                min={0}
                value={profileDraft.maxRunTokens}
                onChange={(event) =>
                  setProfileDraft((current) => ({
                    ...current,
                    maxRunTokens: Number(event.target.value) || 0,
                  }))
                }
              />
            </label>
            <label>
              最大上下文字符数
              <input
                type="number"
                min={0}
                value={profileDraft.maxContextChars}
                onChange={(event) =>
                  setProfileDraft((current) => ({
                    ...current,
                    maxContextChars: Number(event.target.value) || 0,
                  }))
                }
              />
            </label>
          </div>
          <div className="provider-form">
            {PROFILE_ROLES.map((role) => {
              const draftRole = profileDraft.roles.find(
                (entry) => entry.role === role,
              );
              const checked = Boolean(draftRole?.enabled);
              return (
                <label key={role}>
                  <input
                    type="checkbox"
                    checked={checked}
                    onChange={(event) =>
                      setProfileDraft((current) => updateRole(
                        current,
                        role,
                        event.target.checked,
                      ))
                    }
                  />
                  启用 {role}
                </label>
              );
            })}
          </div>
          <button
            type="submit"
            className="primary-action provider-form__submit"
            disabled={profileSaving}
          >
            发布新版本
          </button>
        </form>
        {profileAnnouncement && (
          <p className="profile-form__notice" role="status">
            {profileAnnouncement} 只影响以后新建的作品。
          </p>
        )}
        {profileError && (
          <p className="entry-error">{profileError}</p>
        )}
        <div className="provider-form">
          <label>
            设为默认方案
            <select
              value={defaultProfileId}
              onChange={(event) => setDefaultProfileId(event.target.value)}
            >
              <option value="">（无）</option>
              {profiles.map((profile) => (
                <option
                  key={profile.profileId}
                  value={profile.profileId}
                >
                  {profile.name}（v{profile.version}）
                </option>
              ))}
            </select>
          </label>
          <button
            type="button"
            className="secondary-action"
            disabled={defaultProfileSaving}
            onClick={() => void submitDefaultProfile()}
          >
            保存默认方案
          </button>
        </div>
        {defaultProfileNotice && (
          <p className="profile-form__notice" role="status">
            {defaultProfileNotice}
          </p>
        )}
      </section>

      <section className="application-settings__section">
        <h3>外观</h3>
        <div className="theme-selector">
          {(["system", "light", "dark"] as const).map((value) => (
            <button
              key={value}
              type="button"
              aria-pressed={theme === value}
              disabled={themeSaving}
              onClick={() => void submitTheme(value)}
            >
              {themeLabel(value)}
            </button>
          ))}
        </div>
        {themeNotice && (
          <p className="entry-error">{themeNotice}</p>
        )}
      </section>

      <section className="application-settings__section">
        <h3>高级</h3>
        <p>
          <Settings size={14} aria-hidden="true" />
          高级选项将随版本更新提供。
        </p>
      </section>
    </section>
  );
}

function themeLabel(theme: "system" | "light" | "dark"): string {
  if (theme === "light") return "浅色";
  if (theme === "dark") return "深色";
  return "跟随系统";
}

type ProfileDraft = {
  profileId: string;
  name: string;
  roles: CreativeProfileRoleView[];
  maxRunTokens: number;
  maxContextChars: number;
};

function deriveProfileDraft(
  bootstrap: ApplicationBootstrapView,
): ProfileDraft {
  const current = bootstrap.profiles[0];
  if (current) {
    return deriveProfileDraftFromProfile(current);
  }
  return {
    profileId: "",
    name: "默认方案",
    roles: PROFILE_ROLES.map((role) => ({
      role,
      connectionId: "",
      model: "",
      maxOutputTokens: 4096,
      temperature: 0.7,
      enabled: false,
    })),
    maxRunTokens: 30000,
    maxContextChars: 120000,
  };
}

function deriveProfileDraftFromProfile(
  profile: ApplicationBootstrapView["profiles"][number],
): ProfileDraft {
  return {
    profileId: profile.profileId,
    name: profile.name,
    roles: PROFILE_ROLES.map((role) => {
      const existing = profile.roles.find((entry) => entry.role === role);
      return (
        existing ?? {
          role,
          connectionId: "",
          model: "",
          maxOutputTokens: 4096,
          temperature: 0.7,
          enabled: false,
        }
      );
    }),
    maxRunTokens: profile.maxRunTokens,
    maxContextChars: profile.maxContextChars,
  };
}

function updateRole(
  draft: ProfileDraft,
  role: string,
  enabled: boolean,
): ProfileDraft {
  const roles = draft.roles.map((entry) =>
    entry.role === role ? { ...entry, enabled } : entry,
  );
  return { ...draft, roles };
}
