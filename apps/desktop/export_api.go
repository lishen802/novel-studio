package main

import (
	"context"
	"errors"

	"github.com/lishen802/novel-studio/internal/application/workspace"
)

type manuscriptUseCases interface {
	ExportManuscript(context.Context, string) (workspace.ManuscriptExportView, error)
}

// ExportManuscript 只转发交付请求，格式组装与文件写入分别由应用层和平台层负责。
func (a *DesktopAPI) ExportManuscript(format string) (workspace.ManuscriptExportView, error) {
	service, ok := a.workspace.(manuscriptUseCases)
	if !ok {
		return safeResult(workspace.ManuscriptExportView{}, errors.New("manuscript export unavailable"))
	}
	return safeResult(service.ExportManuscript(a.ctx, format))
}
