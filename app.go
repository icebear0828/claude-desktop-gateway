package main

import (
	"context"

	"github.com/local/claude-desktop-gateway/internal/config"
	"github.com/local/claude-desktop-gateway/internal/gui"
)

type App struct {
	ctx     context.Context
	service *gui.Service
}

func NewApp() *App {
	return &App{service: gui.NewService(gui.Options{ManageGateway: true})}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.service.TryStartGateway(ctx)
}

func (a *App) Shutdown(ctx context.Context) {
	_ = a.service.StopGateway(ctx)
}

func (a *App) Dashboard() gui.Dashboard {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return a.service.Dashboard(ctx)
}

func (a *App) Editor() (gui.EditorState, error) {
	return a.service.Editor()
}

func (a *App) SaveConfig(input config.EditableFile) (gui.EditorState, error) {
	return a.service.SaveConfig(input)
}

func (a *App) SaveSecret(input gui.SecretInput) (gui.EditorState, error) {
	return a.service.SaveSecret(input)
}

func (a *App) DeleteSecret(input gui.SecretNameInput) (gui.EditorState, error) {
	return a.service.DeleteSecret(input)
}

func (a *App) ApplyClaudeDesktopConfig() (gui.ClaudeDesktopApplyResult, error) {
	return a.service.ApplyClaudeDesktopConfig()
}

func (a *App) ApplyCodexAppConfig() (gui.CodexAppApplyResult, error) {
	return a.service.ApplyCodexAppConfig()
}

func (a *App) SuggestedDesktopID(upstreamModel string) string {
	return a.service.SuggestedDesktopID(upstreamModel)
}
