package main

import (
	"context"
	"embed"
	"errors"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"winrouter/internal/singleinstance"
)

//go:embed all:frontend/dist
var frontendAssets embed.FS

//go:embed build/windows/icon.ico
var trayIcon []byte

func main() {
	instanceLock, err := singleinstance.Acquire()
	if errors.Is(err, singleinstance.ErrAlreadyRunning) {
		return
	}
	if err != nil {
		panic(err)
	}
	defer instanceLock.Close()

	application := NewApp()
	if err = wails.Run(&options.App{
		Title: "WinRouter", Width: 1120, Height: 720, MinWidth: 840, MinHeight: 560,
		AssetServer:   &assetserver.Options{Assets: frontendAssets},
		OnStartup:     func(ctx context.Context) { application.Startup(ctx); application.StartTray(trayIcon) },
		OnBeforeClose: func(ctx context.Context) bool { return application.BeforeClose(ctx) },
		OnShutdown:    func(ctx context.Context) { application.Shutdown(ctx) },
		Bind:          []any{application},
	}); err != nil {
		panic(err)
	}
}
