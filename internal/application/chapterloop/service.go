package chapterloop

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"unicode/utf8"

	"crypto/sha256"
	"encoding/hex"

	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	loopdomain "github.com/lishen802/novel-studio/internal/domain/chapterloop"
)

const (
	kindSample     = "opening_sample"
	kindProduction = "production"
	kindSelection  = "selection"
	kindPosthoc    = "posthoc"
)

var ErrContextBudget = errors.New("required authoring context exceeds configured budget")

// Call 是一次产品阶段的模型请求，角色由既有创作方案决定。
type Call struct {
	Stage string
	Role  string
	Input string
}

// Completion 是模型返回的文本和用量。
type Completion struct {
	Text         string
	InputTokens  int
	OutputTokens int
}

// Completer 由工作区接到现有模型供应商，测试使用脚本完成器。
type Completer interface {
	Complete(ctx context.Context, call Call) (Completion, error)
}

// Record 是一条不可变的产品记录。
type Record struct {
	ID             string
	ProjectID      string
	Kind           string
	SubjectID      string
	IdempotencyKey string
	Payload        json.RawMessage
}

// Store 追加并按对象读取产品记录。
type Store interface {
	Append(ctx context.Context, record Record) error
	Latest(ctx context.Context, projectID, kind, subjectID string) (Record, error)
	Get(ctx context.Context, projectID, id string) (Record, error)
	List(ctx context.Context, projectID, kind string) ([]Record, error)
}

// Service 编排单章产品状态，不复制智能体运行时。
type Service struct {
	store    Store
	complete Completer
	clock    ports.Clock
	ids      ports.IDGenerator
}

func NewService(store Store, complete Completer, clock ports.Clock, ids ports.IDGenerator) *Service {
	return &Service{store: store, complete: complete, clock: clock, ids: ids}
}

type OpeningSampleRequest struct {
	ProjectID            string
	HandoffID            string
	DirectionCandidateID string
	DirectionDigest      string
	CurrentDigest        string
	PointOfView          string
	Tone                 string
	Constraints          []string
	TargetRunes          int
}

type PromoteSampleRequest struct {
	ProjectID        string
	SampleID         string
	ArchiveConfirmed bool
	ExplicitAccept   bool
	CurrentDigest    string
}

type ProductionRequest struct {
	ProjectID     string
	RoadmapNodeID string
	ChapterID     string
	Confirmed     bool
	Manifest      map[string]string
	CharBudget    int
	Constraints   []string
	Control       loopdomain.RunControl
	AllowLookup   bool
	Evidence      map[string]string
	ExtraStep     *HarnessStep
}

type HarnessStep struct {
	Name        string
	Thresholds  loopdomain.HarnessThresholds
	Observation loopdomain.HarnessObservation
}

// Production 是一次可追溯的章节候选生产。
type Production struct {
	ID              string                      `json:"id"`
	ProjectID       string                      `json:"projectId"`
	RoadmapNodeID   string                      `json:"roadmapNodeId"`
	ChapterID       string                      `json:"chapterId"`
	ManifestHash    string                      `json:"manifestHash"`
	FrozenTaskHash  string                      `json:"frozenTaskHash"`
	Stages          []string                    `json:"stages"`
	Candidates      []loopdomain.Candidate      `json:"candidates"`
	SelectedID      string                      `json:"selectedId"`
	SelectionReason string                      `json:"selectionReason"`
	Findings        []loopdomain.ProductFinding `json:"findings"`
	Constraints     []string                    `json:"constraints"`
	Status          string                      `json:"status"`
	InputTokens     int                         `json:"inputTokens"`
	OutputTokens    int                         `json:"outputTokens"`
	AcceptedFacts   []string                    `json:"acceptedFacts"`
}

type PosthocRequest struct {
	ProjectID      string
	ChapterID      string
	Revision       int64
	Body           string
	Statements     []loopdomain.Statement
	IdempotencyKey string
}

type SelectionRequest struct {
	ProjectID string
	ChapterID string
	Revision  int64
	Body      string
	StartRune int
	EndRune   int
	Mode      string
}

type AcceptSelectionRequest struct {
	ProjectID   string
	ProposalID  string
	Body        string
	Revision    int64
	Partial     bool
	Replacement string
}

// GenerateOpeningSample 用已保存的局部交接生成非正式试写。
func (service *Service) GenerateOpeningSample(ctx context.Context, request OpeningSampleRequest) (loopdomain.OpeningSample, error) {
	if request.CurrentDigest != request.DirectionDigest {
		return loopdomain.OpeningSample{}, loopdomain.ErrSampleNotPromotable
	}
	completion, err := service.complete.Complete(ctx, Call{
		Stage: "prose_writing", Role: "writer",
		Input: "试写\n视角：" + request.PointOfView + "\n语气：" + request.Tone + "\n约束：" + strings.Join(request.Constraints, "；"),
	})
	if err != nil {
		return loopdomain.OpeningSample{}, err
	}
	sample, err := loopdomain.NewOpeningSample(service.ids.NewID(), request.HandoffID, request.DirectionCandidateID, request.DirectionDigest, completion.Text)
	if err != nil {
		return loopdomain.OpeningSample{}, err
	}
	return sample, service.save(ctx, request.ProjectID, kindSample, request.HandoffID, sample.ID, "", sample)
}

// RecordOpeningSampleFeedback 保存退回策划的意见。
func (service *Service) RecordOpeningSampleFeedback(ctx context.Context, projectID, sampleID string, kind loopdomain.SampleFeedbackKind) (loopdomain.OpeningSample, error) {
	sample, err := service.latestSample(ctx, projectID, sampleID)
	if err != nil {
		return loopdomain.OpeningSample{}, err
	}
	sample, err = sample.RecordFeedback(kind)
	if err != nil {
		return loopdomain.OpeningSample{}, err
	}
	sample.ID = service.ids.NewID()
	return sample, service.save(ctx, projectID, kindSample, sample.HandoffID, sample.ID, "", sample)
}

// PromoteOpeningSample 在档案确认、复检和显式接受都成立时标记转正。
func (service *Service) PromoteOpeningSample(ctx context.Context, request PromoteSampleRequest) (loopdomain.OpeningSample, error) {
	sample, err := service.latestSample(ctx, request.ProjectID, request.SampleID)
	if err != nil {
		return loopdomain.OpeningSample{}, err
	}
	sample = sample.MarkStaleIfDigestChanged(request.CurrentDigest)
	recheck := len([]rune(sample.Body)) > 0 && len([]rune(sample.Body)) <= 10000
	promoted, err := sample.Promote(request.ArchiveConfirmed, recheck, request.ExplicitAccept)
	if err != nil {
		return sample, err
	}
	promoted.ID = service.ids.NewID()
	return promoted, service.save(ctx, request.ProjectID, kindSample, promoted.HandoffID, promoted.ID, "", promoted)
}

// StartProduction 从已确认路线图节点生成章节候选，并按有效评估选稿。
func (service *Service) StartProduction(ctx context.Context, request ProductionRequest) (Production, error) {
	if !request.Confirmed || strings.TrimSpace(request.RoadmapNodeID) == "" {
		return Production{}, loopdomain.ErrInvalidProduction
	}
	if err := loopdomain.AllowCall(request.Control); err != nil {
		return Production{}, err
	}
	if err := service.preflight(ctx, request.ProjectID); err != nil {
		return Production{}, err
	}
	facts, err := service.acceptedFacts(ctx, request.ProjectID)
	if err != nil {
		return Production{}, err
	}
	manifestText, manifestHash, err := freezeManifest(request.Manifest, facts, request.CharBudget)
	if err != nil {
		return Production{}, err
	}
	production := Production{
		ID: service.ids.NewID(), ProjectID: request.ProjectID, RoadmapNodeID: request.RoadmapNodeID,
		ChapterID: request.ChapterID, ManifestHash: manifestHash, Constraints: append([]string(nil), request.Constraints...),
		AcceptedFacts: facts, Stages: []string{"context_preparing"}, Status: "running",
	}
	director, err := service.call(ctx, request.Control, "chapter_directing", "planner", manifestText)
	if err != nil {
		return Production{}, err
	}
	production.addUsage(director)
	production.Stages = append(production.Stages, "chapter_directing")
	production.FrozenTaskHash = hashPayload(director.Text)
	writerInput := `{"frozenTask":` + jsonString(director.Text) + `}`
	prose, err := service.call(ctx, request.Control, "prose_writing", "writer", writerInput)
	if err != nil {
		return Production{}, err
	}
	production.addUsage(prose)
	production.Stages = append(production.Stages, "prose_writing")
	original := loopdomain.Candidate{ID: service.ids.NewID(), Body: strings.TrimSpace(prose.Text), EvaluationID: service.ids.NewID(), Valid: true, Sequence: 1}
	production.Candidates = append(production.Candidates, original)
	var reviewInputs []string
	var rawFindings []loopdomain.ProductFinding
	for range 2 {
		review, err := service.call(ctx, request.Control, "independent_review", "reviewer", original.Body)
		if err != nil {
			return Production{}, err
		}
		production.addUsage(review)
		reviewInputs = append(reviewInputs, original.Body)
		parsed, err := parseFindings(review.Text)
		if err != nil {
			return Production{}, err
		}
		rawFindings = append(rawFindings, parsed...)
	}
	if err := loopdomain.SameFrozenBody(reviewInputs); err != nil {
		return Production{}, err
	}
	production.Stages = append(production.Stages, "independent_review")
	merged, _, err := loopdomain.MergeProductFindings(rawFindings)
	if err != nil {
		return Production{}, err
	}
	verified, err := service.verify(ctx, request, &production, merged)
	if err != nil {
		return Production{}, err
	}
	production.Findings = verified
	production.Stages = append(production.Stages, "revision_verify")
	hardConflict := false
	needsRevision := false
	for _, finding := range verified {
		if finding.Class == loopdomain.ClassHardConflict && finding.Verification == loopdomain.VerificationSupported {
			hardConflict = true
			needsRevision = true
		}
		if finding.Class == loopdomain.ClassUncertain && finding.Verification == loopdomain.VerificationSupported {
			needsRevision = true
		}
	}
	if hardConflict {
		production.Candidates[0].Valid = false
	}
	if needsRevision {
		revision, err := service.call(ctx, request.Control, "bounded_revision", "writer", revisionInput(original.Body, request.Constraints, verified))
		if err != nil {
			return Production{}, err
		}
		production.addUsage(revision)
		production.Stages = append(production.Stages, "bounded_revision")
		next, err := parseRevision(revision.Text, request.Constraints, service.ids.NewID(), service.ids.NewID())
		if err != nil {
			return Production{}, err
		}
		next.Sequence = 2
		production.Candidates = append(production.Candidates, next)
	}
	if request.ExtraStep != nil && request.ExtraStep.Thresholds.AllowsDefault(request.ExtraStep.Observation) {
		production.Stages = append(production.Stages, request.ExtraStep.Name)
	}
	chosen, reason, err := chooseCandidate(production.Candidates)
	if err != nil {
		production.Status = "stopped"
		_ = service.save(ctx, request.ProjectID, kindProduction, request.ChapterID, production.ID, "", production)
		return production, err
	}
	production.SelectedID = chosen.ID
	production.SelectionReason = reason
	production.Stages = append(production.Stages, "candidate_ready")
	production.Status = "candidate_ready"
	return production, service.save(ctx, request.ProjectID, kindProduction, request.ChapterID, production.ID, "", production)
}

// CommitPosthoc 独立提交后验；模型失败只标记待重建。
func (service *Service) CommitPosthoc(ctx context.Context, request PosthocRequest) (loopdomain.Posthoc, error) {
	if existing, ok, err := service.findPosthoc(ctx, request.ProjectID, request.ChapterID, request.IdempotencyKey); err != nil {
		return loopdomain.Posthoc{}, err
	} else if ok {
		return existing, nil
	}
	statements := request.Statements
	if len(statements) == 0 {
		completion, err := service.complete.Complete(ctx, Call{Stage: "chapter_posthoc", Role: "reviewer", Input: request.Body})
		if err != nil {
			failed, failErr := loopdomain.FailedPosthoc(service.ids.NewID(), request.ChapterID, request.Revision, request.Body, request.IdempotencyKey)
			if failErr != nil {
				return loopdomain.Posthoc{}, errors.Join(err, failErr)
			}
			saveErr := service.save(ctx, request.ProjectID, kindPosthoc, request.ChapterID, failed.ID, request.IdempotencyKey, failed)
			return failed, errors.Join(err, saveErr)
		}
		parsed, err := parseStatements(completion.Text)
		if err != nil {
			return loopdomain.Posthoc{}, err
		}
		statements = parsed
	}
	posthoc, err := loopdomain.NewPosthoc(service.ids.NewID(), request.ChapterID, request.Revision, request.Body, statements, request.IdempotencyKey)
	if err != nil {
		return loopdomain.Posthoc{}, err
	}
	committed, err := posthoc.Commit()
	if err != nil {
		return posthoc, err
	}
	return committed, service.save(ctx, request.ProjectID, kindPosthoc, request.ChapterID, committed.ID, request.IdempotencyKey, committed)
}

// PrepareSelection 绑定当前正文范围并生成改写候选。
func (service *Service) PrepareSelection(ctx context.Context, request SelectionRequest) (loopdomain.SelectionProposal, error) {
	proposal, err := loopdomain.PrepareSelection(request.ChapterID, request.Revision, request.Body, request.StartRune, request.EndRune)
	if err != nil {
		return loopdomain.SelectionProposal{}, err
	}
	proposal.ID = service.ids.NewID()
	proposal.Mode = request.Mode
	if request.Mode == "explain" {
		return proposal, nil
	}
	completion, err := service.complete.Complete(ctx, Call{Stage: request.Mode, Role: "writer", Input: proposal.SourceText})
	if err != nil {
		return loopdomain.SelectionProposal{}, err
	}
	proposal = proposal.WithReplacement(strings.TrimSpace(completion.Text), service.ids.NewID())
	return proposal, service.save(ctx, request.ProjectID, kindSelection, request.ChapterID, proposal.ID, "", proposal)
}

// AcceptSelection 在正文未变化时返回新正文；持久化采纳记录，不直接写章节。
func (service *Service) AcceptSelection(ctx context.Context, request AcceptSelectionRequest) (string, loopdomain.SelectionProposal, error) {
	record, err := service.store.Get(ctx, request.ProjectID, request.ProposalID)
	if err != nil {
		return "", loopdomain.SelectionProposal{}, err
	}
	var proposal loopdomain.SelectionProposal
	if err := json.Unmarshal(record.Payload, &proposal); err != nil {
		return "", loopdomain.SelectionProposal{}, err
	}
	replacement := proposal.Replacement
	if strings.TrimSpace(request.Replacement) != "" {
		replacement = request.Replacement
	}
	body, applied, err := loopdomain.ApplySelection(proposal, request.Body, request.Revision, replacement, request.Partial)
	if err != nil {
		return "", loopdomain.SelectionProposal{}, err
	}
	applied.ID = service.ids.NewID()
	return body, applied, service.save(ctx, request.ProjectID, kindSelection, applied.ChapterID, applied.ID, "", applied)
}

// RecoverPosthoc 用来源未变的定稿重新提交后验。
func (service *Service) RecoverPosthoc(ctx context.Context, projectID, chapterID, body string, revision int64) (loopdomain.Posthoc, error) {
	record, err := service.store.Latest(ctx, projectID, kindPosthoc, chapterID)
	if err != nil {
		return loopdomain.Posthoc{}, err
	}
	var posthoc loopdomain.Posthoc
	if err := json.Unmarshal(record.Payload, &posthoc); err != nil {
		return loopdomain.Posthoc{}, err
	}
	if posthoc.SourceRevision != revision {
		return loopdomain.Posthoc{}, loopdomain.ErrPosthocSourceChanged
	}
	recovered, err := posthoc.Recover(loopdomain.BodyHash(body))
	if err != nil {
		return loopdomain.Posthoc{}, err
	}
	recovered.ID = service.ids.NewID()
	return recovered, service.save(ctx, projectID, kindPosthoc, chapterID, recovered.ID, recovered.ID, recovered)
}

// LatestOpeningSample 读取某次交接最新的试写记录。
func (service *Service) LatestOpeningSample(ctx context.Context, projectID, handoffID string) (loopdomain.OpeningSample, error) {
	record, err := service.store.Latest(ctx, projectID, kindSample, handoffID)
	if err != nil {
		return loopdomain.OpeningSample{}, err
	}
	var sample loopdomain.OpeningSample
	if err := json.Unmarshal(record.Payload, &sample); err != nil {
		return loopdomain.OpeningSample{}, err
	}
	return sample, nil
}

// ExplainSelection 只返回说明，不改章节。
func (service *Service) ExplainSelection(ctx context.Context, request SelectionRequest) (string, error) {
	proposal, err := loopdomain.PrepareSelection(request.ChapterID, request.Revision, request.Body, request.StartRune, request.EndRune)
	if err != nil {
		return "", err
	}
	completion, err := service.complete.Complete(ctx, Call{Stage: "explain", Role: "reviewer", Input: proposal.SourceText})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(completion.Text), nil
}

func (service *Service) latestSample(ctx context.Context, projectID, sampleID string) (loopdomain.OpeningSample, error) {
	record, err := service.store.Get(ctx, projectID, sampleID)
	if err != nil {
		return loopdomain.OpeningSample{}, err
	}
	var sample loopdomain.OpeningSample
	if err := json.Unmarshal(record.Payload, &sample); err != nil {
		return loopdomain.OpeningSample{}, err
	}
	latest, err := service.store.Latest(ctx, projectID, kindSample, sample.HandoffID)
	if err != nil {
		return loopdomain.OpeningSample{}, err
	}
	if err := json.Unmarshal(latest.Payload, &sample); err != nil {
		return loopdomain.OpeningSample{}, err
	}
	return sample, nil
}

func (service *Service) preflight(ctx context.Context, projectID string) error {
	records, err := service.store.List(ctx, projectID, kindPosthoc)
	if err != nil {
		return err
	}
	latest := map[string]loopdomain.Posthoc{}
	for _, record := range records {
		var posthoc loopdomain.Posthoc
		if err := json.Unmarshal(record.Payload, &posthoc); err != nil {
			return err
		}
		latest[posthoc.ChapterID] = posthoc
	}
	for _, posthoc := range latest {
		if err := loopdomain.PreflightMemory(posthoc, posthoc.SourceHash); err != nil {
			return err
		}
	}
	return nil
}

func (service *Service) acceptedFacts(ctx context.Context, projectID string) ([]string, error) {
	records, err := service.store.List(ctx, projectID, kindPosthoc)
	if err != nil {
		return nil, err
	}
	latest := map[string]loopdomain.Posthoc{}
	for _, record := range records {
		var posthoc loopdomain.Posthoc
		if err := json.Unmarshal(record.Payload, &posthoc); err != nil {
			return nil, err
		}
		latest[posthoc.ChapterID] = posthoc
	}
	values := make([]loopdomain.Posthoc, 0, len(latest))
	for _, posthoc := range latest {
		values = append(values, posthoc)
	}
	facts := loopdomain.ReadableFacts(values)
	texts := make([]string, 0, len(facts))
	for _, fact := range facts {
		texts = append(texts, fact.Text)
	}
	sort.Strings(texts)
	return texts, nil
}

func (service *Service) findPosthoc(ctx context.Context, projectID, chapterID, key string) (loopdomain.Posthoc, bool, error) {
	records, err := service.store.List(ctx, projectID, kindPosthoc)
	if err != nil {
		return loopdomain.Posthoc{}, false, err
	}
	for _, record := range records {
		if record.SubjectID != chapterID || record.IdempotencyKey != key || key == "" {
			continue
		}
		var posthoc loopdomain.Posthoc
		if err := json.Unmarshal(record.Payload, &posthoc); err != nil {
			return loopdomain.Posthoc{}, false, err
		}
		if posthoc.Status == loopdomain.PosthocCommitted {
			return posthoc, true, nil
		}
	}
	return loopdomain.Posthoc{}, false, nil
}

func (service *Service) verify(ctx context.Context, request ProductionRequest, production *Production, findings []loopdomain.ProductFinding) ([]loopdomain.ProductFinding, error) {
	if len(findings) == 0 {
		return findings, nil
	}
	raw, err := json.Marshal(findings)
	if err != nil {
		return nil, err
	}
	completion, err := service.call(ctx, request.Control, "revision_verify", "reviewer", string(raw))
	if err != nil {
		return nil, err
	}
	production.addUsage(completion)
	var payload struct {
		Results []struct {
			ID       string `json:"id"`
			Status   string `json:"status"`
			Evidence string `json:"evidence"`
			Lookup   string `json:"lookup"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(completion.Text), &payload); err != nil {
		return nil, err
	}
	byID := map[string]struct {
		Status, Evidence, Lookup string
	}{}
	for _, result := range payload.Results {
		byID[result.ID] = struct {
			Status, Evidence, Lookup string
		}{result.Status, result.Evidence, result.Lookup}
	}
	verified := make([]loopdomain.ProductFinding, 0, len(findings))
	for _, finding := range findings {
		result := byID[finding.Finding.ID]
		evidence := result.Evidence
		if strings.TrimSpace(evidence) == "" && result.Lookup != "" {
			looked, lookErr := loopdomain.Lookup(request.Evidence, result.Lookup, request.AllowLookup)
			if lookErr != nil && !errors.Is(lookErr, loopdomain.ErrLookupMiss) && !errors.Is(lookErr, loopdomain.ErrUnauthorizedLookup) {
				return nil, lookErr
			}
			evidence = looked
		}
		next, err := loopdomain.VerifyFinding(finding, loopdomain.VerificationStatus(result.Status), evidence)
		if err != nil {
			return nil, err
		}
		verified = append(verified, next)
	}
	return verified, nil
}

func (service *Service) call(ctx context.Context, control loopdomain.RunControl, stage, role, input string) (Completion, error) {
	if err := loopdomain.AllowCall(control); err != nil {
		return Completion{}, err
	}
	return service.complete.Complete(ctx, Call{Stage: stage, Role: role, Input: input})
}

func (service *Service) save(ctx context.Context, projectID, kind, subjectID, id, key string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return service.store.Append(ctx, Record{ID: id, ProjectID: projectID, Kind: kind, SubjectID: subjectID, IdempotencyKey: key, Payload: raw})
}

func (production *Production) addUsage(completion Completion) {
	production.InputTokens += completion.InputTokens
	production.OutputTokens += completion.OutputTokens
}

func freezeManifest(manifest map[string]string, facts []string, budget int) (string, string, error) {
	keys := make([]string, 0, len(manifest))
	runes := 0
	for key, value := range manifest {
		keys = append(keys, key)
		runes += utf8.RuneCountInString(value)
	}
	for _, fact := range facts {
		runes += utf8.RuneCountInString(fact)
	}
	if budget < 1 || runes > budget {
		return "", "", ErrContextBudget
	}
	sort.Strings(keys)
	payload := map[string]any{"facts": facts, "domains": map[string]string{}}
	domains := payload["domains"].(map[string]string)
	for _, key := range keys {
		domains[key] = manifest[key]
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", "", err
	}
	return string(raw), hashPayload(string(raw)), nil
}

func hashPayload(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func jsonString(value string) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

func parseFindings(text string) ([]loopdomain.ProductFinding, error) {
	var payload struct {
		Findings []struct {
			ID       string `json:"id"`
			Category string `json:"category"`
			Message  string `json:"message"`
			Class    string `json:"class"`
			Speech   bool   `json:"speech"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return nil, err
	}
	findings := make([]loopdomain.ProductFinding, 0, len(payload.Findings))
	for _, item := range payload.Findings {
		finding := loopdomain.ProductFinding{
			Class: loopdomain.ProductClass(item.Class), SpeechOrGuess: item.Speech,
		}
		finding.Finding.ID = item.ID
		finding.Finding.Category = agentdomain.FindingCategory(item.Category)
		finding.Finding.Severity = agentdomain.SeverityMajor
		finding.Finding.Status = agentdomain.FindingOpen
		finding.Finding.Source = agentdomain.FindingModelReview
		finding.Finding.Message = item.Message
		finding.Finding.ConstraintRef = "chapter"
		findings = append(findings, finding)
	}
	return findings, nil
}

func parseRevision(text string, constraints []string, id, evaluationID string) (loopdomain.Candidate, error) {
	var payload struct {
		Prose                    string   `json:"prose"`
		Kept                     []string `json:"kept"`
		IntroducedConflict       bool     `json:"introducedConflict"`
		KnowledgeBoundaryChanged bool     `json:"knowledgeBoundaryChanged"`
		NewIssue                 string   `json:"newIssue"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return loopdomain.Candidate{}, err
	}
	if err := loopdomain.NoteRevision(constraints, payload.Kept); err != nil {
		return loopdomain.Candidate{}, err
	}
	candidate := loopdomain.Candidate{
		ID: id, Body: strings.TrimSpace(payload.Prose), EvaluationID: evaluationID, Sequence: 2,
		IntroducedConflict: payload.IntroducedConflict, KnowledgeBoundaryChanged: payload.KnowledgeBoundaryChanged,
		Valid: payload.IntroducedConflict == false && payload.KnowledgeBoundaryChanged == false && strings.TrimSpace(payload.NewIssue) == "",
	}
	if !candidate.Valid {
		candidate.EvaluationID = ""
	}
	return candidate, nil
}

func revisionInput(body string, constraints []string, findings []loopdomain.ProductFinding) string {
	raw, _ := json.Marshal(map[string]any{"body": body, "kept": constraints, "findings": findings})
	return string(raw)
}

func parseStatements(text string) ([]loopdomain.Statement, error) {
	var payload struct {
		Statements []loopdomain.Statement `json:"statements"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return nil, err
	}
	return payload.Statements, nil
}

func chooseCandidate(candidates []loopdomain.Candidate) (loopdomain.Candidate, string, error) {
	var chosen loopdomain.Candidate
	found := false
	for _, candidate := range candidates {
		if !candidate.Valid || candidate.EvaluationID == "" {
			continue
		}
		if !found || candidate.Sequence > chosen.Sequence {
			chosen = candidate
			found = true
		}
	}
	if !found {
		return loopdomain.Candidate{}, "", loopdomain.ErrCandidateNotSelectable
	}
	selected, err := loopdomain.SelectCandidate(candidates, chosen.ID, "绑定评估 "+chosen.EvaluationID+"，不采用无效后稿")
	if err != nil {
		return loopdomain.Candidate{}, "", err
	}
	return selected, "绑定评估 " + selected.EvaluationID + "，不采用无效后稿", nil
}
