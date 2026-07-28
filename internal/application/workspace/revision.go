package workspace

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/lishen802/novel-studio/internal/application/license"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
)

type ChapterChangeInput struct {
	ChapterID    string `json:"chapterId"`
	Title        string `json:"title"`
	Summary      string `json:"summary"`
	ProposedBody string `json:"proposedBody"`
	Reason       string `json:"reason"`
	CreatedBy    string `json:"createdBy"`
}

type ChangeOperationView struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	TargetID     string `json:"targetId"`
	BaseRevision int64  `json:"baseRevision"`
	BeforeValue  string `json:"beforeValue"`
	AfterValue   string `json:"afterValue"`
	Reason       string `json:"reason"`
	Status       string `json:"status"`
}

type ChangeSetView struct {
	ID         string                `json:"id"`
	Title      string                `json:"title"`
	Summary    string                `json:"summary"`
	CreatedBy  string                `json:"createdBy"`
	Status     string                `json:"status"`
	Operations []ChangeOperationView `json:"operations"`
	CreatedAt  string                `json:"createdAt"`
	UpdatedAt  string                `json:"updatedAt"`
}

func (s *Service) ChangeSets(ctx context.Context) ([]ChangeSetView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return nil, ErrNoActiveProject
	}
	changeSets, err := s.active.store.ListChangeSets(ctx)
	if err != nil {
		return nil, fmt.Errorf("list change sets: %w", err)
	}
	sort.Slice(changeSets, func(i, j int) bool {
		if changeSets[i].CreatedAt.Equal(changeSets[j].CreatedAt) {
			return changeSets[i].ID > changeSets[j].ID
		}
		return changeSets[i].CreatedAt.After(changeSets[j].CreatedAt)
	})
	views := make([]ChangeSetView, 0, len(changeSets))
	for _, changeSet := range changeSets {
		views = append(views, changeSetView(changeSet))
	}
	return views, nil
}

func (s *Service) ProposeChapterChange(ctx context.Context, input ChapterChangeInput) (ChangeSetView, error) {
	if err := s.license.Require(ctx, license.RunAI); err != nil {
		return ChangeSetView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	project, err := s.activeProjectLocked(ctx)
	if err != nil {
		return ChangeSetView{}, err
	}
	chapter, err := s.active.store.Chapter(ctx, strings.TrimSpace(input.ChapterID))
	if err != nil || chapter.ProjectID != project.ID {
		return ChangeSetView{}, fmt.Errorf("chapter change target is invalid")
	}
	changeSet, err := revisiondomain.NewChapterBodyChangeSet(
		s.ids.NewID(),
		s.ids.NewID(),
		project.ID,
		chapter.ID,
		input.Title,
		input.Summary,
		input.CreatedBy,
		chapter.Revision,
		chapter.Body,
		input.ProposedBody,
		input.Reason,
		s.clock.Now(),
	)
	if err != nil {
		return ChangeSetView{}, err
	}
	if err := s.active.store.CreateChangeSet(ctx, changeSet); err != nil {
		return ChangeSetView{}, fmt.Errorf("create change set: %w", err)
	}
	return changeSetView(changeSet), nil
}

func (s *Service) ApplyChangeSet(ctx context.Context, id string) (ChapterView, error) {
	if err := s.license.Require(ctx, license.EditManuscript); err != nil {
		return ChapterView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return ChapterView{}, ErrNoActiveProject
	}
	chapter, err := s.active.store.ApplyChangeSet(ctx, strings.TrimSpace(id), s.clock.Now())
	if err != nil {
		return ChapterView{}, fmt.Errorf("apply change set: %w", err)
	}
	return chapterView(chapter), nil
}

func (s *Service) RejectChangeSet(ctx context.Context, id string) error {
	if err := s.license.Require(ctx, license.EditManuscript); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return ErrNoActiveProject
	}
	if err := s.active.store.RejectChangeSet(ctx, strings.TrimSpace(id), s.clock.Now()); err != nil {
		return fmt.Errorf("reject change set: %w", err)
	}
	return nil
}

func changeSetView(changeSet revisiondomain.ChangeSet) ChangeSetView {
	operations := make([]ChangeOperationView, 0, len(changeSet.Operations))
	for _, operation := range changeSet.Operations {
		operations = append(operations, ChangeOperationView{
			ID:           operation.ID,
			Kind:         string(operation.Kind),
			TargetID:     operation.TargetID,
			BaseRevision: operation.BaseRevision,
			BeforeValue:  operation.BeforeValue,
			AfterValue:   operation.AfterValue,
			Reason:       operation.Reason,
			Status:       string(operation.Status),
		})
	}
	return ChangeSetView{
		ID:         changeSet.ID,
		Title:      changeSet.Title,
		Summary:    changeSet.Summary,
		CreatedBy:  changeSet.CreatedBy,
		Status:     string(changeSet.Status),
		Operations: operations,
		CreatedAt:  changeSet.CreatedAt.Format("2006-01-02T15:04:05.999999999Z07:00"),
		UpdatedAt:  changeSet.UpdatedAt.Format("2006-01-02T15:04:05.999999999Z07:00"),
	}
}
