import {
  Archive,
  Building2,
  Link2,
  Plus,
  ShieldAlert,
  UserRound,
  UsersRound,
} from "lucide-react";
import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type FormEvent,
} from "react";
import type {
  AppearanceRole,
  ChapterView,
  CharacterInput,
  CharacterView,
  FactionInput,
  FactionView,
  MembershipStatus,
  RelationshipInput,
  RelationshipKind,
  RelationshipView,
  StoryStructureView,
  StoryWorldView,
  StudioClient,
} from "../../lib/studio-client";

type Props = { client: StudioClient; chapters: ChapterView[] };
type DialogKind = "character" | "faction" | "relationship" | null;
type ArchiveTarget = {
  kind: "character" | "faction" | "relationship";
  id: string;
  name: string;
};

const emptyWorld: StoryWorldView = {
  characters: [],
  factions: [],
  memberships: [],
  relationships: [],
  chapterAppearances: [],
  sceneAppearances: [],
  consistencyIssues: [],
};
const emptyStructure: StoryStructureView = {
  outlineNodes: [],
  storyLines: [],
  storyLineLinks: [],
  sceneCards: [],
};
const emptyCharacter: CharacterInput = {
  name: "",
  aliases: [],
  role: "supporting",
  summary: "",
  motivation: "",
  fear: "",
  secret: "",
  arc: "",
  status: "active",
};
const emptyFaction: FactionInput = {
  name: "",
  summary: "",
  goal: "",
  status: "active",
};
const emptyRelationship: RelationshipInput = {
  sourceCharacterId: "",
  targetCharacterId: "",
  kind: "ally",
  label: "",
  strength: 50,
  publicState: "",
  trueState: "",
};

const roleNames: Record<CharacterView["role"], string> = {
  protagonist: "主角",
  supporting: "重要配角",
  antagonist: "对立角色",
  minor: "次要角色",
};
const characterStatusNames: Record<CharacterView["status"], string> = {
  active: "活跃",
  inactive: "暂离",
  deceased: "已故",
};
const factionStatusNames: Record<FactionView["status"], string> = {
  active: "活跃",
  inactive: "沉寂",
  disbanded: "解散",
};
const relationNames: Record<RelationshipKind, string> = {
  ally: "盟友",
  family: "亲属",
  romantic: "情感",
  rival: "竞争",
  enemy: "敌对",
  mentor: "师承",
  other: "其他",
};
const appearanceNames: Record<AppearanceRole, string> = {
  pov: "视角",
  present: "出场",
  mentioned: "提及",
};

function messageOf(cause: unknown, fallback: string) {
  return cause instanceof Error ? cause.message : fallback;
}

function topologyPosition(index: number, count: number) {
  if (count <= 1) return { x: 50, y: 50 };
  const angle = -Math.PI / 2 + (index / count) * Math.PI * 2;
  return { x: 50 + Math.cos(angle) * 34, y: 50 + Math.sin(angle) * 34 };
}

function RelationshipTopology({
  characters,
  relationships,
  selectedCharacterId,
  onSelect,
}: {
  characters: CharacterView[];
  relationships: RelationshipView[];
  selectedCharacterId: string;
  onSelect: (id: string) => void;
}) {
  const positions = new Map(
    characters.map((item, index) => [item.id, topologyPosition(index, characters.length)]),
  );
  return (
    <section className="relationship-topology" aria-label="人物关系拓扑">
      <svg viewBox="0 0 100 100" aria-hidden="true" preserveAspectRatio="none">
        <defs><marker id="relationship-arrow" markerWidth="6" markerHeight="6" refX="5" refY="3" orient="auto"><path d="M0,0 L0,6 L6,3 z" /></marker></defs>
        {relationships.map((item) => {
          const source = positions.get(item.sourceCharacterId);
          const target = positions.get(item.targetCharacterId);
          if (!source || !target) return null;
          return <line key={item.id} x1={source.x} y1={source.y} x2={target.x} y2={target.y} markerEnd="url(#relationship-arrow)" />;
        })}
      </svg>
      {characters.map((item) => {
        const position = positions.get(item.id)!;
        return <button key={item.id} type="button" aria-label={`查看角色：${item.name}`} className={item.id === selectedCharacterId ? "is-selected" : ""} style={{ left: `${position.x}%`, top: `${position.y}%` }} onClick={() => onSelect(item.id)}><span>{item.name.slice(0, 1)}</span><small>{item.name}</small></button>;
      })}
    </section>
  );
}

export function StoryWorldWorkbench({ client, chapters }: Props) {
  const [world, setWorld] = useState(emptyWorld);
  const [structure, setStructure] = useState(emptyStructure);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedCharacterId, setSelectedCharacterId] = useState("");
  const [dialog, setDialog] = useState<DialogKind>(null);
  const [archiveTarget, setArchiveTarget] = useState<ArchiveTarget | null>(null);
  const [editingId, setEditingId] = useState("");
  const [character, setCharacter] = useState(emptyCharacter);
  const [aliases, setAliases] = useState("");
  const [faction, setFaction] = useState(emptyFaction);
  const [relationship, setRelationship] = useState(emptyRelationship);
  const [membershipFactionId, setMembershipFactionId] = useState("");
  const [membershipRole, setMembershipRole] = useState("");
  const [membershipStatus, setMembershipStatus] =
    useState<MembershipStatus>("active");
  const [chapterId, setChapterId] = useState("");
  const [chapterRole, setChapterRole] = useState<AppearanceRole>("present");
  const [sceneId, setSceneId] = useState("");
  const [sceneRole, setSceneRole] = useState<AppearanceRole>("present");

  const load = useCallback(async () => {
    try {
      const [nextWorld, nextStructure] = await Promise.all([
        client.storyWorld(),
        client.storyStructure(),
      ]);
      setWorld(nextWorld);
      setStructure(nextStructure);
      setSelectedCharacterId((current) =>
        nextWorld.characters.some((item) => item.id === current)
          ? current
          : (nextWorld.characters[0]?.id ?? ""),
      );
      setError(null);
    } catch (cause) {
      setError(messageOf(cause, "无法读取人物与势力资料"));
    } finally {
      setLoading(false);
    }
  }, [client]);

  useEffect(() => {
    void load();
  }, [load]);

  const selectedCharacter = world.characters.find(
    (item) => item.id === selectedCharacterId,
  );
  const characterById = useMemo(
    () => new Map(world.characters.map((item) => [item.id, item])),
    [world.characters],
  );
  const factionById = useMemo(
    () => new Map(world.factions.map((item) => [item.id, item])),
    [world.factions],
  );
  const chapterById = useMemo(
    () => new Map(chapters.map((item) => [item.id, item])),
    [chapters],
  );
  const sceneById = useMemo(
    () => new Map(structure.sceneCards.map((item) => [item.id, item])),
    [structure.sceneCards],
  );

  const closeDialog = () => {
    setDialog(null);
    setEditingId("");
  };
  const openNewCharacter = () => {
    setCharacter(emptyCharacter);
    setAliases("");
    setEditingId("");
    setDialog("character");
  };
  const openEditCharacter = (item: CharacterView) => {
    const { id, ...input } = item;
    setCharacter(input);
    setAliases(item.aliases.join("、"));
    setEditingId(id);
    setDialog("character");
  };
  const openNewFaction = () => {
    setFaction(emptyFaction);
    setEditingId("");
    setDialog("faction");
  };
  const openEditFaction = (item: FactionView) => {
    const { id, ...input } = item;
    setFaction(input);
    setEditingId(id);
    setDialog("faction");
  };
  const openNewRelationship = () => {
    setRelationship({
      ...emptyRelationship,
      sourceCharacterId: selectedCharacterId || world.characters[0]?.id || "",
      targetCharacterId:
        world.characters.find((item) => item.id !== selectedCharacterId)?.id ||
        "",
    });
    setDialog("relationship");
  };

  const mutate = async (
    operation: () => Promise<unknown>,
    fallback: string,
  ) => {
    try {
      await operation();
      setError(null);
      await load();
      return true;
    } catch (cause) {
      setError(messageOf(cause, fallback));
      return false;
    }
  };

  const saveCharacter = async (event: FormEvent) => {
    event.preventDefault();
    const input = {
      ...character,
      aliases: aliases
        .split(/[、,，]/)
        .map((item) => item.trim())
        .filter(Boolean),
    };
    const saved = await mutate(
      () =>
        editingId
          ? client.updateCharacter(editingId, input)
          : client.createCharacter(input),
      "无法保存角色",
    );
    if (saved) closeDialog();
  };

  const saveFaction = async (event: FormEvent) => {
    event.preventDefault();
    const saved = await mutate(
      () =>
        editingId
          ? client.updateFaction(editingId, faction)
          : client.createFaction(faction),
      "无法保存势力",
    );
    if (saved) closeDialog();
  };

  const saveRelationship = async (event: FormEvent) => {
    event.preventDefault();
    const saved = await mutate(
      () => client.createRelationship(relationship),
      "无法保存人物关系",
    );
    if (saved) closeDialog();
  };

  const confirmArchive = async () => {
    if (!archiveTarget) return;
    if (archiveTarget.kind === "character") {
      await mutate(
        () => client.archiveCharacter(archiveTarget.id),
        "无法归档角色",
      );
    } else if (archiveTarget.kind === "faction") {
      await mutate(
        () => client.archiveFaction(archiveTarget.id),
        "无法归档势力",
      );
    } else {
      await mutate(
        () => client.archiveRelationship(archiveTarget.id),
        "无法归档人物关系",
      );
    }
    setArchiveTarget(null);
  };

  const saveMembership = async () => {
    const role = membershipRole.trim();
    if (!selectedCharacter || !membershipFactionId || !role) return;
    const saved = await mutate(
      () =>
        client.upsertMembership({
          characterId: selectedCharacter.id,
          factionId: membershipFactionId,
          role,
          status: membershipStatus,
        }),
      "无法保存势力归属",
    );
    if (saved) {
      setMembershipFactionId("");
      setMembershipRole("");
      setMembershipStatus("active");
    }
  };
  const linkChapter = async () => {
    if (!selectedCharacter || !chapterId) return;
    const saved = await mutate(
      () =>
        client.linkCharacterToChapter({
          characterId: selectedCharacter.id,
          targetId: chapterId,
          role: chapterRole,
        }),
      "无法关联章节",
    );
    if (saved) setChapterId("");
  };
  const linkScene = async () => {
    if (!selectedCharacter || !sceneId) return;
    const saved = await mutate(
      () =>
        client.linkCharacterToScene({
          characterId: selectedCharacter.id,
          targetId: sceneId,
          role: sceneRole,
        }),
      "无法关联场景",
    );
    if (saved) setSceneId("");
  };

  return (
    <section className="world-workbench">
      <header className="world-header">
        <div>
          <p className="eyebrow">Story World</p>
          <h2>人物与势力</h2>
          <p>维护人物动机、组织归属和可追踪的关系网络。</p>
        </div>
        <div className="world-actions">
          <button type="button" onClick={openNewCharacter}>
            <Plus size={16} />
            新建角色
          </button>
          <button type="button" onClick={openNewFaction}>
            <Building2 size={16} />
            新建势力
          </button>
          <button
            type="button"
            disabled={world.characters.length < 2}
            onClick={openNewRelationship}
          >
            <Link2 size={16} />
            新建关系
          </button>
        </div>
      </header>

      {error && (
        <div className="story-error" role="alert">
          {error}
        </div>
      )}
      {loading ? (
        <p className="story-loading">正在读取人物资料…</p>
      ) : (
        <>
          {world.consistencyIssues.length > 0 && (
            <section className="consistency-strip" aria-label="一致性提醒">
              <ShieldAlert size={18} />
              <div>
                {world.consistencyIssues.map((issue) => (
                  <p key={`${issue.code}-${issue.entityIds.join("-")}`}>
                    {issue.message}
                  </p>
                ))}
              </div>
            </section>
          )}

          <div className="world-grid">
            <article className="world-panel world-panel--characters">
              <h3>
                <UsersRound size={18} />
                角色档案
              </h3>
              {world.characters.length === 0 ? (
                <p className="story-empty">
                  建立第一个角色，开始组织人物网络。
                </p>
              ) : (
                <div className="character-list">
                  {world.characters.map((item) => (
                    <button
                      type="button"
                      key={item.id}
                      className={
                        item.id === selectedCharacterId ? "is-active" : ""
                      }
                      onClick={() => setSelectedCharacterId(item.id)}
                    >
                      <span className="character-avatar">
                        {item.name.slice(0, 1)}
                      </span>
                      <span>
                        <strong>{item.name}</strong>
                        <small>
                          {roleNames[item.role]} ·{" "}
                          {characterStatusNames[item.status]}
                        </small>
                      </span>
                    </button>
                  ))}
                </div>
              )}
              {selectedCharacter && (
                <section className="character-profile">
                  <div className="world-card-heading">
                    <div>
                      <p className="eyebrow">角色档案</p>
                      <h3>{selectedCharacter.name}</h3>
                    </div>
                    <div>
                      <button
                        type="button"
                        aria-label={`编辑：${selectedCharacter.name}`}
                        onClick={() => openEditCharacter(selectedCharacter)}
                      >
                        编辑
                      </button>
                      <button
                        type="button"
                        aria-label={`归档角色：${selectedCharacter.name}`}
                        onClick={() =>
                          setArchiveTarget({
                            kind: "character",
                            id: selectedCharacter.id,
                            name: selectedCharacter.name,
                          })
                        }
                      >
                        <Archive size={15} />
                      </button>
                    </div>
                  </div>
                  {selectedCharacter.aliases.length > 0 && (
                    <p className="world-muted">
                      别名：{selectedCharacter.aliases.join("、")}
                    </p>
                  )}
                  <p>{selectedCharacter.summary || "尚未填写角色概述"}</p>
                  <dl className="profile-facts">
                    <div>
                      <dt>动机</dt>
                      <dd>{selectedCharacter.motivation || "未设置"}</dd>
                    </div>
                    <div>
                      <dt>恐惧</dt>
                      <dd>{selectedCharacter.fear || "未设置"}</dd>
                    </div>
                    <div>
                      <dt>秘密</dt>
                      <dd>{selectedCharacter.secret || "未设置"}</dd>
                    </div>
                    <div>
                      <dt>人物弧</dt>
                      <dd>{selectedCharacter.arc || "未设置"}</dd>
                    </div>
                  </dl>
                </section>
              )}
            </article>

            <article className="world-panel world-panel--links">
              <h3>
                <UserRound size={18} />
                归属与出场
              </h3>
              {!selectedCharacter ? (
                <p className="story-empty">选择角色后管理其归属和出场位置。</p>
              ) : (
                <>
                  <section className="world-subsection">
                    <h4>势力归属</h4>
                    <div className="world-tags">
                      {world.memberships
                        .filter(
                          (item) => item.characterId === selectedCharacter.id,
                        )
                        .map((item) => (
                          <span key={item.factionId}>
                            {item.role || "成员"} ·{" "}
                            {factionById.get(item.factionId)?.name ??
                              "未知势力"}
                            <button
                              type="button"
                              aria-label={`移除归属：${item.role || "成员"}`}
                              onClick={() =>
                                void mutate(
                                  () =>
                                    client.deleteMembership(
                                      item.characterId,
                                      item.factionId,
                                    ),
                                  "无法移除势力归属",
                                )
                              }
                            >
                              ×
                            </button>
                          </span>
                        ))}
                    </div>
                    <div className="world-link-form">
                      <label>
                        加入势力
                        <select
                          aria-label="加入势力"
                          value={membershipFactionId}
                          onChange={(event) =>
                            setMembershipFactionId(event.target.value)
                          }
                        >
                          <option value="">选择势力</option>
                          {world.factions.map((item) => (
                            <option key={item.id} value={item.id}>
                              {item.name}
                            </option>
                          ))}
                        </select>
                      </label>
                      <label>
                        势力身份
                        <input
                          required
                          aria-label="势力身份"
                          value={membershipRole}
                          onChange={(event) =>
                            setMembershipRole(event.target.value)
                          }
                        />
                        {membershipFactionId && !membershipRole.trim() && (
                          <small className="world-field-hint">
                            请填写角色在该势力中的身份
                          </small>
                        )}
                      </label>
                      <label>
                        归属状态
                        <select
                          value={membershipStatus}
                          onChange={(event) =>
                            setMembershipStatus(
                              event.target.value as MembershipStatus,
                            )
                          }
                        >
                          <option value="active">当前</option>
                          <option value="former">曾经</option>
                          <option value="secret">秘密</option>
                        </select>
                      </label>
                      <button
                        type="button"
                        disabled={!membershipFactionId || !membershipRole.trim()}
                        onClick={() => void saveMembership()}
                      >
                        保存归属
                      </button>
                    </div>
                  </section>
                  <section className="world-subsection">
                    <h4>章节出场</h4>
                    <div className="world-tags">
                      {world.chapterAppearances
                        .filter(
                          (item) => item.characterId === selectedCharacter.id,
                        )
                        .map((item) => (
                          <span key={item.chapterId}>
                            {appearanceNames[item.role]} ·{" "}
                            {chapterById.get(item.chapterId)?.title ??
                              "未知章节"}
                            <button
                              type="button"
                              aria-label={`移除章节关联：${chapterById.get(item.chapterId)?.title ?? "未知章节"}`}
                              onClick={() =>
                                void mutate(
                                  () =>
                                    client.unlinkCharacterFromChapter(
                                      item.characterId,
                                      item.chapterId,
                                    ),
                                  "无法移除章节关联",
                                )
                              }
                            >
                              ×
                            </button>
                          </span>
                        ))}
                    </div>
                    <div className="world-link-form world-link-form--compact">
                      <label>
                        关联章节
                        <select
                          aria-label="关联章节"
                          value={chapterId}
                          onChange={(event) => setChapterId(event.target.value)}
                        >
                          <option value="">选择章节</option>
                          {chapters.map((item) => (
                            <option key={item.id} value={item.id}>
                              {item.title}
                            </option>
                          ))}
                        </select>
                      </label>
                      <label>
                        出场方式
                        <select
                          value={chapterRole}
                          onChange={(event) =>
                            setChapterRole(event.target.value as AppearanceRole)
                          }
                        >
                          {Object.entries(appearanceNames).map(
                            ([value, name]) => (
                              <option key={value} value={value}>
                                {name}
                              </option>
                            ),
                          )}
                        </select>
                      </label>
                      <button
                        type="button"
                        disabled={!chapterId}
                        onClick={() => void linkChapter()}
                      >
                        添加章节关联
                      </button>
                    </div>
                  </section>
                  <section className="world-subsection">
                    <h4>场景出场</h4>
                    <div className="world-tags">
                      {world.sceneAppearances
                        .filter(
                          (item) => item.characterId === selectedCharacter.id,
                        )
                        .map((item) => (
                          <span key={item.sceneCardId}>
                            {appearanceNames[item.role]} ·{" "}
                            {sceneById.get(item.sceneCardId)?.title ??
                              "未知场景"}
                            <button
                              type="button"
                              aria-label={`移除场景关联：${sceneById.get(item.sceneCardId)?.title ?? "未知场景"}`}
                              onClick={() =>
                                void mutate(
                                  () =>
                                    client.unlinkCharacterFromScene(
                                      item.characterId,
                                      item.sceneCardId,
                                    ),
                                  "无法移除场景关联",
                                )
                              }
                            >
                              ×
                            </button>
                          </span>
                        ))}
                    </div>
                    <div className="world-link-form world-link-form--compact">
                      <label>
                        关联场景
                        <select
                          aria-label="关联场景"
                          value={sceneId}
                          onChange={(event) => setSceneId(event.target.value)}
                        >
                          <option value="">选择场景</option>
                          {structure.sceneCards.map((item) => (
                            <option key={item.id} value={item.id}>
                              {item.title}
                            </option>
                          ))}
                        </select>
                      </label>
                      <label>
                        出场方式
                        <select
                          value={sceneRole}
                          onChange={(event) =>
                            setSceneRole(event.target.value as AppearanceRole)
                          }
                        >
                          {Object.entries(appearanceNames).map(
                            ([value, name]) => (
                              <option key={value} value={value}>
                                {name}
                              </option>
                            ),
                          )}
                        </select>
                      </label>
                      <button
                        type="button"
                        disabled={!sceneId}
                        onClick={() => void linkScene()}
                      >
                        添加场景关联
                      </button>
                    </div>
                  </section>
                </>
              )}
            </article>

            <article className="world-panel world-panel--network">
              <h3>
                <Link2 size={18} />
                关系网络
              </h3>
              {world.characters.length > 0 && (
                <RelationshipTopology
                  characters={world.characters}
                  relationships={world.relationships}
                  selectedCharacterId={selectedCharacterId}
                  onSelect={setSelectedCharacterId}
                />
              )}
              {world.relationships.length === 0 ? (
                <p className="story-empty">
                  用有方向的关系描述人物之间的真实连接。
                </p>
              ) : (
                world.relationships.map((item) => {
                  const source =
                    characterById.get(item.sourceCharacterId)?.name ??
                    "未知角色";
                  const target =
                    characterById.get(item.targetCharacterId)?.name ??
                    "未知角色";
                  return (
                    <div className="relationship-card" key={item.id}>
                      <div>
                        <strong>
                          {source} → {target}
                        </strong>
                        <span>
                          {relationNames[item.kind]} · 强度 {item.strength}
                        </span>
                      </div>
                      <p>{item.label || "未命名关系"}</p>
                      {(item.publicState || item.trueState) && (
                        <small>
                          {item.publicState || "表面未知"} /{" "}
                          {item.trueState || "真相未知"}
                        </small>
                      )}
                      <button
                        type="button"
                        aria-label={`归档关系：${source} → ${target}`}
                        onClick={() =>
                          setArchiveTarget({
                            kind: "relationship",
                            id: item.id,
                            name: `${source} → ${target}`,
                          })
                        }
                      >
                        <Archive size={14} />
                      </button>
                    </div>
                  );
                })
              )}
              <h3 className="faction-heading">
                <Building2 size={18} />
                势力
              </h3>
              {world.factions.length === 0 ? (
                <p className="story-empty">尚未建立势力。</p>
              ) : (
                world.factions.map((item) => (
                  <div className="faction-card" key={item.id}>
                    <div>
                      <strong>{item.name}</strong>
                      <span>{factionStatusNames[item.status]}</span>
                    </div>
                    <p>{item.summary || "尚未填写势力简介"}</p>
                    <small>目标：{item.goal || "未设置"}</small>
                    <div>
                      <button
                        type="button"
                        aria-label={`编辑势力：${item.name}`}
                        onClick={() => openEditFaction(item)}
                      >
                        编辑
                      </button>
                      <button
                        type="button"
                        aria-label={`归档势力：${item.name}`}
                        onClick={() =>
                          setArchiveTarget({
                            kind: "faction",
                            id: item.id,
                            name: item.name,
                          })
                        }
                      >
                        <Archive size={14} />
                      </button>
                    </div>
                  </div>
                ))
              )}
            </article>
          </div>
        </>
      )}

      {dialog === "character" && (
        <div className="dialog-backdrop">
          <form
            className="project-dialog world-dialog"
            role="dialog"
            aria-label={editingId ? "编辑角色" : "新建角色"}
            onSubmit={saveCharacter}
          >
            <p className="eyebrow">Character Profile</p>
            <h2>{editingId ? "编辑角色" : "新建角色"}</h2>
            <div className="dialog-form-grid">
              <label>
                角色姓名
                <input
                  required
                  aria-label="角色姓名"
                  value={character.name}
                  onChange={(event) =>
                    setCharacter({ ...character, name: event.target.value })
                  }
                />
              </label>
              <label>
                角色定位
                <select
                  aria-label="角色定位"
                  value={character.role}
                  onChange={(event) =>
                    setCharacter({
                      ...character,
                      role: event.target.value as CharacterInput["role"],
                    })
                  }
                >
                  <option value="protagonist">主角</option>
                  <option value="supporting">重要配角</option>
                  <option value="antagonist">对立角色</option>
                  <option value="minor">次要角色</option>
                </select>
              </label>
              <label>
                别名
                <input
                  value={aliases}
                  onChange={(event) => setAliases(event.target.value)}
                  placeholder="用逗号或顿号分隔"
                />
              </label>
              <label>
                状态
                <select
                  value={character.status}
                  onChange={(event) =>
                    setCharacter({
                      ...character,
                      status: event.target.value as CharacterInput["status"],
                    })
                  }
                >
                  <option value="active">活跃</option>
                  <option value="inactive">暂离</option>
                  <option value="deceased">已故</option>
                </select>
              </label>
            </div>
            <label>
              角色概述
              <textarea
                value={character.summary}
                onChange={(event) =>
                  setCharacter({ ...character, summary: event.target.value })
                }
              />
            </label>
            <div className="dialog-form-grid">
              <label>
                动机
                <input
                  aria-label="动机"
                  value={character.motivation}
                  onChange={(event) =>
                    setCharacter({
                      ...character,
                      motivation: event.target.value,
                    })
                  }
                />
              </label>
              <label>
                恐惧
                <input
                  value={character.fear}
                  onChange={(event) =>
                    setCharacter({ ...character, fear: event.target.value })
                  }
                />
              </label>
              <label>
                秘密
                <input
                  value={character.secret}
                  onChange={(event) =>
                    setCharacter({ ...character, secret: event.target.value })
                  }
                />
              </label>
              <label>
                人物弧
                <input
                  value={character.arc}
                  onChange={(event) =>
                    setCharacter({ ...character, arc: event.target.value })
                  }
                />
              </label>
            </div>
            <div className="dialog-actions">
              <button
                type="button"
                className="secondary-action"
                onClick={closeDialog}
              >
                取消
              </button>
              <button type="submit" className="primary-action">
                保存角色
              </button>
            </div>
          </form>
        </div>
      )}

      {dialog === "faction" && (
        <div className="dialog-backdrop">
          <form
            className="project-dialog world-dialog"
            role="dialog"
            aria-label={editingId ? "编辑势力" : "新建势力"}
            onSubmit={saveFaction}
          >
            <p className="eyebrow">Faction Profile</p>
            <h2>{editingId ? "编辑势力" : "新建势力"}</h2>
            <label>
              势力名称
              <input
                required
                aria-label="势力名称"
                value={faction.name}
                onChange={(event) =>
                  setFaction({ ...faction, name: event.target.value })
                }
              />
            </label>
            <label>
              势力简介
              <textarea
                value={faction.summary}
                onChange={(event) =>
                  setFaction({ ...faction, summary: event.target.value })
                }
              />
            </label>
            <label>
              势力目标
              <input
                aria-label="势力目标"
                value={faction.goal}
                onChange={(event) =>
                  setFaction({ ...faction, goal: event.target.value })
                }
              />
            </label>
            <label>
              状态
              <select
                value={faction.status}
                onChange={(event) =>
                  setFaction({
                    ...faction,
                    status: event.target.value as FactionInput["status"],
                  })
                }
              >
                <option value="active">活跃</option>
                <option value="inactive">沉寂</option>
                <option value="disbanded">解散</option>
              </select>
            </label>
            <div className="dialog-actions">
              <button
                type="button"
                className="secondary-action"
                onClick={closeDialog}
              >
                取消
              </button>
              <button type="submit" className="primary-action">
                保存势力
              </button>
            </div>
          </form>
        </div>
      )}

      {dialog === "relationship" && (
        <div className="dialog-backdrop">
          <form
            className="project-dialog world-dialog"
            role="dialog"
            aria-label="新建人物关系"
            onSubmit={saveRelationship}
          >
            <p className="eyebrow">Directed Relation</p>
            <h2>新建人物关系</h2>
            <div className="dialog-form-grid">
              <label>
                关系起点
                <select
                  required
                  aria-label="关系起点"
                  value={relationship.sourceCharacterId}
                  onChange={(event) =>
                    setRelationship({
                      ...relationship,
                      sourceCharacterId: event.target.value,
                    })
                  }
                >
                  <option value="">请选择</option>
                  {world.characters.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.name}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                关系终点
                <select
                  required
                  aria-label="关系终点"
                  value={relationship.targetCharacterId}
                  onChange={(event) =>
                    setRelationship({
                      ...relationship,
                      targetCharacterId: event.target.value,
                    })
                  }
                >
                  <option value="">请选择</option>
                  {world.characters.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.name}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                关系类型
                <select
                  aria-label="关系类型"
                  value={relationship.kind}
                  onChange={(event) =>
                    setRelationship({
                      ...relationship,
                      kind: event.target.value as RelationshipKind,
                    })
                  }
                >
                  {Object.entries(relationNames).map(([value, name]) => (
                    <option key={value} value={value}>
                      {name}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                关系名称
                <input
                  aria-label="关系名称"
                  value={relationship.label}
                  onChange={(event) =>
                    setRelationship({
                      ...relationship,
                      label: event.target.value,
                    })
                  }
                />
              </label>
              <label>
                关系强度
                <input
                  type="number"
                  min={-100}
                  max={100}
                  value={relationship.strength}
                  onChange={(event) =>
                    setRelationship({
                      ...relationship,
                      strength: Number(event.target.value),
                    })
                  }
                />
              </label>
            </div>
            <label>
              公开状态
              <input
                value={relationship.publicState}
                onChange={(event) =>
                  setRelationship({
                    ...relationship,
                    publicState: event.target.value,
                  })
                }
              />
            </label>
            <label>
              真实状态
              <input
                value={relationship.trueState}
                onChange={(event) =>
                  setRelationship({
                    ...relationship,
                    trueState: event.target.value,
                  })
                }
              />
            </label>
            <div className="dialog-actions">
              <button
                type="button"
                className="secondary-action"
                onClick={closeDialog}
              >
                取消
              </button>
              <button type="submit" className="primary-action">
                保存关系
              </button>
            </div>
          </form>
        </div>
      )}

      {archiveTarget && (
        <div className="dialog-backdrop">
          <section
            className="project-dialog"
            role="dialog"
            aria-label={
              archiveTarget.kind === "character"
                ? "归档角色"
                : archiveTarget.kind === "faction"
                  ? "归档势力"
                  : "归档人物关系"
            }
          >
            <p className="eyebrow">Archive Confirmation</p>
            <h2>确认归档“{archiveTarget.name}”？</h2>
            <p>
              {archiveTarget.kind === "character"
                ? "归档后不会在当前角色列表中显示。仍被势力、人物关系、章节或场景引用的角色无法归档。"
                : archiveTarget.kind === "faction"
                  ? "归档后不会在当前势力列表中显示。仍有角色归属的势力无法归档。"
                  : "归档后不会在当前关系网络中显示。"}
            </p>
            <div className="dialog-actions">
              <button
                type="button"
                className="secondary-action"
                onClick={() => setArchiveTarget(null)}
              >
                取消
              </button>
              <button
                type="button"
                className="danger-action"
                onClick={() => void confirmArchive()}
              >
                确认归档
              </button>
            </div>
          </section>
        </div>
      )}
    </section>
  );
}
