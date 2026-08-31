import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { clsx } from 'clsx';
import {
  Cpu, Feather, FileText, Palette, SlidersHorizontal, Save, Check,
  AlertTriangle, TestTube, Trash2, Plus, ArrowLeft, Search,
} from 'lucide-react';
import { useApp } from '../store/useApp';
import type { AppSettings, CreativeProfile, CreativeProfileRole, CreativeRole, ModelConfig } from '../types';
import { getActiveModel } from '../types';
import { canvasPresets } from '../editor';
import type { MarginSize } from '../editor';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { StatusBadge } from '../components/ui/StatusBadge';
import { Modal } from '../components/ui/Modal';
import { studio } from '../api/studio';
import type { ProviderTestView } from '../api/studio';
import { getModelVendor, inferModelVendor, modelVendors } from '../modelCatalog';

type TabKey = 'model' | 'writing' | 'templates' | 'appearance' | 'advanced';

const tabs: { key: TabKey; label: string; icon: typeof Cpu }[] = [
  { key: 'model', label: '模型连接', icon: Cpu },
  { key: 'writing', label: '创作方案', icon: Feather },
  { key: 'templates', label: '作者模板', icon: FileText },
  { key: 'appearance', label: '外观', icon: Palette },
  { key: 'advanced', label: '高级', icon: SlidersHorizontal },
];

const tabDescriptions: Record<TabKey, string> = {
  model: '管理模型服务、接口地址与凭据。',
  writing: '为规划、写作、审阅等职责分配模型。',
  templates: '管理作者常用的创作模板。',
  appearance: '调整主题、正文画布与排版。',
  advanced: '设置自动保存、备份和界面语言。',
};

const creativeRoleLabels: Record<CreativeRole, string> = {
  planner: '规划',
  writer: '写作',
  continuity_reviewer: '审阅',
};

export default function Settings() {
  const navigate = useNavigate();
  const { state, dispatch, saveSettings } = useApp();
  const [activeTab, setActiveTab] = useState<TabKey>('model');
  const [search, setSearch] = useState('');
  const [saved, setSaved] = useState(false);
  const [saving, setSaving] = useState(false);
  const [settingError, setSettingError] = useState<string | null>(null);
  const [testingId, setTestingId] = useState<string | null>(null);
  const [testOkId, setTestOkId] = useState<string | null>(null);
  const [testResult, setTestResult] = useState<ProviderTestView | null>(null);
  const [testModels, setTestModels] = useState<Record<string, string>>({});
  const [deleteBusyId, setDeleteBusyId] = useState<string | null>(null);
  const [deleteTargetId, setDeleteTargetId] = useState<string | null>(null);

  const [models, setModels] = useState<ModelConfig[]>(state.settings.models);
  const [activeModelId, setActiveModelId] = useState(state.settings.activeModelId);
  const [apiKeys, setApiKeys] = useState<Record<string, string>>({});
  const activeModel = useMemo(() => getActiveModel({ ...state.settings, models, activeModelId }), [state.settings, models, activeModelId]);
  const activeVendor = activeModel ? inferModelVendor(activeModel.provider, activeModel.baseUrl) : getModelVendor('openai');
  const activeTestModel = activeModel
    ? testModels[activeModel.id] ?? activeVendor.models[0]?.id ?? ''
    : '';

  const [creativeProfiles, setCreativeProfiles] = useState<CreativeProfile[]>(state.settings.creativeProfiles);
  const [defaultCreativeProfileId, setDefaultCreativeProfileId] = useState(state.settings.defaultCreativeProfileId);
  const [selectedProfileId, setSelectedProfileId] = useState(state.settings.defaultCreativeProfileId);
  const selectedProfile = creativeProfiles.find((profile) => profile.id === selectedProfileId) || creativeProfiles[0];
  const activeTabMeta = tabs.find((tab) => tab.key === activeTab) || tabs[0];
  const visibleTabs = tabs.filter((tab) => `${tab.label}${tabDescriptions[tab.key]}`.includes(search.trim()));

  const [theme, setTheme] = useState(state.settings.theme);
  const [fontSize, setFontSize] = useState(state.settings.fontSize);
  const [lineHeight, setLineHeight] = useState(state.settings.lineHeight);
  const [canvasColorId, setCanvasColorId] = useState(state.settings.canvasColorId);
  const [marginSize, setMarginSize] = useState<MarginSize>(state.settings.marginSize);
  const [indent, setIndent] = useState(state.settings.indent);

  const [autoSave, setAutoSave] = useState(true);
  const [backupCount, setBackupCount] = useState(10);
  const [language, setLanguage] = useState('zh-CN');

  useEffect(() => {
    setModels(state.settings.models);
    setActiveModelId(state.settings.activeModelId);
    setCreativeProfiles(state.settings.creativeProfiles);
    setDefaultCreativeProfileId(state.settings.defaultCreativeProfileId);
    setSelectedProfileId(state.settings.defaultCreativeProfileId);
    setTheme(state.settings.theme);
    setFontSize(state.settings.fontSize);
    setLineHeight(state.settings.lineHeight);
    setCanvasColorId(state.settings.canvasColorId);
    setMarginSize(state.settings.marginSize);
    setIndent(state.settings.indent);
  }, [state.settings]);

  const updateModel = (id: string, patch: Partial<ModelConfig>) => {
    setModels((prev) => prev.map((m) => (m.id === id ? { ...m, ...patch } : m)));
  };

  const addModel = () => {
    const id = `m-${Date.now()}`;
    setModels((prev) => [
      ...prev,
      { id, name: `模型 ${prev.length + 1}`, provider: 'openai', baseUrl: 'https://api.openai.com/v1', apiKeyConfigured: false },
    ]);
    setActiveModelId(id);
  };

  const deleteTarget = models.find((model) => model.id === deleteTargetId);
  const deleteReferences = deleteTargetId
    ? creativeProfiles.filter((profile) => Object.values(profile.roles).some((role) => role.connectionId === deleteTargetId))
    : [];

  /** 使用应用内对话框确认删除，避免桌面 WebView 的原生 confirm 被窗口层级遮挡。 */
  const deleteModel = (id: string) => {
    const target = models.find((m) => m.id === id);
    if (!target) return;
    setSettingError(null);
    setDeleteTargetId(id);
  };

  /** 删除未被当前方案引用的连接；历史版本引用仍由后端权威校验。 */
  const confirmDeleteModel = async () => {
    const id = deleteTargetId;
    if (!id || deleteReferences.length > 0) return;
    setDeleteBusyId(id);
    setSettingError(null);
    try {
      if (!id.startsWith('m-')) {
        await studio.deleteApplicationProvider(id);
      }
      setModels((prev) => {
        const next = prev.filter((m) => m.id !== id);
        if (activeModelId === id) {
          setActiveModelId(next[0]?.id || '');
        }
        return next;
      });
      setApiKeys((prev) => {
        if (!(id in prev)) return prev;
        const next = { ...prev };
        delete next[id];
        return next;
      });
      setDeleteTargetId(null);
    } catch (error) {
      setSettingError(error instanceof Error ? error.message : '删除连接失败');
    } finally {
      setDeleteBusyId(null);
    }
  };

  const handleTestConnection = async (id: string) => {
    setTestingId(id);
    setTestOkId(null);
    setTestResult(null);
    setSettingError(null);
    try {
      if (id.startsWith('m-')) throw new Error('请先保存连接，再测试。');
      if (!activeTestModel.trim()) throw new Error('请先选择或填写测试模型。');
      const result = await studio.testApplicationProvider({ providerId: id, model: activeTestModel });
      setTestResult(result);
      setTestOkId(id);
      setTimeout(() => setTestOkId(null), 3000);
    } catch (error) {
      setSettingError(error instanceof Error ? error.message : '连接测试失败');
    } finally {
      setTestingId(null);
    }
  };

  /** 厂商选择只映射到现有 Provider 协议和接口地址，不引入新的后端枚举。 */
  const selectVendor = (vendorId: string) => {
    if (!activeModel) return;
    const vendor = getModelVendor(vendorId);
    updateModel(activeModel.id, { provider: vendor.protocol, baseUrl: vendor.defaultBaseUrl });
    setTestModels((previous) => ({ ...previous, [activeModel.id]: vendor.models[0]?.id || '' }));
  };

  const handleSave = async () => {
    setSaving(true);
    setSettingError(null);
    try {
      // 逐个保存连接，并用后端返回的真实 ID 替换临时 m- 前缀 ID，避免后续操作引用
      // 已失效的本地占位 ID。
      const idMapping = new Map<string, string>();
      for (const model of models) {
        const saved = await studio.saveApplicationProvider({
          id: model.id.startsWith('m-') ? '' : model.id,
          name: model.name,
          kind: model.provider,
          baseUrl: model.baseUrl || '',
          apiKey: apiKeys[model.id] || '',
        });
        idMapping.set(model.id, saved.id);
      }
      const mappedModels = models.map((model) => ({
        ...model,
        id: idMapping.get(model.id) || model.id,
        apiKeyConfigured: model.apiKeyConfigured || (apiKeys[model.id]?.trim().length ?? 0) > 0,
      }));
      // 角色绑定中可能引用临时 ID，必须先同步计算出完整快照再保存 Store，不能依赖
      // React 异步 setState，否则保存后仍会带着旧 ID 或被 Bootstrap 清空。
      const mappedProfiles = creativeProfiles.map((profile) => ({
        ...profile,
        roles: Object.fromEntries(
          (Object.keys(profile.roles) as CreativeRole[]).map((role) => {
            const binding = profile.roles[role];
            const mapped = idMapping.get(binding.connectionId);
            return [role, mapped ? { ...binding, connectionId: mapped } : binding];
          }),
        ) as Record<CreativeRole, CreativeProfileRole>,
      }));
      const mappedActiveModelId = idMapping.get(activeModelId) || activeModelId;
      setModels(mappedModels);
      setCreativeProfiles(mappedProfiles);
      setActiveModelId(mappedActiveModelId);
      setTestModels((previous) => Object.fromEntries(Object.entries(previous).map(([id, model]) => [idMapping.get(id) || id, model])));
      // API Key 已经写入凭据库，不再保留在 React 状态中，避免内存中残留明文密钥。
      setApiKeys({});
      const next: AppSettings = {
        models: mappedModels,
        activeModelId: mappedActiveModelId,
        creativeProfiles: mappedProfiles,
        defaultCreativeProfileId,
        writingPlan: selectedProfile?.name || state.settings.writingPlan,
        theme,
        fontSize,
        lineHeight,
        canvasColorId,
        marginSize,
        indent,
      };
      await saveSettings(next);
      setSaved(true);
      setTimeout(() => setSaved(false), 2000);
    } catch (error) {
      setSettingError(error instanceof Error ? error.message : '保存设置失败');
    } finally {
      setSaving(false);
    }
  };

  /** 更新方案中的单一角色的模型连接。 */
  const updateProfileRole = (role: CreativeRole, connectionId: string) => {
    if (!selectedProfile) return;
    setCreativeProfiles((profiles) => profiles.map((profile) => (
      profile.id === selectedProfile.id
        ? { ...profile, roles: { ...profile.roles, [role]: { ...profile.roles[role], connectionId } } }
        : profile
    )));
  };

  /** 更新方案中单一角色的模型名称；模型名称属于角色绑定而非连接。 */
  const updateProfileRoleModel = (role: CreativeRole, model: string) => {
    if (!selectedProfile) return;
    setCreativeProfiles((profiles) => profiles.map((profile) => (
      profile.id === selectedProfile.id
        ? { ...profile, roles: { ...profile.roles, [role]: { ...profile.roles[role], model } } }
        : profile
    )));
  };

  /** 更新方案中单一角色的生成参数（最大输出 Token、温度）。 */
  const updateProfileRoleParam = (
    role: CreativeRole,
    patch: Partial<Pick<CreativeProfileRole, 'maxOutputTokens' | 'temperature'>>,
  ) => {
    if (!selectedProfile) return;
    setCreativeProfiles((profiles) => profiles.map((profile) => (
      profile.id === selectedProfile.id
        ? { ...profile, roles: { ...profile.roles, [role]: { ...profile.roles[role], ...patch } } }
        : profile
    )));
  };

  /** 更新方案级运行预算。 */
  const updateProfileBudget = (patch: Partial<Pick<CreativeProfile, 'maxRunTokens' | 'maxContextChars'>>) => {
    if (!selectedProfile) return;
    setCreativeProfiles((profiles) => profiles.map((profile) => (
      profile.id === selectedProfile.id ? { ...profile, ...patch } : profile
    )));
  };

  /** 将当前编辑方案发布为下一版本，避免静默改变已经创建的作品。 */
  const publishNextProfileVersion = async () => {
    if (!selectedProfile) return;
    const roles = (Object.keys(creativeRoleLabels) as CreativeRole[]).map((role) => {
      const binding = selectedProfile.roles[role];
      return {
        role,
        connectionId: binding?.connectionId || '',
        model: binding?.model || '',
        maxOutputTokens: binding?.maxOutputTokens || 4096,
        temperature: binding?.temperature ?? (role === 'continuity_reviewer' ? 0.2 : 0.7),
        enabled: true,
      };
    });
    setSettingError(null);
    try {
      const published = await studio.publishCreativeProfile({
        profileId: selectedProfile.id,
        name: selectedProfile.name,
        roles,
        maxRunTokens: selectedProfile.maxRunTokens || 30000,
        maxContextChars: selectedProfile.maxContextChars || 120000,
      });
      const updatedProfiles = creativeProfiles.map((profile) => (
        profile.id === selectedProfile.id
          ? { ...profile, version: published.version, published: true, maxRunTokens: selectedProfile.maxRunTokens || 30000, maxContextChars: selectedProfile.maxContextChars || 120000 }
          : profile
      ));
      setCreativeProfiles(updatedProfiles);
      await saveSettings({
        ...state.settings,
        models,
        activeModelId,
        creativeProfiles: updatedProfiles,
        defaultCreativeProfileId,
        writingPlan: selectedProfile.name,
        theme,
        fontSize,
        lineHeight,
        canvasColorId,
        marginSize,
        indent,
      });
      // Harness 直接读取 Bootstrap；发布成功后同步替换对应最新版本，使其无需重启即可运行。
      if (state.bootstrap) {
        dispatch({
          type: 'SET_BOOTSTRAP',
          payload: {
            ...state.bootstrap,
            profiles: [
              ...state.bootstrap.profiles.filter((profile) => profile.profileId !== published.profileId),
              published,
            ],
          },
        });
      }
    } catch (error) {
      setSettingError(error instanceof Error ? error.message : '发布方案失败');
    }
  };

  /** 创建待发布的本地方案草稿；只有点击发布后才写入后端。 */
  const createProfileDraft = () => {
    const id = `profile-${Date.now()}`;
    const connectionId = activeModelId;
    const initialModel = activeModel
      ? inferModelVendor(activeModel.provider, activeModel.baseUrl).models[0]?.id || ''
      : '';
    const profile: CreativeProfile = {
      id,
      name: '长篇创作方案',
      version: 0,
      published: false,
      roles: {
        planner: { connectionId, model: initialModel, maxOutputTokens: 4096, temperature: 0.7 },
        writer: { connectionId, model: initialModel, maxOutputTokens: 4096, temperature: 0.7 },
        continuity_reviewer: { connectionId, model: initialModel, maxOutputTokens: 4096, temperature: 0.2 },
      },
      maxRunTokens: 30000,
      maxContextChars: 120000,
    };
    setCreativeProfiles((prev) => [...prev, profile]);
    setSelectedProfileId(id);
  };

  return (
    <div className="h-[calc(100%+3rem)] -m-6 flex overflow-hidden bg-surface animate-fade-up">
      <aside className="w-[260px] shrink-0 border-r border-border bg-surface px-4 py-5 flex flex-col">
        <button
          onClick={() => navigate(state.currentProject ? '/chapters' : '/')}
          className="flex items-center gap-2 px-2 py-2 mb-4 rounded-md text-sm text-text-sub hover:text-text hover:bg-surface-hover transition-colors"
        >
          <ArrowLeft className="w-4 h-4" />
          返回应用
        </button>
        <div className="relative mb-6">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-muted" />
          <Input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="搜索设置…"
            aria-label="搜索设置"
            className="pl-9 rounded-full bg-surface-hover"
          />
        </div>
        <p className="px-2 mb-2 text-xs font-medium text-text-muted uppercase tracking-wider">应用</p>
        <nav className="space-y-1">
        {visibleTabs.map((tab) => {
          const Icon = tab.icon;
          const isActive = activeTab === tab.key;
          return (
            <button
              key={tab.key}
              onClick={() => setActiveTab(tab.key)}
              className={clsx(
                'w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-colors text-left',
                isActive
                  ? 'text-text bg-primary-subtle'
                  : 'text-text-sub hover:text-text hover:bg-surface-hover',
              )}
            >
              <Icon className="w-4 h-4" />
              {tab.label}
              {tab.key === 'model' && activeModel && !activeModel.apiKeyConfigured && !apiKeys[activeModel.id]?.trim() && (
                <span className="ml-auto w-2 h-2 rounded-full bg-warning" title="未配置" />
              )}
            </button>
          );
        })}
        </nav>
        {visibleTabs.length === 0 && <p className="px-3 py-4 text-sm text-text-muted">没有匹配的设置</p>}
      </aside>

      <section className="flex-1 min-w-0 flex flex-col bg-atmosphere">
        <header className="w-full max-w-5xl mx-auto px-10 pt-9 pb-6">
          <h1 className="text-3xl font-display text-text">{activeTabMeta.label}</h1>
          <p className="text-sm text-text-sub mt-2">{tabDescriptions[activeTab]}</p>
        </header>

        <div className="flex-1 min-h-0 overflow-auto px-10 pb-8">
          <div className="w-full max-w-5xl mx-auto">
        {activeTab === 'model' && models.length === 0 && (
          <Card className="max-w-3xl p-10 flex flex-col items-center text-center">
            <Cpu className="w-10 h-10 text-text-muted mb-3" />
            <h3 className="text-lg font-medium text-text">还没有模型连接</h3>
            <p className="text-sm text-text-sub mt-2 mb-5">添加第一个模型连接以启用 AI 创作能力。</p>
            <Button variant="primary" size="sm" onClick={addModel}>
              <Plus className="w-4 h-4" />新增连接
            </Button>
          </Card>
        )}

        {activeTab === 'model' && models.length > 0 && (
          <div className="max-w-3xl space-y-5">
            {!activeModel.apiKeyConfigured && !apiKeys[activeModel.id]?.trim() && (
              <div className="flex items-start gap-3 p-3 bg-warning/10 border border-warning/20 rounded-md text-sm">
                <AlertTriangle className="w-4 h-4 text-warning shrink-0 mt-0.5" />
                <span className="text-text-sub">当前激活模型尚未配置 API Key，AI 功能将无法使用。</span>
              </div>
            )}
            <div className="grid grid-cols-[220px_1fr] gap-5">
              <Card className="p-3 space-y-2 h-fit">
                <div className="flex items-center justify-between mb-2">
                  <h3 className="text-xs font-medium text-text-sub uppercase tracking-wider">模型列表</h3>
                  <Button variant="ghost" size="sm" className="px-2 h-7" onClick={addModel}>
                    <Plus className="w-4 h-4" />
                  </Button>
                </div>
                <div className="space-y-1">
                  {models.map((m) => (
                    <button
                      key={m.id}
                      onClick={() => setActiveModelId(m.id)}
                      className={clsx(
                        'w-full text-left rounded-lg border p-2.5 transition-colors',
                        activeModelId === m.id
                          ? 'bg-primary-subtle border-primary/20'
                          : 'bg-surface border-border hover:border-border-strong'
                      )}
                    >
                      <div className="flex items-center justify-between gap-2">
                        <span className={clsx('text-sm truncate', activeModelId === m.id ? 'text-primary' : 'text-text')}>
                          {m.name}
                        </span>
                        {m.apiKeyConfigured || apiKeys[m.id]?.trim() ? (
                          <span className="w-2 h-2 rounded-full bg-success shrink-0" title="已配置" />
                        ) : (
                          <span className="w-2 h-2 rounded-full bg-warning shrink-0" title="未配置" />
                        )}
                      </div>
                      <div className="text-xs text-text-muted truncate mt-0.5">
                        {inferModelVendor(m.provider, m.baseUrl).label}
                      </div>
                    </button>
                  ))}
                </div>
              </Card>
              <Card className="space-y-5">
                <div>
                  <label className="block text-xs text-text-sub mb-1.5">显示名称</label>
                  <Input
                    value={activeModel.name}
                    onChange={(e) => updateModel(activeModel.id, { name: e.target.value })}
                    placeholder="例如：主力模型"
                  />
                </div>
                <div>
                  <label className="block text-xs text-text-sub mb-1.5">模型厂商</label>
                  <select
                    aria-label="模型厂商"
                    value={activeVendor.id}
                    onChange={(e) => selectVendor(e.target.value)}
                    className="h-9 px-3 w-full bg-surface border border-border rounded-md text-sm text-text focus:outline-none focus:ring-2 focus:ring-primary/35"
                  >
                    {modelVendors.map((vendor) => (
                      <option key={vendor.id} value={vendor.id}>{vendor.label}</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="block text-xs text-text-sub mb-1.5">接口地址</label>
                  <Input
                    aria-label="接口地址"
                    value={activeModel.baseUrl || ''}
                    onChange={(e) => updateModel(activeModel.id, { baseUrl: e.target.value })}
                    placeholder="https://api.openai.com/v1"
                  />
                  {activeVendor.endpointHint && <p className="text-xs text-text-muted mt-1.5">{activeVendor.endpointHint}</p>}
                </div>
                <div>
                  <label className="block text-xs text-text-sub mb-1.5">测试模型</label>
                  {activeVendor.models.length > 0 ? (
                    <div className="space-y-2">
                      <select
                        aria-label="测试模型"
                        value={activeVendor.models.some((model) => model.id === activeTestModel) ? activeTestModel : '__custom__'}
                        onChange={(event) => setTestModels((previous) => ({
                          ...previous,
                          [activeModel.id]: event.target.value === '__custom__' ? '' : event.target.value,
                        }))}
                        className="h-9 px-3 w-full bg-surface border border-border rounded-md text-sm text-text focus:outline-none focus:ring-2 focus:ring-primary/35"
                      >
                        {activeVendor.models.map((model) => <option key={model.id} value={model.id}>{model.label}</option>)}
                        <option value="__custom__">自定义模型 ID…</option>
                      </select>
                      {!activeVendor.models.some((model) => model.id === activeTestModel) && (
                        <Input
                          aria-label="自定义测试模型 ID"
                          value={activeTestModel}
                          onChange={(event) => setTestModels((previous) => ({ ...previous, [activeModel.id]: event.target.value }))}
                          placeholder="输入厂商提供的模型 ID"
                        />
                      )}
                    </div>
                  ) : (
                    <Input
                      aria-label="测试模型"
                      value={activeTestModel}
                      onChange={(event) => setTestModels((previous) => ({ ...previous, [activeModel.id]: event.target.value }))}
                      placeholder="输入兼容服务的模型 ID"
                    />
                  )}
                  <p className="text-xs text-text-muted mt-1.5">这里只用于连接测试；长期使用的模型在“创作方案”中保存。</p>
                </div>
                <div>
                  <label className="block text-xs text-text-sub mb-1.5">API Key</label>
                  <Input
                    type="password"
                    value={apiKeys[activeModel.id] || ''}
                    onChange={(e) => setApiKeys((prev) => ({ ...prev, [activeModel.id]: e.target.value }))}
                    placeholder={activeModel.apiKeyConfigured ? '已配置 · 重新输入可覆盖' : '输入 API Key...'}
                  />
                </div>
                <div className="space-y-2 pt-1">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <Button
                        variant="default"
                        size="sm"
                        onClick={() => void handleTestConnection(activeModel.id)}
                        disabled={testingId === activeModel.id}
                      >
                        <TestTube className="w-4 h-4" />
                        {testingId === activeModel.id ? '测试中...' : '测试连接'}
                      </Button>
                      {testOkId === activeModel.id && testResult && (
                        <span className="text-sm text-success">连接成功 · {testResult.latencyMs}ms · {testResult.outputTokens} tokens</span>
                      )}
                    </div>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="gap-1 text-danger hover:text-danger"
                      onClick={() => deleteModel(activeModel.id)}
                      disabled={deleteBusyId === activeModel.id}
                    >
                      <Trash2 className="w-4 h-4" />
                      {deleteBusyId === activeModel.id ? '删除中...' : '删除'}
                    </Button>
                  </div>
                  <p className="text-xs text-text-muted">测试将产生一次极低额度的真实模型调用。</p>
                </div>
              </Card>
            </div>
          </div>
        )}

        {activeTab === 'writing' && selectedProfile && (
          <div className="max-w-4xl grid grid-cols-1 md:grid-cols-[220px_1fr] gap-5">
            <Card className="p-3 h-fit space-y-2">
              <div className="flex items-center justify-between mb-2">
                <h2 className="text-xs font-medium text-text-sub uppercase tracking-wider">创作方案</h2>
                <StatusBadge variant="success">应用级</StatusBadge>
              </div>
              {creativeProfiles.map((profile) => (
                <button
                  key={profile.id}
                  onClick={() => setSelectedProfileId(profile.id)}
                  className={clsx(
                    'w-full text-left rounded-lg border p-3 transition-colors',
                    selectedProfile.id === profile.id ? 'bg-primary-subtle border-primary/20' : 'bg-surface border-border hover:border-border-strong',
                  )}
                >
                  <p className="text-sm font-medium text-text">{profile.name}</p>
                  <p className="text-xs text-text-muted mt-1">v{profile.version} · {profile.published ? '已发布' : '草案'}</p>
                  {defaultCreativeProfileId === profile.id && <p className="text-xs text-primary mt-1">默认方案</p>}
                </button>
              ))}
              <Button variant="ghost" size="sm" className="w-full justify-center" onClick={createProfileDraft} disabled={!models.length}>
                <Plus className="w-4 h-4" />新增方案
              </Button>
            </Card>

            <Card className="space-y-6">
              <div className="flex flex-col sm:flex-row sm:items-start justify-between gap-3">
                <div>
                  <h2 className="text-base font-medium text-text">{selectedProfile.name} · v{selectedProfile.version}</h2>
                  <p className="text-sm text-text-sub mt-1">为不同创作职责分配模型。每次运行会冻结实际方案与模型来源。</p>
                </div>
                <div className="flex gap-2 shrink-0">
                  <Button
                    variant={defaultCreativeProfileId === selectedProfile.id ? 'default' : 'primary'}
                    size="sm"
                    // 未发布草稿禁止设为默认：后端持久化默认方案前会校验最新版本存在，
                    // 草稿 version=0 还没有持久化版本，设为默认会被后端拒绝。
                    disabled={!selectedProfile.published && defaultCreativeProfileId !== selectedProfile.id}
                    onClick={() => setDefaultCreativeProfileId(
                      defaultCreativeProfileId === selectedProfile.id ? '' : selectedProfile.id,
                    )}
                  >
                    {defaultCreativeProfileId === selectedProfile.id ? '取消默认' : '设为默认'}
                  </Button>
                  <Button variant="default" size="sm" onClick={() => void publishNextProfileVersion()}>发布下一版本</Button>
                </div>
              </div>

              <div className="rounded-lg border border-primary/15 bg-primary-subtle px-3 py-2 text-sm text-text-sub">
                默认方案的调整<strong className="font-medium text-text">只影响以后新建的作品</strong>；已有作品保持创建时选定的方案版本，可在“文件 → 项目设置”中主动更换或覆盖。
              </div>

              <section>
                <h3 className="text-sm font-medium text-text mb-3">角色分配</h3>
                <div className="space-y-4">
                  {(Object.keys(creativeRoleLabels) as CreativeRole[]).map((role) => {
                    const binding = selectedProfile.roles[role];
                    const connection = models.find((model) => model.id === binding.connectionId) || models[0];
                    const vendor = connection
                      ? inferModelVendor(connection.provider, connection.baseUrl)
                      : getModelVendor('openai');
                    const knownModel = vendor.models.some((model) => model.id === binding.model);
                    return (
                      <div key={role} className="rounded-lg border border-border bg-surface px-3 py-3 space-y-2">
                        <div className="grid grid-cols-[80px_1fr_1fr] gap-3 items-center">
                          <label htmlFor={`role-${role}`} className="text-sm text-text-sub">{creativeRoleLabels[role]}</label>
                          <select
                            id={`role-${role}`}
                            aria-label={creativeRoleLabels[role]}
                            value={binding.connectionId}
                            onChange={(event) => {
                              const connectionId = event.target.value;
                              updateProfileRole(role, connectionId);
                              const nextConnection = models.find((model) => model.id === connectionId);
                              const nextVendor = nextConnection
                                ? inferModelVendor(nextConnection.provider, nextConnection.baseUrl)
                                : getModelVendor('openai');
                              updateProfileRoleModel(role, nextVendor.models[0]?.id || '');
                            }}
                            className="h-9 px-3 w-full bg-surface border border-border rounded-md text-sm text-text focus:outline-none focus:ring-2 focus:ring-primary/35"
                          >
                            {models.map((model) => (
                              <option key={model.id} value={model.id}>{model.name}</option>
                            ))}
                          </select>
                          <div className="space-y-2">
                            <select
                              value={knownModel ? binding.model : '__custom__'}
                              onChange={(event) => updateProfileRoleModel(role, event.target.value === '__custom__' ? '' : event.target.value)}
                              aria-label={`${creativeRoleLabels[role]}模型`}
                              className="h-9 px-3 w-full bg-surface border border-border rounded-md text-sm text-text focus:outline-none focus:ring-2 focus:ring-primary/35"
                            >
                              {vendor.models.map((model) => <option key={model.id} value={model.id}>{model.label}</option>)}
                              <option value="__custom__">自定义模型 ID…</option>
                            </select>
                            {!knownModel && (
                              <Input
                                value={binding.model}
                                onChange={(event) => updateProfileRoleModel(role, event.target.value)}
                                placeholder="输入厂商提供的模型 ID"
                                aria-label={`${creativeRoleLabels[role]}自定义模型 ID`}
                              />
                            )}
                          </div>
                        </div>
                        <div className="grid grid-cols-[80px_1fr_1fr] gap-3 items-center">
                          <span className="text-xs text-text-sub">生成参数</span>
                          <label className="flex items-center gap-2 text-xs text-text-sub">
                            <span className="shrink-0">最大输出</span>
                            <Input
                              type="number"
                              min={1}
                              value={binding.maxOutputTokens}
                              onChange={(event) => updateProfileRoleParam(role, { maxOutputTokens: Math.max(1, Number(event.target.value) || 0) })}
                              aria-label={`${creativeRoleLabels[role]}最大输出 Token`}
                              className="h-8"
                            />
                            <span className="shrink-0">tokens</span>
                          </label>
                          <label className="flex items-center gap-2 text-xs text-text-sub">
                            <span className="shrink-0">温度</span>
                            <Input
                              type="number"
                              min={0}
                              max={2}
                              step={0.1}
                              value={binding.temperature}
                              onChange={(event) => updateProfileRoleParam(role, { temperature: Math.min(2, Math.max(0, Number(event.target.value) || 0)) })}
                              aria-label={`${creativeRoleLabels[role]}温度`}
                              className="h-8"
                            />
                          </label>
                        </div>
                      </div>
                    );
                  })}
                </div>
              </section>

              <section>
                <h3 className="text-sm font-medium text-text mb-3">运行预算</h3>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <label className="flex items-center gap-2 text-sm text-text-sub">
                    <span className="shrink-0 w-28">运行 Token 预算</span>
                    <Input
                      type="number"
                      min={1}
                      value={selectedProfile.maxRunTokens || 30000}
                      onChange={(event) => updateProfileBudget({ maxRunTokens: Math.max(1, Number(event.target.value) || 0) })}
                      aria-label="运行 Token 预算"
                    />
                  </label>
                  <label className="flex items-center gap-2 text-sm text-text-sub">
                    <span className="shrink-0 w-28">上下文字符预算</span>
                    <Input
                      type="number"
                      min={1}
                      value={selectedProfile.maxContextChars || 120000}
                      onChange={(event) => updateProfileBudget({ maxContextChars: Math.max(1, Number(event.target.value) || 0) })}
                      aria-label="上下文字符预算"
                    />
                  </label>
                </div>
                <p className="text-xs text-text-muted mt-2">三个角色的最大输出 Token 之和不能超过运行 Token 预算，否则创建运行时会被拒绝。</p>
              </section>
            </Card>
          </div>
        )}

        {activeTab === 'writing' && !selectedProfile && (
          <Card className="max-w-2xl space-y-4">
            <h2 className="text-base font-medium text-text">还没有创作方案</h2>
            <p className="text-sm text-text-sub">先保存至少一个模型连接，再创建方案草稿并为三类角色（规划、写作、审阅）选择模型。点击“发布下一版本”后才会写入后端。</p>
            <Button variant="primary" onClick={createProfileDraft} disabled={!models.length}>
              <Plus className="w-4 h-4" />创建方案草稿
            </Button>
          </Card>
        )}

        {activeTab === 'templates' && (
          <Card className="max-w-2xl space-y-3">
            <h2 className="text-base font-medium text-text">作者模板尚未接入</h2>
            <p className="text-sm leading-6 text-text-sub">当前后端没有作者模板的保存与读取接口，因此这里不会再显示或保存仅存在于内存中的示例模板。</p>
          </Card>
        )}

        {activeTab === 'appearance' && (
          <Card className="max-w-2xl space-y-6">
            <div>
              <label className="block text-xs text-text-sub mb-2">主题</label>
              <div className="flex gap-3">
                {(['dark', 'light', 'system'] as const).map((t) => (
                  <label
                    key={t}
                    className={clsx(
                      'flex items-center gap-2 px-3 py-2 rounded-md border text-sm cursor-pointer transition-colors',
                      theme === t
                        ? 'bg-primary-subtle border-primary/20 text-primary'
                        : 'bg-surface border-border text-text-sub hover:text-text',
                    )}
                  >
                    <input
                      type="radio"
                      name="theme"
                      value={t}
                      checked={theme === t}
                      onChange={() => setTheme(t)}
                      className="accent-primary"
                    />
                    {t === 'dark' ? '深色' : t === 'light' ? '浅色' : '跟随系统'}
                  </label>
                ))}
              </div>
            </div>
            <div>
              <div className="flex justify-between text-xs text-text-sub mb-2">
                <label>字号</label>
                <span>{fontSize}px</span>
              </div>
              <input
                type="range"
                min={14}
                max={24}
                step={1}
                value={fontSize}
                onChange={(e) => setFontSize(Number(e.target.value))}
                className="w-full accent-primary"
              />
            </div>
            <div>
              <div className="flex justify-between text-xs text-text-sub mb-2">
                <label>行高</label>
                <span>{lineHeight}</span>
              </div>
              <input
                type="range"
                min={1.4}
                max={2.2}
                step={0.05}
                value={lineHeight}
                onChange={(e) => setLineHeight(Number(e.target.value))}
                className="w-full accent-primary"
              />
            </div>
            <div>
              <label className="block text-xs text-text-sub mb-2">画布背景</label>
              <div className="grid grid-cols-4 gap-2">
                {canvasPresets.map((preset) => (
                  <button
                    key={preset.id}
                    onClick={() => setCanvasColorId(preset.id)}
                    className={clsx(
                      'h-10 rounded-lg border-2 transition-all flex items-center justify-center',
                      canvasColorId === preset.id ? 'border-primary scale-105' : 'border-transparent hover:border-border-strong'
                    )}
                    style={{ backgroundColor: preset.bg }}
                    title={preset.label}
                  />
                ))}
              </div>
              <div className="flex justify-between text-xs text-text-muted mt-2 px-1">
                {canvasPresets.map((p) => (
                  <span key={p.id}>{p.label}</span>
                ))}
              </div>
            </div>
            <div>
              <label className="block text-xs text-text-sub mb-2">边距</label>
              <div className="flex rounded-lg border border-border overflow-hidden">
                {(['narrow', 'medium', 'wide'] as const).map((m) => (
                  <button
                    key={m}
                    onClick={() => setMarginSize(m)}
                    className={clsx(
                      'flex-1 py-1.5 text-xs transition-colors',
                      marginSize === m ? 'bg-primary text-white' : 'text-text-sub hover:bg-surface-hover'
                    )}
                  >
                    {m === 'narrow' ? '窄' : m === 'medium' ? '中' : '宽'}
                  </button>
                ))}
              </div>
            </div>
            <div className="flex items-center justify-between">
              <label className="text-xs text-text-sub">首行缩进</label>
              <button
                onClick={() => setIndent((v) => !v)}
                className={clsx(
                  'w-9 h-5 rounded-full transition-colors relative',
                  indent ? 'bg-primary' : 'bg-surface-hover'
                )}
              >
                <span
                  className={clsx(
                    'absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow-sm transition-transform',
                    indent && 'translate-x-4'
                  )}
                />
              </button>
            </div>
          </Card>
        )}

        {activeTab === 'advanced' && (
          <Card className="max-w-2xl space-y-5">
            <div className="flex items-center justify-between p-3 bg-surface rounded-md border border-border">
              <div>
                <p className="text-sm font-medium text-text">自动保存</p>
                <p className="text-xs text-text-sub">在创作驾驶舱与章节编辑中定时保存进度</p>
              </div>
              <button
                onClick={() => setAutoSave((v) => !v)}
                className={clsx(
                  'w-11 h-6 rounded-full transition-colors relative',
                  autoSave ? 'bg-primary' : 'bg-surface-hover',
                )}
              >
                <span
                  className={clsx(
                    'absolute top-1 w-4 h-4 rounded-full bg-text transition-all',
                    autoSave ? 'left-6' : 'left-1',
                  )}
                />
              </button>
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label className="block text-xs text-text-sub mb-1.5">本地备份保留数量</label>
                <Input
                  type="number"
                  min={1}
                  max={50}
                  value={backupCount}
                  onChange={(e) => setBackupCount(Number(e.target.value))}
                />
              </div>
              <div>
                <label className="block text-xs text-text-sub mb-1.5">界面语言</label>
                <select
                  value={language}
                  onChange={(e) => setLanguage(e.target.value)}
                  className="h-9 px-3 w-full bg-surface border border-border rounded-md text-sm text-text focus:outline-none focus:ring-2 focus:ring-primary/35"
                >
                  <option value="zh-CN">简体中文</option>
                  <option value="zh-TW">繁体中文</option>
                  <option value="en">English</option>
                </select>
              </div>
            </div>
          </Card>
        )}
          </div>
        </div>

        <footer className="border-t border-border bg-surface/90 px-10 py-4">
          <div className="max-w-5xl mx-auto flex items-center justify-between gap-4">
            {settingError ? (
              <p className="text-sm text-danger flex items-center gap-2">
                <AlertTriangle className="w-4 h-4 shrink-0" />
                <span className="truncate">{settingError}</span>
              </p>
            ) : (
              <span />
            )}
            <Button
              variant={saved ? 'default' : 'primary'}
              onClick={handleSave}
              disabled={saving}
              className={saved ? 'text-success border-success/30' : ''}
            >
              {saved ? <Check className="w-4 h-4" /> : <Save className="w-4 h-4" />}
              {saved ? '已保存' : saving ? '保存中...' : '保存设置'}
            </Button>
          </div>
        </footer>
      </section>
      <Modal
        open={Boolean(deleteTarget)}
        onClose={() => !deleteBusyId && setDeleteTargetId(null)}
        title={deleteReferences.length > 0 ? '无法删除连接' : '确认删除连接'}
      >
        {deleteReferences.length > 0 ? (
          <div className="space-y-4">
            <p className="text-sm text-text-sub">当前连接仍被以下创作方案引用：</p>
            <ul className="rounded-md border border-border bg-surface-hover px-4 py-3 space-y-1">
              {deleteReferences.map((profile) => <li key={profile.id} className="text-sm text-text">{profile.name}</li>)}
            </ul>
            <p className="text-sm text-text-sub">草稿可先把角色切换到其他连接；已发布方案会永久保留不可变历史引用，因此该连接不能物理删除。这样旧项目和历史运行仍能解释原来的模型来源。</p>
            <div className="flex justify-end">
              <Button variant="primary" onClick={() => setDeleteTargetId(null)}>知道了</Button>
            </div>
          </div>
        ) : (
          <div className="space-y-4">
            <p className="text-sm text-text-sub">确认删除连接“{deleteTarget?.name}”？对应的系统凭据也会被删除，此操作不可撤销。</p>
            {settingError && <p className="text-sm text-danger">{settingError}</p>}
            <div className="flex justify-end gap-2">
              <Button variant="ghost" onClick={() => setDeleteTargetId(null)} disabled={Boolean(deleteBusyId)}>取消</Button>
              <Button variant="danger" onClick={() => void confirmDeleteModel()} disabled={Boolean(deleteBusyId)}>
                {deleteBusyId ? '删除中...' : '确认删除'}
              </Button>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
}
