package main

import "context"

type App struct {
	ctx context.Context
}

func newApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}
