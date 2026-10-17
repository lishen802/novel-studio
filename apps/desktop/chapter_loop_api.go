package main

import (
	"context"

	chapterapp "github.com/lishen802/novel-studio/internal/application/chapterloop"
	"github.com/lishen802/novel-studio/internal/application/workspace"
	loopdomain "github.com/lishen802/novel-studio/internal/domain/chapterloop"
)

type chapterLoopUseCases interface {
	GenerateOpeningSample(context.Context) (loopdomain.OpeningSample, error)
	RecordOpeningSampleFeedback(context.Context, string, string) (loopdomain.OpeningSample, error)
	PromoteOpeningSample(context.Context, string) (loopdomain.OpeningSample, error)
	CurrentOpeningSample(context.Context) (loopdomain.OpeningSample, error)
	StartChapterProduction(context.Context, workspace.StartChapterProductionInput) (chapterapp.Production, error)
	FinalizeChapterProduction(context.Context, string) (workspace.ChapterView, error)
	PrepareSelectionRewrite(context.Context, workspace.SelectionRewriteInput) (loopdomain.SelectionProposal, error)
	ExplainSelection(context.Context, workspace.SelectionRewriteInput) (string, error)
	AcceptSelectionRewrite(context.Context, workspace.AcceptSelectionInput) (workspace.ChapterView, error)
	CommitChapterPosthoc(context.Context, string, string) (loopdomain.Posthoc, error)
	RecoverChapterPosthoc(context.Context, string) (loopdomain.Posthoc, error)
}

func (a *DesktopAPI) chapterLoop() (chapterLoopUseCases, error) {
	service, ok := a.workspace.(chapterLoopUseCases)
	if !ok {
		return nil, errFeatureNotAvailable
	}
	return service, nil
}

// GenerateOpeningSample 生成非正式开篇试写。
func (a *DesktopAPI) GenerateOpeningSample() (loopdomain.OpeningSample, error) {
	service, err := a.chapterLoop()
	if err != nil {
		return loopdomain.OpeningSample{}, safeDesktopError(err)
	}
	return safeResult(service.GenerateOpeningSample(a.ctx))
}

// RecordOpeningSampleFeedback 保存试写反馈。
func (a *DesktopAPI) RecordOpeningSampleFeedback(sampleID, kind string) (loopdomain.OpeningSample, error) {
	service, err := a.chapterLoop()
	if err != nil {
		return loopdomain.OpeningSample{}, safeDesktopError(err)
	}
	return safeResult(service.RecordOpeningSampleFeedback(a.ctx, sampleID, kind))
}

// PromoteOpeningSample 在条件满足时转正试写。
func (a *DesktopAPI) PromoteOpeningSample(sampleID string) (loopdomain.OpeningSample, error) {
	service, err := a.chapterLoop()
	if err != nil {
		return loopdomain.OpeningSample{}, safeDesktopError(err)
	}
	return safeResult(service.PromoteOpeningSample(a.ctx, sampleID))
}

// CurrentOpeningSample 读取最近的试写。
func (a *DesktopAPI) CurrentOpeningSample() (loopdomain.OpeningSample, error) {
	service, err := a.chapterLoop()
	if err != nil {
		return loopdomain.OpeningSample{}, safeDesktopError(err)
	}
	return safeResult(service.CurrentOpeningSample(a.ctx))
}

// StartChapterProduction 从已确认路线图节点生成章节候选。
func (a *DesktopAPI) StartChapterProduction(input workspace.StartChapterProductionInput) (chapterapp.Production, error) {
	service, err := a.chapterLoop()
	if err != nil {
		return chapterapp.Production{}, safeDesktopError(err)
	}
	return safeResult(service.StartChapterProduction(a.ctx, input))
}

// FinalizeChapterProduction 将选中候选定稿为新版本。
func (a *DesktopAPI) FinalizeChapterProduction(chapterID string) (workspace.ChapterView, error) {
	service, err := a.chapterLoop()
	if err != nil {
		return workspace.ChapterView{}, safeDesktopError(err)
	}
	return safeResult(service.FinalizeChapterProduction(a.ctx, chapterID))
}

// PrepareSelectionRewrite 准备绑定版本和原文哈希的选区候选。
func (a *DesktopAPI) PrepareSelectionRewrite(input workspace.SelectionRewriteInput) (loopdomain.SelectionProposal, error) {
	service, err := a.chapterLoop()
	if err != nil {
		return loopdomain.SelectionProposal{}, safeDesktopError(err)
	}
	return safeResult(service.PrepareSelectionRewrite(a.ctx, input))
}

// ExplainSelection 解释选区，不覆盖正文。
func (a *DesktopAPI) ExplainSelection(input workspace.SelectionRewriteInput) (string, error) {
	service, err := a.chapterLoop()
	if err != nil {
		return "", safeDesktopError(err)
	}
	return safeResult(service.ExplainSelection(a.ctx, input))
}

// AcceptSelectionRewrite 采纳选区并创建新版本。
func (a *DesktopAPI) AcceptSelectionRewrite(input workspace.AcceptSelectionInput) (workspace.ChapterView, error) {
	service, err := a.chapterLoop()
	if err != nil {
		return workspace.ChapterView{}, safeDesktopError(err)
	}
	return safeResult(service.AcceptSelectionRewrite(a.ctx, input))
}

// CommitChapterPosthoc 提交定稿后的最小记忆。
func (a *DesktopAPI) CommitChapterPosthoc(chapterID, idempotencyKey string) (loopdomain.Posthoc, error) {
	service, err := a.chapterLoop()
	if err != nil {
		return loopdomain.Posthoc{}, safeDesktopError(err)
	}
	return safeResult(service.CommitChapterPosthoc(a.ctx, chapterID, idempotencyKey))
}

// RecoverChapterPosthoc 从来源未变的定稿恢复后验。
func (a *DesktopAPI) RecoverChapterPosthoc(chapterID string) (loopdomain.Posthoc, error) {
	service, err := a.chapterLoop()
	if err != nil {
		return loopdomain.Posthoc{}, safeDesktopError(err)
	}
	return safeResult(service.RecoverChapterPosthoc(a.ctx, chapterID))
}
