import { useEffect, useState, useMemo } from 'react';
import { clsx } from 'clsx';
import { Users, Flag, User, Swords, Eye, HelpCircle } from 'lucide-react';
import type { Character, Faction, CharacterRole, FactionType } from '../../types';
import { CharacterRevisions } from '../../components/CharacterRevisions';
import { useApp } from '../../store/useApp';
import { studio } from '../../api/studio';
import { Card } from '../../components/ui/Card';
import { Button } from '../../components/ui/Button';
import { Modal } from '../../components/ui/Modal';
import { StatusBadge } from '../../components/ui/StatusBadge';
import { EmptyState } from '../../components/ui/EmptyState';

const roleLabels: Record<CharacterRole, string> = {
  protagonist: '主角',
  supporting: '配角',
  antagonist: '反派',
  minor: '次要',
};

const roleBadgeVariant: Record<CharacterRole, 'warning' | 'info' | 'danger' | 'default'> = {
  protagonist: 'warning',
  supporting: 'info',
  antagonist: 'danger',
  minor: 'default',
};

const factionTypeLabels: Record<FactionType, string> = {
  family: '家族',
  organization: '组织',
  nation: '国家',
  sect: '门派',
  other: '其他',
};

const factionTypeVariant: Record<FactionType, 'info' | 'success' | 'warning' | 'default'> = {
  family: 'info',
  organization: 'success',
  nation: 'warning',
  sect: 'default',
  other: 'default',
};

type Tab = 'characters' | 'factions';

export default function Characters() {
  const {state, registerBeforeClose} = useApp();
  const [loadError, setLoadError] = useState('');
  const [activeTab, setActiveTab] = useState<Tab>('characters');
  const [selectedCharacter, setSelectedCharacter] = useState<Character | null>(null);
  const [characters, setCharacters] = useState<Character[]>([]);
  const [factions, setFactions] = useState<Faction[]>([]);

  useEffect(() => {
    let active = true;
    void studio.storyWorld().then((world) => {
      if (!active) return;
      const memberships = new Map(world.memberships.map((item) => [String(item.characterId || ''), String(item.factionId || '')]));
      setCharacters(world.characters.map((item) => ({
        id: String(item.id || ''),
        projectId: '',
        name: String(item.name || ''),
        role: (['protagonist', 'supporting', 'antagonist', 'minor'].includes(String(item.role)) ? item.role : 'minor') as CharacterRole,
        factionId: memberships.get(String(item.id || '')) || undefined,
        tags: Array.isArray(item.aliases) ? item.aliases.map(String) : [],
        summary: String(item.summary || ''),
        appearance: '',
        secret: String(item.secret || ''), fear: String(item.fear || ''), arc: String(item.arc || ''),
        motivation: String(item.motivation || ''),
      })));
      setFactions(world.factions.map((item) => ({
        id: String(item.id || ''),
        projectId: '',
        name: String(item.name || ''),
        type: 'other' as FactionType,
        description: String(item.summary || ''),
        goals: String(item.goal || ''),
        conflicts: '',
      })));
    }).catch((e: unknown) => { if (active) setLoadError(e instanceof Error ? e.message : String(e)); });
    return () => { active = false; };
  }, []);

  const factionMap = useMemo(() => {
    const map = new Map<string, Faction>();
    factions.forEach((f) => map.set(f.id, f));
    return map;
  }, [factions]);

  return (
    <div className="space-y-6 animate-fade-up">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-display text-text">人物与势力</h2>
          <p className="text-sm text-text-sub mt-0.5">管理角色档案与势力关系，保持人设一致性。</p>
        </div>
        <div className="flex bg-surface rounded-md p-1 border border-border">
          <button
            onClick={() => setActiveTab('characters')}
            className={clsx(
              'flex items-center gap-2 px-3 py-1.5 text-sm rounded transition-colors',
              activeTab === 'characters' ? 'bg-surface-elevated text-text shadow-sm' : 'text-text-sub hover:text-text',
            )}
          >
            <Users className="w-4 h-4" />
            人物
          </button>
          <button
            onClick={() => setActiveTab('factions')}
            className={clsx(
              'flex items-center gap-2 px-3 py-1.5 text-sm rounded transition-colors',
              activeTab === 'factions' ? 'bg-surface-elevated text-text shadow-sm' : 'text-text-sub hover:text-text',
            )}
          >
            <Flag className="w-4 h-4" />
            势力
          </button>
        </div>
      </div>

      {loadError && <p role="alert" className="text-sm text-danger">读取人物档案失败：{loadError}</p>}
      <div className={activeTab === 'characters' ? '' : 'hidden'}><CharacterRevisions api={studio} registerBeforeLeave={registerBeforeClose} readOnly={state.backendProject?.accessMode === 'read_only'} /><p className="text-xs text-text-muted mt-2">人物卡片显示原始档案；按章节生效的新资料见修订记录。</p></div>
      {activeTab === 'characters' && (
        <div>
          {characters.length === 0 ? (
            <EmptyState
              icon={User}
              title="还没有角色"
              description="创建主要与次要角色，完善人物小传。"
            />
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
              {characters.map((char, idx) => (
                <Card
                  key={char.id}
                  hover
                  className="cursor-pointer"
                  onClick={() => setSelectedCharacter(char)}
                  style={{ animationDelay: `${idx * 60}ms` }}
                >
                  <div className="flex items-start justify-between gap-3 mb-3">
                    <div className="flex items-center gap-3">
                      <div className="w-10 h-10 rounded-full bg-surface border border-border flex items-center justify-center">
                        <User className="w-5 h-5 text-text-sub" />
                      </div>
                      <div>
                        <h3 className="text-base font-medium text-text">{char.name}</h3>
                        <StatusBadge variant={roleBadgeVariant[char.role]}>
                          {roleLabels[char.role]}
                        </StatusBadge>
                      </div>
                    </div>
                  </div>
                  <p className="text-sm text-text-sub line-clamp-2 mb-3">{char.summary}</p>
                  <div className="flex flex-wrap gap-2">
                    {char.tags.map((tag) => (
                      <span
                        key={tag}
                        className="text-xs px-2 py-0.5 rounded bg-surface-hover text-text-sub border border-border"
                      >
                        {tag}
                      </span>
                    ))}
                  </div>
                </Card>
              ))}
            </div>
          )}
        </div>
      )}

      {activeTab === 'factions' && (
        <div>
          {factions.length === 0 ? (
            <EmptyState
              icon={Flag}
              title="还没有势力"
              description="创建家族、组织或阵营，梳理冲突关系。"
            />
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {factions.map((faction, idx) => {
                const members = characters.filter((c) => c.factionId === faction.id);
                return (
                  <Card key={faction.id} style={{ animationDelay: `${idx * 60}ms` }}>
                    <div className="flex items-start justify-between gap-3 mb-2">
                      <h3 className="text-base font-medium text-text">{faction.name}</h3>
                      <StatusBadge variant={factionTypeVariant[faction.type]}>
                        {factionTypeLabels[faction.type]}
                      </StatusBadge>
                    </div>
                    <p className="text-sm text-text-sub mb-4">{faction.description}</p>
                    <div className="space-y-2 text-sm">
                      <div className="flex gap-2">
                        <Swords className="w-4 h-4 text-text-muted shrink-0 mt-0.5" />
                        <div>
                          <span className="text-text-muted">目标：</span>
                          <span className="text-text-sub">{faction.goals}</span>
                        </div>
                      </div>
                      <div className="flex gap-2">
                        <HelpCircle className="w-4 h-4 text-text-muted shrink-0 mt-0.5" />
                        <div>
                          <span className="text-text-muted">冲突：</span>
                          <span className="text-text-sub">{faction.conflicts}</span>
                        </div>
                      </div>
                    </div>
                    {members.length > 0 && (
                      <div className="mt-4 pt-3 border-t border-border">
                        <p className="text-xs text-text-muted mb-2">关联角色</p>
                        <div className="flex flex-wrap gap-2">
                          {members.map((m) => (
                            <span
                              key={m.id}
                              className="text-xs px-2 py-0.5 rounded bg-primary-subtle text-primary border border-primary/10"
                            >
                              {m.name}
                            </span>
                          ))}
                        </div>
                      </div>
                    )}
                  </Card>
                );
              })}
            </div>
          )}
        </div>
      )}

      <Modal
        open={!!selectedCharacter}
        onClose={() => setSelectedCharacter(null)}
        title={selectedCharacter?.name ?? '角色详情'}
      >
        {selectedCharacter && (
          <div className="space-y-5">
            <div className="flex items-center gap-3">
              <StatusBadge variant={roleBadgeVariant[selectedCharacter.role]}>
                {roleLabels[selectedCharacter.role]}
              </StatusBadge>
              {selectedCharacter.factionId && (
                <StatusBadge variant="default">
                  {factionMap.get(selectedCharacter.factionId)?.name ?? '未知势力'}
                </StatusBadge>
              )}
            </div>

            <div>
              <h4 className="text-xs text-text-muted mb-1.5 flex items-center gap-1.5">
                <Eye className="w-3.5 h-3.5" /> 外貌
              </h4>
              <p className="text-sm text-text-sub leading-relaxed">{selectedCharacter.appearance || '尚未记录外貌'}</p>
            </div>

            <div>
              <h4 className="text-xs text-text-muted mb-1.5 flex items-center gap-1.5">
                <Swords className="w-3.5 h-3.5" /> 动机
              </h4>
              <p className="text-sm text-text-sub leading-relaxed">{selectedCharacter.motivation}</p>
            </div>

            <div>
              <h4 className="text-xs text-text-muted mb-1.5">简介</h4>
              <p className="text-sm text-text-sub leading-relaxed">{selectedCharacter.summary}</p>
            </div>

            {([['恐惧', selectedCharacter.fear], ['秘密', selectedCharacter.secret], ['人物弧', selectedCharacter.arc]] as const).map(([label, value]) => <div key={label}><h4 className="text-xs text-text-muted mb-1.5">{label}</h4><p className="text-sm text-text-sub whitespace-pre-wrap">{value || '尚未记录'}</p></div>)}
            <p className="text-xs text-text-muted">此处为原始档案；按章节生效的新设定见页面的正式人物修订记录。</p>

            <div className="flex flex-wrap gap-2 pt-1">
              {selectedCharacter.tags.map((tag) => (
                <span
                  key={tag}
                  className="text-xs px-2 py-0.5 rounded bg-surface-hover text-text-sub border border-border"
                >
                  {tag}
                </span>
              ))}
            </div>

            <div className="flex justify-end pt-2">
              <Button size="sm" onClick={() => setSelectedCharacter(null)}>
                关闭
              </Button>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
}
