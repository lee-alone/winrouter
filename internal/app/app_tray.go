package app

import (
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"winrouter/internal/observability"
	"winrouter/internal/recovery"
	apptray "winrouter/internal/tray"
)

func (a *App) StartTray(icon []byte) {
	a.tray.Start(icon, apptray.Actions{
		Show:   a.showMainWindow,
		Toggle: a.toggleMainWindow,
		Start: func() {
			a.showMainWindow()
			runtime.EventsEmit(a.ctx, "tray:start-core")
		},
		Stop: func() {
			if _, err := a.StopCore(); err != nil {
				a.observations.Log(observability.LevelError, "tray", "Core stop from tray failed", newCorrelationID(), map[string]any{"error": err.Error()})
			}
		},
		Quit: func() {
			a.exiting.Store(true)
			runtime.Quit(a.ctx)
		},
	})
}

func (a *App) showMainWindow() {
	a.windowVisible.Store(true)
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}

func (a *App) toggleMainWindow() {
	if a.windowVisible.CompareAndSwap(true, false) {
		runtime.WindowHide(a.ctx)
		return
	}
	a.showMainWindow()
}

func trayStatusLabel(status recovery.Status) string {
	switch status.State {
	case recovery.StateMonitoring:
		if status.Desired {
			return "运行中"
		}
	case recovery.StateStopping:
		return "正在安全停止"
	case recovery.StateWaiting:
		return "等待网络"
	case recovery.StateRecovering:
		return "正在恢复"
	case recovery.StateFailed:
		return "恢复失败"
	}
	return "已停止"
}
