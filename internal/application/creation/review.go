package creation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

var (
	ErrCandidateNotFound = errors.New("creation candidate not found")
	// ErrReviewIncomplete 表示仍有未决定的审阅单位，用于入档前的显式选择要求。
	ErrReviewIncomplete = errors.New("creation review is incomplete")
)

// ReviewRefView 是审阅单位在驾驶舱中的投影标识。
type ReviewRefView struct {
	Scope string `json:"scope"`
	ID    string `json:"id"`
}

// ReviewView 是权威的逐项审阅投影：接受、排除与仍未决定的单位。
type ReviewView struct {
	CandidateID   string          `json:"candidateId"`
	Stage         string          `json:"stage"`
	ReviewVersion int64           `json:"reviewVersion"`
	Accepted      []ReviewRefView `json:"accepted"`
	Rejected      []ReviewRefView `json:"rejected"`
	Unreviewed    []ReviewRefView `json:"unreviewed"`
	Complete      bool            `json:"complete"`
	UpdatedAt     string          `json:"updatedAt"`
}

// ReviewCursorView 是作者审阅位置，供重启后恢复同一候选与分节/实体。
type ReviewCursorView struct {
	Stage         string `json:"stage"`
	CandidateID   string `json:"candidateId"`
	EntityID      string `json:"entityId"`
	SectionID     string `json:"sectionId"`
	ComparisonRef string `json:"comparisonRef"`
	ViewVersion   int64  `json:"viewVersion"`
	UpdatedAt     string `json:"updatedAt"`
}

// ReviewFieldDiff 描述同一阶段两个候选版本之间的文本字段差异。
type ReviewFieldDiff struct {
	Path   string `json:"path"`
	Label  string `json:"label"`
	Group  string `json:"group"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// CandidateVersionView 是一个历史版本的审阅摘要与相对比较结果。
type CandidateVersionView struct {
	CandidateID string            `json:"candidateId"`
	Version     int               `json:"version"`
	Kind        string            `json:"kind"`
	Origin      string            `json:"origin"`
	Status      string            `json:"status"`
	Quality     string            `json:"quality"`
	Summary     string            `json:"summary"`
	CreatedAt   string            `json:"createdAt"`
	Current     bool              `json:"current"`
	Diff        []ReviewFieldDiff `json:"diff"`
}

// CandidateVersionSummaryView 是工作台中的版本列表项，不含逐字段差异。
type CandidateVersionSummaryView struct {
	CandidateID string `json:"candidateId"`
	Version     int    `json:"version"`
	Kind        string `json:"kind"`
	Origin      string `json:"origin"`
	Status      string `json:"status"`
	Quality     string `json:"quality"`
	Summary     string `json:"summary"`
	CreatedAt   string `json:"createdAt"`
	Current     bool   `json:"current"`
}

// CandidateCompareView 汇总同一阶段的版本，差异相对指定基线计算。
type CandidateCompareView struct {
	Stage           string                 `json:"stage"`
	BaseCandidateID string                 `json:"baseCandidateId"`
	Versions        []CandidateVersionView `json:"versions"`
}

// ReviewRequest 是逐项审阅命令；决定只改审阅元数据，不改候选载荷。
type ReviewRequest struct {
	CandidateID string `json:"candidateId"`
	Scope       string `json:"scope"`
	RefID       string `json:"refId"`
	Decision    string `json:"decision"`
}

// ReviewCursorRequest 移动作者审阅位置；不使生成输入或候选内容失效。
type ReviewCursorRequest struct {
	Stage         string `json:"stage"`
	CandidateID   string `json:"candidateId"`
	EntityID      string `json:"entityId"`
	SectionID     string `json:"sectionId"`
	ComparisonRef string `json:"comparisonRef"`
}

const (
	maxReviewAttempts    = 3
	maxComparisonDiffs   = 200
	maxReviewCursorField = 256
)

// SetReview 记录作者对单个实体或分节的接受、排除或撤销决定。
func (service *Service) SetReview(ctx context.Context, projectID string, request ReviewRequest) (ReviewView, error) {
	target, err := service.reviewTarget(ctx, projectID, request.CandidateID)
	if err != nil {
		return ReviewView{}, err
	}
	ref, err := creationdomain.NewReviewRef(creationdomain.ReviewScope(strings.TrimSpace(request.Scope)), request.RefID)
	if err != nil {
		return ReviewView{}, err
	}
	if !containsRef(target.refs[ref.Scope], ref) {
		return ReviewView{}, creationdomain.ErrReviewRefMissing
	}
	decision := creationdomain.ReviewDecision(strings.TrimSpace(request.Decision))
	if !decision.Valid() {
		return ReviewView{}, creationdomain.ErrBlankValue
	}
	now := service.clock.Now()
	review, err := service.mutateReview(ctx, target, func(review creationdomain.CandidateReview) (creationdomain.CandidateReview, error) {
		return review.Decide(ref, decision, now)
	})
	if err != nil {
		return ReviewView{}, err
	}
	return projectReview(review, target.refs), nil
}

// AcceptAllReview 是显式“全部接受”命令：接受当前全部审阅单位并清除它们的排除记录。
func (service *Service) AcceptAllReview(ctx context.Context, projectID, candidateID string) (ReviewView, error) {
	target, err := service.reviewTarget(ctx, projectID, candidateID)
	if err != nil {
		return ReviewView{}, err
	}
	all := append(append([]creationdomain.ReviewRef{}, target.refs[creationdomain.ReviewScopeEntity]...),
		target.refs[creationdomain.ReviewScopeSection]...)
	if len(all) == 0 {
		return ReviewView{}, creationdomain.ErrReviewRefMissing
	}
	now := service.clock.Now()
	review, err := service.mutateReview(ctx, target, func(review creationdomain.CandidateReview) (creationdomain.CandidateReview, error) {
		return review.AcceptAll(all, now)
	})
	if err != nil {
		return ReviewView{}, err
	}
	return projectReview(review, target.refs), nil
}

// UseCandidateVersion 显式切换某阶段的工作基线；比较或打开新结果不会切换。
// 同一阶段同一类的其他候选会被标记为已替代，接受的候选不能静默换版。
func (service *Service) UseCandidateVersion(ctx context.Context, projectID, candidateID string) (creationdomain.Candidate, error) {
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	target, ok := candidateByID(candidates, strings.TrimSpace(candidateID))
	if !ok || target.SessionID != session.ID || !stageReached(session.Stage, target.Stage) {
		return creationdomain.Candidate{}, ErrCandidateNotFound
	}
	if target.Status == creationdomain.StatusDiscarded {
		return creationdomain.Candidate{}, creationdomain.ErrInvalidTransition
	}
	if target.Blocked() {
		return creationdomain.Candidate{}, creationdomain.ErrQualityBlocked
	}
	if target.Status == creationdomain.StatusStale {
		return creationdomain.Candidate{}, creationdomain.ErrImpactUnresolved
	}
	current, ok := currentCandidateOfKind(candidates, target.Stage, target.Kind)
	if ok && current.ID == target.ID {
		return current, nil
	}
	if ok && current.Status == creationdomain.StatusConfirmed {
		return creationdomain.Candidate{}, creationdomain.ErrCandidateAcceptanceRequired
	}
	now := service.clock.Now()
	restored, err := target.Restore(now)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	expects := []creationdomain.Candidate{target}
	updates := []creationdomain.Candidate{restored}
	for _, candidate := range candidates {
		if candidate.ID == target.ID || candidate.Stage != target.Stage || candidate.Kind != target.Kind {
			continue
		}
		if candidate.Status == creationdomain.StatusSuperseded || candidate.Status == creationdomain.StatusDiscarded {
			continue
		}
		superseded, err := candidate.MarkSuperseded(now)
		if err != nil {
			return creationdomain.Candidate{}, err
		}
		expects = append(expects, candidate)
		updates = append(updates, superseded)
	}
	if err := service.store.ApplyCandidateChanges(ctx, expects, updates, nil); err != nil {
		return creationdomain.Candidate{}, err
	}
	if ok {
		if err := service.followDirectionSelection(ctx, session, target, current.ID); err != nil {
			return creationdomain.Candidate{}, err
		}
	}
	latest, err := service.loadSession(ctx, projectID)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	if err := service.refreshImpact(ctx, latest); err != nil {
		return creationdomain.Candidate{}, err
	}
	return restored, nil
}

// followDirectionSelection 在工作基线切换后迁移方向选择，卡标识保持不变。
func (service *Service) followDirectionSelection(
	ctx context.Context,
	session creationdomain.Session,
	target creationdomain.Candidate,
	previousID string,
) error {
	if target.Kind != creationdomain.KindDirectionSet || session.SelectedDirectionCandidateID != previousID || previousID == "" {
		return nil
	}
	updated, err := session.FollowDirectionCandidate(previousID, target.ID, service.clock.Now())
	if err != nil {
		return err
	}
	return service.saveSession(ctx, &updated)
}

// CompareCandidateVersions 只读比较同阶段候选版本；不切换工作基线，也不调用模型。
func (service *Service) CompareCandidateVersions(ctx context.Context, projectID, stage, baseCandidateID string) (CandidateCompareView, error) {
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return CandidateCompareView{}, err
	}
	target := creationdomain.Stage(strings.TrimSpace(stage))
	if !target.Valid() {
		return CandidateCompareView{}, ErrInvalidStage
	}
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return CandidateCompareView{}, err
	}
	base, ok := candidateByID(candidates, strings.TrimSpace(baseCandidateID))
	if !ok {
		base, ok = currentCandidate(candidates, target)
		if !ok {
			return CandidateCompareView{Stage: string(target), Versions: []CandidateVersionView{}}, nil
		}
	}
	if base.Stage != target {
		return CandidateCompareView{}, ErrInvalidStage
	}
	baseFields := map[string]TextFieldView{}
	for _, field := range candidateTextFields(base).Fields {
		baseFields[field.Path] = field
	}
	view := CandidateCompareView{Stage: string(target), BaseCandidateID: base.ID, Versions: []CandidateVersionView{}}
	for _, candidate := range candidates {
		if candidate.Stage != target {
			continue
		}
		fields := map[string]TextFieldView{}
		for _, field := range candidateTextFields(candidate).Fields {
			fields[field.Path] = field
		}
		view.Versions = append(view.Versions, CandidateVersionView{
			CandidateID: candidate.ID, Version: candidate.Version, Kind: string(candidate.Kind),
			Origin: string(candidate.Origin), Status: string(candidate.Status), Quality: string(candidate.QualityStatus),
			Summary: candidateVersionSummary(candidate), CreatedAt: candidate.CreatedAt.Format(time.RFC3339Nano),
			Current: candidate.ID == base.ID, Diff: diffTextFieldViews(baseFields, fields),
		})
	}
	sort.Slice(view.Versions, func(i, j int) bool { return view.Versions[i].Version > view.Versions[j].Version })
	return view, nil
}

// SaveReviewCursor 保存作者审阅位置；位置使用独立 view_version，不影响内容版本。
func (service *Service) SaveReviewCursor(ctx context.Context, projectID string, request ReviewCursorRequest) (ReviewCursorView, error) {
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return ReviewCursorView{}, err
	}
	stage := creationdomain.Stage(strings.TrimSpace(request.Stage))
	if !stage.Valid() || !stageReached(session.Stage, stage) {
		return ReviewCursorView{}, ErrInvalidStage
	}
	if len(request.EntityID) > maxReviewCursorField || len(request.SectionID) > maxReviewCursorField ||
		len(request.ComparisonRef) > maxReviewCursorField {
		return ReviewCursorView{}, creationdomain.ErrBlankValue
	}
	if candidateID := strings.TrimSpace(request.CandidateID); candidateID != "" {
		candidates, err := service.store.ListCandidates(ctx, session.ID)
		if err != nil {
			return ReviewCursorView{}, err
		}
		candidate, ok := candidateByID(candidates, candidateID)
		if !ok || candidate.SessionID != session.ID {
			return ReviewCursorView{}, ErrCandidateNotFound
		}
		if err := validateCursorRef(candidate, request); err != nil {
			return ReviewCursorView{}, err
		}
	}
	now := service.clock.Now()
	for range maxReviewAttempts {
		stored, err := service.store.ReviewCursor(ctx, session.ID)
		current := stored
		if errors.Is(err, ErrReviewCursorNotFound) {
			current = creationdomain.NewReviewCursor(session.ID, now)
		} else if err != nil {
			return ReviewCursorView{}, err
		}
		moved, err := current.Moved(stage, request.CandidateID, request.EntityID, request.SectionID, request.ComparisonRef, now)
		if err != nil {
			return ReviewCursorView{}, err
		}
		if err := service.store.SaveReviewCursor(ctx, moved); err != nil {
			if errors.Is(err, creationdomain.ErrReviewVersionConflict) {
				continue
			}
			return ReviewCursorView{}, err
		}
		saved, err := service.store.ReviewCursor(ctx, session.ID)
		if err != nil {
			return ReviewCursorView{}, err
		}
		return projectReviewCursor(saved), nil
	}
	return ReviewCursorView{}, creationdomain.ErrReviewVersionConflict
}

// reviewedCandidate 是逐项审阅命令的校验结果：目标候选与它可审阅的单位。
type reviewedCandidate struct {
	candidate creationdomain.Candidate
	refs      map[creationdomain.ReviewScope][]creationdomain.ReviewRef
}

// reviewTarget 校验命令目标：候选必须属于该会话，且是该阶段当前工作版本（历史版本只读）。
func (service *Service) reviewTarget(ctx context.Context, projectID, candidateID string) (reviewedCandidate, error) {
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return reviewedCandidate{}, err
	}
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return reviewedCandidate{}, err
	}
	target, ok := candidateByID(candidates, strings.TrimSpace(candidateID))
	if !ok || target.SessionID != session.ID || !stageReached(session.Stage, target.Stage) {
		return reviewedCandidate{}, ErrCandidateNotFound
	}
	current, ok := currentCandidateOfKind(candidates, target.Stage, target.Kind)
	if !ok || current.ID != target.ID {
		return reviewedCandidate{}, creationdomain.ErrReviewTargetNotCurrent
	}
	return reviewedCandidate{candidate: target, refs: reviewRefs(target)}, nil
}

// mutateReview 在读改写中重试审阅版本冲突；所有决定都是幂等集合操作。
func (service *Service) mutateReview(
	ctx context.Context,
	target reviewedCandidate,
	mutate func(creationdomain.CandidateReview) (creationdomain.CandidateReview, error),
) (creationdomain.CandidateReview, error) {
	candidate := target.candidate
	now := service.clock.Now()
	for range maxReviewAttempts {
		stored, err := service.store.Review(ctx, candidate.ID)
		if err != nil {
			return creationdomain.CandidateReview{}, err
		}
		expected := stored
		current := stored
		if current.SessionID == "" {
			current, err = creationdomain.NewCandidateReview(candidate, now)
			if err != nil {
				return creationdomain.CandidateReview{}, err
			}
		}
		updated, err := mutate(current)
		if err != nil {
			return creationdomain.CandidateReview{}, err
		}
		if reflect.DeepEqual(updated.Accepted, current.Accepted) && reflect.DeepEqual(updated.Rejected, current.Rejected) {
			if stored.SessionID == "" {
				return current, nil
			}
			return stored, nil
		}
		if err := service.store.SaveReview(ctx, expected, updated); err != nil {
			if errors.Is(err, creationdomain.ErrReviewVersionConflict) {
				continue
			}
			return creationdomain.CandidateReview{}, err
		}
		saved, err := service.store.Review(ctx, candidate.ID)
		if err != nil {
			return creationdomain.CandidateReview{}, err
		}
		return saved, nil
	}
	return creationdomain.CandidateReview{}, creationdomain.ErrReviewVersionConflict
}

// reviewRefsOfStage 返回该阶段当前工作版本的审阅单位；没有候选时返回空集合。
func reviewRefsOfStage(candidates []creationdomain.Candidate, stage creationdomain.Stage) map[creationdomain.ReviewScope][]creationdomain.ReviewRef {
	current, ok := currentCandidate(candidates, stage)
	if !ok {
		return map[creationdomain.ReviewScope][]creationdomain.ReviewRef{
			creationdomain.ReviewScopeEntity: {}, creationdomain.ReviewScopeSection: {},
		}
	}
	return reviewRefs(current)
}

// projectVersionSummaries 列出该阶段全部历史版本，标记当前工作基线。
func projectVersionSummaries(candidates []creationdomain.Candidate, stage creationdomain.Stage) []CandidateVersionSummaryView {
	current, hasCurrent := currentCandidate(candidates, stage)
	views := []CandidateVersionSummaryView{}
	for _, candidate := range candidates {
		if candidate.Stage != stage {
			continue
		}
		views = append(views, CandidateVersionSummaryView{
			CandidateID: candidate.ID, Version: candidate.Version, Kind: string(candidate.Kind),
			Origin: string(candidate.Origin), Status: string(candidate.Status), Quality: string(candidate.QualityStatus),
			Summary: candidateVersionSummary(candidate), CreatedAt: candidate.CreatedAt.Format(time.RFC3339Nano),
			Current: hasCurrent && candidate.ID == current.ID,
		})
	}
	sort.Slice(views, func(i, j int) bool { return views[i].Version > views[j].Version })
	return views
}

// reviewRefs 从公开投影派生候选可审阅单位：方案分节与人物世界/路线图实体。
func reviewRefs(candidate creationdomain.Candidate) map[creationdomain.ReviewScope][]creationdomain.ReviewRef {
	refs := map[creationdomain.ReviewScope][]creationdomain.ReviewRef{
		creationdomain.ReviewScopeEntity:  {},
		creationdomain.ReviewScopeSection: {},
	}
	add := func(scope creationdomain.ReviewScope, id string) {
		ref, err := creationdomain.NewReviewRef(scope, id)
		if err != nil {
			return
		}
		refs[scope] = append(refs[scope], ref)
	}
	switch candidate.Kind {
	case creationdomain.KindProposal:
		for _, section := range projectProposal(candidate, creationdomain.CandidateReview{}).Sections {
			add(creationdomain.ReviewScopeSection, section.ID)
		}
	case creationdomain.KindPeopleWorld:
		for _, entity := range projectPeopleWorld(candidate, creationdomain.CandidateReview{}).Entities {
			add(creationdomain.ReviewScopeEntity, entity.ID)
		}
	case creationdomain.KindRoadmap:
		road := projectRoadmap(candidate, creationdomain.CandidateReview{})
		add(creationdomain.ReviewScopeEntity, road.Book.ID)
		for _, node := range road.Volumes {
			add(creationdomain.ReviewScopeEntity, node.ID)
		}
		for _, node := range road.Arcs {
			add(creationdomain.ReviewScopeEntity, node.ID)
		}
		for _, node := range road.Chapters {
			add(creationdomain.ReviewScopeEntity, node.ID)
		}
	}
	sort.Slice(refs[creationdomain.ReviewScopeEntity], func(i, j int) bool {
		return refs[creationdomain.ReviewScopeEntity][i].ID < refs[creationdomain.ReviewScopeEntity][j].ID
	})
	return refs
}

// projectReview 把审阅记录投影为公开结构，未记录时全部单位按未审阅处理。
func projectReview(review creationdomain.CandidateReview, refs map[creationdomain.ReviewScope][]creationdomain.ReviewRef) ReviewView {
	view := ReviewView{
		CandidateID: review.CandidateID, Stage: string(review.Stage), ReviewVersion: review.ReviewVersion,
		Accepted: []ReviewRefView{}, Rejected: []ReviewRefView{}, Unreviewed: []ReviewRefView{},
	}
	if !review.UpdatedAt.IsZero() {
		view.UpdatedAt = review.UpdatedAt.Format(time.RFC3339Nano)
	}
	all := append(append([]creationdomain.ReviewRef{}, refs[creationdomain.ReviewScopeEntity]...),
		refs[creationdomain.ReviewScopeSection]...)
	for _, ref := range all {
		switch review.State(ref) {
		case creationdomain.DecisionAccept:
			view.Accepted = append(view.Accepted, ReviewRefView{Scope: string(ref.Scope), ID: ref.ID})
		case creationdomain.DecisionReject:
			view.Rejected = append(view.Rejected, ReviewRefView{Scope: string(ref.Scope), ID: ref.ID})
		default:
			view.Unreviewed = append(view.Unreviewed, ReviewRefView{Scope: string(ref.Scope), ID: ref.ID})
		}
	}
	view.Complete = len(all) > 0 && len(view.Unreviewed) == 0
	return view
}

func projectReviewCursor(cursor creationdomain.ReviewCursor) ReviewCursorView {
	view := ReviewCursorView{
		Stage: string(cursor.Stage), CandidateID: cursor.CandidateID, EntityID: cursor.EntityID,
		SectionID: cursor.SectionID, ComparisonRef: cursor.ComparisonRef, ViewVersion: cursor.ViewVersion,
	}
	if !cursor.UpdatedAt.IsZero() {
		view.UpdatedAt = cursor.UpdatedAt.Format(time.RFC3339Nano)
	}
	return view
}

// diffTextFieldViews 按稳定路径排序返回两个版本之间的文本差异，仅保留实际变化。
func diffTextFieldViews(before, after map[string]TextFieldView) []ReviewFieldDiff {
	paths := make([]string, 0, len(before)+len(after))
	seen := map[string]bool{}
	for path := range before {
		seen[path] = true
		paths = append(paths, path)
	}
	for path := range after {
		if !seen[path] {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	diffs := make([]ReviewFieldDiff, 0, len(paths))
	for _, path := range paths {
		left, right := before[path], after[path]
		if left.Value == right.Value {
			continue
		}
		label, group := right.Label, right.Group
		if label == "" {
			label, group = left.Label, left.Group
		}
		diffs = append(diffs, ReviewFieldDiff{Path: path, Label: label, Group: group, Before: left.Value, After: right.Value})
		if len(diffs) >= maxComparisonDiffs {
			break
		}
	}
	return diffs
}

// candidateVersionSummary 用作者可读的标识描述版本，不暴露内部 ID 或载荷。
func candidateVersionSummary(candidate creationdomain.Candidate) string {
	switch candidate.Kind {
	case creationdomain.KindProposal:
		if view := projectProposal(candidate, creationdomain.CandidateReview{}); view.Title != "" {
			return view.Title
		}
		return "作品方案"
	case creationdomain.KindPeopleWorld:
		view := projectPeopleWorld(candidate, creationdomain.CandidateReview{})
		counts := map[string]int{}
		for _, entity := range view.Entities {
			counts[entity.Kind]++
		}
		return fmt.Sprintf("%d 个人物 · %d 个势力 · %d 个地点 · %d 条规则",
			counts["character"], counts["faction"], counts["location"], counts["world_rule"])
	case creationdomain.KindRoadmap:
		view := projectRoadmap(candidate, creationdomain.CandidateReview{})
		return fmt.Sprintf("%s · %d 卷 · %d 故事弧 · %d 近期章节",
			view.Book.Title, len(view.Volumes), len(view.Arcs), len(view.Chapters))
	case creationdomain.KindDirectionSet:
		view := projectDirectionSet(candidate, "")
		if len(view.Cards) > 0 {
			return view.Cards[0].Hook
		}
		return "故事方向"
	case creationdomain.KindDirection:
		var card struct {
			Hook string `json:"hook"`
		}
		if json.Unmarshal([]byte(candidate.PayloadJSON), &card) == nil && card.Hook != "" {
			return card.Hook
		}
		return "组合方向"
	default:
		return "候选"
	}
}

func containsRef(refs []creationdomain.ReviewRef, ref creationdomain.ReviewRef) bool {
	for _, item := range refs {
		if item == ref {
			return true
		}
	}
	return false
}

// validateCursorRef 校验审阅位置引用的实体/分节确实属于该候选。
func validateCursorRef(candidate creationdomain.Candidate, request ReviewCursorRequest) error {
	refs := reviewRefs(candidate)
	if entityID := strings.TrimSpace(request.EntityID); entityID != "" {
		ref, err := creationdomain.NewReviewRef(creationdomain.ReviewScopeEntity, entityID)
		if err != nil || !containsRef(refs[creationdomain.ReviewScopeEntity], ref) {
			return creationdomain.ErrReviewRefMissing
		}
	}
	if sectionID := strings.TrimSpace(request.SectionID); sectionID != "" {
		ref, err := creationdomain.NewReviewRef(creationdomain.ReviewScopeSection, sectionID)
		if err != nil || !containsRef(refs[creationdomain.ReviewScopeSection], ref) {
			return creationdomain.ErrReviewRefMissing
		}
	}
	return nil
}
