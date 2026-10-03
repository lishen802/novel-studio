package creation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
	storydomain "github.com/lishen802/novel-studio/internal/domain/story"
	storyassetdomain "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

type ArchivePreview struct {
	ConfirmationDigest string `json:"confirmationDigest"`
	Title              string `json:"title"`
	CharacterCount     int    `json:"characterCount"`
	OutlineCount       int    `json:"outlineCount"`
	AnchorCount        int    `json:"anchorCount"`
	// Operations 是本次将创建/更新的正式资产；Excluded 是被排除的审阅单位。
	// 预览同时是引用闭合检查：保留内容引用被排除单位时返回错误而不是空预览。
	Operations []ArchiveOperationView `json:"operations"`
	Excluded   []ReviewRefView        `json:"excluded"`
}

// ArchiveOperationView 描述一条将写入的正式资产操作。
type ArchiveOperationView struct {
	Kind    string `json:"kind"`
	Action  string `json:"action"`
	Summary string `json:"summary"`
}

type ArchiveRequest struct {
	ProjectID          string `json:"projectId"`
	ConfirmationDigest string `json:"confirmationDigest"`
	IdempotencyKey     string `json:"idempotencyKey"`
}

type ArchiveResult struct {
	Session     creationdomain.Session `json:"session"`
	ChangeSetID string                 `json:"changeSetId"`
}

type archivePlan struct {
	Digest     string
	Title      string
	Specs      []revisiondomain.PatchOperationSpec
	Confirmed  []creationdomain.Candidate
	Characters int
	Outlines   int
	Anchors    int
	Operations []ArchiveOperationView
	Excluded   []ReviewRefView
}

// archiveCandidateSnapshot 冻结候选及依赖处理状态；成功重放由会话回执判断，不重算摘要。
type archiveCandidateSnapshot struct {
	ID                     string                          `json:"id"`
	ContentSHA256          string                          `json:"contentSha256"`
	Quality                creationdomain.CandidateQuality `json:"quality"`
	Locks                  map[string]bool                 `json:"locks"`
	Status                 creationdomain.CandidateStatus  `json:"status"`
	UpstreamDigest         string                          `json:"upstreamDigest"`
	ExceptedUpstreamDigest string                          `json:"exceptedUpstreamDigest"`
	ImpactSeverity         creationdomain.ImpactSeverity   `json:"impactSeverity"`
	ImpactRefs             []string                        `json:"impactRefs"`
}

func snapshotArchiveCandidate(candidate creationdomain.Candidate) archiveCandidateSnapshot {
	return archiveCandidateSnapshot{
		ID: candidate.ID, ContentSHA256: candidate.ContentSHA256,
		Quality: candidate.QualityStatus, Locks: candidate.LockMap,
		Status: candidate.Status, UpstreamDigest: candidate.UpstreamDigest,
		ExceptedUpstreamDigest: candidate.ExceptedUpstreamDigest,
		ImpactSeverity:         candidate.ImpactSeverity, ImpactRefs: candidate.ImpactRefs,
	}
}

func archiveDigest(
	session creationdomain.Session,
	candidates []creationdomain.Candidate,
	project projectdomain.Project,
	selection archiveSelection,
) (string, error) {
	proposal, people, roadmap, err := requiredArchiveCandidates(session, candidates)
	if err != nil {
		return "", err
	}
	var direction creationdomain.Candidate
	if session.EntryKind() == creationdomain.EntryAuthorPlan {
		direction, _, err = validatedMaterial(session, candidates)
	} else {
		direction, err = requiredSelectedDirectionCandidate(session, candidates)
	}
	if err != nil {
		return "", err
	}
	// 审阅决定必须进入摘要：预览后改接受/排除或改版本同样使确认失效。
	exclusions := make([]string, 0, len(selection.Excluded))
	for id := range selection.Excluded {
		exclusions = append(exclusions, id)
	}
	sort.Strings(exclusions)
	payload, err := json.Marshal(struct {
		Project, Branch, Selected, SelectedCard    string
		ContentVersion                             int64
		ProjectTitle, ProjectGenre, ProjectSummary string
		Direction                                  archiveCandidateSnapshot
		Proposal, People, Roadmap                  archiveCandidateSnapshot
		Review                                     map[string]int64
		Excluded                                   []string
	}{
		Project:        session.ProjectID,
		Branch:         session.BranchID,
		Selected:       session.SelectedDirectionCandidateID,
		SelectedCard:   session.SelectedDirectionCardID,
		ContentVersion: session.ContentVersion,
		ProjectTitle:   project.Title,
		ProjectGenre:   project.Genre,
		ProjectSummary: project.Summary,
		Direction:      snapshotArchiveCandidate(direction),
		Proposal:       snapshotArchiveCandidate(proposal),
		People:         snapshotArchiveCandidate(people),
		Roadmap:        snapshotArchiveCandidate(roadmap),
		Review:         selection.ReviewVersions,
		Excluded:       exclusions,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func requiredArchiveCandidates(
	session creationdomain.Session,
	candidates []creationdomain.Candidate,
) (creationdomain.Candidate, creationdomain.Candidate, creationdomain.Candidate, error) {
	if session.EntryKind() != creationdomain.EntryAuthorPlan && strings.TrimSpace(session.SelectedDirectionCandidateID) == "" {
		return creationdomain.Candidate{}, creationdomain.Candidate{}, creationdomain.Candidate{}, creationdomain.ErrStageIncomplete
	}
	if _, ok := candidateByID(candidates, session.SelectedDirectionCandidateID); !ok && session.EntryKind() != creationdomain.EntryAuthorPlan {
		if _, found := currentCandidate(candidates, creationdomain.StageDirections); !found {
			return creationdomain.Candidate{}, creationdomain.Candidate{}, creationdomain.Candidate{}, creationdomain.ErrStageIncomplete
		}
	}
	proposal, ok := currentCandidate(candidates, creationdomain.StageProposal)
	if !ok {
		return creationdomain.Candidate{}, creationdomain.Candidate{}, creationdomain.Candidate{}, creationdomain.ErrStageIncomplete
	}
	people, ok := currentCandidate(candidates, creationdomain.StagePeopleWorld)
	if !ok {
		return creationdomain.Candidate{}, creationdomain.Candidate{}, creationdomain.Candidate{}, creationdomain.ErrStageIncomplete
	}
	roadmap, ok := currentCandidate(candidates, creationdomain.StageRoadmap)
	if !ok {
		return creationdomain.Candidate{}, creationdomain.Candidate{}, creationdomain.Candidate{}, creationdomain.ErrStageIncomplete
	}
	return proposal, people, roadmap, nil
}

// requiredSelectedDirectionCandidate 返回作者精确选中的有效方向候选，不跟随最新版本漂移。
func requiredSelectedDirectionCandidate(session creationdomain.Session, candidates []creationdomain.Candidate) (creationdomain.Candidate, error) {
	if strings.TrimSpace(session.SelectedDirectionCandidateID) == "" {
		return creationdomain.Candidate{}, creationdomain.ErrStageIncomplete
	}
	candidate, ok := candidateByID(candidates, session.SelectedDirectionCandidateID)
	if !ok {
		return creationdomain.Candidate{}, creationdomain.ErrStageIncomplete
	}
	if candidate.Status == creationdomain.StatusSuperseded || candidate.Status == creationdomain.StatusDiscarded {
		return creationdomain.Candidate{}, creationdomain.ErrStageIncomplete
	}
	return candidate, nil
}

// validateArchiveReadiness 校验确认前置条件：逐项审阅已完成、全链路候选质量未阻断、无未解决 stale、载荷有效。
func validateArchiveReadiness(
	session creationdomain.Session,
	candidates []creationdomain.Candidate,
	selection archiveSelection,
) error {
	if err := validateArchiveReview(selection); err != nil {
		return err
	}
	if session.EntryKind() == creationdomain.EntryAuthorPlan {
		if err := validateMaterialGapsForArchive(session, candidates); err != nil {
			return err
		}
	} else if _, err := validateSelectedDirection(session, candidates); err != nil {
		return err
	}
	for _, stage := range []creationdomain.Stage{
		creationdomain.StageProposal, creationdomain.StagePeopleWorld, creationdomain.StageRoadmap,
	} {
		current, ok := currentCandidate(candidates, stage)
		if !ok {
			return creationdomain.ErrStageIncomplete
		}
		if current.Blocked() {
			return creationdomain.ErrQualityBlocked
		}
		if err := validateCandidateDependency(session, candidates, current); err != nil {
			return err
		}
		if err := validateCandidateSchema(stage, current.PayloadJSON, current.PayloadSchemaVersion); err != nil {
			return err
		}
		if current.Status != creationdomain.StatusConfirmed {
			return creationdomain.ErrCandidateAcceptanceRequired
		}
	}
	return nil
}

// validateSelectedDirection 统一校验方向推进与入档所依赖的精确候选、卡、质量和输入版本。
func validateSelectedDirection(session creationdomain.Session, candidates []creationdomain.Candidate) (creationdomain.Candidate, error) {
	direction, err := requiredSelectedDirectionCandidate(session, candidates)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	if direction.Stage != creationdomain.StageDirections ||
		(direction.Kind != creationdomain.KindDirectionSet && direction.Kind != creationdomain.KindDirection) {
		return creationdomain.Candidate{}, creationdomain.ErrStageIncomplete
	}
	if _, err := directionCardByID(direction, session.SelectedDirectionCardID); err != nil {
		return creationdomain.Candidate{}, err
	}
	if direction.Blocked() {
		return creationdomain.Candidate{}, creationdomain.ErrQualityBlocked
	}
	if err := validateCandidateDependency(session, candidates, direction); err != nil {
		return creationdomain.Candidate{}, err
	}
	if direction.Kind == creationdomain.KindDirection {
		card, err := directionCardByID(direction, session.SelectedDirectionCardID)
		if err != nil || (agentdomain.StoryDirectionOutput{Direction: card}).Validate() != nil {
			return creationdomain.Candidate{}, creationdomain.ErrDirectionCard
		}
	} else if err := validateCandidatePayload(creationdomain.StageDirections, direction.PayloadJSON); err != nil {
		return creationdomain.Candidate{}, err
	}
	return direction, nil
}

// validateCandidateDependency 直接比较候选冻结输入与实时依赖，不依赖异步 stale 标记是否已传播。
func validateCandidateDependency(session creationdomain.Session, candidates []creationdomain.Candidate, candidate creationdomain.Candidate) error {
	live := liveUpstreamDigest(session, candidates, candidate.Stage)
	if live == "" || candidate.UpstreamDigest == "" || candidate.ImpactSeverity == creationdomain.ImpactMust {
		return creationdomain.ErrImpactUnresolved
	}
	excepted := candidate.ImpactSeverity == creationdomain.ImpactMaybe && !candidate.HasLocks() && candidate.ExceptionMatches(live)
	if (candidate.UpstreamDigest != live || candidate.Status == creationdomain.StatusStale) && !excepted {
		return creationdomain.ErrImpactUnresolved
	}
	return nil
}

func buildArchivePlan(
	session creationdomain.Session,
	candidates []creationdomain.Candidate,
	project projectdomain.Project,
	ids ports.IDGenerator,
	selection archiveSelection,
) (archivePlan, error) {
	proposal, people, roadmap, err := requiredArchiveCandidates(session, candidates)
	if err != nil {
		return archivePlan{}, err
	}
	digest, err := archiveDigest(session, candidates, project, selection)
	if err != nil {
		return archivePlan{}, err
	}
	blueprint, err := decodeBlueprint(proposal.PayloadJSON)
	if err != nil {
		return archivePlan{}, err
	}
	cast, err := decodeArchivableCast(people)
	if err != nil {
		return archivePlan{}, err
	}
	road, err := decodeArchivableRoadmap(roadmap)
	if err != nil {
		return archivePlan{}, err
	}
	// 被排除的实体不进入正式资产；保留内容仍引用它们时阻断，不静默丢弃。
	if cast, err = filterArchiveCast(cast, selection.Excluded); err != nil {
		return archivePlan{}, err
	}
	if road, err = filterArchiveRoadmap(road, selection.Excluded); err != nil {
		return archivePlan{}, err
	}
	if roadmap.PayloadSchemaVersion >= 3 {
		if err := road.ValidatePlanningReferences(cast); err != nil {
			return archivePlan{}, err
		}
	}
	mapper := archiveMapper{ids: ids, planningRefs: map[string]planningReference{}}
	if err := mapper.addProject(project, blueprint); err != nil {
		return archivePlan{}, err
	}
	if err := mapper.addProposal(proposal, blueprint); err != nil {
		return archivePlan{}, err
	}
	if err := mapper.addCast(people, cast); err != nil {
		return archivePlan{}, err
	}
	if err := mapper.addRoadmap(roadmap, road); err != nil {
		return archivePlan{}, err
	}
	nowConfirmed := make([]creationdomain.Candidate, 0, 3)
	for _, candidate := range []creationdomain.Candidate{proposal, people, roadmap} {
		nowConfirmed = append(nowConfirmed, candidate)
	}
	return archivePlan{
		Digest: digest, Title: blueprint.Title, Specs: mapper.specs, Confirmed: nowConfirmed,
		Characters: len(cast.Characters), Outlines: mapper.outlines, Anchors: mapper.anchors,
		Operations: archiveOperationViews(mapper.specs), Excluded: selection.ExcludedRefs,
	}, nil
}

type planningReference struct {
	Table string `json:"table"`
	ID    string `json:"id"`
}

type archiveMapper struct {
	planningRefs    map[string]planningReference
	ids             ports.IDGenerator
	specs           []revisiondomain.PatchOperationSpec
	outlines        int
	anchors         int
	chars           map[string]string
	proposalAssetID string
}

func (mapper *archiveMapper) add(kind revisiondomain.OperationKind, target, after, reason string) {
	mapper.specs = append(mapper.specs, revisiondomain.PatchOperationSpec{
		ID: mapper.ids.NewID(), Kind: kind, TargetID: target, AfterValue: after, Reason: reason,
	})
}

func (mapper *archiveMapper) addUpdate(kind revisiondomain.OperationKind, target, before, after, reason string) {
	mapper.specs = append(mapper.specs, revisiondomain.PatchOperationSpec{
		ID: mapper.ids.NewID(), Kind: kind, TargetID: target, BeforeValue: before, AfterValue: after, Reason: reason,
	})
}

func (mapper *archiveMapper) addProject(project projectdomain.Project, blueprint agentdomain.StoryBlueprintOutput) error {
	before, err := encodeJSON(map[string]string{"title": project.Title, "genre": project.Genre, "summary": project.Summary})
	if err != nil {
		return err
	}
	after, err := encodeJSON(map[string]string{
		"title": blueprint.Title, "genre": blueprint.Genre, "summary": blueprint.SellPoint,
	})
	if err != nil {
		return err
	}
	if before == after {
		return nil
	}
	mapper.addUpdate(revisiondomain.OperationUpdateProject, project.ID, before, after, "回写作品标题与定位")
	return nil
}

func (mapper *archiveMapper) addProposal(candidate creationdomain.Candidate, blueprint agentdomain.StoryBlueprintOutput) error {
	payload, err := withSourceLocks(candidate.PayloadJSON, candidate.LockMap)
	if err != nil {
		return err
	}
	after, err := encodeJSON(map[string]string{
		"kind": string(storyassetdomain.KindProposal), "origin": string(storyassetdomain.OriginModel),
		"sourceCandidateId": candidate.ID, "sourceRunId": candidate.RunID, "sourceArtifactId": candidate.ArtifactID,
		"payloadJson": payload,
	})
	if err != nil {
		return err
	}
	assetID := mapper.ids.NewID()
	mapper.add(revisiondomain.OperationCreateStoryAsset, assetID, after, "确认作品方案信封")
	mapper.proposalAssetID = assetID
	for _, statement := range blueprint.Forbidden {
		if err := mapper.addAnchor(candidate, "forbidden", "proposal", assetID, statement); err != nil {
			return err
		}
	}
	for _, anchor := range blueprint.Anchors {
		// 方案锚点只有明确指向方案时才能解析；其他提示缺少节点或章序区间，必须留待作者补齐。
		switch strings.TrimSpace(anchor.ScopeHint) {
		case "proposal":
			if err := mapper.addAnchor(candidate, anchor.Strength, "proposal", assetID, anchor.Statement); err != nil {
				return err
			}
		default:
			return creationdomain.ErrStageIncomplete
		}
	}
	return nil
}

// addAnchor 生成锚点操作：强度、作用域、语句与候选来源完整进入正式约束资产。
func (mapper *archiveMapper) addAnchor(candidate creationdomain.Candidate, strength, scope, scopeID, statement string) error {
	if strings.TrimSpace(scopeID) == "" {
		return creationdomain.ErrStageIncomplete
	}
	after, err := encodeJSON(map[string]string{
		"strength": strength, "scopeKind": scope, "scopeId": scopeID, "statement": statement,
		"origin":            string(storyassetdomain.OriginModel),
		"sourceCandidateId": candidate.ID, "sourceRunId": candidate.RunID, "sourceArtifactId": candidate.ArtifactID,
	})
	if err != nil {
		return err
	}
	mapper.anchors++
	mapper.add(revisiondomain.OperationCreateAnchor, mapper.ids.NewID(), after, "确认锚点与禁写项")
	return nil
}

func (mapper *archiveMapper) addCast(candidate creationdomain.Candidate, cast agentdomain.StoryCastOutput) error {
	mapper.chars = map[string]string{}
	for _, character := range cast.Characters {
		id := mapper.ids.NewID()
		mapper.chars[strings.TrimSpace(character.ID)] = id
		if mapper.planningRefs == nil {
			mapper.planningRefs = map[string]planningReference{}
		}
		mapper.planningRefs["character:"+strings.TrimSpace(character.ID)] = planningReference{Table: "characters", ID: id}
		after, err := encodeJSON(map[string]any{
			"name": character.Name, "aliases": character.Aliases, "role": character.Role,
			"summary": character.Summary, "motivation": character.Motivation, "fear": character.Fear,
			"secret": character.Secret, "arc": character.Arc,
		})
		if err != nil {
			return err
		}
		mapper.add(revisiondomain.OperationCreateCharacter, id, after, "确认人物")
		if err := mapper.addTypedAsset(candidate, storyassetdomain.KindCharacter, "characters", id, after); err != nil {
			return err
		}
	}
	for _, faction := range cast.Factions {
		id := mapper.ids.NewID()
		after, err := encodeJSON(map[string]string{"name": faction.Name, "summary": faction.Summary, "goal": faction.Goal})
		if err != nil {
			return err
		}
		mapper.add(revisiondomain.OperationCreateFaction, id, after, "确认势力")
		if err := mapper.addTypedAsset(candidate, storyassetdomain.KindFaction, "factions", id, after); err != nil {
			return err
		}
	}
	for _, location := range cast.Locations {
		id := mapper.ids.NewID()
		after, err := encodeJSON(map[string]string{"name": location.Name, "kind": location.Kind, "summary": location.Summary})
		if err != nil {
			return err
		}
		mapper.add(revisiondomain.OperationCreateLocation, id, after, "确认地点")
		if err := mapper.addTypedAsset(candidate, storyassetdomain.KindLocation, "locations", id, after); err != nil {
			return err
		}
	}
	for _, relation := range cast.Relationships {
		sourceID, sok := mapper.chars[strings.TrimSpace(relation.SourceCharacterRef)]
		targetID, tok := mapper.chars[strings.TrimSpace(relation.TargetCharacterRef)]
		if !sok || !tok {
			return creationdomain.ErrStageIncomplete
		}
		id := mapper.ids.NewID()
		mapper.planningRefs["relationship:"+strings.TrimSpace(relation.ID)] = planningReference{Table: "character_relationships", ID: id}
		after, err := encodeJSON(map[string]any{
			"sourceCharacterId": sourceID, "targetCharacterId": targetID,
			"kind": relation.Kind, "label": relation.Label, "strength": relation.Strength,
		})
		if err != nil {
			return err
		}
		mapper.add(revisiondomain.OperationCreateRelationship, id, after, "确认人物关系")
		if err := mapper.addTypedAsset(candidate, storyassetdomain.KindRelationship, "character_relationships", id, after); err != nil {
			return err
		}
	}
	for _, rule := range cast.WorldRules {
		id := mapper.ids.NewID()
		after, err := encodeJSON(map[string]string{
			"name": rule.Name, "description": rule.Description, "scopeType": rule.ScopeType,
		})
		if err != nil {
			return err
		}
		mapper.add(revisiondomain.OperationCreateWorldRule, id, after, "确认世界规则")
		if err := mapper.addTypedAsset(candidate, storyassetdomain.KindWorldRule, "world_rules", id, after); err != nil {
			return err
		}
	}
	// 人物世界禁写项必须完整映射为禁止锚点，不能留在候选 JSON 中丢失（P1-11）。
	for _, statement := range cast.Forbidden {
		if err := mapper.addAnchor(candidate, "forbidden", "proposal", mapper.proposalAssetID, statement); err != nil {
			return err
		}
	}
	return nil
}

func (mapper *archiveMapper) addTypedAsset(
	candidate creationdomain.Candidate, kind storyassetdomain.Kind, table, typedID, payloadJSON string,
) error {
	if kind == storyassetdomain.KindCharacter || kind == storyassetdomain.KindOutlineNode {
		var fields map[string]any
		if err := json.Unmarshal([]byte(payloadJSON), &fields); err != nil {
			return err
		}
		fields["_sourceLocks"] = candidate.LockMap
		encoded, err := encodeJSON(fields)
		if err != nil {
			return err
		}
		payloadJSON = encoded
	}
	after, err := encodeJSON(map[string]string{
		"kind": string(kind), "origin": string(storyassetdomain.OriginModel),
		"sourceCandidateId": candidate.ID, "sourceRunId": candidate.RunID, "sourceArtifactId": candidate.ArtifactID,
		"typedTable": table, "typedId": typedID, "payloadJson": payloadJSON,
	})
	if err != nil {
		return err
	}
	mapper.add(revisiondomain.OperationCreateStoryAsset, mapper.ids.NewID(), after, "确认资产信封")
	return nil
}

// addRoadmap 按 ID 映射四层路线图：卷/弧/章用稳定 ID 挂接父子引用，
// 近期章必须指向实际弧（arc_ref），章节锚点全部落为锚点操作（P1-11）。
func (mapper *archiveMapper) addRoadmap(candidate creationdomain.Candidate, road agentdomain.StoryRoadmapOutput) error {
	if mapper.planningRefs == nil {
		mapper.planningRefs = map[string]planningReference{}
	}
	nodeIDs := map[string]string{}
	allocate := func(source string) {
		id := mapper.ids.NewID()
		nodeIDs[strings.TrimSpace(source)] = id
		mapper.planningRefs[strings.TrimSpace(source)] = planningReference{Table: "outline_nodes", ID: id}
	}
	allocate(road.Book.ID)
	for _, n := range road.Volumes {
		allocate(n.ID)
	}
	for _, n := range road.Arcs {
		allocate(n.ID)
	}
	for _, n := range road.NearTermChapters {
		allocate(n.ID)
	}
	bookID := nodeIDs[strings.TrimSpace(road.Book.ID)]
	bookAfter, err := encodeJSON(map[string]any{
		"level": storydomain.LevelBook, "title": road.Book.Title, "summary": road.Book.Proposition,
		"goal": road.Book.ProtagonistArc, "conflict": strings.Join(road.Book.Inviolable, "；"),
		"outcome": road.Book.Ending, "position": 1,
	})
	if err != nil {
		return err
	}
	mapper.add(revisiondomain.OperationCreateOutlineNode, bookID, bookAfter, "确认全书层")
	if err := mapper.addTypedAsset(candidate, storyassetdomain.KindOutlineNode, "outline_nodes", bookID, bookAfter); err != nil {
		return err
	}
	mapper.outlines++
	volumeIDs := map[string]string{}
	for _, volume := range road.Volumes {
		id := nodeIDs[strings.TrimSpace(volume.ID)]
		volumeIDs[strings.TrimSpace(volume.ID)] = id
		after, err := encodeJSON(map[string]any{
			"parentId": bookID, "level": storydomain.LevelVolume, "title": volume.Title,
			"summary": volume.Goal, "goal": volume.Goal, "conflict": volume.Conflict,
			"outcome": volume.Payoff, "position": volume.Position,
		})
		if err != nil {
			return err
		}
		mapper.add(revisiondomain.OperationCreateOutlineNode, id, after, "确认卷层")
		if err := mapper.addTypedAsset(candidate, storyassetdomain.KindOutlineNode, "outline_nodes", id, after); err != nil {
			return err
		}
		mapper.outlines++
	}
	arcIDs := map[string]string{}
	for _, arc := range road.Arcs {
		parent := volumeIDs[strings.TrimSpace(arc.VolumeRef)]
		if parent == "" {
			return creationdomain.ErrStageIncomplete
		}
		id := nodeIDs[strings.TrimSpace(arc.ID)]
		arcIDs[strings.TrimSpace(arc.ID)] = id
		afterMap := map[string]any{
			"parentId": parent, "level": storydomain.LevelArc, "title": arc.Title,
			"summary": arc.CausalChain, "goal": arc.CausalChain, "conflict": arc.CharacterChange,
			"outcome": strings.Join(arc.Foreshadowing, "；"), "position": arc.Position,
		}
		if planning := arc.PlanningObject(); planning != nil {
			bindings := map[string]planningReference{}
			for _, c := range arc.Commitments {
				for _, ref := range c.RelatedRefs {
					target, ok := mapper.planningRefs[ref]
					if !ok {
						return creationdomain.ErrStageIncomplete
					}
					bindings[ref] = target
				}
			}
			planning["reference_bindings"] = bindings
			planning["schema_version"] = 1
			afterMap["planning"] = planning
		}
		after, err := encodeJSON(afterMap)
		if err != nil {
			return err
		}
		mapper.add(revisiondomain.OperationCreateOutlineNode, id, after, "确认故事弧")
		if err := mapper.addTypedAsset(candidate, storyassetdomain.KindOutlineNode, "outline_nodes", id, after); err != nil {
			return err
		}
		mapper.outlines++
	}
	for _, chapter := range road.NearTermChapters {
		parent := arcIDs[strings.TrimSpace(chapter.ArcRef)]
		if parent == "" {
			return creationdomain.ErrStageIncomplete
		}
		id := nodeIDs[strings.TrimSpace(chapter.ID)]
		after, err := encodeJSON(map[string]any{
			"parentId": parent, "level": storydomain.LevelChapter, "title": chapter.Title,
			"summary": chapter.Goal, "goal": chapter.Goal, "conflict": chapter.Conflict,
			"outcome": chapter.Outcome, "position": chapter.Position,
		})
		if err != nil {
			return err
		}
		mapper.add(revisiondomain.OperationCreateOutlineNode, id, after, "确认近期章节范围")
		if err := mapper.addTypedAsset(candidate, storyassetdomain.KindOutlineNode, "outline_nodes", id, after); err != nil {
			return err
		}
		mapper.outlines++
		for _, anchor := range chapter.Anchors {
			if err := mapper.addAnchor(candidate, anchor.Strength, "roadmap_node", id, anchor.Statement); err != nil {
				return err
			}
		}
	}
	return nil
}

func decodeBlueprint(payload string) (agentdomain.StoryBlueprintOutput, error) {
	decoded, err := agentdomain.DecodeTaskOutput(agentdomain.TaskStoryBlueprint, []byte(payload))
	if err != nil {
		return agentdomain.StoryBlueprintOutput{}, err
	}
	output := *decoded.(*agentdomain.StoryBlueprintOutput)
	if err := output.ValidateProduct(); err != nil {
		return agentdomain.StoryBlueprintOutput{}, err
	}
	for _, anchor := range output.Anchors {
		if strings.TrimSpace(anchor.ScopeHint) != "proposal" {
			return agentdomain.StoryBlueprintOutput{}, creationdomain.ErrStageIncomplete
		}
	}
	return output, nil
}

func decodeArchivableCast(candidate creationdomain.Candidate) (agentdomain.StoryCastOutput, error) {
	if candidate.PayloadSchemaID != "story_cast.schema" || candidate.PayloadSchemaVersion != 2 {
		return agentdomain.StoryCastOutput{}, creationdomain.ErrStageIncomplete
	}
	output, err := decodeCast(candidate.PayloadJSON)
	if err != nil {
		return agentdomain.StoryCastOutput{}, err
	}
	if err := output.ValidateStableReferences(); err != nil {
		return agentdomain.StoryCastOutput{}, err
	}
	return output, nil
}

// decodeCast 兼容读取历史 v1 与当前 v2 人物世界载荷；正式入档另行要求 v2 稳定引用。
func decodeCast(payload string) (agentdomain.StoryCastOutput, error) {
	decoded, err := agentdomain.DecodeTaskOutput(agentdomain.TaskStoryCast, []byte(payload))
	if err != nil {
		return agentdomain.StoryCastOutput{}, err
	}
	return *decoded.(*agentdomain.StoryCastOutput), nil
}

func decodeRoadmap(payload string) (agentdomain.StoryRoadmapOutput, error) {
	decoded, err := agentdomain.DecodeTaskOutput(agentdomain.TaskStoryRoadmap, []byte(payload))
	if err != nil {
		return agentdomain.StoryRoadmapOutput{}, err
	}
	return *decoded.(*agentdomain.StoryRoadmapOutput), nil
}

func decodeArchivableRoadmap(candidate creationdomain.Candidate) (agentdomain.StoryRoadmapOutput, error) {
	road, err := decodeRoadmap(candidate.PayloadJSON)
	if err != nil {
		return agentdomain.StoryRoadmapOutput{}, err
	}
	if candidate.PayloadSchemaVersion >= 3 {
		if err := road.ValidatePlanningGoals(); err != nil {
			return agentdomain.StoryRoadmapOutput{}, err
		}
	}
	return road, nil
}

func encodeJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// withSourceLocks 将来源锁冻结进完整规划载荷，后续版本不能因候选更换而解除保护。
func withSourceLocks(payload string, locks map[string]bool) (string, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &object); err != nil {
		return "", err
	}
	object["_sourceLocks"], _ = json.Marshal(locks)
	encoded, err := json.Marshal(object)
	return string(encoded), err
}
