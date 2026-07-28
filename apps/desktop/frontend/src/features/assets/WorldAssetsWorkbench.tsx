import {
  Archive,
  Box,
  GitBranch,
  Landmark,
  Link2,
  Map as MapIcon,
  Pencil,
  Plus,
  ScrollText,
  Sparkles,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import type {
  ChapterView,
  ForeshadowingHookInput,
  ForeshadowingHookView,
  HolderType,
  ItemInput,
  ItemView,
  LocationInput,
  LocationTargetType,
  LocationView,
  RegionAdjacencyInput,
  StoryStructureView,
  StoryWorldView,
  StudioClient,
  WorldAssetsView,
  WorldRegionInput,
  WorldRegionView,
  WorldRuleInput,
  WorldRuleView,
} from "../../lib/studio-client";

type Props = { client: StudioClient; chapters: ChapterView[] };
type Tab = "geography" | "items" | "rules" | "hooks";
type DialogKind =
  | "region"
  | "region-adjacency"
  | "location"
  | "location-link"
  | "item"
  | "transfer"
  | "rule"
  | "hook"
  | null;
type ArchiveTarget = {
  kind: "region" | "location" | "item" | "rule" | "hook";
  id: string;
  name: string;
};

const emptyAssets: WorldAssetsView = {
  regions: [],
  regionAdjacencies: [],
  locations: [],
  locationLinks: [],
  items: [],
  itemTransfers: [],
  rules: [],
  hooks: [],
  consistencyIssues: [],
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
const emptyRegion: WorldRegionInput = {
  parentId: "",
  kind: "territory",
  name: "",
  direction: "center",
  summary: "",
  terrain: "",
  climate: "",
};
const emptyRegionAdjacency: RegionAdjacencyInput = {
  firstRegionId: "",
  secondRegionId: "",
  travelNote: "",
};
const emptyLocation: LocationInput = {
  regionId: "",
  kind: "city",
  name: "",
  summary: "",
  status: "active",
};
const emptyItem: ItemInput = { name: "", summary: "", status: "active" };
const emptyRule: WorldRuleInput = {
  name: "",
  description: "",
  scopeType: "project",
  scopeId: "",
  status: "active",
};
const emptyHook: ForeshadowingHookInput = {
  title: "",
  setup: "",
  plantedChapterId: "",
  dueChapterId: "",
  resolvedChapterId: "",
  status: "planted",
  notes: "",
};

const regionKinds: Array<[WorldRegionInput["kind"], string]> = [
  ["world", "世界"],
  ["continent", "大陆"],
  ["territory", "区域"],
  ["settlement", "聚居地"],
  ["terrain", "地貌"],
  ["secret_realm", "秘境"],
  ["resource", "资源区"],
];
const directionNames: Array<[WorldRegionInput["direction"], string]> = [
  ["center", "中心"],
  ["north", "北"],
  ["north_east", "东北"],
  ["east", "东"],
  ["south_east", "东南"],
  ["south", "南"],
  ["south_west", "西南"],
  ["west", "西"],
  ["north_west", "西北"],
  ["surrounding", "环绕"],
];
const locationKinds: Array<[LocationInput["kind"], string]> = [
  ["city", "城市"],
  ["settlement", "聚居地"],
  ["building", "建筑"],
  ["landmark", "地标"],
  ["wilderness", "荒野"],
  ["other", "其他"],
];
const hookStatusNames: Record<ForeshadowingHookView["status"], string> = {
  planted: "已埋设",
  progressing: "推进中",
  resolved: "已回收",
  abandoned: "已放弃",
};

function messageOf(cause: unknown, fallback: string) {
  return cause instanceof Error ? cause.message : fallback;
}

function Dialog({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="dialog-backdrop">
      <div className="project-dialog world-dialog asset-dialog" role="dialog" aria-label={label}>
        {children}
      </div>
    </div>
  );
}

export function WorldAssetsWorkbench({ client, chapters }: Props) {
  const [assets, setAssets] = useState(emptyAssets);
  const [world, setWorld] = useState(emptyWorld);
  const [structure, setStructure] = useState(emptyStructure);
  const [activeTab, setActiveTab] = useState<Tab>("geography");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [dialog, setDialog] = useState<DialogKind>(null);
  const [editingId, setEditingId] = useState("");
  const [archiveTarget, setArchiveTarget] = useState<ArchiveTarget | null>(null);
  const [region, setRegion] = useState(emptyRegion);
  const [selectedRegionId, setSelectedRegionId] = useState("");
  const [regionAdjacency, setRegionAdjacency] = useState(emptyRegionAdjacency);
  const [location, setLocation] = useState(emptyLocation);
  const [linkLocationId, setLinkLocationId] = useState("");
  const [linkType, setLinkType] = useState<LocationTargetType>("chapter");
  const [linkTargetId, setLinkTargetId] = useState("");
  const [linkRole, setLinkRole] = useState("");
  const [item, setItem] = useState(emptyItem);
  const [transferItemId, setTransferItemId] = useState("");
  const [holderType, setHolderType] = useState<HolderType>("character");
  const [holderId, setHolderId] = useState("");
  const [transferChapterId, setTransferChapterId] = useState("");
  const [transferNote, setTransferNote] = useState("");
  const [rule, setRule] = useState(emptyRule);
  const [hook, setHook] = useState(emptyHook);

  const load = useCallback(async () => {
    try {
      const [nextAssets, nextWorld, nextStructure] = await Promise.all([
        client.worldAssets(),
        client.storyWorld(),
        client.storyStructure(),
      ]);
      setAssets(nextAssets);
      setWorld(nextWorld);
      setStructure(nextStructure);
      setError(null);
    } catch (cause) {
      setError(messageOf(cause, "无法读取世界设定"));
    } finally {
      setLoading(false);
    }
  }, [client]);

  useEffect(() => {
    void load();
  }, [load]);

  const regionById = useMemo(
    () => new Map(assets.regions.map((value) => [value.id, value])),
    [assets.regions],
  );
  const regionsByParentId = useMemo(() => {
    const values = new Map<string, WorldRegionView[]>();
    for (const value of assets.regions) {
      const children = values.get(value.parentId) ?? [];
      children.push(value);
      values.set(value.parentId, children);
    }
    for (const children of values.values()) {
      children.sort((left, right) => left.name.localeCompare(right.name, "zh-Hans-CN"));
    }
    return values;
  }, [assets.regions]);
  const locationById = useMemo(
    () => new Map(assets.locations.map((value) => [value.id, value])),
    [assets.locations],
  );
  const chapterById = useMemo(
    () => new Map(chapters.map((value) => [value.id, value])),
    [chapters],
  );

  useEffect(() => {
    if (selectedRegionId && regionById.has(selectedRegionId)) return;
    setSelectedRegionId(assets.regions.find((value) => !value.parentId)?.id ?? assets.regions[0]?.id ?? "");
  }, [assets.regions, regionById, selectedRegionId]);

  const mutate = async (operation: () => Promise<unknown>, fallback: string) => {
    setSaving(true);
    try {
      await operation();
      await load();
      setError(null);
      return true;
    } catch (cause) {
      setError(messageOf(cause, fallback));
      return false;
    } finally {
      setSaving(false);
    }
  };

  const closeDialog = () => {
    setDialog(null);
    setEditingId("");
  };

  const openRegion = (value?: WorldRegionView) => {
    if (value) {
      const { id, ...input } = value;
      setEditingId(id);
      setRegion(input);
    } else {
      setEditingId("");
      setRegion({ ...emptyRegion, kind: assets.regions.length ? "territory" : "world" });
    }
    setDialog("region");
  };

  const openRegionAdjacency = () => {
    const other = assets.regions.find((value) => value.id !== selectedRegionId);
    setRegionAdjacency({
      firstRegionId: selectedRegionId,
      secondRegionId: other?.id ?? "",
      travelNote: "",
    });
    setDialog("region-adjacency");
  };

  const openLocation = (value?: LocationView) => {
    if (value) {
      const { id, ...input } = value;
      setEditingId(id);
      setLocation(input);
    } else {
      setEditingId("");
      setLocation({ ...emptyLocation, regionId: assets.regions[0]?.id ?? "" });
    }
    setDialog("location");
  };

  const openItem = (value?: ItemView) => {
    setEditingId(value?.id ?? "");
    setItem(value ? { name: value.name, summary: value.summary, status: value.status } : emptyItem);
    setDialog("item");
  };

  const openRule = (value?: WorldRuleView) => {
    if (value) {
      const { id, ...input } = value;
      setEditingId(id);
      setRule(input);
    } else {
      setEditingId("");
      setRule(emptyRule);
    }
    setDialog("rule");
  };

  const openHook = (value?: ForeshadowingHookView) => {
    if (value) {
      const { id, ...input } = value;
      setEditingId(id);
      setHook(input);
    } else {
      setEditingId("");
      setHook(emptyHook);
    }
    setDialog("hook");
  };

  const saveRegion = async (event: FormEvent) => {
    event.preventDefault();
    const ok = await mutate(
      () =>
        editingId
          ? client.updateWorldRegion(editingId, region)
          : client.createWorldRegion(region),
      "无法保存区域",
    );
    if (ok) closeDialog();
  };

  const saveRegionAdjacency = async (event: FormEvent) => {
    event.preventDefault();
    const ok = await mutate(
      () => client.upsertRegionAdjacency(regionAdjacency),
      "无法保存区域相邻关系",
    );
    if (ok) closeDialog();
  };

  const saveLocation = async (event: FormEvent) => {
    event.preventDefault();
    const ok = await mutate(
      () =>
        editingId
          ? client.updateLocation(editingId, location)
          : client.createLocation(location),
      "无法保存地点",
    );
    if (ok) closeDialog();
  };

  const saveLocationLink = async (event: FormEvent) => {
    event.preventDefault();
    const ok = await mutate(
      () =>
        client.linkLocation({
          locationId: linkLocationId,
          targetType: linkType,
          targetId: linkTargetId,
          role: linkRole,
        }),
      "无法保存地点关联",
    );
    if (ok) closeDialog();
  };

  const saveItem = async (event: FormEvent) => {
    event.preventDefault();
    const ok = await mutate(
      () => (editingId ? client.updateItem(editingId, item) : client.createItem(item)),
      "无法保存物品",
    );
    if (ok) closeDialog();
  };

  const saveTransfer = async (event: FormEvent) => {
    event.preventDefault();
    const ok = await mutate(
      () =>
        client.transferItem({
          itemId: transferItemId,
          holderType,
          holderId,
          chapterId: transferChapterId,
          note: transferNote,
        }),
      "无法记录物品流转",
    );
    if (ok) closeDialog();
  };

  const saveRule = async (event: FormEvent) => {
    event.preventDefault();
    const ok = await mutate(
      () => (editingId ? client.updateWorldRule(editingId, rule) : client.createWorldRule(rule)),
      "无法保存世界规则",
    );
    if (ok) closeDialog();
  };

  const saveHook = async (event: FormEvent) => {
    event.preventDefault();
    const ok = await mutate(
      () =>
        editingId
          ? client.updateForeshadowingHook(editingId, hook)
          : client.createForeshadowingHook(hook),
      "无法保存伏笔",
    );
    if (ok) closeDialog();
  };

  const archive = async () => {
    if (!archiveTarget) return;
    const operations = {
      region: () => client.archiveWorldRegion(archiveTarget.id),
      location: () => client.archiveLocation(archiveTarget.id),
      item: () => client.archiveItem(archiveTarget.id),
      rule: () => client.archiveWorldRule(archiveTarget.id),
      hook: () => client.archiveForeshadowingHook(archiveTarget.id),
    };
    const ok = await mutate(operations[archiveTarget.kind], "无法删除故事资料");
    if (ok) setArchiveTarget(null);
  };

  const targetOptions = useMemo(() => {
    if (linkType === "chapter") return chapters.map((value) => [value.id, value.title]);
    if (linkType === "scene") return structure.sceneCards.map((value) => [value.id, value.title]);
    if (linkType === "character") return world.characters.map((value) => [value.id, value.name]);
    return world.factions.map((value) => [value.id, value.name]);
  }, [chapters, linkType, structure.sceneCards, world.characters, world.factions]);

  const holderOptions = useMemo(() => {
    if (holderType === "character") return world.characters.map((value) => [value.id, value.name]);
    if (holderType === "faction") return world.factions.map((value) => [value.id, value.name]);
    return assets.locations.map((value) => [value.id, value.name]);
  }, [assets.locations, holderType, world.characters, world.factions]);

  const holderName = (value: ItemView) => {
    if (!value.holderType) return "尚无持有者";
    if (value.holderType === "character")
      return world.characters.find((item) => item.id === value.holderId)?.name ?? "未知角色";
    if (value.holderType === "faction")
      return world.factions.find((item) => item.id === value.holderId)?.name ?? "未知势力";
    return locationById.get(value.holderId)?.name ?? "未知地点";
  };

  const selectedRegion = regionById.get(selectedRegionId);
  const selectedAdjacencies = assets.regionAdjacencies.filter(
    (value) =>
      value.firstRegionId === selectedRegionId ||
      value.secondRegionId === selectedRegionId,
  );
  const adjacencyRegionName = (value: { firstRegionId: string; secondRegionId: string }) => {
    const otherID = value.firstRegionId === selectedRegionId ? value.secondRegionId : value.firstRegionId;
    return regionById.get(otherID)?.name ?? "已删除区域";
  };

  const renderRegionTree = (parentId: string, seen = new Set<string>()): React.ReactNode => {
    const children = regionsByParentId.get(parentId) ?? [];
    if (!children.length) return null;
    return (
      <ul className="region-tree">
        {children.map((value) => {
          if (seen.has(value.id)) return null;
          const nextSeen = new Set(seen);
          nextSeen.add(value.id);
          return (
            <li key={value.id}>
              <button
                type="button"
                aria-label={`选择${value.name}`}
                className={selectedRegionId === value.id ? "is-selected" : ""}
                onClick={() => setSelectedRegionId(value.id)}
              >
                <span>{directionNames.find(([direction]) => direction === value.direction)?.[1]}</span>
                {value.name}
              </button>
              {renderRegionTree(value.id, nextSeen)}
            </li>
          );
        })}
      </ul>
    );
  };

  if (loading) return <div className="world-workbench world-muted">正在读取世界设定…</div>;

  return (
    <div className="world-workbench asset-workbench">
      <header className="world-header">
        <div>
          <p className="eyebrow">STORY BIBLE</p>
          <h2>世界设定</h2>
          <p>用结构化地理、物品、规则和伏笔保持长篇叙事一致。</p>
        </div>
      </header>

      {error && <div className="workspace-alert" role="alert"><span>{error}</span><button type="button" onClick={() => setError(null)}>关闭</button></div>}
      {assets.consistencyIssues.length > 0 && (
        <div className="asset-consistency" role="alert">
          <Sparkles size={18} aria-hidden="true" />
          <div>{assets.consistencyIssues.map((issue) => <p key={`${issue.code}-${issue.entityIds.join("-")}`}>{issue.message}</p>)}</div>
        </div>
      )}

      <nav className="asset-tabs" aria-label="世界设定分类">
        {([
          ["geography", "地理", MapIcon],
          ["items", "物品", Box],
          ["rules", "世界规则", ScrollText],
          ["hooks", "伏笔", GitBranch],
        ] as const).map(([key, label, Icon]) => (
          <button key={key} type="button" className={activeTab === key ? "is-active" : ""} onClick={() => setActiveTab(key)}>
            <Icon size={17} />{label}
          </button>
        ))}
      </nav>

      {activeTab === "geography" && (
        <section className="asset-section">
          <div className="asset-section__heading">
            <div><h3>地理层级</h3><p>区域负责空间层级，地点负责具体场景及叙事关联。</p></div>
            <div className="world-actions">
              <button type="button" onClick={() => openRegion()}><Plus size={16} />新建区域</button>
              <button type="button" disabled={!assets.regions.length} onClick={() => openLocation()}><Landmark size={16} />新建地点</button>
            </div>
          </div>
          {!assets.regions.length ? <p className="world-muted">先建立世界或根区域。</p> : (
            <>
            <div className="geography-overview">
              <section className="region-map" aria-label="方位地图">
                <div className="region-map__heading"><div><p className="eyebrow">SPATIAL MAP</p><h4>方位地图</h4></div><span>{selectedRegion?.name || "未选择区域"}</span></div>
                <div className="region-map__grid">
                  {(regionsByParentId.get(selectedRegionId) ?? []).map((value) => (
                    <button key={value.id} type="button" className={`region-map__node region-map__node--${value.direction}`} onClick={() => setSelectedRegionId(value.id)}>
                      <small>{directionNames.find(([direction]) => direction === value.direction)?.[1]}</small>{value.name}
                    </button>
                  ))}
                  {!regionsByParentId.get(selectedRegionId)?.length && <p className="world-muted">此区域尚未建立下级区域。</p>}
                </div>
              </section>
              <section className="region-tree-panel">
                <h4>区域层级</h4>
                {renderRegionTree("")}
              </section>
              <section className="region-inspector" aria-label="区域空间关系">
                <div className="region-inspector__heading"><div><span>当前区域</span><h4>{selectedRegion?.name || "未选择"}</h4></div><button type="button" disabled={!selectedRegion || assets.regions.length < 2} onClick={openRegionAdjacency}><Link2 size={15} />新增相邻区域</button></div>
                <p>{selectedRegion?.summary || "选择区域后，可维护其相邻区域与通行说明。"}</p>
                <div className="region-adjacencies">
                  {selectedAdjacencies.map((value) => {
                    const otherName = adjacencyRegionName(value);
                    return <div key={`${value.firstRegionId}-${value.secondRegionId}`}><div><strong>{selectedRegion?.name} ↔ {otherName}</strong><span>{value.travelNote || "未填写通行说明"}</span></div><button type="button" aria-label={`解除${selectedRegion?.name}与${otherName}的相邻关系`} onClick={() => void mutate(() => client.deleteRegionAdjacency(value.firstRegionId, value.secondRegionId), "无法解除区域相邻关系")}>解除</button></div>;
                  })}
                  {!selectedAdjacencies.length && <span className="world-muted">尚未设置相邻区域。</span>}
                </div>
              </section>
            </div>
            <div className="asset-card-grid">
              {assets.regions.map((value) => {
                let depth = 0;
                let parent = value.parentId;
                while (parent && depth < 8) { depth += 1; parent = regionById.get(parent)?.parentId ?? ""; }
                return (
                  <article className="asset-card" key={value.id} style={{ marginLeft: `${Math.min(depth, 3) * 18}px` }}>
                    <div className="asset-card__heading">
                      <div><span>{regionKinds.find(([kind]) => kind === value.kind)?.[1]}</span><h4>{value.name}</h4></div>
                      <div>
                        <button type="button" aria-label={`编辑${value.name}`} onClick={() => openRegion(value)}><Pencil size={15} /></button>
                        <button type="button" aria-label={`删除${value.name}`} onClick={() => setArchiveTarget({ kind: "region", id: value.id, name: value.name })}><Archive size={15} /></button>
                      </div>
                    </div>
                    {value.summary && <p>{value.summary}</p>}
                    <div className="asset-meta"><span>{value.terrain || "未设地貌"}</span><span>{value.climate || "未设气候"}</span></div>
                    {assets.locations.filter((item) => item.regionId === value.id).map((place) => (
                      <div className="asset-location" key={place.id}>
                        <div><Landmark size={15} /><strong>{place.name}</strong><span>{place.status === "active" ? "启用" : place.status === "inactive" ? "停用" : "已毁"}</span></div>
                        <div>
                          <button type="button" aria-label={`关联${place.name}`} onClick={() => { setLinkLocationId(place.id); setLinkType("chapter"); setLinkTargetId(chapters[0]?.id ?? ""); setLinkRole(""); setDialog("location-link"); }}><Link2 size={14} /></button>
                          <button type="button" aria-label={`编辑${place.name}`} onClick={() => openLocation(place)}><Pencil size={14} /></button>
                          <button type="button" aria-label={`删除${place.name}`} onClick={() => setArchiveTarget({ kind: "location", id: place.id, name: place.name })}><Archive size={14} /></button>
                        </div>
                        {assets.locationLinks.filter((link) => link.locationId === place.id).map((link) => (
                          <button className="asset-link-chip" type="button" key={`${link.targetType}-${link.targetId}`} onClick={() => void mutate(() => client.unlinkLocation(link.locationId, link.targetType, link.targetId), "无法取消地点关联")}>
                            {link.role || "关联"} · {link.targetType} ×
                          </button>
                        ))}
                      </div>
                    ))}
                  </article>
                );
              })}
            </div>
            </>
          )}
        </section>
      )}

      {activeTab === "items" && (
        <section className="asset-section">
          <div className="asset-section__heading"><div><h3>物品与流转</h3><p>追踪关键物品在人物、势力和地点之间的去向。</p></div><button className="asset-primary" type="button" onClick={() => openItem()}><Plus size={16} />新建物品</button></div>
          <div className="asset-card-grid asset-card-grid--wide">
            {assets.items.map((value) => (
              <article className="asset-card" key={value.id}>
                <div className="asset-card__heading"><div><span>{value.status === "active" ? "有效" : value.status === "lost" ? "遗失" : "已毁"}</span><h4>{value.name}</h4></div><div>
                  <button type="button" aria-label={`编辑${value.name}`} onClick={() => openItem(value)}><Pencil size={15} /></button>
                  <button type="button" aria-label={`删除${value.name}`} onClick={() => setArchiveTarget({ kind: "item", id: value.id, name: value.name })}><Archive size={15} /></button>
                </div></div>
                <p>{value.summary || "暂无物品说明"}</p>
                <p className="asset-holder">当前持有：<strong>{holderName(value)}</strong></p>
                <button className="asset-inline-action" type="button" aria-label={`流转${value.name}`} onClick={() => { setTransferItemId(value.id); setHolderType("character"); setHolderId(world.characters[0]?.id ?? ""); setTransferChapterId(""); setTransferNote(""); setDialog("transfer"); }}>记录流转</button>
                {assets.itemTransfers.filter((transfer) => transfer.itemId === value.id).map((transfer) => (
                  <p className="asset-history" key={transfer.id}>{chapterById.get(transfer.chapterId)?.title || "未关联章节"} · {transfer.note || "持有者变更"}</p>
                ))}
              </article>
            ))}
          </div>
        </section>
      )}

      {activeTab === "rules" && (
        <section className="asset-section">
          <div className="asset-section__heading"><div><h3>世界规则</h3><p>定义全局或局部生效的力量、社会与自然约束。</p></div><button className="asset-primary" type="button" onClick={() => openRule()}><Plus size={16} />新建规则</button></div>
          <div className="asset-card-grid asset-card-grid--wide">{assets.rules.map((value) => (
            <article className="asset-card" key={value.id}>
              <div className="asset-card__heading"><div><span>{value.scopeType === "project" ? "全局" : value.scopeType === "region" ? regionById.get(value.scopeId)?.name : locationById.get(value.scopeId)?.name}</span><h4>{value.name}</h4></div><div>
                <button type="button" aria-label={`编辑${value.name}`} onClick={() => openRule(value)}><Pencil size={15} /></button>
                <button type="button" aria-label={`删除${value.name}`} onClick={() => setArchiveTarget({ kind: "rule", id: value.id, name: value.name })}><Archive size={15} /></button>
              </div></div><p>{value.description}</p><span className={`asset-status asset-status--${value.status}`}>{value.status === "active" ? "生效中" : "停用"}</span>
            </article>
          ))}</div>
        </section>
      )}

      {activeTab === "hooks" && (
        <section className="asset-section">
          <div className="asset-section__heading"><div><h3>伏笔追踪</h3><p>记录埋设、推进、计划回收与实际回收章节。</p></div><button className="asset-primary" type="button" onClick={() => openHook()}><Plus size={16} />新建伏笔</button></div>
          <div className="asset-card-grid asset-card-grid--wide">{assets.hooks.map((value) => (
            <article className="asset-card" key={value.id}>
              <div className="asset-card__heading"><div><span>{hookStatusNames[value.status]}</span><h4>{value.title}</h4></div><div>
                <button type="button" aria-label={`编辑${value.title}`} onClick={() => openHook(value)}><Pencil size={15} /></button>
                <button type="button" aria-label={`删除${value.title}`} onClick={() => setArchiveTarget({ kind: "hook", id: value.id, name: value.title })}><Archive size={15} /></button>
              </div></div><p>{value.setup}</p><div className="asset-meta"><span>埋设：{chapterById.get(value.plantedChapterId)?.title || "未指定"}</span><span>计划：{chapterById.get(value.dueChapterId)?.title || "未指定"}</span></div>
            </article>
          ))}</div>
        </section>
      )}

      {dialog === "region" && <Dialog label={editingId ? "编辑区域" : "新建区域"}><form onSubmit={saveRegion}><p className="eyebrow">地理层级</p><h2>{editingId ? "编辑区域" : "新建区域"}</h2><label>上级区域<select value={region.parentId} onChange={(event) => setRegion({ ...region, parentId: event.target.value })}><option value="">无（根区域）</option>{assets.regions.filter((value) => value.id !== editingId).map((value) => <option value={value.id} key={value.id}>{value.name}</option>)}</select></label><label>区域类型<select value={region.kind} onChange={(event) => setRegion({ ...region, kind: event.target.value as WorldRegionInput["kind"] })}>{regionKinds.map(([value, name]) => <option value={value} key={value}>{name}</option>)}</select></label><label>区域名称<input required value={region.name} onChange={(event) => setRegion({ ...region, name: event.target.value })} /></label><label>相对方位<select value={region.direction} onChange={(event) => setRegion({ ...region, direction: event.target.value as WorldRegionInput["direction"] })}>{directionNames.map(([value, name]) => <option value={value} key={value}>{name}</option>)}</select></label><label>概述<textarea value={region.summary} onChange={(event) => setRegion({ ...region, summary: event.target.value })} /></label><div className="dialog-field-grid"><label>地貌<input value={region.terrain} onChange={(event) => setRegion({ ...region, terrain: event.target.value })} /></label><label>气候<input value={region.climate} onChange={(event) => setRegion({ ...region, climate: event.target.value })} /></label></div><div className="dialog-actions"><button type="button" className="secondary-action" onClick={closeDialog}>取消</button><button type="submit" className="primary-action" disabled={saving}>保存区域</button></div></form></Dialog>}

      {dialog === "region-adjacency" && <Dialog label="新增区域相邻关系"><form onSubmit={saveRegionAdjacency}><p className="eyebrow">SPATIAL MAP</p><h2>新增区域相邻关系</h2><label>当前区域<input readOnly value={regionById.get(regionAdjacency.firstRegionId)?.name || ""} /></label><label>相邻区域<select required value={regionAdjacency.secondRegionId} onChange={(event) => setRegionAdjacency({ ...regionAdjacency, secondRegionId: event.target.value })}><option value="">请选择</option>{assets.regions.filter((value) => value.id !== regionAdjacency.firstRegionId).map((value) => <option value={value.id} key={value.id}>{value.name}</option>)}</select></label><label>通行说明<textarea value={regionAdjacency.travelNote} onChange={(event) => setRegionAdjacency({ ...regionAdjacency, travelNote: event.target.value })} placeholder="例如：沿海船路可直达，冬季封航" /></label><div className="dialog-actions"><button type="button" className="secondary-action" onClick={closeDialog}>取消</button><button type="submit" className="primary-action" disabled={saving || !regionAdjacency.secondRegionId}>保存相邻关系</button></div></form></Dialog>}

      {dialog === "location" && <Dialog label={editingId ? "编辑地点" : "新建地点"}><form onSubmit={saveLocation}><p className="eyebrow">具体地点</p><h2>{editingId ? "编辑地点" : "新建地点"}</h2><label>所属区域<select required value={location.regionId} onChange={(event) => setLocation({ ...location, regionId: event.target.value })}>{assets.regions.map((value) => <option value={value.id} key={value.id}>{value.name}</option>)}</select></label><label>地点类型<select value={location.kind} onChange={(event) => setLocation({ ...location, kind: event.target.value as LocationInput["kind"] })}>{locationKinds.map(([value, name]) => <option value={value} key={value}>{name}</option>)}</select></label><label>地点名称<input required value={location.name} onChange={(event) => setLocation({ ...location, name: event.target.value })} /></label><label>状态<select value={location.status} onChange={(event) => setLocation({ ...location, status: event.target.value as LocationInput["status"] })}><option value="active">启用</option><option value="inactive">停用</option><option value="destroyed">已毁</option></select></label><label>概述<textarea value={location.summary} onChange={(event) => setLocation({ ...location, summary: event.target.value })} /></label><div className="dialog-actions"><button type="button" className="secondary-action" onClick={closeDialog}>取消</button><button type="submit" className="primary-action" disabled={saving}>保存地点</button></div></form></Dialog>}

      {dialog === "location-link" && <Dialog label="关联地点"><form onSubmit={saveLocationLink}><p className="eyebrow">叙事关联</p><h2>关联地点</h2><label>关联类型<select value={linkType} onChange={(event) => { const next = event.target.value as LocationTargetType; setLinkType(next); setLinkTargetId(""); }}><option value="chapter">章节</option><option value="scene">场景</option><option value="character">角色</option><option value="faction">势力</option></select></label><label>关联对象<select required value={linkTargetId} onChange={(event) => setLinkTargetId(event.target.value)}><option value="">请选择</option>{targetOptions.map(([id, name]) => <option value={id} key={id}>{name}</option>)}</select></label><label>关联说明<input value={linkRole} onChange={(event) => setLinkRole(event.target.value)} placeholder="例如：主舞台、出生地" /></label><div className="dialog-actions"><button type="button" className="secondary-action" onClick={closeDialog}>取消</button><button type="submit" className="primary-action" disabled={saving}>保存关联</button></div></form></Dialog>}

      {dialog === "item" && <Dialog label={editingId ? "编辑物品" : "新建物品"}><form onSubmit={saveItem}><p className="eyebrow">关键物品</p><h2>{editingId ? "编辑物品" : "新建物品"}</h2><label>物品名称<input required value={item.name} onChange={(event) => setItem({ ...item, name: event.target.value })} /></label><label>状态<select value={item.status} onChange={(event) => setItem({ ...item, status: event.target.value as ItemInput["status"] })}><option value="active">有效</option><option value="lost">遗失</option><option value="destroyed">已毁</option></select></label><label>说明<textarea value={item.summary} onChange={(event) => setItem({ ...item, summary: event.target.value })} /></label><div className="dialog-actions"><button type="button" className="secondary-action" onClick={closeDialog}>取消</button><button type="submit" className="primary-action" disabled={saving}>保存物品</button></div></form></Dialog>}

      {dialog === "transfer" && <Dialog label="流转物品"><form onSubmit={saveTransfer}><p className="eyebrow">持有记录</p><h2>流转物品</h2><label>持有者类型<select value={holderType} onChange={(event) => { setHolderType(event.target.value as HolderType); setHolderId(""); }}><option value="character">角色</option><option value="faction">势力</option><option value="location">地点</option></select></label><label>新持有者<select required value={holderId} onChange={(event) => setHolderId(event.target.value)}><option value="">请选择</option>{holderOptions.map(([id, name]) => <option value={id} key={id}>{name}</option>)}</select></label><label>关联章节<select value={transferChapterId} onChange={(event) => setTransferChapterId(event.target.value)}><option value="">不关联章节</option>{chapters.map((value) => <option value={value.id} key={value.id}>{value.title}</option>)}</select></label><label>流转说明<textarea value={transferNote} onChange={(event) => setTransferNote(event.target.value)} /></label><div className="dialog-actions"><button type="button" className="secondary-action" onClick={closeDialog}>取消</button><button type="submit" className="primary-action" disabled={saving}>确认流转</button></div></form></Dialog>}

      {dialog === "rule" && <Dialog label={editingId ? "编辑规则" : "新建规则"}><form onSubmit={saveRule}><p className="eyebrow">约束定义</p><h2>{editingId ? "编辑规则" : "新建规则"}</h2><label>规则名称<input required value={rule.name} onChange={(event) => setRule({ ...rule, name: event.target.value })} /></label><label>生效范围<select value={rule.scopeType} onChange={(event) => setRule({ ...rule, scopeType: event.target.value as WorldRuleInput["scopeType"], scopeId: "" })}><option value="project">全局</option><option value="region">区域</option><option value="location">地点</option></select></label>{rule.scopeType !== "project" && <label>范围对象<select required value={rule.scopeId} onChange={(event) => setRule({ ...rule, scopeId: event.target.value })}><option value="">请选择</option>{(rule.scopeType === "region" ? assets.regions : assets.locations).map((value) => <option value={value.id} key={value.id}>{value.name}</option>)}</select></label>}<label>状态<select value={rule.status} onChange={(event) => setRule({ ...rule, status: event.target.value as WorldRuleInput["status"] })}><option value="active">生效</option><option value="inactive">停用</option></select></label><label>规则说明<textarea required value={rule.description} onChange={(event) => setRule({ ...rule, description: event.target.value })} /></label><div className="dialog-actions"><button type="button" className="secondary-action" onClick={closeDialog}>取消</button><button type="submit" className="primary-action" disabled={saving}>保存规则</button></div></form></Dialog>}

      {dialog === "hook" && <Dialog label={editingId ? "编辑伏笔" : "新建伏笔"}><form onSubmit={saveHook}><p className="eyebrow">伏笔追踪</p><h2>{editingId ? "编辑伏笔" : "新建伏笔"}</h2><label>伏笔标题<input required value={hook.title} onChange={(event) => setHook({ ...hook, title: event.target.value })} /></label><label>埋设内容<textarea required value={hook.setup} onChange={(event) => setHook({ ...hook, setup: event.target.value })} /></label><div className="dialog-field-grid"><label>埋设章节<select value={hook.plantedChapterId} onChange={(event) => setHook({ ...hook, plantedChapterId: event.target.value })}><option value="">未指定</option>{chapters.map((value) => <option value={value.id} key={value.id}>{value.title}</option>)}</select></label><label>计划回收章节<select value={hook.dueChapterId} onChange={(event) => setHook({ ...hook, dueChapterId: event.target.value })}><option value="">未指定</option>{chapters.map((value) => <option value={value.id} key={value.id}>{value.title}</option>)}</select></label></div><label>状态<select value={hook.status} onChange={(event) => setHook({ ...hook, status: event.target.value as ForeshadowingHookInput["status"] })}><option value="planted">已埋设</option><option value="progressing">推进中</option><option value="resolved">已回收</option><option value="abandoned">已放弃</option></select></label><label>实际回收章节<select value={hook.resolvedChapterId} onChange={(event) => setHook({ ...hook, resolvedChapterId: event.target.value })}><option value="">未回收</option>{chapters.map((value) => <option value={value.id} key={value.id}>{value.title}</option>)}</select></label><label>备注<textarea value={hook.notes} onChange={(event) => setHook({ ...hook, notes: event.target.value })} /></label><div className="dialog-actions"><button type="button" className="secondary-action" onClick={closeDialog}>取消</button><button type="submit" className="primary-action" disabled={saving}>保存伏笔</button></div></form></Dialog>}

      {archiveTarget && <Dialog label="确认删除"><p className="eyebrow">安全删除</p><h2>确认删除</h2><p>确定删除“{archiveTarget.name}”吗？如果仍被其他资料引用，系统会阻止删除并说明原因。</p><div className="dialog-actions"><button type="button" className="secondary-action" onClick={() => setArchiveTarget(null)}>取消</button><button type="button" className="danger-action" disabled={saving} onClick={() => void archive()}>确认删除</button></div></Dialog>}
    </div>
  );
}
