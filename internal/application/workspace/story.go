package workspace

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/lishen802/novel-studio/internal/application/license"
	storydomain "github.com/lishen802/novel-studio/internal/domain/story"
)

type CreateOutlineNodeInput struct {
	ParentID  string `json:"parentId"`
	Level     string `json:"level"`
	Title     string `json:"title"`
	Summary   string `json:"summary"`
	Goal      string `json:"goal"`
	Conflict  string `json:"conflict"`
	Outcome   string `json:"outcome"`
	ChapterID string `json:"chapterId"`
}

type CreateStoryLineInput struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
}

type LinkStoryLineInput struct {
	StoryLineID   string `json:"storyLineId"`
	OutlineNodeID string `json:"outlineNodeId"`
}

type CreateSceneCardInput struct {
	ChapterID     string `json:"chapterId"`
	OutlineNodeID string `json:"outlineNodeId"`
	Title         string `json:"title"`
	POV           string `json:"pov"`
	Location      string `json:"location"`
	Goal          string `json:"goal"`
	Conflict      string `json:"conflict"`
	Outcome       string `json:"outcome"`
}

type OutlineNodeView struct {
	ID        string `json:"id"`
	ParentID  string `json:"parentId"`
	Level     string `json:"level"`
	Title     string `json:"title"`
	Summary   string `json:"summary"`
	Goal      string `json:"goal"`
	Conflict  string `json:"conflict"`
	Outcome   string `json:"outcome"`
	Position  int    `json:"position"`
	ChapterID string `json:"chapterId"`
}

type StoryLineView struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Status  string `json:"status"`
}

type StoryLineLinkView struct {
	StoryLineID   string `json:"storyLineId"`
	OutlineNodeID string `json:"outlineNodeId"`
	Position      int    `json:"position"`
}

type SceneCardView struct {
	ID            string `json:"id"`
	ChapterID     string `json:"chapterId"`
	OutlineNodeID string `json:"outlineNodeId"`
	Title         string `json:"title"`
	POV           string `json:"pov"`
	Location      string `json:"location"`
	Goal          string `json:"goal"`
	Conflict      string `json:"conflict"`
	Outcome       string `json:"outcome"`
	Position      int    `json:"position"`
	Status        string `json:"status"`
}

type StoryStructureView struct {
	OutlineNodes   []OutlineNodeView   `json:"outlineNodes"`
	StoryLines     []StoryLineView     `json:"storyLines"`
	StoryLineLinks []StoryLineLinkView `json:"storyLineLinks"`
	SceneCards     []SceneCardView     `json:"sceneCards"`
}

func (s *Service) StoryStructure(ctx context.Context) (StoryStructureView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return StoryStructureView{}, ErrNoActiveProject
	}
	nodes, err := s.active.store.ListOutlineNodes(ctx)
	if err != nil {
		return StoryStructureView{}, fmt.Errorf("list outline nodes: %w", err)
	}
	lines, err := s.active.store.ListStoryLines(ctx)
	if err != nil {
		return StoryStructureView{}, fmt.Errorf("list story lines: %w", err)
	}
	links, err := s.active.store.ListStoryLineLinks(ctx)
	if err != nil {
		return StoryStructureView{}, fmt.Errorf("list story line links: %w", err)
	}
	chapters, err := s.active.store.ListChapters(ctx)
	if err != nil {
		return StoryStructureView{}, fmt.Errorf("list chapters for scene cards: %w", err)
	}
	cards := make([]storydomain.SceneCard, 0)
	for _, chapter := range chapters {
		chapterCards, err := s.active.store.ListSceneCards(ctx, chapter.ID)
		if err != nil {
			return StoryStructureView{}, fmt.Errorf("list scene cards: %w", err)
		}
		cards = append(cards, chapterCards...)
	}
	return storyStructureView(nodes, lines, links, cards), nil
}

func (s *Service) CreateOutlineNode(ctx context.Context, input CreateOutlineNodeInput) (OutlineNodeView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return OutlineNodeView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return OutlineNodeView{}, ErrNoActiveProject
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return OutlineNodeView{}, fmt.Errorf("read active project: %w", err)
	}
	nodes, err := s.active.store.ListOutlineNodes(ctx)
	if err != nil {
		return OutlineNodeView{}, fmt.Errorf("list outline nodes: %w", err)
	}
	var parent *storydomain.OutlineNode
	for i := range nodes {
		if nodes[i].ID == strings.TrimSpace(input.ParentID) {
			parent = &nodes[i]
			break
		}
	}
	if strings.TrimSpace(input.ParentID) != "" && parent == nil {
		return OutlineNodeView{}, storydomain.ErrInvalidReference
	}
	position := nextOutlinePosition(nodes, strings.TrimSpace(input.ParentID))
	node, err := storydomain.NewOutlineNode(s.ids.NewID(), project.ID, parent, storydomain.OutlineLevel(input.Level), input.Title, position, s.clock.Now())
	if err != nil {
		return OutlineNodeView{}, err
	}
	node.Summary, node.Goal = strings.TrimSpace(input.Summary), strings.TrimSpace(input.Goal)
	node.Conflict, node.Outcome = strings.TrimSpace(input.Conflict), strings.TrimSpace(input.Outcome)
	node.ChapterID = strings.TrimSpace(input.ChapterID)
	if node.ChapterID != "" {
		if node.Level != storydomain.LevelChapter {
			return OutlineNodeView{}, storydomain.ErrInvalidReference
		}
		chapter, err := s.active.store.Chapter(ctx, node.ChapterID)
		if err != nil || chapter.ProjectID != project.ID {
			return OutlineNodeView{}, storydomain.ErrInvalidReference
		}
	}
	if err := s.active.store.CreateOutlineNode(ctx, node); err != nil {
		return OutlineNodeView{}, fmt.Errorf("create outline node: %w", err)
	}
	return outlineNodeView(node), nil
}

func (s *Service) CreateStoryLine(ctx context.Context, input CreateStoryLineInput) (StoryLineView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return StoryLineView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return StoryLineView{}, ErrNoActiveProject
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return StoryLineView{}, fmt.Errorf("read active project: %w", err)
	}
	line, err := storydomain.NewStoryLine(s.ids.NewID(), project.ID, input.Name, input.Summary, s.clock.Now())
	if err != nil {
		return StoryLineView{}, err
	}
	if err := s.active.store.CreateStoryLine(ctx, line); err != nil {
		return StoryLineView{}, fmt.Errorf("create story line: %w", err)
	}
	return storyLineView(line), nil
}

func (s *Service) LinkStoryLine(ctx context.Context, input LinkStoryLineInput) (StoryLineLinkView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return StoryLineLinkView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return StoryLineLinkView{}, ErrNoActiveProject
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return StoryLineLinkView{}, fmt.Errorf("read active project: %w", err)
	}
	lines, err := s.active.store.ListStoryLines(ctx)
	if err != nil {
		return StoryLineLinkView{}, err
	}
	nodes, err := s.active.store.ListOutlineNodes(ctx)
	if err != nil {
		return StoryLineLinkView{}, err
	}
	var selected *storydomain.StoryLine
	for i := range lines {
		if lines[i].ID == input.StoryLineID {
			selected = &lines[i]
			break
		}
	}
	if selected == nil || !hasOutlineNode(nodes, input.OutlineNodeID) {
		return StoryLineLinkView{}, storydomain.ErrInvalidReference
	}
	links, err := s.active.store.ListStoryLineLinks(ctx)
	if err != nil {
		return StoryLineLinkView{}, err
	}
	position := 1
	for _, existing := range links {
		if existing.StoryLineID == selected.ID && existing.Position >= position {
			position = existing.Position + 1
		}
	}
	link, err := storydomain.NewStoryLineLink(*selected, input.OutlineNodeID, position, s.clock.Now())
	if err != nil {
		return StoryLineLinkView{}, err
	}
	if err := s.active.store.LinkStoryLine(ctx, project.ID, link); err != nil {
		return StoryLineLinkView{}, fmt.Errorf("link story line: %w", err)
	}
	return storyLineLinkView(link), nil
}

func (s *Service) CreateSceneCard(ctx context.Context, input CreateSceneCardInput) (SceneCardView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return SceneCardView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return SceneCardView{}, ErrNoActiveProject
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return SceneCardView{}, fmt.Errorf("read active project: %w", err)
	}
	chapter, err := s.active.store.Chapter(ctx, input.ChapterID)
	if err != nil || chapter.ProjectID != project.ID {
		return SceneCardView{}, storydomain.ErrInvalidReference
	}
	nodes, err := s.active.store.ListOutlineNodes(ctx)
	if err != nil {
		return SceneCardView{}, err
	}
	var sceneNode *storydomain.OutlineNode
	for i := range nodes {
		if nodes[i].ID == input.OutlineNodeID && nodes[i].Level == storydomain.LevelScene {
			sceneNode = &nodes[i]
			break
		}
	}
	if sceneNode == nil || !sceneBelongsToChapter(nodes, *sceneNode, chapter.ID) {
		return SceneCardView{}, storydomain.ErrInvalidReference
	}
	existing, err := s.active.store.ListSceneCards(ctx, chapter.ID)
	if err != nil {
		return SceneCardView{}, err
	}
	position := 1
	for _, card := range existing {
		if card.Position >= position {
			position = card.Position + 1
		}
	}
	card, err := storydomain.NewSceneCard(s.ids.NewID(), project.ID, chapter.ID, sceneNode.ID, input.Title, position, s.clock.Now())
	if err != nil {
		return SceneCardView{}, err
	}
	card.POV, card.Location = strings.TrimSpace(input.POV), strings.TrimSpace(input.Location)
	card.Goal, card.Conflict, card.Outcome = strings.TrimSpace(input.Goal), strings.TrimSpace(input.Conflict), strings.TrimSpace(input.Outcome)
	if err := s.active.store.CreateSceneCard(ctx, card); err != nil {
		return SceneCardView{}, fmt.Errorf("create scene card: %w", err)
	}
	return sceneCardView(card), nil
}

func (s *Service) ArchiveOutlineNode(ctx context.Context, id string) error {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return ErrNoActiveProject
	}
	if err := s.active.store.ArchiveOutlineNode(ctx, id, s.clock.Now()); err != nil {
		return fmt.Errorf("archive outline node: %w", err)
	}
	return nil
}

func (s *Service) MoveOutlineNode(ctx context.Context, id, direction string) error {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return err
	}
	if direction != "up" && direction != "down" {
		return storydomain.ErrInvalidPosition
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return ErrNoActiveProject
	}
	nodes, err := s.active.store.ListOutlineNodes(ctx)
	if err != nil {
		return fmt.Errorf("list outline nodes: %w", err)
	}
	var selected *storydomain.OutlineNode
	for i := range nodes {
		if nodes[i].ID == id {
			selected = &nodes[i]
			break
		}
	}
	if selected == nil {
		return storydomain.ErrInvalidReference
	}
	siblings := make([]storydomain.OutlineNode, 0)
	for _, node := range nodes {
		if node.ParentID == selected.ParentID {
			siblings = append(siblings, node)
		}
	}
	sort.Slice(siblings, func(i, j int) bool { return siblings[i].Position < siblings[j].Position })
	index := -1
	for i := range siblings {
		if siblings[i].ID == id {
			index = i
			break
		}
	}
	target := index - 1
	if direction == "down" {
		target = index + 1
	}
	if index < 0 || target < 0 || target >= len(siblings) {
		return nil
	}
	if err := s.active.store.SwapOutlineNodePositions(ctx, id, siblings[target].ID, s.clock.Now()); err != nil {
		return fmt.Errorf("move outline node: %w", err)
	}
	return nil
}

func nextOutlinePosition(nodes []storydomain.OutlineNode, parentID string) int {
	position := 1
	for _, node := range nodes {
		if node.ParentID == parentID && node.Position >= position {
			position = node.Position + 1
		}
	}
	return position
}

func hasOutlineNode(nodes []storydomain.OutlineNode, id string) bool {
	for _, node := range nodes {
		if node.ID == id {
			return true
		}
	}
	return false
}

func sceneBelongsToChapter(nodes []storydomain.OutlineNode, scene storydomain.OutlineNode, chapterID string) bool {
	for _, node := range nodes {
		if node.ID == scene.ParentID {
			return node.Level == storydomain.LevelChapter && node.ChapterID == chapterID
		}
	}
	return false
}

func storyStructureView(nodes []storydomain.OutlineNode, lines []storydomain.StoryLine, links []storydomain.StoryLineLink, cards []storydomain.SceneCard) StoryStructureView {
	view := StoryStructureView{
		OutlineNodes: make([]OutlineNodeView, 0, len(nodes)), StoryLines: make([]StoryLineView, 0, len(lines)),
		StoryLineLinks: make([]StoryLineLinkView, 0, len(links)), SceneCards: make([]SceneCardView, 0, len(cards)),
	}
	for _, node := range nodes {
		view.OutlineNodes = append(view.OutlineNodes, outlineNodeView(node))
	}
	for _, line := range lines {
		view.StoryLines = append(view.StoryLines, storyLineView(line))
	}
	for _, link := range links {
		view.StoryLineLinks = append(view.StoryLineLinks, storyLineLinkView(link))
	}
	for _, card := range cards {
		view.SceneCards = append(view.SceneCards, sceneCardView(card))
	}
	sort.Slice(view.SceneCards, func(i, j int) bool {
		if view.SceneCards[i].ChapterID == view.SceneCards[j].ChapterID {
			return view.SceneCards[i].Position < view.SceneCards[j].Position
		}
		return view.SceneCards[i].ChapterID < view.SceneCards[j].ChapterID
	})
	return view
}

func outlineNodeView(node storydomain.OutlineNode) OutlineNodeView {
	return OutlineNodeView{ID: node.ID, ParentID: node.ParentID, Level: string(node.Level), Title: node.Title, Summary: node.Summary, Goal: node.Goal, Conflict: node.Conflict, Outcome: node.Outcome, Position: node.Position, ChapterID: node.ChapterID}
}
func storyLineView(line storydomain.StoryLine) StoryLineView {
	return StoryLineView{ID: line.ID, Name: line.Name, Summary: line.Summary, Status: string(line.Status)}
}
func storyLineLinkView(link storydomain.StoryLineLink) StoryLineLinkView {
	return StoryLineLinkView{StoryLineID: link.StoryLineID, OutlineNodeID: link.OutlineNodeID, Position: link.Position}
}
func sceneCardView(card storydomain.SceneCard) SceneCardView {
	return SceneCardView{ID: card.ID, ChapterID: card.ChapterID, OutlineNodeID: card.OutlineNodeID, Title: card.Title, POV: card.POV, Location: card.Location, Goal: card.Goal, Conflict: card.Conflict, Outcome: card.Outcome, Position: card.Position, Status: string(card.Status)}
}
