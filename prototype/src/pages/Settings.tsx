import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { clsx } from 'clsx';
import {
  Cpu, Feather, FileText, Palette, SlidersHorizontal, Save, Check,
  AlertTriangle, TestTube, Trash2, Plus, ArrowLeft, Search,
} from 'lucide-react';
import { useApp } from '../store/useApp';
import type { AppSettings, CreativeProfile, CreativeRole, ModelConfig } from '../types';
import { getActiveModel } from '../types';
import { canvasPresets } from '../editor';
import type { MarginSize } from '../editor';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { Input, Textarea } from '../components/ui/Input';
import { StatusBadge } from '../components/ui/StatusBadge';
import { Modal } from '../components/ui/Modal';
import { inferVendor, modelVendors } from '../modelCatalog';

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

interface AuthorTemplate {
  id: string;
  name: string;
  content: string;
}

const initialTemplates: AuthorTemplate[] = [
  { id: 't1', name: '场景描写模板', content: '先写环境，再写人物动作，最后写心理反应。' },
  { id: 't2', name: '对话模板', content: '一句台词 + 一个动作细节 + 一句潜台词。' },
];

const creativeRoleLabels: Record<CreativeRole, string> = {
  planning: '规划',
  writing: '写作',
  review: '审阅',
  memory: '记忆',
  impact: '影响分析',
};

export default function Settings() {
  const navigate = useNavigate();
  const { state, saveSettings } = useApp();
  const [activeTab, setActiveTab] = useState<TabKey>('model');
  const [search, setSearch] = useState('');
  const [saved, setSaved] = useState(false);
  const [testingId, setTestingId] = useState<string | null>(null);
  const [testOkId, setTestOkId] = useState<string | null>(null);
  const [deleteTargetId, setDeleteTargetId] = useState<string | null>(null);

  const [models, setModels] = useState<ModelConfig[]>(state.settings.models);
  const [activeModelId, setActiveModelId] = useState(state.settings.activeModelId);
  const [apiKeys, setApiKeys] = useState<Record<string, string>>({});
  const activeModel = useMemo(() => getActiveModel({ ...state.settings, models, activeModelId }), [state.settings, models, activeModelId]);
  const activeVendor = inferVendor(activeModel.provider, activeModel.baseUrl);
  const deleteTarget = models.find((model) => model.id === deleteTargetId);

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

  const [templates, setTemplates] = useState<AuthorTemplate[]>(initialTemplates);
  const [newTemplateName, setNewTemplateName] = useState('');
  const [newTemplateContent, setNewTemplateContent] = useState('');

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
      { id, name: `模型 ${prev.length + 1}`, provider: 'openai', baseUrl: 'https://api.openai.com/v1', modelName: 'gpt-4.1-mini', apiKeyConfigured: false },
    ]);
    setActiveModelId(id);
  };

  const deleteModel = (id: string) => {
    setModels((prev) => {
      const next = prev.filter((m) => m.id !== id);
      if (activeModelId === id) {
        setActiveModelId(next[0]?.id || '');
      }
      return next;
    });
    setDeleteTargetId(null);
  };

  const handleTestConnection = (id: string) => {
    setTestingId(id);
    setTestOkId(null);
    setTimeout(() => {
      setTestingId(null);
      setTestOkId(id);
      setTimeout(() => setTestOkId(null), 3000);
    }, 1200);
  };

  const handleSave = async () => {
    const next: AppSettings = {
      models: models.map((m) => ({
        ...m,
        apiKeyConfigured: m.apiKeyConfigured || apiKeys[m.id]?.trim().length > 0,
      })),
      activeModelId,
      creativeProfiles,
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
  };

  /** 更新方案中的单一角色，项目仍可在文件菜单中进行局部覆盖。 */
  const updateProfileRole = (role: CreativeRole, modelId: string) => {
    if (!selectedProfile) return;
    setCreativeProfiles((profiles) => profiles.map((profile) => (
      profile.id === selectedProfile.id
        ? { ...profile, roleModels: { ...profile.roleModels, [role]: modelId } }
        : profile
    )));
  };

  /** 将当前编辑方案发布为下一版本，避免静默改变已经创建的作品。 */
  const publishNextProfileVersion = () => {
    if (!selectedProfile) return;
    setCreativeProfiles((profiles) => profiles.map((profile) => (
      profile.id === selectedProfile.id
        ? { ...profile, version: profile.version + 1, published: true }
        : profile
    )));
  };

  const addTemplate = () => {
    if (!newTemplateName.trim() || !newTemplateContent.trim()) return;
    setTemplates((prev) => [
      ...prev,
      { id: `t-${Date.now()}`, name: newTemplateName.trim(), content: newTemplateContent.trim() },
    ]);
    setNewTemplateName('');
    setNewTemplateContent('');
  };

  const deleteTemplate = (id: string) => {
    setTemplates((prev) => prev.filter((t) => t.id !== id));
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
              {tab.key === 'model' && !activeModel.apiKeyConfigured && !apiKeys[activeModel.id]?.trim() && (
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
        {activeTab === 'model' && (
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
                      <div className="text-xs text-text-muted truncate mt-0.5">{inferVendor(m.provider, m.baseUrl).label}</div>
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
                    value={activeVendor.id}
                    onChange={(event) => {
                      const vendor = modelVendors.find((item) => item.id === event.target.value) || modelVendors[0];
                      updateModel(activeModel.id, {
                        provider: vendor.protocol,
                        baseUrl: vendor.defaultBaseUrl,
                        modelName: vendor.models[0]?.id || '',
                      });
                    }}
                    className="h-9 px-3 w-full bg-surface border border-border rounded-md text-sm text-text focus:outline-none focus:ring-2 focus:ring-primary/35"
                  >
                    {modelVendors.map((vendor) => <option key={vendor.id} value={vendor.id}>{vendor.label}</option>)}
                  </select>
                </div>
                <div>
                  <label className="block text-xs text-text-sub mb-1.5">接口地址</label>
                  <Input
                    value={activeModel.baseUrl || ''}
                    onChange={(e) => updateModel(activeModel.id, { baseUrl: e.target.value })}
                    placeholder="https://api.openai.com/v1"
                  />
                </div>
                <div>
                  <label className="block text-xs text-text-sub mb-1.5">测试模型</label>
                  {activeVendor.models.length > 0 ? (
                    <select
                      value={activeVendor.models.some((model) => model.id === activeModel.modelName) ? activeModel.modelName : '__custom__'}
                      onChange={(event) => updateModel(activeModel.id, { modelName: event.target.value === '__custom__' ? '' : event.target.value })}
                      className="h-9 px-3 w-full bg-surface border border-border rounded-md text-sm text-text focus:outline-none focus:ring-2 focus:ring-primary/35"
                    >
                      {activeVendor.models.map((model) => <option key={model.id} value={model.id}>{model.label}</option>)}
                      <option value="__custom__">自定义模型 ID…</option>
                    </select>
                  ) : (
                    <Input value={activeModel.modelName} onChange={(e) => updateModel(activeModel.id, { modelName: e.target.value })} placeholder="输入模型 ID" />
                  )}
                  {!activeVendor.models.some((model) => model.id === activeModel.modelName) && activeVendor.models.length > 0 && (
                    <Input className="mt-2" value={activeModel.modelName} onChange={(e) => updateModel(activeModel.id, { modelName: e.target.value })} placeholder="输入模型 ID" />
                  )}
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
                <div className="flex items-center justify-between pt-1">
                  <div className="flex items-center gap-3">
                    <Button
                      variant="default"
                      size="sm"
                      onClick={() => handleTestConnection(activeModel.id)}
                      disabled={testingId === activeModel.id}
                    >
                      <TestTube className="w-4 h-4" />
                      {testingId === activeModel.id ? '测试中...' : '测试连接'}
                    </Button>
                    {testOkId === activeModel.id && <span className="text-sm text-success">连接成功</span>}
                  </div>
                  <Button variant="ghost" size="sm" className="gap-1 text-danger hover:text-danger" onClick={() => setDeleteTargetId(activeModel.id)}>
                    <Trash2 className="w-4 h-4" />
                    删除
                  </Button>
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
                    disabled={defaultCreativeProfileId === selectedProfile.id}
                    onClick={() => setDefaultCreativeProfileId(selectedProfile.id)}
                  >
                    {defaultCreativeProfileId === selectedProfile.id ? '当前默认' : '设为默认'}
                  </Button>
                  <Button variant="default" size="sm" onClick={publishNextProfileVersion}>发布下一版本</Button>
                </div>
              </div>

              <div className="rounded-lg border border-primary/15 bg-primary-subtle px-3 py-2 text-sm text-text-sub">
                默认方案的调整<strong className="font-medium text-text">只影响以后新建的作品</strong>；已有作品保持创建时选定的方案版本，可在“文件 → 项目设置”中主动更换或覆盖。
              </div>

              <section>
                <h3 className="text-sm font-medium text-text mb-3">角色分配</h3>
                <div className="space-y-3">
                  {(Object.keys(creativeRoleLabels) as CreativeRole[]).map((role) => (
                    <div key={role} className="grid grid-cols-[100px_1fr] gap-3 items-center">
                      <label htmlFor={`role-${role}`} className="text-sm text-text-sub">{creativeRoleLabels[role]}</label>
                      <select
                        id={`role-${role}`}
                        aria-label={creativeRoleLabels[role]}
                        value={selectedProfile.roleModels[role]}
                        onChange={(event) => updateProfileRole(role, event.target.value)}
                        className="h-9 px-3 w-full bg-surface border border-border rounded-md text-sm text-text focus:outline-none focus:ring-2 focus:ring-primary/35"
                      >
                        {models.map((model) => (
                          <option key={model.id} value={model.id}>{model.name} · {model.modelName || '未命名模型'}</option>
                        ))}
                      </select>
                    </div>
                  ))}
                </div>
              </section>
            </Card>
          </div>
        )}

        {activeTab === 'templates' && (
          <Card className="max-w-2xl space-y-5">
            <div className="space-y-3">
              {templates.length === 0 && <p className="text-sm text-text-muted">暂无作者模板。</p>}
              {templates.map((t) => (
                <div key={t.id} className="p-3 bg-surface rounded-md border border-border group">
                  <div className="flex items-center justify-between mb-1">
                    <span className="text-sm font-medium text-text">{t.name}</span>
                    <Button variant="ghost" size="sm" className="px-2 opacity-0 group-hover:opacity-100" onClick={() => deleteTemplate(t.id)}>
                      <Trash2 className="w-4 h-4 text-danger" />
                    </Button>
                  </div>
                  <p className="text-sm text-text-sub">{t.content}</p>
                </div>
              ))}
            </div>
            <div className="border-t border-border pt-4 space-y-3">
              <p className="text-xs text-text-muted">添加新模板</p>
              <Input
                value={newTemplateName}
                onChange={(e) => setNewTemplateName(e.target.value)}
                placeholder="模板名称"
              />
              <Textarea
                value={newTemplateContent}
                onChange={(e) => setNewTemplateContent(e.target.value)}
                rows={2}
                placeholder="模板内容..."
              />
              <Button variant="default" size="sm" onClick={addTemplate} disabled={!newTemplateName.trim() || !newTemplateContent.trim()}>
                <Plus className="w-4 h-4" />
                添加模板
              </Button>
            </div>
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
          <div className="max-w-5xl mx-auto flex justify-end">
            <Button
              variant={saved ? 'default' : 'primary'}
              onClick={handleSave}
              className={saved ? 'text-success border-success/30' : ''}
            >
              {saved ? <Check className="w-4 h-4" /> : <Save className="w-4 h-4" />}
              {saved ? '已保存' : '保存设置'}
            </Button>
          </div>
        </footer>
      </section>
      <Modal open={Boolean(deleteTarget)} onClose={() => setDeleteTargetId(null)} title="确认删除连接">
        <div className="space-y-4">
          <p className="text-sm text-text-sub">确认删除连接“{deleteTarget?.name}”？正式应用会同时检查不可变方案引用并清理系统凭据。</p>
          <div className="flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setDeleteTargetId(null)}>取消</Button>
            <Button variant="danger" onClick={() => deleteTarget && deleteModel(deleteTarget.id)}>确认删除</Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
